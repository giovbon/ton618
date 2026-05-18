import type { SearchResult } from '../types';

/**
 * Utilitário de ordenação para blindar o comportamento de ranking no Frontend.
 * Garante que a relevância (final_score) seja respeitada quando disponível.
 */
export const sortSearchResults = (
  results: SearchResult[],
  isSemanticEnabled: boolean,
  lastEditedNormalized: string | null,
  isCompactMode: boolean,
): SearchResult[] => {
  const getTS = (h: SearchResult) => h['@timestamp'] || h._source?.['@timestamp'] || '';
  const normalize = (f: string | null | undefined) => (f || '').replace(/\/+/g, '/').trim();

  let filtered = [...results];

  // 0. Deduplicação para Modo Compacto (Apenas 1 entrada por arquivo)
  if (isCompactMode) {
    const seen = new Set<string>();
    filtered = filtered.filter((h) => {
      const file = normalize(h.arquivo || h._source?.arquivo);
      if (!file || seen.has(file)) return false;
      seen.add(file);
      return true;
    });
  }

  return filtered.sort((a, b) => {
    const fileA = normalize(a.arquivo || a._source?.arquivo);
    const fileB = normalize(b.arquivo || b._source?.arquivo);

    // 1. Pinning: Arquivo editado por último sempre no topo
    if (lastEditedNormalized) {
      if (fileA === lastEditedNormalized) return -1;
      if (fileB === lastEditedNormalized) return 1;
    }

    // 2. Se for modo compacto, priorizamos recência pura (como um explorador de arquivos)
    if (isCompactMode) {
      return getTS(b).localeCompare(getTS(a));
    }

    // 3. Se semântica estiver ligada, confiamos na ordem do backend (híbrido)
    if (isSemanticEnabled) {
      // Nota: Se chegamos aqui e quisermos re-ordenar no front por segurança:
      const scoreA = a.final_score || 0;
      const scoreB = b.final_score || 0;
      if (Math.abs(scoreA - scoreB) > 0.0001) {
        return scoreB - scoreA;
      }
      return getTS(b).localeCompare(getTS(a));
    }

    // 4. Se semântica estiver desligada (Standard), priorizamos Rank se disponível
    const scoreA = a.final_score || 0;
    const scoreB = b.final_score || 0;

    if (Math.abs(scoreA - scoreB) > 0.001) {
      return scoreB - scoreA;
    }

    // 5. Empate no Rank ou Scores zerados -> Recência
    return getTS(b).localeCompare(getTS(a));
  });
};
