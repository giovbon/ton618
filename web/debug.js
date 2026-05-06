const fs = require('fs');
const { SourceMapConsumer } = require('source-map');

async function debug() {
  const mapPath = fs.readdirSync('dist/assets').find(f => f.startsWith('MarkdownEditor') && f.endsWith('.js.map'));
  const rawSourceMap = fs.readFileSync('dist/assets/' + mapPath, 'utf8');
  
  await SourceMapConsumer.with(rawSourceMap, null, consumer => {
    // We need the line and column. In the user trace it was 43:2063
    // but the sourcemap we generated might have different lines/cols.
    // Let's just find the first ReferenceError or find what 'ge' means in the sourcemap.
    // Instead of line/col, let's search for "class WikiLinkPillsPlugin" or similar.
    console.log("Source map loaded.");
  });
}
debug().catch(console.error);
