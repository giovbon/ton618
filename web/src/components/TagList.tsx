import { memo, useState } from 'preact/compat';
import { getTagStyle } from '../utils/tagStyles';

interface TagListProps {
  tags: string[];
  query?: string;
}

export const TagList = memo(({ tags }: TagListProps) => {
  const [showAllTags, setShowAllTags] = useState(false);

  if (!tags || tags.length === 0) return null;

  const visibleTags = showAllTags ? tags : tags.slice(0, 10);
  const hasMore = !showAllTags && tags.length > 10;

  return (
    <div className="flex flex-wrap gap-1 items-center mt-2 justify-start w-full">
      {visibleTags.map((tag) => {
        const style = getTagStyle(tag);
        return (
          <span
            key={tag}
            style={{
              backgroundColor: style.bgCol,
              color: style.mainCol,
              borderColor: style.borderCol,
            }}
            className="inline-flex items-center h-6 border font-sans text-[10.5px] font-semibold rounded-lg px-2.5 hover:brightness-110 transition-all cursor-default shadow-sm shrink-0"
            title={tag}
          >
            <span className="whitespace-nowrap">#{tag}</span>
          </span>
        );
      })}
      {hasMore && !showAllTags && (
        <button
          onClick={(e) => {
            e.stopPropagation();
            setShowAllTags(true);
          }}
          className="px-2 py-0.5 bg-sky-500/10 text-sky-400 border border-sky-500/20 rounded-md text-[9px] font-bold hover:bg-sky-500/20 transition-colors"
        >
          + {tags.length - 10} tags
        </button>
      )}
      {showAllTags && (
        <button
          onClick={(e) => {
            e.stopPropagation();
            setShowAllTags(false);
          }}
          className="ml-auto px-2 py-0.5 bg-zinc-800/20 text-zinc-300 border border-zinc-700/50 rounded-full text-[10px] font-semibold hover:bg-zinc-800/40 transition-colors"
        >
          colapsar
        </button>
      )}
    </div>
  );
});

(TagList as any).displayName = 'TagList';
