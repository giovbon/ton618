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
import { TextStyle } from "@tiptap/extension-text-style";
import { FontFamily } from "@tiptap/extension-font-family";
import Highlight from "@tiptap/extension-highlight";
import Link from "@tiptap/extension-link";
import Mention from "@tiptap/extension-mention";

const CustomLink = Link.extend({
  inclusive: false,
});
import Suggestion from "@tiptap/suggestion";
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

// Custom Paragraph extension to serialize empty paragraphs as &nbsp;
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

// Expõe no window para uso no editor.html
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
  TextStyle,
  FontFamily,
  Highlight,
  Link: CustomLink,
  Mention,
  Suggestion,
  Markdown,
  marked,
  CodeBlockLowlightExt: CodeBlockLangLabel,
  lowlight,
  CustomParagraph,
};

