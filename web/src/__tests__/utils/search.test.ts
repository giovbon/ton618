import { describe, expect, it } from 'vitest';
import { applyHtmlHighlight } from '../../utils/search';

describe('applyHtmlHighlight', () => {
  it('deve aplicar realce em um termo simples', () => {
    const html = '<p>O professor chegou cedo.</p>';
    const query = 'professor';
    const terms = ['professor'];
    const result = applyHtmlHighlight(html, query, terms);

    expect(result).toContain(
      '<mark class="bg-amber-500/40 text-amber-100 font-medium px-0.5 rounded shadow-sm">professor</mark>',
    );
  });

  it('deve priorizar frases completas sobre termos isolados', () => {
    const html = '<p>O laudo mostra lesões graves.</p>';
    const query = 'lesões graves';
    const terms = ['lesões', 'graves'];
    const result = applyHtmlHighlight(html, query, terms);

    // Deve realçar a frase inteira com a cor de 'phrase' (sky)
    expect(result).toContain(
      '<mark class="bg-sky-500/30 text-sky-100 font-bold px-0.5 rounded shadow-sm">lesões graves</mark>',
    );
    // Não deve haver realces duplicados dentro da frase
    expect(result.match(/<mark/g)?.length).toBe(1);
  });

  it('deve ser resiliente a caracteres especiais de Regex na busca', () => {
    const html = '<p>Custo: $100.00 (taxa + bônus).</p>';
    const query = '$100.00';
    const terms = ['$100.00', 'taxa', '+', 'bônus'];

    expect(() => applyHtmlHighlight(html, query, terms)).not.toThrow();
    const result = applyHtmlHighlight(html, query, terms);
    expect(result).toContain('mark');
  });

  it('NÃO deve realçar termos dentro de tags HTML', () => {
    const html = '<a href="/link-com-termo">Texto do link</a>';
    const query = 'link';
    const terms = ['link'];
    const result = applyHtmlHighlight(html, query, terms);

    // O termo no href não deve ser tocado, apenas o do conteúdo (se houvesse)
    expect(result).toContain('href="/link-com-termo"');
    expect(result).not.toContain('href="/<mark');
  });

  it('deve ignorar termos muito curtos ou stopwords conforme lógica global', () => {
    const html = '<p>O rato roeu a roupa.</p>';
    const query = 'o';
    const terms = ['o'];
    const result = applyHtmlHighlight(html, query, terms);
    expect(result).toBe(html); // Termo 'o' tem menos de 2 caracteres
  });
});
