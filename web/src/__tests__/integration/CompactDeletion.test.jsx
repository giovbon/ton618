import { render, screen, fireEvent, waitFor } from '@testing-library/preact';
import { App } from '../../App';
import { vi, describe, it, expect, beforeEach } from 'vitest';

// Mocks para isolar o comportamento do App
vi.mock('@tanstack/react-query', () => ({
  useQueryClient: vi.fn(() => ({
    invalidateQueries: vi.fn(),
    setQueriesData: vi.fn(),
  })),
  QueryClient: vi.fn(),
  QueryClientProvider: ({ children }) => <div>{children}</div>,
}));

vi.mock('../../hooks/useSearchQuery', () => ({
  useSearchQuery: vi.fn(() => ({
    data: { 
      pages: [{ 
        hits: [{ 
          id: 'test-id',
          arquivo: 'notes/test-note.md',
          texto: 'Conteúdo de teste',
          tipo: 'note',
          '@timestamp': new Date().toISOString()
        }], 
        total: 1 
      }] 
    },
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

describe('Fluxo de Exclusão no Modo Compacto', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    localStorage.setItem('ton_auth', 'Basic fake-auth');
    localStorage.setItem('ton_compact_mode', 'true');
    
    global.fetch = vi.fn().mockImplementation((url) => {
      if (url === '/api/tags') {
        return Promise.resolve({
          ok: true,
          json: () => Promise.resolve({ tags: [] })
        });
      }
      if (url === '/api/settings') {
        return Promise.resolve({
          ok: true,
          json: () => Promise.resolve({ semantic_enable: false })
        });
      }
      return Promise.resolve({ ok: true });
    });
  });

  it('Deve abrir o modal de confirmação ao clicar no botão de excluir no modo compacto', async () => {
    render(<App />);

    // Aguardar o carregamento do card
    await waitFor(() => {
      expect(screen.getByText(/test-note/i)).toBeInTheDocument();
    });

    // Encontrar o botão de excluir pelo title
    const deleteBtn = screen.getByTitle(/Excluir arquivo/i);
    expect(deleteBtn).toBeInTheDocument();

    // Clicar no botão de excluir
    fireEvent.click(deleteBtn);

    // Verificar se o modal de confirmação apareceu com timeout maior
    await waitFor(() => {
      expect(screen.getByText(/EXCLUIR ARQUIVO\?/i)).toBeInTheDocument();
    }, { timeout: 3000 });

    // Verificar se o botão de confirmação está presente
    const confirmBtn = screen.getByText(/Sim, Excluir Agora/i);
    expect(confirmBtn).toBeInTheDocument();
    
    // Clicar no botão de confirmação
    fireEvent.click(confirmBtn);
    
    // Verificar se o fetch DELETE foi chamado (isso valida que a prop onConfirm está correta)
    await waitFor(() => {
      expect(global.fetch).toHaveBeenCalledWith(
        expect.stringContaining('/api/file'),
        expect.objectContaining({ method: 'DELETE' })
      );
    });
  });
});
