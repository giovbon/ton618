import { useEffect, useRef, useState } from 'preact/hooks';

interface IntersectionOptions {
  threshold?: number | number[];
  rootMargin?: string;
  root?: Element | null;
}

/**
 * useIntersectionObserver - Detecta se um elemento está visível no viewport.
 */
export function useIntersectionObserver(
  options: IntersectionOptions = { threshold: 0.1, rootMargin: '100px' },
) {
  const [isInView, setIsInView] = useState(false);
  const ref = useRef<HTMLElement>(null);

  useEffect(() => {
    const element = ref.current;
    if (!element) return;

    const observer = new IntersectionObserver(([entry]) => {
      // Uma vez que entrou no view, mantemos como true (lazy load standard)
      if (entry.isIntersecting) {
        setIsInView(true);
        observer.unobserve(element);
      }
    }, options);

    observer.observe(element);

    return () => {
      if (element) observer.unobserve(element);
    };
  }, [options.threshold, options.rootMargin, options]);

  return { ref, isInView };
}
