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
    if (!mmInstance) {
      mmInstance = Markmap.create(svgEl, {
        autoFit: true,
      }, root);
    } else {
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
