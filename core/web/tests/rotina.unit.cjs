const test = require("node:test");
const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");

const SRC = `gantt
    dateFormat YYYY-MM-DD HH:mm
    section Rotina
        Academia :c0ea5e9-1, 2026-10-25 09:00, 2026-10-25 10:00
        Reuniao :c0ea5e9-2, 2026-10-25 14:00, 2026-10-25 15:00
    edge :edge0, 2026-10-25 00:00, 2026-10-25 23:59
`;

test("rotina-utils: compacta o eixo para o intervalo real com folga de 2h", async () => {
  const { tightenSource } = await import("../src/rotina-utils.js");
  const out = tightenSource(SRC);
  assert.ok(out.includes("edge :edge0, 2026-10-25 07:00, 2026-10-25 17:00"), out);
  assert.ok(out.includes("Academia :c0ea5e9-1"), "as atividades devem ser preservadas");
});

test("rotina-utils: garante intervalo mínimo de 8h", async () => {
  const { tightenSource } = await import("../src/rotina-utils.js");
  const src = `gantt
    section Rotina
        Curta :c0ea5e9-1, 2026-10-25 12:00, 2026-10-25 12:30
    edge :edge0, 2026-10-25 00:00, 2026-10-25 23:59
`;
  const out = tightenSource(src);
  assert.ok(out.includes("edge :edge0, 2026-10-25 08:00, 2026-10-25 16:00"), out);
});

test("rotina-utils: fonte sem atividades (ou inválida) volta intacta", async () => {
  const { tightenSource } = await import("../src/rotina-utils.js");
  const empty = "gantt\n    dateFormat YYYY-MM-DD HH:mm\n";
  assert.equal(tightenSource(empty), empty);
  assert.equal(tightenSource(""), "");
  assert.equal(tightenSource(null), null);
});

test("rotina.js: formulário, fragmento e Mermaid estão ligados", () => {
  const js = fs.readFileSync(path.join(__dirname, "../src/rotina.js"), "utf8");
  for (const needle of [
    "renderTimeline",
    "rotina-mermaid",
    "replaceSchedule",
    "rotina-form",
    "rotina-form-error",
    "mermaid.run",
    "rotina-zoom-modal",
    "tightenSource",
  ]) {
    assert.ok(js.includes(needle), `rotina.js deve conter ${needle}`);
  }

  const build = fs.readFileSync(path.join(__dirname, "../build.js"), "utf8");
  assert.ok(build.includes('"src/rotina.js"'), "rotina.js deve estar nos entryPoints do esbuild");
});

test("rotina.js: identifica o formulário por getAttribute (name=id sombreia form.id)", () => {
  const js = fs.readFileSync(path.join(__dirname, "../src/rotina.js"), "utf8");
  assert.ok(
    js.includes('form.getAttribute("id") !== "rotina-form"'),
    "use getAttribute: o campo name=\"id\" sombreia a propriedade form.id"
  );
  assert.ok(
    !/form\.id\s*!==\s*"rotina-form"/.test(js),
    "form.id devolve o <input name=\"id\"> (LegacyOverrideBuiltIns) e quebra o preventDefault"
  );
});

test("rotina.templ e settings: integração com navbar e configurações", () => {
  const root = path.join(__dirname, "..", "..");
  const templ = fs.readFileSync(
    path.join(root, "internal", "features", "dayplan", "rotina.templ"),
    "utf8"
  );
  assert.ok(templ.includes('id="rotina-schedule"'), "fragmento com id estável");
  assert.ok(templ.includes('id="rotina-form"'), "formulário presente");
  assert.ok(templ.includes("/static/js/vendor/mermaid.min.js"), "Mermaid vendorizado");
  assert.ok(templ.includes("/static/rotina.js"), "bundle da Rotina carregado");

  const navbar = fs.readFileSync(path.join(root, "web", "layout", "navbar.templ"), "utf8");
  assert.ok(navbar.includes('id="nav-rotina"'), "ícone desktop da Rotina");
  assert.ok(navbar.includes('id="mobile-nav-rotina"'), "link mobile da Rotina");
  assert.ok(navbar.includes("data-nav-rotina"), "atributo para mostrar/esconder ao vivo");
  assert.ok(navbar.indexOf('id="nav-rotina"') > navbar.indexOf('id="nav-agenda"'), "Rotina ao lado da Agenda");

  const settings = fs.readFileSync(path.join(root, "web", "layout", "settings_modal.templ"), "utf8");
  assert.ok(settings.includes("openTab('rotina')"), "aba Rotina nas configurações");
  assert.ok(settings.includes("/api/settings/rotina"), "toggle persiste a flag");
  assert.ok(settings.includes("applyRotinaNav"), "toggle esconde/mostra o ícone ao vivo");
});
