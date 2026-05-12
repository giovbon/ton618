import { useQueryClient } from "@tanstack/react-query";
import { lazy, Suspense } from "preact/compat";
import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "preact/hooks";
import {
  Virtuoso as VirtuosoOriginal,
  type VirtuosoHandle,
} from "react-virtuoso";
// react-virtuoso usa tipos React; Preact retorna VNode em vez de ReactNode.
// Cast necessario para compatibilidade em tempo de compilacao (runtime funciona).
const Virtuoso = VirtuosoOriginal as any;
import { CompactResultCard } from "./components/CompactResultCard";
import { DataviewSearchView } from "./components/DataviewSearchView";
import { KnowledgeMap } from "./components/KnowledgeMap";
import { ManualSemanticMap } from "./components/ManualSemanticMap";
import Login from "./components/Login";
import { Logo } from "./components/Logo";
import { CaptureLinkModal } from "./components/modals/CaptureLinkModal";
import { CreateNoteModal } from "./components/modals/CreateNoteModal";
import { DeleteConfirmModal } from "./components/modals/DeleteConfirmModal";
import { DocsPanel } from "./components/DocsPanel";

import { SearchResultCard } from "./components/SearchResultCard";
import { TagAutocomplete } from "./components/TagAutocomplete";
import { ToastContainer } from "./components/ToastContainer";
import { useAppUI } from "./hooks/useAppUI";
import { useFileOperations } from "./hooks/useFileOperations";
import { useSearchManager } from "./hooks/useSearchManager";
import { useSSE } from "./hooks/useSSE";
import { useTagManager } from "./hooks/useTagManager";
import { useWikiNavigation } from "./hooks/useWikiNavigation";
import type {
  AppSettings,
  FileObject,
  LastEditedFile,
  SearchResult,
} from "./types";

const TiptapEditor = lazy(() => import("./components/TiptapEditor"));
const WeightsSettings = lazy(() => import("./components/WeightsSettings"));

export function App() {
  return <AppContent />;
}

function AppContent() {
  const queryClient = useQueryClient();
  const virtuosoRef = useRef<VirtuosoHandle>(null);
  const searchInputRef = useRef<HTMLInputElement>(null);
  const cameraInputRef = useRef<HTMLInputElement>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);
  const bundleInputRef = useRef<HTMLInputElement>(null);

  // 1. Auth & Core State
  const [auth, setAuth] = useState<string>(
    () => localStorage.getItem("ton_auth") || "",
  );
  const [indexingFiles, setIndexingFiles] = useState<Record<string, boolean>>(
    {},
  );
  const [_appSettings, setAppSettings] = useState<AppSettings>({
    semantic_enable: true,
    semantic_threshold: 0.2,
  });
  const [deletedFilenames, setDeletedFilenames] = useState<Set<string>>(
    new Set(),
  );
  const [lastEditedFile, _setLastEditedFile] = useState<LastEditedFile>({
    name: null,
    ts: null,
  });
  const [editingFile, setEditingFile] = useState<FileObject | null>(null);
  const [isDocsOpen, setIsDocsOpen] = useState(false);
  const [expandedIds, setExpandedIds] = useState<Record<string, boolean>>({});
  const [isManualMapOpen, setIsManualMapOpen] = useState(false);

  // 2. UI Hook
  const { state: uiState, actions: uiActions } = useAppUI();

  // 3. Auth Helpers
  const handleLogin = (authHeader: string) => {
    localStorage.setItem("ton_auth", authHeader);
    setAuth(authHeader);
  };

  const handleLogout = useCallback(() => {
    localStorage.removeItem("ton_auth");
    setAuth("");
  }, []);

  const fetchWithAuth = useCallback(
    async (url: string, options: RequestInit = {}) => {
      if (!auth) return null;
      const headers = {
        ...(options.headers || {}),
        Authorization: auth,
      } as Record<string, string>;
      const response = await fetch(url, { ...options, headers });
      if (response.status === 401) {
        handleLogout();
        throw new Error("Sessão expirada");
      }
      return response;
    },
    [auth, handleLogout],
  );

  // 4. Search Manager Hook
  const searchManager = useSearchManager({
    auth,
    handleLogout,
    lastEditedFileName: lastEditedFile.name,
    deletedFilenames,
  });
  const { state: searchState, actions: searchActions } = searchManager;

  // 5. Tag Manager Hook
  const tagManager = useTagManager(
    fetchWithAuth,
    searchState.query,
    searchActions.setQuery,
  );
  const { state: tagState, actions: tagActions } = tagManager;

  // 6. File Operations Hook
  const handleDeleteFromList = useCallback(
    (filename: string) => {
      setDeletedFilenames((prev) => {
        const next = new Set(prev);
        next.add(filename);
        return next;
      });
      queryClient.invalidateQueries({ queryKey: ["search"] });
    },
    [queryClient],
  );

  const fileOps = useFileOperations({
    fetchWithAuth,
    queryClient,
    addToast: uiActions.addToast,
    setEditingFile,
    editingFile,
    handleDeleteFromList,
  });
  const { state: fileOpsState, actions: fileOpsActions } = fileOps;

  // 7. Effects & SSE
  const fetchAppSettings = useCallback(async () => {
    try {
      const res = await fetchWithAuth("/api/settings");
      if (res?.ok) {
        const data = await res.json();
        setAppSettings(data);
      }
    } catch (err) {
      console.error("Erro ao carregar configurações:", err);
    }
  }, [fetchWithAuth]);

  useEffect(() => {
    if (auth) fetchAppSettings();
  }, [auth, fetchAppSettings]);

  useEffect(() => {
    if (!fileOpsState.isSyncing && !fileOpsState.isUploading) {
      tagActions.refreshAvailableTags();
    }
  }, [
    fileOpsState.isSyncing,
    fileOpsState.isUploading,
    tagActions.refreshAvailableTags,
  ]);

  useEffect(() => {
    if (
      !fileOpsState.isSettingsOpen &&
      !fileOpsState.isCapturingLink &&
      !fileOpsState.isCreatingNote &&
      !editingFile &&
      !fileOpsState.fileToDelete
    ) {
      searchInputRef.current?.focus();
    }
  }, [
    fileOpsState.isSettingsOpen,
    fileOpsState.isCapturingLink,
    fileOpsState.isCreatingNote,
    editingFile,
    fileOpsState.fileToDelete,
  ]);

  const sseHandlers = useMemo(
    () => ({
      "sync:started": (data: any) => {
        if (data.mode === "manual")
          uiActions.addToast("Sincronização iniciada", "info");
      },
      "sync:finished": (data: any) => {
        if (data.mode === "manual")
          uiActions.addToast(
            `Sincronização concluída: ${data.new_docs} notas encontradas.`,
            "success",
          );
        queryClient.invalidateQueries({ queryKey: ["search"] });
        tagActions.refreshAvailableTags();
        window.dispatchEvent(new CustomEvent("graph-updated"));
      },
      "ocr:started": (data: any) => {
        uiActions.addToast(
          `Iniciando OCR: ${data.filename.split("/").pop()}`,
          "info",
        );
      },
      "ocr:finished": (data: any) => {
        if (data.status === "success") {
          uiActions.addToast(
            `OCR Concluído: ${data.filename.split("/").pop()}`,
            "success",
          );
          queryClient.invalidateQueries({ queryKey: ["search"] });
          window.dispatchEvent(new CustomEvent("graph-updated"));
        } else {
          uiActions.addToast(
            `Erro no OCR: ${data.filename.split("/").pop()}`,
            "error",
          );
        }
      },
      "file:vectorizing": (data: any) => {
        setIndexingFiles((prev) => ({ ...prev, [data.filename]: true }));
      },
      "file:ready": (data: any) => {
        setIndexingFiles((prev) => {
          const next = { ...prev };
          delete next[data.filename];
          return next;
        });
        queryClient.invalidateQueries({ queryKey: ["search"] });
        window.dispatchEvent(new CustomEvent("graph-updated"));
      },
    }),
    [queryClient, tagActions.refreshAvailableTags, uiActions],
  );

  useSSE(auth, sseHandlers);
  // 8. Navigation & Interaction
  const handleOpenWikiLink = useCallback(
    async (filename: string) => {
      console.log("Abrindo nota do mapa:", filename);
      if (isManualMapOpen) setIsManualMapOpen(false);
      try {
        const res = await fetchWithAuth(
          `/api/file?name=${encodeURIComponent(filename)}`,
        );
        if (res?.ok) {
          const content = await res.text();
          setEditingFile({ name: filename, content });
        } else {
          const newPath = `notes/${filename}`;
          setEditingFile({
            name: newPath,
            content: `# ${filename.replace(/\.md$/, "")}\n\n`,
            isNew: true,
          });
        }
      } catch (err) {
        console.error("Erro ao abrir WikiLink:", err);
      }
    },
    [fetchWithAuth],
  );

  useEffect(() => {
    const handleOpenNote = (e: Event) => {
      const customEvent = e as CustomEvent<string>;
      handleOpenWikiLink(customEvent.detail);
    };
    window.addEventListener("open-note", handleOpenNote);
    return () => window.removeEventListener("open-note", handleOpenNote);
  }, [handleOpenWikiLink]);

  useEffect(() => {
    const handleOpenTopic = (_e: Event) => {
      setIsManualMapOpen(true);
    };
    window.addEventListener("open-semantic-topic", handleOpenTopic);
    return () =>
      window.removeEventListener("open-semantic-topic", handleOpenTopic);
  }, []);

  useWikiNavigation(handleOpenWikiLink, !!auth);

  useEffect(() => {
    if (virtuosoRef.current?.scrollToIndex)
      virtuosoRef.current.scrollToIndex(0);
  }, []);

  const toggleExpandText = useCallback((id: string) => {
    setExpandedIds((prev) => ({ ...prev, [id]: !prev[id] }));
  }, []);

  if (!auth) return <Login onLogin={handleLogin} />;

  if (fileOpsState.isSettingsOpen) {
    return (
      <div className="min-h-screen w-full bg-zinc-950 text-zinc-300 font-sans flex flex-col">
        <Suspense
          fallback={
            <div className="fixed inset-0 z-[110] bg-zinc-950/80 backdrop-blur-sm flex items-center justify-center">
              <div className="w-12 h-12 border-4 border-sky-500/30 border-t-sky-500 rounded-full animate-spin" />
            </div>
          }
        >
          <WeightsSettings
            isOpen={true}
            onClose={() => {
              fileOpsActions.setIsSettingsOpen(false);
              fetchAppSettings();
            }}
            fetchWithAuth={fetchWithAuth}
            onUpdate={() =>
              queryClient.invalidateQueries({ queryKey: ["search"] })
            }
            onLogout={handleLogout}
          />
        </Suspense>
        <ToastContainer toasts={uiState.toasts} />
      </div>
    );
  }

  return (
    <div className="min-h-screen w-full bg-zinc-950 text-zinc-300 font-sans flex flex-col">
      <header className="bg-zinc-900/80 backdrop-blur-xl border-b border-zinc-800/50 py-3 px-4 sm:px-6 sticky top-0 z-50 shadow-2xl">
        <div className="max-w-[1600px] mx-auto flex flex-col lg:flex-row items-center justify-between gap-4">
          <div className="flex items-center gap-3 shrink-0">
            <div className="relative group">
              <div className="absolute inset-0 bg-sky-500/20 blur-xl rounded-full opacity-0 group-hover:opacity-100 transition-opacity duration-500" />
              <Logo className="w-8 h-8 sm:w-9 sm:h-9 relative z-10" />
            </div>
            <h1 className="text-lg sm:text-xl font-black tracking-tighter text-zinc-100 uppercase leading-none">
              TON-618
            </h1>
          </div>

          <div className="flex-1 max-w-3xl w-full flex items-center gap-2 group">
            <div
              className={`relative flex-1 flex items-center bg-zinc-950/40 border transition-all duration-500 rounded-2xl
              ${searchState.isSemanticEnabled ? "border-sky-500/40 shadow-[0_0_25px_-5px_rgba(14,165,233,0.15)] bg-sky-500/5" : "border-zinc-800 focus-within:border-zinc-600 focus-within:bg-zinc-950/60"}`}
            >
              <div
                className={`pl-4 flex items-center justify-center transition-colors duration-300 ${searchState.isCompactMode ? "text-sky-400" : "text-zinc-500"}`}
              >
                <svg
                  className="w-4 h-4 sm:w-5 sm:h-5"
                  fill="none"
                  stroke="currentColor"
                  viewBox="0 0 24 24"
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth="2.5"
                    d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"
                  />
                </svg>
              </div>

              <input
                ref={searchInputRef}
                type="text"
                autoComplete="off"
                spellcheck={false}
                value={searchState.query}
                onKeyDown={(e) =>
                  tagActions.handleKeyDown(e) ||
                  (e.key === "Enter" && searchActions.handleExecuteSearch())
                }
                onInput={(e: any) => {
                  searchActions.setQuery(e.target.value);
                  tagActions.handleInput(e.target.value);
                }}
                placeholder="Apenas capture e busque..."
                className="w-full px-3 py-2.5 sm:py-3 bg-transparent text-zinc-100 outline-none placeholder:text-zinc-600 text-sm sm:text-base font-medium"
              />

              <div className="flex items-center gap-1 pl-2 pr-1.5 shrink-0 border-l border-zinc-800/50 ml-2 py-1.5">
                <button
                  onClick={() =>
                    searchActions.setIsCompactMode(!searchState.isCompactMode)
                  }
                  title={
                    searchState.isCompactMode ? "Ver páginas" : "Ver fragmentos"
                  }
                  className={`flex items-center justify-center w-12 h-12 sm:w-10 sm:h-10 rounded-xl transition-all duration-300 ${searchState.isCompactMode ? "bg-sky-500/20 text-sky-400" : "text-zinc-600 hover:text-zinc-400 hover:bg-zinc-800/50"}`}
                >
                  <svg
                    className={`w-5 h-5 sm:w-5 sm:h-5 transition-transform duration-500 ${searchState.isCompactMode ? "rotate-180" : ""}`}
                    fill="none"
                    stroke="currentColor"
                    viewBox="0 0 24 24"
                  >
                    <path
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      strokeWidth="2.5"
                      d="M4 6h16M4 12h16m-7 6h7"
                    />
                  </svg>
                </button>

                <button
                  onClick={() => uiActions.setIsMapOpen(true)}
                  title="Mapa Semântico"
                  className="flex items-center justify-center w-12 h-12 sm:w-10 sm:h-10 rounded-xl transition-all duration-300 text-zinc-600 hover:text-sky-400 hover:bg-sky-500/10 group"
                >
                  <svg
                    className="w-6 h-6 sm:w-6 sm:h-6 transform group-hover:scale-110 transition-transform duration-300"
                    viewBox="0 0 24 24"
                    fill="none"
                  >
                    <defs>
                      <linearGradient
                        id="mapIconGradient"
                        x1="0%"
                        y1="0%"
                        x2="100%"
                        y2="100%"
                      >
                        <stop offset="0%" stopColor="#22d3ee" />
                        <stop offset="50%" stopColor="#a855f7" />
                        <stop offset="100%" stopColor="#f472b6" />
                      </linearGradient>
                    </defs>
                    <path
                      d="M9 20L3 17V4L9 7M9 20L15 17M9 20V7M15 17L21 20V7L15 4M15 17V4M9 7L15 4"
                      stroke="url(#mapIconGradient)"
                      strokeWidth="2"
                      strokeLinecap="round"
                      strokeLinejoin="round"
                    />
                  </svg>
                </button>

                <button
                  onClick={() => setIsManualMapOpen(true)}
                  title="Mapa Estruturado"
                  className="flex items-center justify-center w-12 h-12 sm:w-10 sm:h-10 rounded-xl transition-all duration-300 text-zinc-600 hover:text-violet-400 hover:bg-violet-500/10 group"
                >
                  <svg
                    className="w-5 h-5 sm:w-5 sm:h-5 transform group-hover:scale-110 transition-transform duration-300"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                  >
                    <path
                      d="M12 2L2 7L12 12L22 7L12 2Z"
                      strokeWidth="2"
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      className="text-violet-400"
                    />
                    <path
                      d="M2 17L12 22L22 17"
                      strokeWidth="2"
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      className="text-violet-500"
                    />
                    <path
                      d="M2 12L12 17L22 12"
                      strokeWidth="2"
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      className="text-violet-600"
                    />
                  </svg>
                </button>
              </div>

              <TagAutocomplete
                active={tagState.tagAutocomplete.active}
                matches={tagState.tagAutocomplete.matches}
                selectedIndex={tagState.tagAutocomplete.selectedIndex}
                queryText={tagState.tagAutocomplete.queryText}
                onSelect={tagActions.applyTag}
                onClose={() =>
                  tagActions.setTagAutocomplete((prev) => ({
                    ...prev,
                    active: false,
                  }))
                }
              />
            </div>
          </div>

          <div className="flex items-center gap-1.5 sm:gap-2 shrink-0">
            <button
              onClick={fileOpsActions.handleOpenDailyNote}
              title="Nova Nota do Dia"
              className="flex items-center justify-center w-10 h-10 sm:w-11 sm:h-11 rounded-xl transition-all duration-300 border bg-sky-500/5 border-sky-500/20 text-sky-400 hover:bg-sky-500/10 hover:border-sky-500/40 active:scale-95 shadow-lg shadow-sky-500/5"
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
                  d="M12 4v16m8-8H4"
                />
              </svg>
            </button>

            <button
              onClick={() => fileOpsActions.setIsCapturingLink(true)}
              title="Capturar Link"
              disabled={fileOpsState.isProcessingLink}
              className={`flex items-center justify-center w-10 h-10 sm:w-11 sm:h-11 rounded-xl transition-all duration-300 border shadow-lg ${fileOpsState.isProcessingLink ? "bg-zinc-800 border-zinc-700 text-zinc-500 cursor-wait" : "bg-amber-500/5 border-amber-500/20 text-amber-500 hover:bg-amber-500/10 hover:border-amber-500/40 active:scale-95 shadow-amber-500/5"}`}
            >
              {fileOpsState.isProcessingLink ? (
                <svg className="w-4 h-4 animate-spin" viewBox="0 0 24 24">
                  <circle
                    className="opacity-25"
                    cx="12"
                    cy="12"
                    r="10"
                    stroke="currentColor"
                    strokeWidth="4"
                  ></circle>
                  <path
                    className="opacity-75"
                    fill="currentColor"
                    d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                  ></path>
                </svg>
              ) : (
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
                    d="M13.828 10.172a4 4 0 00-5.656 0l-4 4a4 4 0 105.656 5.656l1.102-1.101m-.758-4.899a4 4 0 005.656 0l4-4a4 4 0 00-5.656-5.656l-1.1 1.1"
                  />
                </svg>
              )}
            </button>

            <button
              onClick={() => cameraInputRef.current?.click()}
              disabled={fileOpsState.isUploading}
              title="OCR de Imagem"
              className={`flex items-center justify-center w-10 h-10 sm:w-11 sm:h-11 rounded-xl transition-all duration-300 border shadow-lg ${fileOpsState.isUploading ? "bg-zinc-800 border-zinc-700 text-zinc-500 cursor-wait" : "bg-emerald-500/5 border-emerald-500/20 text-emerald-400 hover:bg-emerald-500/10 hover:border-emerald-500/40 active:scale-95 shadow-emerald-500/5"}`}
            >
              <input
                type="file"
                ref={cameraInputRef}
                onChange={fileOpsActions.handleFileUpload}
                accept="image/*"
                capture="environment"
                className="hidden"
                data-mode="image"
                data-testid="camera-input"
              />
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
                  d="M3 9a2 2 0 012-2h.93a2 2 0 001.664-.89l.812-1.22A2 2 0 0110.07 4h3.86a2 2 0 011.664.89l.812 1.22A2 2 0 0018.07 7H19a2 2 0 012 2v9a2 2 0 01-2 2H5a2 2 0 01-2-2V9z"
                />
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth="2.5"
                  d="M15 13a3 3 0 11-6 0 3 3 0 016 0z"
                />
              </svg>
            </button>

            <button
              onClick={() => fileInputRef.current?.click()}
              disabled={fileOpsState.isUploading}
              title="Upload PDF"
              className={`flex items-center justify-center w-10 h-10 sm:w-11 sm:h-11 rounded-xl transition-all duration-300 border shadow-lg ${fileOpsState.isUploading ? "bg-zinc-800 border-zinc-700 text-zinc-500 cursor-wait" : "bg-red-500/5 border-red-500/20 text-red-400 hover:bg-red-500/10 hover:border-red-500/40 active:scale-95 shadow-red-500/5"}`}
            >
              <input
                type="file"
                ref={fileInputRef}
                onChange={fileOpsActions.handleFileUpload}
                accept=".pdf"
                className="hidden"
                data-mode="pdf"
                data-testid="pdf-input"
              />
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
                  d="M7 21h10a2 2 0 002-2V9.414a1 1 0 00-.293-.707l-5.414-5.414A1 1 0 0012.586 3H7a2 2 0 00-2 2v14a2 2 0 002 2z"
                />
              </svg>
            </button>

            <button
              onClick={() => bundleInputRef.current?.click()}
              disabled={fileOpsState.isUploading}
              title="Arquivos (Zip + Nota)"
              className={`flex items-center justify-center w-10 h-10 sm:w-11 sm:h-11 rounded-xl transition-all duration-300 border shadow-lg ${fileOpsState.isUploading ? "bg-zinc-800 border-zinc-700 text-zinc-500 cursor-wait" : "bg-indigo-500/5 border-indigo-500/20 text-indigo-400 hover:bg-indigo-500/10 hover:border-indigo-500/40 active:scale-95 shadow-indigo-500/5"}`}
            >
              <input
                type="file"
                ref={bundleInputRef}
                onChange={fileOpsActions.handleBundleUpload}
                multiple
                className="hidden"
              />
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
                  d="M20 7l-8-4-8 4m16 0l-8 4m8-4v10l-8 4m0-10L4 7m8 4v10M4 7v10l8 4"
                />
              </svg>
            </button>

            <button
              onClick={fileOpsActions.handleManualSync}
              disabled={fileOpsState.isSyncing}
              title="Sincronizar"
              className={`flex items-center justify-center w-10 h-10 sm:w-11 sm:h-11 rounded-xl transition-all duration-300 border ${fileOpsState.isSyncing ? "bg-zinc-800 border-zinc-700 text-zinc-500 cursor-wait" : fileOpsState.syncSuccess ? "bg-emerald-500/10 border-emerald-500/50 text-emerald-400 shadow-lg shadow-emerald-500/10" : "bg-zinc-900 border-zinc-800 text-zinc-500 hover:border-sky-500/40 hover:text-sky-400 hover:bg-sky-500/5 active:scale-95 shadow-lg shadow-black/20"}`}
            >
              <svg
                className={`w-5 h-5 ${fileOpsState.isSyncing ? "animate-spin" : ""}`}
                fill="none"
                stroke="currentColor"
                viewBox="0 0 24 24"
              >
                {fileOpsState.syncSuccess ? (
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth="3"
                    d="M5 13l4 4L19 7"
                  />
                ) : (
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth="3"
                    d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"
                  />
                )}
              </svg>
            </button>

            <button
              onClick={() => setIsDocsOpen(true)}
              title="Documentação"
              className="flex items-center justify-center w-10 h-10 sm:w-11 sm:h-11 rounded-xl text-zinc-500 hover:text-amber-400 hover:bg-amber-500/10 transition-all border border-transparent hover:border-amber-500/20"
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
                  strokeWidth="2"
                  d="M12 6.253v13m0-13C10.832 5.477 9.246 5 7.5 5S4.168 5.477 3 6.253v13C4.168 18.477 5.754 18 7.5 18s3.332.477 4.5 1.253m0-13C13.168 5.477 14.754 5 16.5 5c1.747 0 3.332.477 4.5 1.253v13C19.832 18.477 18.247 18 16.5 18c-1.746 0-3.332.477-4.5 1.253"
                />
              </svg>
            </button>

            <button
              onClick={() => fileOpsActions.setIsSettingsOpen(true)}
              title="Configurações"
              className="flex items-center justify-center w-10 h-10 sm:w-11 sm:h-11 rounded-xl text-zinc-500 hover:text-sky-400 hover:bg-sky-500/10 transition-all border border-transparent hover:border-sky-500/20"
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
                  d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z"
                ></path>
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth="2.5"
                  d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"
                ></path>
              </svg>
            </button>
          </div>
        </div>
      </header>

      <main className="flex-1 max-w-4xl w-full mx-auto p-4 sm:p-6 custom-scrollbar scroll-smooth">
        <section className="flex flex-col gap-4">
          <div className="flex items-center justify-between min-h-[1.5rem]">
            {searchState.debouncedQuery.trim() !== "" && (
              <h2 className="text-sm font-bold text-zinc-500 uppercase tracking-widest animate-in fade-in duration-500">
                {searchState.isLoading && searchState.results.length === 0
                  ? "Carregando..."
                  : searchState.isCompactMode
                    ? `${searchState.results.length}${searchState.hasNextPage ? "+" : ""} Notas`
                    : `${searchState.totalHits} Anexos`}
              </h2>
            )}
          </div>
          <div className="pb-20">
            {searchState.isDataviewQuery ? (
              <DataviewSearchView
                query={searchState.debouncedQuery}
                fetchWithAuth={fetchWithAuth}
                onOpenFile={handleOpenWikiLink}
              />
            ) : (
              searchState.results.length > 0 && (
                <Virtuoso
                  ref={virtuosoRef}
                  useWindowScroll
                  data={searchState.results}
                  endReached={() => {
                    if (
                      searchState.hasNextPage &&
                      !searchState.isFetchingNextPage
                    )
                      searchActions.fetchNextPage();
                  }}
                  itemContent={
                    ((index: number, doc: SearchResult) => (
                      <div className="pb-4" key={doc.id || index}>
                        {searchState.isCompactMode ? (
                          <CompactResultCard
                            doc={doc}
                            index={index}
                            query={searchState.debouncedQuery}
                            searchTerms={searchState.searchTerms}
                            onEdit={setEditingFile}
                            onDeleteFile={fileOpsActions.setFileToDelete}
                            fetchWithAuth={fetchWithAuth}
                            auth={auth}
                            isIndexing={
                              indexingFiles[doc.arquivo] || doc.is_indexing
                            }
                            isHighlighted={
                              uiState.highlightedFile === doc.arquivo
                            }
                          />
                        ) : (
                          <SearchResultCard
                            doc={doc}
                            index={index}
                            query={searchState.debouncedQuery}
                            searchTerms={searchState.searchTerms}
                            onEdit={setEditingFile}
                            isCompact={false}
                            isLastCollapsed={false}
                            toggleExpandText={toggleExpandText}
                            isExpanded={!!expandedIds[doc.id]}
                            onDeleteFile={fileOpsActions.setFileToDelete}
                            fetchWithAuth={fetchWithAuth}
                            auth={auth}
                            isIndexing={
                              indexingFiles[doc.arquivo] || doc.is_indexing
                            }
                          />
                        )}
                      </div>
                    )) as any
                  }
                  components={{
                    Footer: (() => (
                      <div className="h-20 flex items-center justify-center">
                        {" "}
                        {(searchState.isLoading ||
                          searchState.isFetchingNextPage) && (
                            <div className="flex gap-1">
                              {" "}
                              <div className="w-1.5 h-1.5 bg-sky-500 rounded-full animate-bounce [animation-delay:-0.3s]" />{" "}
                              <div className="w-1.5 h-1.5 bg-sky-500 rounded-full animate-bounce [animation-delay:-0.15s]" />{" "}
                              <div className="w-1.5 h-1.5 bg-sky-500 rounded-full animate-bounce" />{" "}
                            </div>
                          )}{" "}
                      </div>
                    )) as any,
                  }}
                />
              )
            )}
          </div>
        </section>
      </main>

      {uiState.isMapOpen && (
        <KnowledgeMap
          auth={auth}
          onOpenNote={handleOpenWikiLink}
          onClose={() => uiActions.setIsMapOpen(false)}
        />
      )}

      {isManualMapOpen && (
        <ManualSemanticMap
          auth={auth}
          onOpenNote={handleOpenWikiLink}
          onClose={() => setIsManualMapOpen(false)}
        />
      )}

      {editingFile && (
        <Suspense
          fallback={
            <div className="fixed inset-0 z-[110] bg-zinc-950/80 backdrop-blur-sm flex items-center justify-center">
              <div className="w-12 h-12 border-4 border-sky-500/30 border-t-sky-500 rounded-full animate-spin" />
            </div>
          }
        >
          <TiptapEditor
            key={editingFile.name}
            fileName={editingFile.name}
            initialContent={editingFile.content}
            scrollToText={editingFile.scrollToText}
            fetchWithAuth={fetchWithAuth}
            isSaving={fileOpsState.isSaving}
            onClose={() => {
              uiActions.setHighlightedFile(editingFile.name);
              setEditingFile(null);
              setTimeout(() => uiActions.setHighlightedFile(null), 3000);
            }}
            onSave={(content, isAuto) =>
              fileOpsActions.handleSaveFile(editingFile.name, content, isAuto)
            }
            onDeleteNote={() => {
              setEditingFile(null);
              queryClient.invalidateQueries({ queryKey: ["search"] });
            }}
            onRename={fileOpsActions.handleRenameNote}
          />
        </Suspense>
      )}

      <DeleteConfirmModal
        isOpen={!!fileOpsState.fileToDelete}
        onClose={() => fileOpsActions.setFileToDelete(null)}
        onConfirm={fileOpsActions.confirmDeletion}
        filename={fileOpsState.fileToDelete || ""}
        isDeleting={fileOpsState.isDeletingFile}
      />
      <CreateNoteModal
        isOpen={fileOpsState.isCreatingNote}
        onClose={() => fileOpsActions.setIsCreatingNote(false)}
        onSubmit={fileOpsActions.handleCreateNote}
      />
      <CaptureLinkModal
        isOpen={fileOpsState.isCapturingLink}
        onClose={() => fileOpsActions.setIsCapturingLink(false)}
        onSubmit={fileOpsActions.handleCaptureLink}
        isProcessing={fileOpsState.isProcessingLink}
      />
      <DocsPanel
        isOpen={isDocsOpen}
        onClose={() => setIsDocsOpen(false)}
        fetchWithAuth={fetchWithAuth}
      />
      <ToastContainer toasts={uiState.toasts} />
    </div>
  );
}
