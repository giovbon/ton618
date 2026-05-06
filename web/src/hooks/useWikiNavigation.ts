import { useCallback, useEffect } from 'preact/compat';

/**
 * Hook para centralizar a navegação via WikiLinks.
 * @param onOpenNote Função chamada quando um WikiLink é clicado. Recebe o nome da nota.
 * @param active Se o listener deve estar ativo.
 */
export function useWikiNavigation(onOpenNote: (noteName: string) => void, active: boolean = true) {
  const handleGlobalClick = useCallback(
    (e: MouseEvent) => {
      const target = e.target as HTMLElement | null;
      if (!target) return;

      const wikiLink = target.closest('.wikilink');
      if (wikiLink) {
        const noteName = wikiLink.getAttribute('data-note');
        if (noteName) {
          console.log('VORTEX: WikiLink detectado via useWikiNavigation:', noteName);
          e.preventDefault();
          e.stopPropagation();
          onOpenNote(noteName);
        }
      }
    },
    [onOpenNote],
  );

  useEffect(() => {
    if (!active) return;

    document.addEventListener('click', handleGlobalClick, true);
    return () => document.removeEventListener('click', handleGlobalClick, true);
  }, [active, handleGlobalClick]);
}
