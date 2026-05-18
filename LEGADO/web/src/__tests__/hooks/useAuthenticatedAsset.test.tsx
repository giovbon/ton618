import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderHook, waitFor } from '@testing-library/preact';
import { useAuthenticatedAsset } from '../../hooks/useAuthenticatedAsset';

describe('useAuthenticatedAsset', () => {
  const mockFetchWithAuth = vi.fn();
  const mockBlobUrl = 'blob:test-url';

  beforeEach(() => {
    vi.stubGlobal('URL', {
      createObjectURL: vi.fn(() => mockBlobUrl),
      revokeObjectURL: vi.fn(),
    });
  });

  afterEach(() => {
    vi.restoreAllMocks();
    vi.useRealTimers();
  });

  it('should start in loading state and eventually return a blobUrl', async () => {
    const mockBlob = new Blob(['test content'], { type: 'image/png' });
    mockFetchWithAuth.mockResolvedValueOnce({
      ok: true,
      blob: () => Promise.resolve(mockBlob),
    });

    const { result } = renderHook(() => 
      useAuthenticatedAsset('test.png', mockFetchWithAuth, true)
    );

    expect(result.current.isLoading).toBe(true);

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    }, { timeout: 1000 });

    expect(result.current.blobUrl).toBe(mockBlobUrl);
    expect(result.current.error).toBeNull();
    expect(mockFetchWithAuth).toHaveBeenCalledWith('/api/file?name=test.png');
    expect(URL.createObjectURL).toHaveBeenCalledWith(mockBlob);
  });

  it('should handle fetch errors correctly', async () => {
    mockFetchWithAuth.mockResolvedValueOnce({
      ok: false,
      status: 404,
    });

    const { result } = renderHook(() => 
      useAuthenticatedAsset('missing.png', mockFetchWithAuth, true)
    );

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    }, { timeout: 1000 });

    expect(result.current.error).toBe('Erro ao buscar arquivo: 404');
    expect(result.current.blobUrl).toBeNull();
  });

  it('should not fetch if active is false', () => {
    renderHook(() => 
      useAuthenticatedAsset('test.png', mockFetchWithAuth, false)
    );

    expect(mockFetchWithAuth).not.toHaveBeenCalled();
  });

  it('should revoke the URL after a delay when unmounting', async () => {
    vi.useFakeTimers();
    const mockBlob = new Blob(['test content']);
    mockFetchWithAuth.mockResolvedValueOnce({
      ok: true,
      blob: () => Promise.resolve(mockBlob),
    });

    const { unmount, result } = renderHook(() => 
      useAuthenticatedAsset('test.png', mockFetchWithAuth, true)
    );

    // Esperar carregar usando advanceTimers se necessário ou real timers
    // Mas aqui estamos com fake timers. waitFor vai sofrer.
    // Melhor: disparar o carregamento e então avançar.
    
    await vi.runAllTimersAsync();

    unmount();

    // O revokeObjectURL é chamado dentro de um setTimeout de 10s
    expect(URL.revokeObjectURL).not.toHaveBeenCalled();

    vi.advanceTimersByTime(10001);

    expect(URL.revokeObjectURL).toHaveBeenCalledWith(mockBlobUrl);
    vi.useRealTimers();
  });
});
