/**
 * date-utils.js
 * Date parsing helpers, Portuguese normalizer, and format functions.
 */

/**
 * Checks if a chrono-matched token appears as an isolated word (not inside another word).
 * Prevents false positives like 'dom' matching inside 'armagedom'.
 */
export function isChronoMatchIsolated(fullText, matchedText) {
    if (!matchedText) return false;
    const idx = fullText.indexOf(matchedText);
    if (idx === -1) return false;
    const before = idx > 0 ? fullText[idx - 1] : ' ';
    const after = idx + matchedText.length < fullText.length ? fullText[idx + matchedText.length] : ' ';
    const wordChar = /[\wáàâãéèêíìîóòôõúùûç]/i;
    return !wordChar.test(before) && !wordChar.test(after);
}

/** Formats a date as dd/mm/yyyy */
export function formatShortDate(date) {
    const dd = String(date.getDate()).padStart(2, '0');
    const mm = String(date.getMonth() + 1).padStart(2, '0');
    const yyyy = date.getFullYear();
    return `${dd}/${mm}/${yyyy}`;
}

/** Formats a date as dd/mm/yyyy às HH:MM */
export function formatFullDateTime(date) {
    const dd = String(date.getDate()).padStart(2, '0');
    const mm = String(date.getMonth() + 1).padStart(2, '0');
    const yyyy = date.getFullYear();
    const hh = String(date.getHours()).padStart(2, '0');
    const min = String(date.getMinutes()).padStart(2, '0');
    return `${dd}/${mm}/${yyyy} às ${hh}:${min}`;
}

/** Formats a date as dd/mm/yyyy, HH:MM:SS (for input preview) */
export function formatPreviewDate(date) {
    const dd = String(date.getDate()).padStart(2, '0');
    const mm = String(date.getMonth() + 1).padStart(2, '0');
    const yyyy = date.getFullYear();
    const hh = String(date.getHours()).padStart(2, '0');
    const min = String(date.getMinutes()).padStart(2, '0');
    const sec = String(date.getSeconds()).padStart(2, '0');
    return `${dd}/${mm}/${yyyy}, ${hh}:${min}:${sec}`;
}

/**
 * Normalizes Portuguese natural language date expressions to formats
 * that chrono-node can parse (e.g., "daqui a 3 dias" → "DD/MM/YYYY").
 */
export function normalizarPT(texto, nowOverride) {
    let res = texto;

    // 0. Normalizar AM/PM para 24h: "8 pm" -> "20:00", "8:30am" -> "08:30"
    res = res.replace(/(\b\d{1,2})(?::(\d{2}))?\s*(am|pm)\b/gi, (match, h, m, ampm) => {
        let hour = parseInt(h, 10);
        const min = m || '00';
        if (ampm.toLowerCase() === 'pm' && hour < 12) hour += 12;
        if (ampm.toLowerCase() === 'am' && hour === 12) hour = 0;
        return `${hour}:${min}`;
    });

    // 1. Normalizar notação de hora: "15h30" -> "15:30", "15h" -> "15:00"
    res = res.replace(/(\b\d{1,2})[hH](\d{2})?\b/g, (match, h, m) => {
        return h + ':' + (m || '00');
    });

    const now = nowOverride || new Date();

    // 2. Semanas relativas
    res = res.replace(/daqui a (uma?|\d+) semanas?/gi, (match, val) => {
        const n = (val.toLowerCase() === 'um' || val.toLowerCase() === 'uma') ? 1 : parseInt(val, 10);
        const d = new Date(now); d.setDate(d.getDate() + n * 7);
        return formatShortDate(d);
    });
    res = res.replace(/\bsemana que vem\b/gi, () => {
        const d = new Date(now); d.setDate(d.getDate() + 7);
        return formatShortDate(d);
    });

    // 3. Dias relativos
    res = res.replace(/daqui a (uma?|\d+) dias?/gi, (match, val) => {
        const n = (val.toLowerCase() === 'um' || val.toLowerCase() === 'uma') ? 1 : parseInt(val, 10);
        const d = new Date(now); d.setDate(d.getDate() + n);
        return formatShortDate(d);
    });

    // 4. Meses relativos
    res = res.replace(/daqui a (uma?|\d+) m[eê]s(es)?/gi, (match, val) => {
        const n = (val.toLowerCase() === 'um' || val.toLowerCase() === 'uma') ? 1 : parseInt(val, 10);
        const d = new Date(now); d.setMonth(d.getMonth() + n);
        return formatShortDate(d);
    });
    res = res.replace(/\bm[eê]s que vem\b/gi, () => {
        const d = new Date(now); d.setMonth(d.getMonth() + 1);
        return formatShortDate(d);
    });

    // 5. Anos relativos
    res = res.replace(/daqui a (uma?|\d+) anos?/gi, (match, val) => {
        const n = (val.toLowerCase() === 'um' || val.toLowerCase() === 'uma') ? 1 : parseInt(val, 10);
        const d = new Date(now); d.setFullYear(d.getFullYear() + n);
        return formatShortDate(d);
    });
    res = res.replace(/\bano que vem\b/gi, () => {
        const d = new Date(now); d.setFullYear(d.getFullYear() + 1);
        return formatShortDate(d);
    });

    // 6. Horas relativas
    res = res.replace(/daqui a (uma?|\d+) horas?/gi, (match, val) => {
        const n = (val.toLowerCase() === 'um' || val.toLowerCase() === 'uma') ? 1 : parseInt(val, 10);
        const d = new Date(now); d.setHours(d.getHours() + n);
        return formatFullDateTime(d);
    });

    // 7. Minutos relativos
    res = res.replace(/daqui a (uma?|\d+) minutos?/gi, (match, val) => {
        const n = (val.toLowerCase() === 'um' || val.toLowerCase() === 'uma') ? 1 : parseInt(val, 10);
        const d = new Date(now); d.setMinutes(d.getMinutes() + n);
        return formatFullDateTime(d);
    });

    return res;
}
