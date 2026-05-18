import { describe, it, expect } from 'vitest';
import { isStopword, extractFragment, applyHtmlHighlight, processSearchResults } from '../../utils/search';

describe('Search Utilities', () => {
  
  describe('isStopword', () => {
    it('should identify common stopwords', () => {
      expect(isStopword('de')).toBe(true);
      expect(isStopword('para')).toBe(true);
      expect(isStopword('computador')).toBe(false);
    });

    it('should identify short words as stopwords', () => {
      expect(isStopword('abc')).toBe(true);
      expect(isStopword('note')).toBe(false); // Exactly 4 chars, not a stopword
    });
  });

  describe('extractFragment', () => {
    it('should find a fragment around a full query', () => {
      const text = "Este é um texto muito longo que contém a frase secreta para o teste de integridade.";
      const query = "frase secreta";
      const { fragment, isTruncated } = extractFragment(text, query, ["frase", "secreta"]);
      expect(fragment).toContain("frase secreta");
      expect(fragment.length).toBeLessThanOrEqual(text.length); 
      expect(isTruncated).toBe(false);
    });

    it('should fallback to first strong term if full query missing', () => {
      const text = "O motor de busca do seeker é muito potente.";
      const query = "motor potente";
      // Simula que os termos estão separados no texto
      const { fragment } = extractFragment(text, query, ["motor", "potente"]);
      expect(fragment).toContain("motor");
    });

    it('should truncate long text even if no query match found', () => {
      const text = "A".repeat(1500);
      const query = "inexistente";
      const { fragment, isTruncated, hasMoreAfter } = extractFragment(text, query, ["inexistente"]);
      expect(fragment.length).toBeLessThan(900);
      expect(fragment).not.toContain("...");
      expect(isTruncated).toBe(true);
      expect(hasMoreAfter).toBe(true);
    });

    it('should truncate long text even if query is empty', () => {
      const text = "B".repeat(1500);
      const query = "";
      const { fragment, isTruncated, hasMoreAfter } = extractFragment(text, query, []);
      expect(fragment.length).toBeLessThan(900);
      expect(isTruncated).toBe(true);
      expect(hasMoreAfter).toBe(true);
    });
  });

  describe('applyHtmlHighlight', () => {
    it('should highlight full phrases in sky-blue (Unicode safe)', () => {
      const html = "<p>A situação de erro é crítica.</p>";
      const query = "situação de erro";
      const terms = ["situação", "erro"];
      
      const result = applyHtmlHighlight(html, query, terms);
      expect(result).toContain('bg-sky-500/30');
      expect(result).toContain('situação de erro');
    });

    it('should highlight individual terms in amber', () => {
      const html = "<p>O motor de busca é rápido.</p>";
      const query = "motor rápido";
      const terms = ["motor", "rápido"];
      
      const result = applyHtmlHighlight(html, query, terms);
      expect(result).toContain('bg-amber-500/40');
      expect(result).toContain('motor');
      expect(result).toContain('rápido');
    });

    it('should respect word boundaries at the start', () => {
      const html = "<p>O berro foi alto.</p>";
      const query = "erro";
      const terms = ["erro"];
      
      const result = applyHtmlHighlight(html, query, terms);
      expect(result).not.toContain('mark');
      expect(result).toContain('berro');
    });

    it('should allow partial matches at the end for technical terms', () => {
      const html = "<p>The error occurred.</p>";
      const query = "erro";
      const terms = ["erro"];
      
      const result = applyHtmlHighlight(html, query, terms);
      expect(result).toContain('mark');
      // A palavra 'error' agora está dividida pela tag: erro</mark>r
      expect(result).toContain('erro');
      expect(result).toContain('mark');
    });

    it('should NOT highlight inside HTML tags', () => {
      const html = '<img src="search.png" alt="search icon" />';
      const query = "search";
      const terms = ["search"];
      
      const result = applyHtmlHighlight(html, query, terms);
      expect(result).toBe(html); // No change inside tags
    });

    it('should highlight single-word query (regression test)', () => {
      const html = "<p>O erro foi detectado.</p>";
      const query = "erro";
      const terms = ["erro"];
      
      const result = applyHtmlHighlight(html, query, terms);
      expect(result).toContain('<mark class="bg-amber-500/40 text-amber-100 font-medium px-0.5 rounded shadow-sm">erro</mark>');
    });

    it('should handle unicode characters in word boundaries (À-ÿ)', () => {
      const html = "<p>A situação é crítica.</p>";
      const query = "situação";
      const terms = ["situação"];
      
      const result = applyHtmlHighlight(html, query, terms);
      expect(result).toContain('<mark');
      expect(result).toContain('situação');
    });
  });

  describe('processSearchResults', () => {
    const mockHits = [
      {
        _source: { arquivo: 'nota1.md', secao: 'Geral' },
        highlight: { texto: ['match1'] },
        final_score: 1.5
      },
      {
        _source: { arquivo: 'nota1.md', secao: 'Geral' }, // Fragmento duplicado do mesmo arquivo
        highlight: { texto: ['match2'] },
        final_score: 1.2
      },
      {
        _source: { arquivo: 'nota2.md', secao: 'Intro' },
        highlight: { texto: ['match3'] },
        final_score: 2.0
      }
    ];

    it('should map search result fields correctly', () => {
      const results = processSearchResults(mockHits, false);
      expect(results[0].arquivo).toBe('nota1.md');
      expect(results[0].highlight.texto[0]).toBe('match1');
      expect(results[0].final_score).toBe(1.5);
    });

    it('should deduplicate results by "arquivo" in Compact Mode', () => {
      const results = processSearchResults(mockHits, true);
      expect(results.length).toBe(2);
      expect(results[0].arquivo).toBe('nota1.md');
      expect(results[1].arquivo).toBe('nota2.md');
    });

    it('should NOT deduplicate results in Normal Mode', () => {
      const results = processSearchResults(mockHits, false);
      expect(results.length).toBe(3);
    });
  });
});
