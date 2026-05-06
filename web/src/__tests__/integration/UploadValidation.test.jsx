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


describe('Validação de Upload de Arquivos', () => {
  let alertMock;

  beforeEach(() => {
    vi.clearAllMocks();
    localStorage.setItem('ton_auth', 'Basic fake-auth');
    alertMock = vi.fn();
    vi.stubGlobal('alert', alertMock);
    
    global.fetch = vi.fn().mockImplementation(() => 
      Promise.resolve({ ok: true, json: () => Promise.resolve({ tags: [] }) })
    );
  });

  const renderWithQuery = (component) => {
    return render(component);
  };



  it('Deve rejeitar PDF no botão de Imagem', async () => {
    renderWithQuery(<App />);

    // screen.debug(); // Útil para depurar se o componente montou corretamente
    
    // Encontra o input pelo test-id
    const imageInput = screen.getByTestId('camera-input');
    
    const file = new File(['conteudo'], 'documento.pdf', { type: 'application/pdf' });
    
    fireEvent.change(imageInput, { target: { files: [file] } });
    
    // Fallback: Despacha evento manual se o fireEvent falhar no JSDOM/Preact
    imageInput.dispatchEvent(new Event('change', { bubbles: true }));

    await waitFor(() => {
      expect(alertMock).toHaveBeenCalledWith(expect.stringContaining("exclusivo para Imagens"));
    }, { timeout: 2000 });
  });

  it('Deve rejeitar imagem no botão de PDF', async () => {
    localStorage.setItem('ton_auth', 'Basic fake-auth');
    renderWithQuery(<App />);

    
    // Garante que o input está presente (confirmando que passou do login)
    const pdfInput = screen.getByTestId('pdf-input');
    
    const file = new File(['conteudo'], 'foto.jpg', { type: 'image/jpeg' });
    
    fireEvent.change(pdfInput, { target: { files: [file] } });
    pdfInput.dispatchEvent(new Event('change', { bubbles: true }));

    await waitFor(() => {
      expect(alertMock).toHaveBeenCalledWith(expect.stringContaining("exclusivo para PDFs"));
    }, { timeout: 2000 });
  });

  it('Deve aceitar PDF no botão de PDF', async () => {
    renderWithQuery(<App />);

    
    const pdfInput = screen.getByTestId('pdf-input');
    const file = new File(['conteudo'], 'documento.pdf', { type: 'application/pdf' });
    
    // Mock do fetch para o upload
    const fetchMock = vi.fn().mockResolvedValue({ ok: true });
    global.fetch = fetchMock;

    fireEvent.change(pdfInput, { target: { files: [file], accept: '.pdf' } });

    // Não deve disparar alerta de erro de tipo
    expect(alertMock).not.toHaveBeenCalled();
  });
});
