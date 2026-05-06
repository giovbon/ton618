import { describe, it, expect } from 'vitest';
import { stripFrontmatter, getWikiLinkMatch } from '../../utils/editor';

describe('EditorLogic Utilities', () => {
  describe('stripFrontmatter', () => {
    it('deve remover o frontmatter de um documento válido', () => {
      const content = '---\ntags: [test]\ntitle: Test\n---\n# Real Content';
      expect(stripFrontmatter(content)).toBe('# Real Content');
    });

    it('deve manter o conteúdo intacto se não houver frontmatter', () => {
      const content = '# No Frontmatter\nJust text.';
      expect(stripFrontmatter(content)).toBe(content);
    });

    it('deve lidar com documentos vazios', () => {
      expect(stripFrontmatter('')).toBe('');
    });

    it('não deve remover se o frontmatter não fechar', () => {
      const content = '---\ntags: [test]\n# Just content with triple dash start';
      expect(stripFrontmatter(content)).toBe(content);
    });

    it('deve remover as quebras de linha após o fechamento do frontmatter', () => {
      const content = '---\ntitle: test\n---\n\n# Header';
      expect(stripFrontmatter(content)).toBe('# Header');
    });
  });

  describe('getWikiLinkMatch', () => {
    it('deve detectar o gatilho de WikiLink no final de uma linha', () => {
      expect(getWikiLinkMatch('Esta é uma nota [[Target')).toBe('Target');
    });

    it('deve retornar null se não houver um [[ aberto', () => {
      expect(getWikiLinkMatch('Texto comum sem link')).toBeNull();
    });

    it('deve retornar string vazia se o [[ acabou de ser aberto', () => {
      expect(getWikiLinkMatch('Link vazio [[')).toBe('');
    });

    it('não deve detectar links já fechados', () => {
      expect(getWikiLinkMatch('Link fechado [[target]] ')).toBeNull();
    });

    it('deve ignorar colchetes simples', () => {
      expect(getWikiLinkMatch('Lista simples [item]')).toBeNull();
    });
  });
});
