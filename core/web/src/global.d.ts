/**
 * Declarações de tipos globais para o frontend TON-618.
 *
 * Este arquivo cobre:
 * 1. Globais expostas via `window.*` pelo padrão IIFE
 * 2. Módulos CSS (importados side-effect em JS/JSX)
 * 3. APIs de terceiros sem tipos
 */

// ── CSS Modules ──
declare module '*.css' {
  const content: string;
  export default content;
}

// ── Tabulator (global exposto via static/tabulator.min.js) ──
declare const Tabulator: any;

// ── Editor globals (IIFE exports) ──
interface Window {
  // drawing.jsx
  initDrawing: (containerEl: HTMLElement, options: { initialState?: any; onChange: Function; onReady?: Function }) => any;

  // editor.js (TipTap)
  TipTapEditor: any;

  // mindmap.js
  initMindmap: (svgEl: SVGElement, initialMarkdown: string) => any;

  // Tabulator (static/tabulator.min.js)
  Tabulator: any;

  // Editor helpers (editor-common.js)
  generateHash: (data: string) => Promise<string>;
  doSave: () => void;
  doRename: () => void;
  deleteCurrentNote: () => void;
  duplicateCurrentNote: () => void;
}

// ── Semantic Index (tipos complementares ao JSDoc em semantic.js) ──
interface Window {
  semanticIndex: any;
  _semanticDesktopOnly: boolean;
  markmap: any;
  hljs: any;
}

// ── Agenda / App ──
interface Window {
  loadMarcadores?: () => void;
  loadStopwords?: () => void;
  loadArchives?: () => void;
  onOpenSemanticaTab?: () => void;
  resetAndReindexSemantic?: () => void;
}

// ── Alpine.js store ──
interface Window {
  Alpine?: {
    store: (name: string, value: any) => void;
  };
}

// ── EditorCommon (exposto por editor-common.js) ──
declare namespace EditorCommon {
  function generateHash(text: string): Promise<string>;
  function setStatus(el: HTMLElement, s: 'saved' | 'saving' | 'dirty'): void;
  function getAuthHeaders(): Record<string, string>;
  function httpSaveNote(filename: string, content: string, tags?: string, silent?: boolean): Promise<Response>;
  function httpSaveFile(filename: string, content: string, tags?: string): Promise<Response>;
  function httpRename(oldName: string, newName: string): Promise<Response>;
  function httpDelete(filename: string): Promise<Response>;
  function httpDuplicate(filename: string): Promise<Response>;
  function httpUploadImage(file: File): Promise<{ ok: boolean; url?: string; error?: string }>;
  function toggleBacklinksPopover(event?: MouseEvent): void;
  function setupCodeJarActiveLine(editorEl: HTMLElement | null): void;
  function wikilinksToMarkdown(content: string): string;
  function normalizeFilename(name: string): string;
  function getCurrentFilename(filenameInput: HTMLInputElement): string;
  function getDisplayName(filename: string): string;
  function deleteCurrentNote(filenameInput: HTMLInputElement, confirmMsg?: string): void;
  function duplicateCurrentNote(filenameInput: HTMLInputElement, redirectBase?: string, confirmMsg?: string): void;
  function doRenameContent(filenameInput: HTMLInputElement, getContentFn: (() => string) | null, redirectBase?: string, opts?: { setStatus?: (s: string) => void; tags?: string; onSaved?: (content: string, filename: string) => void }): Promise<void>;
  function setupRenameListeners(filenameInput: HTMLInputElement, opts?: { getContent?: () => string; redirectBase?: string; setStatus?: (s: string) => void; tags?: string }): void;
  function setupCtrlS(saveFn: () => void): void;
  /**
   * Decide se o texto colado deve ser interpretado como markdown em vez de HTML.
   * Retorna true quando o texto puro tem sintaxe markdown E o HTML do clipboard
   * não tem formatação real (caso do "HTML de fachada" com **asteriscos** crus).
   */
  function shouldPasteAsMarkdown(text: string, html: string): boolean;
  /** Detecta tags de formatação real no HTML do clipboard. */
  function htmlHasRealFormatting(html: string): boolean;
  /** Detecta sintaxe markdown (títulos, listas, ênfase, código, links). */
  function textHasMarkdownSyntax(text: string): boolean;
  /**
   * Remove "espaço morto" do HTML colado: blocos vazios, `<br><br>` e listas
   * loose. Nunca descarta conteúdo — só espaço em branco e wrappers redundantes.
   */
  function normalizePastedHtml(html: string): string;
  /**
   * Informa se o novo nome joga a nota para OUTRO editor (ex: "desenho" → /drawing).
   * Quando true o caller deve redirecionar; quando false dá para atualizar a página.
   */
  function renameChangesEditor(newName: string, currentBase?: string): boolean;
  /**
   * Sincroniza a página depois de renomear SEM recarregar: URL (replaceState),
   * input do nome (value + data-filename), título da aba e reload da sidebar.
   * Retorna o filename normalizado.
   */
  function applyRenameToUI(opts?: {
    newName: string;
    filenameInput?: HTMLInputElement;
    base?: string;
    titlePrefix?: string;
  }): string;
}
