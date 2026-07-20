/**
 * Declarações de tipos globais para o frontend TON-618.
 *
 * Este arquivo cobre:
 * 1. Globais expostas via `window.*` pelo padrão IIFE
 * 2. Módulos CSS (importados side-effect em JS/JSX)
 * 3. APIs de terceiros sem tipos (Leaflet, jSuites, jspreadsheet, markmap)
 */

// ── Módulos CSS ──
declare module '*.css' {
  const content: string;
  export default content;
}

// ── Leaflet (importado como script global, sem módulo) ──
declare namespace L {
  let Icon: any;
  let DivIcon: any;
  let Marker: any;
  let TileLayer: any;
  let map: any;
  let Map: any;
  let Control: any;
  let DomUtil: any;
  let Polyline: any;
  let Polygon: any;
  let Circle: any;
  let circleMarker: any;
  let CircleMarker: any;
  let LatLng: any;
  let LatLngBounds: any;
  let Point: any;
  let Bounds: any;
  let layerGroup: any;
  let featureGroup: any;
  let geoJSON: any;
  let tileLayer: any;
  let marker: any;
  let control: any;
  let polyline: any;
  let polygon: any;
  let circle: any;
  let rectangle: any;
  let icon: any;
  let divIcon: any;
  let DomEvent: any;
  let Browser: any;
  function on(el: any, event: string, handler: any): void;
  function off(el: any, event: string, handler: any): void;
}

// ── Semantic Index (tipos complementares ao JSDoc em semantic.js) ──
interface Window {
  semanticIndex: any;
  _semanticIndexNote: (filename: string, content: string) => void;
  _semanticDesktopOnly: boolean;
  jsuites: any;
  jSuites: any;
  JspreadsheetCE: any;
  jspreadsheet: any;
  markmap: any;
}

// ── Editor globals (IIFE exports) ──
interface Window {
  // drawing.jsx
  initDrawing: (containerEl: HTMLElement, options: { initialState?: any; onChange: Function; onReady?: Function }) => any;

  // editor.js (TipTap)
  TipTapEditor: any;

  // map.js
  initMap: (container: HTMLElement | string, markersData: any[] | undefined, onChange: Function) => any;
  _mapRenameMarker: (oldName: string, newName: string) => void;
  _mapGetMarker: (name: string) => any;
  _mapAddMode: () => void;
  _mapToggleMeasureMode: () => void;
  _mapToggleSatellite: () => void;
  _mapOnChange: (content: string) => void;
  _mapSearch: (query: string) => void;
  _mapGoToLocation: (lat: number, lng: number) => void;

  // mindmap.js
  initMindmap: (svgEl: SVGElement, initialMarkdown: string) => any;

  // spreadsheet.js
  initSpreadsheet: (container: HTMLElement, filename: string, content: string, onChange: (content: string) => void) => void;

  // Editor helpers (editor-common.js)
  generateHash: (data: string) => Promise<string>;
  doSave: () => void;
  doRename: () => void;
  deleteCurrentNote: () => void;
  duplicateCurrentNote: () => void;
  saveMap: () => void;
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
}
