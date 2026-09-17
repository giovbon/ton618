// database.js — Inicialização do Tabulator (tabela de notas)
// Extraído de internal/features/notes/database.templ para melhor organização
(function () {
    "use strict";

    var table;

    // Dados completos vindos do servidor (raízes com "_children" quando existe
    // hierarquia, linhas planas quando não existe). A busca reconstrói a árvore
    // a partir disto — ver applySearch.
    var baseRows = [];
    var hasTree = false;
    // Busca ativa: as linhas nascem todas expandidas para o resultado aparecer
    // (ver dataTreeStartExpanded).
    var searchActive = false;
    // A tabela está exibindo um subconjunto filtrado (precisa de setData para
    // voltar ao conjunto completo quando a busca é limpa).
    var filterApplied = false;

    // Campo que o módulo Data Tree lê para montar os níveis (note_tree.go).
    var childrenKey = "_children";

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
            if (trimmed && lt !== "drawing" && lt !== "mindmap" && lt !== "markmap") {
                html += '<span class="tag-pill">#' + trimmed + "</span>";
            }
        });
        return html;
    };

    // ── Ícone fallback ──
    // O ícone real (SVG) já vem pronto do servidor no campo _icon de cada linha
    // (SSOT). Este fallback só cobre o caso raro de o campo estar ausente.
    function noteIconFallback() {
        return `<svg class="w-3.5 h-3.5" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M16 3H5a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2V8Z"/><path d="M15 3v5h5"/></svg>`;
    }

    // ── Hierarquia de notas (propriedade "pai" do frontmatter) ──
    // O estado de expansão é guardado por arquivo no localStorage: sem isso a
    // árvore voltaria toda fechada a cada recarga da página.
    var TREE_EXPANDED_KEY = "db_tree_expanded";
    var expandedRows = null;

    function loadExpandedRows() {
        if (expandedRows) return expandedRows;
        try {
            expandedRows = JSON.parse(localStorage.getItem(TREE_EXPANDED_KEY)) || {};
        } catch (e) {
            expandedRows = {};
        }
        return expandedRows;
    }

    function saveExpandedRows() {
        try {
            localStorage.setItem(TREE_EXPANDED_KEY, JSON.stringify(loadExpandedRows()));
        } catch (e) { }
    }

    // O 1º nível começa aberto; a escolha do usuário (por nota) tem prioridade
    // e sobrevive ao reload. Com busca ativa TUDO nasce expandido: o caminho até
    // o resultado (e as irmãs, no modo contexto) precisa estar visível sem o
    // usuário ter de abrir nível por nível.
    function treeStartExpanded(row, level) {
        if (searchActive) return true;
        var saved = loadExpandedRows()[row.getData().arquivo];
        return saved === undefined ? level === 0 : saved;
    }

    function setupTreeState(table) {
        table.on("dataTreeRowExpanded", function (row) {
            loadExpandedRows()[row.getData().arquivo] = true;
            saveExpandedRows();
        });
        table.on("dataTreeRowCollapsed", function (row) {
            loadExpandedRows()[row.getData().arquivo] = false;
            saveExpandedRows();
        });
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

    // ── Editor adaptativo ──
    // ⚠️ O Tabulator NÃO aceita uma função de coluna que devolva o NOME do
    // editor: `editor: function(cell)` é tratado como EDITOR CUSTOMIZADO e o
    // retorno precisa ser um NÓ do DOM. A versão anterior devolvia "input", e
    // toda célula dessas colunas (inclusive a de hierarquia) ficava sem edição —
    // com "Edit Error - Editor should return an instance of Node" no console.
    function autoEditor(cell, onRendered, success, cancel) {
        var val = cell.getValue();
        var isBool = typeof val === "boolean";
        var isDate = !isBool && typeof val === "string" && /^\d{4}-\d{2}-\d{2}$/.test(val.trim());
        var asText = val === null || val === undefined ? "" : String(val);

        var input = document.createElement("input");
        if (isBool) {
            input.type = "checkbox";
            input.checked = val;
        } else if (isDate) {
            input.type = "date";
            input.value = val.trim();
        } else {
            // Números e listas YAML (ex: `pai: [[nota]]` sem aspas) são editados
            // como texto puro; handleCellEdit converte o que for número.
            input.type = "text";
            input.value = asText;
        }

        // O editor é finalizado UMA vez: success() fecha a célula (o que dispara
        // o blur do input) e sem a trava o commit rodaria de novo.
        var finished = false;
        function commit() {
            if (finished) return;
            finished = true;
            if (isBool) { success(input.checked); return; }
            if (input.value === asText) { success(val); return; } // nada mudou
            success(input.value);
        }
        function abort() {
            if (finished) return;
            finished = true;
            cancel();
        }

        input.addEventListener("keydown", function (e) {
            if (e.key === "Enter") { e.preventDefault(); e.stopPropagation(); commit(); }
            else if (e.key === "Escape") { e.stopPropagation(); abort(); }
        });
        input.addEventListener("blur", commit);
        if (isBool) input.addEventListener("change", commit);

        onRendered(function () {
            input.focus();
            if (input.select && input.type !== "checkbox") input.select();
        });

        return input;
    }

    // ── Contador do topo ──
    // Mostra apenas QUANTAS notas estão na tabela agora (muda a cada busca).
    function setStats(count, query) {
        var el = document.getElementById("db-stats");
        if (!el) return;
        el.innerText = String(count);
        el.title = query
            ? count + " nota(s) exibida(s) para a busca \"" + query + "\""
            : count + " nota(s) exibida(s)";
    }

    // ── Modo de busca ──
    // "contexto" (padrão): a nota encontrada + o caminho de pais + as irmãs.
    // "resultados": apenas as notas que casaram com a busca.
    var SEARCH_MODE_KEY = "db_search_mode";

    function loadSearchMode() {
        try {
            return localStorage.getItem(SEARCH_MODE_KEY) === "resultados" ? "resultados" : "contexto";
        } catch (e) {
            return "contexto";
        }
    }

    function saveSearchMode(mode) {
        try {
            localStorage.setItem(SEARCH_MODE_KEY, mode);
        } catch (e) { }
    }

    function updateSearchModeButton() {
        var btn = document.getElementById("db-search-mode");
        if (!btn) return;
        var isResults = loadSearchMode() === "resultados";

        var label = document.getElementById("db-search-mode-label");
        if (label) label.innerText = isResults ? "Só resultados" : "Pai + irmãos";

        btn.title = isResults
            ? "Busca: mostrando só as notas que casaram — clique para incluir a nota-mãe e as irmãs"
            : "Busca: mostrando o resultado com o caminho de pais e as irmãs — clique para mostrar só as notas que casaram";
        btn.setAttribute("aria-pressed", isResults ? "true" : "false");

        var iconContext = document.getElementById("db-search-mode-icon-contexto");
        var iconResults = document.getElementById("db-search-mode-icon-resultados");
        if (iconContext) iconContext.classList.toggle("hidden", isResults);
        if (iconResults) iconResults.classList.toggle("hidden", !isResults);
    }

    function setupSearchModeButton() {
        var btn = document.getElementById("db-search-mode");
        if (!btn) return;
        updateSearchModeButton();
        btn.addEventListener("click", function () {
            saveSearchMode(loadSearchMode() === "resultados" ? "contexto" : "resultados");
            updateSearchModeButton();
            var input = /** @type {HTMLInputElement} */ (document.getElementById("db-search"));
            applySearch(input ? input.value : "");
        });
    }

    // ── Filtro de busca ──
    // O filtro é aplicado nos DADOS (setData) e não com table.setFilter: o
    // módulo Data Tree esconde os filhos de uma linha filtrada, então um
    // resultado aninhado desapareceria se o pai não casasse (bug que já obrigou
    // o predicado a manter os ancestrais). Reconstruindo a floresta aqui, o
    // resultado sempre aparece e as duas modalidades de busca ficam explícitas.
    function makeMatcher(parsed) {
        // Pré-normaliza os filtros (evita toLowerCase/split repetidos por linha).
        var filters = parsed.filters.map(function (f) {
            var filterVal = String(f.value).toLowerCase();
            return {
                key: f.key,
                value: filterVal,
                searchTags: f.key === "tags"
                    ? filterVal.split(",").map(function (t) { return t.trim(); }).filter(Boolean)
                    : null
            };
        });
        var generalTerms = parsed.generalTerms;

        return function (data) {
            for (var i = 0; i < filters.length; i++) {
                var f = filters[i];
                var cellVal = data[f.key];
                if (cellVal === undefined || cellVal === null) return false;
                var cellStr = String(cellVal).toLowerCase();

                if (f.key === "tags") {
                    var noteTags = cellStr.split(",").map(function (t) { return t.trim(); }).filter(Boolean);
                    var anyTagMatched = f.searchTags.some(function (sTag) {
                        return noteTags.some(function (nTag) { return nTag.indexOf(sTag) !== -1; });
                    });
                    if (!anyTagMatched) return false;
                } else {
                    if (cellStr.indexOf(f.value) === -1) return false;
                }
            }

            for (var j = 0; j < generalTerms.length; j++) {
                var term = generalTerms[j];
                var termFound = false;
                for (var key in data) {
                    // Campos internos (_children da árvore, _icon, _url, _blank)
                    // não são conteúdo da nota e virariam "[object Object]".
                    if (key.charAt(0) === "_") continue;
                    if (data[key] && String(data[key]).toLowerCase().indexOf(term) !== -1) {
                        termFound = true;
                        break;
                    }
                }
                if (!termFound) return false;
            }
            return true;
        };
    }

    // Cópia rasa sem "_children": os filhos são recalculados pelo filtro (manter
    // o array original renderizaria ramos que não casaram com a busca).
    function copyNodeWithoutChildren(row) {
        var copy = {};
        for (var k in row) {
            if (k === childrenKey) continue;
            copy[k] = row[k];
        }
        return copy;
    }

    function copyNodeWithChildren(row, kids) {
        var copy = copyNodeWithoutChildren(row);
        if (kids.length) copy[childrenKey] = kids;
        return copy;
    }

    // Modo "contexto": mantém o resultado, a cadeia de pais e as irmãs dele.
    // siblingMatched desce a partir do pai que tem um filho casado — é o que
    // faz as irmãs do resultado aparecerem.
    function filterContext(node, siblingMatched, matches) {
        var kids = node[childrenKey] || [];
        var anyKidMatched = false;
        for (var i = 0; i < kids.length; i++) {
            if (matches(kids[i])) {
                anyKidMatched = true;
                break;
            }
        }

        var kept = [];
        for (var j = 0; j < kids.length; j++) {
            var sub = filterContext(kids[j], anyKidMatched, matches);
            if (sub) kept.push(sub);
        }

        if (!matches(node) && !siblingMatched && kept.length === 0) return null;
        return copyNodeWithChildren(node, kept);
    }

    // Modo "resultados": só o que casou. Devolve uma LISTA porque um nó que não
    // casou é omitido e os resultados que estavam abaixo dele SOBEM um nível
    // (nada de caminho de pais aqui). Resultados aninhados entre si continuam
    // aninhados quando o ancestral também casou.
    function filterResults(node, matches) {
        var kids = node[childrenKey] || [];
        var kept = [];
        var i;

        if (matches(node)) {
            for (i = 0; i < kids.length; i++) {
                kept = kept.concat(filterResults(kids[i], matches));
            }
            return [copyNodeWithChildren(node, kept)];
        }

        var promoted = [];
        for (i = 0; i < kids.length; i++) {
            promoted = promoted.concat(filterResults(kids[i], matches));
        }
        return promoted;
    }

    function filterRows(rows, matches, mode) {
        var out = [];
        for (var i = 0; i < rows.length; i++) {
            if (mode === "resultados") {
                out = out.concat(filterResults(rows[i], matches));
                continue;
            }
            var filtered = filterContext(rows[i], false, matches);
            if (filtered) out.push(filtered);
        }
        return out;
    }

    // Quantas linhas a floresta tem, contando todos os níveis — é o número de
    // notas efetivamente exibidas na tabela.
    function countRows(rows) {
        var total = 0;
        for (var i = 0; i < rows.length; i++) {
            total++;
            var kids = rows[i][childrenKey];
            if (kids && kids.length) total += countRows(kids);
        }
        return total;
    }

    // Recalcula o que a tabela mostra. A busca é aplicada com setData (o
    // Tabulator reconstrói a árvore a partir dos "_children" recalculados) e o
    // contador do topo acompanha o que está sendo exibido.
    function applySearch(queryValue) {
        if (!table) return;

        var val = (queryValue || "").trim();
        var parsed = val ? parseSearchQuery(val) : null;
        searchActive = !!(parsed && (parsed.filters.length || parsed.generalTerms.length));

        var nodes = baseRows;
        if (searchActive) {
            nodes = filterRows(baseRows, makeMatcher(parsed), loadSearchMode());
        }

        setStats(countRows(nodes), searchActive ? val : "");

        // Sem busca e sem filtro anterior a tabela já está com os dados
        // completos: nada a recarregar (evita um render extra no boot).
        if (searchActive || filterApplied) {
            filterApplied = searchActive;
            table.setData(nodes);
        }
    }

    // ── Inicialização principal ──
    function initTabulator() {
        fetch("/api/notes/database")
            .then(function (res) { return res.json(); })
            .then(function (data) {
                hasTree = !!(data.meta && data.meta.hasTree);
                // Com árvore ligada, data.data contém apenas as raízes; sem ela,
                // todas as linhas. O contador do topo é recalculado por
                // applySearch (mostra quantas notas estão sendo exibidas agora).
                baseRows = data.data || [];
                filterApplied = false;

                var savedVisibility = {};
                try {
                    savedVisibility = JSON.parse(localStorage.getItem("db_column_visibility")) || {};
                } catch (e) { }

                var cols = data.columns.map(function (c) {
                    if (savedVisibility[c.field] !== undefined) {
                        c.visible = savedVisibility[c.field];
                    }

                    // No modo árvore o botão de abrir/fechar fica na coluna
                    // "titulo". Ocultá-la desativaria o módulo Data Tree, então
                    // ela fica travada visível.
                    if (hasTree && c.field === "titulo") {
                        c.visible = true;
                    }

                    // embedded column
                    if (c.field === "embeded") {
                        c.formatter = function (cell) {
                            var rowData = cell.getRow().getData();
                            var typeStr = String(rowData.type || rowData.Type || "").toLowerCase();
                            if (typeStr === "desenho" || typeStr === "pdf" || typeStr === "anexo" || typeStr === "arquivo" || typeStr === "epub") {
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
                            // URL, target e ícone já vêm prontos do servidor (SSOT) —
                            // elimina a duplicação client-side que causava ícones divergentes.
                            var url = rowData._url || "/editor";
                            var target = rowData._blank ? " target='_blank'" : "";
                            var iconHtml = rowData._icon || noteIconFallback();
                            return "<a href='" + url + "'" + target + " class='text-sky-400 hover:text-sky-300 font-bold flex items-center gap-1 justify-center' title='Abrir'>" + iconHtml + " <span class='underline'>Abrir</span></a>";
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
                            c.editor = autoEditor;
                            c.formatter = function (cell) {
                                var val = cell.getValue();
                                if (typeof val === "boolean") return val ? "<span class='text-emerald-500 font-bold'>✔️</span>" : "<span class='text-rose-500 font-bold'>❌</span>";
                                // ⚠️ Devolver objeto/array limpa a célula e gera
                                // "Format Error" no console (o Tabulator só aceita
                                // string/Node). Acontece com `pai: [[nota]]` sem
                                // aspas no frontmatter — String() achata a lista e
                                // mostra o valor legível.
                                if (val !== null && typeof val === "object") return String(val);
                                return val;
                            };
                        }
                    }

                    return c;
                });

                table = new Tabulator("#notes-table", {
                    data: baseRows,
                    columns: cols,
                    height: "calc(100vh - 190px)",
                    layout: "fitColumns",
                    responsiveLayout: "collapse",
                    columnResizeGuide: true,
                    index: "arquivo",
                    // ── Hierarquia (propriedade "pai" do frontmatter) ──
                    // As notas vêm do servidor já agrupadas em "_children"
                    // quando existe algum vínculo válido (meta.hasTree).
                    dataTree: hasTree,
                    dataTreeChildField: "_children",
                    dataTreeElementColumn: "titulo",
                    dataTreeChildIndent: 18,
                    dataTreeBranchElement: false,
                    dataTreeStartExpanded: treeStartExpanded,
                    dataTreeSort: false, // filhos mantêm a ordem vinda do servidor
                    // Sem dataTreeExpandElement/CollapseElement de propósito: o
                    // Tabulator SUBSTITUI o controle inteiro quando esses
                    // elementos são fornecidos (some o wrapper
                    // ".tabulator-data-tree-control" e o tabIndex). Usando o
                    // controle padrão, o tema cuida do layout e o CSS de
                    // database.templ só ajusta as cores do dark mode.
                    // No modo árvore a paginação é desligada: filhos contam como
                    // linhas e uma página cortaria uma família no meio.
                    pagination: hasTree ? false : "local",
                    paginationSize: 100,
                    paginationSizeSelector: [100, 200, 500, true],
                    placeholder: "Nenhuma nota encontrada no Tabulator.",
                    initialSort: [{ column: "mtime", dir: "desc" }]
                });

                if (hasTree) setupTreeState(table);

                // Populate column selector checkboxes (a coluna do toggle da
                // árvore não pode ser ocultada — ver populateColumnCheckboxes)
                populateColumnCheckboxes(cols, table, hasTree ? ["titulo"] : null);

                // ⚠️ Os listeners de DOM (busca, botão de modo e seletor de
                // colunas) são registrados UMA vez no bootstrap: initTabulator
                // roda de novo a cada reloadTable (edição da coluna Pai) e
                // re-registrar os mesmos handlers faria cada clique valer por
                // dois (o botão de modo alternava e voltava) e cada tecla
                // disparar a busca em duplicata.

                // Cell edit handler
                table.on("cellEdited", function (cell) {
                    handleCellEdit(cell);
                });

                // Abrir a nota: duplo clique na linha (a coluna "Abrir" nasce
                // oculta no payload — ver HandleGetDatabaseData). O 1º clique de
                // uma célula editável abre o editor, então o duplo clique
                // costuma cair DENTRO do input: nesse caso a edição é cancelada
                // para a nota abrir.
                table.on("rowDblClick", function (e, row) {
                    var target = /** @type {HTMLElement} */ (e.target);
                    if (target.closest(".tabulator-data-tree-control")) return; // expandir/recolher
                    if (target.closest("a")) return; // link da coluna "Abrir"

                    var data = row.getData();
                    if (!data || !data._url) return;

                    var cells = row.getCells ? row.getCells() : [];
                    for (var i = 0; i < cells.length; i++) {
                        var el = cells[i].getElement();
                        if (el && el.contains(target) && el.querySelector("input, textarea, select") && cells[i].cancelEdit) {
                            cells[i].cancelEdit();
                        }
                    }

                    if (data._blank) window.open(data._url, "_blank");
                    else window.location.href = data._url;
                });

                // Restore last query.                // A busca só pode ser aplicada depois do evento tableBuilt —
                // antes disso o Tabulator avisa "Table Not Initialized" e o
                // resultado pode ficar inconsistente.
                var lastQuery = localStorage.getItem("db_last_query") || "";
                var searchInput = /** @type {HTMLInputElement} */ (document.getElementById("db-search"));
                var clearBtn = document.getElementById("db-search-clear");
                if (lastQuery && searchInput) {
                    searchInput.value = lastQuery;
                    if (clearBtn) clearBtn.classList.remove("hidden");
                    if (table.initialized) {
                        applySearch(lastQuery);
                    } else {
                        table.on("tableBuilt", function () {
                            applySearch(lastQuery);
                        });
                    }
                } else {
                    setStats(countRows(baseRows), "");
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
    // lockedFields: colunas que não podem ser ocultadas (no modo árvore, a
    // coluna do título é onde vive o botão de abrir/fechar dos filhos).
    function populateColumnCheckboxes(cols, table, lockedFields) {
        var container = document.getElementById("column-checkboxes");
        if (!container) return;
        container.innerHTML = "";

        cols.forEach(function (c) {
            var fieldName = c.field;
            var titleName = c.title || fieldName;
            if (!fieldName) return;
            if (lockedFields && lockedFields.indexOf(fieldName) !== -1) return;

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
            input.className = "w-3.5 h-3.5 rounded-sm border-zinc-800 text-sky-500 bg-zinc-900 focus:ring-sky-500 focus:ring-offset-zinc-950 transition-colors";

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
            if (!menu.contains(/** @type {Node} */ (e.target)) && e.target !== btn) {
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
        if (field === "titulo" || field === "tags") {
            parsedVal = String(newVal == null ? "" : newVal);
        } else if (typeof newVal === "string") {
            var trimmed = newVal.trim();
            if (trimmed === "true") parsedVal = true;
            else if (trimmed === "false") parsedVal = false;
            else if (trimmed !== "" && !isNaN(Number(trimmed))) parsedVal = Number(trimmed);
        }

        var payload = { file: rowData.arquivo, key: field, value: parsedVal };

        fetch("/api/notes/update-property", {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify(payload)
        }).then(function (res) {
            if (!res.ok) {
                res.text().then(function (errMsg) {
                    alert("Erro ao salvar propriedade no arquivo: " + (errMsg || "Erro no servidor."));
                }).catch(function () {
                    alert("Erro ao salvar propriedade no arquivo.");
                });
                cell.restoreOldValue();
                return;
            }
            // Editar a hierarquia ("pai", com o nome antigo "parent" ainda aceito
            // pelo servidor) muda a estrutura inteira da árvore e o Tabulator
            // não a remonta sozinho: recarregamos a tabela (o servidor devolve
            // os "_children" já recalculados). Colunas, busca e estado de
            // expansão são restaurados do localStorage.
            if (field === "pai" || field === "parent") {
                reloadTable();
                return;
            }
            if (field === "titulo") {
                var rawVal = String(newVal || "").trim();
                var newFileName = rawVal;
                if (!newFileName.endsWith(".md")) newFileName += ".md";
                if (!newFileName.startsWith("notes/")) newFileName = "notes/" + newFileName;
                var newUrl = "/editor?file=" + encodeURIComponent(newFileName);
                cell.getRow().update({ arquivo: newFileName, _url: newUrl });
                cell.getRow().reformat();
            }
        }).catch(function () {
            alert("Erro de conexão ao salvar.");
            cell.restoreOldValue();
        });
    }

    // Reconstrói a tabela do zero a partir do servidor. É o caminho mais seguro
    // depois de uma mudança estrutural (parent): o Tabulator não recalcula a
    // árvore sozinho, e as preferências (colunas visíveis, última busca e
    // estado de expansão) vivem no localStorage — então nada se perde.
    function reloadTable() {
        if (!table) {
            initTabulator();
            return;
        }
        var old = table;
        table = null;
        var reinit = function () { initTabulator(); };
        try {
            var destroyed = old.destroy();
            if (destroyed && typeof destroyed.then === "function") {
                destroyed.then(reinit).catch(reinit);
            } else {
                reinit();
            }
        } catch (e) {
            reinit();
        }
    }

    // ── Search ──
    // A digitação é agrupada (debounce): cada tecla recalcula o conjunto de
    // linhas, e com milhares de notas o setData por caractere travaria a tela.
    var SEARCH_DEBOUNCE_MS = 120;
    var searchTimer = null;

    function setupSearch() {
        var searchInput = /** @type {HTMLInputElement} */ (document.getElementById("db-search"));
        var clearBtn = document.getElementById("db-search-clear");
        if (!searchInput || !clearBtn) return;

        function updateClearButton() {
            if (searchInput.value) clearBtn.classList.remove("hidden");
            else clearBtn.classList.add("hidden");
        }

        searchInput.addEventListener("input", function () {
            var val = searchInput.value;
            try {
                localStorage.setItem("db_last_query", val);
            } catch (e) { }
            updateClearButton();

            if (searchTimer) clearTimeout(searchTimer);
            searchTimer = setTimeout(function () {
                searchTimer = null;
                applySearch(val);
            }, SEARCH_DEBOUNCE_MS);
        });

        clearBtn.addEventListener("click", function () {
            if (searchTimer) {
                clearTimeout(searchTimer);
                searchTimer = null;
            }
            searchInput.value = "";
            try {
                localStorage.setItem("db_last_query", "");
            } catch (e) { }
            applySearch("");
            updateClearButton();
            searchInput.focus();
        });
    }

    // ── Bootstrap ──
    var styleLoaded = false;
    var scriptLoaded = false;
    var bootstrapped = false;

    // Assets (CSS + Tabulator) carregados sob demanda; quando os dois chegam, o
    // bootstrap roda UMA vez e registra os listeners de DOM que não dependem da
    // tabela (ver o comentário em initTabulator).
    function checkAndInit() {
        if (bootstrapped || !styleLoaded || !scriptLoaded) return;
        bootstrapped = true;

        setupSearch();
        setupSearchModeButton();
        setupColumnSelector();
        initTabulator();
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
