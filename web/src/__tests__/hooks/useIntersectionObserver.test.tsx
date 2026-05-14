import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook, act } from '@testing-library/preact';
import { useIntersectionObserver } from '../../hooks/useIntersectionObserver';

describe('useIntersectionObserver', () => {
  let observerCallback;
  const mockObserve = vi.fn();
  const mockUnobserve = vi.fn();
  const mockDisconnect = vi.fn();

  beforeEach(() => {
    vi.stubGlobal('IntersectionObserver', vi.fn((callback) => {
      observerCallback = callback;
      return {
        observe: mockObserve,
        unobserve: mockUnobserve,
        disconnect: mockDisconnect,
      };
    }));
    vi.clearAllMocks();
  });

  it('should initialize with isInView as false', () => {
    const { result } = renderHook(() => useIntersectionObserver());
    expect(result.current.isInView).toBe(false);
  });

  it('should update isInView to true when element intersects', () => {
    // Usamos renderHook de uma forma que possamos forçar o useEffect a rodar com o ref preenchido
    const { result, rerender } = renderHook(({ options }) => useIntersectionObserver(options), {
      initialProps: { options: { threshold: 0.1 } }
    });
    
    const element = document.createElement('div');
    result.current.ref.current = element;

    // Forçamos o re-run do effect alterando as opções (ou apenas disparando rerender se o hook dependesse do ref)
    // Como o hook depende de options.threshold e options.rootMargin, mudamos um deles ligeiramente
    rerender({ options: { threshold: 0.1, rootMargin: '101px' } });

    // Simular a chamada do observer pelo browser
    act(() => {
      if (observerCallback) {
        observerCallback([{ isIntersecting: true }]);
      }
    });

    expect(result.current.isInView).toBe(true);
    expect(mockUnobserve).toHaveBeenCalledWith(element);
  });

  it('should cleanup observer on unmount', () => {
    const { unmount, result, rerender } = renderHook(({ options }) => useIntersectionObserver(options), {
      initialProps: { options: { threshold: 0.1 } }
    });
    
    const element = document.createElement('div');
    result.current.ref.current = element;
    rerender({ options: { threshold: 0.1, rootMargin: '101px' } });
    
    unmount();
    expect(mockUnobserve).toHaveBeenCalled();
  });
});
