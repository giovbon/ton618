import { memo, useState } from 'preact/compat';

import type { FileObject, SearchResult } from '../types';

interface ResultActionsProps {
  doc: SearchResult;
  onEdit: (file: FileObject) => void;
  onDeleteFile: (filename: string) => void;
  fetchWithAuth: (url: string, options?: RequestInit) => Promise<Response | null>;
  auth: string | null;
  blobUrl?: string | null;
}

export const ResultActions = memo(
  ({ doc, onEdit, onDeleteFile, fetchWithAuth, auth, blobUrl }: ResultActionsProps) => {
    const [isOpening, setIsOpening] = useState(false);

    const handleEdit = async (e: MouseEvent) => {
      e.stopPropagation();
      if (isOpening) return;
      setIsOpening(true);
      try {
        const res = await fetchWithAuth(`/api/file?name=${encodeURIComponent(doc.arquivo)}`);
        if (res?.ok) {
          const text = await res.text();
          onEdit({
            name: doc.arquivo,
            content: text,
            scrollToText: doc.texto,
          });
        } else {
          alert(`Erro ao abrir nota: ${res?.statusText} (${res?.status})`);
        }
      } catch (e) {
        console.error('Edit error:', e);
        alert('Erro de conexão ao carregar arquivo.');
      } finally {
        setIsOpening(false);
      }
    };

    const handleOpenAsset = () => {
      // Tenta usar blobUrl se disponível, senão usa link direto com token
      const base = blobUrl || `/api/file?name=${encodeURIComponent(doc.arquivo)}&token=${auth}`;
      const url = doc.tipo === 'pdf' ? `${base}#page=${doc.pagina || 1}` : base;
      window.open(url, '_blank');
    };

    return (
      <div className="flex items-center gap-1.5 shrink-0 transition-opacity opacity-100">
        {doc.tipo === 'note' ||
        doc.tipo === 'link' ||
        doc.tipo === 'markdown' ||
        doc.tipo === 'documento' ? (
          <button
            onClick={handleEdit as any}
            disabled={isOpening}
            className={`p-2 rounded-lg bg-zinc-800/50 hover:bg-zinc-800 border transition-all active:scale-95 flex items-center justify-center ${isOpening ? 'cursor-wait opacity-50' : ''} ${doc.arquivo?.startsWith('links/') ? 'text-amber-500/70 hover:text-amber-500 border-amber-500/20' : 'text-sky-400/70 hover:text-sky-400 border-sky-500/20'}`}
            title={isOpening ? 'Carregando...' : 'Editar Nota'}
          >
            {isOpening ? (
              <svg className="w-3.5 h-3.5 animate-spin" viewBox="0 0 24 24">
                <circle
                  className="opacity-25"
                  cx="12"
                  cy="12"
                  r="10"
                  stroke="currentColor"
                  strokeWidth="4"
                ></circle>
                <path
                  className="opacity-75"
                  fill="currentColor"
                  d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                ></path>
              </svg>
            ) : (
              <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth="2"
                  d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z"
                />
              </svg>
            )}
          </button>
        ) : doc.tipo === 'pdf' ? (
          <button
            onClick={handleOpenAsset}
            className={`px-3 py-1.5 rounded-lg transition-all active:scale-95 flex items-center justify-center gap-2 bg-red-500/10 hover:bg-red-500/20 text-red-400 border border-red-500/20`}
            title={`${'Abrir PDF na página'} ${doc.pagina || 1}`}
          >
            <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth="2.5"
                d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14"
              />
            </svg>
            <span className="text-[10px] font-bold uppercase tracking-tight">
              {'Pág.'} {doc.pagina || 1}
            </span>
          </button>
        ) : null}
      </div>
    );
  },
);
