// Checks if a chrono-matched token appears as an isolated word (not inside another word).
// This prevents false positives like 'dom' matching inside 'armagedom'.
function isChronoMatchIsolated(fullText, matchedText) {
    if (!matchedText) return false;
    const idx = fullText.indexOf(matchedText);
    if (idx === -1) return false;
    // Check character before match (if exists) is not a word character
    const before = idx > 0 ? fullText[idx - 1] : ' ';
    // Check character after match (if exists) is not a word character
    const after = idx + matchedText.length < fullText.length ? fullText[idx + matchedText.length] : ' ';
    const wordChar = /[\wáàâãéèêíìîóòôõúùûç]/i;
    return !wordChar.test(before) && !wordChar.test(after);
}

function normalizarPT(texto, nowOverride) {
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

    // 2. Semanas relativas: "daqui a X semanas", "semana que vem", "daqui a uma semana"
    res = res.replace(/daqui a (uma?|\d+) semanas?/gi, (match, val) => {
        const n = (val.toLowerCase() === 'um' || val.toLowerCase() === 'uma') ? 1 : parseInt(val, 10);
        const d = new Date(now);
        d.setDate(d.getDate() + n * 7);
        return formatShortDate(d);
    });
    res = res.replace(/\bsemana que vem\b/gi, () => {
        const d = new Date(now);
        d.setDate(d.getDate() + 7);
        return formatShortDate(d);
    });

    // 3. Dias relativos: "daqui a X dias", "daqui a um dia"
    res = res.replace(/daqui a (uma?|\d+) dias?/gi, (match, val) => {
        const n = (val.toLowerCase() === 'um' || val.toLowerCase() === 'uma') ? 1 : parseInt(val, 10);
        const d = new Date(now);
        d.setDate(d.getDate() + n);
        return formatShortDate(d);
    });

    // 4. Meses relativos: "daqui a X meses", "daqui a um mês", "mês que vem"
    res = res.replace(/daqui a (uma?|\d+) m[eê]s(es)?/gi, (match, val) => {
        const n = (val.toLowerCase() === 'um' || val.toLowerCase() === 'uma') ? 1 : parseInt(val, 10);
        const d = new Date(now);
        d.setMonth(d.getMonth() + n);
        return formatShortDate(d);
    });
    res = res.replace(/\bm[eê]s que vem\b/gi, () => {
        const d = new Date(now);
        d.setMonth(d.getMonth() + 1);
        return formatShortDate(d);
    });

    // 5. Anos relativos: "daqui a X anos", "daqui a um ano", "ano que vem"
    res = res.replace(/daqui a (uma?|\d+) anos?/gi, (match, val) => {
        const n = (val.toLowerCase() === 'um' || val.toLowerCase() === 'uma') ? 1 : parseInt(val, 10);
        const d = new Date(now);
        d.setFullYear(d.getFullYear() + n);
        return formatShortDate(d);
    });
    res = res.replace(/\bano que vem\b/gi, () => {
        const d = new Date(now);
        d.setFullYear(d.getFullYear() + 1);
        return formatShortDate(d);
    });

    // 6. Horas relativas: "daqui a X horas"
    res = res.replace(/daqui a (uma?|\d+) horas?/gi, (match, val) => {
        const n = (val.toLowerCase() === 'um' || val.toLowerCase() === 'uma') ? 1 : parseInt(val, 10);
        const d = new Date(now);
        d.setHours(d.getHours() + n);
        return formatFullDateTime(d);
    });

    // 7. Minutos relativos: "daqui a X minutos"
    res = res.replace(/daqui a (uma?|\d+) minutos?/gi, (match, val) => {
        const n = (val.toLowerCase() === 'um' || val.toLowerCase() === 'uma') ? 1 : parseInt(val, 10);
        const d = new Date(now);
        d.setMinutes(d.getMinutes() + n);
        return formatFullDateTime(d);
    });

    return res;
}

function formatShortDate(date) {
    const dd = String(date.getDate()).padStart(2, '0');
    const mm = String(date.getMonth() + 1).padStart(2, '0');
    const yyyy = date.getFullYear();
    return `${dd}/${mm}/${yyyy}`;
}

function formatFullDateTime(date) {
    const dd = String(date.getDate()).padStart(2, '0');
    const mm = String(date.getMonth() + 1).padStart(2, '0');
    const yyyy = date.getFullYear();
    const hh = String(date.getHours()).padStart(2, '0');
    const min = String(date.getMinutes()).padStart(2, '0');
    return `${dd}/${mm}/${yyyy} às ${hh}:${min}`;
}

function formatPreviewDate(date) {
    const dd = String(date.getDate()).padStart(2, '0');
    const mm = String(date.getMonth() + 1).padStart(2, '0');
    const yyyy = date.getFullYear();
    const hh = String(date.getHours()).padStart(2, '0');
    const min = String(date.getMinutes()).padStart(2, '0');
    const sec = String(date.getSeconds()).padStart(2, '0');
    return `${dd}/${mm}/${yyyy}, ${hh}:${min}:${sec}`;
}

if (typeof document !== 'undefined') {
    document.addEventListener('DOMContentLoaded', async () => {
    const input = document.getElementById('agenda-input');
    const preview = document.getElementById('agenda-preview');
    const saveBtn = document.getElementById('agenda-save');
    const purgeBtn = document.getElementById('agenda-purge');
    const treeContainer = document.getElementById('agenda-tree');
    const timelineContainer = document.getElementById('agenda-timeline');
    const countBadge = document.getElementById('agenda-count');

    let timeline = null;
    let items = null;
    let appointments = [];
    let cachedDecorations = null; // Timeline background decorations (weekends + week numbers)
    let pinnedTooltip = null;
    
    const timezoneCoordinates = {
        'America/Sao_Paulo': { lat: -23.5505, lng: -46.6333 },
        'America/Manaus': { lat: -3.1190, lng: -60.0217 },
        'America/Noronha': { lat: -3.8548, lng: -32.4231 },
        'America/Belem': { lat: -1.4558, lng: -48.4902 },
        'America/Fortaleza': { lat: -3.7319, lng: -38.5267 },
        'America/New_York': { lat: 40.7128, lng: -74.0060 },
        'Europe/London': { lat: 51.5074, lng: -0.1278 },
        'Europe/Paris': { lat: 48.8566, lng: 2.3522 },
        'UTC': { lat: 0, lng: 0 }
    };

    const storedTz = localStorage.getItem('agenda-timezone') || 'America/Sao_Paulo';
    const coords = timezoneCoordinates[storedTz] || timezoneCoordinates['America/Sao_Paulo'];
    let lat = coords.lat;
    let lng = coords.lng;

    function initTimezoneSelect() {
        const select = document.getElementById('setting-agenda-timezone');
        if (select) {
            select.value = storedTz;
        }
    }
    initTimezoneSelect();

    window.saveTimezoneSetting = function(tz) {
        localStorage.setItem('agenda-timezone', tz);
        const newCoords = timezoneCoordinates[tz] || timezoneCoordinates['America/Sao_Paulo'];
        lat = newCoords.lat;
        lng = newCoords.lng;
        if (timeline) {
            const nightIds = items.get({ filter: (item) => item.className === 'night-shade' }).map(i => i.id);
            items.remove(nightIds);
            items.add(buildNightDecorations());
        }
    };
    
    // Dynamic tag styling sheet to override vis-timeline colors
    const tagStylesElement = document.createElement('style');
    tagStylesElement.id = 'agenda-tag-styles';
    document.head.appendChild(tagStylesElement);
    const registeredTags = new Set();

    function registerTagStyle(tag) {
        const clean = tag.toLowerCase().replace(/#/g, '').trim();
        const colors = getTagColor(clean);
        
        if (!registeredTags.has(clean)) {
            registeredTags.add(clean);
            const css = `
                .vis-timeline .vis-item.vis-dot.tag-item-${clean} {
                    background-color: ${colors.base} !important;
                    border-color: ${colors.base} !important;
                }
                .vis-timeline .vis-item.vis-dot.tag-item-${clean}:hover {
                    box-shadow: 0 0 8px ${colors.base} !important;
                }
            `;
            tagStylesElement.appendChild(document.createTextNode(css));
        }
        return `tag-item-${clean}`;
    }

    // Initialize Chrono (Portuguese) from local vendored script
    const chronoPt = typeof chrono !== 'undefined' ? chrono.pt : null;
    if (!chronoPt) {
        console.error('Failed to load chrono-node Portuguese parser.');
        preview.textContent = 'Erro: Analisador de data não disponível offline.';
        return;
    }



    // Keep the date preview listener on the main input
    input.addEventListener('input', (e) => {
        const text = e.target.value;
        if (!text.trim()) {
            preview.textContent = '';
            return;
        }

        const normalizedText = normalizarPT(text);
        const results = chronoPt.parse(normalizedText, new Date(), { forwardDate: true });
        // Use normalizedText (not raw text) because r.text comes from the normalized string
        const isolated = results.filter(r => isChronoMatchIsolated(normalizedText, r.text));
        if (isolated.length > 0) {
            const date = isolated[0].start.date();
            preview.textContent = `Data reconhecida: ${formatPreviewDate(date)}`;
            preview.className = 'text-xs text-emerald-400 mt-2 h-4';
        } else {
            preview.textContent = 'Nenhuma data reconhecida...';
            preview.className = 'text-xs text-zinc-500 mt-2 h-4';
        }
    });

    // Initialize Autocomplete from external module
    if (window.setupAutocomplete) {
        window.setupAutocomplete(input);
    }

    // Save appointment
    saveBtn.addEventListener('click', async () => {
        const text = input.value.trim();
        if (!text) return;

        const normalizedText = normalizarPT(text);
        const rawResults = chronoPt.parse(normalizedText, new Date(), { forwardDate: true });
        // Filter results to only accept date tokens that appear as isolated words,
        // preventing substring matches like 'dom' inside 'armagedom'
        const results = rawResults.filter(r => isChronoMatchIsolated(normalizedText, r.text));
        let eventDate = new Date();
        let description = normalizedText;

        if (results && results.length > 0) {
            eventDate = results[0].start.date();
            // Remove the parsed date string from description
            description = normalizedText.replace(results[0].text, '').trim();
            if (!description) description = "Compromisso (" + results[0].text + ")";
        }

        // Calculate ISO week
        const d = new Date(eventDate);
        d.setHours(0, 0, 0, 0);
        d.setDate(d.getDate() + 3 - (d.getDay() + 6) % 7);
        const week1 = new Date(d.getFullYear(), 0, 4);
        const weekNum = 1 + Math.round(((d.getTime() - week1.getTime()) / 86400000 - 3 + (week1.getDay() + 6) % 7) / 7);

        const pad = (n) => n.toString().padStart(2, '0');
        const localFloatingTime = `${eventDate.getFullYear()}-${pad(eventDate.getMonth() + 1)}-${pad(eventDate.getDate())}T${pad(eventDate.getHours())}:${pad(eventDate.getMinutes())}:${pad(eventDate.getSeconds())}`;

        const data = {
            description: description,
            event_date: localFloatingTime,
            year: eventDate.getFullYear(),
            month: eventDate.getMonth() + 1,
            week_number: weekNum
        };

        saveBtn.disabled = true;
        saveBtn.textContent = 'Salvando...';

        try {
            const res = await fetch('/api/appointments/create', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(data)
            });

            if (res.ok) {
                input.value = '';
                preview.textContent = '';
                await loadAppointments();
            } else {
                alert('Erro ao salvar apontamento.');
            }
        } catch (e) {
            console.error(e);
            alert('Erro de conexão ao salvar.');
        } finally {
            saveBtn.disabled = false;
            saveBtn.textContent = 'Salvar';
        }
    });

    // Purge appointments older than 7 days
    purgeBtn.addEventListener('click', async () => {
        const cutoff = new Date();
        cutoff.setDate(cutoff.getDate() - 7);
        const older = appointments.filter(a => new Date(a.event_date) < cutoff);
        if (older.length === 0) {
            alert('Nenhum compromisso com mais de 7 dias encontrado.');
            return;
        }
        if (!confirm(`Excluir ${older.length} compromisso(s) com data anterior a ${cutoff.toLocaleDateString('pt-BR')}?`)) return;
        purgeBtn.disabled = true;
        purgeBtn.textContent = 'Limpando...';
        try {
            const res = await fetch('/api/appointments/purge-old', { method: 'DELETE' });
            if (res.ok) {
                await loadAppointments();
            } else {
                alert('Erro ao limpar compromissos antigos.');
            }
        } catch(e) {
            console.error(e);
            alert('Erro de conexão.');
        } finally {
            purgeBtn.disabled = false;
            purgeBtn.innerHTML = '🗑️ Limpar anteriores';
        }
    });

    // Load and render timeline
    async function loadAppointments() {
        try {
            const res = await fetch('/api/appointments');
            if (res.ok) {
                appointments = await res.json() || [];
                renderTimeline();
                document.body.dispatchEvent(new Event("agenda-updated"));
                if (countBadge) countBadge.textContent = appointments.length;
            }
        } catch (e) {
            console.error("Erro ao carregar agenda", e);
        }
    }

    // Delete appointment
    window.deleteAppointment = async (id) => {
        if (!confirm('Deseja excluir este apontamento?')) return;
        try {
            const res = await fetch(`/api/appointments/delete?id=${id}`, { method: 'DELETE' });
            if (res.ok) {
                await loadAppointments();
            }
        } catch (e) {
            console.error(e);
        }
    };

    // Update appointment description inline
    window.startEdit = (id, currentDesc) => {
        const span = document.getElementById(`desc-${id}`);
        if (!span) return;
        const input = document.createElement('input');
        input.type = 'text';
        input.value = currentDesc;
        input.className = 'bg-transparent border-b border-sky-500 text-sm font-semibold text-zinc-100 outline-none flex-1 w-full';
        span.replaceWith(input);
        
        // Activating autocomplete for this inline edit input field
        setupAutocomplete(input);
        
        input.focus();
        input.select();

        const commit = async () => {
            const newDesc = input.value.trim();
            if (!newDesc || newDesc === currentDesc) {
                await loadAppointments();
                return;
            }
            const app = appointments.find(a => a.id === id);
            if (!app) return;
            try {
                await fetch('/api/appointments/update', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ ...app, description: newDesc })
                });
            } catch(e) { console.error(e); }
            await loadAppointments();
        };

        input.addEventListener('blur', commit);
        input.addEventListener('keydown', (e) => {
            if (e.key === 'Enter') { 
                if (window.isAutocompleteVisible && window.isAutocompleteVisible()) {
                    return; // Let setupAutocomplete handle selecting the note
                }
                e.preventDefault(); 
                input.blur(); 
            }
            if (e.key === 'Escape') { 
                if (window.isAutocompleteVisible && window.isAutocompleteVisible()) {
                    return; // Let setupAutocomplete close the autocomplete dropdown
                }
                input.value = currentDesc; 
                input.blur(); 
            }
        });
    };

    function getTagColor(tag) {
        const colors = [
            { base: '#f43f5e', alpha: 'rgba(244, 63, 94, 0.2)' }, // rose
            { base: '#ec4899', alpha: 'rgba(236, 72, 153, 0.2)' }, // pink
            { base: '#d946ef', alpha: 'rgba(217, 70, 239, 0.2)' }, // fuchsia
            { base: '#a855f7', alpha: 'rgba(168, 85, 247, 0.2)' }, // purple
            { base: '#8b5cf6', alpha: 'rgba(139, 92, 246, 0.2)' }, // violet
            { base: '#6366f1', alpha: 'rgba(99, 102, 241, 0.2)' }, // indigo
            { base: '#14b8a6', alpha: 'rgba(20, 184, 166, 0.2)' }, // teal
            { base: '#10b981', alpha: 'rgba(16, 185, 129, 0.2)' }, // emerald
            { base: '#22c55e', alpha: 'rgba(34, 197, 94, 0.2)' },  // green
            { base: '#84cc16', alpha: 'rgba(132, 204, 22, 0.2)' }, // lime
            { base: '#eab308', alpha: 'rgba(234, 179, 8, 0.2)' },  // yellow
            { base: '#f59e0b', alpha: 'rgba(245, 158, 11, 0.2)' }, // amber
            { base: '#f97316', alpha: 'rgba(249, 115, 22, 0.2)' }  // orange
        ];

        let hash = 0;
        const clean = tag.toLowerCase().replace(/#/g, '').trim();
        for (let i = 0; i < clean.length; i++) {
            hash = clean.charCodeAt(i) + ((hash << 5) - hash);
        }
        hash = Math.abs(hash);
        return colors[hash % colors.length];
    }

    // Format description text specifically for tooltips: stripping tags and wikilinks completely
    function formatTooltipDescription(desc) {
        if (!desc) return '';
        
        // Escape HTML to prevent XSS
        let html = escapeHtml(desc);
        
        // Remove wikilinks entirely: [[Note Name]] or [[Note Name|Alias]]
        html = html.replace(/\[\[[^\]]+\]\]/g, '');
        
        // Remove tags entirely: #tag
        html = html.replace(/(^|\s)#[\w\-]+/g, '');
        
        // Collapse spaces and trim
        return html.replace(/\s+/g, ' ').trim();
    }

    // Escape HTML special chars to safely inject text into innerHTML
    function escapeHtml(str) {
        return String(str).replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;').replace(/'/g,'&#39;');
    }

    // Event delegation for HTMX rendered tree
    treeContainer.addEventListener('click', (e) => {
        const editBtn = e.target.closest('.edit-btn');
        if (editBtn) {
            const id = editBtn.dataset.id;
            const desc = editBtn.dataset.desc;
            if (id && desc) startEdit(id, desc);
        }
        
        const delBtn = e.target.closest('.del-btn');
        if (delBtn) {
            const id = delBtn.dataset.id;
            if (id) deleteAppointment(id);
        }
    });

    // Returns ISO week number for a given date
    function getISOWeek(date) {
        const d = new Date(date);
        d.setHours(0, 0, 0, 0);
        d.setDate(d.getDate() + 3 - (d.getDay() + 6) % 7);
        const week1 = new Date(d.getFullYear(), 0, 4);
        return 1 + Math.round(((d.getTime() - week1.getTime()) / 86400000 - 3 + (week1.getDay() + 6) % 7) / 7);
    }

    let holidaysList = [];

    async function loadHolidays() {
        const startYear = new Date().getFullYear() - 1;
        const endYear = new Date().getFullYear() + 2;
        const fetchedHolidays = [];

        for (let y = startYear; y <= endYear; y++) {
            const cacheKey = `agenda-holidays-${y}`;
            const cached = localStorage.getItem(cacheKey);
            if (cached) {
                try {
                    fetchedHolidays.push(...JSON.parse(cached));
                    continue;
                } catch (e) {
                    console.error("Error parsing cached holidays", e);
                }
            }

            try {
                const res = await fetch(`https://brasilapi.com.br/api/feriados/v1/${y}`);
                if (res.ok) {
                    const data = await res.json();
                    localStorage.setItem(cacheKey, JSON.stringify(data));
                    fetchedHolidays.push(...data);
                }
            } catch (e) {
                console.error(`Failed to fetch holidays for year ${y}`, e);
            }
        }

        holidaysList = fetchedHolidays;
        renderTimeline();
    }

    let lastPinnedTime = 0;

    function getRemainingTimeText(eventDate) {
        const now = new Date();
        const diffMs = eventDate - now;
        if (diffMs <= 0) return '';

        const diffSecs = Math.floor(diffMs / 1000);
        const diffMins = Math.floor(diffSecs / 60);
        const diffHours = Math.floor(diffMins / 60);
        const diffDays = Math.floor(diffHours / 24);

        const rHours = diffHours % 24;
        const rMins = diffMins % 60;

        let parts = [];
        if (diffDays > 0) parts.push(`${diffDays}d`);
        if (rHours > 0 || diffDays > 0) parts.push(`${rHours}h`);
        parts.push(`${rMins}m`);

        return `em ${parts.join(' ')}`;
    }

    function removePinnedTooltip() {
        if (pinnedTooltip) {
            pinnedTooltip.remove();
            pinnedTooltip = null;
            document.body.classList.remove('has-pinned-tooltip');
        }
    }

    function showPinnedTooltip(app, targetEl) {
        removePinnedTooltip();
        document.body.classList.add('has-pinned-tooltip');
        
        pinnedTooltip = document.createElement('div');
        pinnedTooltip.className = 'vis-tooltip pinned-tooltip';
        
        const date = new Date(app.event_date);
        const dateStr = date.toLocaleString('pt-BR', { weekday: 'short', day: '2-digit', month: '2-digit', year: 'numeric', hour: '2-digit', minute: '2-digit' });
        const remaining = getRemainingTimeText(date);
        const remainingHTML = remaining ? `<div class="tt-remaining" style="font-size: 11px; color: #38bdf8; margin-top: 4px; font-weight: 600;">⏱️ ${remaining}</div>` : '';
        
        pinnedTooltip.innerHTML = `<div class="tt-desc">${formatTooltipDescription(app.description)}</div><div class="tt-date">📅 ${dateStr}</div>${remainingHTML}`;
        
        document.body.appendChild(pinnedTooltip);
        
        const anchorEl = targetEl.closest('.vis-item') || targetEl;
        const rect = anchorEl.getBoundingClientRect();
        const tooltipRect = pinnedTooltip.getBoundingClientRect();
        
        let top = rect.top - tooltipRect.height - 8;
        if (top < 8) {
            top = rect.bottom + 8;
        }
        
        let left = rect.left + (rect.width / 2) - (tooltipRect.width / 2);
        if (left < 8) left = 8;
        if (left + tooltipRect.width > window.innerWidth - 8) {
            left = window.innerWidth - tooltipRect.width - 8;
        }
        
        pinnedTooltip.style.top = `${top}px`;
        pinnedTooltip.style.left = `${left}px`;
        
        pinnedTooltip.addEventListener('click', (e) => {
            e.stopPropagation();
        });
        
        lastPinnedTime = Date.now();
    }

    document.addEventListener('click', (e) => {
        if (Date.now() - lastPinnedTime < 100) {
            return;
        }
        if (pinnedTooltip && !pinnedTooltip.contains(e.target)) {
            removePinnedTooltip();
        }
    });

    // Render custom timeline from scratch
    function renderTimeline() {
        timelineContainer.innerHTML = '';
        timelineContainer.style.overflow = 'hidden';

        // Use the explicit pixel height of the container so that
        // absolute children with top-0/bottom-0 have a valid reference.
        const tlHeight = timelineContainer.offsetHeight || 144;

        const container = document.createElement('div');
        container.className = 'custom-timeline-scroll w-full overflow-x-auto overflow-y-hidden select-none cursor-grab active:cursor-grabbing';
        container.style.cssText = `position: relative; scroll-behavior: auto; height: ${tlHeight}px;`;
        
        const canvas = document.createElement('div');
        canvas.className = 'custom-timeline-canvas bg-zinc-950';
        canvas.style.cssText = `position: relative; height: ${tlHeight}px;`;
        container.appendChild(canvas);

        const now = new Date();
        const startDate = new Date(now.getFullYear(), now.getMonth() - 3, now.getDate(), 0, 0, 0);
        const endDate = new Date(now.getFullYear(), now.getMonth() + 9, now.getDate(), 23, 59, 59);
        const totalMs = endDate - startDate;

        let msPerPixel = (24 * 60 * 60 * 1000) / 140; // Default: 1 day = 140px

        const weekdaysPt = ['dom', 'seg', 'ter', 'qua', 'qui', 'sex', 'sáb'];
        const monthsPtShort = ['jan', 'fev', 'mar', 'abr', 'mai', 'jun', 'jul', 'ago', 'set', 'out', 'nov', 'dez'];

        let redLineInterval;

        function draw() {
            canvas.innerHTML = '';
            const totalWidth = totalMs / msPerPixel;
            canvas.style.width = `${totalWidth}px`;

            const scaleDaysPx = (24 * 60 * 60 * 1000) / msPerPixel;
            let scale = 'days';
            if (scaleDaysPx < 10) scale = 'months';
            else if (scaleDaysPx < 50) scale = 'weeks';

            // 1. Render Night Shades (Drawn first/behind)
            if (typeof SunCalc !== 'undefined' && scale === 'days') {
                let shadeStart = new Date(startDate);
                while (shadeStart <= endDate) {
                    const times = SunCalc.getTimes(shadeStart, lat, lng);
                    const sunset = times.sunset;
                    
                    const nextDay = new Date(shadeStart.getTime() + 24 * 60 * 60 * 1000);
                    const nextTimes = SunCalc.getTimes(nextDay, lat, lng);
                    const nextSunrise = nextTimes.sunrise;
                    
                    if (sunset && nextSunrise && sunset >= startDate && nextSunrise <= endDate) {
                        const xStart = (sunset - startDate) / msPerPixel;
                        const xEnd = (nextSunrise - startDate) / msPerPixel;
                        const nightShade = document.createElement('div');
                        nightShade.style.cssText = `position: absolute; top: 0; bottom: 0; left: ${xStart}px; width: ${xEnd - xStart}px; background: linear-gradient(180deg, rgba(13, 20, 38, 0.72) 0%, rgba(9, 9, 11, 0.45) 100%); pointer-events: none;`;
                        canvas.appendChild(nightShade);
                    }
                    shadeStart.setDate(shadeStart.getDate() + 1);
                }
            }

            // 2. Draw Axis (Drawn on top of night shades)
            let cur = new Date(startDate);
            let lastMonthStr = '';

            while (cur <= endDate) {
                const colMs = cur.getTime() - startDate.getTime();
                const colLeft = colMs / msPerPixel;

                let nextDate = new Date(cur);
                if (scale === 'months') {
                    nextDate.setMonth(nextDate.getMonth() + 1);
                    nextDate.setDate(1);
                    if (cur.getDate() !== 1) nextDate = new Date(cur.getFullYear(), cur.getMonth() + 1, 1);
                } else if (scale === 'weeks') {
                    nextDate.setDate(nextDate.getDate() + 7);
                    const dayNr = (nextDate.getDay() + 6) % 7;
                    nextDate.setDate(nextDate.getDate() - dayNr);
                    if (cur.getDay() !== 1) nextDate = new Date(cur.getFullYear(), cur.getMonth(), cur.getDate() + (8 - (cur.getDay()||7)));
                } else {
                    nextDate.setDate(nextDate.getDate() + 1);
                    nextDate.setHours(0,0,0,0);
                }

                if (nextDate > endDate) nextDate = new Date(endDate);
                if (nextDate <= cur) break;

                const nextMs = nextDate.getTime() - startDate.getTime();
                const colWidth = (nextMs - colMs) / msPerPixel;

                const col = document.createElement('div');
                col.style.cssText = `position: absolute; top: 0; bottom: 0; left: ${colLeft}px; width: ${colWidth}px; border-left: 1px solid rgba(63,63,70,0.5);`;

                const monthStr = `${monthsPtShort[cur.getMonth()]} ${cur.getFullYear()}`;
                
                if (scale === 'months') {
                    const monthLabel = document.createElement('div');
                    monthLabel.style.cssText = 'position: absolute; left: 10px; top: 20px; font-size: 13px; font-weight: 700; text-transform: uppercase; letter-spacing: 0.05em; color: #f4f4f5; z-index: 10;';
                    monthLabel.textContent = monthStr;
                    col.appendChild(monthLabel);
                } else if (scale === 'weeks') {
                    if (monthStr !== lastMonthStr) {
                        lastMonthStr = monthStr;
                        const monthLabel = document.createElement('div');
                        monthLabel.style.cssText = 'position: absolute; left: 10px; top: 6px; font-size: 9px; font-weight: 700; text-transform: uppercase; letter-spacing: 0.05em; color: #71717a; z-index: 10;';
                        monthLabel.textContent = monthStr;
                        col.appendChild(monthLabel);
                    }
                    const weekLabel = document.createElement('div');
                    weekLabel.style.cssText = 'position: absolute; left: 10px; top: 20px; font-size: 11px; font-family: monospace; font-weight: 700; color: #d4d4d8; z-index: 10;';
                    weekLabel.textContent = `Semana ${cur.getDate()} ${monthsPtShort[cur.getMonth()]}`;
                    col.appendChild(weekLabel);
                } else {
                    const dayOfWeek = cur.getDay();
                    if (dayOfWeek === 0 || dayOfWeek === 6) {
                        col.style.background = 'repeating-linear-gradient(45deg, rgba(63, 63, 70, 0.18), rgba(63, 63, 70, 0.18) 6px, transparent 6px, transparent 12px)';
                    }
                    if (monthStr !== lastMonthStr || colLeft === 0) {
                        lastMonthStr = monthStr;
                        const monthLabel = document.createElement('div');
                        monthLabel.style.cssText = 'position: absolute; left: 10px; top: 6px; font-size: 9px; font-weight: 700; text-transform: uppercase; letter-spacing: 0.05em; color: #71717a; z-index: 10;';
                        monthLabel.textContent = monthStr;
                        col.appendChild(monthLabel);
                    }
                    const isToday = cur.getDate() === now.getDate() && cur.getMonth() === now.getMonth() && cur.getFullYear() === now.getFullYear();
                    const dayLabel = document.createElement('div');
                    dayLabel.style.cssText = `position: absolute; left: 10px; top: 20px; font-size: 13px; font-family: monospace; font-weight: ${isToday ? '700' : '400'}; color: ${isToday ? '#f4f4f5' : '#a1a1aa'}; z-index: 10;`;
                    dayLabel.textContent = `${weekdaysPt[dayOfWeek]} ${cur.getDate().toString().padStart(2, '0')}`;
                    col.appendChild(dayLabel);
                }
                
                canvas.appendChild(col);
                cur = nextDate;
            }

            // 3. Render Holidays (Drawn on top of night shades/axis)
            holidaysList.forEach((h) => {
                const hDate = new Date(h.date + 'T12:00:00');
                if (hDate >= startDate && hDate <= endDate) {
                    const msDiff = hDate.getTime() - startDate.getTime();
                    const hLeft = msDiff / msPerPixel;
                    const hWidth = (24 * 60 * 60 * 1000) / msPerPixel;

                    const hCol = document.createElement('div');
                    hCol.style.cssText = `position: absolute; top: 0; bottom: 0; left: ${hLeft - hWidth/2}px; width: ${hWidth}px; background: rgba(244, 63, 94, 0.04); border-left: 1px solid rgba(159,18,57,0.3); border-right: 1px solid rgba(159,18,57,0.3); pointer-events: none;`;


                    const hLabel = document.createElement('div');
                    hLabel.style.cssText = 'position: absolute; left: 10px; bottom: 22px; font-size: 8px; font-weight: 700; color: rgba(251,113,133,0.85); width: calc(100% - 20px); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; pointer-events: none; z-index: 10;';
                    hLabel.textContent = h.name;
                    hCol.appendChild(hLabel);
                    canvas.appendChild(hCol);
                }
            });

            // 4. Current Time Line
            const updateRedLine = () => {
                const nowTime = new Date();
                if (nowTime >= startDate && nowTime <= endDate) {
                    const xNow = (nowTime - startDate) / msPerPixel;
                    let redLine = canvas.querySelector('.timeline-current-time');
                    if (!redLine) {
                        redLine = document.createElement('div');
                        redLine.className = 'timeline-current-time';
                        redLine.style.cssText = 'position: absolute; top: 0; bottom: 0; width: 1.5px; background-color: #ef4444; z-index: 10; pointer-events: none;';
                        canvas.appendChild(redLine);
                    }
                    redLine.style.left = `${xNow}px`;
                }
            };
            updateRedLine();
            if (redLineInterval) clearInterval(redLineInterval);
            redLineInterval = setInterval(updateRedLine, 60000);

            // 5. Appointments
            const sortedApps = [...appointments].sort((a, b) => a.event_date.localeCompare(b.event_date));
            const tracks = [0, 0, 0];

            sortedApps.forEach((app) => {
                const appDate = new Date(app.event_date);
                if (appDate >= startDate && appDate <= endDate) {
                    const xApp = (appDate - startDate) / msPerPixel;
                    
                    let trackIndex = 0;
                    for (let t = 0; t < tracks.length; t++) {
                        if (xApp > tracks[t] + 16) {
                            trackIndex = t;
                            break;
                        }
                        if (t === tracks.length - 1) {
                            let minTrack = 0;
                            for (let j = 1; j < tracks.length; j++) {
                                if (tracks[j] < tracks[minTrack]) minTrack = j;
                            }
                            trackIndex = minTrack;
                        }
                    }
                    tracks[trackIndex] = xApp;
                    
                    const dotY = 54 + trackIndex * 18;
                    
                    const dot = document.createElement('div');
                    dot.className = 'absolute rounded-full cursor-pointer transition-all duration-150 z-20 hover:scale-[1.6]';
                    dot.style.cssText = `left: ${xApp}px; top: ${dotY}px; width: 9px; height: 9px; transform: translate(-50%, -50%);`;
                    
                    const tags = app.description.match(/#[\w\-]+/g) || [];
                    let dotColor = '#38bdf8';
                    if (tags.length > 0) {
                        const colors = getTagColor(tags[0]);
                        dotColor = colors.base;
                    }
                    dot.style.backgroundColor = dotColor;
                    dot.style.boxShadow = `0 0 0 2px ${dotColor}40`;
                    
                    dot.addEventListener('mouseenter', () => dot.style.boxShadow = `0 0 10px ${dotColor}`);
                    dot.addEventListener('mouseleave', () => dot.style.boxShadow = `0 0 0 2px ${dotColor}40`);
                    
                    dot.addEventListener('click', (e) => {
                        e.stopPropagation();
                        removePinnedTooltip();
                        showPinnedTooltip(app, dot);
                    });
                    
                    canvas.appendChild(dot);
                }
            });
        }

        draw();
        timelineContainer.appendChild(container);

        // Initial scroll: show one day before today at the left edge
        const yesterday = new Date(now.getFullYear(), now.getMonth(), now.getDate() - 1, 0, 0, 0);
        const xYesterday = (yesterday - startDate) / msPerPixel;
        container.scrollLeft = xYesterday;

        // Allow drawing initially before DOM is fully settled
        setTimeout(() => {
            container.scrollLeft = xYesterday;
        }, 50);

        // Zoom Logic
        container.addEventListener('wheel', (e) => {
            e.preventDefault();
            
            const mouseX = e.pageX - container.getBoundingClientRect().left;
            const timeAtMouse = startDate.getTime() + (container.scrollLeft + mouseX) * msPerPixel;

            const zoomFactor = e.deltaY > 0 ? 1.2 : 0.8;
            let newMsPerPixel = msPerPixel * zoomFactor;

            // Constrain zoom
            const maxMsPerPixel = (30 * 24 * 60 * 60 * 1000) / 100; // max zoomed out: 1 month = 100px
            const minMsPerPixel = (60 * 60 * 1000) / 100; // max zoomed in: 1 hour = 100px

            if (newMsPerPixel > maxMsPerPixel) newMsPerPixel = maxMsPerPixel;
            if (newMsPerPixel < minMsPerPixel) newMsPerPixel = minMsPerPixel;

            if (Math.abs(newMsPerPixel - msPerPixel) > 1000) {
                msPerPixel = newMsPerPixel;
                draw();
                
                const newScrollLeft = (timeAtMouse - startDate.getTime()) / msPerPixel - mouseX;
                container.scrollLeft = newScrollLeft;
            }
        }, { passive: false });

        // Drag to scroll
        let isDown = false;
        let startX;
        let scrollLeft;
        
        container.addEventListener('mousedown', (e) => {
            isDown = true;
            startX = e.pageX - container.offsetLeft;
            scrollLeft = container.scrollLeft;
            container.style.cursor = 'grabbing';
        });
        container.addEventListener('mouseleave', () => {
            isDown = false;
            container.style.cursor = 'grab';
        });
        container.addEventListener('mouseup', () => {
            isDown = false;
            container.style.cursor = 'grab';
        });
        container.addEventListener('mousemove', (e) => {
            if (!isDown) return;
            e.preventDefault();
            const x = e.pageX - container.offsetLeft;
            const walk = (x - startX) * 1.5;
            container.scrollLeft = scrollLeft - walk;
        });
    }
    loadAppointments();
    loadHolidays();
    });
}

if (typeof window !== 'undefined') {
    window.normalizarPT = normalizarPT;
    window.isChronoMatchIsolated = isChronoMatchIsolated;
} else if (typeof global !== 'undefined') {
    global.normalizarPT = normalizarPT;
    global.isChronoMatchIsolated = isChronoMatchIsolated;
}

