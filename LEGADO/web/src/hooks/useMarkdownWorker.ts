import { useEffect, useRef, useState } from 'preact/hooks';

// Interface para as requisições pendentes
interface PendingRequest {
  resolve: (html: string) => void;
  reject: (err: any) => void;
}

// Instância única do Worker (Singleton)
let workerInstance: Worker | null = null;
const pendingRequests = new Map<string, PendingRequest>();

/**
 * Hook para processar Markdown de forma assíncrona usando Web Workers.
 */
export const useMarkdownWorker = (text: string, query: string, terms: string[]) => {
  const [html, setHtml] = useState<string>('');
  const [isLoading, setIsLoading] = useState<boolean>(false);
  const requestIdRef = useRef<string>('');

  useEffect(() => {
    // Inicializa o worker se ainda não existir
    if (!workerInstance && typeof window !== 'undefined') {
      // @ts-expect-error - Vite trata ?worker automaticamente
      import('../workers/markdown.worker?worker').then((m) => {
        workerInstance = new m.default();
        workerInstance!.onmessage = (e) => {
          const { id, html, error } = e.data;
          const pending = pendingRequests.get(id);
          if (pending) {
            if (error) pending.reject(error);
            else pending.resolve(html);
            pendingRequests.delete(id);
          }
        };
      });
    }

    if (!text) {
      setHtml('');
      return;
    }

    // Gera um ID único para esta requisição
    const id = Math.random().toString(36).substring(2, 15);
    requestIdRef.current = id;
    setIsLoading(true);

    const process = async () => {
      // Espera o worker estar pronto
      while (!workerInstance) {
        await new Promise((r) => setTimeout(r, 50));
      }

      return new Promise<string>((resolve, reject) => {
        pendingRequests.set(id, { resolve, reject });
        workerInstance?.postMessage({ id, text, query, terms });
      });
    };

    process()
      .then((resultHtml) => {
        // Apenas atualiza se for a requisição mais recente deste hook
        if (requestIdRef.current === id) {
          setHtml(resultHtml);
          setIsLoading(false);
        }
      })
      .catch((err) => {
        console.error('Markdown Worker Error:', err);
        if (requestIdRef.current === id) setIsLoading(false);
      });
  }, [text, query, terms]);

  return { html, isLoading };
};
