import { select } from 'd3-selection';
import { zoom as d3zoom, type ZoomTransform, zoomIdentity } from 'd3-zoom';
import { useEffect, useRef, useState } from 'preact/hooks';

import KnowledgeWorker from '../hooks/knowledge.worker?worker';

interface Point {
  id: string;
  title: string;
  x: number;
  y: number;
  cluster_id: number;
}

interface Cluster {
  id: number;
  label: string;
  keywords: string[];
  size: number;
  x: number;
  y: number;
}

interface ReindexStatus {
  is_reindexing: boolean;
  total: number;
  processed: number;
  percent: number;
}

interface KnowledgeMapProps {
  auth: string;
  onOpenNote: (id: string) => void;
  onClose: () => void;
}

function noteTitle(id: string): string {
  return id.split('/').pop()?.replace(/\.md$/i, '') || id;
}

const CLUSTER_COLORS = [
  '#38bdf8',
  '#10b981',
  '#f59e0b',
  '#ef4444',
  '#8b5cf6',
  '#ec4899',
  '#14b8a6',
  '#6366f1',
];

export function KnowledgeMap({ auth, onOpenNote, onClose }: KnowledgeMapProps) {
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const zoomRef = useRef<any>(null);
  const transformRef = useRef<ZoomTransform>(zoomIdentity);
  const [data, setData] = useState<{ notes: Point[]; clusters: Cluster[] } | null>(null);
  const [loading, setLoading] = useState(true);
  const [hoveredNote, setHoveredNote] = useState<Point | null>(null);
  const [hoveredCluster, setHoveredCluster] = useState<Cluster | null>(null);
  const [tooltipPos, setTooltipPos] = useState<{ x: number; y: number } | null>(null);
  const [reindexing, setReindexing] = useState<'idle' | 'running' | 'done' | 'error'>('idle');
  const [reindexStatus, setReindexStatus] = useState<ReindexStatus | null>(null);

  // Query-point state
  interface QueryPoint { x: number; y: number; nearest_notes: { id: string; x: number; y: number; dist: number }[]; query: string; }
  const [queryPoint, setQueryPoint] = useState<QueryPoint | null>(null);
  const [showQueryModal, setShowQueryModal] = useState(false);
  const [queryInput, setQueryInput] = useState('');
  const [queryLoading, setQueryLoading] = useState(false);
  const [queryError, setQueryError] = useState<string | null>(null);
  const queryPointRef = useRef<QueryPoint | null>(null);
  queryPointRef.current = queryPoint;

  // Worker state
  const workerRef = useRef<Worker | null>(null);
  const [workerData, setWorkerData] = useState<{
    grid: Map<string, Point[]>;
    cellSize: number;
    voronoiPaths: Path2D[];
  }>({
    grid: new Map(),
    cellSize: 10,
    voronoiPaths: [],
  });

  // 1. Initialize Worker
  useEffect(() => {
    workerRef.current = new KnowledgeWorker();
    workerRef.current.onmessage = (e) => {
      const { grid, cellSize, voronoiPaths } = e.data;
      setWorkerData({
        grid,
        cellSize,
        voronoiPaths: voronoiPaths.map((p: string) => (p ? new Path2D(p) : null)).filter(Boolean),
      });
    };
    return () => workerRef.current?.terminate();
  }, []);

  // 2. Load Data and offload to worker
  useEffect(() => {
    fetch('/api/graph/map', { headers: { Authorization: auth } })
      .then((res) => res.json())
      .then((d) => {
        setData(d);
        if (workerRef.current) {
          workerRef.current.postMessage({ notes: d.notes, clusters: d.clusters });
        }
      })
      .finally(() => setLoading(false));
  }, [auth]);

  // 3. Canvas Resizing
  useEffect(() => {
    const handleResize = () => {
      if (canvasRef.current) {
        canvasRef.current.width = window.innerWidth;
        canvasRef.current.height = window.innerHeight;
      }
    };
    window.addEventListener('resize', handleResize);
    handleResize();
    return () => window.removeEventListener('resize', handleResize);
  }, []);

  // 4. Render Loop (requestAnimationFrame)
  useEffect(() => {
    let frameId: number;
    const render = () => {
      const canvas = canvasRef.current;
      const ctx = canvas?.getContext('2d');
      if (!ctx || !canvas || !data || data.notes.length === 0) {
        frameId = requestAnimationFrame(render);
        return;
      }

      const transform = transformRef.current;
      ctx.clearRect(0, 0, canvas.width, canvas.height);
      ctx.fillStyle = '#020617';
      ctx.fillRect(0, 0, canvas.width, canvas.height);

      ctx.save();
      ctx.translate(transform.x, transform.y);
      ctx.scale(transform.k, transform.k);

      // Render Territories (Energy Field Voronoi)
      if (workerData.voronoiPaths.length > 0) {
        const time = performance.now() / 1000;
        ctx.globalCompositeOperation = 'source-over';

        workerData.voronoiPaths.forEach((path, i) => {
          const color = CLUSTER_COLORS[i % CLUSTER_COLORS.length];
          const clusterId = data.clusters[i]?.id;
          const isHovered = hoveredCluster?.id === clusterId;

          const pulse = Math.sin(time * 1.2 + i * 0.5) * 0.5 + 0.5;
          const flowOffset = -time * 25;

          ctx.save();

          // 1. Aura de Brilho (Glow Externo)
          ctx.strokeStyle = color;
          ctx.lineWidth = (isHovered ? 12 : 8) / transform.k;
          ctx.globalAlpha = (isHovered ? 0.12 : 0.05) * (0.8 + pulse * 0.2);
          ctx.stroke(path);

          // 2. Linha de Fronteira (Core)
          ctx.lineWidth = (isHovered ? 2 : 1) / transform.k;
          ctx.globalAlpha = (isHovered ? 0.5 : 0.25);
          ctx.stroke(path);

          // 3. Pulso de Energia (Arestas vivas)
          ctx.setLineDash([20 / transform.k, 60 / transform.k]);
          ctx.lineDashOffset = flowOffset / transform.k;
          ctx.strokeStyle = '#fff'; // Pulso branco para contraste
          ctx.lineWidth = (isHovered ? 1.5 : 0.8) / transform.k;
          ctx.globalAlpha = (isHovered ? 0.4 : 0.2) * pulse;
          ctx.stroke(path);

          // 4. Preenchimento de Nebulosa
          ctx.fillStyle = color;
          ctx.globalAlpha = (isHovered ? 0.08 : 0.03);
          ctx.fill(path);

          ctx.restore();
        });
      }

      // Render Stars (Notes) with Frustum Culling
      ctx.globalCompositeOperation = 'lighter';
      const padding = 20;
      const worldLeft = (-transform.x - padding) / transform.k;
      const worldTop = (-transform.y - padding) / transform.k;
      const worldRight = (canvas.width - transform.x + padding) / transform.k;
      const worldBottom = (canvas.height - transform.y + padding) / transform.k;

      data.notes.forEach((note) => {
        if (note.x < worldLeft || note.x > worldRight || note.y < worldTop || note.y > worldBottom)
          return;

        const color = CLUSTER_COLORS[note.cluster_id % CLUSTER_COLORS.length];
        const isHovered = hoveredNote?.id === note.id;

        ctx.beginPath();
        const baseSize = isHovered ? 5.5 : 2.2;
        ctx.arc(note.x, note.y, baseSize / transform.k, 0, 2 * Math.PI);
        ctx.fillStyle = isHovered ? '#fff' : color;
        if (isHovered) {
          ctx.shadowBlur = 25 / transform.k;
          ctx.shadowColor = '#fff';
        }
        ctx.fill();
        ctx.shadowBlur = 0;
      });

      // Labels were removed here

      const noteAlpha = Math.max(0, Math.min(1, (transform.k - 4) / 2));
      if (noteAlpha > 0 || hoveredNote) {
        data.notes.forEach((note) => {
          const isHovered = hoveredNote?.id === note.id;
          if (!isHovered && noteAlpha <= 0) return;
          if (
            note.x < worldLeft ||
            note.x > worldRight ||
            note.y < worldTop ||
            note.y > worldBottom
          )
            return;

          const screenFontPx = isHovered ? 12 : 10;
          const worldFontSize = screenFontPx / transform.k;
          ctx.font = `${isHovered ? '700' : '500'} ${worldFontSize}px "Inter", sans-serif`;
          ctx.textAlign = 'center';
          ctx.textBaseline = 'top';

          const currentAlpha = isHovered ? 1 : noteAlpha;
          const title = noteTitle(note.id);
          const display = title.length > 25 ? `${title.substring(0, 22)}…` : title;
          ctx.fillStyle = isHovered ? '#fff' : `rgba(148, 163, 184, ${0.7 * currentAlpha})`;
          ctx.fillText(display, note.x, note.y + 2 / transform.k);
        });
      }

      // Render Query Point
      const qp = queryPointRef.current;
      if (qp) {
        const time = performance.now() / 1000;
        const pulse = Math.sin(time * 3) * 0.5 + 0.5;
        ctx.globalCompositeOperation = 'source-over';

        // Lines to nearest notes
        qp.nearest_notes.forEach((n, i) => {
          const alpha = [0.7, 0.5, 0.35][i] ?? 0.3;
          const dashOffset = -time * 30;
          ctx.save();
          ctx.beginPath();
          ctx.moveTo(qp.x, qp.y);
          ctx.lineTo(n.x, n.y);
          ctx.strokeStyle = '#a78bfa';
          ctx.lineWidth = (1.5 - i * 0.3) / transform.k;
          ctx.globalAlpha = alpha * (0.6 + pulse * 0.4);
          ctx.setLineDash([8 / transform.k, 14 / transform.k]);
          ctx.lineDashOffset = dashOffset / transform.k;
          ctx.shadowBlur = 12 / transform.k;
          ctx.shadowColor = '#a78bfa';
          ctx.stroke();
          ctx.restore();
        });

        // Glowing query dot
        ctx.save();
        const dotR = 6 / transform.k;
        ctx.beginPath();
        ctx.arc(qp.x, qp.y, dotR * (1.4 + pulse * 0.6), 0, 2 * Math.PI);
        ctx.fillStyle = 'rgba(167,139,250,0.15)';
        ctx.fill();
        ctx.beginPath();
        ctx.arc(qp.x, qp.y, dotR, 0, 2 * Math.PI);
        ctx.fillStyle = '#fff';
        ctx.shadowBlur = 20 / transform.k;
        ctx.shadowColor = '#a78bfa';
        ctx.fill();
        ctx.restore();
      }

      ctx.restore();
      frameId = requestAnimationFrame(render);
    };

    frameId = requestAnimationFrame(render);
    return () => cancelAnimationFrame(frameId);
  }, [data, hoveredNote, hoveredCluster, workerData]);

  // 5. Zoom Control
  useEffect(() => {
    if (!canvasRef.current || !data || data.notes.length === 0) return;
    const canvas = select(canvasRef.current);
    const zoomBehavior = d3zoom<HTMLCanvasElement, unknown>()
      .scaleExtent([0.05, 100])
      .on('zoom', (event) => {
        transformRef.current = event.transform;
      });
    zoomRef.current = zoomBehavior;
    canvas.call(zoomBehavior);

    // Initial Fit
    let minX = Infinity,
      maxX = -Infinity,
      minY = Infinity,
      maxY = -Infinity;
    data.notes.forEach((n) => {
      minX = Math.min(minX, n.x);
      maxX = Math.max(maxX, n.x);
      minY = Math.min(minY, n.y);
      maxY = Math.max(maxY, n.y);
    });

    const padding = 80;
    const width = window.innerWidth - padding * 2;
    const height = window.innerHeight - padding * 2;
    const dx = maxX - minX;
    const dy = maxY - minY;
    const scale = dx > 0 && dy > 0 ? Math.min(width / dx, height / dy, 15) : 5;
    const t = zoomIdentity
      .translate(window.innerWidth / 2, window.innerHeight / 2)
      .scale(scale)
      .translate(-(minX + maxX) / 2, -(minY + maxY) / 2);
    canvas.call(zoomBehavior.transform, t);
  }, [data]);

  // Interaction Helpers
  const findNearestNote = (worldX: number, worldY: number, screenTolerancePx: number) => {
    const { grid, cellSize } = workerData;
    const worldTolerance = screenTolerancePx / transformRef.current.k;
    const gx = Math.floor(worldX / cellSize);
    const gy = Math.floor(worldY / cellSize);
    const cellRadius = Math.max(2, Math.ceil(worldTolerance / cellSize));

    let bestNote: Point | null = null;
    let bestDist = worldTolerance;

    for (let dx = -cellRadius; dx <= cellRadius; dx++) {
      for (let dy = -cellRadius; dy <= cellRadius; dy++) {
        const cell = grid.get(`${gx + dx},${gy + dy}`);
        if (cell) {
          cell.forEach((note: Point) => {
            const d = Math.hypot(note.x - worldX, note.y - worldY);
            if (d < bestDist) {
              bestDist = d;
              bestNote = note;
            }
          });
        }
      }
    }
    return bestNote;
  };

  const handleCanvasClick = (e: MouseEvent) => {
    if (e.defaultPrevented || !data) return;
    const rect = canvasRef.current?.getBoundingClientRect();
    if (!rect) return;
    const transform = transformRef.current;
    const x = (e.clientX - rect.left - transform.x) / transform.k;
    const y = (e.clientY - rect.top - transform.y) / transform.k;
    const note = findNearestNote(x, y, 20);
    if (note) onOpenNote(note.id);
  };

  const handleMouseMove = (e: MouseEvent) => {
    if (!data) return;
    const rect = canvasRef.current?.getBoundingClientRect();
    if (!rect) return;
    const transform = transformRef.current;
    const x = (e.clientX - rect.left - transform.x) / transform.k;
    const y = (e.clientY - rect.top - transform.y) / transform.k;

    const note = findNearestNote(x, y, 20);

    if (note?.id !== hoveredNote?.id) setHoveredNote(note);

    if (note) setTooltipPos({ x: e.clientX, y: e.clientY });
    else if (tooltipPos) setTooltipPos(null);
  };

  // Reindex Logic
  const handleReindex = async () => {
    try {
      await fetch('/api/graph/reindex', { method: 'POST', headers: { Authorization: auth } });
    } catch {
      setReindexing('error');
    }
  };

  useEffect(() => {
    let interval: any;
    const checkStatus = async () => {
      try {
        const res = await fetch('/api/graph/status', { headers: { Authorization: auth } });
        const status: ReindexStatus = await res.json();
        setReindexStatus(status);
        if (status.is_reindexing) setReindexing('running');
        else if (reindexing === 'running') {
          const dataRes = await fetch('/api/graph/map', { headers: { Authorization: auth } });
          const d = await dataRes.json();
          setData(d);
          if (workerRef.current)
            workerRef.current.postMessage({ notes: d.notes, clusters: d.clusters });
          setReindexing('idle');
        }
      } catch (e) {
        console.error('Error polling status:', e);
      }
    };
    checkStatus();
    interval = setInterval(checkStatus, 1500);
    return () => clearInterval(interval);
  }, [auth, reindexing]);

  const isEmpty = !loading && (!data || data.notes.length === 0);

  return (
    <div
      className="fixed inset-0 z-[60] bg-[#020617] overflow-hidden"
      style={{ cursor: hoveredNote ? 'pointer' : 'grab' }}
    >
      {loading && (
        <div className="absolute inset-0 flex items-center justify-center bg-[#020617]/50 backdrop-blur-md z-10">
          <div className="flex flex-col items-center gap-4">
            <div className="w-12 h-12 border-4 border-sky-500/30 border-t-sky-500 rounded-full animate-spin" />
            <span className="text-sky-400/60 font-medium animate-pulse tracking-widest text-xs uppercase">
              Sincronizando {'Mapa Semântico'}...
            </span>
          </div>
        </div>
      )}

      {isEmpty && (
        <div className="absolute inset-0 flex items-center justify-center z-10">
          <div className="text-center max-w-sm space-y-5 px-6">
            <div className="text-5xl">{reindexing === 'running' ? '⏳' : '🗺️'}</div>
            <h2 className="text-xl font-bold text-zinc-200">
              {reindexing === 'running' ? 'Gerando Projeções...' : 'Mapa Semântico' + ' Vazio'}
            </h2>
            <p className="text-zinc-500 text-sm leading-relaxed">
              {reindexing === 'running'
                ? `O Ollama está processando as notas (${reindexStatus?.processed || 0}/${reindexStatus?.total || 0}). Isso pode levar alguns minutos.`
                : 'Esta instância tem notas, mas seus vetores de similaridade ainda não foram gerados.'}
            </p>
            {reindexing === 'running' && reindexStatus && (
              <div className="space-y-2">
                <div className="w-full bg-zinc-900 border border-zinc-800 rounded-full h-3 overflow-hidden">
                  <div
                    className="bg-gradient-to-r from-sky-600 to-sky-400 h-full transition-all duration-700 ease-out shadow-[0_0_15px_rgba(56,189,248,0.5)]"
                    style={{ width: `${reindexStatus.percent}%` }}
                  />
                </div>
                <div className="text-[10px] text-sky-400/50 font-bold uppercase tracking-widest flex justify-between">
                  <span>{reindexStatus.percent}% Concluído</span>
                  <span>
                    {reindexStatus.processed} de {reindexStatus.total} notas
                  </span>
                </div>
              </div>
            )}
            <div className="flex gap-3 justify-center pt-2">
              <button
                onClick={onClose}
                className="px-5 py-2.5 bg-zinc-900 border border-zinc-800 text-zinc-400 rounded-xl hover:bg-zinc-800 transition-all text-sm"
              >
                Voltar
              </button>
              {reindexing === 'idle' && (
                <button
                  onClick={handleReindex}
                  className="px-6 py-2.5 bg-sky-600 border border-sky-500/50 text-white rounded-xl hover:bg-sky-500 transition-all text-sm font-medium shadow-lg shadow-sky-600/20"
                >
                  ✦ Gerar Mapa
                </button>
              )}
            </div>
          </div>
        </div>
      )}

      <div className="absolute top-6 left-6 z-20 flex items-center gap-2">
        <button
          onClick={onClose}
          className="bg-zinc-900/80 backdrop-blur-xl border border-zinc-800 px-4 py-2.5 rounded-2xl text-zinc-400 hover:text-white hover:bg-zinc-800 transition-all shadow-2xl flex items-center gap-2"
        >
          <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M10 19l-7-7m0 0l7-7m-7 7h18" />
          </svg>
          <span className="font-bold text-xs uppercase tracking-widest">Voltar</span>
        </button>

        {/* Query button */}
        {data && data.notes.length > 0 && (
          <button
            id="map-query-btn"
            onClick={() => { setShowQueryModal(true); setQueryError(null); }}
            title="Perguntar ao mapa"
            className="bg-zinc-900/80 backdrop-blur-xl border border-violet-500/40 w-10 h-10 rounded-2xl text-violet-400 hover:text-white hover:bg-violet-900/40 transition-all shadow-2xl flex items-center justify-center text-lg font-bold"
          >
            ?
          </button>
        )}

        {/* Clear query button */}
        {queryPoint && (
          <button
            id="map-clear-query-btn"
            onClick={() => setQueryPoint(null)}
            title="Limpar pergunta"
            className="bg-zinc-900/80 backdrop-blur-xl border border-violet-500/40 px-3 py-2.5 rounded-2xl text-violet-300 hover:text-white hover:bg-violet-900/40 transition-all shadow-2xl flex items-center gap-1.5 text-xs font-bold"
          >
            <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2.5" d="M6 18L18 6M6 6l12 12" />
            </svg>
            Limpar
          </button>
        )}
      </div>

      {/* Query Modal */}
      {showQueryModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm">
          <div className="bg-zinc-900 border border-violet-500/30 rounded-2xl shadow-2xl shadow-violet-900/30 p-6 w-full max-w-md mx-4">
            <h3 className="text-sm font-bold uppercase tracking-widest text-violet-300 mb-1">Consultar o Mapa</h3>
            <p className="text-zinc-500 text-xs mb-4">Digite uma pergunta ou tema. O mapa mostrará as notas mais relevantes.</p>
            <textarea
              id="map-query-input"
              autoFocus
              rows={3}
              value={queryInput}
              onInput={(e) => setQueryInput((e.target as HTMLTextAreaElement).value)}
              placeholder="Ex: Como funciona a memória de longo prazo?"
              className="w-full bg-zinc-800 border border-zinc-700 rounded-xl px-3 py-2.5 text-sm text-zinc-100 placeholder-zinc-600 resize-none outline-none focus:border-violet-500 transition-colors"
            />
            {queryError && <p className="text-red-400 text-xs mt-2">{queryError}</p>}
            {queryPoint && (
              <p className="text-violet-400/70 text-xs mt-2 truncate">✦ Mapeado: &quot;{queryPoint.query}&quot;</p>
            )}
            <div className="flex gap-2 mt-4">
              <button
                onClick={() => { setShowQueryModal(false); setQueryInput(''); }}
                className="flex-1 px-4 py-2 bg-zinc-800 border border-zinc-700 text-zinc-400 rounded-xl hover:bg-zinc-700 transition-all text-sm"
              >Cancelar</button>
              <button
                id="map-query-submit"
                disabled={queryLoading || !queryInput.trim()}
                onClick={async () => {
                  if (!queryInput.trim()) return;
                  setQueryLoading(true);
                  setQueryError(null);
                  try {
                    const res = await fetch('/api/graph/query-point', {
                      method: 'POST',
                      headers: { Authorization: auth, 'Content-Type': 'application/json' },
                      body: JSON.stringify({ query: queryInput.trim() }),
                    });
                    if (!res.ok) {
                      const txt = await res.text();
                      setQueryError(txt || 'Erro ao processar pergunta.');
                    } else {
                      const result = await res.json();
                      setQueryPoint({ ...result, query: queryInput.trim() });
                      setShowQueryModal(false);
                      setQueryInput('');
                    }
                  } catch {
                    setQueryError('Erro de conexão.');
                  } finally {
                    setQueryLoading(false);
                  }
                }}
                className="flex-1 px-4 py-2 bg-violet-600 border border-violet-500/50 text-white rounded-xl hover:bg-violet-500 transition-all text-sm font-medium disabled:opacity-40 disabled:cursor-not-allowed flex items-center justify-center gap-2"
              >
                {queryLoading ? (
                  <><span className="w-3.5 h-3.5 border-2 border-white/30 border-t-white rounded-full animate-spin" />Processando...</>
                ) : '✦ Mapear'}
              </button>
            </div>
          </div>
        </div>
      )}

      {data && data.notes.length > 0 && (
        <div className="absolute bottom-6 left-6 z-20 bg-zinc-900/60 backdrop-blur-md border border-zinc-800/50 px-4 py-2 rounded-2xl text-[10px] text-sky-400/60 font-bold uppercase tracking-widest">
          {data.notes.length} Notas · {data.clusters.length} Agrupamentos
        </div>
      )}

      {hoveredNote && tooltipPos && (
        <div
          className="absolute z-30 bg-zinc-900/90 backdrop-blur-md border border-sky-500/50 px-3 py-2 rounded-xl text-xs text-zinc-100 pointer-events-none shadow-[0_0_20px_rgba(56,189,248,0.2)]"
          style={{ left: tooltipPos.x + 12, top: tooltipPos.y - 40 }}
        >
          {noteTitle(hoveredNote.id)}
        </div>
      )}



      <canvas
        ref={canvasRef}
        onClick={handleCanvasClick}
        onMouseMove={handleMouseMove}
        className="w-full h-full"
      />
    </div>
  );
}
