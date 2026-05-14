import { useMemo, useState } from "preact/compat";

import { useAuthenticatedAsset } from "../hooks/useAuthenticatedAsset";
import { useIntersectionObserver } from "../hooks/useIntersectionObserver";
import type { FileObject, SearchResult } from "../types";
import { extractFragment } from "../utils/search";
import { HighlightMarkdown } from "./HighlightMarkdown";
import { HighlightText } from "./HighlightText";
import { ResultActions } from "./ResultActions";
import { ScoreBreakdown } from "./ScoreBreakdown";
import { Skeleton } from "./Skeleton";
import { TagList } from "./TagList";
import { ResultIcon } from "./ResultIcon";

interface SearchResultCardProps {
  doc: SearchResult;
  index: number;
  query: string;
  searchTerms: string[];
  onEdit: (file: FileObject) => void;
  isCompact: boolean;
  isLastCollapsed: boolean;
  fetchWithAuth: (
    url: string,
    options?: RequestInit,
  ) => Promise<Response | null>;
  auth: string | null;
  isIndexing?: boolean;
  isExpanded?: boolean;
  toggleExpandText?: (id: string) => void;
  onDeleteFile: (filename: string) => void;
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
  onDeleteFile,
}: SearchResultCardProps) => {
  const [isOpening, setIsOpening] = useState(false);

  const handleEdit = async (e: MouseEvent) => {
    e.stopPropagation();
    if (isOpening) return;
    setIsOpening(true);
    try {
      const res = await fetchWithAuth(
        `/api/file?name=${encodeURIComponent(doc.arquivo)}`,
      );
      if (res?.ok) {
        const text = await res.text();
        onEdit({
          name: doc.arquivo,
          content: text,
          scrollToText: doc.texto,
        });
      }
    } catch (e) {
      console.error("Edit error:", e);
    } finally {
      setIsOpening(false);
    }
  };

  const handleOpenAsset = (e: MouseEvent) => {
    e.stopPropagation();
    const base =
      blobUrl ||
      `/api/file?name=${encodeURIComponent(doc.arquivo)}&token=${auth}`;
    const url = doc.tipo === "pdf" ? `${base}#page=${doc.pagina || 1}` : base;
    window.open(url, "_blank");
  };

  const { ref, isInView } = useIntersectionObserver({
    threshold: 0.05,
    rootMargin: "200px",
  });

  const isMedia = doc.tipo === "image" || doc.tipo === "pdf";
  const { blobUrl, isLoading: isAssetLoading } = useAuthenticatedAsset(
    doc.arquivo,
    fetchWithAuth,
    isMedia && isInView,
  );

  const terms = searchTerms || [];
  const isArquivos =
    doc.tags?.some((t) => t.toLowerCase() === "arquivos") ||
    (doc.arquivo &&
      (doc.arquivo.includes("arquivos_") || doc.arquivo.includes("bundle_")));
  const isImagem =
    doc.tags?.some((t) => t.toLowerCase() === "imagem") ||
    doc.tipo === "image" ||
    doc.tipo === "imagem";
  const isPDF =
    doc.tags?.some(
      (t) => t.toLowerCase() === "pdf" || t.toLowerCase() === "documento",
    ) || doc.tipo === "pdf";

  return (
    <li
      ref={ref as any}
      className={`relative bg-transparent p-0 rounded-none border-b border-zinc-900/50 hover:border-zinc-800 transition-all duration-300 group overflow-hidden scroll-mt-[160px] pb-8
      `}
    >
      <div className="relative z-10 px-1">
        <div className="flex items-center justify-between mb-2 gap-4">
          <div className="flex items-center gap-2 min-w-0">
            {doc.tipo === "pdf" ? (
              <span className="text-[9px] font-black tracking-widest text-zinc-500 uppercase bg-zinc-800/30 px-1.5 py-0.5 rounded shrink-0">
                Pág {doc.pagina || 1}
              </span>
            ) : (
            <div className="flex items-center opacity-70 shrink-0">
              <ResultIcon doc={doc} isIndexing={isIndexing} />
            </div>
            )}

            <h3
              onClick={isMedia ? (handleOpenAsset as any) : (handleEdit as any)}
              className={`text-[13px] font-black tracking-tight cursor-pointer hover:opacity-70 transition-all truncate ${doc.arquivo.startsWith("links/") ? "text-amber-400" : isArquivos ? "text-indigo-400" : isImagem ? "text-emerald-400" : isPDF ? "text-red-400" : "text-sky-400"}`}
            >
              {isOpening ? (
                <span className="animate-pulse">Abrindo...</span>
              ) : (
                <HighlightText
                  text={doc.arquivo.split("/").pop()?.replace(/\.(md|pdf)$/i, "")}
                  query={query}
                  searchTerms={searchTerms}
                />
              )}
            </h3>
          </div>
        </div>

        <div className="relative">
          <div className="text-[14px] leading-relaxed text-zinc-400 font-sans mt-2">
            {isMedia ? (
              <div className="flex flex-col gap-4">
                {doc.texto && (
                  <div className={`overflow-hidden ${isExpanded ? "" : "line-clamp-[5]"}`}>
                    <HighlightMarkdown
                      text={doc.texto}
                      query={query}
                      searchTerms={terms}
                      isExpanded={!!isExpanded}
                    />
                  </div>
                )}
                
                {doc.tipo === "image" && blobUrl && (
                  <div className="rounded-lg overflow-hidden border border-zinc-800/50 max-w-lg mt-2">
                    <img src={blobUrl} className="w-full h-auto max-h-[300px] object-contain" />
                  </div>
                )}
              </div>
            ) : (
              <div className={`overflow-hidden ${isExpanded ? "" : "line-clamp-[5]"}`}>
                <HighlightMarkdown
                  text={doc.texto}
                  query={query}
                  searchTerms={terms}
                  isExpanded={!!isExpanded}
                />
              </div>
            )}
          </div>

          {doc.texto &&
            (doc.texto.length > 300 ||
              (doc.texto.match(/\n/g) || []).length > 5) && (
              <button
                onClick={(e) => {
                  e.stopPropagation();
                  toggleExpandText?.(doc.id);
                }}
                className="mt-4 flex items-center gap-1.5 text-[10px] font-black uppercase tracking-[0.2em] text-zinc-500 hover:text-sky-400 transition-colors"
              >
                <span>{isExpanded ? "Recolher Texto" : "Expandir Texto"}</span>
              </button>
            )}
        </div>

        <div className="mt-4 flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4">
          <div className="flex flex-wrap items-center gap-4 flex-1 min-w-0">
            {doc.score_details && (
              <ScoreBreakdown
                details={doc.score_details}
                score={doc.final_score || 0}
                timestamp={doc["@timestamp"]}
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
