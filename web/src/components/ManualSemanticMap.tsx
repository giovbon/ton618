import { select as d3Select } from "d3-selection";
import { zoom as d3Zoom, zoomIdentity, type ZoomTransform } from "d3-zoom";
import { tree as d3Tree, hierarchy as d3Hierarchy, type HierarchyNode } from "d3-hierarchy";

import { useEffect, useRef, useState } from "preact/hooks";
import { wrapText } from "../utils/canvasWordWrap";

interface ManualTopic {
  id: string;
  label: string;
  level: number;
  type?: "topic" | "tag";
  has_file?: boolean; // true quando existe notes/Label.md
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

interface TreeNodeData {
  id: string;
  label: string;
  type: "topic" | "note" | "tag" | "root";
  level?: number;
  degree?: number;
  hasFile?: boolean;
  children?: TreeNodeData[];
}

// Extended type for D3 nodes with collapse support
type HierNode = HierarchyNode<TreeNodeData> & {
  _children?: HierNode[] | null;
  children?: HierNode[] | null;
  x: number;
  y: number;
};

interface ManualSemanticMapProps {
  auth: string;
  onOpenNote: (filename: string) => void;
  onClose: () => void;
}

// ─── hooks ──────────────────────────────────────────────────────────

function useSemanticMapData(auth: string, refreshKeyParam: number) {
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
  }, [auth, refreshKeyParam]);

  return { data, loading, error };
}

// ─── main component ─────────────────────────────────────────────────

export function ManualSemanticMap({
  auth,
  onOpenNote,
  onClose,
}: ManualSemanticMapProps) {
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const zoomTransformRef = useRef<ZoomTransform>(zoomIdentity);
  const [refreshKey, setRefreshKey] = useState(0);
  const [focusNode, setFocusNode] = useState<TreeNodeData | null>(null);
  const { data, loading, error } = useSemanticMapData(auth, refreshKey);

  useEffect(() => {
    if (data) console.log("DEBUG: Dados recebidos:", data);
    if (error) console.error("DEBUG: Erro nos dados:", error);
    if (loading) console.log("DEBUG: Carregando dados...");
  }, [data, loading, error]);
  const [tooltip, setTooltip] = useState<{
    x: number;
    y: number;
    text: string;
    node?: HierNode;
    pinned?: boolean;
  } | null>(null);
  const tooltipRef = useRef(tooltip);
  useEffect(() => { tooltipRef.current = tooltip; }, [tooltip]);

  // visual configurations
  const [radialSpacing, setRadialSpacing] = useState(() => {
    const saved = localStorage.getItem('manualMap_radialSpacing');
    return saved ? parseFloat(saved) : 150;
  });
  const [lineWidthMult, setLineWidthMult] = useState(() => {
    const saved = localStorage.getItem('manualMap_lineWidthMult');
    return saved ? parseFloat(saved) : 1;
  });
  const [labelScaleMult, setLabelScaleMult] = useState(() => {
    const saved = localStorage.getItem('manualMap_labelScaleMult');
    return saved ? parseFloat(saved) : 1;
  });
  const [nodeRadiusMult, setNodeRadiusMult] = useState(() => {
    const saved = localStorage.getItem('manualMap_nodeRadiusMult');
    return saved ? parseFloat(saved) : 1;
  });
  const [angleSpread, setAngleSpread] = useState(() => {
    const saved = localStorage.getItem('manualMap_angleSpread');
    return saved ? parseFloat(saved) : 1;
  });
  const [showControls, setShowControls] = useState(false);

  const visualConfig = useRef({ radialSpacing, lineWidthMult, labelScaleMult, nodeRadiusMult, angleSpread });

  useEffect(() => {
    visualConfig.current = { radialSpacing, lineWidthMult, labelScaleMult, nodeRadiusMult, angleSpread };
    localStorage.setItem('manualMap_radialSpacing', radialSpacing.toString());
    localStorage.setItem('manualMap_lineWidthMult', lineWidthMult.toString());
    localStorage.setItem('manualMap_labelScaleMult', labelScaleMult.toString());
    localStorage.setItem('manualMap_nodeRadiusMult', nodeRadiusMult.toString());
    localStorage.setItem('manualMap_angleSpread', angleSpread.toString());
    setLayoutVersion(v => v + 1);
  }, [radialSpacing, lineWidthMult, labelScaleMult, nodeRadiusMult, angleSpread]);

  const [visible, setVisible] = useState(true);
  const [hoveredNode, setHoveredNode] = useState<any | null>(null);
  const [hierarchyRoot, setHierarchyRoot] = useState<HierNode | null>(null);
  const [layoutVersion, setLayoutVersion] = useState(0);

  useEffect(() => {
    console.log("DEBUG: ManualSemanticMap montado!");
  }, []);

  // 1. Build initial hierarchy when data arrives
  useEffect(() => {
    if (!data) return;
    const rootData: TreeNodeData = { id: "root", label: "Knowledge", type: "root", children: [] };
    const topicMap = new Map<string, TreeNodeData>();
    topicMap.set("", rootData);

    data.topics.sort((a, b) => a.level - b.level).forEach(t => {
      const parts = t.id.split("/");
      const label = parts.pop()!;
      const parentId = parts.join("/");
      const node: TreeNodeData = {
        id: t.id,
        label,
        type: (t.type || "topic") as any,
        level: t.level,
        hasFile: t.has_file,
        children: []
      };
      topicMap.set(t.id, node);
      const parent = topicMap.get(parentId) || rootData;
      if (!parent.children) parent.children = [];
      parent.children.push(node);
    });

    data.links.filter(l => l.type === "note").forEach(l => {
      const parent = topicMap.get(l.target);
      if (parent) {
        if (!parent.children) parent.children = [];
        parent.children.push({
          id: l.source,
          label: l.source.split("/").pop()?.replace(/\.(md|pdf)$/i, "") || l.source,
          type: "note"
        });
      }
    });

    // Se houver um nó em foco, usamos ele como raiz real do D3
    let effectiveRootData = rootData;
    if (focusNode) {
      const found = topicMap.get(focusNode.id);
      if (found) effectiveRootData = found;
    }

    const root = d3Hierarchy(effectiveRootData) as HierNode;
    setHierarchyRoot(root);
    setLayoutVersion(v => v + 1);
  }, [data, focusNode]);

  // 2. Tree Layout and Rendering
  useEffect(() => {
    if (!hierarchyRoot || !canvasRef.current) return;
    const canvas = canvasRef.current;
    const dpr = window.devicePixelRatio || 1;
    canvas.width = window.innerWidth * dpr;
    canvas.height = window.innerHeight * dpr;

    // Apply tree layout
    const treeLayout = d3Tree<TreeNodeData>()
      .size([2 * Math.PI * visualConfig.current.angleSpread, 100])
      .separation((a, b) => (a.parent === b.parent ? 1 : 2) / (a.depth || 1));

    const root = treeLayout(hierarchyRoot) as HierNode;
    const nodes = root.descendants();
    const links = root.links();

    // Zoom Behavior
    const zoomBehavior = d3Zoom<HTMLCanvasElement, unknown>()
      .scaleExtent([0.05, 20])
      .on("zoom", (event) => {
        zoomTransformRef.current = event.transform;
      });
    d3Select(canvas).call(zoomBehavior as any);

    if (zoomTransformRef.current.k === 1 && zoomTransformRef.current.x === 0) {
      const initialTransform = zoomIdentity.translate(window.innerWidth / 2, window.innerHeight / 2).scale(0.6);
      d3Select(canvas).call(zoomBehavior.transform as any, initialTransform);
      zoomTransformRef.current = initialTransform;
    }

    // Interaction Helpers
    const findNodeAt = (mx: number, my: number) => {
      const t = zoomTransformRef.current;
      const rect = canvas.getBoundingClientRect();
      const px = (mx - rect.left - t.x) / t.k;
      const py = (my - rect.top - t.y) / t.k;

      for (const node of nodes) {
        const radius = node.depth * visualConfig.current.radialSpacing;
        const angle = node.x - Math.PI / 2;
        const nx = radius * Math.cos(angle);
        const ny = radius * Math.sin(angle);
        const dist = Math.hypot(px - nx, py - ny);
        if (dist < 25 * visualConfig.current.nodeRadiusMult) return node;
      }
      return null;
    };

    const handleClick = (event: MouseEvent) => {
      // Se clicou no tooltip, não faz nada (deixa o evento pro botão)
      if ((event.target as HTMLElement).closest('.manual-map-tooltip')) {
        return;
      }

      const node = findNodeAt(event.clientX, event.clientY) as HierNode | null;

      if (!node) {
        if (tooltipRef.current?.pinned) setTooltip(null);
        return;
      }

      // Nós de nota (ciano) → abre o arquivo imediatamente
      if (node.data.type === "note") {
        onOpenNote(node.data.id);
        return;
      }

      // Tópico abstrato (violeta) ou materializado → Mostra tooltip com opção de focar
      if (node.data.type === "topic") {
        setTooltip({
          x: event.clientX,
          y: event.clientY - 10,
          text: node.data.id,
          node,
          pinned: true
        });
      }
    };
    canvas.addEventListener("click", handleClick);

    const handleMouseMove = (event: MouseEvent) => {
      if (tooltipRef.current?.pinned) return;

      const node = findNodeAt(event.clientX, event.clientY);
      if (node) {
        setTooltip({ x: event.clientX, y: event.clientY - 10, text: node.data.id, node });
        canvas.style.cursor = "pointer";
        setHoveredNode(node);
      } else {
        setTooltip(null);
        canvas.style.cursor = "";
        setHoveredNode(null);
      }
    };
    canvas.addEventListener("mousemove", handleMouseMove);

    // Render Loop
    let renderId = 0;
    const render = () => {
      const ctx = canvas.getContext("2d");
      if (!ctx) return;
      const t = zoomTransformRef.current;
      const time = performance.now() / 1000;

      ctx.save();
      ctx.clearRect(0, 0, canvas.width, canvas.height);

      // Grid removed — keeps background clean

      ctx.setTransform(dpr * t.k, 0, 0, dpr * t.k, dpr * t.x, dpr * t.y);

      const spacing = visualConfig.current.radialSpacing;
      const lineMult = visualConfig.current.lineWidthMult;

      const branchIds = new Set<string>();
      if (hoveredNode) {
        let curr = hoveredNode;
        while (curr) { branchIds.add(curr.data.id); curr = curr.parent; }
      }

      // Links
      ctx.globalCompositeOperation = "screen";
      links.forEach((link) => {
        if (isNaN(link.source.x) || isNaN(link.target.x)) return;

        const isHighlighted = branchIds.has(link.target.data.id);
        const isStructural = link.target.depth <= 1;

        // --- LOD (Level of Detail) Logic for Links ---
        let linkOpacity = 0.18;
        if (!isStructural && !isHighlighted) {
          linkOpacity = Math.max(0, Math.min(0.18, (t.k - 0.25) * 0.36));
        }
        if (isHighlighted) linkOpacity = 0.8;
        if (linkOpacity <= 0 && !isHighlighted) return;

        const sourceAngle = link.source.x - Math.PI / 2;
        const sourceRadius = link.source.depth * spacing;
        const targetAngle = link.target.x - Math.PI / 2;
        const targetRadius = link.target.depth * spacing;

        const sx = sourceRadius * Math.cos(sourceAngle);
        const sy = sourceRadius * Math.sin(sourceAngle);
        const tx = targetRadius * Math.cos(targetAngle);
        const ty = targetRadius * Math.sin(targetAngle);

        ctx.beginPath();
        ctx.moveTo(sx, sy);
        const midRadius = (sourceRadius + targetRadius) / 2;
        const cp1x = midRadius * Math.cos(sourceAngle);
        const cp1y = midRadius * Math.sin(sourceAngle);
        const cp2x = midRadius * Math.cos(targetAngle);
        const cp2y = midRadius * Math.sin(targetAngle);
        ctx.bezierCurveTo(cp1x, cp1y, cp2x, cp2y, tx, ty);

        if (isHighlighted) {
          ctx.strokeStyle = "rgba(167,139,250,0.8)";
          ctx.lineWidth = (2.5 * lineMult) / t.k;
          ctx.shadowBlur = 15 / t.k;
          ctx.shadowColor = "#a78bfa";
        } else {
          ctx.strokeStyle = `rgba(167,139,250,${linkOpacity})`;
          ctx.lineWidth = (1 * lineMult) / t.k;
          ctx.shadowBlur = 0;
        }
        ctx.stroke();
      });
      ctx.shadowBlur = 0;
      ctx.globalCompositeOperation = "source-over";

      // Nodes
      nodes.forEach((node) => {
        if (isNaN(node.x)) return;
        if (node.data.id === "root" && node.depth === 0) {
          ctx.beginPath();
          ctx.arc(0, 0, 10, 0, 2 * Math.PI);
          ctx.fillStyle = "#fff";
          ctx.shadowBlur = 12 / t.k;
          ctx.shadowColor = "#a78bfa";
          ctx.fill();
          ctx.shadowBlur = 0;
          return;
        }

        const isStructural = node.depth <= 1;
        const isInBranch = branchIds.has(node.data.id);
        const isHovered = hoveredNode?.data.id === node.data.id;
        const isHighlighted = isHovered || isInBranch;

        let nodeOpacity = 1;
        if (!isStructural && !isHighlighted) {
          nodeOpacity = Math.max(0, Math.min(1, (t.k - 0.2) * 2));
        }
        if (nodeOpacity <= 0) return;

        const angle = node.x - Math.PI / 2;
        const radius = node.depth * spacing;
        const nx = radius * Math.cos(angle);
        const ny = radius * Math.sin(angle);

        const isTopic = node.data.type === "topic";
        const isTag = node.data.type === "tag";
        const hasFile = !!node.data.hasFile;
        const isCollapsed = (node as any)._children?.length > 0;

        const weight = 1 + (node.children?.length || (node as any)._children?.length || 0);
        const scale = Math.sqrt(weight);
        const baseRadius = isTopic ? 5 : isTag ? 4 : 3;
        const r = (baseRadius * scale * visualConfig.current.nodeRadiusMult) * (isHovered ? 1.3 : 1);

        const color = isTopic
          ? hasFile ? "#34d399" : "#a78bfa"
          : isTag ? "#f472b6"
            : "#38bdf8";

        ctx.globalAlpha = nodeOpacity;

        // Core Node
        ctx.beginPath();
        ctx.arc(nx, ny, r, 0, 2 * Math.PI);
        ctx.fillStyle = isHovered ? "#fff" : color;

        if (isCollapsed) {
          ctx.lineWidth = 3 / t.k;
          ctx.strokeStyle = "#fff";
          ctx.stroke();
        }

        if (isInBranch) {
          ctx.shadowBlur = 15 / t.k;
          ctx.shadowColor = color;
        }
        ctx.fill();
        ctx.shadowBlur = 0;

        // Anel extra para tópicos com arquivo real (indica que são "materializados")
        if (hasFile && isTopic) {
          ctx.beginPath();
          ctx.arc(nx, ny, r + 4 / t.k, 0, 2 * Math.PI);
          ctx.strokeStyle = isHovered ? "#fff" : "#34d399";
          ctx.lineWidth = 1.5 / t.k;
          ctx.globalAlpha = 0.45;
          ctx.shadowBlur = 0;
          ctx.stroke();
          ctx.globalAlpha = 1;
        }

        // Labels
        // Notas: label só aparece no hover — libera espaço visual
        if (node.data.type === "note" && !isHovered) {
          ctx.globalAlpha = 1;
          return;
        }
        // LOD: esconde labels não-estruturais no zoom out
        if (t.k < 0.55 && !isStructural && !isHovered) {
          ctx.globalAlpha = 1;
          return;
        }

        const labelScale = visualConfig.current.labelScaleMult;
        const fontSize = (isTopic ? 13 : isTag ? 12 : 11) * labelScale * Math.min(1.4, Math.max(0.8, visualConfig.current.nodeRadiusMult));
        ctx.font = `${isStructural || isHovered ? "bold" : "normal"} ${fontSize}px "Inter", sans-serif`;

        ctx.shadowBlur = 4 / t.k;
        ctx.shadowColor = "rgba(0,0,0,0.8)";

        // Cor do texto com opacidade do LOD
        ctx.fillStyle = isHovered || isInBranch ? "#fff" : `rgba(255,255,255,${0.75 * nodeOpacity})`;

        ctx.textAlign = "center";
        ctx.textBaseline = "top";

        // Tópicos com arquivo mantêm o label original (diferenciação apenas pela cor/anel)
        const label = isCollapsed ? `[+] ${node.data.label}` : node.data.label;
        const maxWidth = isTopic ? 140 : isTag ? 120 : 110;
        const lines = wrapText(ctx, label, maxWidth);
        const lh = fontSize * 1.3;
        lines.forEach((line, i) =>
          ctx.fillText(line.trim(), nx, ny + r + 6 + i * lh),
        );
        ctx.shadowBlur = 0;
        ctx.globalAlpha = 1; // Reset alpha
      });

      ctx.restore();
      renderId = requestAnimationFrame(render);
    };

    render();
    return () => {
      cancelAnimationFrame(renderId);
      canvas.removeEventListener("click", handleClick);
      canvas.removeEventListener("mousemove", handleMouseMove);
    };
  }, [hierarchyRoot, layoutVersion, radialSpacing, lineWidthMult, labelScaleMult, nodeRadiusMult, angleSpread, hoveredNode]);

  // ── resize handler ──────────────────────────────────────────
  useEffect(() => {
    const handleResize = () => {
      if (!canvasRef.current) return;
      const dpr = window.devicePixelRatio || 1;
      canvasRef.current.width = window.innerWidth * dpr;
      canvasRef.current.height = window.innerHeight * dpr;
    };
    window.addEventListener("resize", handleResize);
    return () => window.removeEventListener("resize", handleResize);
  }, []);

  return (
    <div
      className={`fixed inset-0 z-[1000] bg-[#0a0a0c] overflow-hidden`}
    >
      {/* Breadcrumbs / Focus Path */}
      {focusNode && (
        <div className="absolute top-6 left-1/2 -translate-x-1/2 z-30 flex items-center gap-1.5 px-4 py-2 bg-zinc-900/80 backdrop-blur-md border border-sky-500/30 rounded-full shadow-2xl">
          <button
            onClick={() => setFocusNode(null)}
            className="text-[10px] font-black uppercase tracking-widest text-sky-400 hover:text-sky-300 transition-colors px-1"
          >
            Mundo
          </button>

          {focusNode.id.split('/').map((part, i, arr) => (
            <div key={i} className="flex items-center gap-1.5">
              <span className="text-zinc-600 text-[10px]">/</span>
              <button
                onClick={() => {
                  const newPath = arr.slice(0, i + 1).join('/');
                  if (newPath === focusNode.id) return;
                  setFocusNode({ ...focusNode, id: newPath, label: part });
                }}
                className={`text-[11px] font-bold transition-colors px-1 ${i === arr.length - 1 ? 'text-zinc-100 cursor-default' : 'text-zinc-400 hover:text-sky-400'}`}
              >
                {part}
              </button>
            </div>
          ))}

          <button
            onClick={() => setFocusNode(null)}
            className="ml-2 p-1 hover:bg-white/10 rounded-full transition-all"
            title="Sair do Foco"
          >
            <svg className="w-3 h-3 text-zinc-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="3" d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>
      )}

      <div className="absolute top-6 left-6 z-10 flex gap-2">
        <button
          onClick={onClose}
          className="px-4 py-2 bg-white/5 hover:bg-white/10 border border-white/10 rounded-xl text-zinc-400 hover:text-white transition-all flex items-center gap-2 text-sm font-medium backdrop-blur-md"
        >
          <svg className="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
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
            Knowledge Galaxy
            <button onClick={() => {
              setRadialSpacing(150);
              setLineWidthMult(1);
              setLabelScaleMult(1);
              setNodeRadiusMult(1);
              setAngleSpread(1);
            }} className="text-[9px] text-sky-400 hover:text-sky-300 px-2 py-0.5 bg-sky-500/10 rounded">RESET</button>
          </h3>
          <div className="flex flex-col gap-2">
            <div className="flex justify-between items-center text-[10px] text-zinc-400 font-medium">
              <span>Expansão Radial</span>
              <span className="text-sky-400">{radialSpacing.toFixed(0)}px</span>
            </div>
            <input type="range" min="80" max="400" step="10" value={radialSpacing} onChange={e => setRadialSpacing(parseFloat((e.target as HTMLInputElement).value))} className="w-full accent-sky-500" />
          </div>
          <div className="flex flex-col gap-2">
            <div className="flex justify-between items-center text-[10px] text-zinc-400 font-medium">
              <span>Espessura das Linhas</span>
              <span className="text-sky-400">{lineWidthMult.toFixed(1)}x</span>
            </div>
            <input type="range" min="0.5" max="5" step="0.5" value={lineWidthMult} onChange={e => setLineWidthMult(parseFloat((e.target as HTMLInputElement).value))} className="w-full accent-sky-500" />
          </div>
          <div className="flex flex-col gap-2">
            <div className="flex justify-between items-center text-[10px] text-zinc-400 font-medium">
              <span>Tamanho do Texto</span>
              <span className="text-sky-400">{labelScaleMult.toFixed(1)}x</span>
            </div>
            <input type="range" min="0.5" max="2.5" step="0.1" value={labelScaleMult} onChange={e => setLabelScaleMult(parseFloat((e.target as HTMLInputElement).value))} className="w-full accent-sky-500" />
          </div>
          <div className="flex flex-col gap-2">
            <div className="flex justify-between items-center text-[10px] text-zinc-400 font-medium">
              <span>Tamanho dos Nós</span>
              <span className="text-sky-400">{nodeRadiusMult.toFixed(1)}x</span>
            </div>
            <input type="range" min="0.5" max="3" step="0.1" value={nodeRadiusMult} onChange={e => setNodeRadiusMult(parseFloat((e.target as HTMLInputElement).value))} className="w-full accent-sky-500" />
          </div>
          <div className="flex flex-col gap-2">
            <div className="flex justify-between items-center text-[10px] text-zinc-400 font-medium">
              <span>Ângulo de Abertura</span>
              <span className="text-sky-400">{(angleSpread * 360).toFixed(0)}°</span>
            </div>
            <input type="range" min="0.1" max="1" step="0.05" value={angleSpread} onChange={e => setAngleSpread(parseFloat((e.target as HTMLInputElement).value))} className="w-full accent-sky-500" />
          </div>
        </div>
      )}

      {loading && (
        <div className="absolute inset-0 flex items-center justify-center text-white/20 uppercase tracking-[0.2em] text-[10px] animate-pulse">
          Sincronizando Galáxia de Conhecimento...
        </div>
      )}
      {error && (
        <div className="absolute inset-0 flex flex-col items-center justify-center gap-4">
          <p className="text-red-400 text-sm">Erro ao carregar: {error}</p>
          <button onClick={onClose} className="px-4 py-2 bg-white/10 border border-white/10 rounded-xl text-zinc-400 hover:text-white text-sm">Fechar</button>
        </div>
      )}
      {tooltip && (
        <div
          className={`manual-map-tooltip fixed z-[60] p-3 bg-zinc-900/95 backdrop-blur-md border border-zinc-700/50 rounded-2xl shadow-2xl flex flex-col gap-2 min-w-[160px] ${tooltip.pinned ? 'ring-2 ring-violet-500/30' : 'pointer-events-none'}`}
          style={{ left: tooltip.x + 12, top: tooltip.y - 24 }}
        >
          <div className="flex justify-between items-center border-b border-zinc-800 pb-1 mb-0.5">
            <div className="text-[10px] font-bold text-zinc-500 uppercase tracking-widest">
              {tooltip.node?.data.type === 'note' ? 'Arquivo' : 'Tópico'}
            </div>
            {tooltip.pinned && (
              <button
                onClick={(e) => { e.stopPropagation(); setTooltip(null); }}
                className="text-zinc-500 hover:text-white"
              >
                <svg className="w-3 h-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="3">
                  <path d="M18 6L6 18M6 6l12 12" />
                </svg>
              </button>
            )}
          </div>
          <div className="text-[12px] text-zinc-100 font-bold whitespace-nowrap mb-1">
            {tooltip.text}
          </div>

          <div className="flex items-center gap-2">
            {/* Botão de Focar (Sempre disponível para tópicos) */}
            {tooltip.node?.data.type === 'topic' && (
              <button
                onClick={(e) => {
                  e.stopPropagation();
                  setFocusNode(tooltip.node!.data);
                  setTooltip(null);
                }}
                className="flex-1 px-2 py-1.5 bg-sky-500/20 hover:bg-sky-500/40 border border-sky-500/30 rounded-lg text-[10px] text-sky-300 font-bold uppercase tracking-tighter transition-all flex items-center justify-center gap-1"
              >
                <svg className="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2.5" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
                </svg>
                Focar
              </button>
            )}

            {/* Botão de Abrir Nota (Se já materializado) */}
            {tooltip.node?.data.type === 'topic' && tooltip.node.data.hasFile && (
              <button
                onClick={(e) => {
                  e.stopPropagation();
                  onOpenNote(`notes/${tooltip.node!.data.label}.md`);
                  setTooltip(null);
                }}
                className="flex-1 px-2 py-1.5 bg-emerald-500/20 hover:bg-emerald-500/40 border border-emerald-500/30 rounded-lg text-[10px] text-emerald-300 font-bold uppercase tracking-tighter transition-all flex items-center justify-center gap-1"
              >
                <svg className="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2.5" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2.5" d="M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z" />
                </svg>
                Abrir
              </button>
            )}
          </div>

          {/* Botão de Materializar (Se ainda não materializado) */}
          {tooltip.pinned && tooltip.node?.data.type === 'topic' && !tooltip.node.data.hasFile && (
            <button
              onClick={async (e) => {
                e.stopPropagation();
                const label = tooltip.node!.data.label;
                const filename = `notes/${label}.md`;
                const content = "";

                try {
                  const res = await fetch(`/api/file?name=${encodeURIComponent(filename)}`, {
                    method: "POST",
                    headers: {
                      "Authorization": auth,
                      "Content-Type": "application/json"
                    },
                    body: JSON.stringify({ name: filename, content })
                  });

                  if (res.ok) {
                    window.dispatchEvent(new CustomEvent("graph-updated"));
                    setRefreshKey(prev => prev + 1);
                    setTooltip(null);
                    setTimeout(() => onOpenNote(filename), 100);
                  }
                } catch (err) {
                  console.error("Erro ao materializar nota:", err);
                }
              }}
              className="mt-1 px-2 py-1.5 bg-violet-600/20 hover:bg-violet-600/40 border border-violet-500/30 rounded-lg text-[10px] text-violet-300 font-bold uppercase tracking-tighter transition-all text-center flex items-center justify-center gap-1"
            >
              <svg className="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2.5" d="M12 4v16m8-8H4" />
              </svg>
              Materializar Nota
            </button>
          )}
        </div>
      )}
      <canvas ref={canvasRef} className="w-full h-full cursor-grab active:cursor-grabbing" />
    </div>
  );
}
