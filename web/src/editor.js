import { Editor } from "@tiptap/core";
import StarterKit from "@tiptap/starter-kit";
import Placeholder from "@tiptap/extension-placeholder";
import { Table } from "@tiptap/extension-table";
import { TableRow } from "@tiptap/extension-table-row";
import { TableCell } from "@tiptap/extension-table-cell";
import { TableHeader } from "@tiptap/extension-table-header";
import ImageExt from "@tiptap/extension-image";
import TaskList from "@tiptap/extension-task-list";
import TaskItem from "@tiptap/extension-task-item";
import Underline from "@tiptap/extension-underline";
import Highlight from "@tiptap/extension-highlight";
import Link from "@tiptap/extension-link";

const CustomLink = Link.extend({
  inclusive: false,
});
import { Markdown } from "tiptap-markdown";
import { marked } from "marked";
import CodeBlockLowlightExt from "@tiptap/extension-code-block-lowlight";
import { createLowlight, common } from "lowlight";
import Paragraph from "@tiptap/extension-paragraph";

const lowlight = createLowlight(common);

// Extensão customizada que adiciona data-language no <pre> para exibir o label
const CodeBlockLangLabel = CodeBlockLowlightExt.extend({
  renderHTML({ node, HTMLAttributes }) {
    return [
      "pre",
      {
        ...HTMLAttributes,
        ...(node.attrs.language
          ? { "data-language": node.attrs.language }
          : {}),
      },
      [
        "code",
        {
          class: node.attrs.language
            ? `language-${node.attrs.language}`
            : null,
        },
        0,
      ],
    ];
  },
});

// Extensão customizada do Paragraph que serializa parágrafos vazios como &nbsp;
const CustomParagraph = Paragraph.extend({
  addStorage() {
    return {
      markdown: {
        serialize(state, node) {
          if (node.content.size === 0) {
            state.write("&nbsp;");
            state.closeBlock(node);
          } else {
            state.renderInline(node);
            state.closeBlock(node);
          }
        },
        parse: {
          // handled by markdown-it
        },
      },
    };
  },
});

/**
 * @namespace TipTapEditor
 * @description Módulo de configuração do editor TipTap.
 * Expõe todas as extensões e utilitários necessários para inicializar
 * o editor de markdown no browser.
 * 
 * @property {Object} Editor - Classe principal do TipTap
 * @property {Object} StarterKit - Extensão base (negrito, itálico, listas, etc.)
 * @property {Object} Placeholder - Placeholder para campos vazios
 * @property {Object} Table - Tabelas
 * @property {Object} TableRow - Linha de tabela
 * @property {Object} TableCell - Célula de tabela
 * @property {Object} TableHeader - Cabeçalho de tabela
 * @property {Object} ImageExt - Imagens
 * @property {Object} TaskList - Lista de tarefas
 * @property {Object} TaskItem - Item de tarefa
 * @property {Object} Underline - Sublinhado
 * @property {Object} TextStyle - Estilo de texto
 * @property {Object} FontFamily - Família de fonte
 * @property {Object} Highlight - Marcação (highlights)
 * @property {Object} Link - Links (customizado, inclusive: false)
 * @property {Object} Markdown - Parse/serialização markdown
 * @property {Object} marked - Biblioteca marked
 * @property {Object} CodeBlockLowlightExt - Bloco de código com syntax highlight
 * @property {Object} lowlight - Instância lowlight para highlight
 * @property {Object} CustomParagraph - Parágrafo customizado (&nbsp;)
 */
window.TipTapEditor = {
  Editor,
  StarterKit,
  Placeholder,
  Table,
  TableRow,
  TableCell,
  TableHeader,
  ImageExt,
  TaskList,
  TaskItem,
  Underline,
  Highlight,
  Link: CustomLink,
  Markdown,
  marked,
  CodeBlockLowlightExt: CodeBlockLangLabel,
  lowlight,
  CustomParagraph,
};

