import { Mark, mergeAttributes } from "@tiptap/core";

export const SemanticLinkMark = Mark.create({
  name: "semanticlink",

  inclusive: false,

  addOptions() {
    return {
      HTMLAttributes: {
        class: "",
      },
    };
  },

  parseHTML() {
    return [
      {
        tag: "span[data-semantic-link]",
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
        // Detecta @[texto]
        find: /@\[([^\]]+)\]$/,
        handler: ({ state, range, match }) => {
          const { tr } = state;
          const start = range.from;
          const end = range.to;

          tr.addMark(
            start,
            end,
            this.type.create({
              "data-semantic-link": match[1],
            }),
          );
        },
      },
    ];
  },

  addPasteRules() {
    return [
      {
        undoable: true,
        // Detecta @[texto] em texto colado
        find: /@\[([^\]]+)\]/g,
        handler: ({ state, range, match }) => {
          const { tr } = state;
          const start = range.from + match.index!;
          const end = start + match[0].length;

          tr.addMark(
            start,
            end,
            this.type.create({
              "data-semantic-link": match[1],
            }),
          );
        },
      },
    ];
  },
});
