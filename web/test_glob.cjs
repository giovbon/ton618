const fg = require("fast-glob");
const entries = fg.sync([
    "../internal/features/**/*.templ",
    "../web/layout/**/*.templ",
    "./layout/**/*.templ",
]);
console.log("Found " + entries.length + " files");
console.log(entries);
