import { memo } from 'preact/compat';
import type { SearchResult } from '../types';

interface ResultIconProps {
  doc: SearchResult;
  isIndexing?: boolean;
}

/**
 * ResultIcon - Ícone representativo do tipo de arquivo com indicador de status de vetorização.
 */
export const ResultIcon = memo(({ doc, isIndexing }: ResultIconProps) => {
  const type = doc.tipo;
  const filename = doc.arquivo;
  const hasEmbedTag = doc.tags?.some((t) => t.toLowerCase() === 'embed');
  const isVectorizable = type === 'image' || type === 'imagem' || type === 'desenho' || hasEmbedTag;

  const renderBaseIcon = () => {
    const isPDF =
      type === 'pdf' ||
      doc.tags?.some((t) => t.toLowerCase() === 'pdf' || t.toLowerCase() === 'documento');
    if (isPDF) {
      return (
        <div
          className="bg-red-500/10 border border-red-500/20 p-1 rounded-lg text-red-500 shadow-sm shrink-0"
          title="Documento PDF"
        >
          <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth="2.5"
              d="M7 21h10a2 2 0 002-2V9.414a1 1 0 00-.293-.707l-5.414-5.414A1 1 0 0012.586 3H7a2 2 0 00-2 2v14a2 2 0 002 2z"
            />
          </svg>
        </div>
      );
    }
    const isArquivos =
      doc.tags?.some((t) => t.toLowerCase() === 'arquivos') ||
      (filename && (filename.includes('arquivos_') || filename.includes('bundle_')));
    if (isArquivos) {
      return (
        <div
          className="bg-indigo-500/10 border border-indigo-500/20 p-1 rounded-lg text-indigo-400 shadow-sm shrink-0"
          title="Arquivos (Temporário)"
        >
          <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth="2.5"
              d="M20 7l-8-4-8 4m16 0l-8 4m8-4v10l-8 4m0-10L4 7m8 4v10M4 7v10l8 4"
            />
          </svg>
        </div>
      );
    }
    if (filename?.startsWith('links/')) {
      return (
        <div
          className="bg-amber-500/10 border border-amber-500/20 p-1 rounded-lg text-amber-500 shadow-sm shrink-0"
          title="Link Capturado"
        >
          <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth="2.5"
              d="M13.828 10.172a4 4 0 0 0-5.656 0l-4 4a4 4 0 1 0 5.656 5.656l1.102-1.101m-.758-4.899a4 4 0 0 0 5.656 0l4-4a4 4 0 0 0-5.656-5.656l-1.1 1.1"
            />
          </svg>
        </div>
      );
    }
    const isImagem =
      type === 'image' || type === 'imagem' || doc.tags?.some((t) => t.toLowerCase() === 'imagem');
    if (isImagem) {
      return (
        <div
          className="bg-emerald-500/10 border border-emerald-500/20 p-1 rounded-lg text-emerald-400 shadow-sm shrink-0"
          title="Anexo de Imagem (OCR)"
        >
          <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth="2.5"
              d="M4 16l4.586-4.586a2 2 0 012.828 0L16 16m-2-2l1.586-1.586a2 2 0 012.828 0L20 14m-6-6h.01M6 20h12a2 2 0 002-2V6a2 2 0 00-2-2H6a2 2 0 00-2 2v12a2 2 0 002 2z"
            />
          </svg>
        </div>
      );
    }
    if (type === 'desenho') {
      return (
        <div
          className="bg-purple-500/10 border border-purple-500/20 p-1 rounded-lg text-purple-400 shadow-sm shrink-0"
          title="Desenho Tldraw"
        >
          <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth="2.5"
              d="M15.232 5.232l3.536 3.536m-2.036-5.036a2.5 2.5 0 113.536 3.536L6.5 21.036H3v-3.572L16.732 3.732z"
            />
          </svg>
        </div>
      );
    }
    return (
      <div
        className="p-1 rounded-lg shadow-sm border bg-sky-500/10 border-sky-500/20 text-sky-400 shrink-0"
        title="Nota Markdown"
      >
        <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth="2.5"
            d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"
          />
        </svg>
      </div>
    );
  };

  return (
    <div className="relative inline-flex shrink-0 p-1 overflow-visible">
      {renderBaseIcon()}
      {isVectorizable && (
        <div
          className={`absolute bottom-0.5 right-0.5 w-2 h-2 flex-none aspect-square rounded-full border border-zinc-950 shadow-sm z-30 ${isIndexing ? 'bg-red-500 animate-pulse' : 'bg-emerald-500'
            }`}
          title={isIndexing ? 'Vetorizando (Ollama)...' : 'Vetorização concluída'}
        />
      )}
    </div>
  );
});
