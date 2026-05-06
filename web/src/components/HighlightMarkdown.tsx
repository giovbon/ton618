import { memo, useMemo } from 'preact/compat';
import { useMarkdownWorker } from '../hooks/useMarkdownWorker';
import { extractFragment } from '../utils/search';

interface HighlightMarkdownProps {
  text: string;
  query: string;
  searchTerms: string[];
  isExpanded: boolean;
}

/**
 * Componente para renderizar Markdown com realce via Web Worker.
 */
export const HighlightMarkdown = memo(function HighlightMarkdown({
  text,
  query,
  searchTerms,
  isExpanded,
}: HighlightMarkdownProps) {
  const terms = searchTerms || [];

  // 1. Extração de Fragmento (Síncrona - rápida)
  const { fragment, hasMoreBefore, hasMoreAfter } = useMemo(() => {
    return isExpanded
      ? { fragment: text || '', hasMoreBefore: false, hasMoreAfter: false }
      : extractFragment(text, query, terms);
  }, [text, query, terms, isExpanded]);

  // 2. Parsing de Markdown via Worker (Assíncrona - pesada)
  const { html, isLoading } = useMarkdownWorker(fragment, query, terms);

  return (
    <div className="w-full flex flex-col">
      {hasMoreBefore && (
        <div className="text-[10px] font-black tracking-widest text-zinc-600 mb-1.5 opacity-60">
          ...
        </div>
      )}
      <div
        className={`inline prose prose-sm prose-invert prose-zinc w-full max-w-full 
                   prose-p:inline prose-p:w-full prose-p:max-w-full
                   prose-a:text-sky-400 prose-headings:text-zinc-200 prose-strong:text-zinc-300
                   prose-table:border prose-table:border-zinc-600 prose-th:bg-zinc-800 prose-th:p-3 prose-th:text-white prose-td:p-3 prose-td:border prose-td:border-zinc-700
                   transition-opacity duration-200 ${isLoading ? 'opacity-50' : 'opacity-100'}`}
      >
        <span dangerouslySetInnerHTML={{ __html: html || fragment }} />
      </div>
      {hasMoreAfter && (
        <div className="text-[10px] font-black tracking-widest text-zinc-600 mt-1.5 opacity-60">
          ...
        </div>
      )}
    </div>
  );
});
