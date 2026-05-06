import { ReactRenderer } from "@tiptap/react";
import tippy, { Instance as TippyInstance } from "tippy.js";
import { SlashCommandList } from "./SlashCommandUI";

export interface CommandItem {
  title: string;
  description: string;
  icon: any; // We'll pass an SVG string or Preact component
  command: ({ editor, range }: { editor: any; range: any }) => void;
}

export function getSlashCommandConfig() {
  return {
    char: "/",
    allowSpaces: false,
    startOfLine: false,

    items: ({ query }: { query: string }) => {
      const commands: CommandItem[] = [
        {
          title: "Texto",
          description: "Texto normal para parágrafos.",
          icon: '<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2.5" d="M4 6h16M4 12h16m-7 6h7" /></svg>',
          command: ({ editor, range }) => {
            editor.chain().focus().deleteRange(range).setParagraph().run();
          },
        },
        {
          title: "Título 1",
          description: "Cabeçalho principal (H1).",
          icon: '<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2.5" d="M3 5h2m0 0v14m0-14h2M17 5h2m0 0v14m0-14h2M5 12h14" /></svg>',
          command: ({ editor, range }) => {
            editor
              .chain()
              .focus()
              .deleteRange(range)
              .setNode("heading", { level: 1 })
              .run();
          },
        },
        {
          title: "Título 2",
          description: "Cabeçalho secundário (H2).",
          icon: '<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2.5" d="M4 5h2m0 0v14m0-14h2M16 5h2a2 2 0 012 2v2a2 2 0 01-2 2h-4v7m0 0h6M4 12h8" /></svg>',
          command: ({ editor, range }) => {
            editor
              .chain()
              .focus()
              .deleteRange(range)
              .setNode("heading", { level: 2 })
              .run();
          },
        },
        {
          title: "Título 3",
          description: "Sub-cabeçalho (H3).",
          icon: '<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2.5" d="M4 5h2m0 0v14m0-14h2M15 5h2a2 2 0 012 2v2a2 2 0 01-2 2h-4m4 0a2 2 0 012 2v2a2 2 0 01-2 2h-4v-7M4 12h8" /></svg>',
          command: ({ editor, range }) => {
            editor
              .chain()
              .focus()
              .deleteRange(range)
              .setNode("heading", { level: 3 })
              .run();
          },
        },
        {
          title: "Lista",
          description: "Lista com marcadores.",
          icon: '<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2.5" d="M4 6h16M4 12h16M4 18h16" /></svg>',
          command: ({ editor, range }) => {
            editor.chain().focus().deleteRange(range).toggleBulletList().run();
          },
        },
        {
          title: "Lista Numerada",
          description: "Lista ordenada.",
          icon: '<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2.5" d="M7 6h14M7 12h14M7 18h14M3 6h.01M3 12h.01M3 18h.01" /></svg>',
          command: ({ editor, range }) => {
            editor.chain().focus().deleteRange(range).toggleOrderedList().run();
          },
        },
        {
          title: "Tarefa",
          description: "Lista de tarefas (checkbox).",
          icon: '<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2.5" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" /></svg>',
          command: ({ editor, range }) => {
            editor.chain().focus().deleteRange(range).toggleTaskList().run();
          },
        },
        {
          title: "Bloco de Código",
          description: "Área para código-fonte.",
          icon: '<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2.5" d="M10 20l4-16m4 4l4 4-4 4M6 16l-4-4 4-4" /></svg>',
          command: ({ editor, range }) => {
            editor.chain().focus().deleteRange(range).toggleCodeBlock().run();
          },
        },
        {
          title: "Citação",
          description: "Bloco de citação.",
          icon: '<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2.5" d="M8 10h4L9 14h4m-5 4h4" /></svg>',
          command: ({ editor, range }) => {
            editor.chain().focus().deleteRange(range).toggleBlockquote().run();
          },
        },
        {
          title: "Divisor",
          description: "Linha horizontal.",
          icon: '<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2.5" d="M4 12h16" /></svg>',
          command: ({ editor, range }) => {
            editor.chain().focus().deleteRange(range).setHorizontalRule().run();
          },
        },
        {
          title: "Tabela 3x3",
          description: "Inserir tabela tabela (2x2 a 5x5).",
          icon: '<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2.5" d="M3 10h18M3 14h18m-9-4v8m-7 0h14a2 2 0 002-2V8a2 2 0 00-2-2H5a2 2 0 00-2 2v8a2 2 0 002 2z"/></svg>',
          command: ({ editor, range }) => {
            editor.chain().focus().deleteRange(range).insertTable({ rows: 3, cols: 3, withHeaderRow: true }).run();
          },
        }
      ];

      return commands
        .filter((item) =>
          item.title.toLowerCase().includes(query.toLowerCase()),
        )
        .slice(0, 10);
    },

    command: ({ editor, range, props }: any) => {
      props.command({ editor, range });
    },

    render: () => {
      let component: ReactRenderer;
      let popup: TippyInstance[];

      return {
        onStart: (props: any) => {
          component = new ReactRenderer(SlashCommandList, {
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

          if (popup && popup[0]) {
            popup[0].setProps({
              getReferenceClientRect: props.clientRect,
            });
            popup[0].show();
          }
        },

        onKeyDown(props: any) {
          if (props.event.key === "Escape") {
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
