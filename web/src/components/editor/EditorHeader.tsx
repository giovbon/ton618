import { formatDisplayTitle } from '../../utils/search';

interface EditorHeaderProps {
  fileName: string;
  newFileName: string;
  setNewFileName: (name: string) => void;
  isEditingName: boolean;
  setIsEditingName: (val: boolean) => void;
  handleRename: () => void;
  onClose: () => void;
  setShowDeleteConfirm: (val: boolean) => void;
  editorStatus: 'saved' | 'saving' | 'dirty';
  type?: string;
  tags?: string[];
}

export const EditorHeader = ({
  fileName,
  newFileName,
  setNewFileName,
  isEditingName,
  setIsEditingName,
  handleRename,
  onClose,
  setShowDeleteConfirm,
  editorStatus,
  type,
  tags,
}: EditorHeaderProps) => {
  const renderIcon = () => {
    const lowerTags = (tags || []).map((t) => t.toLowerCase());
    const isPDF =
      type === 'pdf' || fileName.toLowerCase().includes('pdfs/') || lowerTags.includes('pdf');
    const isLink = fileName.toLowerCase().includes('links/');
    const isImage = type === 'imagem' || lowerTags.includes('imagem');
    const isDrawing = type === 'desenho' || lowerTags.includes('desenho');

    if (isPDF) {
      return (
        <div className="p-1.5 bg-red-500/10 rounded-xl text-red-500 shrink-0">
          <svg className="w-4 h-4 sm:w-5 sm:h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
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

    if (isLink) {
      return (
        <div className="p-1.5 bg-amber-500/10 rounded-xl text-amber-500 shrink-0">
          <svg className="w-4 h-4 sm:w-5 sm:h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
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

    if (isImage) {
      return (
        <div className="p-1.5 bg-emerald-500/10 rounded-xl text-emerald-400 shrink-0">
          <svg className="w-4 h-4 sm:w-5 sm:h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
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

    if (isDrawing) {
      return (
        <div className="p-1.5 bg-purple-500/10 rounded-xl text-purple-400 shrink-0">
          <svg className="w-4 h-4 sm:w-5 sm:h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
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
      <div className="p-1.5 bg-sky-500/10 rounded-xl text-sky-400 shrink-0">
        <svg className="w-4 h-4 sm:w-5 sm:h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
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
    <header className="flex items-center justify-between gap-2 px-3 py-2 sm:p-2.5 border-b border-zinc-700/60 bg-zinc-900 relative z-20">
      <div className="flex items-center gap-3 min-w-0 flex-1">
        {renderIcon()}

        <div className="flex flex-col min-w-0 flex-1">
          {isEditingName ? (
            <div className="flex items-center gap-2">
              <input
                type="text"
                value={newFileName.split('/').pop()?.replace(/\.md$/, '') || newFileName}
                onChange={(e: any) => setNewFileName(e.target.value)}
                onKeyDown={(e: any) => e.key === 'Enter' && handleRename()}
                className="bg-zinc-800 border border-zinc-700 rounded-lg px-3 py-1.5 text-zinc-100 font-mono text-xs focus:ring-2 focus:ring-sky-500 outline-none w-48"
              />
              <button onClick={handleRename} className="text-sky-500 hover:text-sky-400">
                <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth="3"
                    d="M5 13l4 4L19 7"
                  />
                </svg>
              </button>
            </div>
          ) : (
            <h2
              className="text-sm font-bold text-zinc-100 truncate cursor-pointer hover:text-sky-400 transition-colors"
              onClick={() => setIsEditingName(true)}
            >
              {formatDisplayTitle(fileName)}
            </h2>
          )}
          <div className="flex items-center gap-2 mt-0.5">
            <span className="text-[9px] text-zinc-500 font-mono uppercase tracking-widest">
              Editor de Notas
            </span>
            <span
              className={`text-[8px] font-bold uppercase tracking-tighter px-1.5 py-0.5 rounded transition-all duration-300 ${
                editorStatus === 'saved'
                  ? 'text-emerald-500 bg-emerald-500/5'
                  : editorStatus === 'saving'
                    ? 'text-sky-400 bg-sky-400/10 animate-pulse'
                    : 'text-amber-500 bg-amber-500/5'
              }`}
            >
              {editorStatus === 'saved'
                ? `• ${'Salvo'}`
                : editorStatus === 'saving'
                  ? `• ${'Salvando...'}`
                  : `• ${'Auto-save ativo'}`}
            </span>
          </div>
        </div>
      </div>

      <div className="flex items-center gap-2 shrink-0">
        <button
          onClick={() => setShowDeleteConfirm(true)}
          className="p-2 text-zinc-500 hover:text-red-400 hover:bg-red-500/10 rounded-xl transition-all border border-transparent hover:border-red-500/30"
          title={'Excluir Nota?'}
        >
          <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth="2.5"
              d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"
            />
          </svg>
        </button>

        <button
          onClick={onClose}
          className="p-2 text-zinc-500 hover:text-zinc-100 hover:bg-zinc-800 rounded-xl transition-all border border-transparent hover:border-zinc-700"
          title="Fechar"
        >
          <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth="2.5"
              d="M6 18L18 6M6 6l12 12"
            />
          </svg>
        </button>
      </div>
    </header>
  );
};
