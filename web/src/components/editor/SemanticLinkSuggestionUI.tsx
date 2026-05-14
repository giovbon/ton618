import { forwardRef, useEffect, useImperativeHandle, useState } from "preact/compat";
import type { SemanticSuggestionItem } from "./semanticLinkSuggestion";

export const SemanticLinkSuggestionList = forwardRef((props: any, ref) => {
  const [selectedIndex, setSelectedIndex] = useState(0);
  const items: SemanticSuggestionItem[] = props.items || [];

  const selectItem = (index: number) => {
    const item = items[index];
    if (item) props.command(item);
  };

  useEffect(() => setSelectedIndex(0), [items]);

  useImperativeHandle(ref, () => ({
    onKeyDown: ({ event }: any) => {
      if (event.key === "ArrowUp") {
        setSelectedIndex((i) => (i + items.length - 1) % items.length);
        return true;
      }
      if (event.key === "ArrowDown") {
        setSelectedIndex((i) => (i + 1) % items.length);
        return true;
      }
      if (event.key === "Enter") {
        selectItem(selectedIndex);
        return true;
      }
      return false;
    },
  }));

  if (items.length === 0) {
    return (
      <div className="bg-zinc-900 border border-zinc-800 shadow-xl rounded-xl p-3 text-xs text-zinc-500 font-medium min-w-[200px]">
        Nenhum tópico ou nota encontrado
      </div>
    );
  }

  // Separa tópicos e notas para exibir com headers
  const topics = items.filter((i) => i.type === "topic");
  const notes = items.filter((i) => i.type === "note");

  let globalIndex = 0;

  const renderItem = (item: SemanticSuggestionItem, idx: number) => {
    const isSel = idx === selectedIndex;
    return (
      <button
        key={item.label}
        className={`text-left px-3 py-2 rounded-lg text-sm font-medium transition-colors w-full flex items-center gap-2 ${
          isSel
            ? "bg-violet-500/20 text-violet-300"
            : "text-zinc-300 hover:bg-zinc-800/50"
        }`}
        onClick={() => selectItem(idx)}
      >
        {item.type === "topic" ? (
          <span className="text-[10px] font-bold text-violet-400 bg-violet-500/10 px-1 rounded">@</span>
        ) : (
          <span className="text-[10px] font-bold text-sky-400 bg-sky-500/10 px-1 rounded">[[</span>
        )}
        {item.label}
      </button>
    );
  };

  return (
    <div className="bg-zinc-900/95 backdrop-blur-xl border border-zinc-700/50 shadow-2xl rounded-xl p-1 flex flex-col gap-0.5 min-w-[220px] max-h-[300px] overflow-y-auto custom-scrollbar">
      {topics.length > 0 && (
        <>
          <div className="px-3 py-1 text-[9px] font-bold text-zinc-500 uppercase tracking-widest">
            Tópicos
          </div>
          {topics.map((item) => renderItem(item, globalIndex++))}
        </>
      )}
      {notes.length > 0 && (
        <>
          <div className="px-3 py-1 text-[9px] font-bold text-zinc-500 uppercase tracking-widest border-t border-zinc-800 mt-1 pt-2">
            Notas
          </div>
          {notes.map((item) => renderItem(item, globalIndex++))}
        </>
      )}
    </div>
  );
});
