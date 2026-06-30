/**
 * tooltip.js
 * Pinned tooltip management for appointment dots on the timeline.
 */

import { formatTooltipDescription } from './tags.js';

let pinnedTooltip = null;
let lastPinnedTime = 0;

/** Returns a human-readable "em Xd Yh Zm" string for a future event. */
export function getRemainingTimeText(eventDate) {
    const now = new Date();
    const diffMs = eventDate - now;
    if (diffMs <= 0) return '';

    const diffMins  = Math.floor(diffMs / 60000);
    const diffHours = Math.floor(diffMins / 60);
    const diffDays  = Math.floor(diffHours / 24);
    const rHours    = diffHours % 24;
    const rMins     = diffMins % 60;

    const parts = [];
    if (diffDays > 0)            parts.push(`${diffDays}d`);
    if (rHours > 0 || diffDays > 0) parts.push(`${rHours}h`);
    parts.push(`${rMins}m`);
    return `em ${parts.join(' ')}`;
}

export function removePinnedTooltip() {
    if (pinnedTooltip) {
        pinnedTooltip.remove();
        pinnedTooltip = null;
        document.body.classList.remove('has-pinned-tooltip');
    }
}

export function showPinnedTooltip(app, targetEl) {
    removePinnedTooltip();
    document.body.classList.add('has-pinned-tooltip');

    pinnedTooltip = document.createElement('div');
    pinnedTooltip.className = 'vis-tooltip pinned-tooltip';

    const date = new Date(app.event_date);
    const dateStr = date.toLocaleString('pt-BR', {
        weekday: 'short', day: '2-digit', month: '2-digit',
        year: 'numeric', hour: '2-digit', minute: '2-digit'
    });
    const remaining = getRemainingTimeText(date);
    const remainingHTML = remaining
        ? `<div class="tt-remaining" style="font-size:11px;color:#38bdf8;margin-top:4px;font-weight:600;">⏱️ ${remaining}</div>`
        : '';

    pinnedTooltip.innerHTML =
        `<div class="tt-desc">${formatTooltipDescription(app.description)}</div>` +
        `<div class="tt-date">📅 ${dateStr}</div>` +
        remainingHTML;

    document.body.appendChild(pinnedTooltip);

    const anchorEl  = targetEl.closest('.vis-item') || targetEl;
    const rect       = anchorEl.getBoundingClientRect();
    const tipRect    = pinnedTooltip.getBoundingClientRect();

    let top  = rect.top - tipRect.height - 8;
    if (top < 8) top = rect.bottom + 8;

    let left = rect.left + rect.width / 2 - tipRect.width / 2;
    if (left < 8) left = 8;
    if (left + tipRect.width > window.innerWidth - 8)
        left = window.innerWidth - tipRect.width - 8;

    pinnedTooltip.style.top  = `${top}px`;
    pinnedTooltip.style.left = `${left}px`;

    pinnedTooltip.addEventListener('click', (e) => e.stopPropagation());
    lastPinnedTime = Date.now();
}

/** Returns true if a click event should be ignored because a tooltip was just pinned. */
export function isRecentPin() {
    return Date.now() - lastPinnedTime < 100;
}

export function hasPinnedTooltip() {
    return pinnedTooltip !== null;
}

export function tooltipContains(el) {
    return pinnedTooltip && pinnedTooltip.contains(el);
}
