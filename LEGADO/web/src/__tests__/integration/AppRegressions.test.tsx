import { render, screen, fireEvent, waitFor } from '@testing-library/preact';
import { App } from '../../App';
// Mock dos hooks
vi.mock('@tanstack/react-query', () => ({
  useQueryClient: vi.fn(() => ({
    invalidateQueries: vi.fn(),
  })),
  QueryClient: vi.fn(),
  QueryClientProvider: ({ children }) => <div>{children}</div>,
}));

vi.mock('../../hooks/useSearchQuery', () => ({
  useSearchQuery: vi.fn(() => ({
    data: { pages: [{ hits: [], total: 0 }] },
    fetchNextPage: vi.fn(),
    hasNextPage: false,
    isFetching: false,
    isFetchingNextPage: false,
    isLoading: false,
    error: null
  }))
}));


vi.mock('../../hooks/useSSE', () => ({
  useSSE: vi.fn()
}));


describe('Regressões do Frontend TON-618', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    localStorage.setItem('ton_auth', 'Basic fake-auth');
    
    // Mock do fetch para o refreshAvailableTags
    global.fetch = vi.fn().mockImplementation((url) => {
      if (url === '/api/tags') {
        return Promise.resolve({
          ok: true,
          json: () => Promise.resolve({ tags: ['golang', 'teste', 'ai'], total: 3 })
        });
      }
      return Promise.resolve({ ok: true, text: () => Promise.resolve('conteúdo da nota') });
    });
  });

  const renderWithQuery = (component) => {
    return render(component);
  };


  it('Deve garantir a integridade dos imports (Smoke Test)', async () => {

    // Este teste valida indiretamente o import do MarkdownEditor. 
    // Se o import estivesse quebrado (undefined), a renderização do App falharia.
    const { getByText } = renderWithQuery(<App />);

    
    // Aguardar o render inicial
    await waitFor(() => {
      expect(getByText(/TON-618/i)).toBeInTheDocument();
    });
  });
});
