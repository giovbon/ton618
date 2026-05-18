import { mergeAttributes, Node } from '@tiptap/core';

export const WikiLinkNode = Node.create({
  name: 'wikilink',
  group: 'inline',
  inline: true,
  selectable: false,
  atom: true,

  addAttributes() {
    return {
      title: {
        default: null,
        parseHTML: element => element.getAttribute('data-wikilink'),
      },
    };
  },

  addStorage() {
    return {
      markdown: {
        serialize(state: any, node: any) {
          state.write(`[[${node.attrs.title}]]`);
        },
      },
    };
  },

  parseHTML() {
    return [
      {
        tag: 'span[data-wikilink]',
      },
    ];
  },

  renderHTML({ HTMLAttributes }) {
    return [
      'span',
      mergeAttributes(HTMLAttributes, {
        'data-wikilink': '',
        class: 'cm-wikilink-pill cursor-pointer text-sky-400 bg-sky-500/10 px-1.5 py-0.5 rounded-md border border-sky-500/20 font-bold',
      }),
      `[[${HTMLAttributes.title}]]`,
    ];
  },

  addNodeView() {
    return (props) => {
      const dom = document.createElement('span');
      dom.classList.add('cm-wikilink-pill', 'cursor-pointer', 'text-sky-400', 'bg-sky-500/10', 'px-1.5', 'py-0.5', 'rounded-md', 'border', 'border-sky-500/20', 'font-bold', 'hover:bg-sky-500/20', 'transition-colors');
      dom.setAttribute('data-wikilink', props.node.attrs.title);
      dom.innerText = `[[${props.node.attrs.title}]]`;

      dom.onclick = () => {
        // Trigger global event or custom logic to open note
        window.dispatchEvent(new CustomEvent('open-note', { detail: props.node.attrs.title }));
      };

      return {
        dom,
      };
    };
  },
});
