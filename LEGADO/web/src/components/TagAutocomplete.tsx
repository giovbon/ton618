import { useEffect, useRef } from 'preact/hooks';

interface TagAutocompleteProps {
  active: boolean;
  matches: string[];
  selectedIndex: number;
  queryText: string;
  onSelect: (tag: string) => void;
  onClose: () => void;
}

/**
 * Componente de Autocomplete de Tags blindado.
 */
export function TagAutocomplete({
  active,
  matches,
  selectedIndex,
  queryText,
  onSelect,
  onClose,
}: TagAutocompleteProps) {
  const dropdownRef = useRef<HTMLUListElement>(null);

  useEffect(() => {
    function handleClickOutside(event: MouseEvent) {
      if (dropdownRef.current && !dropdownRef.current.contains(event.target as Node)) {
        onClose();
      }
    }
    if (active) {
      document.addEventListener('mousedown', handleClickOutside);
    }
    return () => {
      document.removeEventListener('mousedown', handleClickOutside);
    };
  }, [active, onClose]);

  // Scroll automático para manter o item selecionado visível
  useEffect(() => {
    if (active && dropdownRef.current) {
      const selectedElement = dropdownRef.current.children[selectedIndex] as HTMLElement;
      if (selectedElement) {
        selectedElement.scrollIntoView({ block: 'nearest' });
      }
    }
  }, [selectedIndex, active]);

  if (!active) return null;

  if (matches.length === 0) {
    return (
      <div className="absolute top-full left-0 right-0 mt-2 bg-zinc-900 border border-zinc-700/80 rounded-xl shadow-2xl z-[100] p-4 text-center backdrop-blur-md">
        <p className="text-zinc-500 text-sm italic">Nenhuma tag indexada ainda...</p>
        <p className="text-zinc-600 text-[10px] mt-1">Adicione #tag em suas notas para começar</p>
      </div>
    );
  }

  return (
    <ul
      ref={dropdownRef}
      className="absolute top-full left-0 right-0 mt-2 bg-zinc-900 border border-zinc-700/80 rounded-xl shadow-2xl z-[100] overflow-y-auto overflow-x-hidden max-h-64 divide-y divide-zinc-800/50 backdrop-blur-md animate-in fade-in slide-in-from-top-2 duration-200"
    >
      {matches.map((match, idx) => (
        <li
          key={match}
          onClick={() => onSelect(match)}
          className={`px-4 py-3 cursor-pointer flex items-center gap-2 transition-all border-l-4 ${
            idx === selectedIndex
              ? 'bg-sky-500/15 text-sky-400 border-sky-500'
              : 'text-zinc-300 hover:bg-zinc-800/80 hover:text-zinc-100 border-transparent'
          }`}
        >
          <span className="font-bold tracking-wide leading-none pt-0.5 flex-1">
            <span className="text-sky-500/80">#</span>
            {match.substring(0, queryText.length)}
            <span className="opacity-40 font-medium">{match.substring(queryText.length)}</span>
          </span>
          {idx === selectedIndex && (
            <span className="text-[10px] font-mono opacity-30 uppercase tracking-tighter">
              Enter to select
            </span>
          )}
        </li>
      ))}
    </ul>
  );
}
