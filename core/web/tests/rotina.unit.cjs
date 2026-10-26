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

test("rotina-timeline: parseClock valida e converte HH:MM", async () => {
  const { parseClock } = await import("../src/rotina-timeline.js");
  assert.equal(parseClock("00:00"), 0);
  assert.equal(parseClock("09:30"), 570);
  assert.equal(parseClock("23:59"), 1439);
  assert.equal(parseClock("24:00"), null);
  assert.equal(parseClock("9h"), null);
  assert.equal(parseClock(""), null);
  assert.equal(parseClock(null), null);
});

test("rotina-timeline: formatDuration e formatClock", async () => {
  const { formatDuration, formatClock } = await import("../src/rotina-timeline.js");
  assert.equal(formatDuration(45), "45min");
  assert.equal(formatDuration(60), "1h");
  assert.equal(formatDuration(90), "1h 30min");
  assert.equal(formatClock(570), "09:30");
  assert.equal(formatClock(1440), "24:00");
});

test("rotina-timeline: layoutBars posiciona por tempo e separa sobreposições", async () => {
  const { layoutBars } = await import("../src/rotina-timeline.js");
  const activities = [
    { id: "a", start_time: "08:00", end_time: "09:00", description: "Academia" },
    { id: "b", start_time: "08:30", end_time: "09:30", description: "Reunião" },
    { id: "c", start_time: "10:00", end_time: "11:00", description: "Mercado" },
  ];

  // 1440px de largura = 1px por minuto.
  const layout = layoutBars(activities, 1440);
  assert.equal(layout.bars.length, 3);
  assert.equal(layout.lanes, 2, "A e B se sobrepõem; C volta para a raia 0");

  const [a, b, c] = layout.bars;
  assert.equal(a.x, 480);
  assert.equal(a.width, 60);
  assert.equal(a.lane, 0);
  assert.equal(b.lane, 1);
  assert.equal(b.x, 510);
  assert.equal(c.lane, 0);
  assert.equal(a.overlapping, true);
  assert.equal(b.overlapping, true);
  assert.equal(c.overlapping, false);
  assert.equal(layout.height, 90, "10 topo + 2*26 + 4 gap + 24 eixo");
});

test("rotina-timeline: atividades encostadas compartilham a raia", async () => {
  const { layoutBars } = await import("../src/rotina-timeline.js");
  const layout = layoutBars(
    [
      { id: "a", start_time: "08:00", end_time: "09:00", description: "A" },
      { id: "b", start_time: "09:00", end_time: "10:00", description: "B" },
    ],
    1440
  );
  assert.equal(layout.lanes, 1);
  assert.equal(layout.bars[1].lane, 0);
  assert.equal(layout.bars[1].overlapping, false);
});

test("rotina-timeline: ignora horários inválidos e largura mínima", async () => {
  const { layoutBars } = await import("../src/rotina-timeline.js");
  const layout = layoutBars(
    [
      { id: "a", start_time: "10:00", end_time: "09:00", description: "Inválida" },
      { id: "b", start_time: "12:00", end_time: "12:01", description: "Curta" },
    ],
    1440
  );
  assert.equal(layout.bars.length, 1);
  assert.equal(layout.bars[0].width, 4, "barra curta tem largura mínima clicável");
});

test("rotina.js: formulário, fragmento e os dois renderers estão ligados", () => {
  const js = fs.readFileSync(path.join(__dirname, "../src/rotina.js"), "utf8");
  for (const needle of [
    "renderTimeline",
    "rotina-mermaid",
    "mountNativeTimeline",
    "hasNativeTimeline",
    "openNativeZoom",
    "replaceSchedule",
    "rotina-form",
    "rotina-form-error",
    "mermaid.run",
    "rotina-zoom-modal",
    "window.rotinaZoom",
    "data-mermaid-src",
    "tightenSource",
  ]) {
    assert.ok(js.includes(needle), `rotina.js deve conter ${needle}`);
  }

  const build = fs.readFileSync(path.join(__dirname, "../build.js"), "utf8");
  assert.ok(build.includes('"src/rotina.js"'), "rotina.js deve estar nos entryPoints do esbuild");
});

test("rotina-timeline.js: renderer nativo sem dependência de DOM no import", async () => {
  const mod = await import("../src/rotina-timeline.js");
  for (const name of ["mount", "unmount", "unmountAll", "unmountWithin", "openZoom", "hasNativeTimeline"]) {
    assert.equal(typeof mod[name], "function", `export ${name} ausente`);
  }
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
  assert.ok(templ.includes('id="rotina-timeline"'), "renderer nativo presente");
  assert.ok(templ.includes("data-rotina-viewport"), "viewport da timeline nativa");
  assert.ok(templ.includes("data-rotina-canvas"), "canvas da timeline nativa");
  assert.ok(templ.includes('templ.JSONScript("rotina-timeline-data"'), "atividades em JSON para o renderer");
  assert.ok(templ.includes("data-rotina-zoom-in"), "controle de zoom nativo");
  assert.ok(templ.includes('id="rotina-zoom-btn"'), "botão Ampliar presente");
  assert.ok(templ.includes('onclick="window.rotinaZoom()"'), "Ampliar chama a função exposta pelo JS");
  assert.ok(templ.includes('role="dialog"'), "modal do Ampliar com semântica de diálogo");
  assert.ok(templ.includes("/static/js/vendor/mermaid.min.js"), "Mermaid vendorizado (modo de comparação)");
  assert.ok(
    templ.includes("if !data.Schedule.TimelineNative {"),
    "Mermaid só deve ser carregado no modo legado"
  );
  assert.ok(templ.includes("/static/rotina.js"), "bundle da Rotina carregado");

  const native = fs.readFileSync(path.join(__dirname, "../src/rotina-timeline.js"), "utf8");
  assert.ok(native.includes("layoutBars"), "geometria pura exportada");
  assert.ok(native.includes("data-rotina-canvas"), "renderer desenha no canvas do template");

  const navbar = fs.readFileSync(path.join(root, "web", "layout", "navbar.templ"), "utf8");
  assert.ok(navbar.includes('id="nav-rotina"'), "ícone desktop da Rotina");
  assert.ok(navbar.includes('id="mobile-nav-rotina"'), "link mobile da Rotina");
  assert.ok(navbar.includes("data-nav-rotina"), "atributo para mostrar/esconder ao vivo");
  assert.ok(navbar.indexOf('id="nav-rotina"') > navbar.indexOf('id="nav-agenda"'), "Rotina ao lado da Agenda");

  const settings = fs.readFileSync(path.join(root, "web", "layout", "settings_modal.templ"), "utf8");
  assert.ok(settings.includes("openTab('rotina')"), "aba Rotina nas configurações");
  assert.ok(settings.includes("/api/settings/rotina"), "toggle persiste a flag");
  assert.ok(settings.includes("applyRotinaNav"), "toggle esconde/mostra o ícone ao vivo");
  assert.ok(settings.includes("data-rotina-native"), "estado do renderer vem do servidor");
  assert.ok(settings.includes("saveNative"), "toggle do renderer persiste a escolha");
});
