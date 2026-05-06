import { useEffect, useRef, useState } from 'preact/hooks';

/**
 * useAuthenticatedAsset - Hook para carregar arquivos (imagens, PDFs) de forma autenticada.
 */
export const useAuthenticatedAsset = (
  relPath: string,
  fetchWithAuth: (url: string) => Promise<Response | null>,
  active: boolean = true,
) => {
  const [blobUrl, setBlobUrl] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const blobRef = useRef<string | null>(null);

  useEffect(() => {
    if (!relPath || !active) return;

    let isMounted = true;
    setIsLoading(true);
    setError(null);

    const loadAsset = async () => {
      // Pequeno delay para scroll suave
      await new Promise((resolve) => setTimeout(resolve, 150));
      if (!isMounted) return;

      try {
        // Correção da URL para bater com o endpoint da API
        const res = await fetchWithAuth(`/api/file?name=${encodeURIComponent(relPath)}`);
        if (!res?.ok) throw new Error(`Erro ao buscar arquivo: ${res?.status || 'desconhecido'}`);

        const blob = await res.blob();
        if (isMounted) {
          const url = URL.createObjectURL(blob);
          blobRef.current = url;
          setBlobUrl(url);
        }
      } catch (err: any) {
        if (isMounted) setError(err.message);
      } finally {
        if (isMounted) setIsLoading(false);
      }
    };

    loadAsset();

    return () => {
      isMounted = false;
      const urlToRevoke = blobRef.current;
      if (urlToRevoke) {
        setTimeout(() => {
          URL.revokeObjectURL(urlToRevoke);
        }, 10000);
      }
    };
  }, [relPath, active, fetchWithAuth]);

  return { blobUrl, isLoading, error };
};
