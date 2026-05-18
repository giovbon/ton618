import { useEffect, useRef, useState } from 'preact/hooks';

interface CreateNoteModalProps {
  isOpen: boolean;
  onClose: () => void;
  onSubmit: (title: string) => void;
}

export function CreateNoteModal({ isOpen, onClose, onSubmit }: CreateNoteModalProps) {
  const [fileName, setFileName] = useState('');
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (isOpen) {
      setFileName('');
      setTimeout(() => inputRef.current?.focus(), 100);
    }
  }, [isOpen]);

  if (!isOpen) return null;

  const handleSubmit = (e: any) => {
    if (e) e.preventDefault();
    const title = fileName.trim();
    if (!title) return;
    onSubmit(title);
    setFileName('');
  };

  return (
    <div className="fixed inset-0 z-[200] flex items-center justify-center p-4 animate-in fade-in duration-300">
      <div className="absolute inset-0 bg-black/60 backdrop-blur-sm" onClick={onClose} />
      <div className="bg-zinc-900 border border-zinc-800 p-8 rounded-3xl shadow-2xl relative z-10 w-full max-w-md animate-in zoom-in slide-in-from-bottom-4">
        <h2 className="text-xl font-black text-zinc-100 mb-6 tracking-tight uppercase">
          Criar Nova Nota
        </h2>
        <form onSubmit={handleSubmit}>
          <div className="relative mb-6">
            <input
              ref={inputRef}
              type="text"
              value={fileName}
              onInput={(e: any) => setFileName(e.target.value)}
              placeholder="nome-da-nota"
              className="w-full bg-zinc-950 border border-zinc-800 rounded-xl px-4 py-4 text-zinc-100 font-mono text-sm focus:ring-2 focus:ring-sky-500 outline-none transition-all placeholder:text-zinc-700"
            />
            <span className="absolute right-4 top-4 text-zinc-600 font-mono text-sm">.md</span>
          </div>
          <div className="flex gap-3">
            <button
              type="submit"
              disabled={!fileName.trim()}
              className="flex-1 py-3 rounded-xl bg-sky-500 text-sky-950 font-black text-xs uppercase hover:bg-sky-400 disabled:opacity-50 disabled:cursor-not-allowed"
            >
              Criar Agora
            </button>
            <button
              type="button"
              onClick={onClose}
              className="px-6 py-3 rounded-xl bg-zinc-800 text-zinc-400 font-medium text-xs uppercase hover:bg-zinc-700"
            >
              Cancelar
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
