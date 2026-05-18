import { useCallback, useState } from "preact/hooks";
import type { Toast } from "../types";

let _toastId = 0;

export function useAppUI() {
  const [toasts, setToasts] = useState<Toast[]>([]);
  const [isMapOpen, setIsMapOpen] = useState<boolean>(false);
  const [highlightedFile, setHighlightedFile] = useState<string | null>(null);

  const addToast = useCallback(
    (message: string, type: Toast["type"] = "info") => {
      const id = ++_toastId;
      setToasts((prev) => [...prev, { id, message, type }]);
      setTimeout(() => {
        setToasts((prev) => prev.filter((t) => t.id !== id));
      }, 5000);
    },
    [],
  );

  return {
    state: { toasts, isMapOpen, highlightedFile },
    actions: { setIsMapOpen, setHighlightedFile, addToast },
  };
}
