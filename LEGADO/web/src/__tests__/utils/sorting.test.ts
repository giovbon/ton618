import { describe, expect, it } from 'vitest';
import type { SearchResult } from '../../types';
import { sortSearchResults } from '../../utils/sorting';

describe('sortSearchResults', () => {
  const mockResults: SearchResult[] = [
    {
      id: '1',
      arquivo: 'antigo.md',
      texto: '...',
      tipo: 'note',
      final_score: 10.5,
      '@timestamp': '2026-04-01T10:00:00Z',
    },
    {
      id: '2',
      arquivo: 'novo.md',
      texto: '...',
      tipo: 'note',
      final_score: 5.2,
      '@timestamp': '2026-04-20T10:00:00Z',
    },
    {
      id: '3',
      arquivo: 'relevante.md',
      texto: '...',
      tipo: 'note',
      final_score: 15.0,
      '@timestamp': '2026-04-10T10:00:00Z',
    },
  ];

  it('deve priorizar final_score (Rank) sobre a data no modo Standard', () => {
    const sorted = sortSearchResults(mockResults, false, null, false);

    expect(sorted[0].id).toBe('3'); // Score 15.0
    expect(sorted[1].id).toBe('1'); // Score 10.5
    expect(sorted[2].id).toBe('2'); // Score 5.2
  });

  it('deve priorizar a data no modo Compacto', () => {
    const sorted = sortSearchResults(mockResults, false, null, true);

    expect(sorted[0].id).toBe('2'); // 2026-04-20
    expect(sorted[1].id).toBe('3'); // 2026-04-10
    expect(sorted[2].id).toBe('1'); // 2026-04-01
  });

  it('deve respeitar o pinning do último arquivo editado', () => {
    const sorted = sortSearchResults(mockResults, false, 'novo.md', false);

    expect(sorted[0].id).toBe('2'); // novo.md está no topo pelo pinning
    expect(sorted[1].id).toBe('3'); // Segue o rank
    expect(sorted[2].id).toBe('1');
  });

  it('deve desempatar por data se os scores forem iguais', () => {
    const tieResults: SearchResult[] = [
      {
        id: 'a',
        arquivo: 'a.md',
        texto: '',
        tipo: 'note',
        final_score: 10,
        '@timestamp': '2026-01-01T00:00:00Z',
      },
      {
        id: 'b',
        arquivo: 'b.md',
        texto: '',
        tipo: 'note',
        final_score: 10,
        '@timestamp': '2026-02-01T00:00:00Z',
      },
    ];
    const sorted = sortSearchResults(tieResults, false, null, false);
    expect(sorted[0].id).toBe('b'); // Mais recente
  });
});
