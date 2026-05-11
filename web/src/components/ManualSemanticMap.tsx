import { select as d3Select } from "d3-selection";
import { zoom as d3Zoom, zoomIdentity, type ZoomTransform } from "d3-zoom";
import {
  forceSimulation,
  forceLink,
  forceManyBody,
  forceCenter,
  forceCollide,
  type SimulationNodeDatum,
  type SimulationLinkDatum,
} from "d3-force";

import { useEffect, useRef, useState } from "preact/hooks";
import { wrapText } from "../utils/canvasWordWrap";

interface ManualTopic {
  id: string;
  label: string;
  level: number;
}

interface ManualLink {
  source: string;
  target: string;
  type: "hierarchy" | "note";
}

interface ManualMapData {
  topics: ManualTopic[];
  links: ManualLink[];
}

interface Node extends SimulationNodeDatum {
  id: string;
  label: string;
  type: "topic" | "note";
  level?: number;
}

interface Link extends SimulationLinkDatum<Node> {
  type: "hierarchy" | "note";
}

interface ManualSemanticMapProps {
  auth: string;
  onOpenNote: (filename: string) => void;
  onClose: () => void;
}

// ─── hooks ──────────────────────────────────────────────────────────

/** Busca dados do mapa com AbortController, loading e error states. */
function useSemanticMapData(auth: string) {
  const [data, setData] = useState<ManualMapData | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const controller = new AbortController();
    setLoading(true);
    setError(null);
    fetch("/api/graph/manual-map", {
      headers: { Authorization: auth },
      signal: controller.signal,
    })
      .then((res) => res.json())
      .then((d) => {
        setData(d);
        setLoading(false);
      })
      .catch((err) => {
        if (err.name !== "AbortError") {
          setError(err.message || "Erro ao carregar");
          setLoading(false);
        }
      });
    return () => controller.abort();
  }, [auth]);

  return { data, loading, error };
}

// ─── main component ─────────────────────────────────────────────────

export function ManualSemanticMap({
  auth,
  onOpenNote,
  onClose,
}: ManualSemanticMapProps) {
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const simulationRef = useRef<any>(null);
  const zoomTransformRef = useRef<ZoomTransform>(zoomIdentity);
  const { data, loading, error } = useSemanticMapData(auth);
  const [tooltip, setTooltip] = useState<{
    x: number;
    y: number;
    text: string;
  } | null>(null);
  const [visible, setVisible] = useState(false);

  // fade-in entry animation
  useEffect(() => {
    requestAnimationFrame(() => setVisible(true));
  }, []);

  // ── initialize / update force graph ──────────────────────────
  useEffect(() => {
    if (!data || !canvasRef.current) return;
    const canvas = canvasRef.current;
    const dpr = window.devicePixelRatio || 1;
    canvas.width = window.innerWidth * dpr;
    canvas.height = window.innerHeight * dpr;

    const noteIds = new Set<string>();
    data.links.forEach((l) => {
      if (l.type === "note") noteIds.add(l.source);
    });

    const nodes: Node[] = [
      ...data.topics.map((t) => ({
        id: t.id,
        label: t.label,
        type: "topic" as const,
        level: t.level,
      })),
      ...Array.from(noteIds).map((id) => ({
        id,
        label:
          id
            .split("/")
            .pop()
            ?.replace(/\.(md|pdf)$/i, "") || id,
        type: "note" as const,
      })),
    ];

    const links: Link[] = data.links.map((l) => ({
      source: l.source,
      target: l.target,
      type: l.type,
    }));

    const simulation = forceSimulation<Node>(nodes)
      .force(
        "link",
        forceLink<Node, Link>(links)
          .id((d) => d.id)
          .distance((d) => (d.type === "hierarchy" ? 60 : 100)),
      )
      .force("charge", forceManyBody().strength(-300))
      .force(
        "center",
        forceCenter(canvas.width / dpr / 2, canvas.height / dpr / 2),
      )
      .force("collide", forceCollide().radius(50));

    simulationRef.current = simulation;

    // zoom (estilo Obsidian: scroll/pinch para zoom, arrasto no fundo para pan)
    // Na mousedown, verifica se tem um no sob o cursor — se tiver, pula o zoom
    // para nao competir com o drag manual.
    const zoomBehavior = d3Zoom<HTMLCanvasElement, unknown>()
      .scaleExtent([0.1, 10])
      .filter((event: any) => {
        if (event.type === "wheel" || event.type === "touchstart") return true;
        if (event.type === "mousedown" && event.button === 0) {
          const t = zoomTransformRef.current;
          const r = canvas.getBoundingClientRect();
          const px = t.invertX(event.clientX - r.left);
          const py = t.invertY(event.clientY - r.top);
          if (simulation.find(px, py, 20)) return false; // no sob cursor → drag manual
        }
        return true;
      })
      .on("zoom", (event) => {
        zoomTransformRef.current = event.transform;
      });
    d3Select(canvas).call(zoomBehavior as any);

    // drag manual (estilo Obsidian): clica e arrasta, no fica onde soltou
    let dragNode: Node | null = null;
    const rect = canvas.getBoundingClientRect();
    const onMouseDown = (e: MouseEvent) => {
      const t = zoomTransformRef.current;
      const px = t.invertX(e.clientX - rect.left);
      const py = t.invertY(e.clientY - rect.top);
      const node = simulation.find(px, py, 20);
      if (node) {
        dragNode = node as Node;
        dragNode.fx = dragNode.x;
        dragNode.fy = dragNode.y;
        simulation.alphaTarget(0.3).restart();
        e.preventDefault();
      }
    };
    const onMouseMove = (e: MouseEvent) => {
      if (!dragNode) return;
      const t = zoomTransformRef.current;
      dragNode.fx = t.invertX(e.clientX - rect.left);
      dragNode.fy = t.invertY(e.clientY - rect.top);
    };
    const onMouseUp = () => {
      if (dragNode) {
        simulation.alphaTarget(0);
        dragNode = null;
      }
    };
    canvas.addEventListener("mousedown", onMouseDown);
    window.addEventListener("mousemove", onMouseMove);
    window.addEventListener("mouseup", onMouseUp);

    // click → open note
    const handleClick = (event: MouseEvent) => {
      const t = zoomTransformRef.current;
      const rect = canvas.getBoundingClientRect();
      const px = t.invertX(event.clientX - rect.left);
      const py = t.invertY(event.clientY - rect.top);
      const node = simulation.find(px, py, 20);
      if (node && node.type === "note") onOpenNote(node.id);
    };
    canvas.addEventListener("click", handleClick);

    // mousemove → tooltip
    const handleMouseMove = (event: MouseEvent) => {
      const t = zoomTransformRef.current;
      const rect = canvas.getBoundingClientRect();
      const px = t.invertX(event.clientX - rect.left);
      const py = t.invertY(event.clientY - rect.top);
      const node = simulation.find(px, py, 15);
      if (node) {
        setTooltip({ x: event.clientX, y: event.clientY - 10, text: node.id });
      } else {
        setTooltip(null);
      }
    };
    canvas.addEventListener("mousemove", handleMouseMove);

    // render loop — para automaticamente quando a simulação esfria
    let renderId = 0;
    let lastAlpha = 1;
    const render = () => {
      const ctx = canvas.getContext("2d");
      if (!ctx) return;
      const t = zoomTransformRef.current;

      ctx.save();
      ctx.clearRect(0, 0, canvas.width, canvas.height);
      ctx.setTransform(dpr * t.k, 0, 0, dpr * t.k, dpr * t.x, dpr * t.y);

      // links
      links.forEach((link: any) => {
        if (!link.source.x || !link.target.x) return;
        ctx.beginPath();
        ctx.moveTo(link.source.x, link.source.y);
        ctx.lineTo(link.target.x, link.target.y);
        ctx.strokeStyle =
          link.type === "hierarchy"
            ? "rgba(167,139,250,0.4)"
            : "rgba(56,189,248,0.2)";
        ctx.setLineDash(link.type === "note" ? [2, 2] : []);
        ctx.lineWidth = 1;
        ctx.stroke();
      });
      ctx.setLineDash([]);

      // nodes
      nodes.forEach((node) => {
        const isTopic = node.type === "topic";
        const radius = isTopic ? 6 : 4;
        ctx.beginPath();
        ctx.arc(node.x!, node.y!, radius, 0, 2 * Math.PI);
        ctx.fillStyle = isTopic ? "#a78bfa" : "#38bdf8";
        ctx.fill();

        if (t.k < 0.6 && !isTopic) return;

        const fontSize = isTopic ? 12 : 10;
        ctx.font = `${isTopic ? "bold" : "normal"} ${fontSize}px "Inter", sans-serif`;
        ctx.fillStyle = "rgba(255,255,255,0.7)";
        ctx.textAlign = "center";
        ctx.textBaseline = "top";

        const maxWidth = isTopic ? 120 : 100;
        const lines = wrapText(ctx, node.label, maxWidth);
        const lh = fontSize * 1.2;
        lines.forEach((line, i) =>
          ctx.fillText(line.trim(), node.x!, node.y! + radius + 4 + i * lh),
        );
      });

      ctx.restore();

      const alpha = simulation.alpha();
      if (alpha > 0.01) {
        lastAlpha = alpha;
        renderId = requestAnimationFrame(render);
      } else if (lastAlpha > 0.01) {
        // one last frame after alpha settles
        lastAlpha = 0;
        renderId = requestAnimationFrame(render);
      }
      // else: render loop stops — CPU idle
    };

    render();
    return () => {
      cancelAnimationFrame(renderId);
      simulation.stop();
      canvas.removeEventListener("click", handleClick);
      canvas.removeEventListener("mousemove", handleMouseMove);
      canvas.removeEventListener("mousedown", onMouseDown);
      window.removeEventListener("mousemove", onMouseMove);
      window.removeEventListener("mouseup", onMouseUp);
    };
  }, [data]);

  // ── resize com throttle ──────────────────────────────────────
  useEffect(() => {
    let timer = 0;
    const handleResize = () => {
      clearTimeout(timer);
      timer = window.setTimeout(() => {
        if (!canvasRef.current) return;
        const dpr = window.devicePixelRatio || 1;
        canvasRef.current.width = window.innerWidth * dpr;
        canvasRef.current.height = window.innerHeight * dpr;
        if (simulationRef.current) {
          simulationRef.current
            .force(
              "center",
              forceCenter(window.innerWidth / 2, window.innerHeight / 2),
            )
            .alpha(0.3)
            .restart();
        }
      }, 150);
    };
    window.addEventListener("resize", handleResize);
    handleResize();
    return () => {
      clearTimeout(timer);
      window.removeEventListener("resize", handleResize);
    };
  }, []);

  // ── render ───────────────────────────────────────────────────
  return (
    <div
      className={`fixed inset-0 z-[50] bg-[#0a0a0c] overflow-hidden transition-opacity duration-300 ${visible ? "opacity-100" : "opacity-0"}`}
    >
      <div className="absolute top-6 left-6 z-10">
        <button
          onClick={onClose}
          className="px-4 py-2 bg-white/5 hover:bg-white/10 border border-white/10 rounded-xl text-zinc-400 hover:text-white transition-all flex items-center gap-2 text-sm font-medium backdrop-blur-md"
        >
          <svg
            className="w-4 h-4"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            strokeWidth="2"
          >
            <path d="M19 12H5M12 19l-7-7 7-7" />
          </svg>
          VOLTAR
        </button>
      </div>
      {loading && (
        <div className="absolute inset-0 flex items-center justify-center text-white/20 uppercase tracking-[0.2em] text-[10px] animate-pulse">
          Sincronizando Grafo Estruturado...
        </div>
      )}
      {error && (
        <div className="absolute inset-0 flex flex-col items-center justify-center gap-4">
          <p className="text-red-400 text-sm">Erro ao carregar: {error}</p>
          <button
            onClick={onClose}
            className="px-4 py-2 bg-white/10 border border-white/10 rounded-xl text-zinc-400 hover:text-white text-sm"
          >
            Fechar
          </button>
        </div>
      )}
      {tooltip && (
        <div
          className="fixed z-[60] pointer-events-none px-2 py-1 bg-zinc-900/90 border border-zinc-700/50 rounded text-[11px] text-zinc-300 whitespace-nowrap"
          style={{ left: tooltip.x + 12, top: tooltip.y - 24 }}
        >
          {tooltip.text}
        </div>
      )}
      <canvas
        ref={canvasRef}
        className="w-full h-full cursor-grab active:cursor-grabbing"
      />
    </div>
  );
}
