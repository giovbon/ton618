// @ts-check
import { Transformer } from "markmap-lib";
import * as markmap from "markmap-view";
const { Markmap, loadCSS, loadJS } = markmap;

// Expose markmap globally for plugins
window.markmap = markmap;

const transformer = new Transformer();

/**
 * Escapes HTML entities in a string to prevent raw HTML rendering.
 * Used as fallback when highlight.js is not yet loaded.
 * @param {string} str
 * @returns {string}
 */
function escapeHtml(str) {
  return str
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&#039;");
}

// Fix: override highlight function to ensure HTML code inside fenced blocks
// is always escaped, even when highlight.js hasn't loaded from CDN yet.
// The markmap-lib browser hljs plugin returns raw `str` when window.hljs
// is undefined, causing HTML tags to render as DOM inside foreignObject.
const origHighlight = transformer.md.options.highlight;
transformer.md.set({
  highlight: (str, language) => {
    const { hljs } = window;
    if (hljs) {
      // hljs available — use it (properly escapes HTML internally)
      return hljs.highlightAuto(str, language ? [language] : void 0).value;
    }
    // hljs not loaded yet — at least escape HTML to prevent rendering
    return escapeHtml(str);
  }
});

/**
 * @typedef {Object} MindmapInstance
 * @property {Function} update - Updates the mindmap with new markdown content.
 * @property {Function} fit - Recalculates zoom and fits the mindmap in the SVG container.
 */

/**
 * Initializes the Markmap mindmap inside the given SVG element.
 * Exposed globally as initMindmap.
 * 
 * @param {SVGElement} svgEl - The SVG element where markmap will render.
 * @param {string} initialMarkdown - The initial Markdown source to generate the mindmap.
 * @returns {MindmapInstance}
 */
window.initMindmap = function (svgEl, initialMarkdown) {
  /** @type {any} */
  let mmInstance = null;
  let hljsCssText = null;
  let lastCompileBody = '';

  /**
   * Loads the local highlight.js CSS once and caches it.
   * Injects it as a <style> inside the SVG <defs> so it works inside foreignObject.
   */
  async function ensureHljsStyleInSvg() {
    // Load CSS text once
    if (hljsCssText === null) {
      try {
        const resp = await fetch("/static/hljs-github-dark.min.css");
        hljsCssText = await resp.text();
      } catch (e) {
        console.warn("[Markmap] Não foi possível carregar hljs CSS:", e);
        hljsCssText = "";
      }
    }
    if (!hljsCssText) return;

    // Inject or update a <style> inside SVG <defs>
    let defs = svgEl.querySelector("defs");
    if (!defs) {
      defs = document.createElementNS("http://www.w3.org/2000/svg", "defs");
      svgEl.prepend(defs);
    }
    let styleEl = defs.querySelector("style[data-hljs]");
    if (!styleEl) {
      styleEl = document.createElementNS("http://www.w3.org/2000/svg", "style");
      styleEl.setAttribute("data-hljs", "1");
      defs.appendChild(styleEl);
    }
    styleEl.textContent = hljsCssText;
  }

  function getFilename() {
    const filenameInput = /** @type {HTMLInputElement|null} */ (document.getElementById("file-name"));
    const val = filenameInput ? (filenameInput.getAttribute("data-filename") || filenameInput.dataset.filename || filenameInput.value) : "default";
    console.log("[Markmap] getFilename:", val);
    return val;
  }

  function saveFoldState() {
    console.log("[Markmap] saveFoldState triggered");
    if (!mmInstance || !mmInstance.state || !mmInstance.state.data) {
      console.warn("[Markmap] saveFoldState: mmInstance not fully ready");
      return;
    }
    const filename = getFilename();
    const foldedPaths = [];

    function traverse(node, currentPath) {
      const cleanContent = node.content ? node.content.replace(/<[^>]*>/g, '').trim() : "";
      const path = currentPath ? `${currentPath} > ${cleanContent}` : cleanContent;
      
      if (node.payload && node.payload.fold) {
        foldedPaths.push(path);
      }
      
      if (node.children) {
        for (const child of node.children) {
          traverse(child, path);
        }
      }
    }

    traverse(mmInstance.state.data, "");
    console.log("[Markmap] saveFoldState: folded paths found:", foldedPaths);
    
    try {
      localStorage.setItem(`markmap_folds:${filename}`, JSON.stringify(foldedPaths));
      console.log("[Markmap] saveFoldState: saved to localStorage for", filename);
    } catch (e) {
      console.error("Error saving fold state to localStorage", e);
    }
  }

  function applyFoldState(rootNode) {
    const filename = getFilename();
    let foldedPaths = [];
    try {
      const stored = localStorage.getItem(`markmap_folds:${filename}`);
      console.log("[Markmap] applyFoldState: loaded from localStorage:", stored);
      if (stored) {
        foldedPaths = JSON.parse(stored);
      }
    } catch (e) {
      console.error("Error loading fold state from localStorage", e);
    }

    if (!foldedPaths || foldedPaths.length === 0) return;

    const foldedSet = new Set(foldedPaths);

    function traverse(node, currentPath) {
      const cleanContent = node.content ? node.content.replace(/<[^>]*>/g, '').trim() : "";
      const path = currentPath ? `${currentPath} > ${cleanContent}` : cleanContent;

      if (foldedSet.has(path)) {
        console.log("[Markmap] applyFoldState: folding node matching path:", path);
        node.payload = node.payload || {};
        node.payload.fold = 1;
      }

      if (node.children) {
        for (const child of node.children) {
          traverse(child, path);
        }
      }
    }

    traverse(rootNode, "");
  }

  /**
   * Compiles and updates the markmap structure.
   * 
   * @param {string} markdown 
   */
  async function update(markdown) {
    try {
      let compileBody = markdown;
      const FRONTMATTER_REGEX = /^---\r?\n([\s\S]*?)\r?\n---\r?\n?([\s\S]*)$/;
      const fmMatch = markdown.match(FRONTMATTER_REGEX);
      if (fmMatch) {
        compileBody = fmMatch[2];
      }

      // Track for retransform (hljs loading)
      lastCompileBody = compileBody;

      const { root, features } = transformer.transform(compileBody);
      
      // Load assets dynamically for features (like Prism for syntax highlighting or KaTeX for math)
      const { styles, scripts } = transformer.getUsedAssets(features);
      if (styles) loadCSS(styles);
      if (scripts) await loadJS(scripts, { getMarkmap: () => markmap });

      console.log("[Markmap] update: transformer returned root tree. Applying fold state.");
      applyFoldState(root);

      if (!mmInstance) {
        console.log("[Markmap] Creating mmInstance");
        mmInstance = Markmap.create(svgEl, {
          autoFit: true,
        }, root);

        // Intercept fold changes
        const originalToggleNode = mmInstance.toggleNode;
        mmInstance.toggleNode = async function(...args) {
          console.log("[Markmap] toggleNode intercepted");
          const res = await originalToggleNode.apply(this, args);
          saveFoldState();
          return res;
        };
      } else {
        console.log("[Markmap] Updating data in existing mmInstance");
        mmInstance.setData(root);
        mmInstance.fit();
      }

      // Inject hljs CSS into SVG after render (works inside foreignObject)
      if (features && features.hljs) {
        await ensureHljsStyleInSvg();
      }
    } catch (e) {
      console.error("[Markmap] Erro ao renderizar / atualizar mapa mental:", e);
    }
  }

  // Subscribe to retransform hook (triggered after hljs loads from CDN)
  // so we can re-render the mindmap with proper syntax highlighting.
  transformer.hooks.retransform.tap(() => {
    if (mmInstance && lastCompileBody) {
      console.log("[Markmap] retransform hook fired — re-rendering with hljs");
      try {
        const { root, features } = transformer.transform(lastCompileBody);
        applyFoldState(root);
        mmInstance.setData(root);
        mmInstance.fit();
        if (features && features.hljs) {
          ensureHljsStyleInSvg();
        }
      } catch (e) {
        console.error("[Markmap] Erro no retransform:", e);
      }
    }
  });

  // Initial render
  update(initialMarkdown);

  return /** @type {MindmapInstance} */ ({
    update: update,
    fit: () => {
      if (mmInstance) {
        mmInstance.fit();
      }
    }
  });
};
