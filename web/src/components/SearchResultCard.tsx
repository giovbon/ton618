import { useMemo } from 'preact/compat';

import { useAuthenticatedAsset } from '../hooks/useAuthenticatedAsset';
import { useIntersectionObserver } from '../hooks/useIntersectionObserver';
import type { FileObject, SearchResult } from '../types';
import { extractFragment } from '../utils/search';
import { HighlightMarkdown } from './HighlightMarkdown';
import { HighlightText } from './HighlightText';
import { ResultActions } from './ResultActions';
import { ScoreBreakdown } from './ScoreBreakdown';
import { Skeleton } from './Skeleton';
import { TagList } from './TagList';

interface SearchResultCardProps {
  doc: SearchResult;
  index: number;
  query: string;
  searchTerms: string[];
  onEdit: (file: FileObject) => void;
  isCompact: boolean;
  isLastCollapsed: boolean;
  fetchWithAuth: (url: string, options?: RequestInit) => Promise<Response | null>;
  auth: string | null;
  isIndexing?: boolean;
  isExpanded?: boolean;
  toggleExpandText?: (id: string) => void;
}

export const SearchResultCard = ({
  doc,
  query,
  searchTerms,
  onEdit,
  isLastCollapsed,
  fetchWithAuth,
  auth,
  isIndexing,
  isExpanded,
  toggleExpandText,
}: SearchResultCardProps) => {
  const { ref, isInView } = useIntersectionObserver({ threshold: 0.05, rootMargin: '200px' });

  const isMedia = doc.tipo === 'image' || doc.tipo === 'pdf';
  const { blobUrl, isLoading: isAssetLoading } = useAuthenticatedAsset(
    doc.arquivo,
    fetchWithAuth,
    isMedia && isInView,
  );

  const terms = searchTerms || [];
  const isArquivos =
    doc.tags?.some((t) => t.toLowerCase() === 'arquivos') ||
    (doc.arquivo && (doc.arquivo.includes('arquivos_') || doc.arquivo.includes('bundle_')));
  const isImagem =
    doc.tags?.some((t) => t.toLowerCase() === 'imagem') ||
    doc.tipo === 'image' ||
    doc.tipo === 'imagem';
  const isPDF =
    doc.tags?.some((t) => t.toLowerCase() === 'pdf' || t.toLowerCase() === 'documento') ||
    doc.tipo === 'pdf';

  return (
    <li
      ref={ref as any}
      className={`relative premium-card bg-zinc-900/80 p-5 rounded-2xl border transition-all duration-300 group overflow-hidden scroll-mt-[160px]
        ${
          isLastCollapsed
            ? isArquivos
              ? 'border-indigo-500/50 shadow-[0_0_20px_rgba(99,102,241,0.15)] ring-1 ring-indigo-500/20'
              : isImagem
                ? 'border-emerald-500/50 shadow-[0_0_20px_rgba(16,185,129,0.15)] ring-1 ring-emerald-500/20'
                : isPDF
                  ? 'border-red-500/50 shadow-[0_0_20px_rgba(239,68,68,0.15)] ring-1 ring-red-500/20'
                  : 'border-sky-500/50 shadow-[0_0_20px_rgba(14,165,233,0.15)] ring-1 ring-sky-500/20'
            : `border-zinc-800/60 ${isArquivos ? 'hover:border-indigo-500/30' : isImagem ? 'hover:border-emerald-500/30' : isPDF ? 'hover:border-red-500/30' : 'hover:border-zinc-700/60'}`
        }
      `}
    >
      <div className="relative z-10">
        <div className="flex items-start justify-between mb-3 gap-4">
          <div className="flex flex-col gap-1 min-w-0 px-1">
            <div className="flex items-center gap-1.5 mb-1">
              {doc.tipo === 'pdf' ? (
                <span className="text-[9px] font-black tracking-widest text-zinc-500 uppercase bg-zinc-800/50 px-1.5 py-0.5 rounded">
                  Página {doc.pagina || 1}
                </span>
              ) : (
                <div className="flex items-center flex-wrap gap-x-2 gap-y-1 opacity-60 hover:opacity-100 transition-opacity">
                  <div className="flex items-center gap-1 shrink-0">
                    <svg
                      className="w-2.5 h-2.5 text-zinc-500"
                      fill="none"
                      stroke="currentColor"
                      viewBox="0 0 24 24"
                    >
                      <path
                        strokeLinecap="round"
                        strokeLinejoin="round"
                        strokeWidth="3"
                        d="M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z"
                      />
                    </svg>
                    <span className="text-[9px] font-black text-zinc-500 uppercase tracking-[0.15em]">
                      {doc.arquivo.split('/').slice(0, -1).join(' / ')}
                    </span>
                  </div>
                  {doc.secao && doc.secao !== 'Geral' && (
                    <div className="flex items-center gap-1 min-w-0">
                      <div className="w-1 h-1 rounded-full bg-zinc-800" />
                      <span className="text-[9px] font-black text-sky-500/80 uppercase tracking-widest italic">
                        {doc.secao}
                      </span>
                    </div>
                  )}
                </div>
              )}
            </div>
            <div className="flex items-center gap-2 mt-1">
              <h3
                className={`text-sm md:text-base font-black group-hover:opacity-80 transition-opacity whitespace-normal break-words tracking-tight ${doc.arquivo.startsWith('links/') ? 'text-amber-500' : isArquivos ? 'text-indigo-400' : isImagem ? 'text-emerald-400' : isPDF ? 'text-red-400' : 'text-sky-400'}`}
              >
                <HighlightText
                  text={doc.arquivo
                    .split('/')
                    .pop()
                    ?.replace(/\.(md|pdf)$/i, '')}
                  query={query}
                  searchTerms={searchTerms}
                />
              </h3>
            </div>
          </div>

          <ResultActions
            doc={doc}
            onEdit={onEdit}
            fetchWithAuth={fetchWithAuth}
            auth={auth}
            blobUrl={blobUrl}
          />
        </div>

        <div className="relative">
          <div className="text-sm leading-relaxed text-zinc-300 font-sans overflow-hidden">
            {isMedia ? (
              <div className="flex flex-col gap-4">
                <div
                  className={`flex flex-col gap-3 bg-zinc-950/80 p-5 rounded-xl border-2 group/ocr relative overflow-hidden order-first
                  ${
                    doc.tipo === 'pdf'
                      ? 'border-red-500/40 shadow-[0_0_20px_rgba(239,68,68,0.1)]'
                      : 'border-emerald-500/40 shadow-[0_0_20px_rgba(16,185,129,0.1)]'
                  }
                `}
                >
                  <div className="flex items-center justify-between mb-1">
                    <div className="flex items-center gap-2">
                      <span
                        className={`text-[10px] font-black tracking-[0.2em] uppercase ${doc.tipo === 'pdf' ? 'text-red-400' : 'text-emerald-400'}`}
                      >
                        {'Texto Extraído (OCR)'}
                      </span>
                    </div>

                    {doc.texto && (
                      <button
                        onClick={(e) => {
                          e.stopPropagation();
                          navigator.clipboard.writeText(doc.texto);
                        }}
                        className={`px-3 py-1 border rounded text-[10px] font-bold tracking-widest transition-colors flex items-center gap-1.5 ${doc.tipo === 'pdf' ? 'bg-red-500/10 hover:bg-red-500/20 border-red-500/30 text-red-400' : 'bg-emerald-500/10 hover:bg-emerald-500/20 border-emerald-500/30 text-emerald-400'}`}
                      >
                        {'Copiar'}
                      </button>
                    )}
                  </div>

                  {!doc.texto && isIndexing ? (
                    <div className="py-2">
                      <p
                        className={`text-[11px] font-bold uppercase tracking-widest animate-pulse ${doc.tipo === 'pdf' ? 'text-red-500/60' : 'text-emerald-500/60'}`}
                      >
                        {'Lendo arquivo via IA...'}
                      </p>
                    </div>
                  ) : doc.texto ? (
                    <div
                      className={`text-sm text-zinc-100 font-sans leading-relaxed ${isExpanded ? '' : 'line-clamp-[5]'} overflow-hidden`}
                    >
                      <HighlightMarkdown
                        text={doc.texto}
                        query={query}
                        searchTerms={terms}
                        isExpanded={!!isExpanded}
                      />
                    </div>
                  ) : (
                    <p className="text-[11px] italic text-zinc-600 font-medium tracking-wide">
                      {'Nenhum texto detectado neste arquivo.'}
                    </p>
                  )}
                </div>

                {doc.tipo === 'image' &&
                  (isAssetLoading ? (
                    <Skeleton className="w-full h-48 rounded-xl" />
                  ) : blobUrl ? (
                    <div className="relative group/img overflow-hidden rounded-xl border border-zinc-800 shadow-2xl bg-black/50">
                      <img
                        src={blobUrl}
                        alt={doc.arquivo}
                        className="w-full h-auto object-contain max-h-[300px] transition-all duration-700 cursor-zoom-in"
                        onClick={() => window.open(blobUrl, '_blank')}
                      />
                    </div>
                  ) : (
                    <div className="p-4 bg-red-950/20 border border-red-900/50 rounded-xl text-red-400 text-xs flex items-center gap-2">
                      {'Erro ao carregar anexo.'}
                    </div>
                  ))}
              </div>
            ) : (
              <div className={`${isExpanded ? '' : 'line-clamp-[5]'} overflow-hidden`}>
                <HighlightMarkdown
                  text={doc.texto}
                  query={query}
                  searchTerms={terms}
                  isExpanded={!!isExpanded}
                />
              </div>
            )}
          </div>

          {doc.texto && (doc.texto.length > 300 || (doc.texto.match(/\n/g) || []).length > 5) && (
            <button
              onClick={(e) => {
                e.stopPropagation();
                toggleExpandText?.(doc.id);
              }}
              className="mt-4 flex items-center gap-1.5 text-[10px] font-black uppercase tracking-[0.2em] text-zinc-500 hover:text-sky-400 transition-colors group/expand"
            >
              <span>{isExpanded ? 'Recolher Texto' : 'Expandir Texto'}</span>
              <svg
                className={`w-3 h-3 transition-transform duration-300 ${isExpanded ? 'rotate-180' : ''}`}
                fill="none"
                stroke="currentColor"
                viewBox="0 0 24 24"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth="3"
                  d="M19 9l-7 7-7-7"
                />
              </svg>
            </button>
          )}
        </div>

        <div className="mt-4 pt-4 border-t border-zinc-800/40 flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4">
          <div className="flex flex-wrap items-center gap-4 flex-1 min-w-0">
            {doc.score_details && (
              <ScoreBreakdown
                details={doc.score_details}
                score={doc.final_score || 0}
                timestamp={doc['@timestamp']}
              />
            )}
            <div className="w-px h-4 bg-zinc-800 hidden sm:block opacity-50" />
            <div className="flex-1 min-w-0">
              <TagList tags={doc.tags || []} query={query} />
            </div>
          </div>
        </div>
      </div>
    </li>
  );
};
