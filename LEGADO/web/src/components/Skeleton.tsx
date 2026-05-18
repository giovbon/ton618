/**
 * Skeleton - Componente de placeholder para carregamento
 */
export const Skeleton = ({ className }: { className?: string }) => (
  <div className={`animate-pulse space-y-4 ${className || ''}`}>
    <div className="flex items-start gap-4 p-4 bg-zinc-900/40 border border-zinc-800/60 rounded-xl">
      <div className="w-10 h-10 bg-zinc-800 rounded-lg shrink-0" />
      <div className="flex-1 space-y-3 py-1">
        <div className="h-2.5 bg-zinc-800 rounded w-1/4" />
        <div className="space-y-2">
          <div className="h-2 bg-zinc-800 rounded" />
          <div className="h-2 bg-zinc-800 rounded w-5/6" />
        </div>
      </div>
    </div>
  </div>
);
