import { useCallback, useMemo, useState } from "preact/hooks";
import type { Toast } from "../types";

export function useAppUI() {
  const [toasts, setToasts] = useState<Toast[]>([]);
  const [isMapOpen, setIsMapOpen] = useState<boolean>(false);
  const [highlightedFile, setHighlightedFile] = useState<string | null>(null);

  const addToast = useCallback(
    (message: string, type: Toast["type"] = "info") => {
      const id = Date.now();
      setToasts((prev) => [...prev, { id, message, type }]);
      setTimeout(() => {
        setToasts((prev) => prev.filter((t) => t.id !== id));
      }, 5000);
    },
    [],
  );

  return useMemo(
    () => ({
      state: {
        toasts,
        isMapOpen,
        highlightedFile,
      },
      actions: {
        setIsMapOpen,
        setHighlightedFile,
        addToast,
      },
    }),
    [toasts, isMapOpen, highlightedFile, addToast],
  );
}
