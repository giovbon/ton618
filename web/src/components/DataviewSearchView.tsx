import { useEffect, useState } from 'preact/hooks';
import { QueryResultsTable } from './QueryResultsTable';

interface DataviewSearchViewProps {
  query: string;
  fetchWithAuth: (url: string, options?: RequestInit) => Promise<Response | null>;
  onOpenFile: (filename: string) => void;
}

export const DataviewSearchView = ({
  query,
  fetchWithAuth,
  onOpenFile,
}: DataviewSearchViewProps) => {
  const [data, setData] = useState<{ headers: string[]; rows: any[][]; error?: string }>({
    headers: [],
    rows: [],
  });
  const [isLoading, setIsLoading] = useState(false);

  useEffect(() => {
    let isMounted = true;

    const fetchData = async () => {
      if (!query.trim()) return;

      setIsLoading(true);
      try {
        const res = await fetchWithAuth('/api/query', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ query }),
        });

        if (isMounted) {
          if (res?.ok) {
            const result = await res.json();
            setData(result || { headers: [], rows: [], error: 'Resultado vazio' });
          } else {
            const errorText = res ? await res.text() : 'Sem resposta do servidor';
            setData({ headers: [], rows: [], error: errorText || 'Erro na requisição' });
          }
        }
      } catch (err: any) {
        if (isMounted) {
          setData({ headers: [], rows: [], error: err.message });
        }
      } finally {
        if (isMounted) setIsLoading(false);
      }
    };

    fetchData();

    return () => {
      isMounted = false;
    };
  }, [query, fetchWithAuth]);

  return (
    <div className="animate-in fade-in duration-500">
      <div className="flex items-center gap-2 mb-4 px-2">
        <div className="w-2 h-2 rounded-full bg-sky-500 shadow-[0_0_8px_rgba(14,165,233,0.5)] animate-pulse" />
        <span className="text-[10px] font-black text-zinc-500 uppercase tracking-[0.2em]">
          Modo Consulta Ativo
        </span>
      </div>
      <QueryResultsTable data={data} isLoading={isLoading} onOpenFile={onOpenFile} />
    </div>
  );
};
