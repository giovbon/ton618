import { marked } from '../utils/markdown';
import { applyHtmlHighlight } from '../utils/search';

// Listener de Mensagens
self.onmessage = async (e: MessageEvent) => {
  const { id, text, query, terms } = e.data;

  try {
    const mdHtml = await marked.parse(text);
    const finalHtml = applyHtmlHighlight(mdHtml, query, terms);

    self.postMessage({ id, html: finalHtml });
  } catch (err) {
    self.postMessage({ id, error: String(err) });
  }
};
