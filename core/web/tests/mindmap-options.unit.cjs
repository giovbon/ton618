const test = require("node:test");
const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");

test("markmap-options: Transformer e deriveOptions extraem maxWidth de frontmatter", async () => {
  const { Transformer } = await import("markmap-lib");
  const markmap = await import("markmap-view");
  const tf = new Transformer();

  const md = `---
type: mindmap
markmap:
  maxWidth: 300
---
# Root Node
- Child 1
- Child 2
`;

  const res = tf.transform(md);
  assert.equal(res.frontmatter?.markmap?.maxWidth, 300);
  const options = markmap.deriveOptions(res.frontmatter?.markmap);
  assert.equal(options.maxWidth, 300);
  assert.equal(res.root.content, "Root Node");
});

test("markmap-options: normalização tolerante para markmap: sem delimitadores ---", async () => {
  const { Transformer } = await import("markmap-lib");
  const markmap = await import("markmap-view");
  const tf = new Transformer();

  let input = `markmap:
  maxWidth: 250

# Root Node
- Item
`;

  if (!input.trim().startsWith("---") && /^\s*markmap:\s*\n/.test(input)) {
    input = "---\n" + input.replace(/^\s*(markmap:\s*\n(?:[ \t]+[^\n]*\n?)*)/, "$1---\n");
  }

  const res = tf.transform(input);
  assert.equal(res.frontmatter?.markmap?.maxWidth, 250);
  const options = markmap.deriveOptions(res.frontmatter?.markmap);
  assert.equal(options.maxWidth, 250);
});

test("mindmap.js e build.js: src/mindmap.js utiliza deriveOptions e está nos entryPoints do esbuild", () => {
  const mindmapSrc = fs.readFileSync(path.join(__dirname, "../src/mindmap.js"), "utf8");
  assert.ok(mindmapSrc.includes("deriveOptions"), "mindmap.js deve importar e usar deriveOptions");
  assert.ok(mindmapSrc.includes("markmapOptions"), "mindmap.js deve passar markmapOptions na criação e atualização");
  assert.ok(!mindmapSrc.includes("compileBody = fmMatch[2]"), "mindmap.js não deve descartar o frontmatter");

  const buildSrc = fs.readFileSync(path.join(__dirname, "../build.js"), "utf8");
  assert.ok(buildSrc.includes('"src/mindmap.js"'), "build.js deve conter src/mindmap.js nos entryPoints");
});
