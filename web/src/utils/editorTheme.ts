import { EditorView } from '@codemirror/view';

export const ton618Theme = EditorView.theme(
  {
    '&': {
      height: '100%',
      backgroundColor: 'transparent',
      color: '#d4d4d8',
      fontSize: '15px',
    },
    '.cm-content': {
      fontFamily: "'JetBrains Mono', monospace",
      padding: '20px 10px',
      lineHeight: '1.7',
    },
    '.cm-gutters': {
      backgroundColor: '#282c34',
      color: '#5c6370',
      border: 'none',
    },
    '.cm-activeLine': {
      backgroundColor: 'rgba(39, 39, 42, 0.3)',
    },
    // Estilização de Markdown no editor (Hybrid mode)
    '.cm-header': { fontWeight: 'bold', color: '#fff' },
    '.cm-strong': { fontWeight: 'bold', color: '#fff' },
    '.cm-emphasis': { fontStyle: 'italic', color: '#fff' },
    '.cm-link': { color: '#0ea5e9', textDecoration: 'underline' },
    '.cm-url': { color: '#52525b' },
    '.cm-comment': { color: '#52525b' },
    '.cm-scroller': {
      overflow: 'auto',
    },
  },
  { dark: true },
);

export const foldMarker = (open: boolean) => {
  const el = document.createElement('div');
  el.className = 'vortex-fold-marker';
  el.innerHTML = open ? '▼' : '▶';
  return el;
};
