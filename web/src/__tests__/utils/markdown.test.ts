import { describe, it, expect } from 'vitest';
import { marked } from '../../utils/markdown';

describe('Markdown WikiLink Extension', () => {
  it('should render a simple [[WikiLink]] as an anchor tag', () => {
    const md = 'Confira minha [[Nota de Teste]] aqui.';
    const html = marked.parse(md);
    
    expect(html).toContain('<a class="wikilink font-bold text-sky-400');
    expect(html).toContain('data-note="Nota de Teste"');
    expect(html).toContain('>Nota de Teste</a>');
  });

  it('should handle multiple wikilinks in the same line', () => {
    const md = 'Links: [[A]] e [[B]].';
    const html = marked.parse(md);
    
    expect(html).toContain('data-note="A"');
    expect(html).toContain('data-note="B"');
  });

  it('should trim spaces inside brackets', () => {
    const md = '[[  Espaçado  ]]';
    const html = marked.parse(md);
    
    expect(html).toContain('data-note="Espaçado"');
    expect(html).toContain('>Espaçado</a>');
  });

  it('should not break standard links', () => {
    const md = '[Link Normal](https://google.com)';
    const html = marked.parse(md);
    
    expect(html).toContain('href="https://google.com"');
    expect(html).not.toContain('wikilink');
  });
});

describe('Markdown Code Blocks', () => {
  it('should render a regular code block with syntax highlighting', () => {
    const md = '```javascript\nconst x = 1;\n```';
    const html = marked.parse(md);
    
    expect(html).toContain('<pre><code class="hljs language-javascript">');
    expect(html).toContain('hljs-keyword'); // 'const' deve ser realçado
  });

  it('should render a mermaid block inside a specific container', () => {
    const md = '```mermaid\ngraph TD; A-->B;\n```';
    const html = marked.parse(md);
    
    expect(html).toContain('<div class="mermaid-container');
    expect(html).toContain('<pre class="mermaid">');
    // Verificamos apenas partes que sabemos que não sofrem escape ambíguo
    expect(html).toContain('graph TD;');
    expect(html).toContain('A');
    expect(html).toContain('B;');
  });

  it('should fallback to plaintext for unknown languages', () => {
    const md = '```unknownlang\ndata\n```';
    const html = marked.parse(md);
    
    expect(html).toContain('language-plaintext');
  });
});
