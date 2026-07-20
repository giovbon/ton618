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
    function detectNoteType(file, tagsStr, typeStr) {
        var f = String(file).toLowerCase();
        var t = String(tagsStr || "").toLowerCase();
        var ty = String(typeStr || "").toLowerCase();

        if (f.indexOf("pdfs/") === 0) return { url: "/file?name=" + encodeURIComponent(file), icon: "📕", blank: true };
        if (f.indexOf("attachments/") === 0) return { url: "/file/download?name=" + encodeURIComponent(file), icon: "📦", blank: true };
        if (f.indexOf("archives/") === 0 || ty === "arquivo") return { url: "/file/download?name=" + encodeURIComponent(file), icon: "💾", blank: true };
        if (f.indexOf("epubs/") === 0 || ty === "epub" || f.indexOf(".epub") > 0) return { url: "/epub/reader?file=" + encodeURIComponent(file), icon: "📒", blank: false };
        if (t.indexOf("spreadsheet") !== -1 || f.indexOf("sheet") !== -1 || ty === "spreadsheet" || ty === "planilha") return { url: "/spreadsheet?file=" + encodeURIComponent(file), icon: "📊", blank: false };
        if (t.indexOf("drawing") !== -1 || f.indexOf("drawing") !== -1 || ty === "drawing" || ty === "desenho") return { url: "/drawing?file=" + encodeURIComponent(file), icon: "🎨", blank: false };
        if (t.indexOf("typst") !== -1 || ty === "typst") return { url: "/typst?file=" + encodeURIComponent(file), icon: "📘", blank: false };
        if (t.indexOf("mermaid") !== -1 || ty === "mermaid") return { url: "/mermaid?file=" + encodeURIComponent(file), icon: "🧜", blank: false };
        if (t.indexOf("markmap") !== -1 || t.indexOf("mindmap") !== -1 || ty === "markmap" || ty === "mindmap") return { url: "/mindmap?file=" + encodeURIComponent(file), icon: "🔱", blank: false };
        if ((t.indexOf("map") !== -1 || f.indexOf("map") !== -1 || ty === "map" || ty === "mapa") && !(t.indexOf("markmap") !== -1 || t.indexOf("mindmap") !== -1 || ty === "markmap" || ty === "mindmap")) return { url: "/map?file=" + encodeURIComponent(file), icon: "🗺️", blank: false };
        return { url: "/editor?file=" + encodeURIComponent(file), icon: "📄", blank: false };
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
                } catch (e) {}

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
            } catch (e) {}

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
