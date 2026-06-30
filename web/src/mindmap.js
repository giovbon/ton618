import { Transformer } from "markmap-lib";
import { Markmap } from "markmap-view";

const transformer = new Transformer();

window.initMindmap = function (svgEl, initialMarkdown) {
  let mmInstance = null;

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
