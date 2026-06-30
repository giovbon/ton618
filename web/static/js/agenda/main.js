/**
 * main.js
 * Entry point for the Agenda page.
 * Wires up all modules, DOM elements, and global window hooks.
 */

import { normalizarPT, isChronoMatchIsolated, formatPreviewDate } from './date-utils.js';
import { getTagColor } from './tags.js';
import { renderTimeline, loadHolidays, setCoords } from './timeline.js';
import {
    showPinnedTooltip, removePinnedTooltip,
    isRecentPin, tooltipContains,
} from './tooltip.js';
import {
    appointments, loadAppointments,
    saveAppointment, purgeOldAppointments,
    deleteAppointment, startEdit,
} from './crud.js';

/**
 * @typedef {import('./timeline.js').Appointment} Appointment
 * @typedef {import('./timeline.js').Holiday} Holiday
 */

// ─── Keep window globals for test compatibility ──────────────────────────────
// @ts-ignore
window.normalizarPT          = normalizarPT;
// @ts-ignore
window.isChronoMatchIsolated = isChronoMatchIsolated;

// ─── Timezone coordinates map ────────────────────────────────────────────────
const TIMEZONE_COORDS = {
    'America/Sao_Paulo': { lat: -23.5505, lng: -46.6333 },
    'America/Manaus':    { lat: -3.1190,  lng: -60.0217 },
    'America/Noronha':   { lat: -3.8548,  lng: -32.4231 },
    'America/Belem':     { lat: -1.4558,  lng: -48.4902 },
    'America/Fortaleza': { lat: -3.7319,  lng: -38.5267 },
    'America/New_York':  { lat: 40.7128,  lng: -74.0060 },
    'Europe/London':     { lat: 51.5074,  lng: -0.1278  },
    'Europe/Paris':      { lat: 48.8566,  lng: 2.3522   },
    'UTC':               { lat: 0,        lng: 0         },
};

document.addEventListener('DOMContentLoaded', async () => {
    // ── DOM refs ──────────────────────────────────────────────────────────────
    /** @type {HTMLInputElement | null} */
    const input             = document.querySelector('#agenda-input');
    const preview           = document.getElementById('agenda-preview');
    /** @type {HTMLButtonElement | null} */
    const saveBtn           = document.querySelector('#agenda-save');
    /** @type {HTMLButtonElement | null} */
    const purgeBtn          = document.querySelector('#agenda-purge');
    const treeContainer     = document.getElementById('agenda-tree');
    const timelineContainer = document.getElementById('agenda-timeline');
    const countBadge        = document.getElementById('agenda-count');

    if (!input || !timelineContainer || !preview || !saveBtn || !purgeBtn) return; // not on agenda page

    // ── Chrono sanity check ───────────────────────────────────────────────────
    // @ts-ignore
    const chronoPt = typeof chrono !== 'undefined' ? chrono.pt : null;
    if (!chronoPt) {
        console.error('chrono-node Portuguese parser not loaded.');
        preview.textContent = 'Erro: Analisador de data não disponível offline.';
        return;
    }

    // ── Timezone init ─────────────────────────────────────────────────────────
    const storedTz = localStorage.getItem('agenda-timezone') || 'America/Sao_Paulo';
    // @ts-ignore
    const coords   = TIMEZONE_COORDS[storedTz] || TIMEZONE_COORDS['America/Sao_Paulo'];
    setCoords(coords.lat, coords.lng);

    /** @type {HTMLSelectElement | null} */
    const tzSelect = document.querySelector('#setting-agenda-timezone');
    if (tzSelect) tzSelect.value = storedTz;

    // @ts-ignore
    window.saveTimezoneSetting = (tz) => {
        localStorage.setItem('agenda-timezone', tz);
        // @ts-ignore
        const c = TIMEZONE_COORDS[tz] || TIMEZONE_COORDS['America/Sao_Paulo'];
        setCoords(c.lat, c.lng);
        redrawTimeline();
    };

    // ── Shared state ──────────────────────────────────────────────────────────
    /** @type {Appointment[]} */
    let currentAppointments = [];
    /** @type {Holiday[]} */
    let currentHolidays     = [];

    function redrawTimeline() {
        if (!timelineContainer) return;
        renderTimeline(timelineContainer, currentAppointments, currentHolidays);
    }

    const reload = async () => {
        await loadAppointments({
            countBadge,
            onLoaded: (apps) => {
                currentAppointments = apps;
                redrawTimeline();
            },
        });
    };

    // ── Expose window hooks for HTMX-rendered tree buttons ───────────────────
    // @ts-ignore
    window.deleteAppointment = (id)              => deleteAppointment(id, reload);
    // @ts-ignore
    window.startEdit         = (id, currentDesc) => startEdit(id, currentDesc, reload);

    // ── Input preview ─────────────────────────────────────────────────────────
    input.addEventListener('input', (e) => {
        const target = /** @type {HTMLInputElement} */ (e.target);
        const text = target.value;
        if (!text.trim()) { preview.textContent = ''; return; }

        const normalized = normalizarPT(text);
        const results    = chronoPt.parse(normalized, new Date(), { forwardDate: true });
        // @ts-ignore
        const isolated   = results.filter(r => isChronoMatchIsolated(normalized, r.text));

        if (isolated.length > 0) {
            preview.textContent = `Data reconhecida: ${formatPreviewDate(isolated[0].start.date())}`;
            preview.className   = 'text-xs text-emerald-400 mt-2 h-4';
        } else {
            preview.textContent = 'Nenhuma data reconhecida...';
            preview.className   = 'text-xs text-zinc-500 mt-2 h-4';
        }
    });

    // ── Autocomplete ──────────────────────────────────────────────────────────
    // @ts-ignore
    if (window.setupAutocomplete) window.setupAutocomplete(input);

    // ── Save ──────────────────────────────────────────────────────────────────
    saveBtn.addEventListener('click', async () => {
        const text = input.value.trim();
        if (!text) return;
        await saveAppointment({ text, saveBtn, input, preview, reload });
    });

    // ── Purge ─────────────────────────────────────────────────────────────────
    purgeBtn.addEventListener('click', () => purgeOldAppointments({ purgeBtn, reload }));

    // ── Tree click delegation (edit / delete) ─────────────────────────────────
    if (treeContainer) {
        treeContainer.addEventListener('click', (e) => {
            const target = /** @type {HTMLElement} */ (e.target);
            const editBtn = target.closest('.edit-btn');
            if (editBtn) {
                // @ts-ignore
                const { id, desc } = editBtn.dataset;
                if (id && desc) startEdit(id, desc, reload);
            }
            const delBtn = target.closest('.del-btn');
            // @ts-ignore
            if (delBtn && delBtn.dataset.id) deleteAppointment(delBtn.dataset.id, reload);
        });
    }

    // ── Global click: close pinned tooltip ───────────────────────────────────
    document.addEventListener('click', (e) => {
        if (isRecentPin()) return;
        if (!tooltipContains(/** @type {Node} */ (e.target))) removePinnedTooltip();
    });

    // ── Initial data load ─────────────────────────────────────────────────────
    currentHolidays = await loadHolidays();

    await loadAppointments({
        countBadge,
        onLoaded: (apps) => {
            currentAppointments = apps;
            redrawTimeline();
        },
    });

    // Re-render timeline after holidays load (they may arrive after appointments)
    redrawTimeline();
});
