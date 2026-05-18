import { h } from 'preact';
import { render, screen, fireEvent } from '@testing-library/preact';
import { describe, it, expect, vi } from 'vitest';
import { CaptureLinkModal } from '../../../components/modals/CaptureLinkModal';

describe('CaptureLinkModal Component', () => {
  const defaultProps = {
    isOpen: true,
    isProcessing: false,
    onClose: vi.fn(),
    onSubmit: vi.fn()
  };

  it('não deve renderizar se isOpen for false', () => {
    const { container } = render(<CaptureLinkModal {...defaultProps} isOpen={false} />);
    expect(container.firstChild).toBeNull();
  });

  it('deve renderizar corretamente quando aberto', () => {
    render(<CaptureLinkModal {...defaultProps} />);
    expect(screen.getByText(/Capturar Link/i)).toBeInTheDocument();
    expect(screen.getByPlaceholderText(/https:\/\/exemplo.com/i)).toBeInTheDocument();
  });

  it('deve gerenciar o estado local da URL', () => {
    render(<CaptureLinkModal {...defaultProps} />);
    const input = screen.getByPlaceholderText(/https:\/\/exemplo.com/i);
    
    fireEvent.input(input, { target: { value: 'https://google.com' } });
    expect(input.value).toBe('https://google.com');
  });

  it('deve chamar onSubmit com a URL ao submeter', () => {
    render(<CaptureLinkModal {...defaultProps} />);
    const input = screen.getByPlaceholderText(/https:\/\/exemplo.com/i);
    const form = input.closest('form');
    
    fireEvent.input(input, { target: { value: 'https://vortex.pkm' } });
    fireEvent.submit(form);
    
    expect(defaultProps.onSubmit).toHaveBeenCalledWith('https://vortex.pkm');
  });

  it('deve exibir estado de carregamento e desabilitar botões quando isProcessing é true', () => {
    render(<CaptureLinkModal {...defaultProps} isProcessing={true} />);
    
    expect(screen.getByText(/Extraindo texto.../i)).toBeInTheDocument();
    const submitBtn = screen.getByText(/Extraindo texto.../i);
    expect(submitBtn).toBeDisabled();
  });

  it('não deve fechar ao clicar no backdrop se estiver processando', () => {
    const { container } = render(<CaptureLinkModal {...defaultProps} isProcessing={true} />);
    const backdrop = container.querySelector('.absolute.inset-0.bg-black\\/60');
    
    fireEvent.click(backdrop);
    expect(defaultProps.onClose).not.toHaveBeenCalled();
  });

  it('deve chamar onClose ao clicar no botão cancelar e não estiver processando', () => {
    render(<CaptureLinkModal {...defaultProps} />);
    const cancelButton = screen.getByText(/Cancelar/i);
    fireEvent.click(cancelButton);
    expect(defaultProps.onClose).toHaveBeenCalled();
  });
});
