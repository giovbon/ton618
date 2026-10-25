/**
 * rotina.js — linha do tempo da Rotina (Mermaid gantt).
 *
 * O servidor entrega o bloco #rotina-schedule pronto (resumo + lista + form).
 * Toda mutação devolve o bloco atualizado, que substitui o anterior; depois
 * disso o Mermaid é renderizado de novo. Nada de estado no cliente além do
 * formulário de edição.
 */

import { tightenSource } from "./rotina-utils.js";

(function () {
  "use strict";

  var initialized = false;

  function isNarrow() {
    return !!(window.matchMedia && window.matchMedia("(max-width: 820px)").matches);
  }

  function initMermaid() {
    if (!window.mermaid) return false;
    if (initialized) return true;
    // Tema "base" com variáveis zinc/sky para casar com o resto do TON-618.
    window.mermaid.initialize({
      startOnLoad: false,
      theme: "base",
      securityLevel: "strict",
      fontFamily: "system-ui, -apple-system, 'Segoe UI', Roboto, sans-serif",
      themeVariables: {
        darkMode: true,
        background: "transparent",
        fontSize: "12px",
        primaryColor: "#0ea5e9",
        primaryTextColor: "#f8fafc",
        primaryBorderColor: "#0284c7",
        secondaryColor: "#27272a",
        tertiaryColor: "#18181b",
        lineColor: "#3f3f46",
        textColor: "#d4d4d8",
        taskTextColor: "#f8fafc",
        taskTextOutsideColor: "#cbd5e1",
        taskTextLightColor: "#18181b",
        taskTextDarkColor: "#f8fafc",
        sectionBkgColor: "transparent",
        altSectionBkgColor: "transparent",
        sectionBkgColor2: "transparent",
        gridColor: "#3f3f46",
        todayLineColor: "#0ea5e9"
      },
      gantt: {
        barGap: 6,
        barHeight: 22,
        topPadding: 34,
        leftPadding: 58,
        useMaxWidth: false,
        fontFamily: "system-ui, -apple-system, 'Segoe UI', Roboto, sans-serif"
      }
    });
    initialized = true;
    return true;
  }

  // ── Pós-processamento do SVG ──────────────────────────────────────────────
  //  1. Esconde a barra-fantasma que força o eixo do dia inteiro;
  //  2. Pinta as barras de sky-500 e ajusta o contraste do texto;
  //  3. Redesenha as horas ("0h, 1h, …") na parte de baixo do gráfico.

  /** @param {Element|null} scope */
  function postProcess(scope) {
    /** @type {SVGElement|null} */
    var svg = scope
      ? /** @type {SVGElement} */ (scope.tagName === "svg" ? scope : scope.querySelector("svg"))
      : null;
    if (!svg) return;

    svg.style.backgroundColor = "transparent";
    svg.style.maxWidth = "none";
    svg.style.width = "100%";
    svg.style.height = "auto";

    // 1. Barra-fantasma.
    var edge = /** @type {SVGElement|null} */ (svg.querySelector('rect.task[id^="edge"]'));
    if (edge) {
      var ey = edge.getAttribute("y");
      edge.style.display = "none";
      svg.querySelectorAll("rect.section").forEach(function (s) {
        if (s.getAttribute("y") === ey) /** @type {SVGElement} */ (s).style.display = "none";
      });
    }
    svg.querySelectorAll("text").forEach(function (t) {
      if ((t.textContent || "").trim() === "edge") /** @type {SVGElement} */ (t).style.display = "none";
    });

    // 2. Barras (id c{hex}-{n}) na cor única da Rotina.
    svg.querySelectorAll("rect.task").forEach(function (node) {
      var r = /** @type {SVGElement} */ (node);
      var id = r.getAttribute("id") || "";
      var m = id.match(/^c([0-9a-fA-F]{6})/);
      if (!m) return;
      r.style.fill = "#" + m[1].toLowerCase();
      r.style.stroke = "#0284c7";
      r.style.strokeWidth = "1";

      var txt = /** @type {SVGElement|null} */ (svg.querySelector('text[id="' + id + '-text"]'));
      if (txt) {
        var cls = txt.getAttribute("class") || "";
        txt.style.fill = cls.indexOf("taskTextOutside") > -1 ? "#cbd5e1" : "#f8fafc";
        txt.style.fontWeight = "600";
      }
    });

    // Nomes das seções e textos padrão em zinc.
    svg.querySelectorAll("text").forEach(function (t) {
      var cls = t.getAttribute("class") || "";
      if (cls.indexOf("sectionTitle") > -1) /** @type {SVGElement} */ (t).style.fill = "#71717a";
    });

    relabelAxis(svg);

    // Fallback: qualquer rótulo de hora que tenha sobrado no topo desce.
    var vb = (svg.getAttribute("viewBox") || "").split(/\s+/);
    var height = parseFloat(vb[3]) || svg.getBoundingClientRect().height;
    svg.querySelectorAll("text").forEach(function (t) {
      var txt = (t.textContent || "").trim();
      if (/^\d{1,2}(h|:\d{2})?$/.test(txt) && parseFloat(t.getAttribute("y") || "0") < 20) {
        t.setAttribute("y", String(height - 8));
      }
    });
  }

  // Reconstrói o eixo com as horas na parte de baixo, usando a geometria
  // dos ticks do Mermaid (tempo → pixel).
  /** @param {SVGElement} svg */
  function relabelAxis(svg) {
    var gridG = svg.querySelector("g.grid");
    if (!gridG) return;
    var gx = 0;
    var gm = (gridG.getAttribute("transform") || "").match(/translate\((-?[\d.]+)/);
    if (gm) gx = parseFloat(gm[1]);

    var ticks = [];
    svg.querySelectorAll("g.tick").forEach(function (g) {
      var txt = (g.textContent || "").trim();
      var mins = null;
      var m = txt.match(/^(\d{1,2}):(\d{2})$/);
      if (m) {
        mins = parseInt(m[1], 10) * 60 + parseInt(m[2], 10);
      } else {
        m = txt.match(/^(\d{1,2})h?$/);
        if (m) mins = parseInt(m[1], 10) * 60;
      }
      if (mins === null) return;
      var tm = (g.getAttribute("transform") || "").match(/translate\((-?[\d.]+)/);
      ticks.push({ mins: mins, x: gx + (tm ? parseFloat(tm[1]) : 0) });
    });
    if (ticks.length < 2) return;

    ticks.sort(function (a, b) { return a.mins - b.mins; });
    var span = ticks[ticks.length - 1].mins - ticks[0].mins;
    if (span <= 0) return;
    var pxPerMin = (ticks[ticks.length - 1].x - ticks[0].x) / span;
    var leftX = ticks[0].x - ticks[0].mins * pxPerMin;

    svg.querySelectorAll("g.tick text").forEach(function (t) { t.remove(); });

    var vb = (svg.getAttribute("viewBox") || "").split(/\s+/);
    var viewW = parseFloat(vb[2]) || 640;
    var height = parseFloat(vb[3]) || svg.getBoundingClientRect().height;
    var clientW = svg.getBoundingClientRect().width || viewW;
    var scale = clientW > 0 ? clientW / viewW : 1;
    var pxPerHour = 60 * pxPerMin * scale;
    var step = pxPerHour > 0 ? Math.max(1, Math.ceil(38 / pxPerHour)) : 1;
    var fontSize = Math.max(8, Math.round(8 / scale));

    var NS = "http://www.w3.org/2000/svg";
    // Só rotula horas DENTRO do domínio desenhado (no mobile o eixo é
    // compactado, então 0h/23h ficariam fora da área útil) e ancora o passo em
    // múltiplos redondos de hora.
    var firstHour = Math.ceil(ticks[0].mins / 60 / step) * step;
    var lastHour = Math.floor(ticks[ticks.length - 1].mins / 60 / step) * step;
    for (var h = firstHour; h <= lastHour; h += step) {
      var x = leftX + h * 60 * pxPerMin;
      var text = /** @type {SVGTextElement} */ (document.createElementNS(NS, "text"));
      text.setAttribute("x", String(x));
      text.setAttribute("y", String(height - 10));
      text.setAttribute("text-anchor", "middle");
      text.setAttribute("font-size", fontSize + "px");
      text.style.fill = "#64748b";
      text.textContent = h + "h";
      svg.appendChild(text);
    }
  }

  // Em telas pequenas o eixo acompanha o intervalo real das atividades (com
  // folga de 2h, mínimo de 8h) em vez de espremer 24h. O modal continua com o
  // dia inteiro. A função pura vive em rotina-utils.js (testável sem DOM).
  function renderTimeline() {
    var el = document.getElementById("rotina-mermaid");
    if (!el || !initMermaid()) return;

    if (isNarrow()) {
      var src = (el.textContent || "").trim();
      if (src.indexOf("gantt") === 0) el.textContent = tightenSource(src);
    }

    window.mermaid.run({ nodes: [el] })
      .then(function () { postProcess(el); })
      .catch(function (err) {
        console.warn("rotina: mermaid.run falhou:", err);
      });
  }

  // ── Formulário (criar/editar) ─────────────────────────────────────────────

  function setFormError(message) {
    var box = document.getElementById("rotina-form-error");
    if (!box) return;
    if (!message) {
      box.textContent = "";
      box.classList.add("hidden");
      return;
    }
    box.textContent = message;
    box.classList.remove("hidden");
  }

  function replaceSchedule(html) {
    var current = document.getElementById("rotina-schedule");
    if (!current) return;
    current.outerHTML = html;
    setFormError("");
    renderTimeline();
  }

  function resetForm() {
    var form = /** @type {HTMLFormElement|null} */ (document.getElementById("rotina-form"));
    if (!form) return;
    form.reset();
    /** @type {HTMLInputElement|null} */
    var id = form.querySelector('[name="id"]');
    if (id) id.value = "";
    var label = document.getElementById("rotina-submit-label");
    if (label) label.textContent = "Adicionar";
    var title = document.getElementById("rotina-form-title-text");
    if (title) title.textContent = "Nova atividade";
    var cancel = document.getElementById("rotina-cancel");
    if (cancel) cancel.classList.add("hidden");
    setFormError("");
  }

  document.addEventListener("submit", function (ev) {
    var form = /** @type {HTMLFormElement} */ (ev.target);
    // getAttribute, e não form.id: o formulário tem um campo `name="id"` e o
    // named getter de HTMLFormElement (LegacyOverrideBuiltIns) sombreia a
    // propriedade `id`, devolvendo o <input> em vez do atributo.
    if (!form || form.getAttribute("id") !== "rotina-form") return;
    ev.preventDefault();

    /** @type {HTMLInputElement|null} */
    var idField = form.querySelector('[name="id"]');
    var id = idField ? idField.value.trim() : "";
    var url = id ? "/api/rotina/update" : "/api/rotina/create";
    var button = /** @type {HTMLButtonElement|null} */ (document.getElementById("rotina-submit"));
    if (button) button.disabled = true;
    setFormError("");

    fetch(url, {
      method: "POST",
      body: new URLSearchParams(/** @type {any} */ (new FormData(form)))
    })
      .then(function (r) {
        return r.text().then(function (text) {
          if (!r.ok) throw new Error((text || "").trim() || ("HTTP " + r.status));
          return text;
        });
      })
      .then(function (html) { replaceSchedule(html); })
      .catch(function (err) { setFormError(err.message || "Erro ao salvar"); })
      .finally(function () {
        var b = /** @type {HTMLButtonElement|null} */ (document.getElementById("rotina-submit"));
        if (b) b.disabled = false;
      });
  });

  document.addEventListener("click", function (ev) {
    var target = /** @type {Element|null} */ (ev.target);
    var cancel = target && target.closest("#rotina-cancel");
    if (cancel) {
      resetForm();
      return;
    }
  });

  /** @param {HTMLElement|null} button */
  window.rotinaEdit = function (button) {
    var form = /** @type {HTMLFormElement|null} */ (document.getElementById("rotina-form"));
    if (!form || !button) return;

    /** @type {HTMLInputElement|null} */
    var id = form.querySelector('[name="id"]');
    /** @type {HTMLInputElement|null} */
    var start = form.querySelector('[name="start_time"]');
    /** @type {HTMLInputElement|null} */
    var end = form.querySelector('[name="end_time"]');
    /** @type {HTMLInputElement|null} */
    var desc = form.querySelector('[name="description"]');
    if (id) id.value = button.dataset.id || "";
    if (start) start.value = button.dataset.start || "";
    if (end) end.value = button.dataset.end || "";
    if (desc) desc.value = button.dataset.description || "";

    var label = document.getElementById("rotina-submit-label");
    if (label) label.textContent = "Salvar";
    var title = document.getElementById("rotina-form-title-text");
    if (title) title.textContent = "Editar atividade";
    var cancel = document.getElementById("rotina-cancel");
    if (cancel) cancel.classList.remove("hidden");

    setFormError("");
    form.scrollIntoView({ behavior: "smooth", block: "center" });
    if (desc) desc.focus({ preventScroll: true });
  };

  /** @param {HTMLElement|null} button */
  window.rotinaDelete = function (button) {
    if (!button) return;
    var id = button.dataset.id || "";
    var date = button.dataset.date || "";
    if (!id) return;
    if (!window.confirm("Excluir esta atividade?")) return;

    fetch("/api/rotina/delete?id=" + encodeURIComponent(id) + "&date=" + encodeURIComponent(date), {
      method: "DELETE"
    })
      .then(function (r) {
        return r.text().then(function (text) {
          if (!r.ok) throw new Error((text || "").trim() || ("HTTP " + r.status));
          return text;
        });
      })
      .then(function (html) { replaceSchedule(html); })
      .catch(function (err) { window.alert("Erro ao excluir: " + (err.message || err)); });
  };

  // ── Zoom (modal com o dia inteiro) ───────────────────────────────────────

  function sourceFromTemplate() {
    var tmpl = /** @type {HTMLTemplateElement|null} */ (document.getElementById("rotina-mermaid-src"));
    if (!tmpl) return "";
    return ((tmpl.content && tmpl.content.textContent) || tmpl.textContent || "").trim();
  }

  function openZoom() {
    var modal = document.getElementById("rotina-zoom-modal");
    var holder = document.getElementById("rotina-zoom-holder");
    var source = sourceFromTemplate();
    if (!modal || !holder || !source || !initMermaid()) return;

    modal.classList.remove("hidden");
    modal.classList.add("flex");
    holder.innerHTML = '<p class="text-xs text-zinc-500">Renderizando…</p>';

    window.mermaid.render("rotina-zoom-" + Date.now(), source)
      .then(function (result) {
        holder.innerHTML = result.svg;
        postProcess(holder);
      })
      .catch(function (err) {
        holder.innerHTML = '<p class="text-xs text-red-400">Erro ao gerar o gráfico: ' +
          String(err).slice(0, 120).replace(/[<>&]/g, "") + "</p>";
      });
  }

  function closeZoom() {
    var modal = document.getElementById("rotina-zoom-modal");
    var holder = document.getElementById("rotina-zoom-holder");
    if (modal) {
      modal.classList.add("hidden");
      modal.classList.remove("flex");
    }
    if (holder) holder.innerHTML = "";
  }

  document.addEventListener("click", function (ev) {
    var target = /** @type {Element|null} */ (ev.target);
    if (target && target.closest("#rotina-zoom-btn")) {
      openZoom();
      return;
    }
    if (target && target.closest("[data-rotina-zoom-close]")) {
      closeZoom();
      return;
    }
    var modal = document.getElementById("rotina-zoom-modal");
    if (modal && ev.target === modal) closeZoom();
  });

  document.addEventListener("keydown", function (ev) {
    if (ev.key === "Escape") closeZoom();
  });

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", renderTimeline);
  } else {
    renderTimeline();
  }

  // Exposto para testes de unidade (sem depender do DOM real).
  window.rotinaInternals = {
    tightenSource: tightenSource,
    postProcess: postProcess
  };
})();
