import { EditorContent, useEditor } from "@tiptap/react";
import { BubbleMenu } from "@tiptap/react/menus";
import StarterKit from "@tiptap/starter-kit";
import Placeholder from "@tiptap/extension-placeholder";
import { Markdown } from "tiptap-markdown";
import { useEffect, useRef, useState, useCallback } from "preact/hooks";

import { EditorHeader } from "./editor/EditorHeader";
import { DeleteConfirmModal } from "./modals/DeleteConfirmModal";
import { WikiLinkNode } from "./editor/WikiLinkExtension";
import Mention from "@tiptap/extension-mention";
import { getSuggestionConfig } from "./editor/wikiLinkSuggestion";
import { SlashCommandExtension } from "./editor/SlashCommandExtension";
import { getSlashCommandConfig } from "./editor/slashCommandSuggestion";
import { Table } from "@tiptap/extension-table";
import { TableCell } from "@tiptap/extension-table-cell";
import { TableRow } from "@tiptap/extension-table-row";
import { TableHeader } from "@tiptap/extension-table-header";

interface TiptapEditorProps {
  fileName: string;
  initialContent: string;
  onSave: (content: string, isAuto?: boolean) => Promise<boolean>;
  onClose: () => void;
  isSaving: boolean;
  scrollToText?: string | null;
  onDeleteNote?: (filename: string) => void;
  onRename?: (
    oldName: string,
    newName: string,
    currentContent: string,
  ) => Promise<boolean>;
  fetchWithAuth: (
    url: string,
    options?: RequestInit,
  ) => Promise<Response | null>;
}

type EditorStatus = "saved" | "dirty" | "saving";

const TiptapEditor = ({
  fileName,
  initialContent,
  onSave,
  onClose,
  isSaving,
  scrollToText,
  onDeleteNote,
  onRename,
  fetchWithAuth,
}: TiptapEditorProps) => {
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false);
  const [isDeleting, setIsDeleting] = useState(false);
  const [isEditingName, setIsEditingName] = useState(false);
  const [newFileName, setNewFileName] = useState(fileName);
  const [editorStatus, setEditorStatus] = useState<EditorStatus>("saved");
  const autoSaveTimerRef = useRef<any>(null);
  const onSaveRef = useRef(onSave);
  const hasScrolledRef = useRef(false);

  // Frontmatter parsing regex: strictly requires it to start with a YAML key (e.g., 'title:' or 'tags:')
  // to avoid capturing Markdown horizontal rules that wrap standard content. Supports CRLF.
  const FRONTMATTER_REGEX =
    /^---\r?\n([a-zA-Z0-9_-]+:[ \t]*[\s\S]*?)\r?\n---\r?\n([\s\S]*)$/;

  // Frontmatter parsing state
  const [frontmatter, setFrontmatter] = useState(() => {
    const match = initialContent.match(FRONTMATTER_REGEX);
    return match ? match[1] : "";
  });
  const [showFrontmatter, setShowFrontmatter] = useState(false);
  const frontmatterRef = useRef(frontmatter);
  const [isMobile, setIsMobile] = useState(false);
  const [hasSelection, setHasSelection] = useState(false);

  useEffect(() => {
    const checkMobile = () => {
      setIsMobile(
        window.innerWidth < 768 ||
          /Android|webOS|iPhone|iPad|iPod|BlackBerry|IEMobile|Opera Mini/i.test(
            navigator.userAgent,
          ),
      );
    };
    checkMobile();
    window.addEventListener("resize", checkMobile);
    return () => window.removeEventListener("resize", checkMobile);
  }, []);

  const cleanInitialContent = useRef(() => {
    const match = initialContent.match(FRONTMATTER_REGEX);
    let base = match ? match[2] : initialContent;
    return base.replace(
      /\[\[([^\]]+)\]\]/g,
      '<span data-wikilink="$1"></span>',
    );
  }).current();

  // WikiLink Notes list for suggestions
  const notesRef = useRef<string[]>([]);

  useEffect(() => {
    fetchWithAuth("/api/notes")
      .then((res) => (res?.ok ? res.json() : null))
      .then((data) => {
        if (data && data.notes) {
          notesRef.current = data.notes.map((n: string) => {
            const clean = n.split("/").pop() || n;
            return clean.replace(/\.md$/, "");
          });
        }
      })
      .catch(console.error);
  }, [fetchWithAuth]);

  useEffect(() => {
    onSaveRef.current = onSave;
  }, [onSave]);

  useEffect(() => {
    setNewFileName(fileName);
    setEditorStatus("saved");
  }, [fileName]);

  useEffect(() => {
    // Disable background scrolling when the editor modal is open
    const originalOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";

    return () => {
      document.body.style.overflow = originalOverflow;
      if (autoSaveTimerRef.current) clearTimeout(autoSaveTimerRef.current);
    };
  }, []);

  const editor = useEditor({
    extensions: [
      StarterKit.configure({
        heading: {
          levels: [1, 2, 3],
        },
      }),
      WikiLinkNode,
      Markdown.configure({
        html: true,
        transformPastedText: true,
        transformCopiedText: true,
      }),
      Placeholder.configure({
        placeholder: ({ node }) => {
          if (node.type.name === "heading") {
            return "Título...";
          }
          return "Pressione / para comandos, ou comece a escrever...";
        },
      }),
      Mention.extend({
        name: "wikilinkSuggestion",
      }).configure({
        HTMLAttributes: {
          class: "wikilink-mention",
        },
        suggestion: getSuggestionConfig(notesRef),
        renderLabel({ options, node }) {
          return `[[${node.attrs.id ?? node.attrs.label}]]`;
        },
      }),
      SlashCommandExtension.configure({
        suggestion: getSlashCommandConfig(),
      }),
      Table.configure({ resizable: true }),
      TableCell,
      TableRow,
      TableHeader,
    ],
    content: cleanInitialContent,
    onSelectionUpdate: ({ editor }) => {
      setHasSelection(!editor.state.selection.empty);
    },
    onUpdate: ({ editor }) => {
      setEditorStatus("dirty");
      if (autoSaveTimerRef.current) clearTimeout(autoSaveTimerRef.current);

      autoSaveTimerRef.current = setTimeout(() => {
        setEditorStatus("saving");
        let content = editor.storage.markdown.getMarkdown();

        // Unescape brackets if tiptap-markdown escaped them, or clean raw HTML if it fell back to HTML
        content = content.replace(/\\\[\\\[/g, "[[").replace(/\\\]\\\]/g, "]]");
        content = content.replace(
          /<span data-wikilink="([^"]+)".*?<\/span>/g,
          "[[$1]]",
        );

        const fm = frontmatterRef.current;
        const finalContent = fm.trim()
          ? `---\n${fm}\n---\n${content}`
          : content;

        onSaveRef
          .current(finalContent, true)
          .then(() => setEditorStatus("saved"))
          .catch(() => setEditorStatus("dirty"));
      }, 1000);
    },
    editorProps: {
      attributes: {
        class:
          "prose prose-invert max-w-2xl mx-auto focus:outline-none min-h-[500px] font-sans pb-32",
      },
    },
  });

  useEffect(() => {
    if (editor && scrollToText && !hasScrolledRef.current) {
      // Logic for scrolling to text would go here, maybe traversing nodes
      hasScrolledRef.current = true;
    }
  }, [editor, scrollToText]);

  async function handleDelete() {
    setIsDeleting(true);
    try {
      const res = await fetchWithAuth(
        `/api/file?name=${encodeURIComponent(fileName)}`,
        {
          method: "DELETE",
        },
      );
      if (res?.ok) {
        setShowDeleteConfirm(false);
        if (onDeleteNote) onDeleteNote(fileName);
        onClose();
        window.location.reload();
      }
    } catch (err) {
      console.error("Erro ao deletar:", err);
    } finally {
      setIsDeleting(false);
    }
  }

  async function handleRename() {
    if (!editor) return;
    if (autoSaveTimerRef.current) clearTimeout(autoSaveTimerRef.current);
    const trimmed = newFileName.trim();
    if (trimmed === fileName || trimmed === "") {
      setIsEditingName(false);
      setNewFileName(fileName);
      return;
    }

    const cleanName = trimmed.split("/").pop() || "";
    const finalNewName = `notes/${cleanName.replace(/\.md$/, "")}`;
    const newNameWithExt = `${finalNewName}.md`;

    try {
      const content = editor.storage.markdown.getMarkdown();
      const fm = frontmatterRef.current;
      const finalContent = fm.trim() ? `---\n${fm}\n---\n${content}` : content;

      await onSave(finalContent, true);

      if (onRename) {
        const success = await onRename(fileName, newNameWithExt, finalContent);
        if (success) {
          setIsEditingName(false);
        }
      }
    } catch (err) {
      console.error("Erro ao renomear:", err);
    }
  }

  const handleFrontmatterChange = (e: any) => {
    const val = e.target.value;
    setFrontmatter(val);
    frontmatterRef.current = val;
    setEditorStatus("dirty");
    if (autoSaveTimerRef.current) clearTimeout(autoSaveTimerRef.current);
    autoSaveTimerRef.current = setTimeout(() => {
      setEditorStatus("saving");
      let content = editor.storage.markdown.getMarkdown();
      content = content.replace(/\\\[\\\[/g, "[[").replace(/\\\]\\\]/g, "]]");
      content = content.replace(
        /<span data-wikilink="([^"]+)".*?<\/span>/g,
        "[[$1]]",
      );
      const finalContent = val.trim()
        ? `---\n${val}\n---\n${content}`
        : content;
      onSaveRef
        .current(finalContent, true)
        .then(() => setEditorStatus("saved"))
        .catch(() => setEditorStatus("dirty"));
    }, 1000);
  };

  if (!editor) {
    return null;
  }

  return (
    <div className="editor-root bg-zinc-950 flex flex-col h-full fixed inset-0 z-[100] animate-in fade-in">
      <EditorHeader
        fileName={fileName}
        newFileName={newFileName}
        setNewFileName={setNewFileName}
        isEditingName={isEditingName}
        setIsEditingName={setIsEditingName}
        handleRename={handleRename}
        onClose={onClose}
        setShowDeleteConfirm={setShowDeleteConfirm}
        editorStatus={editorStatus}
      />

      <main className="flex-1 overflow-y-auto px-4 sm:px-10 md:px-20 lg:px-[15%] py-10 custom-scrollbar relative editor-main">
        {/* Frontmatter Editor */}
        <div className="max-w-2xl mx-auto mb-8">
          <button
            onClick={() => setShowFrontmatter(!showFrontmatter)}
            className="flex items-center gap-2 text-xs font-mono text-zinc-500 hover:text-sky-400 transition-colors"
          >
            <svg
              className={`w-3.5 h-3.5 transition-transform ${showFrontmatter ? "rotate-90" : ""}`}
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth="2"
                d="M9 5l7 7-7 7"
              />
            </svg>
            FRONTMATTER / METADADOS
          </button>

          {showFrontmatter && (
            <div className="mt-3 animate-in slide-in-from-top-2 fade-in duration-200">
              <textarea
                value={frontmatter}
                onChange={handleFrontmatterChange}
                placeholder={"title: Minha Nota\ntags:\n  - tag1\n  - tag2"}
                className="w-full bg-zinc-900/50 border border-zinc-800 rounded-lg p-4 font-mono text-xs text-zinc-300 focus:outline-none focus:border-sky-500/50 focus:bg-zinc-900 transition-all resize-y min-h-[120px] custom-scrollbar"
                spellCheck={false}
              />
            </div>
          )}
        </div>

        {editor && (
          <BubbleMenu
            editor={editor}
            shouldShow={({ editor }) =>
              !isMobile && !editor.state.selection.empty && !editor.isActive('table')
            }
            tippyOptions={{ duration: 150, animation: "scale" }}
            className="flex items-center gap-1 p-1 bg-zinc-900/95 backdrop-blur-xl border border-zinc-700/50 rounded-xl shadow-[0_10px_40px_-10px_rgba(0,0,0,0.7)]"
          >
            <button
              onClick={() => editor.chain().focus().toggleBold().run()}
              className={`p-1.5 rounded-lg transition-all ${
                editor.isActive("bold")
                  ? "bg-sky-500 text-white shadow-md"
                  : "text-zinc-400 hover:text-zinc-200 hover:bg-zinc-800"
              }`}
              title="Negrito"
            >
              <svg
                className="w-4 h-4"
                fill="none"
                stroke="currentColor"
                viewBox="0 0 24 24"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth="3"
                  d="M6 4h8a4 4 0 014 4 4 4 0 01-4 4H6zM6 12h9a4 4 0 014 4 4 4 0 01-4 4H6z"
                />
              </svg>
            </button>
            <button
              onClick={() => editor.chain().focus().toggleItalic().run()}
              className={`p-1.5 rounded-lg transition-all ${
                editor.isActive("italic")
                  ? "bg-sky-500 text-white shadow-md"
                  : "text-zinc-400 hover:text-zinc-200 hover:bg-zinc-800"
              }`}
              title="Itálico"
            >
              <svg
                className="w-4 h-4"
                fill="none"
                stroke="currentColor"
                viewBox="0 0 24 24"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth="3"
                  d="M10 20l4-16"
                />
              </svg>
            </button>
            <button
              onClick={() => editor.chain().focus().toggleStrike().run()}
              className={`p-1.5 rounded-lg transition-all ${
                editor.isActive("strike")
                  ? "bg-sky-500 text-white shadow-md"
                  : "text-zinc-400 hover:text-zinc-200 hover:bg-zinc-800"
              }`}
              title="Tachado"
            >
              <svg
                className="w-4 h-4"
                fill="none"
                stroke="currentColor"
                viewBox="0 0 24 24"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth="2.5"
                  d="M13 12h8m-15 0h.01M7 12h.01M10 12h.01M5 20l14-16"
                />
              </svg>
            </button>

            <div className="w-px h-4 bg-zinc-800 mx-1" />

            <button
              onClick={() => editor.chain().focus().toggleCode().run()}
              className={`p-1.5 rounded-lg transition-all ${
                editor.isActive("code")
                  ? "bg-emerald-500 text-white shadow-md"
                  : "text-zinc-400 hover:text-zinc-200 hover:bg-zinc-800"
              }`}
              title="Código Inline"
            >
              <svg
                className="w-4 h-4"
                fill="none"
                stroke="currentColor"
                viewBox="0 0 24 24"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth="2.5"
                  d="M10 20l4-16m4 4l4 4-4 4M6 16l-4-4 4-4"
                />
              </svg>
            </button>
            <button
              onClick={() => {
                const title = window.prompt("Nome da nota (WikiLink):");
                if (title) {
                  editor
                    .chain()
                    .focus()
                    .insertContent(`<span data-wikilink="${title}"></span>`)
                    .run();
                }
              }}
              className="p-1.5 text-zinc-400 hover:text-sky-400 hover:bg-sky-500/10 rounded-lg transition-all"
              title="Inserir Link"
            >
              <svg
                className="w-4 h-4"
                fill="none"
                stroke="currentColor"
                viewBox="0 0 24 24"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth="2.5"
                  d="M13.828 10.172a4 4 0 0 0-5.656 0l-4 4a4 4 0 1 0 5.656 5.656l1.102-1.101m-.758-4.899a4 4 0 0 0 5.656 0l4-4a4 4 0 0 0-5.656-5.656l-1.1 1.1"
                />
              </svg>
            </button>
          </BubbleMenu>
        )}

        {/* Table BubbleMenu */}
        {editor && editor.isActive("table") && (
          <BubbleMenu
            editor={editor}
            shouldShow={() => editor.isActive("table")}
            tippyOptions={{ duration: 150, placement: "top" }}
            className="flex items-center gap-0.5 p-1 bg-zinc-900/95 backdrop-blur-xl border border-zinc-700/50 rounded-xl shadow-[0_10px_40px_-10px_rgba(0,0,0,0.7)]"
          >
            <button
              onClick={() => editor.chain().focus().addColumnBefore().run()}
              title="Coluna antes"
              className="p-1.5 text-zinc-400 hover:text-sky-400 hover:bg-sky-500/10 rounded-lg transition-all"
            >
              <svg
                class="w-4 h-4"
                fill="none"
                stroke="currentColor"
                viewBox="0 0 24 24"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2" d="M4 4h16M4 20h16M4 4v16M20 4v16"
                  d="M13 5l-7 7 7 7M3 5v14"
                />
              </svg>
            </button>
            <button
              onClick={() => editor.chain().focus().addColumnAfter().run()}
              title="Coluna depois"
              className="p-1.5 text-zinc-400 hover:text-sky-400 hover:bg-sky-500/10 rounded-lg transition-all"
            >
              <svg
                class="w-4 h-4"
                fill="none"
                stroke="currentColor"
                viewBox="0 0 24 24"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2" d="M4 4h16M4 12h16M4 20h16"
                  d="M11 5l7 7-7 7M21 5v14"
                />
              </svg>
            </button>
            <button
              onClick={() => editor.chain().focus().addRowBefore().run()}
              title="Linha antes"
              className="p-1.5 text-zinc-400 hover:text-sky-400 hover:bg-sky-500/10 rounded-lg transition-all"
            >
              <svg
                class="w-4 h-4"
                fill="none"
                stroke="currentColor"
                viewBox="0 0 24 24"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2" d="M9 3v18M3 9h18"
                  d="M5 13l7-7 7 7M5 3v18"
                />
              </svg>
            </button>
            <button
              onClick={() => editor.chain().focus().addRowAfter().run()}
              title="Linha depois"
              className="p-1.5 text-zinc-400 hover:text-sky-400 hover:bg-sky-500/10 rounded-lg transition-all"
            >
              <svg
                class="w-4 h-4"
                fill="none"
                stroke="currentColor"
                viewBox="0 0 24 24"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2" d="M9 3v18M3 9h18"
                  d="M5 11l7 7 7-7M5 21V3"
                />
              </svg>
            </button>
            <div class="w-px h-5 bg-zinc-700 mx-0.5" />
            <button
              onClick={() => editor.chain().focus().deleteColumn().run()}
              title="Deletar coluna"
              className="p-1.5 text-zinc-400 hover:text-red-400 hover:bg-red-500/10 rounded-lg transition-all"
            >
              <svg
                class="w-4 h-4"
                fill="none"
                stroke="currentColor"
                viewBox="0 0 24 24"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2" d="M9 3v18M3 9h18"
                />
              </svg>
            </button>
            <button
              onClick={() => editor.chain().focus().deleteRow().run()}
              title="Deletar linha"
              className="p-1.5 text-zinc-400 hover:text-red-400 hover:bg-red-500/10 rounded-lg transition-all"
            >
              <svg
                class="w-4 h-4"
                fill="none"
                stroke="currentColor"
                viewBox="0 0 24 24"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2" d="M9 3v18M3 9h18"
                />
              </svg>
            </button>
            <div class="w-px h-5 bg-zinc-700 mx-0.5" />
            <button
              onClick={() => editor.chain().focus().deleteTable().run()}
              title="Deletar tabela"
              className="p-1.5 text-zinc-400 hover:text-red-400 hover:bg-red-500/10 rounded-lg transition-all"
            >
              <svg
                class="w-4 h-4"
                fill="none"
                stroke="currentColor"
                viewBox="0 0 24 24"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M3 6h18M8 6V4a1 1 0 011-1h6a1 1 0 011 1v2m2 0v14a1 1 0 01-1 1H7a1 1 0 01-1-1V6h14"
                />
              </svg>
            </button>
          </BubbleMenu>
        )}

        {/* Toolbar Móvel (Fixa embaixo quando há seleção) */}
        {isMobile && hasSelection && editor && (
          <div className="fixed bottom-0 left-0 right-0 z-[110] bg-zinc-900/95 backdrop-blur-xl border-t border-zinc-800 p-3 flex items-center justify-around animate-in slide-in-from-bottom-full duration-200 pb-safe">
            <button
              onClick={() => editor.chain().focus().toggleBold().run()}
              className={`p-2 rounded-xl transition-all ${
                editor.isActive("bold")
                  ? "bg-sky-500 text-white shadow-lg scale-110"
                  : "text-zinc-400"
              }`}
            >
              <svg
                className="w-5 h-5"
                fill="none"
                stroke="currentColor"
                viewBox="0 0 24 24"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth="3"
                  d="M6 4h8a4 4 0 014 4 4 4 0 01-4 4H6zM6 12h9a4 4 0 014 4 4 4 0 01-4 4H6z"
                />
              </svg>
            </button>
            <button
              onClick={() => editor.chain().focus().toggleItalic().run()}
              className={`p-2 rounded-xl transition-all ${
                editor.isActive("italic")
                  ? "bg-sky-500 text-white shadow-lg scale-110"
                  : "text-zinc-400"
              }`}
            >
              <svg
                className="w-5 h-5"
                fill="none"
                stroke="currentColor"
                viewBox="0 0 24 24"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth="3"
                  d="M10 20l4-16"
                />
              </svg>
            </button>
            <button
              onClick={() => editor.chain().focus().toggleStrike().run()}
              className={`p-2 rounded-xl transition-all ${
                editor.isActive("strike")
                  ? "bg-sky-500 text-white shadow-lg scale-110"
                  : "text-zinc-400"
              }`}
            >
              <svg
                className="w-5 h-5"
                fill="none"
                stroke="currentColor"
                viewBox="0 0 24 24"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth="2.5"
                  d="M13 12h8m-15 0h.01M7 12h.01M10 12h.01M5 20l14-16"
                />
              </svg>
            </button>
            <button
              onClick={() => editor.chain().focus().toggleCode().run()}
              className={`p-2 rounded-xl transition-all ${
                editor.isActive("code")
                  ? "bg-emerald-500 text-white shadow-lg scale-110"
                  : "text-zinc-400"
              }`}
            >
              <svg
                className="w-5 h-5"
                fill="none"
                stroke="currentColor"
                viewBox="0 0 24 24"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth="2.5"
                  d="M10 20l4-16m4 4l4 4-4 4M6 16l-4-4 4-4"
                />
              </svg>
            </button>
            <button
              onClick={() => {
                const title = window.prompt("Nome da nota (WikiLink):");
                if (title) {
                  editor
                    .chain()
                    .focus()
                    .insertContent(`<span data-wikilink="${title}"></span>`)
                    .run();
                }
              }}
              className="p-2 text-zinc-400"
            >
              <svg
                className="w-5 h-5"
                fill="none"
                stroke="currentColor"
                viewBox="0 0 24 24"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth="2.5"
                  d="M13.828 10.172a4 4 0 0 0-5.656 0l-4 4a4 4 0 1 0 5.656 5.656l1.102-1.101m-.758-4.899a4 4 0 0 0 5.656 0l4-4a4 4 0 0 0-5.656-5.656l-1.1 1.1"
                />
              </svg>
            </button>
          </div>
        )}

        <EditorContent editor={editor} className="w-full h-full min-h-full" />
      </main>

      <DeleteConfirmModal
        filename={showDeleteConfirm ? fileName : null}
        isDeleting={isDeleting}
        onClose={() => setShowDeleteConfirm(false)}
        onConfirm={handleDelete}
      />
    </div>
  );
};

export default TiptapEditor;
