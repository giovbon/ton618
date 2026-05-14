import { mergeAttributes, Node } from "@tiptap/core";

/**
 * SemanticLinkNode — implementado como Node atom (igual ao WikiLink),
 * garantindo:
 * 1. Formatação visual estável (pill violeta)
 * 2. Serialização markdown robusta via storage.markdown.serialize
 * 3. Sem bloqueio de linha: o comando de inserção sempre adiciona um
 *    espaço de texto simples após o node, então o cursor cai nesse espaço
 *    e o Enter funciona normalmente.
 * 4. Click para abrir o Mapa Estruturado focado no tópico
 */
export const SemanticLinkNode = Node.create({
  name: "semanticlink",
  group: "inline",
  inline: true,
  selectable: true,  // permite clicar para selecionar e Backspace para deletar
  atom: true,        // sem cursor interno → nunca bloqueia Enter

  addAttributes() {
    return {
      topic: {
        default: null,
        parseHTML: (element) => element.getAttribute("data-semantic-link"),
        renderHTML: (attrs) => ({ "data-semantic-link": attrs.topic }),
      },
    };
  },

  addStorage() {
    return {
      markdown: {
        serialize(state: any, node: any) {
          state.write(`@[${node.attrs.topic}]`);
        },
      },
    };
  },

  parseHTML() {
    return [{ tag: "span[data-semantic-link]" }];
  },

  renderHTML({ HTMLAttributes }) {
    return [
      "span",
      mergeAttributes(HTMLAttributes, {
        class:
          "semantic-link-pill cursor-pointer text-violet-400 bg-violet-500/10 px-1.5 py-0.5 rounded-md border border-violet-500/20 font-bold hover:bg-violet-500/20 transition-colors select-none",
      }),
      `@[${HTMLAttributes["data-semantic-link"]}]`,
    ];
  },

  addNodeView() {
    return ({ node, getPos }: any) => {
      const dom = document.createElement("span");
      dom.className =
        "semantic-link-pill cursor-pointer text-violet-400 bg-violet-500/10 px-1.5 py-0.5 rounded-md border border-violet-500/20 font-bold hover:bg-violet-500/20 transition-colors select-none";
      dom.setAttribute("data-semantic-link", node.attrs.topic ?? "");
      dom.innerText = `@[${node.attrs.topic}]`;
      dom.title = "Clique para editar";

      // 1 clique → abre o editor flutuante no TiptapEditor (fora do DOM ProseMirror)
      dom.onclick = (e: MouseEvent) => {
        e.preventDefault();
        e.stopPropagation();
        const pos = typeof getPos === "function" ? getPos() : null;
        if (pos === null) return;
        window.dispatchEvent(
          new CustomEvent("semantic-link-edit", {
            detail: { topic: node.attrs.topic, pos, rect: dom.getBoundingClientRect() },
          })
        );
      };

      // 2 cliques → abre o Mapa Estruturado
      dom.ondblclick = (e: MouseEvent) => {
        e.preventDefault();
        e.stopPropagation();
        window.dispatchEvent(
          new CustomEvent("open-semantic-topic", { detail: node.attrs.topic })
        );
      };

      return { dom };
    };
  },
});
