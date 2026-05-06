import { beforeEach, vi, expect } from 'vitest';
import * as matchers from '@testing-library/jest-dom/matchers';
import '@testing-library/jest-dom';
import { h } from 'preact';

// Estende os matchers do Vitest com as funções do Testing Library (ex: toBeInTheDocument)
expect.extend(matchers);

// Mock global do fetch para evitar erros nos testes legados
global.fetch = vi.fn();

// Mock do Virtuoso para garantir renderização em ambiente JSDOM
vi.mock('react-virtuoso', () => {
  return {
    Virtuoso: (props) => {
      const { data, itemContent, components } = props;
      if (!data || !itemContent) return <div id="virtuoso-empty" />;

      return (
        <div id="virtuoso-mock">
          {data.map((item, index) => (
            <div key={index} id={`item-${index}`}>
              {itemContent(index, item)}
            </div>
          ))}
          {components?.Footer && <components.Footer />}
        </div>
      );
    }
  };
});

beforeEach(() => {
  // Mock da autenticação global para os testes
  window.localStorage.setItem('ton_auth', 'Basic YWRtaW46c2Vla2VyMTIz');

  // Limpa os mocks entre os testes
  vi.clearAllMocks();
});
// Mock do CodeMirror e suas extensões para evitar erros de renderização no JSDOM
vi.mock('@codemirror/view', () => {
  const EditorView = vi.fn().mockImplementation(() => ({
    destroy: vi.fn(),
    dispatch: vi.fn(),
    focus: vi.fn(),
    state: { doc: { toString: () => "conteudo teste" }, selection: { main: { from: 0, to: 0 } } },
    dom: document.createElement('div')
  }));

  EditorView.theme = vi.fn().mockReturnValue({});
  EditorView.baseTheme = vi.fn().mockReturnValue({});
  EditorView.updateListener = { of: vi.fn() };
  EditorView.lineWrapping = {};

  return {
    EditorView,
    ViewPlugin: { fromClass: vi.fn().mockReturnValue({}) },
    Decoration: { widget: vi.fn().mockReturnValue({}), replace: vi.fn().mockReturnValue({}), mark: vi.fn().mockReturnValue({}) },
    keymap: { of: vi.fn() },
    highlightActiveLine: vi.fn(),
    placeholder: vi.fn(),
    drawSelection: vi.fn(),
    dropCursor: vi.fn(),
    rectangularSelection: vi.fn(),
    crosshairCursor: vi.fn()
  };
});

vi.mock('@codemirror/state', () => ({
  EditorState: { create: vi.fn(() => ({ doc: { toString: () => "" } })) }
}));

vi.mock('@codemirror/autocomplete', () => ({
  autocompletion: vi.fn(),
  completionKeymap: [],
  closeBrackets: vi.fn(),
  closeBracketsKeymap: []
}));

vi.mock('@codemirror/lang-markdown', () => ({
  markdown: vi.fn(),
  markdownLanguage: {}
}));

vi.mock('@codemirror/language-data', () => ({
  languages: []
}));

vi.mock('@codemirror/search', () => ({
  search: vi.fn(),
  searchKeymap: [],
  highlightSelectionMatches: vi.fn(),
  selectNextOccurrence: vi.fn()
}));

vi.mock('@codemirror/language', () => ({
  foldGutter: vi.fn(),
  foldKeymap: [],
  defaultHighlightStyle: {},
  syntaxHighlighting: vi.fn(),
  indentUnit: { of: vi.fn() }
}));

vi.mock('@codemirror/theme-one-dark', () => ({
  oneDark: {}
}));

// Mock do tldraw
vi.mock('tldraw', () => {
  return {
    Tldraw: ({ onMount }) => {
      if (onMount) {
        Promise.resolve().then(() => {
          onMount({
            store: {
              getSnapshot: () => ({ message: 'fake-snapshot' }),
              listen: () => () => {},
              loadSnapshot: vi.fn()
            }
          });
        });
      }
      return <div data-testid="tldraw-canvas">MOCK_TLDRAW_CANVAS</div>;
    },
    useEditor: vi.fn(),
    createTLStore: vi.fn(),
    defaultEditorAssetUrls: {}
  };
});
// Mock do i18n para evitar erros de context em todos os testes
vi.mock('../hooks/useI18n', () => ({
  useI18n: () => ({
    t: (key, def) => {
      const translations = {
        'common.cancel': 'cancelar',
        'common.close': 'Fechar',
        'editor.delete_confirm': 'deletar',
        'editor.saving': 'salvando',
        'editor.delete_title': 'Excluir Nota',
        'editor.delete_confirm_title': 'EXCLUIR ARQUIVO?',
        'editor.delete_confirm_btn': 'Sim, Excluir Agora',
        'editor.delete_warning': 'Esta ação não pode ser desfeita.',
        'search.delete_file': 'Excluir arquivo'
      };
      return translations[key] || def || key;
    },
    i18n: { language: 'pt' },
    changeLanguage: vi.fn()
  }),
  I18nProvider: ({ children }) => children
}));

// Mock global do Worker para JSDOM
class MockWorker {
  onmessage = (e) => {};
  postMessage(data) {
    setTimeout(() => {
      this.onmessage({ data: { id: data.id, html: data.text, error: null } });
    }, 10);
  }
  terminate() {}
  addEventListener() {}
  removeEventListener() {}
}
global.Worker = MockWorker;

// Mock global do IntersectionObserver para JSDOM
const IntersectionObserverMock = vi.fn(() => ({
  disconnect: vi.fn(),
  observe: vi.fn(),
  takeRecords: vi.fn(),
  unobserve: vi.fn(),
}));
vi.stubGlobal('IntersectionObserver', IntersectionObserverMock);


