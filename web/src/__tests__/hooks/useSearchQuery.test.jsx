import { describe, it, expect, vi, beforeEach } from 'vitest';
import { searchFetcher } from '../../hooks/useSearchQuery';

// Mock dos utilitários de busca
vi.mock('../../utils/search/globalSearch', () => ({
  buildGlobalPayload: vi.fn((query, page) => ({ query, from: page })),
  processGlobalResults: vi.fn((hits) => hits.map(h => ({ ...h, processed: true })))
}));

vi.mock('../../utils/search/compactSearch', () => ({
  buildCompactPayload: vi.fn((query, page) => ({ query, from: page, compact: true })),
  processCompactResults: vi.fn((hits) => hits.map(h => ({ ...h, compactProcessed: true })))
}));

describe('searchFetcher logic', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    global.fetch = vi.fn();
  });

  it('deve retornar resultados de busca global processados', async () => {
    const mockHits = [{ arquivo: 'nota1.md', texto: 'conteudo' }];
    global.fetch.mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({
        hits: {
          hits: mockHits,
          total: { value: 1 }
        }
      })
    });

    const result = await searchFetcher({
      query: 'teste',
      isCompact: false,
      authHeader: 'Bearer token'
    });

    expect(result.hits[0].processed).toBe(true);
    expect(result.total).toBe(1);
    expect(result.nextOffset).toBe(null); // Menos de 50 resultados
  });

  it('deve processar resultados em modo compacto', async () => {
    global.fetch.mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({
        hits: { hits: [{ arquivo: 'nota.md' }], total: { value: 1 } }
      })
    });

    const result = await searchFetcher({
      query: 'teste',
      isCompact: true,
      authHeader: 'Bearer token'
    });

    expect(result.hits[0].compactProcessed).toBe(true);
    expect(result.total).toBe(0); // Comportamento esperado no modo compacto
  });

  it('deve sinalizar unauthorized e disparar callback no status 401', async () => {
    const onUnauthorized = vi.fn();
    global.fetch.mockResolvedValue({
      ok: false,
      status: 401
    });

    await expect(searchFetcher({
      query: 'teste',
      isCompact: false,
      authHeader: 'Bearer token',
      onUnauthorized
    })).rejects.toThrow("Unauthorized");

    expect(onUnauthorized).toHaveBeenCalled();
  });

  it('deve calcular corretamente o nextOffset para paginação', async () => {
    // Simula uma página cheia (50 itens)
    const page1Hits = Array.from({ length: 50 }, (_, i) => ({ arquivo: `f${i}.md` }));

    global.fetch.mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({
        hits: { hits: page1Hits, total: { value: 100 } }
      })
    });

    const result = await searchFetcher({
      query: 'teste',
      isCompact: false,
      authHeader: 'Bearer token',
      pageParam: 0
    });

    expect(result.nextOffset).toBe(50);
  });

  it('deve retornar vazio se authHeader estiver ausente', async () => {
    const result = await searchFetcher({
      query: 'teste',
      isCompact: false,
      authHeader: null
    });

    expect(result.hits).toEqual([]);
    expect(global.fetch).not.toHaveBeenCalled();
  });

  it('deve lançar erro genérico para falhas na API', async () => {
    global.fetch.mockResolvedValue({
      ok: false,
      status: 500
    });

    await expect(searchFetcher({
      query: 'teste',
      isCompact: false,
      authHeader: 'Bearer token'
    })).rejects.toThrow("Erro na busca: 500");
  });
});
