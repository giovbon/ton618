import { forwardRef, useEffect, useImperativeHandle, useState } from 'preact/compat';

export const WikiLinkSuggestionList = forwardRef((props: any, ref) => {
  const [selectedIndex, setSelectedIndex] = useState(0);

  const selectItem = (index: number) => {
    const item = props.items[index];
    if (item) {
      props.command({ id: item });
    }
  };

  const upHandler = () => {
    setSelectedIndex((selectedIndex + props.items.length - 1) % props.items.length);
  };

  const downHandler = () => {
    setSelectedIndex((selectedIndex + 1) % props.items.length);
  };

  const enterHandler = () => {
    selectItem(selectedIndex);
  };

  useEffect(() => setSelectedIndex(0), [props.items]);

  useImperativeHandle(ref, () => ({
    onKeyDown: ({ event }: any) => {
      if (event.key === 'ArrowUp') {
        upHandler();
        return true;
      }
      if (event.key === 'ArrowDown') {
        downHandler();
        return true;
      }
      if (event.key === 'Enter') {
        enterHandler();
        return true;
      }
      return false;
    },
  }));

  if (!props.items || props.items.length === 0) {
    return (
      <div className="bg-zinc-900 border border-zinc-800 shadow-xl rounded-xl p-2 text-xs text-zinc-500 font-medium">
        Nenhuma nota encontrada...
      </div>
    );
  }

  return (
    <div className="bg-zinc-900/95 backdrop-blur-xl border border-zinc-700/50 shadow-2xl rounded-xl p-1 flex flex-col gap-0.5 min-w-[200px] max-h-[300px] overflow-y-auto custom-scrollbar">
      {props.items.map((item: string, index: number) => (
        <button
          key={index}
          className={`text-left px-3 py-2 rounded-lg text-sm font-medium transition-colors ${
            index === selectedIndex
              ? 'bg-sky-500/20 text-sky-400'
              : 'text-zinc-300 hover:bg-zinc-800/50'
          }`}
          onClick={() => selectItem(index)}
        >
          {item}
        </button>
      ))}
    </div>
  );
});
