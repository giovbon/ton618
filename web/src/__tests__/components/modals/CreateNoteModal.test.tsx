import { h } from 'preact';
import { render, screen, fireEvent, waitFor } from '@testing-library/preact';
import { describe, it, expect, vi } from 'vitest';
import { CreateNoteModal } from '../../../components/modals/CreateNoteModal';

describe('CreateNoteModal Component', () => {
  const defaultProps = {
    isOpen: true,
    onClose: vi.fn(),
    onSubmit: vi.fn()
  };

  it('não deve renderizar se isOpen for false', () => {
    const { container } = render(<CreateNoteModal {...defaultProps} isOpen={false} />);
    expect(container.firstChild).toBeNull();
  });

  it('deve renderizar corretamente quando aberto', () => {
    render(<CreateNoteModal {...defaultProps} />);
    expect(screen.getByText(/Criar Nova Nota/i)).toBeInTheDocument();
    expect(screen.getByPlaceholderText(/nome-da-nota/i)).toBeInTheDocument();
  });

  it('deve gerenciar o estado local do input', () => {
    render(<CreateNoteModal {...defaultProps} />);
    const input = screen.getByPlaceholderText(/nome-da-nota/i);
    
    fireEvent.input(input, { target: { value: 'minha-nova-nota' } });
    expect(input.value).toBe('minha-nova-nota');
  });

  it('deve chamar onSubmit com o valor do input ao submeter', () => {
    render(<CreateNoteModal {...defaultProps} />);
    const input = screen.getByPlaceholderText(/nome-da-nota/i);
    const form = screen.getByRole('textbox').closest('form');
    
    fireEvent.input(input, { target: { value: 'test-note' } });
    fireEvent.submit(form);
    
    expect(defaultProps.onSubmit).toHaveBeenCalledWith('test-note');
  });

  it('deve desabilitar o botão de criar se o input estiver vazio', () => {
    render(<CreateNoteModal {...defaultProps} />);
    const button = screen.getByText(/Criar Agora/i);
    expect(button).toBeDisabled();
    
    const input = screen.getByPlaceholderText(/nome-da-nota/i);
    fireEvent.input(input, { target: { value: 'abc' } });
    expect(button).not.toBeDisabled();
  });

  it('deve chamar onClose ao clicar no botão cancelar', () => {
    render(<CreateNoteModal {...defaultProps} />);
    const cancelButton = screen.getByText(/Cancelar/i);
    fireEvent.click(cancelButton);
    expect(defaultProps.onClose).toHaveBeenCalled();
  });

  it('deve chamar onClose ao clicar no backdrop', () => {
    const { container } = render(<CreateNoteModal {...defaultProps} />);
    const backdrop = container.querySelector('.absolute.inset-0.bg-black\\/60');
    fireEvent.click(backdrop);
    expect(defaultProps.onClose).toHaveBeenCalled();
  });
});
