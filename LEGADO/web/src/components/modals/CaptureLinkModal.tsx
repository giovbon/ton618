import { useEffect, useRef, useState } from 'preact/hooks';

interface CaptureLinkModalProps {
  isOpen: boolean;
  isProcessing: boolean;
  onClose: () => void;
  onSubmit: (url: string) => void;
}

export function CaptureLinkModal({
  isOpen,
  isProcessing,
  onClose,
  onSubmit,
}: CaptureLinkModalProps) {
  const [url, setUrl] = useState('');
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (isOpen) {
      setUrl('');
      setTimeout(() => inputRef.current?.focus(), 100);
    }
  }, [isOpen]);

  if (!isOpen) return null;

  const handleSubmit = (e: any) => {
    if (e) e.preventDefault();
    if (!url.trim() || isProcessing) return;
    onSubmit(url.trim());
  };

  return (
    <div className="fixed inset-0 z-[200] flex items-center justify-center p-4 animate-in fade-in duration-300">
      <div
        className="absolute inset-0 bg-black/60 backdrop-blur-sm"
        onClick={() => !isProcessing && onClose()}
      />
      <div className="bg-zinc-900 border border-zinc-800 p-8 rounded-3xl shadow-2xl relative z-10 w-full max-w-md animate-in zoom-in slide-in-from-bottom-4">
        <h2 className="text-xl font-black text-zinc-100 mb-2 tracking-tight uppercase flex items-center gap-2">
          <svg
            className="w-6 h-6 text-amber-500"
            fill="none"
            stroke="currentColor"
            viewBox="0 0 24 24"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth="2.5"
              d="M13.828 10.172a4 4 0 00-5.656 0l-4 4a4 4 0 105.656 5.656l1.102-1.101m-.758-4.899a4 4 0 005.656 0l4-4a4 4 0 00-5.656-5.656l-1.1 1.1"
            />
          </svg>
          Capturar Link
        </h2>
        <p className="text-zinc-500 text-xs mb-6 leading-relaxed">
          Cole a URL de um artigo ou youtube para extrair o conteúdo e salvar em Markdown.
        </p>
        <form onSubmit={handleSubmit}>
          <div className="relative mb-6">
            <input
              ref={inputRef}
              type="url"
              value={url}
              onInput={(e: any) => setUrl(e.target.value)}
              placeholder="https://exemplo.com/artigo-para-salvar"
              className="w-full bg-zinc-950 border border-zinc-800 rounded-xl px-4 py-4 text-amber-500 font-medium text-sm focus:ring-2 focus:ring-amber-500 outline-none transition-all placeholder:text-zinc-700 shadow-inner"
            />
          </div>
          <div className="flex gap-3">
            <button
              type="submit"
              disabled={isProcessing || !url.trim()}
              className={`flex-1 py-4 rounded-xl bg-amber-500 text-zinc-950 font-black text-[10px] tracking-widest uppercase hover:bg-amber-400 transition-all active:scale-95 shadow-lg shadow-amber-500/20 ${isProcessing ? 'opacity-50 cursor-wait' : ''}`}
            >
              {isProcessing ? 'Extraindo texto...' : 'Capturar Artigo'}
            </button>
            <button
              type="button"
              onClick={() => !isProcessing && onClose()}
              className="px-6 py-4 rounded-xl bg-zinc-800 text-zinc-400 font-bold text-[10px] tracking-widest uppercase hover:bg-zinc-700 transition-all"
            >
              Cancelar
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
