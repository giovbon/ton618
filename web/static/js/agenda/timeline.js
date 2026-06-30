/**
 * timeline.js
 * Custom canvas-based timeline renderer.
 * Draws day/night shades, day axis, weekend highlights,
 * month labels, holidays, the current-time red line, and appointment dots.
 */

import { getTagColor } from './tags.js';
import { showPinnedTooltip, removePinnedTooltip } from './tooltip.js';

const WEEKDAYS_PT   = ['dom', 'seg', 'ter', 'qua', 'qui', 'sex', 'sáb'];
const MONTHS_PT_SHORT = ['jan', 'fev', 'mar', 'abr', 'mai', 'jun', 'jul', 'ago', 'set', 'out', 'nov', 'dez'];

// Shared state — updated by saveTimezoneSetting
let lat = -23.5505;
let lng = -46.6333;

export function setCoords(newLat, newLng) {
    lat = newLat;
    lng = newLng;
}

/**
 * Renders the full timeline into `timelineContainer`.
 * @param {HTMLElement} timelineContainer
 * @param {Array}       appointments
 * @param {Array}       holidaysList
 */
export function renderTimeline(timelineContainer, appointments, holidaysList) {
    timelineContainer.innerHTML = '';
    timelineContainer.style.overflow = 'hidden';

    // Read actual pixel height so absolute children have a valid reference
    const tlHeight = timelineContainer.offsetHeight || 144;

    const container = document.createElement('div');
    container.className = 'custom-timeline-scroll w-full overflow-x-auto overflow-y-hidden select-none';
    container.style.cssText = `position:relative; scroll-behavior:auto; height:${tlHeight}px; cursor:grab;`;

    const canvas = document.createElement('div');
    canvas.className = 'custom-timeline-canvas bg-zinc-950';
    canvas.style.cssText = `position:relative; height:${tlHeight}px;`;
    container.appendChild(canvas);

    const now       = new Date();
    const startDate = new Date(now.getFullYear(), now.getMonth() - 3, now.getDate(), 0, 0, 0);
    const endDate   = new Date(now.getFullYear(), now.getMonth() + 9, now.getDate(), 23, 59, 59);
    const totalMs   = endDate - startDate;

    // Default: 1 day = 140 px
    let msPerPixel = (24 * 60 * 60 * 1000) / 140;

    let redLineInterval;

    function draw() {
        canvas.innerHTML = '';
        const totalWidth = totalMs / msPerPixel;
        canvas.style.width = `${totalWidth}px`;

        const scaleDaysPx = (24 * 60 * 60 * 1000) / msPerPixel;
        let scale = 'days';
        if (scaleDaysPx < 10)  scale = 'months';
        else if (scaleDaysPx < 50) scale = 'weeks';

        // ── 1. Night shades ────────────────────────────────────────────────
        if (typeof SunCalc !== 'undefined' && scale === 'days') {
            let shadeStart = new Date(startDate);
            while (shadeStart <= endDate) {
                const times    = SunCalc.getTimes(shadeStart, lat, lng);
                const nextDay  = new Date(shadeStart.getTime() + 86400000);
                const nextTimes = SunCalc.getTimes(nextDay, lat, lng);

                if (times.sunset && nextTimes.sunrise &&
                    times.sunset >= startDate && nextTimes.sunrise <= endDate) {
                    const xStart = (times.sunset    - startDate) / msPerPixel;
                    const xEnd   = (nextTimes.sunrise - startDate) / msPerPixel;
                    const shade  = document.createElement('div');
                    shade.style.cssText = `position:absolute; top:0; bottom:0; left:${xStart}px; width:${xEnd - xStart}px;` +
                        `background:linear-gradient(180deg,rgba(13,20,38,0.72) 0%,rgba(9,9,11,0.45) 100%); pointer-events:none;`;
                    canvas.appendChild(shade);
                }
                shadeStart.setDate(shadeStart.getDate() + 1);
            }
        }

        // ── 2. Day / week / month axis ─────────────────────────────────────
        let cur = new Date(startDate);
        let lastMonthStr = '';

        while (cur <= endDate) {
            const colMs   = cur.getTime() - startDate.getTime();
            const colLeft = colMs / msPerPixel;

            let nextDate = new Date(cur);
            if (scale === 'months') {
                nextDate.setMonth(nextDate.getMonth() + 1, 1);
                if (cur.getDate() !== 1)
                    nextDate = new Date(cur.getFullYear(), cur.getMonth() + 1, 1);
            } else if (scale === 'weeks') {
                nextDate.setDate(nextDate.getDate() + 7);
                const dayNr = (nextDate.getDay() + 6) % 7;
                nextDate.setDate(nextDate.getDate() - dayNr);
                if (cur.getDay() !== 1)
                    nextDate = new Date(cur.getFullYear(), cur.getMonth(),
                        cur.getDate() + (8 - (cur.getDay() || 7)));
            } else {
                nextDate.setDate(nextDate.getDate() + 1);
                nextDate.setHours(0, 0, 0, 0);
            }
            if (nextDate > endDate) nextDate = new Date(endDate);
            if (nextDate <= cur) break;

            const colWidth = (nextDate.getTime() - startDate.getTime() - colMs) / msPerPixel;
            const monthStr = `${MONTHS_PT_SHORT[cur.getMonth()]} ${cur.getFullYear()}`;

            const col = document.createElement('div');
            col.style.cssText = `position:absolute; top:0; bottom:0; left:${colLeft}px; width:${colWidth}px; border-left:1px solid rgba(63,63,70,0.5);`;

            if (scale === 'months') {
                const lbl = document.createElement('div');
                lbl.style.cssText = 'position:absolute; left:10px; top:20px; font-size:13px; font-weight:700; text-transform:uppercase; letter-spacing:0.05em; color:#f4f4f5; z-index:10;';
                lbl.textContent = monthStr;
                col.appendChild(lbl);

            } else if (scale === 'weeks') {
                if (monthStr !== lastMonthStr) {
                    lastMonthStr = monthStr;
                    const ml = document.createElement('div');
                    ml.style.cssText = 'position:absolute; left:10px; top:6px; font-size:9px; font-weight:700; text-transform:uppercase; letter-spacing:0.05em; color:#71717a; z-index:10;';
                    ml.textContent = monthStr;
                    col.appendChild(ml);
                }
                const wl = document.createElement('div');
                wl.style.cssText = 'position:absolute; left:10px; top:20px; font-size:11px; font-family:monospace; font-weight:700; color:#d4d4d8; z-index:10;';
                wl.textContent = `Semana ${cur.getDate()} ${MONTHS_PT_SHORT[cur.getMonth()]}`;
                col.appendChild(wl);

            } else {
                const dow = cur.getDay();
                if (dow === 0 || dow === 6) {
                    col.style.background = 'repeating-linear-gradient(45deg,rgba(63,63,70,0.18),rgba(63,63,70,0.18) 6px,transparent 6px,transparent 12px)';
                }
                if (monthStr !== lastMonthStr || colLeft === 0) {
                    lastMonthStr = monthStr;
                    const ml = document.createElement('div');
                    ml.style.cssText = 'position:absolute; left:10px; top:6px; font-size:9px; font-weight:700; text-transform:uppercase; letter-spacing:0.05em; color:#71717a; z-index:10;';
                    ml.textContent = monthStr;
                    col.appendChild(ml);
                }
                const isToday = cur.toDateString() === now.toDateString();
                const dl = document.createElement('div');
                dl.style.cssText = `position:absolute; left:10px; top:20px; font-size:13px; font-family:monospace; font-weight:${isToday ? 700 : 400}; color:${isToday ? '#f4f4f5' : '#a1a1aa'}; z-index:10;`;
                dl.textContent = `${WEEKDAYS_PT[dow]} ${String(cur.getDate()).padStart(2, '0')}`;
                col.appendChild(dl);
            }

            canvas.appendChild(col);
            cur = nextDate;
        }

        // ── 3. Holidays ────────────────────────────────────────────────────
        holidaysList.forEach((h) => {
            const hDate = new Date(h.date + 'T12:00:00');
            if (hDate < startDate || hDate > endDate) return;

            const hLeft  = (hDate - startDate) / msPerPixel;
            const hWidth = 86400000 / msPerPixel;

            const hCol = document.createElement('div');
            hCol.style.cssText = `position:absolute; top:0; bottom:0; left:${hLeft - hWidth / 2}px; width:${hWidth}px;` +
                `background:rgba(244,63,94,0.04); border-left:1px solid rgba(159,18,57,0.3); border-right:1px solid rgba(159,18,57,0.3); pointer-events:none;`;

            const hLabel = document.createElement('div');
            hLabel.style.cssText = 'position:absolute; left:10px; bottom:22px; font-size:8px; font-weight:700; color:rgba(251,113,133,0.85); width:calc(100% - 20px); overflow:hidden; text-overflow:ellipsis; white-space:nowrap; pointer-events:none; z-index:10;';
            hLabel.textContent = h.name;
            hCol.appendChild(hLabel);
            canvas.appendChild(hCol);
        });

        // ── 4. Current-time red line ───────────────────────────────────────
        const updateRedLine = () => {
            const nowTime = new Date();
            if (nowTime < startDate || nowTime > endDate) return;
            const xNow = (nowTime - startDate) / msPerPixel;
            let redLine = canvas.querySelector('.timeline-current-time');
            if (!redLine) {
                redLine = document.createElement('div');
                redLine.className = 'timeline-current-time';
                redLine.style.cssText = 'position:absolute; top:0; bottom:0; width:1.5px; background-color:#ef4444; z-index:10; pointer-events:none;';
                canvas.appendChild(redLine);
            }
            redLine.style.left = `${xNow}px`;
        };
        updateRedLine();
        if (redLineInterval) clearInterval(redLineInterval);
        redLineInterval = setInterval(updateRedLine, 60000);

        // ── 5. Appointment dots ────────────────────────────────────────────
        const sorted = [...appointments].sort((a, b) => a.event_date.localeCompare(b.event_date));
        const tracks = [0, 0, 0];

        sorted.forEach((app) => {
            const appDate = new Date(app.event_date);
            if (appDate < startDate || appDate > endDate) return;

            const xApp = (appDate - startDate) / msPerPixel;

            let trackIndex = 0;
            let found = false;
            for (let t = 0; t < tracks.length; t++) {
                if (xApp > tracks[t] + 16) { trackIndex = t; found = true; break; }
            }
            if (!found) {
                let minTrack = 0;
                for (let j = 1; j < tracks.length; j++) {
                    if (tracks[j] < tracks[minTrack]) minTrack = j;
                }
                trackIndex = minTrack;
            }
            tracks[trackIndex] = xApp;

            const dotY   = 54 + trackIndex * 18;
            const tags   = app.description.match(/#[\w\-]+/g) || [];
            const dotColor = tags.length > 0 ? getTagColor(tags[0]).base : '#38bdf8';

            const dot = document.createElement('div');
            dot.style.cssText = `position:absolute; left:${xApp}px; top:${dotY}px; width:9px; height:9px; border-radius:50%; background-color:${dotColor}; box-shadow:0 0 0 2px ${dotColor}40; transform:translate(-50%,-50%); cursor:pointer; z-index:20; transition:transform 0.15s,box-shadow 0.15s;`;

            dot.addEventListener('mouseenter', () => { dot.style.transform = 'translate(-50%,-50%) scale(1.6)'; dot.style.boxShadow = `0 0 10px ${dotColor}`; });
            dot.addEventListener('mouseleave', () => { dot.style.transform = 'translate(-50%,-50%) scale(1)';   dot.style.boxShadow = `0 0 0 2px ${dotColor}40`; });
            dot.addEventListener('click', (e) => {
                e.stopPropagation();
                removePinnedTooltip();
                showPinnedTooltip(app, dot);
            });

            canvas.appendChild(dot);
        });
    }

    draw();
    timelineContainer.appendChild(container);

    // Scroll to show yesterday at left edge
    const yesterday = new Date(now.getFullYear(), now.getMonth(), now.getDate() - 1);
    const xYesterday = (yesterday - startDate) / msPerPixel;
    container.scrollLeft = xYesterday;
    setTimeout(() => { container.scrollLeft = xYesterday; }, 50);

    // ── Zoom ────────────────────────────────────────────────────────────────
    container.addEventListener('wheel', (e) => {
        e.preventDefault();
        const mouseX      = e.pageX - container.getBoundingClientRect().left;
        const timeAtMouse = startDate.getTime() + (container.scrollLeft + mouseX) * msPerPixel;

        let newMpp = msPerPixel * (e.deltaY > 0 ? 1.2 : 0.8);
        const maxMpp = (30 * 86400000) / 100;
        const minMpp = 3600000 / 100;
        newMpp = Math.min(maxMpp, Math.max(minMpp, newMpp));

        if (Math.abs(newMpp - msPerPixel) > 1000) {
            msPerPixel = newMpp;
            draw();
            container.scrollLeft = (timeAtMouse - startDate.getTime()) / msPerPixel - mouseX;
        }
    }, { passive: false });

    // ── Drag to scroll ───────────────────────────────────────────────────────
    let isDown = false, startX, scrollLeft;
    container.addEventListener('mousedown', (e) => { isDown = true; startX = e.pageX - container.offsetLeft; scrollLeft = container.scrollLeft; container.style.cursor = 'grabbing'; });
    container.addEventListener('mouseleave', () => { isDown = false; container.style.cursor = 'grab'; });
    container.addEventListener('mouseup',    () => { isDown = false; container.style.cursor = 'grab'; });
    container.addEventListener('mousemove', (e) => {
        if (!isDown) return;
        e.preventDefault();
        container.scrollLeft = scrollLeft - (e.pageX - container.offsetLeft - startX) * 1.5;
    });
}

/** Fetches holidays for prev/current/next years, caches in localStorage, returns array. */
export async function loadHolidays() {
    const startYear = new Date().getFullYear() - 1;
    const endYear   = new Date().getFullYear() + 2;
    const result    = [];

    for (let y = startYear; y <= endYear; y++) {
        const key    = `agenda-holidays-${y}`;
        const cached = localStorage.getItem(key);
        if (cached) {
            try { result.push(...JSON.parse(cached)); continue; }
            catch { /* fall through to fetch */ }
        }
        try {
            const res = await fetch(`https://brasilapi.com.br/api/feriados/v1/${y}`);
            if (res.ok) {
                const data = await res.json();
                localStorage.setItem(key, JSON.stringify(data));
                result.push(...data);
            }
        } catch (e) {
            console.warn(`Falha ao buscar feriados de ${y}:`, e);
        }
    }
    return result;
}
