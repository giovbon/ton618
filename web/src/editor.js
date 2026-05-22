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
import Suggestion from "@tiptap/suggestion";
import { Markdown } from "tiptap-markdown";
import { marked } from "marked";

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
  Link,
  Mention,
  Suggestion,
  Markdown,
  marked,
};
