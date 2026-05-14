import type { QueryClient } from "@tanstack/react-query";
import type { Dispatch, SetStateAction } from "preact/compat";
import { useCallback, useState } from "preact/compat";
import type { FileObject } from "../types";

interface UseFileOperationsProps {
  fetchWithAuth: (
    url: string,
    options?: RequestInit,
  ) => Promise<Response | null>;
  queryClient: QueryClient;
  addToast: (message: string, type: "success" | "error" | "info") => void;
  setEditingFile: Dispatch<SetStateAction<FileObject | null>>;
  editingFile: FileObject | null;
  handleDeleteFromList: (filename: string) => void;
}

import { useRef } from 'preact/compat';

export const useFileOperations = ({
  fetchWithAuth,
  queryClient,
  addToast,
  setEditingFile,
  editingFile,
  handleDeleteFromList,
}: UseFileOperationsProps) => {
  const [isSyncing, setIsSyncing] = useState(false);
  const [isUploading, setIsUploading] = useState(false);
  const [syncSuccess, setSyncSuccess] = useState(false);
  const [fileToDelete, setFileToDelete] = useState<string | null>(null);
  const [isDeletingFile, setIsDeletingFile] = useState(false);
  const [isCapturingLink, setIsCapturingLink] = useState(false);
  const [isProcessingLink, setIsProcessingLink] = useState(false);
  const [isCreatingNote, setIsCreatingNote] = useState(false);
  const [isSaving, setIsSaving] = useState(false);
  const [isSettingsOpen, setIsSettingsOpen] = useState(false);
  const pendingContentRef = useRef<string | null>(null);

  const handleManualSync = useCallback(async () => {
    if (isSyncing) return;
    setIsSyncing(true);
    setSyncSuccess(false);
    try {
      const resp = await fetchWithAuth("/api/sync?force=true", {
        method: "POST",
      });
      if (resp?.ok) {
        setSyncSuccess(true);
        setTimeout(() => setSyncSuccess(false), 3000);
      }
    } catch (err) {
      console.error("Erro na sincronização:", err);
    } finally {
      setIsSyncing(false);
    }
  }, [fetchWithAuth, isSyncing]);

  const handleCreateNote = useCallback(
    (title: string) => {
      if (!title) return;
      const cleanName = title.split("/").pop() || "";
      const fileName = `notes/${cleanName.replace(/\.md$/, "")}.md`;
      const template = `# ${title}\n\n`;
      setIsCreatingNote(false);
      setEditingFile({ name: fileName, content: template, isNew: true });
    },
    [setEditingFile],
  );

  const handleOpenDailyNote = useCallback(async () => {
    const now = new Date();
    const ymd = now.toISOString().slice(0, 10);
    const hm =
      now.getHours().toString().padStart(2, "0") +
      now.getMinutes().toString().padStart(2, "0");
    const s = now.getSeconds().toString().padStart(2, "0");
    const fileName = `notes/${ymd}-${hm}${s}.md`;
    let content = "";
    let isNew = true;

    try {
      const readRes = await fetchWithAuth(
        `/api/file?name=${encodeURIComponent(fileName)}`,
      );

      if (readRes?.ok) {
        content = await readRes.text();
        isNew = false;
      }
      setEditingFile({ name: fileName, content, scrollToText: null, isNew });
    } catch (err) {
      console.error("Erro ao abrir nota rápida:", err);
    }
  }, [fetchWithAuth, setEditingFile]);

  const handleRenameNote = useCallback(
    async (oldName: string, newName: string, currentContent: string) => {
      try {
        const res = await fetchWithAuth(
          `/api/rename?from=${encodeURIComponent(oldName)}&to=${encodeURIComponent(newName)}`,
          { method: "PUT" },
        );

        if (res?.ok) {
          // Refatoração Global de Links Semânticos
          const getTopic = (name: string) => name.split('/').pop()?.replace(/\.md$/, "") || "";
          const oldTopic = getTopic(oldName);
          const newTopic = getTopic(newName);
          
          if (oldTopic && newTopic && oldTopic !== newTopic) {
             fetchWithAuth("/api/graph/refactor-links", {
               method: "POST",
               headers: { "Content-Type": "application/json" },
               body: JSON.stringify({ oldTopic, newTopic })
             }).catch(err => console.error("Erro ao refatorar links:", err));
          }

          if (editingFile && editingFile.name === oldName) {
            setEditingFile((prev) =>
              prev
                ? {
                    ...prev,
                    name: newName,
                    content: currentContent,
                    isNew: false,
                  }
                : null,
            );
          }
          handleDeleteFromList(oldName);
          queryClient.invalidateQueries({ queryKey: ["search"] });
          return true;
        }
        return false;
      } catch (err) {
        console.error("Erro ao renomear:", err);
        return false;
      }
    },
    [
      fetchWithAuth,
      editingFile,
      setEditingFile,
      handleDeleteFromList,
      queryClient,
    ],
  );

  const handleCaptureLink = useCallback(
    async (url: string) => {
      if (!url) return;
      setIsProcessingLink(true);
      try {
        const res = await fetchWithAuth("/api/link", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ url }),
        });
        if (res?.ok) {
          const data = await res.json();
          setIsCapturingLink(false);
          addToast(
            "Link capturado! O conteúdo está sendo processado.",
            "success",
          );
          try {
            const fileRes = await fetchWithAuth(
              `/api/file?name=${encodeURIComponent(data.filename)}`,
            );
            if (fileRes?.ok) {
              const content = await fileRes.text();
              setEditingFile({ name: data.filename, content });
            }
          } catch (e) {
            console.error("Erro ao abrir nota capturada:", e);
          }
        } else {
          alert(
            "Erro ao capturar link. O site pode estar bloqueando o acesso.",
          );
        }
      } catch (err) {
        console.error("Erro Pocket:", err);
        alert("Erro de conexão ao capturar link.");
      } finally {
        setIsProcessingLink(false);
      }
    },
    [fetchWithAuth, addToast, setEditingFile],
  );

  const handleFileUpload = useCallback(
    async (e: any) => {
      const mode = e.target.getAttribute("data-mode");
      const file = e.target.files[0];
      if (!file) return;

      const name = file.name.toLowerCase();
      const isPdf = name.endsWith(".pdf");
      const isImage =
        name.endsWith(".png") ||
        name.endsWith(".jpg") ||
        name.endsWith(".jpeg");

      if (mode === "pdf" && !isPdf) {
        window.alert(
          "Este botão é exclusivo para PDFs. Para fotos ou imagens, use o botão ao lado (Imagem / Câmera).",
        );
        e.target.value = null;
        return;
      }
      if (mode === "image" && !isImage) {
        window.alert(
          "Este botão é exclusivo para Imagens / Câmera. Para documentos PDF, use o botão de Upload PDF.",
        );
        e.target.value = null;
        return;
      }

      setIsUploading(true);
      const formData = new FormData();
      formData.append("file", file);

      try {
        const res = await fetchWithAuth("/api/upload", {
          method: "POST",
          body: formData,
        });

        if (res?.ok) {
          handleManualSync();
          e.target.value = null;
        } else {
          alert("Erro no upload.");
        }
      } catch (err) {
        console.error("Erro no upload:", err);
        alert("Erro de conexão ao enviar arquivo.");
      } finally {
        setIsUploading(false);
      }
    },
    [fetchWithAuth, handleManualSync],
  );

  const handleBundleUpload = useCallback(
    async (e: any) => {
      const files = e.target.files;
      if (!files || files.length === 0) return;

      setIsUploading(true);
      const formData = new FormData();
      for (let i = 0; i < files.length; i++) {
        formData.append("files", files[i]);
      }

      try {
        const res = await fetchWithAuth("/api/bundle", {
          method: "POST",
          body: formData,
        });

        if (res?.ok) {
          const data = await res.json();
          addToast("Arquivos enviados com sucesso!", "success");
          handleManualSync();
          e.target.value = null;
          if (data.note) {
            try {
              const fileRes = await fetchWithAuth(
                `/api/file?name=${encodeURIComponent(data.note)}`,
              );
              if (fileRes?.ok) {
                const content = await fileRes.text();
                setEditingFile({ name: data.note, content });
              }
            } catch (e) {
              console.error("Erro ao abrir nota do bundle:", e);
            }
          }
        } else {
          alert("Erro ao criar pacote.");
        }
      } catch (err) {
        console.error("Erro no bundle upload:", err);
        alert("Erro de conexão ao enviar pacote.");
      } finally {
        setIsUploading(false);
      }
    },
    [fetchWithAuth, handleManualSync, addToast, setEditingFile],
  );

  const confirmDeletion = useCallback(async () => {
    if (!fileToDelete) return;
    setIsDeletingFile(true);
    try {
      if (editingFile && editingFile.name === fileToDelete) {
        setEditingFile(null);
      }
      const res = await fetchWithAuth(
        `/api/file?name=${encodeURIComponent(fileToDelete)}`,
        {
          method: "DELETE",
        },
      );
      if (res?.ok) {
        handleDeleteFromList(fileToDelete);
        setFileToDelete(null);
      } else {
        alert("Erro ao excluir arquivo.");
      }
    } catch (err) {
      console.error("Erro na exclusão:", err);
    } finally {
      setIsDeletingFile(false);
    }
  }, [
    fileToDelete,
    editingFile,
    fetchWithAuth,
    handleDeleteFromList,
    setEditingFile,
  ]);

  const handleSaveFile = useCallback(
    async (fileName: string, content: string, isAuto: boolean = false) => {
      if (isSaving) {
        // Guarda o conteudo mais recente e agenda save depois do atual terminar
        pendingContentRef.current = content;
        return false;
      }
      setIsSaving(true);
      try {
        const res = await fetchWithAuth(
          `/api/file?name=${encodeURIComponent(fileName)}`,
          {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ content }),
          },
        );
        if (res?.ok) {
          setEditingFile((prev) =>
            prev ? { ...prev, isNew: false, content } : null,
          );
          if (!isAuto) {
            addToast("Arquivo salvo com sucesso!", "success");
            queryClient.invalidateQueries({ queryKey: ["search"] });
          }
          return true;
        }
        return false;
      } catch (err) {
        console.error("Erro ao salvar:", err);
        return false;
      } finally {
        setIsSaving(false);
        // Se tem conteudo pendente, salva agora (evita perder edicoes feitas durante save)
        const pending = pendingContentRef.current;
        pendingContentRef.current = null;
        if (pending !== null && pending !== content) {
          handleSaveFile(fileName, pending, isAuto);
        }
      }
    },
    [isSaving, fetchWithAuth, setEditingFile, addToast, queryClient, pendingContentRef],
  );

  return {
      state: {
        isSyncing,
        isUploading,
        syncSuccess,
        fileToDelete,
        isDeletingFile,
        isCapturingLink,
        isProcessingLink,
        isCreatingNote,
        isSaving,
        isSettingsOpen,
      },
      actions: {
        setIsSyncing,
        setIsUploading,
        setSyncSuccess,
        setFileToDelete,
        setIsDeletingFile,
        setIsCapturingLink,
        setIsProcessingLink,
        setIsCreatingNote,
        setIsSettingsOpen,
        handleManualSync,
        handleCreateNote,
        handleOpenDailyNote,
        handleRenameNote,
        handleCaptureLink,
        handleFileUpload,
        handleBundleUpload,
        confirmDeletion,
        handleSaveFile,
      },
};
}
