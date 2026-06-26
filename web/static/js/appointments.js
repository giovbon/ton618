document.addEventListener('DOMContentLoaded', async () => {
    const input = document.getElementById('agenda-input');
    const preview = document.getElementById('agenda-preview');
    const saveBtn = document.getElementById('agenda-save');
    const purgeBtn = document.getElementById('agenda-purge');
    const treeContainer = document.getElementById('agenda-tree');
    const timelineContainer = document.getElementById('agenda-timeline');
    const countBadge = document.getElementById('agenda-count');

    let timeline = null;
    let items = new vis.DataSet();
    let appointments = [];
    let cachedDecorations = null; // Timeline background decorations (weekends + week numbers)
    
    // Wiki note autocomplete state variables
    let availableNotes = [];
    let filteredNotes = [];
    let autocompleteVisible = false;
    let selectedIndex = 0;
    const autocompleteContainer = document.getElementById('agenda-autocomplete');
    let activeInput = null;

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

    // Initialize Chrono (Portuguese)
    let chronoPt;
    if (typeof chrono !== 'undefined' && chrono.pt) {
        chronoPt = chrono.pt;
    } else {
        try {
            const chronoModule = await import('https://esm.sh/chrono-node@2.7.5');
            chronoPt = chronoModule.pt;
        } catch (err) {
            console.error('Failed to load chrono-node from CDN', err);
            preview.textContent = 'Erro: Não foi possível carregar o analisador de data.';
            return;
        }
    }

    function normalizarPT(texto) {
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

        const now = new Date();

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

    async function fetchNotesForAutocomplete() {
        if (availableNotes.length > 0) return;
        try {
            const res = await fetch('/api/notes');
            if (res.ok) {
                const data = await res.json();
                availableNotes = (data.notes || []).map(n => {
                    const filename = n.arquivo.split('/').pop() || n.arquivo;
                    return filename.replace(/\.md$/i, '');
                });
            }
        } catch (e) {
            console.error("Erro ao carregar notas para autocomplete", e);
        }
    }

    function positionAutocomplete(targetInput) {
        const rect = targetInput.getBoundingClientRect();
        autocompleteContainer.style.top = `${rect.bottom + 4}px`;
        autocompleteContainer.style.left = `${rect.left}px`;
        autocompleteContainer.style.width = `${rect.width}px`;
    }

    function showAutocomplete(query) {
        const q = query.toLowerCase().trim();
        filteredNotes = availableNotes.filter(name => name.toLowerCase().includes(q));
        
        if (filteredNotes.length === 0) {
            hideAutocomplete();
            return;
        }
        
        selectedIndex = Math.min(selectedIndex, filteredNotes.length - 1);
        if (selectedIndex < 0) selectedIndex = 0;
        
        autocompleteContainer.innerHTML = '';
        filteredNotes.forEach((name, idx) => {
            const btn = document.createElement('button');
            btn.className = `w-full text-left px-3 py-2 text-[13px] text-zinc-300 hover:bg-zinc-800 rounded flex items-center gap-2 transition-colors ${idx === selectedIndex ? 'bg-zinc-800 text-white font-medium' : ''}`;
            btn.innerHTML = `<span class="text-sky-400 text-[11px] font-bold shrink-0">[[]]</span><span class="truncate">${escapeHtml(name)}</span>`;
            
            btn.addEventListener('mousedown', (e) => {
                e.preventDefault();
                e.stopPropagation();
            });
            btn.addEventListener('click', (e) => {
                e.preventDefault();
                e.stopPropagation();
                selectNote(name);
            });
            autocompleteContainer.appendChild(btn);
        });
        
        autocompleteContainer.classList.remove('hidden');
        autocompleteVisible = true;
    }
    
    function hideAutocomplete() {
        autocompleteContainer.classList.add('hidden');
        autocompleteContainer.innerHTML = '';
        autocompleteVisible = false;
    }
    
    function selectNote(name) {
        if (!activeInput) return;
        const text = activeInput.value;
        const cursor = activeInput.selectionStart;
        const textBefore = text.substring(0, cursor);
        const bracketPos = textBefore.lastIndexOf('[[');
        if (bracketPos !== -1) {
            const textAfter = text.substring(cursor);
            const newValue = textBefore.substring(0, bracketPos) + `[[${name}]] ` + textAfter;
            activeInput.value = newValue;
            const newCursorPos = bracketPos + name.length + 5;
            activeInput.setSelectionRange(newCursorPos, newCursorPos);
        }
        hideAutocomplete();
        activeInput.focus();
        
        // Trigger input event to re-evaluate date preview after selecting a note
        activeInput.dispatchEvent(new Event('input'));
    }

    function updateAutocompleteSelection() {
        const buttons = autocompleteContainer.querySelectorAll('button');
        buttons.forEach((btn, idx) => {
            if (idx === selectedIndex) {
                btn.classList.add('bg-zinc-800', 'text-white', 'font-medium');
                btn.scrollIntoView({ block: 'nearest' });
            } else {
                btn.classList.remove('bg-zinc-800', 'text-white', 'font-medium');
            }
        });
    }

    function setupAutocomplete(targetInput) {
        targetInput.addEventListener('input', async (e) => {
            activeInput = targetInput;
            const text = targetInput.value;
            if (!text.trim()) {
                hideAutocomplete();
                return;
            }

            // Handle Wiki Link autocomplete detection
            const cursor = targetInput.selectionStart;
            const textBefore = text.substring(0, cursor);
            const bracketPos = textBefore.lastIndexOf('[[');
            
            if (bracketPos !== -1) {
                const textAfterBracket = textBefore.substring(bracketPos + 2);
                if (!textAfterBracket.includes(']]')) {
                    await fetchNotesForAutocomplete();
                    positionAutocomplete(targetInput);
                    showAutocomplete(textAfterBracket);
                } else {
                    hideAutocomplete();
                }
            } else {
                hideAutocomplete();
            }
        });

        targetInput.addEventListener('keydown', (e) => {
            if (!autocompleteVisible || activeInput !== targetInput) return;
            
            if (e.key === 'ArrowDown') {
                e.preventDefault();
                selectedIndex = (selectedIndex + 1) % filteredNotes.length;
                updateAutocompleteSelection();
            } else if (e.key === 'ArrowUp') {
                e.preventDefault();
                selectedIndex = (selectedIndex - 1 + filteredNotes.length) % filteredNotes.length;
                updateAutocompleteSelection();
            } else if (e.key === 'Enter' || e.key === 'Tab') {
                e.preventDefault();
                if (filteredNotes[selectedIndex]) {
                    selectNote(filteredNotes[selectedIndex]);
                }
            } else if (e.key === 'Escape') {
                e.preventDefault();
                hideAutocomplete();
            }
        });
    }

    // Set up autocomplete on the main input field
    setupAutocomplete(input);

    // Keep the date preview listener on the main input
    input.addEventListener('input', (e) => {
        const text = e.target.value;
        if (!text.trim()) {
            preview.textContent = '';
            return;
        }

        const results = chronoPt.parse(normalizarPT(text), new Date(), { forwardDate: true });
        if (results && results.length > 0) {
            const date = results[0].start.date();
            preview.textContent = `Data reconhecida: ${formatPreviewDate(date)}`;
            preview.className = 'text-xs text-emerald-400 mt-2 h-4';
        } else {
            preview.textContent = 'Nenhuma data reconhecida...';
            preview.className = 'text-xs text-zinc-500 mt-2 h-4';
        }
    });

    // Close autocomplete when clicking outside
    document.addEventListener('click', (e) => {
        if (autocompleteVisible && !autocompleteContainer.contains(e.target) && e.target !== activeInput) {
            hideAutocomplete();
        }
    });

    // Save appointment
    saveBtn.addEventListener('click', async () => {
        const text = input.value.trim();
        if (!text) return;

        const normalizedText = normalizarPT(text);
        const results = chronoPt.parse(normalizedText, new Date(), { forwardDate: true });
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

        const data = {
            description: description,
            event_date: eventDate.toISOString(),
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
                if (autocompleteVisible && activeInput === input) {
                    return; // Let setupAutocomplete handle selecting the note
                }
                e.preventDefault(); 
                input.blur(); 
            }
            if (e.key === 'Escape') { 
                if (autocompleteVisible && activeInput === input) {
                    return; // Let setupAutocomplete close the autocomplete dropdown
                }
                input.value = currentDesc; 
                input.blur(); 
            }
        });
    };

    // Generate a deterministic HSL color based on the tag string
    function getTagColor(tag) {
        let hash = 0;
        const clean = tag.toLowerCase().replace(/#/g, '').trim();
        for (let i = 0; i < clean.length; i++) {
            hash = clean.charCodeAt(i) + ((hash << 5) - hash);
        }
        const hue = Math.abs(hash) % 360;
        return {
            base: `hsl(${hue}, 80%, 65%)`,
            alpha: `hsla(${hue}, 80%, 65%, 0.2)`
        };
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

    // Builds background decoration items: weekend shading + week-number markers
    // Result is cached so it's only computed once per session
    function buildDecorations() {
        if (cachedDecorations) return cachedDecorations;
        const result = [];
        const rangeStart = new Date();
        rangeStart.setFullYear(rangeStart.getFullYear() - 1);
        const rangeEnd = new Date();
        rangeEnd.setFullYear(rangeEnd.getFullYear() + 2);

        // Align to the Monday of the week that contains rangeStart
        let cur = new Date(rangeStart);
        const dow = cur.getDay();
        cur.setDate(cur.getDate() - (dow === 0 ? 6 : dow - 1));
        cur.setHours(0, 0, 0, 0);

        while (cur < rangeEnd) {
            const wn = getISOWeek(cur);
            const weekStart = new Date(cur);

            // Week number marker: vertical left border on Monday + small "S{n}" label
            result.push({
                id: `wn-${cur.getTime()}`,
                content: `<span style="font-size:9px;color:#3f3f46;font-weight:700;font-family:monospace;pointer-events:none;user-select:none;">S${wn}</span>`,
                start: weekStart,
                end: new Date(weekStart.getTime() + 86400000), // Monday only
                type: 'background',
                className: 'week-num-bg'
            });

            // Weekend shading: Saturday 00:00 → Monday 00:00
            const satStart = new Date(cur.getTime() + 5 * 86400000);
            const monStart = new Date(cur.getTime() + 7 * 86400000);
            result.push({
                id: `we-${cur.getTime()}`,
                content: '',
                start: satStart,
                end: monStart,
                type: 'background',
                className: 'weekend-shade'
            });

            cur.setDate(cur.getDate() + 7);
        }

        cachedDecorations = result;
        return result;
    }

    // Render vis timeline
    function renderTimeline() {
        items.clear();

        const data = appointments.map(a => {
            const date = new Date(a.event_date);
            const dateStr = date.toLocaleString('pt-BR', { weekday: 'short', day: '2-digit', month: '2-digit', year: 'numeric', hour: '2-digit', minute: '2-digit' });
            
            const tags = a.description.match(/#[\w\-]+/g) || [];
            let itemClassName = '';
            if (tags.length > 0) {
                itemClassName = registerTagStyle(tags[0]);
            }
            
            return {
                id: a.id,
                content: '',
                start: a.event_date,
                type: 'point',
                className: itemClassName,
                title: `<div class="tt-desc">${formatTooltipDescription(a.description)}</div><div class="tt-date">📅 ${dateStr}</div>`
            };
        });
        items.add(data);
        // Re-add decorations after every clear() — they are cached so no recomputation cost
        items.add(buildDecorations());

        if (!timeline) {
            const options = {
                locale: 'pt-br',
                height: '100%',
                margin: { item: 10, axis: 5 },
                orientation: 'top',
                showCurrentTime: true,
                tooltip: { followMouse: true, overflowMethod: 'cap', delay: 0 },
                zoomMin: 1000 * 60 * 60 * 24,           // 1 day
                zoomMax: 1000 * 60 * 60 * 24 * 31 * 12 * 2, // 2 years
            };
            timeline = new vis.Timeline(timelineContainer, items, options);

            // Centraliza o tempo atual e abre uma janela de visualização de ~2 semanas
            const start = new Date();
            start.setDate(start.getDate() - 3);
            const end = new Date();
            end.setDate(end.getDate() + 14);
            timeline.setWindow(start, end);
        } else {
            timeline.setItems(items);
        }
    }

    // Load initial data
    loadAppointments();
});
