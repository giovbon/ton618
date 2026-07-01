// @ts-check
import { Transformer } from "markmap-lib";
import { Markmap } from "markmap-view";

const transformer = new Transformer();

/**
 * @typedef {Object} MindmapInstance
 * @property {function(string): void} update - Updates the mindmap with new markdown content.
 * @property {function(): void} fit - Recalculates zoom and fits the mindmap in the SVG container.
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

  function getFilename() {
    const filenameInput = document.getElementById("file-name");
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
  function update(markdown) {
    let compileBody = markdown;
    const FRONTMATTER_REGEX = /^---\r?\n([\s\S]*?)\r?\n---\r?\n?([\s\S]*)$/;
    const fmMatch = markdown.match(FRONTMATTER_REGEX);
    if (fmMatch) {
      compileBody = fmMatch[2];
    }

    const { root } = transformer.transform(compileBody);
    
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
  }

  // Initial render
  update(initialMarkdown);

  return {
    update: update,
    fit: () => {
      if (mmInstance) {
        mmInstance.fit();
      }
    }
  };
};
