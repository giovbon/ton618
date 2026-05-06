import { ReactRenderer } from '@tiptap/react';
import tippy, { Instance as TippyInstance } from 'tippy.js';
import { WikiLinkSuggestionList } from './WikiLinkSuggestionUI';

export function getSuggestionConfig(notesRef: React.MutableRefObject<string[]>) {
  return {
    char: '[[',
    allowSpaces: true,

    items: ({ query }: { query: string }) => {
      const allNotes = notesRef.current || [];
      return allNotes
        .filter(item => item.toLowerCase().includes(query.toLowerCase()))
        .slice(0, 10);
    },

    command: ({ editor, range, props }: any) => {
      editor
        .chain()
        .focus()
        .insertContentAt(range, {
          type: 'wikilink',
          attrs: { title: props.id },
        })
        .insertContent(' ')
        .run();
    },

    render: () => {
      let component: ReactRenderer;
      let popup: TippyInstance[];

      return {
        onStart: (props: any) => {
          component = new ReactRenderer(WikiLinkSuggestionList, {
            props,
            editor: props.editor,
          });

          popup = tippy(props.editor.view.dom, {
            getReferenceClientRect: props.clientRect,
            appendTo: () => props.editor.view.dom.closest('.editor-main') || document.body,
            content: component.element,
            showOnCreate: true,
            interactive: true,
            trigger: 'manual',
            placement: 'bottom-start',
            zIndex: 10,
          });
        },

        onUpdate(props: any) {
          component.updateProps(props);

          if (!props.clientRect) {
            popup?.[0]?.hide();
            return;
          }

          if (popup && popup[0]) {
            popup[0].setProps({
              getReferenceClientRect: props.clientRect,
            });
            popup[0].show();
          }
        },

        onKeyDown(props: any) {
          if (props.event.key === 'Escape') {
            popup?.[0]?.hide();
            return true;
          }
          return component.ref?.onKeyDown(props);
        },

        onExit() {
          if (popup && popup[0]) {
            popup[0].hide();
            popup[0].destroy();
          }
          if (component) {
            component.destroy();
          }
        },
      };
    },
  };
}
