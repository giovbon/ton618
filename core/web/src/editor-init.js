// editor-init.js — Inicialização do editor TipTap TON-618
// Extraído de internal/features/notes/editor.templ para melhor organização
// @ts-nocheck — Código legado de script inline, tipagem dinâmica de DOM proposital

import { initSlash, buildSlashActions, SLASH_COMMANDS, scoreSlashCommand } from './editor-slash.js';
import { updateToc, applyToc } from './editor-toc.js';
import { initFrontmatter } from './editor-frontmatter.js';
import { initHierarchy } from './editor-hierarchy.js';

(function () {
    "use strict";
    var T = window.TipTapEditor;
    var el = /** @type {HTMLElement} */(document.getElementById("editor-content"));
    var statusEl = /** @type {HTMLElement} */(document.getElementById("editor-status"));
    var filenameInput = /** @type {HTMLInputElement} */(document.getElementById("file-name"));
    var bubbleEl = /** @type {HTMLElement} */(document.getElementById("bubble-menu"));
    var tableBubbleEl = /** @type {HTMLElement} */(document.getElementById("table-bubble-menu"));
    var slashEl = /** @type {HTMLElement} */(document.getElementById("slash-menu"));
    var slashItems = /** @type {HTMLElement} */(document.getElementById("slash-items"));

    var raw = /** @type {HTMLTextAreaElement} */(document.getElementById("content-output")).value;
    var saveTimer = null;
    var currentStatus = "saved";
    var editor = null;
    var originalFilename = filenameInput.dataset.filename;
    var originalDisplayName = filenameInput.value.replace(/\.md$/, "");
    var slashPos = null;
    var frontmatterText = "";
    var FRONTMATTER_REGEX = /^---\r?\n([\s\S]*?)\r?\n---\r?\n?([\s\S]*)$/;
    var autoDetectTimer = null;
    var isInitialLoad = true;
    var lastSavedHash = null;
    var _scrollTimer = null;
    var _startTimer = null;

    // generateHash via EditorCommon (lida com crypto.subtle em HTTP)
    var generateHash = EditorCommon.generateHash;

    // ── Parse frontmatter do raw content ──
    var bodyContent = raw;
    var fmMatch = raw.match(FRONTMATTER_REGEX);
    if (fmMatch) {
        frontmatterText = fmMatch[1];
        bodyContent = fmMatch[2];
    }
    // Notas antigas podem ter o placeholder "&nbsp;" gravado no meio do texto
    // (ver stripNbspParagraphs): limpa já na abertura, para o lixo não reaparecer
    // na tela antes do primeiro save.
    bodyContent = EditorCommon.stripNbspParagraphs(bodyContent);

    // ── Painel de frontmatter ──
    // Toggle, autosize e sugestões de tags vivem em editor-frontmatter.js.
    // O onChange marca a nota como suja e agenda o save (inclusive ao clicar
    // numa tag sugerida, que dispara um "input" sintético).
    var frontmatter = initFrontmatter({
        onChange: function () {
            if (isInitialLoad) return;
            setStatus("dirty");
            if (saveTimer) clearTimeout(saveTimer);
            saveTimer = setTimeout(doSave, 2000);
        }
    });

    // ── Helper: limpa timers pendentes ──
    function clearAllTimers() {
        if (saveTimer) { clearTimeout(saveTimer); saveTimer = null; }
        if (autoDetectTimer) { clearTimeout(autoDetectTimer); autoDetectTimer = null; }
        if (tocUpdateTimer) { clearTimeout(tocUpdateTimer); tocUpdateTimer = null; }
        if (_scrollTimer) { clearInterval(_scrollTimer); _scrollTimer = null; }
        if (_startTimer) { clearInterval(_startTimer); _startTimer = null; }
    }

    // ── Slash commands ──
    var slashFilterText = "";

    initSlash({
        getEditor: function() { return editor; },
        getSlashPos: function() { return slashPos; },
        setSlashPos: function(p) { slashPos = p; },
        getSlashFilter: function() { return slashFilterText; },
        setSlashFilter: function(t) { slashFilterText = t; },
        hideSlashMenu: hideSlashMenu,
        organizeHeadings: organizeHeadings
    });
    buildSlashActions();

    // ── Wikilink Autocomplete ──
    var wikiNotes = [];
    var wikiVisible = false;
    var wikiSelected = 0;

    function loadWikiNotes() {
        fetch("/api/notes")
            .then(function (r) {
                return r.json();
            })
            .then(function (data) {
                wikiNotes = (data.notes || []).map(function (n) {
                    return {
                        fullPath: n.arquivo,
                        label: (
                            n.arquivo.split("/").pop() || n.arquivo
                        ).replace(/\.md$/i, ""),
                        tags: n.tags || [],
                    };
                });
            })
            .catch(function () {});
    }

    function showWikiMenu(filter) {
        var f = filter.toLowerCase();
        var items = wikiNotes.filter(function (n) {
            return n.label.toLowerCase().indexOf(f) !== -1;
        });
        var el = document.getElementById("wikilink-menu");
        var list = document.getElementById("wikilink-items");
        if (items.length === 0) {
            el.classList.add("hidden");
            wikiVisible = false;
            return;
        }
        wikiSelected = 0;
        var html = "";
        items.forEach(function (item, i) {
            var name = item.label;
            var idx = name.toLowerCase().indexOf(f);
            var before = htmlEscape(name.substring(0, idx));
            var match = htmlEscape(name.substring(idx, idx + f.length));
            var after = htmlEscape(name.substring(idx + f.length));
            html +=
                '<button class="wikilink-item' +
                (i === 0 ? " active" : "") +
                '" data-index="' +
                i +
                '" data-wiki-name="' +
                htmlEscape(name) +
                '" data-wiki-path="' +
                htmlEscape(item.fullPath) +
                '">';
            html += '<span class="wiki-icon">[[]]</span>';
            html +=
                "<span>" +
                before +
                "<strong>" +
                match +
                "</strong>" +
                after +
                "</span>";
            html += "</button>";
        });
        list.innerHTML = html;
        el.classList.remove("hidden");
        wikiVisible = true;
        var sel = window.getSelection();
        if (sel.rangeCount) {
            var range = sel.getRangeAt(0);
            var rect = range.getBoundingClientRect();
            var left = rect.left - 10;
            var mh = el.offsetHeight || 240;
            var spaceBelow = window.innerHeight - rect.bottom - 8;
            var top;
            if (spaceBelow < mh && rect.top > mh + 8) {
                top = rect.top - mh - 4;
            } else {
                top = rect.bottom + 4;
            }
            el.style.left = left + "px";
            el.style.top = top + "px";
        }
    }

    function hideWikiMenu() {
        document.getElementById("wikilink-menu").classList.add("hidden");
        wikiVisible = false;
    }

    // Click delegation for wikilink items (evita escopo global do onclick)
    document
        .getElementById("wikilink-items")
        .addEventListener("click", function (e) {
            var btn = (/** @type {Element} */(e.target)).closest(".wikilink-item");
            if (btn) {
                var name = btn.getAttribute("data-wiki-name");
                if (name) selectWiki(name);
            }
        });

    function selectWiki(name) {
        hideWikiMenu();
        if (!editor) return;

        // Get full path from the closest .wikilink-item
        var btn = document.querySelector(".wikilink-item.active");
        var fullPath = btn ? btn.getAttribute("data-wiki-path") : "";
        if (!fullPath) {
            fullPath = "notes/" + name + ".md";
        }

        // Build href based on file type
        var ext = fullPath.split(".").pop().toLowerCase();
        var href;
        if (ext === "pdf") {
            href =
                "/file/download?name=" + encodeURIComponent("pdfs/" + name);
        } else if (ext === "zip") {
            href =
                "/file/download?name=" +
                encodeURIComponent("attachments/" + name);
        } else if (ext === "epub") {
            href = "/epub/reader?file=" + encodeURIComponent(fullPath || "epubs/" + name + ".epub");
        } else {
            href = "/editor?file=notes/" + encodeURIComponent(name) + ".md";
        }

        var $from = editor.state.selection.$from;
        var textBefore = $from.parent.textBetween(0, $from.parentOffset);
        var bracketPos = textBefore.lastIndexOf("[[");
        if (bracketPos !== -1) {
            var deleteFrom = $from.pos - (textBefore.length - bracketPos);
            var deleteTo = $from.pos;
            editor
                .chain()
                .focus()
                .deleteRange({ from: deleteFrom, to: deleteTo })
                .setLink({ href: href })
                .insertContent(name)
                .run();
        } else {
            editor
                .chain()
                .focus()
                .setLink({ href: href })
                .insertContent(name)
                .run();
        }
    }

    // ── Auto-detecção de linguagem em code blocks ──
    function autoDetectCodeLanguage() {
        if (!editor || editor.isDestroyed) return;
        var doc = editor.state.doc;
        var found = false;
        doc.descendants(function (node, pos) {
            if (found) return;
            if (node.type.name === "codeBlock") {
                var lang = node.attrs.language;
                if (lang && lang !== "auto") return;
                var codeText = node.textContent;
                if (!codeText || codeText.length < 5) return;
                try {
                    var result = T.lowlight.highlightAuto(codeText);
                    if (result && result.language) {
                        // Aplica sem mover o cursor do usuário
                        var tr = editor.state.tr;
                        tr.setNodeMarkup(pos, null, {
                            language: result.language,
                        });
                        editor.view.dispatch(tr);
                        found = true;
                    }
                } catch (e) {
                    // Silencia erros de detecção
                }
            }
        });
    }

    // ── Organizar títulos (corrige hierarquia bagunçada de colagens de IA) ──
    function organizeHeadings() {
        if (!editor || editor.isDestroyed) return;
        var headings = [];
        var doc = editor.state.doc;
        doc.descendants(function (node, pos) {
            if (node.type.name === "heading") {
                headings.push({
                    pos: pos,
                    level: node.attrs.level,
                    newLevel: node.attrs.level,
                });
            }
        });
        if (headings.length < 2) return;
        var baseLevel = headings[0].level;
        var tr = editor.state.tr;
        var modified = false;
        for (var i = 0; i < headings.length; i++) {
            var h = headings[i];
            if (i > 0) {
                var prev = headings[i - 1];
                var origLevel = h.level;
                if (origLevel <= baseLevel) {
                    // Mesmo nível ou acima → nova seção no nível base
                    h.newLevel = baseLevel;
                } else {
                    // Sub-título: calcula profundidade relativa ao base
                    var depth = origLevel - baseLevel;
                    var suggested = baseLevel + depth;
                    // Evita saltos (ex: h2 → h4 vira h2 → h3)
                    if (suggested > prev.newLevel + 1) {
                        suggested = prev.newLevel + 1;
                    }
                    h.newLevel = Math.min(suggested, 6);
                }
                if (h.newLevel !== h.level) {
                    tr.setNodeMarkup(h.pos, null, {
                        level: h.newLevel,
                    });
                    modified = true;
                }
            }
        }
        if (modified) {
            editor.view.dispatch(tr);
        }
    }

    // ── Converte [[wikilinks]] no HTML do editor para links clicáveis ──
    function convertWikilinksToLinks(html) {
        return html.replace(/\[\[([^\]]+)\]\]/g, function (match, name) {
            var target = name.trim();
            var ext = target.includes(".")
                ? target.split(".").pop().toLowerCase()
                : "";
            var isMd = ext === "" || ext === "md" || ext === "markdown";
            if (isMd) {
                var noteName = target;
                if (!noteName.endsWith(".md")) noteName += ".md";
                return '<a href="/editor?file=notes/' +
                    encodeURIComponent(noteName) +
                    '" class="wikish-link">' + target + "</a>";
            } else if (ext === "epub") {
                var path = target.startsWith("epubs/") ? target : "epubs/" + target;
                return '<a href="/epub/reader?file=' +
                    encodeURIComponent(path) +
                    '" class="wikish-link">' + target + "</a>";
            } else {
                var dir = ext === "pdf" ? "pdfs" : "attachments";
                return '<a href="/file/download?name=' +
                    encodeURIComponent(dir + "/" + target) +
                    '" class="wikish-link" download>' + target + "</a>";
            }
        });
    }

    // ── Editor ──
    editor = new T.Editor({
        element: el,
        extensions: [
            T.StarterKit.configure({
                heading: { levels: [1, 2, 3, 4, 5, 6] },
                codeBlock: false,
            }),
            T.Placeholder.configure({
                placeholder: "Pressione / para comandos...",
            }),
            T.Table.configure({ resizable: true }),
            T.TableRow,
            T.TableCell,
            T.TableHeader,
            T.TaskList,
            T.TaskItem.configure({ nested: true }),
            T.Underline,
            T.Highlight,
            T.ImageExt,
            T.Link,
            T.Markdown.configure({
                transformPastedText: true,
                transformCopiedText: false,
            }),
            T.CodeBlockLowlightExt.configure({
                lowlight: T.lowlight,
                defaultLanguage: null,
            }),
        ],
        content: bodyContent || "<p></p>",
        onSelectionUpdate: function () {
            if (!slashEl.classList.contains("hidden")) return;
            updateBubble();
            updateTableBubble();
        },
        onTransaction: function (props) {
            // Captura mudanças de documento que onUpdate pode perder (delete de nó, paste sem Enter)
            if (isInitialLoad) return;
            if (!props.transaction.docChanged) return;
            if (!editor || editor.isDestroyed) return;
            setStatus("dirty");
            if (saveTimer) clearTimeout(saveTimer);
            saveTimer = setTimeout(doSave, 2000);
        },
        onUpdate: function () {
            // Pula o primeiro onUpdate (disparado ao carregar o conteudo)
            if (isInitialLoad) return;
            // Proteção contra destroy durante update
            if (!editor || editor.isDestroyed) return;
            setStatus("dirty");
            if (saveTimer) clearTimeout(saveTimer);
            saveTimer = setTimeout(doSave, 2000);
            // Auto-detecção de linguagem em code blocks (debounced)
            if (autoDetectTimer) clearTimeout(autoDetectTimer);
            autoDetectTimer = setTimeout(autoDetectCodeLanguage, 1500);
            // TOC update
            if (tocVisible) {
                if (tocUpdateTimer) clearTimeout(tocUpdateTimer);
                tocUpdateTimer = setTimeout(doUpdateToc, 300);
            }
            // Wikilink detection (também funciona sem keydown, ex: mobile)
            if (editor) {
                var $from = editor.state.selection.$from;
                var textBefore = $from.parent.textBetween(
                    0,
                    $from.parentOffset,
                );
                var bracketPos = textBefore.lastIndexOf("[[");
                if (bracketPos !== -1) {
                    if (!wikiVisible) {
                        wikiVisible = true;
                    }
                    var filterText = textBefore.substring(bracketPos + 2);
                    showWikiMenu(filterText);
                } else if (wikiVisible) {
                    hideWikiMenu();
                }
            }
            // Mobile fallback: detectar / sem keydown (teclados virtuais)
            if (editor && slashPos === null && slashEl.classList.contains("hidden")) {
                var $sl = editor.state.selection.$from;
                var txtBefore = $sl.parent.textBetween(0, $sl.parentOffset);
                if (txtBefore === "/") {
                    slashPos = $sl.pos - 1;
                    showSlashMenu(editor.view);
                    slashFilterText = "";
                }
            }

            // Filtragem do slash menu enquanto digita
            if (
                !slashEl.classList.contains("hidden") &&
                slashPos !== null
            ) {
                var $sel = editor.state.selection.$from;
                if ($sel.pos <= slashPos) {
                    // Cursor antes ou no slash → fecha menu
                    hideSlashMenu();
                    slashPos = null;
                    slashFilterText = "";
                } else {
                    var typed = editor.state.doc.textBetween(
                        slashPos,
                        $sel.pos,
                    );
                    if (!typed.startsWith("/")) {
                        // Barra foi apagada → fecha menu
                        hideSlashMenu();
                        slashPos = null;
                        slashFilterText = "";
                    } else {
                        slashFilterText = typed.slice(1);
                        renderSlashItems(slashFilterText);
                    }
                }
            }
        },
        editorProps: {
            attributes: { spellcheck: "false" },
            handleClick: function (view, pos, event) {
                // Navega em links na mesma aba
                var target = event.target;
                if (target && target.tagName === "A" && target.href) {
                    // Ignora cliques com Ctrl/Meta (navegador ja abre em nova aba)
                    if (!event.ctrlKey && !event.metaKey && !event.shiftKey) {
                        // So navega se for link interno do editor
                        if (target.href.indexOf("/editor?file=") !== -1 || target.href.indexOf("/epub/reader") !== -1 || target.href.indexOf("/file/") !== -1) {
                            event.preventDefault();
                            window.location.href = target.href;
                            return true;
                        }
                    }
                }
                return false;
            },
            handlePaste: function (view, event) {
                var items =
                    event.clipboardData && event.clipboardData.items;
                if (!items) return false;
                for (var i = 0; i < items.length; i++) {
                    if (items[i].type.indexOf("image") === 0) {
                        event.preventDefault();
                        var file = items[i].getAsFile();
                        if (!file) return true;
                        uploadAndInsertImage(file);
                        return true;
                    }
                }
                var text = event.clipboardData.getData("text/plain");
                var html = event.clipboardData.getData("text/html");
                // Shift+colar = texto puro (convenção do navegador): deixa o
                // ProseMirror inserir o texto cru, sem interpretar nada.
                if (event.shiftKey) return false;
                if (!editor || editor.isActive("codeBlock")) return false;

                // A decisão fica no EditorCommon (testável em node): só converte o
                // texto puro quando ele tem markdown E o HTML não tem formatação
                // real (evita "HTML de fachada" com **asteriscos** crus).
                var isMarkdown = EditorCommon.shouldPasteAsMarkdown(text, html);
                var source = isMarkdown ? T.marked.parse(text) : html;

                if (!source) return false;

                // Remove espaço vazio vindo do clipboard (blocos vazios, <br><br>,
                // listas loose) — era o que criava linhas em branco entre parágrafos.
                var clean = EditorCommon.normalizePastedHtml(source);

                // HTML que já está limpo segue o fluxo normal do ProseMirror
                // (nenhuma mudança de comportamento quando não há nada a limpar).
                if (!isMarkdown && clean === source) return false;

                event.preventDefault();
                editor.chain().focus().insertContent(clean).run();
                return true;
            },
            handleKeyDown: function (view, event) {
                // Tab dentro de code block: insere indentação
                if (event.key === "Tab" && view.state.selection.$from.parent.type.name === "codeBlock") {
                    event.preventDefault();
                    view.dispatch(view.state.tr.insertText("    "));
                    return true;
                }
                if (
                    event.key === "Backspace" &&
                    !slashEl.classList.contains("hidden")
                ) {
                    // Deixa o onUpdate refiltrar; se o texto acabar, o menu some
                    return false;
                }
                if (event.key === "/" && !event.ctrlKey && !event.metaKey) {
                    var $from = view.state.selection.$from;
                    var textBefore = $from.parent.textBetween(
                        0,
                        $from.parentOffset,
                    );
                    if (textBefore === "") {
                        slashPos = view.state.selection.from;
                        setTimeout(function () {
                            showSlashMenu(view);
                        }, 10);
                        return false;
                    }
                }
                // Wikilink detection: [[ typing
                if (event.key === "[" && !event.ctrlKey && !event.metaKey) {
                    var $from = view.state.selection.$from;
                    var textBefore = $from.parent.textBetween(
                        0,
                        $from.parentOffset,
                    );
                    if (
                        textBefore.length >= 1 &&
                        textBefore.slice(-1) === "["
                    ) {
                        wikiVisible = true;
                        setTimeout(function () {
                            showWikiMenu("");
                        }, 10);
                        return false;
                    }
                }
                if (wikiVisible) {
                    if (event.key === "ArrowDown") {
                        event.preventDefault();
                        var items =
                            document.querySelectorAll(".wikilink-item");
                        if (items.length) {
                            wikiSelected = Math.min(
                                wikiSelected + 1,
                                items.length - 1,
                            );
                            items.forEach(function (el, i) {
                                el.classList.toggle(
                                    "active",
                                    i === wikiSelected,
                                );
                            });
                        }
                        return true;
                    }
                    if (event.key === "ArrowUp") {
                        event.preventDefault();
                        var items =
                            document.querySelectorAll(".wikilink-item");
                        if (items.length) {
                            wikiSelected = Math.max(wikiSelected - 1, 0);
                            items.forEach(function (el, i) {
                                el.classList.toggle(
                                    "active",
                                    i === wikiSelected,
                                );
                            });
                        }
                        return true;
                    }
                    if (event.key === "Enter" || event.key === "Tab") {
                        event.preventDefault();
                        var items =
                            document.querySelectorAll(".wikilink-item");
                        if (items[wikiSelected])
                            (/** @type {HTMLElement} */(items[wikiSelected])).click();
                        return true;
                    }
                    if (event.key === "Escape") {
                        hideWikiMenu();
                        return true;
                    }
                }
                if (event.key === "Escape") {
                    hideSlashMenu();
                    hideBubble();
                    slashPos = null;
                    slashFilterText = "";
                }
                return false;
            },
        },
        onCreate: function () {
            // Set frontmatter textarea after editor is created
            if (frontmatterText) frontmatter.setValue(frontmatterText);

            // Converte wikilinks existentes e calcula hash inicial em sequência
            (function initSequence() {
                // 1. Auto-detectar linguagem em code blocks existentes
                setTimeout(function () {
                    autoDetectCodeLanguage();
                }, 500);

                // 2. Calc results in existing calc blocks
                setTimeout(function () {
                    if (typeof window.updateCalcResults === 'function') window.updateCalcResults();
                }, 800);

                // 3. Convert existing [[wikilinks]] to clickable links + hash inicial
                setTimeout(function () {
                    if (!editor) return;
                    var html = editor.getHTML();
                    var newHtml = convertWikilinksToLinks(html);
                    if (newHtml !== html) {
                        editor.commands.setContent(newHtml);
                    }
                    // Calcula hash inicial e garante isInitialLoad = false
                    computeInitialHash().then(function () {
                        isInitialLoad = false;
                    });
                }, 100);
            })();
        },
    });

    // ── Calcula hash inicial do conteúdo (extraído para evitar race condition) ──
    async function computeInitialHash() {
        if (!editor) return;
        var md = "";
        try { md = editor.storage.markdown.getMarkdown(); } catch (e) { md = editor.getHTML(); }
        md = EditorCommon.stripNbspParagraphs(md);
        var fm = document.getElementById("frontmatter-area").value.trim();
        var finalContent = fm ? "---\n" + fm + "\n---\n" + md : md;
        finalContent = EditorCommon.wikilinksToMarkdown(finalContent);
        lastSavedHash = await generateHash(finalContent);
    }

    // ── Limpeza no destroy ──
    editor.on('destroy', function () {
        clearAllTimers();
    });

    // ── Scroll to text or line from URL (ex: #line-42 or ?text=...) ──
    (function () {
        var hash = window.location.hash;
        var urlParams = new URLSearchParams(window.location.search);
        var searchText = urlParams.get("text");
        var lineNum = hash && hash.startsWith("#line-") ? parseInt(hash.replace("#line-", ""), 10) : null;
        if (!searchText && (!lineNum || isNaN(lineNum) || lineNum < 1)) return;

        // Espera o editor renderizar e o onCreate terminar
        var scrollAttempts = 0;
        _scrollTimer = setInterval(function () {
            scrollAttempts++;
            if (!editor || editor.isDestroyed) {
                clearInterval(_scrollTimer);
                _scrollTimer = null;
                return;
            }

            var doc = editor.state.doc;
            if (!doc || doc.content.size <= 2) {
                if (scrollAttempts > 25) { clearInterval(_scrollTimer); _scrollTimer = null; }
                return;
            }

            var targetPos = -1;

            // 1. Tenta buscar pelo texto exato
            if (searchText) {
                var cleanSearch = searchText.trim().toLowerCase();
                cleanSearch = cleanSearch.replace(/\*\*/g, '').replace(/\*/g, '').replace(/__/g, '').replace(/_/g, '').replace(/~~/g, '').replace(/`/g, '');
                doc.descendants(function (node, pos) {
                    if (targetPos !== -1) return false;
                    if (["paragraph", "taskItem", "heading", "listItem", "codeBlock"].includes(node.type.name)) {
                        var nodeText = node.textContent.toLowerCase();
                        if (nodeText.includes(cleanSearch)) {
                            targetPos = pos;
                        }
                    }
                });
            }

            var shouldScroll = targetPos !== -1;
            var forceFallback = !shouldScroll && scrollAttempts >= 15;

            if (forceFallback && lineNum && !isNaN(lineNum)) {
                var rawContent = (/** @type {HTMLTextAreaElement} */(document.getElementById("content-output"))).value;
                if (rawContent) {
                    var lines = rawContent.split("\n");
                    if (lineNum >= 1 && lineNum <= lines.length) {
                        var targetLineText = lines[lineNum - 1].trim();
                        var fmEnd = 0;
                        if (lines.length > 0 && lines[0].trim() === "---") {
                            for (var fi = 1; fi < lines.length; fi++) {
                                if (lines[fi].trim() === "---") {
                                    fmEnd = fi + 1;
                                    break;
                                }
                            }
                        }
                        if (!(fmEnd > 0 && lineNum <= fmEnd) && targetLineText) {
                            var lineSearch = targetLineText.toLowerCase();
                            lineSearch = lineSearch.replace(/^(todo|fixme|bug|task|note|hack|optimize|xxx|warning|change|review):\s*/i, '');
                            lineSearch = lineSearch.replace(/\*\*/g, '').replace(/\*/g, '').replace(/__/g, '').replace(/_/g, '').replace(/~~/g, '').replace(/`/g, '');
                            doc.descendants(function (node, pos) {
                                if (targetPos !== -1) return false;
                                var nodeText = node.textContent.toLowerCase();
                                if (nodeText.includes(lineSearch)) {
                                    targetPos = pos;
                                }
                                return undefined;
                            });
                        }
                    }
                }
                if (targetPos === -1) {
                    var currentLine = 0;
                    doc.forEach(function (node, offset) {
                        if (currentLine < lineNum - 1) {
                            targetPos = offset + 1;
                            currentLine++;
                        }
                    });
                }
                shouldScroll = targetPos !== -1;
            }

            if (shouldScroll) {
                clearInterval(_scrollTimer);
                _scrollTimer = null;
                try {
                    try {
                        var coords = editor.view.coordsAtPos(targetPos);
                        if (coords && typeof coords.top === 'number') {
                            var targetY = coords.top + window.scrollY - (window.innerHeight / 2);
                            if (targetY >= 0) {
                                window.scrollTo({ top: targetY, behavior: "smooth" });
                            }
                        }
                    } catch (e2) {}

                    setTimeout(function () {
                        var sel = window.getSelection();
                        if (sel && sel.rangeCount > 0) {
                            var range = sel.getRangeAt(0);
                            var highlightNode = range.startContainer;
                            if (highlightNode) {
                                var hNode = /** @type {HTMLElement} */(highlightNode.nodeType === 3
                                    ? (/** @type {Text} */(highlightNode)).parentElement
                                    : highlightNode);
                                var hEl = hNode;
                                while (hEl && hEl.classList && !hEl.classList.contains('ProseMirror')
                                    && hEl.parentElement && !hEl.parentElement.classList.contains('ProseMirror')) {
                                    hEl = hEl.parentElement;
                                }
                                if (hEl && hEl.style) {
                                    hEl.style.transition = "background-color 0.5s";
                                    hEl.style.backgroundColor = "rgba(245, 158, 11, 0.25)";
                                    setTimeout(function () {
                                        hEl.style.backgroundColor = "";
                                    }, 2500);
                                }
                            }
                        }
                    }, 100);
                } catch (e) {}
            } else if (scrollAttempts > 25) {
                clearInterval(_scrollTimer);
                _scrollTimer = null;
            }
        }, 100);
    })();

    // Se nao tem hash #line- e nem texto de busca, vai para o inicio do documento
    (function () {
        var hash = window.location.hash;
        var urlParams = new URLSearchParams(window.location.search);
        if ((hash && hash.startsWith("#line-")) || urlParams.get("text")) return;
        var attempts = 0;
        _startTimer = setInterval(function () {
            attempts++;
            if (!editor || editor.isDestroyed) {
                clearInterval(_startTimer);
                _startTimer = null;
                return;
            }
            var doc = editor.state.doc;
            if (!doc || doc.content.size === 0) {
                if (attempts > 20) { clearInterval(_startTimer); _startTimer = null; }
                return;
            }
            clearInterval(_startTimer);
            _startTimer = null;
            editor.commands.setTextSelection(1);
            editor.commands.scrollIntoView();
        }, 100);
    })();

    // ── Bubble Menu ──
    function updateBubble() {
        if (!editor || editor.isDestroyed) {
            hideBubble();
            return;
        }
        var sel = window.getSelection();
        if (!sel || sel.isCollapsed || !sel.rangeCount) {
            hideBubble();
            return;
        }
        var range = sel.getRangeAt(0);
        var rect = range.getBoundingClientRect();
        if (!rect || rect.width === 0) {
            hideBubble();
            return;
        }
        bubbleEl.classList.remove("hidden");
        var bw = bubbleEl.offsetWidth;
        var left = rect.left + rect.width / 2 - bw / 2;
        var top = rect.top - 48;
        if (top < 8) top = rect.bottom + 8;
        if (left < 8) left = 8;
        if (left + bw > window.innerWidth - 8)
            left = window.innerWidth - bw - 8;
        bubbleEl.style.left = left + "px";
        bubbleEl.style.top = top + "px";
        bubbleEl.querySelectorAll(".bubble-btn").forEach(function (btn) {
            var cmd = btn.dataset.cmd;
            if (cmd)
                btn.classList.toggle("is-active", editor.isActive(cmd));
        });
    }

    function hideBubble() {
        bubbleEl.classList.add("hidden");
    }

    bubbleEl.addEventListener("mousedown", function (e) {
        var btn = e.target.closest(".bubble-btn");
        if (!btn) return;
        e.preventDefault();
        var cmd = btn.dataset.cmd;
        if (cmd) exec(cmd);
        if (btn.id === "bubble-link") promptLink();
    });

    // ── Table Bubble Menu ──
    function updateTableBubble() {
        if (!editor || editor.isDestroyed || !tableBubbleEl) {
            hideTableBubble();
            return;
        }
        if (!editor.isActive("table")) {
            hideTableBubble();
            return;
        }
        var sel = window.getSelection();
        if (!sel || !sel.rangeCount) {
            hideTableBubble();
            return;
        }
        var cells = document.querySelectorAll('.selectedCell');
        if (cells.length <= 1) {
            hideTableBubble();
            return;
        }
        tableBubbleEl.classList.remove("hidden");
        var left = Infinity, top = Infinity, right = -Infinity, bottom = -Infinity;
        cells.forEach(function(c) {
            var r = c.getBoundingClientRect();
            if (r.left < left) left = r.left;
            if (r.top < top) top = r.top;
            if (r.right > right) right = r.right;
            if (r.bottom > bottom) bottom = r.bottom;
        });
        var rect = {
            left: left,
            top: top,
            right: right,
            bottom: bottom,
            width: right - left,
            height: bottom - top
        };
        var bw = tableBubbleEl.offsetWidth;
        var menuLeft = rect.left + rect.width / 2 - bw / 2;
        var menuTop = rect.top - tableBubbleEl.offsetHeight - 8;
        if (menuTop < 8) {
            menuTop = rect.bottom + 8;
        }
        if (menuLeft < 8) menuLeft = 8;
        if (menuLeft + bw > window.innerWidth - 8) {
            menuLeft = window.innerWidth - bw - 8;
        }
        tableBubbleEl.style.left = menuLeft + "px";
        tableBubbleEl.style.top = menuTop + "px";

        tableBubbleEl.querySelectorAll(".table-bubble-btn").forEach(function (btn) {
            var cmd = btn.dataset.cmd;
            if (cmd) {
                btn.classList.toggle("is-active", editor.isActive(cmd));
            }
        });
    }

    function hideTableBubble() {
        if (tableBubbleEl) tableBubbleEl.classList.add("hidden");
    }

    if (tableBubbleEl) {
        tableBubbleEl.addEventListener("mousedown", function (e) {
            var btn = e.target.closest(".table-bubble-btn");
            if (!btn) return;
            e.preventDefault();
            var cmd = btn.dataset.cmd;
            if (cmd && editor) {
                var chain = editor.chain().focus();
                switch (cmd) {
                    case "addRowBefore":
                        chain.addRowBefore();
                        break;
                    case "addRowAfter":
                        chain.addRowAfter();
                        break;
                    case "addColumnBefore":
                        chain.addColumnBefore();
                        break;
                    case "addColumnAfter":
                        chain.addColumnAfter();
                        break;
                    case "deleteRow":
                        chain.deleteRow();
                        break;
                    case "deleteColumn":
                        chain.deleteColumn();
                        break;
                    case "deleteTable":
                        if (confirm("Excluir esta tabela?")) {
                            chain.deleteTable();
                        }
                        break;
                    case "mergeCells":
                        chain.mergeCells();
                        break;
                    case "splitCell":
                        chain.splitCell();
                        break;
                }
                chain.run();
                setTimeout(updateTableBubble, 50);
            }
        });
    }

    window.addEventListener(
        "scroll",
        function () {
            if (!bubbleEl.classList.contains("hidden")) updateBubble();
            if (tableBubbleEl && !tableBubbleEl.classList.contains("hidden")) updateTableBubble();
        },
        true,
    );

    window.addEventListener("resize", function () {
        if (!bubbleEl.classList.contains("hidden")) updateBubble();
        if (tableBubbleEl && !tableBubbleEl.classList.contains("hidden")) updateTableBubble();
    });

    // ── Slash Menu ──
    function showSlashMenu(view) {
        var coords = view.coordsAtPos(view.state.selection.from);
        if (!coords) return;
        renderSlashItems("");
        slashEl.classList.remove("hidden");
        var mw = 224;
        var left = coords.left - 10;
        var mh = slashEl.offsetHeight || 240;
        var top;
        var spaceBelow = window.innerHeight - coords.bottom - 8;
        if (spaceBelow < mh && coords.top > mh + 8) {
            top = coords.top - mh - 4;
        } else {
            top = coords.bottom + 4;
        }
        if (left + mw > window.innerWidth - 8)
            left = window.innerWidth - mw - 8;
        if (left < 8) left = 8;
        slashEl.style.left = left + "px";
        slashEl.style.top = top + "px";
        selectSlashItem(0);
    }

    function hideSlashMenu() {
        slashEl.classList.add("hidden");
    }

    if (slashEl) {
        slashEl.addEventListener("mousedown", function (e) {
            e.preventDefault();
        });
    }

    // scoreSlashCommand importada de editor-slash.js

    function renderSlashItems(filter) {
        var f = filter.toLowerCase().trim();
        var isInTable = editor && editor.isActive("table");

        var items = [];
        SLASH_COMMANDS.forEach(function (c) {
            if (c.tableOnly && !isInTable) return;
            var score = scoreSlashCommand(c, f);
            if (score > 0) {
                items.push({ cmd: c, score: score });
            }
        });

        items.sort(function (a, b) {
            return b.score - a.score;
        });

        slashItems.innerHTML = "";
        items.forEach(function (entry, i) {
            var cmd = entry.cmd;
            var labelHtml = htmlEscape(cmd.label);
            if (f) {
                var idx = cmd.label.toLowerCase().indexOf(f);
                if (idx !== -1) {
                    var before = htmlEscape(cmd.label.substring(0, idx));
                    var match = htmlEscape(
                        cmd.label.substring(idx, idx + f.length),
                    );
                    var after = htmlEscape(
                        cmd.label.substring(idx + f.length),
                    );
                    labelHtml =
                        before + "<strong>" + match + "</strong>" + after;
                }
            }
            var btn = document.createElement("button");
            btn.className = "slash-item";
            btn.innerHTML =
                '<span class="icon">' +
                cmd.icon +
                '</span><span class="label">' +
                labelHtml +
                "</span>";
            btn.onclick = cmd.action;
            btn.dataset.index = i;
            slashItems.appendChild(btn);
        });
        selectSlashItem(0);
    }

    var slashSelected = 0;

    function htmlEscape(s) {
        return s
            .replace(/&/g, "&amp;")
            .replace(/"/g, "&quot;")
            .replace(/</g, "&lt;")
            .replace(/>/g, "&gt;");
    }

    function selectSlashItem(idx) {
        slashSelected = idx;
        slashItems
            .querySelectorAll(".slash-item")
            .forEach(function (el, i) {
                el.classList.toggle("active", i === idx);
                if (i === idx) el.scrollIntoView({ block: "nearest" });
            });
    }

    document.addEventListener("keydown", function (e) {
        if (!slashEl.classList.contains("hidden")) {
            var items = slashItems.querySelectorAll(".slash-item");
            if (items.length === 0) return;
            if (e.key === "ArrowDown") {
                e.preventDefault();
                selectSlashItem(
                    Math.min(slashSelected + 1, items.length - 1),
                );
            } else if (e.key === "ArrowUp") {
                e.preventDefault();
                selectSlashItem(Math.max(slashSelected - 1, 0));
            } else if (e.key === "Enter" || e.key === "Tab") {
                e.preventDefault();
                items[slashSelected] && items[slashSelected].click();
            }
        }
    });

    // ── Executar ──
    function exec(cmd, attrs) {
        if (!editor) return;
        var chain = editor.chain().focus();
        switch (cmd) {
            case "bold":
                chain.toggleBold();
                break;
            case "italic":
                chain.toggleItalic();
                break;
            case "underline":
                chain.toggleUnderline();
                break;
            case "strike":
                chain.toggleStrike();
                break;
            case "highlight":
                chain.toggleHighlight();
                break;
            case "code":
                chain.toggleCode();
                break;
            case "heading":
                chain.toggleHeading(attrs);
                break;
            case "bulletList":
                chain.toggleBulletList();
                break;
            case "orderedList":
                chain.toggleOrderedList();
                break;
            case "taskList":
                chain.toggleTaskList();
                break;
            case "blockquote":
                chain.toggleBlockquote();
                break;
            case "codeBlock":
                chain.toggleCodeBlock();
                break;
            case "horizontalRule":
                chain.setHorizontalRule();
                break;
            case "addRowBefore":
                chain.addRowBefore();
                break;
            case "addRowAfter":
                chain.addRowAfter();
                break;
            case "addColumnBefore":
                chain.addColumnBefore();
                break;
            case "addColumnAfter":
                chain.addColumnAfter();
                break;
            case "deleteRow":
                chain.deleteRow();
                break;
            case "deleteColumn":
                chain.deleteColumn();
                break;
            case "deleteTable":
                chain.deleteTable();
                break;
            default:
                return;
        }
        chain.run();
    }

    function promptLink() {
        var url = prompt("URL do link:");
        if (url && editor) {
            // Valida URL básica para evitar javascript: etc
            try {
                var parsed = new URL(url);
                if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:' && parsed.protocol !== 'mailto:') {
                    alert("URL inválida. Apenas http://, https:// e mailto: são permitidos.");
                    return;
                }
            } catch (e) {
                // Tenta como relativa
                if (url.startsWith('/') || url.startsWith('#')) {
                    // URL relativa é permitida
                } else {
                    alert("URL inválida. Use http://, https:// ou mailto:.");
                    return;
                }
            }
            editor.chain().focus().setLink({ href: url }).run();
        }
    }

    // ── Frontmatter ──
    // O toggle do painel, o autosize do textarea e as sugestões de tags vivem em
    // editor-frontmatter.js (instanciado no topo, quando o frontmatter é lido).

    // ── TOC ──
    var tocVisible = false;
    var tocUpdateTimer = null;

    function toggleToc() {
        tocVisible = !tocVisible;
        var el = document.getElementById("toc-area");
        var arrow = document.getElementById("toc-arrow");
        if (el) el.classList.toggle("hidden", !tocVisible);
        if (arrow) arrow.classList.toggle("rotate-90", tocVisible);
        if (tocVisible) doUpdateToc();
    }

    // ── Backlinks ──
    var backlinksVisible = true;
    function toggleBacklinks() {
        backlinksVisible = !backlinksVisible;
        var el = document.getElementById("backlinks-content");
        var arrow = document.getElementById("backlinks-arrow");
        if (el) el.classList.toggle("hidden", !backlinksVisible);
        if (arrow) arrow.classList.toggle("rotate-90", backlinksVisible);
    }

    // updateToc e applyToc importadas de editor-toc.js.
    // Wrappers locais injetam o editor e o textarea, mantendo a assinatura
    // sem argumentos esperada pelos handlers e exports de window.
    function doUpdateToc() {
        updateToc(editor, /** @type {HTMLTextAreaElement} */(document.getElementById("toc-area")));
    }

    function doApplyToc(tocText) {
        applyToc(editor, tocText);
    }

    document.addEventListener("input", function (e) {
        if (e.target && e.target.id === "toc-area") {
            if (tocUpdateTimer) clearTimeout(tocUpdateTimer);
            tocUpdateTimer = setTimeout(function () {
                doApplyToc(document.getElementById("toc-area").value);
            }, 300);
        }
    });

    // Tab no TOC insere/remove tabulacao
    document.addEventListener("keydown", function (e) {
        if (e.target && e.target.id === "toc-area" && e.key === "Tab") {
            e.preventDefault();
            var ta = e.target;
            var start = ta.selectionStart;
            var end = ta.selectionEnd;
            if (e.shiftKey) {
                var lineStart = ta.value.lastIndexOf("\n", start - 1) + 1;
                if (ta.value.charAt(lineStart) === "\t") {
                    ta.value = ta.value.substring(0, lineStart) + ta.value.substring(lineStart + 1);
                    ta.selectionStart = ta.selectionEnd = Math.max(start - 1, lineStart);
                }
            } else {
                ta.value = ta.value.substring(0, start) + "\t" + ta.value.substring(end);
                ta.selectionStart = ta.selectionEnd = start + 1;
            }
            ta.dispatchEvent(new Event("input", { bubbles: true }));
        }
    });

    // ── Frontmatter: sugestões de tags e autosize ──
    // Movidos para editor-frontmatter.js (com o toggle do painel e o listener de
    // input, que agora chama o onChange passado na instanciação).

    // ── Status ──
    function setStatus(s) {
        currentStatus = s;
        EditorCommon.setStatus(statusEl, s);
    }

    // ── Save ──
    async function saveNow() {
        if (saveTimer) clearTimeout(saveTimer);
        await doSave();
    }

    async function doSave() {
        if (!editor || currentStatus === "saving") return;
        setStatus("saving");
        var md = "";
        try {
            md = editor.storage.markdown.getMarkdown();
        } catch (e) {
            md = editor.getHTML();
        }
        md = EditorCommon.stripNbspParagraphs(md);
        var fm = (/** @type {HTMLTextAreaElement} */(document.getElementById("frontmatter-area"))).value.trim();
        var finalContent = fm ? "---\n" + fm + "\n---\n" + md : md;
        // Converte links do editor de volta para [[wikilinks]]
        finalContent = EditorCommon.wikilinksToMarkdown(finalContent);

        var currentHash = await generateHash(finalContent);
        if (currentHash === lastSavedHash) {
            setStatus("saved");
            return;
        }

        var fd = new FormData();
        fd.append("filename", EditorCommon.getCurrentFilename(filenameInput));
        fd.append("content", finalContent);
        fd.append("tags", "");
        try {
            var resp = await fetch("/file/save", {
                method: "POST",
                body: fd,
                headers: EditorCommon.getAuthHeaders(),
            });
            if (resp.ok || resp.status === 303) {
                setStatus("saved");
                lastSavedHash = currentHash;
                // A nota pode ter criado/removido tarefas: avisa o badge do
                // cabeçalho (hx-trigger="todos-updated") para recontar.
                try {
                    document.body.dispatchEvent(new Event("todos-updated"));
                } catch (e) { /* ambiente sem body/Event */ }
            } else {
                var errText = await resp.text().catch(function () {
                    return "";
                });
                setStatus("dirty");
                console.error("Save failed:", resp.status, errText);
            }
        } catch (e) {
            setStatus("dirty");
        }
    }

    // ── Rename ──
    filenameInput.addEventListener("keydown", function (e) {
        if (e.key === "Enter") {
            e.preventDefault();
            filenameInput.blur();
        }
    });
    filenameInput.addEventListener("blur", function () {
        doRename();
    });

    var isRenaming = false;
    async function doRename() {
        if (isRenaming) return;
        isRenaming = true;
        var newName = filenameInput.value.trim();
        if (newName.endsWith(".md")) newName = newName.slice(0, -3);
        if (!newName || newName === originalDisplayName) {
            isRenaming = false;
            return;
        }
        newName += ".md";
        if (saveTimer) clearTimeout(saveTimer);
        setStatus("saving");

        var md = "";
        try {
            md = editor.storage.markdown.getMarkdown();
        } catch (e) {
            md = editor.getHTML();
        }
        md = EditorCommon.stripNbspParagraphs(md);
        var fm = document.getElementById("frontmatter-area").value.trim();
        var finalContent = fm ? "---\n" + fm + "\n---\n" + md : md;
        // Converte links do editor de volta para [[wikilinks]]
        finalContent = EditorCommon.wikilinksToMarkdown(finalContent);

        try {
            if (originalFilename !== "notes/" + newName) {
                var renameFd = new FormData();
                renameFd.append("old", originalFilename);
                renameFd.append("new", "notes/" + newName);
                var renameResp = await fetch("/file/rename", {
                    method: "POST",
                    body: renameFd,
                    headers: EditorCommon.getAuthHeaders(),
                });
                if (!renameResp.ok) {
                    var errTxt = await renameResp.text().catch(function () { return ""; });
                    throw new Error(errTxt || "Erro ao renomear no servidor");
                }
                // O backend propaga o `pai:` das filhas no rename (elas não ficam
                // mais órfãs). O contador vem no header para avisar o usuário.
                var kidsRepointed = parseInt(renameResp.headers.get("X-Children-Updated") || "0", 10);
                if (kidsRepointed > 0) {
                    hierarchy.notify(
                        kidsRepointed === 1
                            ? "1 nota filha atualizada"
                            : kidsRepointed + " notas filhas atualizadas"
                    );
                }
            }

            var fd = new FormData();
            fd.append("filename", newName);
            fd.append("content", finalContent);
            fd.append("tags", "");
            var saveResp = await fetch("/file/save", {
                method: "POST",
                body: fd,
                headers: EditorCommon.getAuthHeaders(),
            });
            if (saveResp.ok || saveResp.status === 303) {
                setStatus("saved");
                var newHash = await generateHash(finalContent);
                lastSavedHash = newHash;
            } else {
                setStatus("dirty");
                console.error("Save failed:", saveResp.status);
            }

            originalFilename = "notes/" + newName;
            originalDisplayName = newName.replace(/\.md$/, "");

            // Sem reload: o rename é aplicado na própria página (URL, título da aba,
            // input/data-filename e sidebar). Só mantém o redirect quando o novo nome
            // cai em OUTRO editor (ex: renomear para algo com "desenho" → /drawing),
            // pois aí a rota é decidida pelo servidor.
            if (EditorCommon.renameChangesEditor(newName, "/editor")) {
                window.location.href =
                    "/editor?file=" + encodeURIComponent("notes/" + newName);
                return;
            }
            EditorCommon.applyRenameToUI({
                newName: newName,
                filenameInput: filenameInput,
                base: "/editor",
            });
        } catch (err) {
            setStatus("dirty");
            console.error("Rename failed:", err);
            alert("Erro ao renomear nota: " + (err.message || "desconhecido"));
            filenameInput.value = originalDisplayName;
        } finally {
            isRenaming = false;
        }
    }

    // ── Delete current note ──
    function deleteCurrentNote() {
        EditorCommon.deleteCurrentNote(filenameInput);
    }

    // ── Duplicate current note ──
    function duplicateCurrentNote() {
        EditorCommon.duplicateCurrentNote(filenameInput, "/editor");
    }

    // ── Ctrl+S ──
    document.addEventListener("keydown", function (e) {
        if ((e.ctrlKey || e.metaKey) && e.key === "s") {
            e.preventDefault();
            saveNow();
        }
    });

    // ── Save button ──
    var saveBtn = document.getElementById("save-btn");
    if (saveBtn) saveBtn.addEventListener("click", saveNow);

    // ── Hierarquia (pai/filhas) ──
    // O painel de frontmatter já foi inicializado no topo (antes da criação do
    // editor, que precisa preencher o textarea). Aqui ligamos a barra de
    // hierarquia: botão "Nota-mãe" e popover de filhas.
    var hierarchy = initHierarchy({
        getFilename: function () {
            return EditorCommon.getCurrentFilename(filenameInput);
        },
        authHeaders: EditorCommon.getAuthHeaders,
    });

    // ── Clicar fora fecha menus ──
    document.addEventListener("mousedown", function (e) {
        if (
            !bubbleEl.classList.contains("hidden") &&
            !bubbleEl.contains(e.target)
        )
            hideBubble();
        if (
            !slashEl.classList.contains("hidden") &&
            !slashEl.contains(e.target)
        )
            hideSlashMenu();
        if (
            tableBubbleEl &&
            !tableBubbleEl.classList.contains("hidden") &&
            !tableBubbleEl.contains(e.target) &&
            !e.target.closest(".ProseMirror")
        ) {
            hideTableBubble();
        }
        var wm = document.getElementById("wikilink-menu");
        if (
            !wm.classList.contains("hidden") &&
            !wm.contains(e.target) &&
            !e.target.closest(".ProseMirror")
        ) {
            hideWikiMenu();
        }
    });

    // ── Image upload ──
    async function uploadAndInsertImage(file) {
        var fd = new FormData();
        fd.append("file", file);

        try {
            var resp = await fetch("/api/upload-image", {
                method: "POST",
                body: fd,
                headers: EditorCommon.getAuthHeaders(),
            });
            var data = await resp.json();
            if (data.ok && editor) {
                editor.chain().focus().setImage({ src: data.url }).run();
                setStatus("dirty");
            } else {
                alert(
                    "Erro ao fazer upload da imagem: " +
                        (data.error || "desconhecido"),
                );
            }
        } catch (err) {
            alert("Erro ao fazer upload da imagem: " + err.message);
        }
    }

    document
        .getElementById("editor-image-input")
        .addEventListener("change", async function (e) {
            var file = e.target.files[0];
            if (!file) return;
            e.target.value = "";
            await uploadAndInsertImage(file);
        });

    loadWikiNotes();
    setStatus("saved");
    window.deleteCurrentNote = deleteCurrentNote;
    window.duplicateCurrentNote = duplicateCurrentNote;
    window.toggleToc = toggleToc;
    window.toggleBacklinks = toggleBacklinks;
    window.updateToc = doUpdateToc;
    window.applyToc = doApplyToc;
})();
