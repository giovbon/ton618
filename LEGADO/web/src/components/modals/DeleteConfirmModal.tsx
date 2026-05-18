interface DeleteConfirmModalProps {
  isOpen?: boolean;
  filename: string | null;
  isDeleting: boolean;
  onClose: () => void;
  onConfirm: () => void;
}

export function DeleteConfirmModal({
  filename,
  isDeleting,
  onClose,
  onConfirm,
}: DeleteConfirmModalProps) {
  if (!filename) return null;

  return (
    <div className="fixed inset-0 z-[100] flex items-center justify-center p-4 animate-in fade-in duration-300">
      <div
        className="absolute inset-0 bg-zinc-950/80 backdrop-blur-sm"
        onClick={() => !isDeleting && onClose()}
      ></div>
      <div className="relative bg-zinc-900 border border-zinc-800 w-full max-w-md p-8 rounded-3xl shadow-2xl flex flex-col items-center text-center animate-in zoom-in-95 duration-300">
        <div className="w-20 h-20 bg-red-500/10 rounded-full flex items-center justify-center mb-6 ring-4 ring-red-500/5">
          <svg
            className="w-10 h-10 text-red-500"
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
        </div>

        <h3 className="text-xl font-black text-zinc-100 uppercase tracking-tight mb-2">
          EXCLUIR ARQUIVO?
        </h3>
        <p className="text-zinc-400 text-sm mb-8 leading-relaxed">
          Você está prestes a apagar permanentemente{' '}
          <span className="text-red-400 font-bold font-mono">{filename}</span>.
          <br />
          Esta ação não pode ser desfeita.
        </p>

        <div className="flex flex-col sm:flex-row gap-3 w-full">
          <button
            disabled={isDeleting}
            onClick={onClose}
            className="flex-1 px-6 py-3 rounded-xl bg-zinc-800 text-zinc-400 font-bold text-xs uppercase tracking-widest hover:bg-zinc-700 transition-all active:scale-95"
          >
            Cancelar
          </button>
          <button
            disabled={isDeleting}
            onClick={onConfirm}
            className={`flex-1 px-6 py-3 rounded-xl bg-red-500 text-zinc-950 font-black text-xs uppercase tracking-widest hover:bg-red-400 transition-all active:scale-95 flex items-center justify-center gap-2 ${isDeleting ? 'opacity-50' : ''}`}
          >
            {isDeleting ? 'Excluindo...' : 'Sim, Excluir Agora'}
          </button>
        </div>
      </div>
    </div>
  );
}
