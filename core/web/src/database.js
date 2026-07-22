// database.js — Inicialização do Tabulator (tabela de notas)
// Extraído de internal/features/notes/database.templ para melhor organização
(function () {
    "use strict";

    var table;

    // ── Helpers de carregamento dinâmico ──
    function loadStyle(url, callback) {
        if (document.querySelector('link[href="' + url + '"]')) {
            if (callback) callback();
            return;
        }
        var link = document.createElement("link");
        link.rel = "stylesheet";
        link.href = url;
        if (callback) link.onload = callback;
        document.head.appendChild(link);
    }

    function loadScript(url, callback) {
        if (window.Tabulator) {
            if (callback) callback();
            return;
        }
        var script = document.createElement("script");
        script.type = "text/javascript";
        script.src = url;
        script.onload = callback;
        document.head.appendChild(script);
    }

    // ── Formatter de tags ──
    var tagsFormatter = function (cell) {
        var value = cell.getValue();
        if (!value) return "";
        var tags = value.split(",");
        var html = "";
        tags.forEach(function (t) {
            var trimmed = t.trim();
            var lt = trimmed.toLowerCase();
            if (trimmed && lt !== "typst" && lt !== "drawing" && lt !== "spreadsheet" && lt !== "mermaid" && lt !== "mindmap" && lt !== "markmap" && lt !== "map" && lt !== "mapa") {
                html += '<span class="tag-pill">#' + trimmed + "</span>";
            }
        });
        return html;
    };

    // ── Detecção de tipo de nota (duplicado do backend para formatação) ──
    // Converte valor de cor para atributo class ou style.
    // Se for hex (#F54927), rgb() ou hsl(), retorna {style:"color:#F54927"}.
    // Se for classe Tailwind (text-pink-400), retorna {class:"text-pink-400"}.
    function resolveColor(color) {
        if (!color) return {class: "", style: ""};
        if (color.charAt(0) === '#' || color.indexOf('rgb(') === 0 || color.indexOf('rgba(') === 0 || color.indexOf('hsl(') === 0 || color.indexOf('hsla(') === 0) {
            return {class: "", style: "color:" + color};
        }
        return {class: color, style: ""};
    }

    function getLucideIcon(type) {
        var baseCls = "w-3.5 h-3.5";
        var iconName = type;
        var colorVal = "";

        if (window.TON_ICON_CONFIG && window.TON_ICON_CONFIG[type]) {
            iconName = window.TON_ICON_CONFIG[type].Icon || type;
            colorVal = window.TON_ICON_CONFIG[type].Color || "";
        }

        var colorRes = resolveColor(colorVal);
        var cls = colorRes.class ? baseCls + " " + colorRes.class : baseCls;
        var sty = colorRes.style || "";
        var styleAttr = sty ? 'style="' + sty + '"' : '';

        switch (iconName) {
            case "pdf": case "book-text":
                return `<svg class="${cls}" ${styleAttr} viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M4 19.5v-15A2.5 2.5 0 0 1 6.5 2H19a1 1 0 0 1 1 1v18a1 1 0 0 1-1 1H6.5a1 1 0 0 1 0-5H20"/><path d="M8 7h6"/><path d="M8 11h8"/></svg>`;
            case "epub": case "book-open":
                return `<svg class="${cls}" ${styleAttr} xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 7v14"/><path d="M3 18a1 1 0 0 1-1-1V4a1 1 0 0 1 1-1h5a4 4 0 0 1 4 4 4 4 0 0 1 4-4h5a1 1 0 0 1 1 1v13a1 1 0 0 1-1 1h-6a3 3 0 0 0-3 3 3 3 0 0 0-3-3z"/></svg>`;
            case "package": case "package-plus": case "anexo":
                return `<svg class="${cls}" ${styleAttr} xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M16 16h6"/><path d="M19 13v6"/><path d="M21 10V8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l2-1.14"/><path d="m7.5 4.27 9 5.15"/><polyline points="3.29 7 12 12 20.71 7"/><line x1="12" x2="12" y1="22" y2="12"/></svg>`;
            case "archive": case "arquivo":
                return `<svg class="${cls}" ${styleAttr} xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect width="20" height="5" x="2" y="3" rx="1"/><path d="M4 8v11a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8"/><path d="M10 12h4"/></svg>`;
            case "table": case "planilha": case "spreadsheet":
                return `<svg class="${cls}" ${styleAttr} viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 3v18"/><rect width="18" height="18" x="3" y="3" rx="2"/><path d="M3 9h18"/><path d="M3 15h18"/></svg>`;
            case "pencil-ruler": case "desenho": case "drawing":
                return `<svg class="${cls}" ${styleAttr} xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m15 5 4 4"/><path d="M13 7 8.7 11.3a2 2 0 0 0-.57 1.21L8 16l3.49-.13a2 2 0 0 0 1.21-.57L17 11"/><path d="M2 22h20"/><path d="M4 18v-4h4"/><path d="M12 18v-2h4"/><path d="M18 18v-4h4"/></svg>`;
            case "book-down": case "typst":
                return `<svg class="${cls}" ${styleAttr} xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 13V7"/><path d="M4 19.5v-15A2.5 2.5 0 0 1 6.5 2H19a1 1 0 0 1 1 1v18a1 1 0 0 1-1 1H6.5a1 1 0 0 1 0-5H20"/><path d="m9 10 3 3 3-3"/></svg>`;
            case "vector-square": case "mermaid":
                return `<svg class="${cls}" ${styleAttr} xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect width="14" height="14" x="5" y="5" rx="2"/><rect width="4" height="4" x="3" y="3"/><rect width="4" height="4" x="17" y="3"/><rect width="4" height="4" x="3" y="17"/><rect width="4" height="4" x="17" y="17"/></svg>`;
            case "chart-no-axes-gantt": case "mindmap": case "markmap":
                return `<svg class="${cls}" ${styleAttr} xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M8 6h10"/><path d="M6 12h9"/><path d="M11 18h7"/></svg>`;
            case "map": case "mapa":
                return `<svg class="${cls}" ${styleAttr} xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M14.106 5.553a2 2 0 0 0-1.788 0l-3.648 1.824a2 2 0 0 1-1.788 0L2.35 5.11a1 1 0 0 0-1.35 1.348l4.086 8.172a2 2 0 0 0 1.788 0l3.648-1.824a2 2 0 0 1 1.788 0l4.532 2.266a1 1 0 0 0 1.35-1.348l-4.086-8.172Z"/><path d="M15 5.764v15"/><path d="M9 3.236v15"/></svg>`;
            default: // sticky-note
                return `<svg class="${cls}" ${styleAttr} xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M16 3H5a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2V8Z"/><path d="M15 3v5h5"/></svg>`;
        }
    }

    function detectNoteType(file, tagsStr, typeStr) {
        var f = String(file).toLowerCase();
        var t = String(tagsStr || "").toLowerCase();
        var ty = String(typeStr || "").toLowerCase();

        if (f.indexOf("pdfs/") === 0) return { url: "/file?name=" + encodeURIComponent(file), icon: getLucideIcon("pdf"), blank: true };
        if (f.indexOf("attachments/") === 0) return { url: "/file/download?name=" + encodeURIComponent(file), icon: getLucideIcon("package"), blank: true };
        if (f.indexOf("archives/") === 0 || ty === "arquivo") return { url: "/file/download?name=" + encodeURIComponent(file), icon: getLucideIcon("archive"), blank: true };
        if (f.indexOf("epubs/") === 0 || ty === "epub" || f.indexOf(".epub") > 0) return { url: "/epub/reader?file=" + encodeURIComponent(file), icon: getLucideIcon("epub"), blank: false };
        if (t.indexOf("spreadsheet") !== -1 || f.indexOf("sheet") !== -1 || ty === "spreadsheet" || ty === "planilha") return { url: "/spreadsheet?file=" + encodeURIComponent(file), icon: getLucideIcon("table"), blank: false };
        if (t.indexOf("drawing") !== -1 || f.indexOf("drawing") !== -1 || ty === "drawing" || ty === "desenho") return { url: "/drawing?file=" + encodeURIComponent(file), icon: getLucideIcon("pencil-ruler"), blank: false };
        if (t.indexOf("typst") !== -1 || ty === "typst") return { url: "/typst?file=" + encodeURIComponent(file), icon: getLucideIcon("book-down"), blank: false };
        if (t.indexOf("mermaid") !== -1 || ty === "mermaid") return { url: "/mermaid?file=" + encodeURIComponent(file), icon: getLucideIcon("vector-square"), blank: false };
        if (t.indexOf("markmap") !== -1 || t.indexOf("mindmap") !== -1 || ty === "markmap" || ty === "mindmap") return { url: "/mindmap?file=" + encodeURIComponent(file), icon: getLucideIcon("chart-no-axes-gantt"), blank: false };
        if ((t.indexOf("map") !== -1 || f.indexOf("map") !== -1 || ty === "map" || ty === "mapa") && !(t.indexOf("markmap") !== -1 || t.indexOf("mindmap") !== -1 || ty === "markmap" || ty === "mindmap")) return { url: "/map?file=" + encodeURIComponent(file), icon: getLucideIcon("map"), blank: false };
        return { url: "/editor?file=" + encodeURIComponent(file), icon: getLucideIcon("sticky-note"), blank: false };
    }

    // ── Parser de busca avançada ──
    function parseSearchQuery(query) {
        var filters = [];
        var generalTerms = [];
        var i = 0;
        var len = query.length;

        function skipWhitespace() {
            while (i < len && /\s/.test(query[i])) i++;
        }

        while (i < len) {
            skipWhitespace();
            if (i >= len) break;

            var keyMatch = query.slice(i).match(/^([a-zA-ZÀ-ÿ0-9_\-]+)\s*:/);
            if (keyMatch) {
                var rawKey = keyMatch[1];
                var key = rawKey.toLowerCase();
                if (key === "título" || key === "titulo" || key === "title") key = "titulo";
                else if (key === "file" || key === "arquivo") key = "arquivo";
                else if (key === "tipo" || key === "type") key = "type";
                else if (key === "mtime" || key === "modificacao" || key === "modificação" || key === "date" || key === "data") key = "mtime";
                else if (key === "tags" || key === "tag") key = "tags";

                i += keyMatch[0].length;
                skipWhitespace();

                var val = "";
                if (i < len && (query[i] === '"' || query[i] === "'")) {
                    var quoteChar = query[i];
                    i++;
                    while (i < len && query[i] !== quoteChar) { val += query[i]; i++; }
                    if (i < len) i++;
                } else {
                    while (i < len) {
                        var nextKeyMatch = query.slice(i).match(/^\s+([a-zA-ZÀ-ÿ0-9_\-]+)\s*:/);
                        if (nextKeyMatch) break;
                        val += query[i];
                        i++;
                    }
                    val = val.trim();
                }
                filters.push({ key: key, value: val });
            } else {
                var term = "";
                if (query[i] === '"' || query[i] === "'") {
                    var quoteChar = query[i];
                    i++;
                    while (i < len && query[i] !== quoteChar) { term += query[i]; i++; }
                    if (i < len) i++;
                } else {
                    while (i < len && !/\s/.test(query[i])) { term += query[i]; i++; }
                }
                term = term.trim();
                if (term) generalTerms.push(term.toLowerCase());
            }
        }
        return { filters: filters, generalTerms: generalTerms };
    }

    // ── Inicialização principal ──
    function initTabulator() {
        fetch("/api/notes/database")
            .then(function (res) { return res.json(); })
            .then(function (data) {
                var statsEl = document.getElementById("db-stats");
                if (statsEl) statsEl.innerText = data.data.length + " notas registradas";

                var savedVisibility = {};
                try {
                    savedVisibility = JSON.parse(localStorage.getItem("db_column_visibility")) || {};
                } catch (e) { }

                var cols = data.columns.map(function (c) {
                    if (savedVisibility[c.field] !== undefined) {
                        c.visible = savedVisibility[c.field];
                    }

                    // embedded column
                    if (c.field === "embeded") {
                        c.formatter = function (cell) {
                            var rowData = cell.getRow().getData();
                            var typeStr = String(rowData.type || rowData.Type || "").toLowerCase();
                            if (typeStr === "desenho" || typeStr === "planilha" || typeStr === "mapa" || typeStr === "mermaid" || typeStr === "pdf" || typeStr === "anexo" || typeStr === "arquivo" || typeStr === "epub") {
                                return "N/A";
                            }
                            return cell.getValue() ? "true" : "false";
                        };
                        return c;
                    }

                    // abrir_link column
                    if (c.field === "abrir_link") {
                        c.formatter = function (cell) {
                            var rowData = cell.getRow().getData();
                            var file = rowData.arquivo || "";
                            var tagsStr = String(rowData.tags || "");
                            var typeStr = String(rowData.type || rowData.Type || "");
                            var info = detectNoteType(file, tagsStr, typeStr);
                            var target = info.blank ? " target='_blank'" : "";
                            return "<a href='" + info.url + "'" + target + " class='text-sky-400 hover:text-sky-300 font-bold flex items-center gap-1 justify-center' title='Abrir'>" + info.icon + " <span class='underline'>Abrir</span></a>";
                        };
                        return c;
                    }

                    // tags column
                    if (c.field === "tags") {
                        c.formatter = tagsFormatter;
                        return c;
                    }

                    // titulo column
                    if (c.field === "titulo") {
                        c.formatter = function (cell) {
                            return "<strong class='text-sky-400'>" + cell.getValue() + "</strong>";
                        };
                        return c;
                    }

                    // Editor adaptativo para colunas editáveis
                    if (c.editor) {
                        c.headerTooltip = "Clique nas células para editar";
                        if (c.field !== "titulo" && c.field !== "tags") {
                            c.editor = function (cell) {
                                var val = cell.getValue();
                                if (typeof val === "boolean") return "tickCross";
                                if (typeof val === "string" && /^\d{4}-\d{2}-\d{2}/.test(val.trim())) return "date";
                                if (typeof val === "number" || (typeof val === "string" && val.trim() !== "" && !isNaN(val.trim()))) return "number";
                                return "input";
                            };
                            c.formatter = function (cell) {
                                var val = cell.getValue();
                                if (typeof val === "boolean") return val ? "<span class='text-emerald-500 font-bold'>✔️</span>" : "<span class='text-rose-500 font-bold'>❌</span>";
                                return val;
                            };
                        }
                    }

                    return c;
                });

                table = new Tabulator("#notes-table", {
                    data: data.data,
                    columns: cols,
                    height: "calc(100vh - 190px)",
                    layout: "fitColumns",
                    responsiveLayout: "collapse",
                    columnResizeGuide: true,
                    pagination: "local",
                    paginationSize: 100,
                    paginationSizeSelector: [100, 200, 500, true],
                    placeholder: "Nenhuma nota encontrada no Tabulator.",
                    initialSort: [{ column: "mtime", dir: "desc" }]
                });

                // Populate column selector checkboxes
                populateColumnCheckboxes(cols, table);

                // Column selector dropdown
                setupColumnSelector();

                // Cell edit handler
                table.on("cellEdited", function (cell) {
                    handleCellEdit(cell);
                });

                // Search setup
                setupSearch(table);

                // Restore last query
                var lastQuery = localStorage.getItem("db_last_query") || "";
                var searchInput = document.getElementById("db-search");
                var clearBtn = document.getElementById("db-search-clear");
                if (lastQuery && searchInput) {
                    searchInput.value = lastQuery;
                    applyFilter(table, lastQuery);
                    if (clearBtn) clearBtn.classList.remove("hidden");
                }
            })
            .catch(function (err) {
                console.error("Error loading database:", err);
                var statsEl = document.getElementById("db-stats");
                if (statsEl) statsEl.innerText = "Erro ao carregar dados";
                var tableEl = document.getElementById("notes-table");
                if (tableEl) tableEl.innerHTML = "<div class='p-8 text-center text-red-400'>Não foi possível carregar a tabela.</div>";
            });
    }

    // ── Column selector ──
    function populateColumnCheckboxes(cols, table) {
        var container = document.getElementById("column-checkboxes");
        if (!container) return;
        container.innerHTML = "";

        cols.forEach(function (c) {
            var fieldName = c.field;
            var titleName = c.title || fieldName;
            if (!fieldName) return;

            var isVisible = c.visible !== false;
            var savedVisibility = {};
            try {
                savedVisibility = JSON.parse(localStorage.getItem("db_column_visibility")) || {};
            } catch (e) { }

            var label = document.createElement("label");
            label.className = "flex items-center gap-2.5 px-2 py-1.5 rounded-lg text-zinc-300 hover:text-white hover:bg-zinc-900 cursor-pointer transition-colors text-[12px] font-medium";

            var input = document.createElement("input");
            input.type = "checkbox";
            input.checked = isVisible;
            input.className = "w-3.5 h-3.5 rounded border-zinc-800 text-sky-500 bg-zinc-900 focus:ring-sky-500 focus:ring-offset-zinc-950 transition-colors";

            input.addEventListener("change", function () {
                var checked = input.checked;
                if (checked) table.showColumn(fieldName);
                else table.hideColumn(fieldName);
                savedVisibility[fieldName] = checked;
                localStorage.setItem("db_column_visibility", JSON.stringify(savedVisibility));
            });

            var span = document.createElement("span");
            span.innerText = titleName;

            label.appendChild(input);
            label.appendChild(span);
            container.appendChild(label);
        });
    }

    function setupColumnSelector() {
        var btn = document.getElementById("column-selector-btn");
        var menu = document.getElementById("column-selector-menu");
        if (!btn || !menu) return;

        btn.addEventListener("click", function (e) {
            e.stopPropagation();
            menu.classList.toggle("hidden");
        });

        document.addEventListener("click", function (e) {
            if (!menu.contains(e.target) && e.target !== btn) {
                menu.classList.add("hidden");
            }
        });
    }

    // ── Cell edit handler ──
    function handleCellEdit(cell) {
        var field = cell.getField();
        var rowData = cell.getRow().getData();
        var newVal = cell.getValue();
        var oldVal = cell.getOldValue();
        if (newVal === oldVal) return;

        var parsedVal = newVal;
        if (typeof newVal === "string") {
            var trimmed = newVal.trim();
            if (trimmed === "true") parsedVal = true;
            else if (trimmed === "false") parsedVal = false;
            else if (trimmed !== "" && !isNaN(trimmed)) parsedVal = Number(trimmed);
        }

        var payload = { file: rowData.arquivo, key: field, value: parsedVal };

        fetch("/api/notes/update-property", {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify(payload)
        }).then(function (res) {
            if (!res.ok) {
                alert("Erro ao salvar propriedade no arquivo.");
                cell.restoreOldValue();
                return;
            }
            if (field === "titulo") {
                var newFileName = newVal.trim();
                if (!newFileName.endsWith(".md")) newFileName += ".md";
                if (!newFileName.startsWith("notes/")) newFileName = "notes/" + newFileName;
                cell.getRow().update({ arquivo: newFileName });
                cell.getRow().reformat();
            }
        }).catch(function () {
            alert("Erro de conexão ao salvar.");
            cell.restoreOldValue();
        });
    }

    // ── Search ──
    function setupSearch(table) {
        var searchInput = document.getElementById("db-search");
        var clearBtn = document.getElementById("db-search-clear");
        if (!searchInput || !clearBtn) return;

        function updateClearButton() {
            if (searchInput.value) clearBtn.classList.remove("hidden");
            else clearBtn.classList.add("hidden");
        }

        searchInput.addEventListener("input", function () {
            var val = searchInput.value;
            localStorage.setItem("db_last_query", val);
            applyFilter(table, val);
            updateClearButton();
        });

        clearBtn.addEventListener("click", function () {
            searchInput.value = "";
            localStorage.setItem("db_last_query", "");
            applyFilter(table, "");
            updateClearButton();
            searchInput.focus();
        });
    }

    function applyFilter(table, queryValue) {
        var val = queryValue.trim();
        if (!val) {
            table.clearFilter();
            return;
        }

        table.setFilter(function (data) {
            var parsed = parseSearchQuery(val);
            if (parsed.filters.length === 0 && parsed.generalTerms.length === 0) return true;

            for (var i = 0; i < parsed.filters.length; i++) {
                var f = parsed.filters[i];
                var cellVal = data[f.key];
                if (cellVal === undefined || cellVal === null) return false;
                var cellStr = String(cellVal).toLowerCase();
                var filterVal = f.value.toLowerCase();

                if (f.key === "tags") {
                    var searchTags = filterVal.split(",").map(function (t) { return t.trim(); }).filter(Boolean);
                    var noteTags = cellStr.split(",").map(function (t) { return t.trim(); }).filter(Boolean);
                    var anyTagMatched = searchTags.some(function (sTag) {
                        return noteTags.some(function (nTag) { return nTag.indexOf(sTag) !== -1; });
                    });
                    if (!anyTagMatched) return false;
                } else {
                    if (cellStr.indexOf(filterVal) === -1) return false;
                }
            }

            for (var j = 0; j < parsed.generalTerms.length; j++) {
                var term = parsed.generalTerms[j];
                var termFound = false;
                for (var key in data) {
                    if (data[key] && String(data[key]).toLowerCase().indexOf(term) !== -1) {
                        termFound = true;
                        break;
                    }
                }
                if (!termFound) return false;
            }
            return true;
        });
    }

    // ── Bootstrap ──
    var styleLoaded = false;
    var scriptLoaded = false;

    function checkAndInit() {
        if (styleLoaded && scriptLoaded) {
            initTabulator();
        }
    }

    loadStyle("/static/tabulator_midnight.min.css", function () {
        styleLoaded = true;
        checkAndInit();
    });

    loadScript("/static/tabulator.min.js", function () {
        scriptLoaded = true;
        checkAndInit();
    });
})();
