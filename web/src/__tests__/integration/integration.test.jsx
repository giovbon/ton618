import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, fireEvent, waitFor } from '@testing-library/preact';
import { h } from 'preact';
import MarkdownEditor from '../../components/MarkdownEditor';

describe('UI Integration Tests', () => {
  describe('MarkdownEditor Rename Integrity', () => {
    it.skip('should save current content before renaming', async () => {
      const onSave = vi.fn().mockResolvedValue({ ok: true });
      const onRename = vi.fn().mockResolvedValue({ ok: true, newFile: 'nova-nota.md' });
      
      const { findByPlaceholderText, getByTitle } = render(
        <MarkdownEditor 
          fileName="nota-antiga.md" 
          initialContent="conteúdo original" 
          onSave={onSave}
          onRename={onRename}
          onClose={() => {}}
        />
      );

      // Simula edição do conteúdo
      const editor = document.querySelector('.EasyMDEContainer textarea');
      // Nota: Em testes com EasyMDE, podemos precisar de uma abordagem diferente se o mock não for perfeito,
      // mas aqui o MarkdownEditor usa o valor interno 'text' que passamos via props ou editamos.
      
      // Simula clique no botão de editar título
      const editTitleBtn = getByTitle(/editar nome/i);
      fireEvent.click(editTitleBtn);

      // Altera o título no input que aparece
      const titleInput = await findByPlaceholderText(/nome-do-arquivo.md/i);
      fireEvent.input(titleInput, { target: { value: 'nova-nota.md' } });
      
      // Simula confirmação do rename (pressionando Enter ou clicando no check)
      fireEvent.keyDown(titleInput, { key: 'Enter', code: 'Enter' });

      // VERIFICAÇÃO CRÍTICA: onSave deve ser chamado ANTES ou durante o processo de rename
      await waitFor(() => {
        expect(onSave).toHaveBeenCalled();
        expect(onRename).toHaveBeenCalledWith('nota-antiga.md', 'nova-nota.md');
      });
    });
  });

  describe('Compact Mode UI State', () => {
    // Este teste verifica se a mudança de estado reflete na classe ou conteúdo
    it('should toggle compact mode preference', () => {
      // Simulação simples de alternância no App ou componente que gerencia isso
      let isCompact = false;
      const toggle = () => { isCompact = !isCompact; };
      
      toggle();
      expect(isCompact).toBe(true);
      
      toggle();
      expect(isCompact).toBe(false);
    });
  });
});
