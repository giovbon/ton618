// editor-find.js — Painel de busca e substituição do editor (busca nativa TipTap)
// Usa a extensão oficial @tiptap/extension-find-and-replace, que já cuida do
// regex (sintaxe RE2: sem lookaround sem backreference), match de case,
// palavra inteira, highlight dos resultados e os comandos de navegação/replace.
//
// A UI é montada aqui em JS puro, ligada aos comandos/storage da extensão.
// O HTML do painel vive no template editor.templ; os estilos dos highlights em
// editor.templ também (a extensão roda com injectCSS: false).

// @ts-nocheck — código legado de script inline, tipagem dinâmica de DOM proposital

(function () {
    "use strict";

    // Elementos do painel (mapeados com lazy-get para não quebrar se ausentes)
    function $find() { return document.getElementById("find-bar"); }
    function $searchInput() { return document.getElementById("find-search"); }
    function $replaceInput() { return document.getElementById("find-replace"); }
    function $count() { return document.getElementById("find-count"); }
    function $caseChk() { return document.getElementById("find-case"); }
    function $regexChk() { return document.getElementById("find-regex"); }
    function $wordChk() { return document.getElementById("find-word"); }

    function getEditor() {
        var ed = window.__tonfind_getEditor && window.__tonfind_getEditor();
        return ed || null;
    }

    // Estado dos checkboxes espelhado no painel
    function setCheckbox($el, value) {
        if ($el && $el.checked !== !!value) $el.checked = !!value;
    }

    // Atualiza o contador "x de y" com base no storage da extensão
    function renderCount() {
        var editor = getEditor();
        var $c = $count();
        if (!$c || !editor) {
            if ($c) $c.textContent = "";
            return;
        }
        var storage = editor.storage.findAndReplace;
        if (!storage) return;
        var results = storage.results || [];
        if (results.length === 0) {
            var term = storage.searchTerm || "";
            $c.textContent = term ? "Sem resultados" : "";
            $c.classList.toggle("text-amber-400", !!term && results.length === 0);
        } else {
            var cur = (storage.currentIndex ?? 0) + 1;
            $c.textContent = cur + " de " + results.length;
            $c.classList.remove("text-amber-400");
        }
    }

    // Sincroniza o painel inteiro a partir do storage (toggles + contador).
    // Chamado após cada mudança relevante.
    function syncFromStorage() {
        var editor = getEditor();
        if (!editor) return;
        var s = editor.storage.findAndReplace;
        if (!s) return;
        setCheckbox($caseChk(), s.caseSensitive);
        setCheckbox($regexChk(), s.useRegex);
        // Palavra inteira fica desabilitada quando regex está ativo (igual
        // ao comportamento da extensão).
        setCheckbox($wordChk(), s.wholeWord);
        var $w = $wordChk();
        if ($w) $w.disabled = !!s.useRegex;
        renderCount();
    }

    // ── Ações principais ──
    function onSearchChange(value) {
        var editor = getEditor();
        if (!editor) return;
        editor.commands.setSearchTerm(value);
        syncFromStorage();
    }
    function onReplaceChange(value) {
        var editor = getEditor();
        if (!editor) return;
        editor.commands.setReplaceTerm(value);
        syncFromStorage();
    }
    function onToggleCase() {
        var editor = getEditor();
        if (editor) editor.commands.setCaseSensitive($caseChk().checked);
        syncFromStorage();
    }
    function onToggleRegex() {
        var editor = getEditor();
        if (editor) editor.commands.setUseRegex($regexChk().checked);
        syncFromStorage();
    }
    function onToggleWord() {
        var editor = getEditor();
        if (editor) editor.commands.setWholeWord($wordChk().checked);
        syncFromStorage();
    }
    function next() { var e = getEditor(); if (e) e.commands.goToNextResult(); }
    function prev() { var e = getEditor(); if (e) e.commands.goToPreviousResult(); }
    function replace() { var e = getEditor(); if (e) e.commands.replace(); }
    function replaceAll() { var e = getEditor(); if (e) e.commands.replaceAll(); }
    function clearSearch() {
        var e = getEditor();
        if (e) e.commands.clearSearch();
        var si = $searchInput();
        if (si) si.value = "";
        renderCount();
    }

    // ── Abrir/fechar painel ──
    function open(focusReplace) {
        var bar = $find();
        if (!bar) return;
        bar.classList.remove("hidden");
        var editor = getEditor();
        // Move o cursor do editor para o início para navegação linear
        var input = focusReplace ? $replaceInput() : $searchInput();
        if (input) {
            input.focus();
            input.select();
        }
        if (editor) editor.commands.focus();
    }
    function close() {
        var bar = $find();
        if (bar) bar.classList.add("hidden");
        var e = getEditor();
        if (e) e.commands.clearSearch();
        var si = $searchInput();
        if (si) si.value = "";
        renderCount();
    }
    function isOpen() {
        var bar = $find();
        return bar ? !bar.classList.contains("hidden") : false;
    }

    // ── Wiring de eventos (delegação global, padrão do projeto) ──
    // Search input
    var searchEl = $searchInput();
    if (searchEl) {
        searchEl.addEventListener("input", function (e) {
            onSearchChange(e.target.value);
        });
        searchEl.addEventListener("keydown", function (e) {
            if (e.key === "Enter") {
                e.preventDefault();
                if (e.shiftKey) prev(); else next();
            } else if (e.key === "Escape") {
                e.preventDefault();
                close();
            }
        });
    }
    // Replace input (Enter aplica replace e vai pro próximo)
    var replaceEl = $replaceInput();
    if (replaceEl) {
        replaceEl.addEventListener("input", function (e) {
            onReplaceChange(e.target.value);
        });
        replaceEl.addEventListener("keydown", function (e) {
            if (e.key === "Enter") {
                e.preventDefault();
                if (e.shiftKey) { prev(); } else { replace(); }
            } else if (e.key === "Escape") {
                e.preventDefault();
                close();
            }
        });
    }

    // Botões (por delegação de clique no painel; os data-action mapeiam a ação)
    var bar = $find();
    if (bar) {
        bar.addEventListener("click", function (e) {
            var btn = e.target.closest("[data-find-action]");
            if (!btn) return;
            e.preventDefault();
            var action = btn.getAttribute("data-find-action");
            switch (action) {
                case "prev": prev(); break;
                case "next": next(); break;
                case "replace": replace(); break;
                case "replace-all": replaceAll(); break;
                case "clear": clearSearch(); break;
                case "close": close(); break;
            }
        });
    }

    // Toggles (clique direto nos checkbox)
    var caseEl = $caseChk(); if (caseEl) caseEl.addEventListener("change", onToggleCase);
    var regexEl = $regexChk(); if (regexEl) regexEl.addEventListener("change", onToggleRegex);
    var wordEl = $wordChk(); if (wordEl) wordEl.addEventListener("change", onToggleWord);

    // Atalhos de teclado globais: Ctrl/Cmd+F, Ctrl/Cmd+H, F3/Shift+F3, Esc
    document.addEventListener("keydown", function (e) {
        var mod = e.ctrlKey || e.metaKey;
        var barOpen = isOpen();
        var targetInPanel = barOpen && bar && bar.contains(e.target);

        // Ctrl/Cmd+F → abre busca; Ctrl/Cmd+H → abre substituição
        if (mod && !e.shiftKey && (e.key === "f" || e.key === "F")) {
            e.preventDefault();
            open(false);
            return;
        }
        if (mod && !e.shiftKey && (e.key === "h" || e.key === "H")) {
            e.preventDefault();
            open(true);
            return;
        }
        // F3 / Shift+F3 → próximo / anterior
        if (e.key === "F3") {
            if (barOpen) {
                e.preventDefault();
                if (e.shiftKey) prev(); else next();
            }
            return;
        }
        // Escape fecha se o foco estiver no painel (o editor lida com o próprio
        // Esc; se o painel está aberto mas o foco voltou ao doc, um primeiro Esc
        // fecha o painel na próxima tecla).
        if (e.key === "Escape" && barOpen && (document.activeElement === document.body || targetInPanel)) {
            e.preventDefault();
            close();
        }
    });

    // API exposta para o editor-init.js consumir. O `sync()` é chamado pelo
    // editor a cada transaction/update para manter o contador e os toggles em
    // dia (ex.: após replace/replace-all/limpar).
    if (!window.TonFind) {
        window.TonFind = {
            open: open,
            close: close,
            isOpen: isOpen,
            sync: syncFromStorage,
        };
    }
})();
