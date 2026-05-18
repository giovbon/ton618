import { Mark, mergeAttributes } from "@tiptap/core";

export const HashtagMark = Mark.create({
  name: "hashtag",

  inclusive: false,

  addOptions() {
    return {
      HTMLAttributes: {
        class: "hashtag-pill",
        "data-hashtag": "true",
      },
    };
  },

  parseHTML() {
    return [
      {
        tag: "span.hashtag-pill",
      },
    ];
  },

  renderHTML({ HTMLAttributes }) {
    return [
      "span",
      mergeAttributes(this.options.HTMLAttributes, HTMLAttributes),
      0,
    ];
  },

  addInputRules() {
    return [
      {
        undoable: true,
        find: /(?:\s|^)#([a-zA-Z0-9_À-ÿ\-]+)\s$/,
        handler: ({ state, range, match }) => {
          const { tr } = state;
          const start = range.from;
          const end = range.to;

          // O match inclui o espaco final (\s$). Excluimos o espaco do range da mark.
          const fullMatch = match[0];
          const hasSpaceAtEnd = fullMatch.endsWith(" ");
          const actualEnd = hasSpaceAtEnd ? end - 1 : end;

          tr.addMark(start, actualEnd, this.type.create());
          // NAO insere espaco extra - o espaco do match ja esta no documento
        },
      },
    ];
  },
});
