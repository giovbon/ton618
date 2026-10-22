// tabulator-note.js — Editor de nota Tabulator (planilha de inserção de dados).
//
// Expõe window.initTabulatorNote(container, opts), que monta uma tabela
// Tabulator editável e devolve uma API para o template usar (serializar,
// adicionar linha/coluna, filtrar). O template cuida do autosave/rename/export.
//
// Recursos:
// - Editor de célula por tipo de coluna (texto/número/data/sim-não/arquivo).
// - Coluna "arquivo": clique na célula abre o seletor e envia o arquivo para
//   /api/tabulator/upload; a célula guarda o caminho (datasets/...) e mostra um
//   link para abrir/baixar + um "×" para remover.
// - Ordenação por clique no cabeçalho (sorter correto por tipo).
// - Filtro por coluna (input no cabeçalho de cada coluna, "contém").
// - Filtro rápido da tabela (caixa única, "contém" em todas as colunas).
// - Menu do cabeçalho: renomear coluna / tipo da coluna / excluir coluna.
//
// Ordenação e filtros são apenas VISUAIS: a ordem gravada no arquivo segue a
// ordem canônica das linhas (campo interno `_idx`), de modo que ordenar/filtrar
// para inspecionar nunca reordena nem apaga dados no disco.
//
// A biblioteca Tabulator (window.Tabulator) é carregada separadamente pelo
// template a partir de /static/tabulator.min.js — este bundle só a consome.
(function () {
    "use strict";

    // ── Helpers ──────────────────────────────────────────────────────────

    // Gera um field único e estável para uma coluna nova (nunca começa com "_",
    // prefixo reservado para campos internos: "_idx"/"_rownum"/"_actions").
    function genField() {
        return "c" + Date.now().toString(36) + Math.random().toString(36).slice(2, 6);
    }

    // Editor do Tabulator adequado ao tipo lógico da coluna. Colunas "arquivo"
    // não têm editor inline (o upload é tratado no clique da célula).
    function editorForType(type) {
        switch (type) {
            case "number": return "number";
            case "date": return "date";
            case "boolean": return "tickCross";
            case "file": return false;
            default: return "input";
        }
    }

    // Comparador (sorter) do Tabulator adequado ao tipo lógico da coluna.
    function sorterForType(type) {
        switch (type) {
            case "number": return "number";
            case "date": return "date";
            case "boolean": return "boolean";
            default: return "string";
        }
    }

    function isFileType(type) {
        return type === "file";
    }

    // Normaliza o texto digitado pelo usuário em um tipo lógico.
    function normalizeType(raw) {
        var s = String(raw || "").trim().toLowerCase();
        if (/^(num|núm|númer|numer)/.test(s)) return "number";
        if (/^(data|date)/.test(s)) return "date";
        if (/^(bool|sim|check|caixa)/.test(s)) return "boolean";
        if (/^(arq|file|anexo)/.test(s)) return "file";
        return "string";
    }

    function typeLabel(type) {
        switch (type) {
            case "number": return "número";
            case "date": return "data";
            case "boolean": return "booleano";
            case "file": return "arquivo";
            default: return "texto";
        }
    }

    // Ícones (feather) das opções do seletor de tipo. Inline para o bundle não
    // depender de nenhum outro arquivo.
    var TYPE_ICONS = {
        string: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="4 7 4 4 20 4 20 7"/><line x1="9" y1="20" x2="15" y2="20"/><line x1="12" y1="4" x2="12" y2="20"/></svg>',
        number: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="4" y1="9" x2="20" y2="9"/><line x1="4" y1="15" x2="20" y2="15"/><line x1="10" y1="3" x2="8" y2="21"/><line x1="16" y1="3" x2="14" y2="21"/></svg>',
        date: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="4" width="18" height="18" rx="2" ry="2"/><line x1="16" y1="2" x2="16" y2="6"/><line x1="8" y1="2" x2="8" y2="6"/><line x1="3" y1="10" x2="21" y2="10"/></svg>',
        boolean: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="9 11 12 14 22 4"/><path d="M21 12v7a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h11"/></svg>',
        file: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21.44 11.05l-9.19 9.19a6 6 0 0 1-8.49-8.49l9.19-9.19a4 4 0 0 1 5.66 5.66l-9.2 9.19a2 2 0 0 1-2.83-2.83l8.49-8.48"/></svg>'
    };

    // Tipos lógicos oferecidos no submenu "Tipo da coluna" (menu ⋮ do cabeçalho).
    // `value` é o que vai para o JSON da nota; a ordem é a de exibição.
    var TYPE_CHOICES = [
        { value: "string", label: "Texto", icon: TYPE_ICONS.string },
        { value: "number", label: "Número", icon: TYPE_ICONS.number },
        { value: "date", label: "Data", icon: TYPE_ICONS.date },
        { value: "boolean", label: "Sim/Não", icon: TYPE_ICONS.boolean },
        { value: "file", label: "Arquivo", icon: TYPE_ICONS.file }
    ];

    // SSOT em editor-common.js (o layout carrega antes deste bundle).
    var escapeHtml = EditorCommon.escapeHtml;

    // Converte o caminho guardado na célula em URL de download (servido por
    // /file/download, que valida o prefixo datasets/ no backend).
    function fileDownloadURL(path) {
        return "/file/download?name=" + encodeURIComponent(path).replace(/%2F/gi, "/");
    }

    function fileNameFromPath(path) {
        return String(path || "").split("/").pop();
    }

    // ── initTabulatorNote ────────────────────────────────────────────────

    // Executado pelo template (drawing.js/mindmap.js seguem o mesmo padrão de
    // expor a API de init em `window`).
    window.initTabulatorNote = function (container, opts) {
        opts = opts || {};
        var data = opts.initialData || { columns: [], rows: [] };
        var onChange = typeof opts.onChange === "function" ? opts.onChange : function () {};
        var onFilter = typeof opts.onFilter === "function" ? opts.onFilter : function () {};

        var colTypes = {};   // field -> tipo lógico
        var nextIdx = 0;     // próximo valor de _idx (ordem canônica das linhas)
        var table = null;
        var pendingFileCell = null; // célula aguardando o arquivo do seletor

        // Copia as linhas iniciais anexando _idx (ordem canônica = ordem do arquivo).
        var initialRows = (data.rows || []).map(function (r) {
            var row = {};
            Object.keys(r || {}).forEach(function (k) { row[k] = r[k]; });
            row._idx = nextIdx++;
            return row;
        });

        function dataColumns() {
            return table.getColumns().filter(function (c) {
                var f = c.getField();
                return f && String(f).charAt(0) !== "_";
            });
        }

        function headerMenu() {
            return [
                {
                    label: "Renomear coluna",
                    action: function (e, column) {
                        var cur = column.getDefinition().title;
                        var novo = window.prompt("Nome da coluna:", cur);
                        if (novo === null) return;
                        novo = novo.trim();
                        if (!novo) return;
                        column.updateDefinition({ title: novo });
                        onChange();
                    }
                },
                {
                    label: "Tipo da coluna",
                    menu: TYPE_CHOICES.map(typeMenuItem)
                },
                {
                    label: "Excluir coluna",
                    action: function (e, column) {
                        if (dataColumns().length <= 1) {
                            window.alert("A tabela precisa de pelo menos uma coluna.");
                            return;
                        }
                        if (!window.confirm('Excluir a coluna "' + column.getDefinition().title + '"?')) {
                            return;
                        }
                        column.delete().then(onChange);
                    }
                }
            ];
        }

        // ── Submenu "Tipo da coluna" ─────────────────────────────────────
        //
        // O item "Tipo da coluna" do menu ⋮ é um SUBMENU nativo do Tabulator
        // (`menu: [...]`): ao clicar, a lista de tipos abre AO LADO do item. O
        // tipo atual vem marcado (✓) e um clique aplica — nada de digitar.

        // Aplica o tipo à coluna: registra em colTypes e troca editor, sorter e
        // formatter conforme o tipo. Colunas "arquivo" não têm editor inline (o
        // upload acontece no clique da célula).
        function applyColumnType(column, type) {
            if (!column) return;
            var field = column.getField();
            var t = normalizeType(type);
            if ((colTypes[field] || "string") === t) return; // nada mudou
            colTypes[field] = t;
            if (isFileType(t)) {
                column.updateDefinition({
                    editor: false,
                    sorter: sorterForType(t),
                    formatter: fileCellFormatter,
                    cellClick: fileCellClick
                });
            } else {
                column.updateDefinition({
                    editor: editorForType(t),
                    sorter: sorterForType(t),
                    formatter: false,
                    cellClick: null
                });
            }
            onChange();
        }

        // Item do submenu para um tipo: o `label` do Tabulator aceita função, que
        // é reavaliada a cada abertura (por isso o ✓ acompanha o tipo atual).
        function typeMenuItem(choice) {
            return {
                label: function (column) {
                    var cur = "string";
                    if (column && typeof column.getField === "function") {
                        cur = colTypes[column.getField()] || "string";
                    }
                    var current = cur === choice.value;
                    return '<span class="tab-type-item' + (current ? " is-current" : "") + '">' +
                        '<span class="tab-type-ico" aria-hidden="true">' + choice.icon + "</span>" +
                        '<span class="tab-type-name">' + escapeHtml(choice.label) + "</span>" +
                        '<span class="tab-type-check" aria-hidden="true">' +
                        (current ? "✓" : "") + "</span>" +
                        "</span>";
                },
                action: function (e, column) {
                    applyColumnType(column, choice.value);
                }
            };
        }

        function dataColumnDef(title, field, type) {
            colTypes[field] = type || "string";
            var def = {
                title: title || field,
                field: field,
                editor: editorForType(type),
                sorter: sorterForType(type),
                headerSort: true,
                // Input de filtro no cabeçalho (default = "contém", case-insensitive).
                headerFilter: true,
                headerMenu: headerMenu(),
                minWidth: 120,
                widthGrow: 1
            };
            if (isFileType(type)) {
                def.formatter = fileCellFormatter;
                def.cellClick = fileCellClick;
            }
            return def;
        }

        function buildColumnDef(col) {
            return dataColumnDef(col.title || col.field, col.field, col.type);
        }

        // ── Coluna "arquivo" ─────────────────────────────────────────────

        function fileCellFormatter(cell) {
            var v = cell.getValue();
            var path = v === undefined || v === null ? "" : String(v).trim();
            if (!path) {
                return '<span class="tab-file-empty">+ enviar arquivo</span>';
            }
            // Valor que não foi gravado por esta coluna (ex: coluna convertida em
            // "arquivo" com texto solto): mostra como texto, sem link quebrado.
            if (path.indexOf("datasets/") !== 0) {
                return '<span class="tab-file-raw">' + escapeHtml(path) + "</span>";
            }
            var name = fileNameFromPath(path);
            return '<span class="tab-file-cell">' +
                '<a class="tab-file-link" href="' + escapeHtml(fileDownloadURL(path)) +
                '" target="_blank" rel="noopener" title="Abrir/baixar ' + escapeHtml(name) + '">' +
                escapeHtml(name) + "</a>" +
                '<button type="button" class="tab-file-del" title="Remover arquivo">&times;</button>' +
                "</span>";
        }

        function fileCellClick(e, cell) {
            var t = e.target;
            if (t && t.classList && t.classList.contains("tab-file-del")) {
                e.stopPropagation();
                cell.setValue("");
                onChange();
                return;
            }
            if (t && t.classList && t.classList.contains("tab-file-link")) {
                return; // deixa o link abrir/baixar o arquivo
            }
            openFilePicker(cell);
        }

        // Um único <input type="file"> escondido serve todas as células. O
        // template já o declara (#tab-file-input); criamos um se faltar.
        /** @type {HTMLInputElement|null} */
        var fileInputEl = null;
        var fileInputBound = false;

        function fileInput() {
            if (!fileInputEl) {
                fileInputEl = /** @type {HTMLInputElement|null} */ (document.getElementById("tab-file-input"));
            }
            if (!fileInputEl) {
                var el = document.createElement("input");
                el.type = "file";
                el.id = "tab-file-input";
                el.style.display = "none";
                document.body.appendChild(el);
                fileInputEl = el;
            }
            if (!fileInputBound) {
                fileInputBound = true;
                fileInputEl.addEventListener("change", function () {
                    var cell = pendingFileCell;
                    pendingFileCell = null;
                    var f = fileInputEl && fileInputEl.files ? fileInputEl.files[0] : null;
                    if (cell && f) uploadFileToCell(cell, f);
                });
            }
            return fileInputEl;
        }

        function openFilePicker(cell) {
            pendingFileCell = cell;
            var input = fileInput();
            input.value = ""; // permite reenviar o mesmo arquivo
            input.click();
        }

        function uploadFileToCell(cell, file) {
            var el = cell.getElement();
            if (el) el.classList.add("tab-file-uploading");

            var fd = new FormData();
            fd.append("file", file);

            var headers = (window.EditorCommon && window.EditorCommon.getAuthHeaders)
                ? window.EditorCommon.getAuthHeaders()
                : {};

            fetch("/api/tabulator/upload", { method: "POST", body: fd, headers: headers })
                .then(function (r) {
                    if (!r.ok) {
                        return r.text().then(function (t) { throw new Error(t || ("HTTP " + r.status)); });
                    }
                    return r.json();
                })
                .then(function (res) {
                    if (el) el.classList.remove("tab-file-uploading");
                    if (!res || !res.path) throw new Error("resposta inválida do servidor");
                    cell.setValue(res.path);
                    onChange();
                })
                .catch(function (err) {
                    if (el) el.classList.remove("tab-file-uploading");
                    window.alert("Falha ao enviar o arquivo: " + (err && err.message ? err.message : err));
                });
        }

        function rowNumColumn() {
            return {
                title: "#",
                field: "_rownum",
                width: 52,
                minWidth: 52,
                headerSort: false,
                resizable: false,
                hozAlign: "center",
                formatter: "rownum"
            };
        }

        function actionColumn() {
            return {
                title: "",
                field: "_actions",
                width: 44,
                minWidth: 44,
                headerSort: false,
                resizable: false,
                hozAlign: "center",
                formatter: function () {
                    return '<button type="button" class="tab-del-btn" title="Excluir registro">&times;</button>';
                },
                // Só o clique no botão exclui (evita apagar ao selecionar a célula).
                cellClick: function (e, cell) {
                    if (e.target && e.target.classList && e.target.classList.contains("tab-del-btn")) {
                        cell.getRow().delete().then(onChange);
                    }
                }
            };
        }

        function initialColumns() {
            var cols = [];
            cols.push(rowNumColumn());
            (data.columns || []).forEach(function (c) {
                if (c && c.field) cols.push(buildColumnDef(c));
            });
            cols.push(actionColumn());
            return cols;
        }

        table = new Tabulator(container, {
            data: initialRows,
            columns: initialColumns(),
            // fitColumns distribui a largura da tela entre as colunas de dados
            // (widthGrow: 1); # e a coluna de ações ficam fixas. Evita a faixa
            // vazia à direita que o fitDataFill deixava.
            layout: "fitColumns",
            height: "100%",
            movableColumns: true,
            editTriggerEvent: "click",
            headerFilterLiveFilter: true,
            placeholder: "Nenhum registro. Use “+ Novo registro” para começar.",
            columnDefaults: {
                headerHozAlign: "left",
                vertAlign: "middle"
            }
        });

        // Alterações de dados → autosave. Filtros/ordenação são só visuais.
        table.on("cellEdited", onChange);
        table.on("rowAdded", onChange);
        table.on("rowDeleted", onChange);
        table.on("columnMoved", onChange);
        // Mudanças de filtro/ordenação só atualizam contadores na UI.
        table.on("dataFiltered", onFilter);
        table.on("dataSorted", onFilter);

        function addRow(values) {
            var row = { _idx: nextIdx++ };
            dataColumns().forEach(function (c) { row[c.getField()] = ""; });
            if (values) {
                Object.keys(values).forEach(function (k) { row[k] = values[k]; });
            }
            return table.addRow(row).then(function () {
                onChange();
                return row;
            });
        }

        function addColumn(title, type) {
            return table.addColumn(dataColumnDef(title, genField(), type), false).then(function () {
                onChange();
            });
        }

        // Serializa o estado atual no formato persistido na nota.
        //
        // Usa table.getRows() (todas as linhas, ignorando filtros) e reordena por
        // _idx — assim ordenar/filtrar na tela não reordena nem descarta dados.
        // Só os campos de colunas entram; campos internos (_idx) ficam de fora.
        function serialize() {
            var cols = dataColumns().map(function (c) {
                var def = c.getDefinition();
                var field = c.getField();
                var col = { field: field, title: def.title || field };
                var t = colTypes[field];
                if (t && t !== "string") col.type = t;
                return col;
            });
            var fields = cols.map(function (c) { return c.field; });

            var allRows = table.getRows().map(function (rc) { return rc.getData(); });
            allRows.sort(function (a, b) { return (a._idx || 0) - (b._idx || 0); });

            var rows = allRows.map(function (r) {
                var out = {};
                fields.forEach(function (f) {
                    var v = r[f];
                    if (v !== undefined && v !== null) out[f] = v;
                });
                return out;
            });
            return { columns: cols, rows: rows };
        }

        // Filtro rápido da tabela: "contém" em qualquer coluna de dados.
        // Substitui o filtro programático anterior; filtros de cabeçalho e a
        // ordem canônica dos dados não são afetados.
        function setQuickFilter(q) {
            var needle = String(q || "").trim().toLowerCase();
            if (!needle) {
                table.clearFilter(); // limpa só o filtro programático (mantém os de cabeçalho)
                return;
            }
            var fields = dataColumns().map(function (c) { return c.getField(); });
            table.setFilter(function (row) {
                for (var i = 0; i < fields.length; i++) {
                    var v = row[fields[i]];
                    if (v !== undefined && v !== null &&
                        String(v).toLowerCase().indexOf(needle) > -1) {
                        return true;
                    }
                }
                return false;
            });
        }

        return {
            getData: serialize,
            addRow: addRow,
            addColumn: addColumn,
            setQuickFilter: setQuickFilter,
            getRowCount: function () { return table.getDataCount(); },
            getFilteredRowCount: function () { return table.getDataCount("active"); },
            getColumnCount: function () { return dataColumns().length; },
            redraw: function () { table.redraw(); }
        };
    };

    // Helpers puros expostos para testes unitários (node, sem DOM/Tabulator).
    window.TabulatorNoteUtils = {
        genField: genField,
        editorForType: editorForType,
        sorterForType: sorterForType,
        normalizeType: normalizeType,
        typeLabel: typeLabel,
        columnTypes: TYPE_CHOICES,
        isFileType: isFileType,
        escapeHtml: escapeHtml,
        fileDownloadURL: fileDownloadURL,
        fileNameFromPath: fileNameFromPath
    };
})();
