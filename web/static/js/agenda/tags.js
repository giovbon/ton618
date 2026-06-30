/**
 * tags.js
 * Tag color generation, HTML escaping, and tooltip description formatting.
 */

const TAG_COLORS = [
    { base: '#f43f5e', alpha: 'rgba(244, 63, 94, 0.2)' },   // rose
    { base: '#ec4899', alpha: 'rgba(236, 72, 153, 0.2)' },   // pink
    { base: '#d946ef', alpha: 'rgba(217, 70, 239, 0.2)' },   // fuchsia
    { base: '#a855f7', alpha: 'rgba(168, 85, 247, 0.2)' },   // purple
    { base: '#8b5cf6', alpha: 'rgba(139, 92, 246, 0.2)' },   // violet
    { base: '#6366f1', alpha: 'rgba(99, 102, 241, 0.2)' },   // indigo
    { base: '#14b8a6', alpha: 'rgba(20, 184, 166, 0.2)' },   // teal
    { base: '#10b981', alpha: 'rgba(16, 185, 129, 0.2)' },   // emerald
    { base: '#22c55e', alpha: 'rgba(34, 197, 94, 0.2)' },    // green
    { base: '#84cc16', alpha: 'rgba(132, 204, 22, 0.2)' },   // lime
    { base: '#eab308', alpha: 'rgba(234, 179, 8, 0.2)' },    // yellow
    { base: '#f59e0b', alpha: 'rgba(245, 158, 11, 0.2)' },   // amber
    { base: '#f97316', alpha: 'rgba(249, 115, 22, 0.2)' },   // orange
];

/** Returns a deterministic {base, alpha} color pair for a tag string. */
export function getTagColor(tag) {
    let hash = 0;
    const clean = tag.toLowerCase().replace(/#/g, '').trim();
    for (let i = 0; i < clean.length; i++) {
        hash = clean.charCodeAt(i) + ((hash << 5) - hash);
    }
    return TAG_COLORS[Math.abs(hash) % TAG_COLORS.length];
}

/** Escapes HTML special chars to safely inject text into innerHTML. */
export function escapeHtml(str) {
    return String(str)
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;')
        .replace(/"/g, '&quot;')
        .replace(/'/g, '&#39;');
}

/**
 * Formats a description for tooltip display:
 * strips wikilinks [[...]] and hashtags #tag, collapses whitespace.
 */
export function formatTooltipDescription(desc) {
    if (!desc) return '';
    let html = escapeHtml(desc);
    html = html.replace(/\[\[[^\]]+\]\]/g, '');
    html = html.replace(/(^|\s)#[\w\-]+/g, '');
    return html.replace(/\s+/g, ' ').trim();
}
