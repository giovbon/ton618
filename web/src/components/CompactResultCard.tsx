import { memo, useEffect, useMemo, useRef } from "preact/compat";

import { useAuthenticatedAsset } from "../hooks/useAuthenticatedAsset";
import { useIntersectionObserver } from "../hooks/useIntersectionObserver";
import type { FileObject, SearchResult } from "../types";
import { formatAge } from "../utils/time";
import { HighlightText } from "./HighlightText";
import { ResultIcon } from "./ResultIcon";

interface CompactResultCardProps {
  doc: SearchResult;
  index: number;
  query: string;
  searchTerms: string[];
  onEdit: (file: FileObject) => void;
  onDeleteFile: (filename: string) => void;
  fetchWithAuth: (
    url: string,
    options?: RequestInit,
  ) => Promise<Response | null>;
  auth: string | null;
  isIndexing?: boolean;
  isHighlighted?: boolean;
}

export const CompactResultCard = memo(
  ({
    doc,
    query,
    searchTerms,
    onEdit,
    onDeleteFile,
    fetchWithAuth,
    auth,
    isIndexing,
    isHighlighted,
  }: CompactResultCardProps) => {
    const liRef = useRef<HTMLLIElement>(null);

    useEffect(() => {
      if (isHighlighted && liRef.current) {
        const el = liRef.current;
        el.style.borderColor = "#0ea5e9";
        el.style.boxShadow = "0 0 20px rgba(14,165,233,0.4)";
        el.style.transform = "scale(1.01)";
        el.style.transition = "all 2.5s cubic-bezier(0.4,0,0.2,1)";
        // After animation, reset to normal
        setTimeout(() => {
          el.style.borderColor = "";
          el.style.boxShadow = "";
          el.style.transform = "";
          el.style.transition = "";
        }, 2500);
      }
    }, [isHighlighted]);
    const { ref, isInView } = useIntersectionObserver({ threshold: 0.1 });

    const isMedia = doc.tipo === "image" || doc.tipo === "pdf";
    const { blobUrl } = useAuthenticatedAsset(
      doc.arquivo,
      fetchWithAuth,
      isMedia && isInView,
    );

    const handleOpen = async (e: MouseEvent) => {
      e.stopPropagation();
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
        console.error("Open error:", e);
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

    const age = useMemo(
      () => formatAge(doc["@timestamp"]),
      [doc["@timestamp"]],
    );
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
        ref={(el) => {
          (ref as any).current = el;
          liRef.current = el;
        }}
        onClick={isMedia ? (handleOpenAsset as any) : (handleOpen as any)}
        className="group relative flex items-center justify-between gap-2 px-3 py-3 sm:py-1.5 bg-zinc-950/20 hover:bg-zinc-900/60 border-b border-zinc-900/40 hover:border-zinc-800 transition-all duration-200 cursor-pointer overflow-hidden"
      >
        <div className="flex items-center flex-1 min-w-0 gap-2">
          <ResultIcon doc={doc} isIndexing={isIndexing} />

          <span
            className={`text-[11px] font-medium truncate flex-1 min-w-0 ${doc.arquivo.startsWith("links/") ? "text-amber-500/90" : isArquivos ? "text-indigo-400/90" : isImagem ? "text-emerald-400/90" : isPDF ? "text-red-400/90" : "text-sky-400/90"}`}
          >
            <HighlightText
              text={doc.arquivo
                .split("/")
                .pop()
                ?.replace(/\.(md|pdf)$/i, "")}
              query={query}
              searchTerms={searchTerms}
            />
          </span>
        </div>

        <div className="flex items-center gap-2 shrink-0">
          <div className="hidden md:flex flex-wrap justify-end gap-1">
            {(doc.tags || []).slice(0, 3).map((tag) => (
              <span
                key={tag}
                className="px-1 py-0 rounded bg-zinc-800/30 text-[9px] font-bold text-zinc-600"
              >
                #{tag}
              </span>
            ))}
          </div>

          <span className="text-[10px] text-zinc-600 font-medium tabular-nums w-10 text-right">
            {age}
          </span>

          <button
            onClick={(e) => {
              e.stopPropagation();
              onDeleteFile(doc.arquivo);
            }}
            title={"Excluir arquivo"}
            className="p-1 rounded text-zinc-600 hover:text-red-400 hover:bg-red-400/10 transition-all opacity-0 group-hover:opacity-100"
          >
            <svg
              className="w-3 h-3"
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth="2"
                d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"
              />
            </svg>
          </button>
        </div>
      </li>
    );
  },
);
