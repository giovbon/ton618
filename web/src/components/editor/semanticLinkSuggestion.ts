import { ReactRenderer } from "@tiptap/react";
import tippy, { Instance as TippyInstance } from "tippy.js";
import { SemanticLinkSuggestionList } from "./SemanticLinkSuggestionUI";
import type { MutableRefObject } from "preact/compat";

export interface SemanticSuggestionItem {
  label: string;
  type: "topic" | "note";
}

export function getSemanticLinkSuggestionConfig(
  topicsRef: MutableRefObject<string[]>,
  notesRef: MutableRefObject<string[]>
) {
  return {
    char: "@[",
    allowSpaces: true,
    startOfLine: false,

    items: ({ query }: { query: string }): SemanticSuggestionItem[] => {
      const q = query.toLowerCase();

      const topicMatches: SemanticSuggestionItem[] = (topicsRef.current || [])
        .filter((t) => t.toLowerCase().includes(q))
        .slice(0, 6)
        .map((t) => ({ label: t, type: "topic" }));

      const topicLabels = new Set(topicMatches.map((i) => i.label));

      const noteMatches: SemanticSuggestionItem[] = (notesRef.current || [])
        .filter((n) => n.toLowerCase().includes(q) && !topicLabels.has(n))
        .slice(0, 4)
        .map((n) => ({ label: n, type: "note" }));

      return [...topicMatches, ...noteMatches];
    },

    command: ({ editor, range, props }: any) => {
      const item: SemanticSuggestionItem = props;
      editor
        .chain()
        .focus()
        .insertContentAt(range, {
          type: "semanticlink",
          attrs: { topic: item.label },
        })
        // Espaço após o atom → cursor fora do node, Enter funciona normalmente
        .insertContent(" ")
        .run();
    },

    render: () => {
      let component: ReactRenderer;
      let popup: TippyInstance[];

      return {
        onStart: (props: any) => {
          component = new ReactRenderer(SemanticLinkSuggestionList, {
            props,
            editor: props.editor,
          });

          popup = tippy(props.editor.view.dom, {
            getReferenceClientRect: props.clientRect,
            appendTo: () =>
              props.editor.view.dom.closest(".editor-main") || document.body,
            content: component.element,
            showOnCreate: true,
            interactive: true,
            trigger: "manual",
            placement: "bottom-start",
            zIndex: 10,
          });
        },

        onUpdate(props: any) {
          component.updateProps(props);
          if (!props.clientRect) {
            popup?.[0]?.hide();
            return;
          }
          popup?.[0]?.setProps({ getReferenceClientRect: props.clientRect });
          popup?.[0]?.show();
        },

        onKeyDown(props: any) {
          if (props.event.key === "Escape") {
            popup?.[0]?.hide();
            return true;
          }
          return (component.ref as any)?.onKeyDown(props) ?? false;
        },

        onExit() {
          popup?.[0]?.hide();
          popup?.[0]?.destroy();
          component?.destroy();
        },
      };
    },
  };
}
