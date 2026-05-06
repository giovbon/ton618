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
}: EditorHeaderProps) => {
  return (
    <header className="flex items-center justify-between gap-2 px-3 py-2 sm:p-2.5 border-b border-zinc-700/60 bg-zinc-900 relative z-20">
      <div className="flex items-center gap-3 min-w-0 flex-1">
        <div className="p-1.5 bg-sky-500/10 rounded-xl text-sky-400 shrink-0">
          <svg
            className="w-4 h-4 sm:w-5 sm:h-5"
            fill="none"
            stroke="currentColor"
            viewBox="0 0 24 24"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth="2.5"
              d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"
            />
          </svg>
        </div>

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
