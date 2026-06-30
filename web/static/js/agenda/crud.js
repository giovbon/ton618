/**
 * crud.js
 * Appointment CRUD operations: load, save, delete, purge, inline edit.
 */

import { normalizarPT } from './date-utils.js';
import { isChronoMatchIsolated } from './date-utils.js';

/**
 * @typedef {import('./timeline.js').Appointment} Appointment
 */

/** Shared in-memory appointment list — updated on every load.
 * @type {Appointment[]}
 */
export let appointments = [];

/**
 * Fetches all appointments from the API, refreshes `appointments`,
 * calls the provided callbacks and dispatches agenda-updated on body.
 * 
 * @param {Object} [options]
 * @param {function(Appointment[]): void} [options.onLoaded]
 * @param {HTMLElement | null} [options.countBadge]
 * @returns {Promise<void>}
 */
export async function loadAppointments({ onLoaded, countBadge } = {}) {
    try {
        const res = await fetch('/api/appointments');
        if (res.ok) {
            appointments = await res.json() || [];
            document.body.dispatchEvent(new Event('agenda-updated'));
            if (countBadge) countBadge.textContent = String(appointments.length);
            if (onLoaded) onLoaded(appointments);
        }
    } catch (e) {
        console.error('Erro ao carregar agenda', e);
    }
}

/**
 * Saves a new appointment parsed from the input text.
 * Calls `reload` after a successful save.
 * 
 * @param {Object} params
 * @param {string} params.text
 * @param {HTMLButtonElement} params.saveBtn
 * @param {HTMLInputElement} params.input
 * @param {HTMLElement} params.preview
 * @param {function(): Promise<void>} params.reload
 * @returns {Promise<void>}
 */
export async function saveAppointment({ text, saveBtn, input, preview, reload }) {
    // @ts-ignore
    const chronoPt = typeof chrono !== 'undefined' ? chrono.pt : null;
    if (!chronoPt || !text) return;

    const normalized = normalizarPT(text);
    const rawResults = chronoPt.parse(normalized, new Date(), { forwardDate: true });
    // @ts-ignore
    const results    = rawResults.filter(r => isChronoMatchIsolated(normalized, r.text));

    let eventDate   = new Date();
    let description = normalized;

    if (results.length > 0) {
        eventDate   = results[0].start.date();
        description = normalized.replace(results[0].text, '').trim() ||
                      `Compromisso (${results[0].text})`;
    }

    // ISO week calculation
    const d = new Date(eventDate);
    d.setHours(0, 0, 0, 0);
    d.setDate(d.getDate() + 3 - (d.getDay() + 6) % 7);
    const week1   = new Date(d.getFullYear(), 0, 4);
    const weekNum = 1 + Math.round(((d.getTime() - week1.getTime()) / 86400000 - 3 + (week1.getDay() + 6) % 7) / 7);

    const pad = (/** @type {number} */ n) => String(n).padStart(2, '0');
    const localFloatingTime = `${eventDate.getFullYear()}-${pad(eventDate.getMonth() + 1)}-${pad(eventDate.getDate())}T${pad(eventDate.getHours())}:${pad(eventDate.getMinutes())}:${pad(eventDate.getSeconds())}`;

    saveBtn.disabled    = true;
    saveBtn.textContent = 'Salvando...';

    try {
        const res = await fetch('/api/appointments/create', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                description,
                event_date:  localFloatingTime,
                year:        eventDate.getFullYear(),
                month:       eventDate.getMonth() + 1,
                week_number: weekNum,
            }),
        });
        if (res.ok) {
            input.value         = '';
            preview.textContent = '';
            await reload();
        } else {
            alert('Erro ao salvar apontamento.');
        }
    } catch (e) {
        console.error(e);
        alert('Erro de conexão ao salvar.');
    } finally {
        saveBtn.disabled    = false;
        saveBtn.textContent = 'Salvar';
    }
}

/** 
 * Purges appointments older than 7 days. 
 * 
 * @param {Object} params
 * @param {HTMLButtonElement} params.purgeBtn
 * @param {function(): Promise<void>} params.reload
 * @returns {Promise<void>}
 */
export async function purgeOldAppointments({ purgeBtn, reload }) {
    const cutoff = new Date();
    cutoff.setDate(cutoff.getDate() - 7);
    const older = appointments.filter(a => new Date(a.event_date) < cutoff);

    if (older.length === 0) {
        alert('Nenhum compromisso com mais de 7 dias encontrado.');
        return;
    }
    if (!confirm(`Excluir ${older.length} compromisso(s) com data anterior a ${cutoff.toLocaleDateString('pt-BR')}?`)) return;

    purgeBtn.disabled    = true;
    purgeBtn.textContent = 'Limpando...';
    try {
        const res = await fetch('/api/appointments/purge-old', { method: 'DELETE' });
        if (res.ok) await reload();
        else alert('Erro ao limpar compromissos antigos.');
    } catch (e) {
        console.error(e);
        alert('Erro de conexão.');
    } finally {
        purgeBtn.disabled  = false;
        purgeBtn.innerHTML = '🗑️ Limpar';
    }
}

/**
 * Deletes a single appointment by id.
 * Exposed on window for HTML onclick buttons in the HTMX-rendered tree.
 * 
 * @param {string} id 
 * @param {function(): Promise<void>} reload 
 * @returns {Promise<void>}
 */
export async function deleteAppointment(id, reload) {
    if (!confirm('Deseja excluir este apontamento?')) return;
    try {
        const res = await fetch(`/api/appointments/delete?id=${id}`, { method: 'DELETE' });
        if (res.ok) await reload();
    } catch (e) {
        console.error(e);
    }
}

/**
 * Starts an inline edit for a tree item.
 * Exposed on window for HTML onclick buttons in the HTMX-rendered tree.
 * 
 * @param {string} id 
 * @param {string} currentDesc 
 * @param {function(): Promise<void>} reload 
 * @returns {void}
 */
export function startEdit(id, currentDesc, reload) {
    const span = document.getElementById(`desc-${id}`);
    if (!span) return;

    const editInput = document.createElement('input');
    editInput.type      = 'text';
    editInput.value     = currentDesc;
    editInput.className = 'bg-transparent border-b border-sky-500 text-sm font-semibold text-zinc-100 outline-none flex-1 w-full';
    span.replaceWith(editInput);

    // @ts-ignore
    if (window.setupAutocomplete) window.setupAutocomplete(editInput);

    editInput.focus();
    editInput.select();

    const commit = async () => {
        const newDesc = editInput.value.trim();
        if (!newDesc || newDesc === currentDesc) { await reload(); return; }
        const app = appointments.find(a => a.id === id);
        if (!app) return;
        try {
            await fetch('/api/appointments/update', {
                method:  'POST',
                headers: { 'Content-Type': 'application/json' },
                body:    JSON.stringify({ ...app, description: newDesc }),
            });
        } catch (e) { console.error(e); }
        await reload();
    };

    editInput.addEventListener('blur', commit);
    editInput.addEventListener('keydown', (e) => {
        if (e.key === 'Enter') {
            // @ts-ignore
            if (window.isAutocompleteVisible && window.isAutocompleteVisible()) return;
            e.preventDefault();
            editInput.blur();
        }
        if (e.key === 'Escape') {
            // @ts-ignore
            if (window.isAutocompleteVisible && window.isAutocompleteVisible()) return;
            editInput.value = currentDesc;
            editInput.blur();
        }
    });
}
