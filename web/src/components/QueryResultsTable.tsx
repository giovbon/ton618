interface QueryResultsTableProps {
  data: {
    headers: string[];
    rows: any[][];
    error?: string;
  };
  isLoading: boolean;
  onOpenFile: (filename: string) => void;
}

export const QueryResultsTable = ({ data, isLoading, onOpenFile }: QueryResultsTableProps) => {
  if (isLoading) {
    return (
      <div className="flex flex-col items-center justify-center py-20 animate-pulse">
        <div className="w-12 h-12 border-4 border-sky-500/20 border-t-sky-500 rounded-full animate-spin mb-4" />
        <p className="text-zinc-500 font-medium animate-bounce">Processando consulta avançada...</p>
      </div>
    );
  }

  if (data.error) {
    return (
      <div className="bg-red-500/5 border border-red-500/20 rounded-2xl p-8 text-center my-4 animate-in fade-in zoom-in-95 duration-300">
        <div className="text-red-400 text-4xl mb-4">⚠️</div>
        <h3 className="text-red-400 font-bold uppercase tracking-widest text-sm mb-2">
          Erro na Consulta
        </h3>
        <p className="text-red-300/60 text-xs font-mono">{data.error}</p>
      </div>
    );
  }

  if (!data.rows || data.rows.length === 0) {
    return (
      <div className="bg-zinc-900/50 border border-zinc-800 rounded-2xl p-12 text-center my-4 animate-in fade-in duration-500">
        <p className="text-zinc-500 font-medium">Nenhum resultado encontrado para esta consulta.</p>
      </div>
    );
  }

  return (
    <div className="w-full my-4 overflow-hidden rounded-2xl border border-zinc-800 bg-zinc-900/40 backdrop-blur-md shadow-2xl animate-in slide-in-from-bottom-4 duration-500">
      <div className="overflow-x-auto custom-scrollbar">
        <table className="w-full text-left border-collapse">
          <thead>
            <tr className="bg-zinc-900/80 border-b border-zinc-800">
              {data.headers.map((header, i) => (
                <th
                  key={i}
                  className="px-6 py-4 text-[10px] font-black text-sky-400 uppercase tracking-[0.15em] border-r border-zinc-800/50 last:border-none"
                >
                  {header}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {data.rows.map((row, rowIndex) => (
              <tr
                key={rowIndex}
                className="group hover:bg-sky-500/5 transition-colors duration-200 border-b border-zinc-800/50 last:border-none"
              >
                {row.map((cell, cellIndex) => {
                  const isFileColumn = data.headers[cellIndex]?.toLowerCase() === 'file';
                  const cellValue =
                    cell === null ? (
                      <span className="text-zinc-700 opacity-50 italic">null</span>
                    ) : (
                      String(cell)
                    );

                  return (
                    <td
                      key={cellIndex}
                      className="px-6 py-4 text-sm text-zinc-300 border-r border-zinc-800/30 last:border-none"
                    >
                      {isFileColumn ? (
                        <button
                          onClick={() => onOpenFile(String(cell))}
                          className="text-sky-400 hover:text-sky-300 hover:underline underline-offset-4 decoration-sky-500/50 font-bold transition-all flex items-center gap-2 group/link"
                        >
                          <svg
                            className="w-3 h-3 opacity-0 group-hover/link:opacity-100 transition-opacity"
                            fill="none"
                            stroke="currentColor"
                            viewBox="0 0 24 24"
                          >
                            <path
                              strokeLinecap="round"
                              strokeLinejoin="round"
                              strokeWidth="3"
                              d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14"
                            />
                          </svg>
                          {cellValue}
                        </button>
                      ) : (
                        cellValue
                      )}
                    </td>
                  );
                })}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <div className="bg-zinc-900/60 px-6 py-3 border-t border-zinc-800 flex justify-between items-center">
        <span className="text-[10px] text-zinc-500 uppercase font-bold tracking-widest">
          {data.rows.length} resultados encontrados
        </span>
        <div className="flex gap-2">
          <div className="w-2 h-2 rounded-full bg-emerald-500/50 shadow-[0_0_8px_rgba(16,185,129,0.3)]" />
          <span className="text-[9px] text-zinc-600 font-bold uppercase tracking-tighter">
            Dataview Engine v1.0
          </span>
        </div>
      </div>
    </div>
  );
};
