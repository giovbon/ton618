import { select as d3Select } from "d3-selection";
import { zoom as d3Zoom, zoomIdentity, type ZoomTransform } from "d3-zoom";
import {
  forceSimulation,
  forceLink,
  forceManyBody,
  forceCenter,
  forceCollide,
  forceX,
  forceY,
  type SimulationNodeDatum,
  type SimulationLinkDatum,
} from "d3-force";

import { useEffect, useRef, useState } from "preact/hooks";
import { wrapText } from "../utils/canvasWordWrap";

interface ManualTopic {
  id: string;
  label: string;
  level: number;
  type?: "topic" | "tag";
}

interface ManualLink {
  source: string;
  target: string;
  type: "hierarchy" | "note" | "tag";
}

interface ManualMapData {
  topics: ManualTopic[];
  links: ManualLink[];
}

interface Node extends SimulationNodeDatum {
  id: string;
  label: string;
  type: "topic" | "note" | "tag";
  level?: number;
}

interface Link extends SimulationLinkDatum<Node> {
  type: "hierarchy" | "note" | "tag";
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
  const [refreshKey, setRefreshKey] = useState(0);

  useEffect(() => {
    const handleUpdate = () => setRefreshKey((k) => k + 1);
    window.addEventListener("graph-updated", handleUpdate);
    return () => window.removeEventListener("graph-updated", handleUpdate);
  }, []);

  useEffect(() => {
    const controller = new AbortController();
    setLoading(true);
    setError(null);
    fetch("/api/graph/manual-map", {
      headers: { Authorization: auth },
      signal: controller.signal,
      cache: "no-store",
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
  }, [auth, refreshKey]);

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

  // visual configurations
  const [linkDistanceMult, setLinkDistanceMult] = useState(() => {
    const saved = localStorage.getItem('manualMap_linkDistanceMult');
    return saved ? parseFloat(saved) : 1;
  });
  const [lineWidthMult, setLineWidthMult] = useState(() => {
    const saved = localStorage.getItem('manualMap_lineWidthMult');
    return saved ? parseFloat(saved) : 1;
  });
  const [nodeRadiusMult, setNodeRadiusMult] = useState(() => {
    const saved = localStorage.getItem('manualMap_nodeRadiusMult');
    return saved ? parseFloat(saved) : 1;
  });
  const [chargeStrengthMult, setChargeStrengthMult] = useState(() => {
    const saved = localStorage.getItem('manualMap_chargeStrengthMult');
    return saved ? parseFloat(saved) : 1;
  });
  const [gravityMult, setGravityMult] = useState(() => {
    const saved = localStorage.getItem('manualMap_gravityMult');
    return saved ? parseFloat(saved) : 1;
  });
  const [showControls, setShowControls] = useState(false);

  const visualConfig = useRef({ linkDistanceMult, lineWidthMult, nodeRadiusMult, chargeStrengthMult, gravityMult });
  useEffect(() => {
    visualConfig.current = { linkDistanceMult, lineWidthMult, nodeRadiusMult, chargeStrengthMult, gravityMult };
    localStorage.setItem('manualMap_linkDistanceMult', linkDistanceMult.toString());
    localStorage.setItem('manualMap_lineWidthMult', lineWidthMult.toString());
    localStorage.setItem('manualMap_nodeRadiusMult', nodeRadiusMult.toString());
    localStorage.setItem('manualMap_chargeStrengthMult', chargeStrengthMult.toString());
    localStorage.setItem('manualMap_gravityMult', gravityMult.toString());
  }, [linkDistanceMult, lineWidthMult, nodeRadiusMult, chargeStrengthMult, gravityMult]);

  // fade-in entry animation
  useEffect(() => {
    requestAnimationFrame(() => setVisible(true));
  }, []);

  // update force distance when linkDistanceMult changes
  useEffect(() => {
    if (simulationRef.current) {
      const linkForce = simulationRef.current.force("link");
      if (linkForce) {
        linkForce.distance((d: any) =>
          d.type === "hierarchy" ? 60 * linkDistanceMult : 100 * linkDistanceMult
        );
        simulationRef.current.alpha(0.3).restart();
      }
    }
  }, [linkDistanceMult]);

  // update charge/repulsion strength when chargeStrengthMult changes
  useEffect(() => {
    if (simulationRef.current) {
      const chargeForce = simulationRef.current.force("charge");
      const collideForce = simulationRef.current.force("collide");
      if (chargeForce) chargeForce.strength(-300 * chargeStrengthMult);
      if (collideForce) collideForce.radius(50 * chargeStrengthMult);
      simulationRef.current.alpha(0.3).restart();
    }
  }, [chargeStrengthMult]);

  // update gravity/centering force when gravityMult changes
  useEffect(() => {
    if (simulationRef.current && canvasRef.current) {
      const dpr = window.devicePixelRatio || 1;
      const xForce = simulationRef.current.force("x");
      const yForce = simulationRef.current.force("y");
      if (xForce) xForce.strength(0.01 * gravityMult);
      if (yForce) yForce.strength(0.01 * gravityMult);
      simulationRef.current.alpha(0.3).restart();
    }
  }, [gravityMult]);

  // ── initialize / update force graph ──────────────────────────
  useEffect(() => {
    if (!data || !canvasRef.current) return;
    const canvas = canvasRef.current;
    const dpr = window.devicePixelRatio || 1;
    canvas.width = window.innerWidth * dpr;
    canvas.height = window.innerHeight * dpr;

    const noteIds = new Set<string>();
    data.links.forEach((l) => {
      if (l.type === "note" || l.type === "tag") noteIds.add(l.source);
    });

    const nodes: Node[] = [
      ...data.topics.map((t) => ({
        id: t.id,
        label: t.label,
        type: (t.type || "topic") as "topic" | "tag",
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
          .distance((d) =>
            d.type === "hierarchy"
              ? 60 * visualConfig.current.linkDistanceMult
              : 100 * visualConfig.current.linkDistanceMult
          ),
      )
      .force("charge", forceManyBody().strength(-300 * visualConfig.current.chargeStrengthMult))
      .force(
        "center",
        forceCenter(canvas.width / dpr / 2, canvas.height / dpr / 2),
      )
      .force("x", forceX(canvas.width / dpr / 2).strength(0.01 * visualConfig.current.gravityMult))
      .force("y", forceY(canvas.height / dpr / 2).strength(0.01 * visualConfig.current.gravityMult))
      .force("collide", forceCollide().radius(50 * visualConfig.current.chargeStrengthMult));

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
          if (simulation.find(px, py, 20 * visualConfig.current.nodeRadiusMult)) return false; // no sob cursor → drag manual
        }
        return true;
      })
      .on("zoom", (event) => {
        zoomTransformRef.current = event.transform;
      });
    d3Select(canvas).call(zoomBehavior as any);

    // drag manual (estilo Obsidian): clica e arrasta, no fica onde soltou
    let dragNode: Node | null = null;
    let mouseDownPos = { x: 0, y: 0 };
    let mouseDownTime = 0;
    const rect = canvas.getBoundingClientRect();
    const onMouseDown = (e: MouseEvent) => {
      mouseDownPos = { x: e.clientX, y: e.clientY };
      mouseDownTime = Date.now();
      const t = zoomTransformRef.current;
      const px = t.invertX(e.clientX - rect.left);
      const py = t.invertY(e.clientY - rect.top);
      const node = simulation.find(px, py, 20 * visualConfig.current.nodeRadiusMult);
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
      const dist = Math.hypot(event.clientX - mouseDownPos.x, event.clientY - mouseDownPos.y);
      const timeElapsed = Date.now() - mouseDownTime;
      if (dist > 5 || timeElapsed > 300) return;

      const t = zoomTransformRef.current;
      const rect = canvas.getBoundingClientRect();
      const px = t.invertX(event.clientX - rect.left);
      const py = t.invertY(event.clientY - rect.top);
      const node = simulation.find(px, py, 20 * visualConfig.current.nodeRadiusMult);
      if (node && node.type === "note") onOpenNote(node.id);
    };
    canvas.addEventListener("click", handleClick);

    // mousemove → tooltip
    const handleMouseMove = (event: MouseEvent) => {
      const t = zoomTransformRef.current;
      const rect = canvas.getBoundingClientRect();
      const px = t.invertX(event.clientX - rect.left);
      const py = t.invertY(event.clientY - rect.top);
      const node = simulation.find(px, py, 15 * visualConfig.current.nodeRadiusMult);
      if (node) {
        setTooltip({ x: event.clientX, y: event.clientY - 10, text: node.id });
        canvas.style.cursor = "pointer";
      } else {
        setTooltip(null);
        canvas.style.cursor = "";
      }
    };
    canvas.addEventListener("mousemove", handleMouseMove);

    // render loop — para automaticamente quando a simulação esfria
    let renderId = 0;
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
        
        if (link.type === "hierarchy") {
          ctx.strokeStyle = "rgba(167,139,250,0.4)";
          ctx.setLineDash([]);
        } else if (link.type === "tag") {
          ctx.strokeStyle = "rgba(244,114,182,0.3)";
          ctx.setLineDash([2, 2]);
        } else {
          ctx.strokeStyle = "rgba(56,189,248,0.2)";
          ctx.setLineDash([2, 2]);
        }
        ctx.lineWidth = 1 * visualConfig.current.lineWidthMult;
        ctx.stroke();
      });
      ctx.setLineDash([]);

      // nodes
      nodes.forEach((node) => {
        const isTopic = node.type === "topic";
        const isTag = node.type === "tag";
        const isStructural = isTopic || isTag;
        const radius = (isTopic ? 6 : isTag ? 5 : 4) * visualConfig.current.nodeRadiusMult;
        
        ctx.beginPath();
        ctx.arc(node.x!, node.y!, radius, 0, 2 * Math.PI);
        ctx.fillStyle = isTopic ? "#a78bfa" : isTag ? "#f472b6" : "#38bdf8";
        ctx.fill();

        if (t.k < 0.6 && !isStructural) return;

        const fontSize = (isTopic ? 12 : isTag ? 11 : 10) * Math.min(1.5, Math.max(0.8, visualConfig.current.nodeRadiusMult));
        ctx.font = `${isStructural ? "bold" : "normal"} ${fontSize}px "Inter", sans-serif`;
        ctx.fillStyle = "rgba(255,255,255,0.7)";
        ctx.textAlign = "center";
        ctx.textBaseline = "top";

        const maxWidth = isTopic ? 120 : isTag ? 110 : 100;
        const lines = wrapText(ctx, node.label, maxWidth);
        const lh = fontSize * 1.2;
        lines.forEach((line, i) =>
          ctx.fillText(line.trim(), node.x!, node.y! + radius + 4 + i * lh),
        );
      });

      ctx.restore();

      renderId = requestAnimationFrame(render);
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
          const centerX = window.innerWidth / 2;
          const centerY = window.innerHeight / 2;
          simulationRef.current
            .force("center", forceCenter(centerX, centerY))
            .force("x", simulationRef.current.force("x")?.x(centerX))
            .force("y", simulationRef.current.force("y")?.y(centerY))
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
      <div className="absolute top-6 left-6 z-10 flex gap-2">
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
        <button
          onClick={() => setShowControls(!showControls)}
          className={`px-3 py-2 border rounded-xl transition-all flex items-center gap-2 text-sm font-medium backdrop-blur-md ${showControls ? 'bg-sky-500/20 text-sky-400 border-sky-500/50' : 'bg-white/5 hover:bg-white/10 text-zinc-400 hover:text-white border-white/10'}`}
        >
          <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M12 6V4m0 2a2 2 0 100 4m0-4a2 2 0 110 4m-6 8a2 2 0 100-4m0 4a2 2 0 110-4m0 4v2m0-6V4m6 6v10m6-2a2 2 0 100-4m0 4a2 2 0 110-4m0 4v2m0-6V4" />
          </svg>
          CONTROLES
        </button>
      </div>

      {showControls && (
        <div className="absolute top-20 left-6 z-20 bg-zinc-900/90 backdrop-blur-xl border border-zinc-700/50 p-5 rounded-2xl flex flex-col gap-5 min-w-[240px] shadow-2xl animate-in slide-in-from-top-2">
          <h3 className="text-[11px] font-bold text-zinc-300 uppercase tracking-widest border-b border-zinc-700/50 pb-2 flex items-center justify-between">
            Visualização
            <button onClick={() => { 
              setLinkDistanceMult(1); 
              setLineWidthMult(1); 
              setNodeRadiusMult(1); 
              setChargeStrengthMult(1); 
              setGravityMult(1);
            }} className="text-[9px] text-sky-400 hover:text-sky-300 px-2 py-0.5 bg-sky-500/10 rounded">RESET</button>
          </h3>
          <div className="flex flex-col gap-2">
            <div className="flex justify-between items-center text-[10px] text-zinc-400 font-medium">
              <span>Espaçamento (Proximidade)</span>
              <span className="text-sky-400">{linkDistanceMult.toFixed(1)}x</span>
            </div>
            <input type="range" min="0.2" max="3" step="0.1" value={linkDistanceMult} onChange={e => setLinkDistanceMult(parseFloat(e.target.value))} className="w-full accent-sky-500" />
          </div>
          <div className="flex flex-col gap-2">
            <div className="flex justify-between items-center text-[10px] text-zinc-400 font-medium">
              <span>Espessura das Linhas</span>
              <span className="text-sky-400">{lineWidthMult.toFixed(1)}x</span>
            </div>
            <input type="range" min="0.1" max="5" step="0.1" value={lineWidthMult} onChange={e => setLineWidthMult(parseFloat(e.target.value))} className="w-full accent-sky-500" />
          </div>
          <div className="flex flex-col gap-2">
            <div className="flex justify-between items-center text-[10px] text-zinc-400 font-medium">
              <span>Tamanho dos Nós</span>
              <span className="text-sky-400">{nodeRadiusMult.toFixed(1)}x</span>
            </div>
            <input type="range" min="0.2" max="4" step="0.1" value={nodeRadiusMult} onChange={e => setNodeRadiusMult(parseFloat(e.target.value))} className="w-full accent-sky-500" />
          </div>
          <div className="flex flex-col gap-2">
            <div className="flex justify-between items-center text-[10px] text-zinc-400 font-medium">
              <span>Expansão (Repulsão)</span>
              <span className="text-sky-400">{chargeStrengthMult.toFixed(1)}x</span>
            </div>
            <input type="range" min="0.1" max="5" step="0.1" value={chargeStrengthMult} onChange={e => setChargeStrengthMult(parseFloat(e.target.value))} className="w-full accent-sky-500" />
          </div>
          <div className="flex flex-col gap-2">
            <div className="flex justify-between items-center text-[10px] text-zinc-400 font-medium">
              <span>Agrupamento (Gravidade)</span>
              <span className="text-sky-400">{gravityMult.toFixed(1)}x</span>
            </div>
            <input type="range" min="0" max="5" step="0.1" value={gravityMult} onChange={e => setGravityMult(parseFloat(e.target.value))} className="w-full accent-sky-500" />
          </div>
        </div>
      )}

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
