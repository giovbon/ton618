const fs = require('fs');
const { SourceMapConsumer } = require('source-map');

async function debug() {
  const mapPath = fs.readdirSync('dist/assets').find(f => f.startsWith('MarkdownEditor') && f.endsWith('.js.map'));
  const rawSourceMap = fs.readFileSync('dist/assets/' + mapPath, 'utf8');
  
  await SourceMapConsumer.with(rawSourceMap, null, consumer => {
    // The error was: at T.kC [as constructor] (MarkdownEditor-DKHK7per.js:43:2063)
    // 43 is line, 2063 is column. Let's find out what that is.
    const pos = consumer.originalPositionFor({
      line: 43,
      column: 2063
    });
    console.log("Original position:", pos);
  });
}
debug().catch(console.error);
