import { useState } from 'preact/hooks';

interface ScoreBreakdownProps {
  score: number;
  details: Record<string, number>;
  timestamp?: string;
}

/**
 * Componente para mostrar o detalhamento do score (Debug Mode)
 * Exibe os bônus e penalidades aplicados pelo motor de re-ranking
 */
export const ScoreBreakdown = ({ score, details, timestamp }: ScoreBreakdownProps) => {
  if (score === undefined || !details) return null;
  const [showDetails, setShowDetails] = useState(false);

  const formattedDate = timestamp
    ? new Date(timestamp).toLocaleDateString('pt-BR', { month: '2-digit', year: 'numeric' })
    : 'N/A';

  const WEIGHT_CONFIGS: Record<string, { short: string; color: string; desc: string }> = {
    // Legacy / Global
    zinc_base: {
      short: 'Base',
      color: 'text-zinc-500 bg-zinc-500/10 border-zinc-500/20',
      desc: 'Score base estatístico (BM25) do motor de busca',
    },
    titulo: {
      short: 'Título',
      color: 'text-amber-500 bg-amber-500/10 border-amber-500/20',
      desc: 'Bônus para termos encontrados no título da seção',
    },
    proximidade_exata: {
      short: 'Frase',
      color: 'text-indigo-400 bg-indigo-500/10 border-indigo-500/20',
      desc: 'Bônus por encontrar a frase exata',
    },
    recencia: {
      short: 'Novo',
      color: 'text-emerald-400 bg-emerald-500/10 border-emerald-500/20',
      desc: 'Bônus por recência de edição',
    },
    caminho: {
      short: 'Path',
      color: 'text-sky-400 bg-sky-500/10 border-sky-500/20',
      desc: 'Bônus para termos no nome do arquivo',
    },
    relevancia_textual: {
      short: 'Texto',
      color: 'text-blue-400 bg-blue-500/10 border-blue-500/20',
      desc: 'Bônus por match de palavras-chave ou radicais',
    },
    semantica: {
      short: 'IA',
      color: 'text-pink-400 bg-pink-500/10 border-pink-500/20',
      desc: 'Similaridade semântica (Modo Global)',
    },
    autoridade_links: {
      short: 'Grafo',
      color: 'text-orange-500 bg-orange-500/10 border-orange-500/20',
      desc: 'Bônus por autoridade de links',
    },
    riqueza_estrutural: {
      short: 'Rico',
      color: 'text-fuchsia-400 bg-fuchsia-500/10 border-fuchsia-500/20',
      desc: 'Bônus para notas estruturadas',
    },

    // Hybrid (Alpha Mode)
    alpha_hybrid_base: {
      short: 'α-Base',
      color: 'text-sky-400 bg-sky-500/10 border-sky-500/30',
      desc: 'Combinação Convexa (IA vs Texto)',
    },
    lexico_base: {
      short: 'Léxico',
      color: 'text-zinc-400 bg-zinc-500/10 border-zinc-500/20',
      desc: 'Base de texto normalizada',
    },
    boost_titulo: {
      short: 'B-Título',
      color: 'text-amber-500 bg-amber-500/10 border-amber-500/20',
      desc: 'Multiplicador de Título',
    },
    boost_tags_keywords: {
      short: 'B-Tags',
      color: 'text-blue-400 bg-blue-500/10 border-blue-500/20',
      desc: 'Multiplicador de Tags e Termos',
    },
    boost_frase: {
      short: 'B-Frase',
      color: 'text-indigo-400 bg-indigo-500/10 border-indigo-500/20',
      desc: 'Multiplicador de Frase Exata',
    },
    boost_recencia: {
      short: 'B-Novo',
      color: 'text-emerald-400 bg-emerald-500/10 border-emerald-500/20',
      desc: 'Multiplicador de Recência',
    },
    boost_riqueza: {
      short: 'B-Rico',
      color: 'text-fuchsia-400 bg-fuchsia-500/10 border-fuchsia-500/20',
      desc: 'Multiplicador de Riqueza',
    },
    boost_popularidade: {
      short: 'B-Pop',
      color: 'text-zinc-400 bg-zinc-500/10 border-zinc-500/20',
      desc: 'Multiplicador de Popularidade',
    },
    boost_links: {
      short: 'B-Links',
      color: 'text-orange-500 bg-orange-500/10 border-orange-500/20',
      desc: 'Multiplicador de Links',
    },
  };

  return (
    <div className="flex flex-wrap items-center gap-1.5 transition-all duration-300">
      <div
        className={`flex items-center gap-1 overflow-hidden transition-all duration-500 ${showDetails ? 'max-w-4xl opacity-100' : 'max-w-0 opacity-0'}`}
      >
        {Object.entries(details).map(([key, val]) => {
          if (
            val === 0 ||
            key === 'zinc_base' ||
            key === 'alpha_hybrid_base' ||
            key === 'raw_lexical_score' ||
            key === 'raw_semantic_sim'
          )
            return null;
          const config = WEIGHT_CONFIGS[key] || {
            short: key,
            color: 'text-zinc-500 border-zinc-500/20',
            desc: '',
          };

          // Lógica de Diferenciação Visual:
          const isMultiplier = key.startsWith('boost_') || key === 'proximidade_exata';

          return (
            <span
              key={key}
              title={config.desc}
              className={`px-1.5 py-0.5 rounded text-[8px] font-bold border uppercase tracking-tighter cursor-help transition-all shrink-0
                  ${config.color} hover:brightness-125`}
            >
              {config.short}: {isMultiplier ? 'x' : val > 0 ? '+' : ''}
              {val.toFixed(2)}
            </span>
          );
        })}
      </div>

      <button
        onClick={() => setShowDetails(!showDetails)}
        title={`${'Métricas de Re-Ranking'} (${'Última edição'}: ${formattedDate}) - ${'Clique para detalhar'}`}
        className={`flex items-center gap-1.5 px-2 py-0.5 rounded border transition-all active:scale-95 group leading-none
           ${
             showDetails
               ? 'bg-sky-500/10 border-sky-500/40 shadow-[0_0_15px_rgba(14,165,233,0.2)]'
               : 'bg-zinc-900 border-sky-500/30 hover:border-sky-500/50 hover:bg-zinc-800 shadow-[0_0_10px_rgba(14,165,233,0.1)]'
           }`}
      >
        <span
          className={`text-[7px] font-black uppercase tracking-tighter shrink-0 leading-none transition-colors ${showDetails ? 'text-sky-300' : 'text-zinc-500 group-hover:text-zinc-400'}`}
        >
          Rank
        </span>
        <span
          className={`text-[10px] font-black leading-none transition-colors ${showDetails ? 'text-sky-400' : 'text-sky-400/80 group-hover:text-sky-400'}`}
        >
          {score.toFixed(1)}
        </span>
        <div className="w-px h-2.5 bg-zinc-800 mx-0.5" />
        <span
          className={`text-[9px] font-bold font-mono tracking-tighter leading-none transition-colors ${showDetails ? 'text-zinc-300' : 'text-zinc-500 group-hover:text-zinc-400'}`}
        >
          {formattedDate}
        </span>
      </button>
    </div>
  );
};
