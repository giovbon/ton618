import { describe, it, expect, vi, beforeEach } from "vitest";
import { searchFetcher } from "../../hooks/useSearchQuery";

describe("searchFetcher", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    global.fetch = vi.fn();
  });

  it("should return empty results when no auth header", async () => {
    const result = await searchFetcher({
      query: "test",
      isCompact: false,
      authHeader: null,
    });
    expect(result.hits).toEqual([]);
    expect(result.total).toBe(0);
  });

  it("should build correct payload for normal search", async () => {
    (global.fetch as any).mockResolvedValueOnce({
      ok: true,
      json: async () => ({
        hits: {
          hits: [{ id: "1", _source: { id: "1", arquivo: "test.md", texto: "hello", tipo: "note" }, _score: 1.0 }],
          total: { value: 1 },
        },
      }),
    });

    const result = await searchFetcher({
      query: "hello world",
      isCompact: false,
      authHeader: "Basic YWRtaW46dGVzdA==",
    });

    expect(global.fetch).toHaveBeenCalledTimes(1);
    const callArgs = (global.fetch as any).mock.calls[0];
    const body = JSON.parse(callArgs[1].body);
    expect(body.query.term).toBe("hello world");
    expect(body.from).toBe(0);
    expect(body.size).toBe(50);
    expect(callArgs[1].headers.Authorization).toBe("Basic YWRtaW46dGVzdA==");
    expect(result.hits.length).toBe(1);
    expect(result.total).toBe(1);
  });

  it("should build compact payload", async () => {
    (global.fetch as any).mockResolvedValueOnce({
      ok: true,
      json: async () => ({ hits: { hits: [] } }),
    });

    await searchFetcher({
      query: "note",
      isCompact: true,
      authHeader: "Basic test",
    });

    const body = JSON.parse((global.fetch as any).mock.calls[0][1].body);
    expect(body.compact).toBe(true);
    expect(body.size).toBe(50);
  });

  it("should handle pagination offset", async () => {
    (global.fetch as any).mockResolvedValueOnce({
      ok: true,
      json: async () => ({
        hits: { hits: [], total: { value: 0 } },
      }),
    });

    await searchFetcher({
      query: "test",
      isCompact: false,
      authHeader: "Basic test",
      pageParam: 50,
    });

    const body = JSON.parse((global.fetch as any).mock.calls[0][1].body);
    expect(body.from).toBe(50);
  });

  it("should throw on 401 and call onUnauthorized", async () => {
    const onUnauthorized = vi.fn();
    (global.fetch as any).mockResolvedValueOnce({
      ok: false,
      status: 401,
    });

    await expect(
      searchFetcher({
        query: "test",
        isCompact: false,
        authHeader: "Basic test",
        onUnauthorized,
      }),
    ).rejects.toThrow("Unauthorized");

    expect(onUnauthorized).toHaveBeenCalled();
  });

  it("should throw on other errors", async () => {
    (global.fetch as any).mockResolvedValueOnce({
      ok: false,
      status: 500,
    });

    await expect(
      searchFetcher({
        query: "test",
        isCompact: false,
        authHeader: "Basic test",
      }),
    ).rejects.toThrow("Erro na busca: 500");
  });

  it("should include t parameter for cache busting", async () => {
    (global.fetch as any).mockResolvedValueOnce({
      ok: true,
      json: async () => ({ hits: { hits: [], total: { value: 0 } } }),
    });

    await searchFetcher({
      query: "test",
      isCompact: false,
      authHeader: "Basic test",
    });

    const url = (global.fetch as any).mock.calls[0][0];
    expect(url).toMatch(/\/api\/search\?t=\d+/);
  });

  it("should handle semantic search mode", async () => {
    (global.fetch as any).mockResolvedValueOnce({
      ok: true,
      json: async () => ({ hits: { hits: [], total: { value: 0 } } }),
    });

    await searchFetcher({
      query: "test",
      isCompact: false,
      authHeader: "Basic test",
      isSemantic: true,
    });

    const body = JSON.parse((global.fetch as any).mock.calls[0][1].body);
    expect(body.semantic).toBe(true);
  });
});
