import { h } from 'preact';
import { render, screen, fireEvent } from '@testing-library/preact';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import MarkdownEditor from '../../components/MarkdownEditor';

// Mocks locais específicos (se necessário) podem vir aqui. 
// CodeMirror e outros já estão no setup.jsx global.

describe('MarkdownEditor Component', () => {
  const defaultProps = {
    fileName: 'notes/test.md',
    initialContent: '# Test Content',
    onSave: vi.fn(),
    onClose: vi.fn(),
    fetchWithAuth: vi.fn(() => Promise.resolve({ ok: true, json: () => Promise.resolve({ notes: [] }) }))
  };

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('deve renderizar o cabeçalho com o nome do arquivo', async () => {
    render(<MarkdownEditor {...defaultProps} />);
    expect(screen.getByText(/test/i)).toBeInTheDocument();
  });

  it('deve alternar para o modo preview ao clicar no botão Preview', async () => {
    render(<MarkdownEditor {...defaultProps} />);
    const previewBtn = screen.getByText(/Preview/i);
    fireEvent.click(previewBtn);
    
    // O texto do botão muda para 'Editar'
    expect(screen.getByText(/Editar/i)).toBeInTheDocument();
  });
 
  it('deve exibir modal de confirmação ao clicar em deletar', async () => {
    render(<MarkdownEditor {...defaultProps} />);
    const deleteBtn = screen.getByTitle(/Excluir nota/i);
    fireEvent.click(deleteBtn);
    
    expect(screen.getByText(/EXCLUIR ARQUIVO\?/i)).toBeInTheDocument();
  });
 
  it('deve disparar onClose ao clicar no botão fechar', () => {
    render(<MarkdownEditor {...defaultProps} />);
    const closeBtn = screen.getByTitle(/Fechar/i);
    fireEvent.click(closeBtn);
    
    expect(defaultProps.onClose).toHaveBeenCalled();
  });
});
