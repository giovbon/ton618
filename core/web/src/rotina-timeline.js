/**
 * rotina-timeline.js — linha do tempo nativa da Rotina.
 *
 * Substitui o gantt do Mermaid por um desenho em DOM (mesma técnica da Agenda
 * em static/js/agenda/timeline.js): um canvas de largura variável, barras
 * posicionadas por tempo, eixo de horas, zoom, arrastar para rolar e tooltip.
 *
 * O servidor entrega as atividades como JSON em
 * `<script id="rotina-timeline-data" type="application/json">`; este módulo só
 * cuida da geometria e da interação. As funções puras (parseClock, layoutBars,
 * formatDuration) são exportadas para os testes de unidade rodarem no Node.
 */

const MINUTES_PER_DAY = 24 * 60;
const LANE_HEIGHT = 26;
const LANE_GAP = 4;
const TOP_PAD = 10;
const AXIS_HEIGHT = 24;
// Abaixo disso o rótulo da hora começa a colidir; o canvas rola na horizontal.
const MIN_PX_PER_HOUR = 36;
const MAX_PX_PER_HOUR = 320;
const MIN_BAR_WIDTH = 4;
const LABEL_MIN_WIDTH = 44;
const ZOOM_FACTOR = 1.4;

/**
 * @typedef {Object} Activity
 * @property {string} id
 * @property {string} start_time
 * @property {string} end_time
 * @property {string} description
 */

/**
 * @typedef {Object} Bar
 * @property {Activity} activity
 * @property {number} startMin
 * @property {number} endMin
 * @property {number} lane
 * @property {boolean} overlapping
 * @property {number} x
 * @property {number} y
 * @property {number} width
 * @property {number} height
 */

/**
 * @typedef {Object} Layout
 * @property {Bar[]} bars
 * @property {number} lanes
 * @property {number} height
 */

/**
 * @typedef {Object} Instance
 * @property {HTMLElement} root
 * @property {HTMLElement} viewport
 * @property {HTMLElement} canvas
 * @property {Activity[]} activities
 * @property {string} date
 * @property {boolean} expanded
 * @property {number} pxPerHour
 * @property {boolean} userZoom
 * @property {boolean} firstDraw
 * @property {boolean} suppressClick
 * @property {() => void} draw
 * @property {() => void} updateNow
 * @property {() => void} destroy
 */

/** @type {Map<HTMLElement, Instance>} */
const instances = new Map();

/** @type {Activity[]} */
let currentActivities = [];
let currentDate = "";

/** @type {number | null} */
let nowTimer = null;

/** @type {HTMLElement | null} */
let tooltip = null;

// ── Funções puras (testáveis sem DOM) ───────────────────────────────────────

/**
 * Converte "HH:MM" em minutos desde 00:00. Devolve null para valor inválido.
 * @param {string} value
 * @returns {number | null}
 */
export function parseClock(value) {
  if (typeof value !== "string") return null;
  const match = value.trim().match(/^(\d{1,2}):(\d{2})$/);
  if (!match) return null;
  const hours = parseInt(match[1], 10);
  const minutes = parseInt(match[2], 10);
  if (hours < 0 || hours > 23 || minutes < 0 || minutes > 59) return null;
  return hours * 60 + minutes;
}

/**
 * Formata minutos em "HH:MM" (aceita 1440 como 24:00).
 * @param {number} value
 * @returns {string}
 */
export function formatClock(value) {
  const total = Math.max(0, Math.min(MINUTES_PER_DAY, Math.round(value)));
  const hours = Math.floor(total / 60);
  const minutes = total % 60;
  return String(hours).padStart(2, "0") + ":" + String(minutes).padStart(2, "0");
}

/**
 * Formata uma duração em minutos: "45min", "2h", "1h 30min".
 * @param {number} value
 * @returns {string}
 */
export function formatDuration(value) {
  const total = Math.max(0, Math.round(value));
  const hours = Math.floor(total / 60);
  const minutes = total % 60;
  if (hours === 0) return minutes + "min";
  if (minutes === 0) return hours + "h";
  return hours + "h " + minutes + "min";
}

/**
 * Distribui as atividades em raias para que sobreposições não se cubram.
 * Também devolve posição/largura em pixels para a largura informada.
 *
 * Regra: a primeira raia cujo último fim <= início da atividade. Atividades
 * encostadas (fim == início) compartilham a mesma raia.
 *
 * @param {Activity[]} activities
 * @param {number} width largura do canvas em pixels
 * @param {{ laneHeight?: number, laneGap?: number, topPad?: number, axisHeight?: number }} [options]
 * @returns {Layout}
 */
export function layoutBars(activities, width, options) {
  const opts = options || {};
  const laneHeight = opts.laneHeight || LANE_HEIGHT;
  const laneGap = opts.laneGap || LANE_GAP;
  const topPad = opts.topPad || TOP_PAD;
  const axisHeight = opts.axisHeight || AXIS_HEIGHT;
  const safeWidth = Math.max(0, Number(width) || 0);
  const pxPerMinute = safeWidth / MINUTES_PER_DAY;
  /** @type {Array<{ activity: Activity, startMin: number, endMin: number }>} */
  const valid = [];

  (Array.isArray(activities) ? activities : []).forEach((activity) => {
    const startMin = parseClock(activity.start_time);
    const endMin = parseClock(activity.end_time);
    if (startMin === null || endMin === null || endMin <= startMin) return;
    valid.push({ activity, startMin, endMin });
  });

  valid.sort((a, b) => {
    if (a.startMin !== b.startMin) return a.startMin - b.startMin;
    if (a.endMin !== b.endMin) return a.endMin - b.endMin;
    return String(a.activity.id || "").localeCompare(String(b.activity.id || ""));
  });

  /** @type {number[]} */
  const laneEnds = [];
  /** @type {Bar[]} */
  const bars = valid.map((item) => {
    let lane = laneEnds.findIndex((end) => end <= item.startMin);
    if (lane === -1) {
      lane = laneEnds.length;
      laneEnds.push(item.endMin);
    } else {
      laneEnds[lane] = item.endMin;
    }
    return {
      activity: item.activity,
      startMin: item.startMin,
      endMin: item.endMin,
      lane,
      overlapping: false,
      x: item.startMin * pxPerMinute,
      y: topPad + lane * (laneHeight + laneGap),
      width: Math.max(MIN_BAR_WIDTH, (item.endMin - item.startMin) * pxPerMinute),
      height: laneHeight
    };
  });

  // Sobreposição real (não apenas "não coube na raia 0"): interseção de
  // intervalos com qualquer outra barra. N é pequeno (atividades de um dia).
  bars.forEach((bar, index) => {
    bar.overlapping = bars.some((other, otherIndex) => {
      if (index === otherIndex) return false;
      return bar.startMin < other.endMin && other.startMin < bar.endMin;
    });
  });

  const lanes = Math.max(1, laneEnds.length);
  return {
    bars,
    lanes,
    height: topPad + lanes * laneHeight + (lanes - 1) * laneGap + axisHeight
  };
}

// ── Estado/leitura do servidor ──────────────────────────────────────────────

/**
 * Lê as atividades do JSON renderizado pelo templ.
 * @returns {Activity[]}
 */
function readActivities() {
  if (typeof document === "undefined") return [];
  const script = document.getElementById("rotina-timeline-data");
  if (!script) return [];
  try {
    const parsed = JSON.parse(script.textContent || "[]");
    return Array.isArray(parsed) ? parsed : [];
  } catch (err) {
    console.warn("rotina: JSON da linha do tempo inválido", err);
    return [];
  }
}

/**
 * Informa se a página está no modo nativo.
 * @returns {boolean}
 */
export function hasNativeTimeline() {
  return typeof document !== "undefined" && !!document.getElementById("rotina-timeline");
}

/** @returns {string} */
function todayString() {
  const now = new Date();
  const month = String(now.getMonth() + 1).padStart(2, "0");
  const day = String(now.getDate()).padStart(2, "0");
  return now.getFullYear() + "-" + month + "-" + day;
}

// ── Tooltip compartilhado ───────────────────────────────────────────────────

/** @returns {HTMLElement} */
function ensureTooltip() {
  if (tooltip) return tooltip;
  tooltip = document.createElement("div");
  tooltip.className = "rotina-tooltip";
  tooltip.setAttribute("role", "tooltip");
  tooltip.hidden = true;
  document.body.appendChild(tooltip);
  return tooltip;
}

/**
 * @param {HTMLElement} bar
 */
function showTooltip(bar) {
  const start = parseClock(bar.dataset.start || "");
  const end = parseClock(bar.dataset.end || "");
  const description = bar.dataset.description || "Atividade";
  if (start === null || end === null) return;

  const el = ensureTooltip();
  el.textContent = "";

  const title = document.createElement("strong");
  title.className = "rotina-tooltip-title";
  title.textContent = description;

  const time = document.createElement("span");
  time.className = "rotina-tooltip-time";
  time.textContent = formatClock(start) + " – " + formatClock(end) + " · " + formatDuration(end - start);

  const hint = document.createElement("span");
  hint.className = "rotina-tooltip-hint";
  hint.textContent = "Clique para editar";

  el.appendChild(title);
  el.appendChild(time);
  el.appendChild(hint);
  el.hidden = false;

  const rect = bar.getBoundingClientRect();
  const tip = el.getBoundingClientRect();
  let left = rect.left + rect.width / 2 - tip.width / 2;
  let top = rect.top - tip.height - 8;
  if (top < 8) top = rect.bottom + 8;
  left = Math.max(8, Math.min(window.innerWidth - tip.width - 8, left));
  el.style.left = left + "px";
  el.style.top = top + "px";
}

function hideTooltip() {
  if (tooltip) tooltip.hidden = true;
}

// ── Desenho ─────────────────────────────────────────────────────────────────

/**
 * @param {HTMLElement} parent
 * @param {string} className
 * @param {number} left
 * @param {number} width
 * @returns {HTMLElement}
 */
function addBlock(parent, className, left, width) {
  const block = document.createElement("div");
  block.className = className;
  block.style.left = left + "px";
  block.style.width = width + "px";
  parent.appendChild(block);
  return block;
}

/**
 * @param {Bar} bar
 * @returns {HTMLButtonElement}
 */
function buildBar(bar) {
  const button = document.createElement("button");
  button.type = "button";
  button.className = "rotina-bar" + (bar.overlapping ? " is-overlap" : "");
  button.style.left = bar.x + "px";
  button.style.top = bar.y + "px";
  button.style.width = bar.width + "px";
  button.style.height = bar.height + "px";

  const description = bar.activity.description || "Atividade";
  button.dataset.id = bar.activity.id || "";
  button.dataset.start = bar.activity.start_time || "";
  button.dataset.end = bar.activity.end_time || "";
  button.dataset.description = description;
  button.setAttribute(
    "aria-label",
    formatClock(bar.startMin) +
      " – " +
      formatClock(bar.endMin) +
      " · " +
      description +
      " · " +
      formatDuration(bar.endMin - bar.startMin) +
      ". Clique para editar."
  );

  if (bar.width >= LABEL_MIN_WIDTH) {
    const label = document.createElement("span");
    label.className = "rotina-bar-label";
    label.textContent = description;
    button.appendChild(label);
  }
  return button;
}

/**
 * @param {HTMLElement} viewport
 * @param {boolean} expanded
 * @returns {number}
 */
function fitPxPerHour(viewport, expanded) {
  const width = viewport.clientWidth || 0;
  const minimum = expanded ? 56 : MIN_PX_PER_HOUR;
  return Math.max(minimum, width / 24);
}

/**
 * @param {Instance} instance
 * @param {number} next
 * @param {number | null} anchorMin minuto que deve ficar sob o cursor
 * @param {number} anchorOffset deslocamento em px dentro do viewport
 */
function applyZoom(instance, next, anchorMin, anchorOffset) {
  const clamped = Math.max(MIN_PX_PER_HOUR, Math.min(MAX_PX_PER_HOUR, next));
  if (Math.abs(clamped - instance.pxPerHour) < 0.01) return;
  instance.pxPerHour = clamped;
  instance.userZoom = true;
  instance.draw();
  if (anchorMin !== null) {
    instance.viewport.scrollLeft = Math.max(
      0,
      (anchorMin / 60) * instance.pxPerHour - anchorOffset
    );
  }
}

/**
 * @param {Instance} instance
 * @param {number} factor
 */
function zoomBy(instance, factor) {
  const viewport = instance.viewport;
  const centerMin =
    ((viewport.scrollLeft + viewport.clientWidth / 2) / instance.pxPerHour) * 60;
  applyZoom(instance, instance.pxPerHour * factor, centerMin, viewport.clientWidth / 2);
}

/**
 * @param {Instance} instance
 */
function fit(instance) {
  instance.userZoom = false;
  instance.pxPerHour = fitPxPerHour(instance.viewport, instance.expanded);
  instance.draw();
  centerOnActivities(instance);
}

/**
 * Centraliza a rolagem horizontal no intervalo das atividades (só quando o
 * canvas é mais largo que o viewport).
 * @param {Instance} instance
 */
function centerOnActivities(instance) {
  const viewport = instance.viewport;
  if (instance.canvas.offsetWidth <= viewport.clientWidth) return;
  const bars = instance.canvas.querySelectorAll(".rotina-bar");
  if (!bars.length) {
    viewport.scrollLeft = 0;
    return;
  }
  let minX = Infinity;
  let maxX = -Infinity;
  bars.forEach((node) => {
    const bar = /** @type {HTMLElement} */ (node);
    minX = Math.min(minX, bar.offsetLeft);
    maxX = Math.max(maxX, bar.offsetLeft + bar.offsetWidth);
  });
  viewport.scrollLeft = Math.max(0, (minX + maxX) / 2 - viewport.clientWidth / 2);
}

/**
 * Monta (ou remonta) a linha do tempo dentro de `root`.
 *
 * `root` precisa conter `[data-rotina-viewport]` e `[data-rotina-canvas]`.
 *
 * @param {HTMLElement} root
 * @param {{ activities?: Activity[], date?: string, expanded?: boolean }} [options]
 * @returns {Instance | null}
 */
export function mount(root, options) {
  if (!root) return null;
  const opts = options || {};
  const viewport = /** @type {HTMLElement | null} */ (
    root.querySelector("[data-rotina-viewport]")
  );
  const canvas = /** @type {HTMLElement | null} */ (
    root.querySelector("[data-rotina-canvas]")
  );
  if (!viewport || !canvas) return null;

  // Remonta sem duplicar listeners/timer.
  unmount(root);

  const expanded = !!opts.expanded;
  const activities = opts.activities || readActivities();
  const date = opts.date || root.getAttribute("data-date") || "";
  if (!expanded) {
    currentActivities = activities;
    currentDate = date;
  }

  /** @type {Instance} */
  const instance = {
    root,
    viewport,
    canvas,
    activities,
    date,
    expanded,
    pxPerHour: fitPxPerHour(viewport, expanded),
    userZoom: false,
    firstDraw: true,
    suppressClick: false,
    draw: () => {},
    updateNow: () => {},
    destroy: () => {}
  };

  instance.draw = () => {
    const width = Math.max(1, Math.round(instance.pxPerHour * 24));
    const metrics = instance.expanded
      ? { laneHeight: 34, laneGap: 6, topPad: 14, axisHeight: 26 }
      : undefined;
    const layout = layoutBars(instance.activities, width, metrics);

    canvas.textContent = "";
    canvas.style.width = width + "px";
    canvas.style.height = layout.height + "px";

    // Fora do "dia útil": 00h–06h e 22h–24h.
    addBlock(canvas, "rotina-offhours", 0, width * (6 / 24));
    addBlock(canvas, "rotina-offhours", width * (22 / 24), width * (2 / 24));

    const labelStep =
      instance.pxPerHour >= 90 ? 1 : instance.pxPerHour >= 52 ? 2 : 3;
    for (let hour = 0; hour <= 24; hour++) {
      const x = hour * instance.pxPerHour;
      const line = document.createElement("div");
      line.className = "rotina-hour-line" + (hour % 6 === 0 ? " is-major" : "");
      line.style.left = x + "px";
      canvas.appendChild(line);

      if (hour < 24 && hour % labelStep === 0) {
        const label = document.createElement("span");
        label.className = "rotina-hour-label" + (hour === 0 ? " is-first" : "");
        label.textContent = hour + "h";
        if (hour > 0) label.style.left = x + "px";
        canvas.appendChild(label);
      }
    }

    layout.bars.forEach((bar) => {
      canvas.appendChild(buildBar(bar));
    });

    instance.updateNow();
    root.setAttribute("data-ready", "1");

    if (instance.firstDraw) {
      instance.firstDraw = false;
      // Espera o layout do canvas para medir as barras.
      if (typeof window !== "undefined") {
        window.requestAnimationFrame(() => centerOnActivities(instance));
      }
    }
  };

  instance.updateNow = () => {
    const existing = /** @type {HTMLElement | null} */ (
      canvas.querySelector(".rotina-now")
    );
    if (instance.date !== todayString()) {
      if (existing) existing.remove();
      return;
    }

    const now = new Date();
    const minutes =
      now.getHours() * 60 + now.getMinutes() + now.getSeconds() / 60;
    const width = Math.max(1, instance.pxPerHour * 24);
    const x = (minutes / MINUTES_PER_DAY) * width;

    let line = existing;
    if (!line) {
      line = document.createElement("div");
      line.className = "rotina-now";
      const label = document.createElement("span");
      label.className = "rotina-now-label";
      line.appendChild(label);
      canvas.appendChild(line);
    }
    line.style.left = x + "px";
    line.classList.toggle("is-right", x > width - 48);
    const label = line.querySelector(".rotina-now-label");
    if (label) label.textContent = formatClock(minutes);
  };

  const cleanup = bindEvents(instance);
  instance.destroy = () => {
    cleanup();
    root.removeAttribute("data-ready");
    hideTooltip();
    instances.delete(root);
    stopNowTimerIfIdle();
  };

  instances.set(root, instance);
  instance.draw();
  ensureNowTimer();
  return instance;
}

/**
 * Remove a instância de `root` (se existir).
 * @param {HTMLElement | null} root
 */
export function unmount(root) {
  if (!root) return;
  const instance = instances.get(root);
  if (instance) instance.destroy();
}

/**
 * Remove todas as instâncias (usado quando o bloco #rotina-schedule é trocado).
 */
export function unmountAll() {
  Array.from(instances.keys()).forEach((root) => unmount(root));
}

/**
 * Remove instâncias dentro de um container (ex.: o holder do modal).
 * @param {HTMLElement | null} container
 */
export function unmountWithin(container) {
  if (!container) return;
  Array.from(instances.keys()).forEach((root) => {
    if (container === root || container.contains(root)) unmount(root);
  });
}

/**
 * Abre a versão ampliada dentro do holder do modal.
 * @param {HTMLElement | null} holder
 * @returns {Instance | null}
 */
export function openZoom(holder) {
  if (!holder || typeof document === "undefined") return null;
  unmountWithin(holder);
  holder.textContent = "";

  const activities = currentActivities.length ? currentActivities : readActivities();
  const date = currentDate || todayString();

  const root = document.createElement("div");
  root.id = "rotina-timeline-zoom";
  root.className = "rotina-timeline rotina-timeline-expanded";
  root.setAttribute("data-date", date);

  const viewport = document.createElement("div");
  viewport.className = "rotina-timeline-viewport";
  viewport.setAttribute("data-rotina-viewport", "");

  const canvas = document.createElement("div");
  canvas.className = "rotina-timeline-canvas";
  canvas.setAttribute("data-rotina-canvas", "");

  viewport.appendChild(canvas);
  root.appendChild(viewport);
  holder.appendChild(root);

  return mount(root, { expanded: true, activities, date });
}

// ── Eventos ─────────────────────────────────────────────────────────────────

/**
 * @param {Instance} instance
 * @returns {() => void} cleanup
 */
function bindEvents(instance) {
  const { canvas, viewport } = instance;

  /**
   * @param {Event} ev
   * @returns {HTMLElement | null}
   */
  const barFrom = (ev) => {
    const target = /** @type {Element | null} */ (ev.target);
    if (!target || !target.closest) return null;
    const bar = target.closest(".rotina-bar");
    return bar && canvas.contains(bar) ? /** @type {HTMLElement} */ (bar) : null;
  };

  /** @param {Event} ev */
  const onClick = (ev) => {
    if (instance.suppressClick) {
      instance.suppressClick = false;
      return;
    }
    const bar = barFrom(ev);
    if (!bar) return;
    hideTooltip();
    if (typeof window.rotinaEdit === "function") window.rotinaEdit(bar);
  };

  /** @param {PointerEvent} ev */
  const onPointerOver = (ev) => {
    const bar = barFrom(ev);
    if (!bar) return;
    const related = /** @type {Node | null} */ (ev.relatedTarget);
    if (related && bar.contains(related)) return;
    showTooltip(bar);
  };

  /** @param {PointerEvent} ev */
  const onPointerOut = (ev) => {
    const bar = barFrom(ev);
    if (!bar) return;
    const related = /** @type {Node | null} */ (ev.relatedTarget);
    if (related && bar.contains(related)) return;
    hideTooltip();
  };

  /** @param {FocusEvent} ev */
  const onFocusIn = (ev) => {
    const bar = barFrom(ev);
    if (bar) showTooltip(bar);
  };

  /** @param {FocusEvent} ev */
  const onFocusOut = (ev) => {
    const bar = barFrom(ev);
    if (bar) hideTooltip();
  };

  /** @type {{ x: number, scroll: number, moved: boolean, pointerId: number } | null} */
  let drag = null;

  /** @param {PointerEvent} ev */
  const onPointerDown = (ev) => {
    if (ev.button !== 0) return;
    // No toque, deixa o scroll nativo do viewport cuidar do gesto.
    if (ev.pointerType && ev.pointerType !== "mouse") return;
    if (barFrom(ev)) return;
    drag = {
      x: ev.clientX,
      scroll: viewport.scrollLeft,
      moved: false,
      pointerId: ev.pointerId
    };
    viewport.classList.add("is-dragging");
    if (viewport.setPointerCapture) viewport.setPointerCapture(ev.pointerId);
  };

  /** @param {PointerEvent} ev */
  const onPointerMove = (ev) => {
    if (!drag) return;
    const delta = ev.clientX - drag.x;
    if (Math.abs(delta) > 4) drag.moved = true;
    if (!drag.moved) return;
    viewport.scrollLeft = drag.scroll - delta;
    instance.suppressClick = true;
    ev.preventDefault();
  };

  /** @param {PointerEvent} ev */
  const onPointerUp = (ev) => {
    if (!drag) return;
    const moved = drag.moved;
    drag = null;
    viewport.classList.remove("is-dragging");
    if (viewport.hasPointerCapture && viewport.hasPointerCapture(ev.pointerId)) {
      viewport.releasePointerCapture(ev.pointerId);
    }
    if (moved) {
      // Fallback: se nenhum click vier depois do arrasto, não deixa a flag
      // presa e engolindo o próximo clique.
      window.setTimeout(() => {
        instance.suppressClick = false;
      }, 250);
    }
  };

  /** @param {WheelEvent} ev */
  const onWheel = (ev) => {
    if (!ev.ctrlKey && !ev.metaKey) return;
    ev.preventDefault();
    const rect = viewport.getBoundingClientRect();
    const mouseX = ev.clientX - rect.left;
    const anchorMin =
      ((viewport.scrollLeft + mouseX) / instance.pxPerHour) * 60;
    const factor = ev.deltaY > 0 ? 1 / 1.15 : 1.15;
    applyZoom(instance, instance.pxPerHour * factor, anchorMin, mouseX);
  };

  const onScroll = () => hideTooltip();

  /** @type {ResizeObserver | null} */
  let observer = null;
  let lastWidth = viewport.clientWidth;
  if (typeof ResizeObserver !== "undefined") {
    observer = new ResizeObserver(() => {
      const width = viewport.clientWidth;
      if (!width || width === lastWidth) return;
      lastWidth = width;
      if (!instance.userZoom) {
        instance.pxPerHour = fitPxPerHour(viewport, instance.expanded);
      }
      instance.draw();
    });
    observer.observe(viewport);
  }

  canvas.addEventListener("click", onClick);
  canvas.addEventListener("pointerover", onPointerOver);
  canvas.addEventListener("pointerout", onPointerOut);
  canvas.addEventListener("focusin", onFocusIn);
  canvas.addEventListener("focusout", onFocusOut);
  viewport.addEventListener("pointerdown", onPointerDown);
  viewport.addEventListener("pointermove", onPointerMove);
  viewport.addEventListener("pointerup", onPointerUp);
  viewport.addEventListener("pointercancel", onPointerUp);
  viewport.addEventListener("wheel", onWheel, { passive: false });
  viewport.addEventListener("scroll", onScroll, { passive: true });

  return () => {
    canvas.removeEventListener("click", onClick);
    canvas.removeEventListener("pointerover", onPointerOver);
    canvas.removeEventListener("pointerout", onPointerOut);
    canvas.removeEventListener("focusin", onFocusIn);
    canvas.removeEventListener("focusout", onFocusOut);
    viewport.removeEventListener("pointerdown", onPointerDown);
    viewport.removeEventListener("pointermove", onPointerMove);
    viewport.removeEventListener("pointerup", onPointerUp);
    viewport.removeEventListener("pointercancel", onPointerUp);
    viewport.removeEventListener("wheel", onWheel);
    viewport.removeEventListener("scroll", onScroll);
    if (observer) observer.disconnect();
  };
}

function ensureNowTimer() {
  if (nowTimer !== null || typeof window === "undefined") return;
  nowTimer = window.setInterval(() => {
    instances.forEach((instance) => instance.updateNow());
  }, 60000);
}

function stopNowTimerIfIdle() {
  if (instances.size > 0 || nowTimer === null) return;
  window.clearInterval(nowTimer);
  nowTimer = null;
}

// Botões de zoom ficam no cabeçalho da seção, fora do root da timeline.
if (typeof document !== "undefined") {
  document.addEventListener("click", (ev) => {
    const target = /** @type {Element | null} */ (ev.target);
    if (!target || !target.closest) return;
    const button = target.closest(
      "[data-rotina-zoom-in],[data-rotina-zoom-out],[data-rotina-zoom-fit]"
    );
    if (!button) return;

    const root = document.getElementById("rotina-timeline");
    const instance = root ? instances.get(root) : null;
    if (!instance) return;

    if (button.hasAttribute("data-rotina-zoom-in")) zoomBy(instance, ZOOM_FACTOR);
    else if (button.hasAttribute("data-rotina-zoom-out"))
      zoomBy(instance, 1 / ZOOM_FACTOR);
    else fit(instance);
  });
}
