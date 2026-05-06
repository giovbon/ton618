import hljs from 'highlight.js/lib/core';
import bash from 'highlight.js/lib/languages/bash';
import css from 'highlight.js/lib/languages/css';
import go from 'highlight.js/lib/languages/go';
import javascript from 'highlight.js/lib/languages/javascript';
import json from 'highlight.js/lib/languages/json';
import markdownLang from 'highlight.js/lib/languages/markdown';
import python from 'highlight.js/lib/languages/python';
import typescript from 'highlight.js/lib/languages/typescript';
import { Marked, type TokenizerAndRendererExtension } from 'marked';

hljs.registerLanguage('javascript', javascript);
hljs.registerLanguage('typescript', typescript);
hljs.registerLanguage('bash', bash);
hljs.registerLanguage('json', json);
hljs.registerLanguage('go', go);
hljs.registerLanguage('python', python);
hljs.registerLanguage('markdown', markdownLang);
hljs.registerLanguage('css', css);

// Extensão para WikiLinks [[link]]
const wikiLinkExtension: TokenizerAndRendererExtension = {
  name: 'wikiLink',
  level: 'inline',
  start(src: string) {
    return src.indexOf('[[');
  },
  tokenizer(src: string) {
    const rule = /^\[\[([^\]]+)\]\]/;
    const match = rule.exec(src);
    if (match) {
      return {
        type: 'wikiLink',
        raw: match[0],
        text: match[1].trim(),
      };
    }
    return undefined;
  },
  renderer(token: any) {
    // Classe 'wikilink' é usada pelo listener global em App.tsx
    return `<a class="wikilink font-bold text-sky-400 hover:text-sky-300 transition-colors cursor-pointer select-none underline decoration-sky-500/30 underline-offset-4" href="#" data-note="${token.text}">${token.text}</a>`;
  },
};

// Instância explícita do Marked (v18+) para garantir consistência
const marked = new Marked({
  gfm: true,
  breaks: true,
});

marked.use({
  extensions: [wikiLinkExtension],
  renderer: {
    code({ text, lang }: { text: string; lang?: string }) {
      if (lang === 'mermaid') {
        return `<div class="mermaid-container my-4 bg-zinc-900/50 p-4 rounded-xl border border-zinc-800/50 flex justify-center"><pre class="mermaid">${text}</pre></div>`;
      }
      const language = lang && hljs.getLanguage(lang) ? lang : null;
      if (language) {
        const highlighted = hljs.highlight(text, { language }).value;
        return `<pre><code class="hljs language-${language}">${highlighted}</code></pre>`;
      }
      return `<pre><code class="hljs language-plaintext">${text}</code></pre>`;
    },
  },
});

export { marked };
