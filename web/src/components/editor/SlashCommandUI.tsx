import { forwardRef, useEffect, useImperativeHandle, useRef, useState } from 'preact/compat';
import { CommandItem } from './slashCommandSuggestion';

interface SlashCommandListProps {
  items: CommandItem[];
  command: (item: CommandItem) => void;
}

export const SlashCommandList = forwardRef((props: SlashCommandListProps, ref) => {
  const [selectedIndex, setSelectedIndex] = useState(0);
  const containerRef = useRef<HTMLDivElement>(null);

  useEffect(() => setSelectedIndex(0), [props.items]);

  useEffect(() => {
    if (containerRef.current && selectedIndex >= 0) {
      const parent = containerRef.current;
      const item = parent.children[selectedIndex] as HTMLElement;

      if (item) {
        item.scrollIntoView({
          block: 'nearest',
          inline: 'nearest',
        });
      }
    }
  }, [selectedIndex]);

  useImperativeHandle(ref, () => ({
    onKeyDown: ({ event }: { event: KeyboardEvent }) => {
      if (event.key === 'ArrowUp') {
        setSelectedIndex((selectedIndex + props.items.length - 1) % props.items.length);
        return true;
      }

      if (event.key === 'ArrowDown') {
        setSelectedIndex((selectedIndex + 1) % props.items.length);
        return true;
      }

      if (event.key === 'Enter') {
        selectItem(selectedIndex);
        return true;
      }

      return false;
    },
  }));

  const selectItem = (index: number) => {
    const item = props.items[index];
    if (item) {
      props.command(item);
    }
  };

  if (props.items.length === 0) {
    return null;
  }

  return (
    <div className="bg-zinc-900/95 backdrop-blur-xl border border-zinc-700/50 rounded-xl shadow-[0_10px_40px_-10px_rgba(0,0,0,0.7)] overflow-hidden min-w-[240px] flex flex-col p-1">
      <div className="px-3 py-2 text-[10px] font-black uppercase tracking-widest text-zinc-500 border-b border-zinc-800/50 mb-1">
        Blocos Básicos
      </div>
      <div
        ref={containerRef}
        className="flex flex-col gap-0.5 overflow-y-auto max-h-[300px] custom-scrollbar"
      >
        {props.items.map((item, index) => (
          <button
            key={index}
            className={`flex items-center gap-3 px-3 py-2.5 rounded-lg text-left transition-all ${
              index === selectedIndex
                ? 'bg-sky-500/10 text-sky-400'
                : 'text-zinc-300 hover:bg-zinc-800/50 hover:text-zinc-100'
            }`}
            onClick={() => selectItem(index)}
          >
            <div
              className={`w-8 h-8 rounded-md flex items-center justify-center border transition-all ${
                index === selectedIndex
                  ? 'bg-sky-500/20 border-sky-500/30 text-sky-400'
                  : 'bg-zinc-800/80 border-zinc-700/50 text-zinc-400'
              }`}
              dangerouslySetInnerHTML={{ __html: item.icon }}
            />
            <div className="flex flex-col">
              <span className={`text-sm font-bold ${index === selectedIndex ? 'text-sky-400' : 'text-zinc-200'}`}>
                {item.title}
              </span>
              <span className={`text-[10px] leading-tight mt-0.5 ${index === selectedIndex ? 'text-sky-500/70' : 'text-zinc-500'}`}>
                {item.description}
              </span>
            </div>
          </button>
        ))}
      </div>
    </div>
  );
});
