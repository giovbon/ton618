// editor-common.js — Helpers compartilhados entre todos os editores TON-618
// Evita duplicação de generateHash, setStatus, getAuthHeaders e HTTP wrappers

(function () {
    "use strict";

    window.EditorCommon = {

        // ── generateHash: SHA-256 via crypto.subtle, com fallback para HTTP ──
        generateHash: async function (text) {
            try {
                var encoder = new TextEncoder();
                var data = encoder.encode(text);
                var hashBuffer = await crypto.subtle.digest("SHA-256", data);
                var hashArray = Array.from(new Uint8Array(hashBuffer));
                return hashArray.map(function (b) { return b.toString(16).padStart(2, "0"); }).join("");
            } catch (e) {
                // crypto.subtle nao disponivel em HTTP (nao-HTTPS)
                return "hash-" + Date.now() + "-" + Math.random().toString(36).slice(2, 8);
            }
        },

        // ── setStatus: atualiza o indicador de status (salvo/salvando/sujo) ──
        setStatus: function (el, s) {
            if (s === "saved") {
                el.textContent = "\u2713";
                el.className = "text-base font-mono text-emerald-500 shrink-0";
            } else if (s === "saving") {
                el.textContent = "\u27F3";
                el.className = "text-base font-mono text-sky-400 shrink-0";
            } else {
                el.textContent = "\u25CF";
                el.className = "text-base font-mono text-amber-500 shrink-0";
            }
        },

        // ── getAuthHeaders: retorna headers Authorization do localStorage ──
        getAuthHeaders: function () {
            var auth = localStorage.getItem("ton_auth");
            if (auth) {
                var header = auth.startsWith("Basic ") ? auth : "Basic " + auth;
                return { "Authorization": header };
            }
            return {};
        },

        // ── HTTP wrappers ─────────────────────────────────────────────────

        // saveNote: POST /api/note/save com FormData
        httpSaveNote: async function (filename, content, tags, silent) {
            var fd = new FormData();
            fd.append("filename", filename);
            fd.append("content", content);
            fd.append("tags", tags || "");
            if (silent) fd.append("silent", "true");

            var r = await fetch("/api/note/save", {
                method: "POST",
                body: fd,
                headers: this.getAuthHeaders()
            });
            if (!r.ok && r.status !== 303) throw new Error("HTTP " + r.status);
            return r;
        },

        // saveFile: POST /file/save com FormData (redirect-based)
        httpSaveFile: async function (filename, content, tags) {
            var fd = new FormData();
            fd.append("filename", filename);
            fd.append("content", content);
            fd.append("tags", tags || "");

            var r = await fetch("/file/save", {
                method: "POST",
                body: fd,
                headers: this.getAuthHeaders()
            });
            return r;
        },

        // rename: POST /file/rename com FormData
        httpRename: async function (oldName, newName) {
            var fd = new FormData();
            fd.append("old", oldName);
            fd.append("new", newName);
            var r = await fetch("/file/rename", {
                method: "POST",
                body: fd,
                headers: this.getAuthHeaders()
            });
            if (!r.ok) throw new Error("Erro ao renomear no servidor");
            return r;
        },

        // delete: POST /file/delete
        httpDelete: function (filename) {
            var fd = new FormData();
            fd.append("filename", filename);
            return fetch("/file/delete", {
                method: "POST",
                body: fd,
                headers: this.getAuthHeaders()
            });
        },

        // duplicate: POST /api/note/duplicate
        httpDuplicate: function (filename) {
            var fd = new FormData();
            fd.append("filename", filename);
            return fetch("/api/note/duplicate", {
                method: "POST",
                body: fd,
                headers: this.getAuthHeaders()
            });
        },

        // uploadImage: POST /api/upload-image
        httpUploadImage: async function (file) {
            var fd = new FormData();
            fd.append("file", file);
            var r = await fetch("/api/upload-image", {
                method: "POST",
                body: fd,
                headers: this.getAuthHeaders()
            });
            return r.json();
        },

        // toggleBacklinksPopover: abre/fecha o popup de backlinks
        toggleBacklinksPopover: function (event) {
            if (event) event.stopPropagation();
            var popover = document.getElementById('backlinks-popover');
            if (popover) {
                popover.classList.toggle('hidden');
            }
        },

        // ── wikilinksToMarkdown: converte links do editor de volta pra [[wikilinks]] ──
        wikilinksToMarkdown: function (content) {
            return content
                .replace(
                    /\[([^\]]+)\]\(\/editor\?file=notes\/(?:[^)]|\([^)]*\))*\.md\)/g,
                    "[[$1]]",
                )
                .replace(
                    /\[([^\]]+)\]\(\/epub\/reader\?file=(?:[^)]|\([^)]*\))*\.epub\)/g,
                    "[[$1]]",
                )
                .replace(
                    /\[([^\]]+)\]\(\/file\/download\?name=(?:[^)]|\([^)]*\))*\)/g,
                    "[[$1]]",
                );
        },

        // ── setupCodeJarActiveLine: highlights active line in CodeJar editor ──
        setupCodeJarActiveLine: function (editorEl) {
            if (!editorEl) return;

            // Create highlight overlay element
            var highlight = document.createElement("div");
            highlight.className = "codejar-active-line";
            highlight.style.position = "absolute";
            highlight.style.left = "1px";
            highlight.style.right = "1px";
            highlight.style.pointerEvents = "none";
            highlight.style.backgroundColor = "rgba(56, 189, 248, 0.035)"; // very light sky blue
            highlight.style.borderLeft = "2.5px solid #38bdf8"; // premium sky-blue bar
            highlight.style.transition = "top 0.08s ease-out, height 0.08s ease-out, opacity 0.15s ease";
            highlight.style.opacity = "0";
            highlight.style.zIndex = "15"; 

            // Insert as sibling of the editor inside its relative container
            if (editorEl.parentNode) {
                editorEl.parentNode.appendChild(highlight);
            }

            var highlightRaf = 0;
            // Coalesce múltiplos eventos (click/keyup/scroll/selectionchange/
            // resize) em um único update por frame. Antes cada evento lia
            // getClientRects/getBoundingClientRect e escrevia style.top/height
            // sem throttle → layout thrashing durante seleção/scroll.
            function updateHighlight() {
                if (highlightRaf) return;
                highlightRaf = requestAnimationFrame(function () {
                    highlightRaf = 0;
                    updateHighlightNow();
                });
            }

            function updateHighlightNow() {
                if (document.activeElement !== editorEl) {
                    highlight.style.opacity = "0";
                    return;
                }

                var sel = window.getSelection();
                if (!sel || sel.rangeCount === 0) {
                    highlight.style.opacity = "0";
                    return;
                }

                var range = sel.getRangeAt(0);
                if (!editorEl.contains(range.startContainer)) {
                    highlight.style.opacity = "0";
                    return;
                }

                // Hide highlight when there is a text selection block active
                if (!range.collapsed) {
                    highlight.style.opacity = "0";
                    return;
                }

                var rect = null;
                var rects = range.getClientRects();
                if (rects.length > 0) {
                    rect = rects[0];
                }

                // Fallback for empty lines
                if (!rect || rect.height === 0) {
                    var node = range.startContainer;
                    if (node.nodeType === Node.TEXT_NODE) {
                        node = node.parentNode;
                    }
                    if (node && node !== editorEl) {
                        rect = node.getBoundingClientRect();
                    }
                }

                if (rect && rect.height > 0) {
                    var editorRect = editorEl.getBoundingClientRect();
                    var relativeTop = rect.top - editorRect.top;
                    
                    highlight.style.top = (relativeTop + editorEl.offsetTop) + "px";
                    highlight.style.height = rect.height + "px";
                    highlight.style.opacity = "1";
                } else {
                    highlight.style.opacity = "0";
                }
            }

            // Bind all selection and layout change events
            editorEl.addEventListener("click", updateHighlight);
            editorEl.addEventListener("keyup", updateHighlight);
            editorEl.addEventListener("focus", function() {
                setTimeout(updateHighlight, 10);
            });
            editorEl.addEventListener("blur", function() {
                highlight.style.opacity = "0";
            });
            editorEl.addEventListener("scroll", updateHighlight);
            
            document.addEventListener("selectionchange", function() {
                if (document.activeElement === editorEl) {
                    updateHighlight();
                }
            });

            window.addEventListener("resize", updateHighlight);
        },

        // ── normalizeFilename: garante formato notes/<nome>.md ──
        normalizeFilename: function (name) {
            if (!name.endsWith(".md")) name += ".md";
            if (!name.startsWith("notes/")) name = "notes/" + name;
            return name;
        },

        // ── getCurrentFilename: obtém filename do input com fallback ──
        getCurrentFilename: function (filenameInput) {
            var input = (filenameInput && filenameInput.value !== undefined) ? filenameInput : document.getElementById("file-name");
            if (!input) return "";
            var name = (input.dataset && input.dataset.filename) || input.value.trim();
            if (!name) return "";
            return this.normalizeFilename(name);
        },

        // ── getDisplayName: extrai nome de exibição de um filename ──
        getDisplayName: function (filename) {
            return (filename.split("/").pop() || "").replace(/^captura-/i, "").replace(/\.md$/i, "");
        },

        // ── deleteCurrentNote: genérico para todos os tipos de nota ──
        deleteCurrentNote: function (filenameInput, confirmMsg) {
            var input = (filenameInput && filenameInput.value !== undefined) ? filenameInput : document.getElementById("file-name");
            var msg = typeof filenameInput === "string" ? filenameInput : confirmMsg;
            var filename = this.getCurrentFilename(input);
            if (!filename) return;
            if (!confirm(msg || 'Excluir definitivamente "' + filename + '"?')) return;
            var fd = new FormData();
            fd.append("filename", filename);
            fetch("/file/delete", { method: "POST", body: fd, headers: this.getAuthHeaders() })
                .then(function (r) {
                    if (r.ok || r.status === 303) {
                        window.location.href = "/";
                    } else {
                        return r.text().then(function (t) { alert("Erro ao excluir nota: " + (t || r.statusText)); });
                    }
                })
                .catch(function (err) {
                    alert("Erro ao excluir nota: " + err.message);
                });
        },

        // ── duplicateCurrentNote: genérico para todos os tipos de nota ──
        duplicateCurrentNote: function (filenameInput, redirectBase, confirmMsg) {
            var input = (filenameInput && filenameInput.value !== undefined) ? filenameInput : document.getElementById("file-name");
            var base = typeof redirectBase === "string" ? redirectBase : (window.location.pathname || "/editor");
            var msg = typeof confirmMsg === "string" ? confirmMsg : null;
            var filename = this.getCurrentFilename(input);
            if (!filename) return;
            if (!confirm(msg || 'Duplicar "' + filename + '"?')) return;

            var fd = new FormData();
            fd.append("filename", filename);

            fetch("/api/note/duplicate", { method: "POST", body: fd, headers: this.getAuthHeaders() })
                .then(function (r) {
                    if (!r.ok) return r.text().then(function (t) { throw new Error(t); });
                    return r.json();
                })
                .then(function (data) {
                    if (data && data.new_filename) {
                        window.location.href = base + "?file=" + encodeURIComponent(data.new_filename);
                    }
                })
                .catch(function (err) {
                    alert("Erro ao duplicar: " + err.message);
                });
        },

        // ── doRenameContent: renomeia + salva conteúdo, comum a todos os tipos ──
        doRenameContent: async function (filenameInput, getContentFn, redirectBase, opts) {
            if (filenameInput._isRenaming) return;
            filenameInput._isRenaming = true;
            opts = opts || {};
            var newName = filenameInput.value.trim();
            if (newName.endsWith(".md")) newName = newName.slice(0, -3);

            var currentFilename = this.getCurrentFilename(filenameInput);
            var currentDisplayName = this.getDisplayName(currentFilename);

            if (!newName || newName === currentDisplayName) {
                filenameInput._isRenaming = false;
                return;
            }

            var fullNewName = "notes/" + newName + ".md";

            try {
                if (opts.setStatus) opts.setStatus("saving");

                var content = typeof getContentFn === "function" ? getContentFn() : "";

                // 1. Rename
                if (currentFilename !== fullNewName) {
                    var renameFd = new FormData();
                    renameFd.append("old", currentFilename);
                    renameFd.append("new", fullNewName);
                    var renameResp = await fetch("/file/rename", { method: "POST", body: renameFd, headers: this.getAuthHeaders() });
                    if (!renameResp.ok) {
                        var errTxt = await renameResp.text().catch(function () { return ""; });
                        throw new Error(errTxt || "Erro ao renomear no servidor");
                    }
                }

                // 2. Save
                var saveFd = new FormData();
                saveFd.append("filename", fullNewName);
                saveFd.append("content", content);
                saveFd.append("tags", opts.tags || "");

                var saveResp = await fetch("/api/note/save", { method: "POST", body: saveFd, headers: this.getAuthHeaders() });
                if (!saveResp.ok) throw new Error("Erro ao salvar sob novo nome");

                if (opts.onSaved) opts.onSaved(content, fullNewName);

                // 3. Update + redirect
                var base = redirectBase || "/editor";
                if (this.renameChangesEditor(fullNewName, base)) {
                    filenameInput.dataset.filename = fullNewName;
                    window.location.href = base + "?file=" + encodeURIComponent(fullNewName);
                    return;
                }
                // Mesmo editor: atualiza a página sem recarregar.
                this.applyRenameToUI({ newName: fullNewName, filenameInput: filenameInput, base: base });
            } catch (e) {
                console.error(e);
                alert("Erro ao renomear: " + (e.message || "desconhecido"));
                filenameInput.value = currentDisplayName;
                if (opts.setStatus) opts.setStatus("dirty");
            } finally {
                filenameInput._isRenaming = false;
            }
        },

        // ── renameChangesEditor: o novo nome troca o EDITOR da nota? ──
        //
        // O backend escolhe a rota pelo NOME do arquivo (domain.DetectNoteType):
        // nomes com "mindmap"/"markmap" abrem em /mindmap e com "drawing"/"desenho"
        // em /drawing. Se o novo nome cai em OUTRO editor, quem tem que decidir é o
        // servidor — mantemos o redirect. Em todos os outros casos o rename é
        // aplicado na própria página, sem recarregar (ver applyRenameToUI).
        renameChangesEditor: function (newName, currentBase) {
            var base = String(newName || "").replace(/^notes\//i, "").toLowerCase();
            var atual = String(currentBase || "/editor");
            if (/(mindmap|markmap)/.test(base) && atual !== "/mindmap") return true;
            if (/(drawing|desenho)/.test(base) && atual !== "/drawing") return true;
            return false;
        },

        // ── applyRenameToUI: sincroniza a página depois de renomear, SEM reload ──
        //
        // O reload existia para "reidratar" a página depois do rename, mas custa a
        // rolagem/posição do cursor e o estado do editor. O que realmente precisa
        // acompanhar o novo nome:
        //   1. a URL (replaceState: mantém o histórico limpo e o F5 no lugar certo);
        //   2. o input do nome — `value` + `data-filename`, que é a fonte usada por
        //      salvar/excluir/duplicar (EditorCommon.getCurrentFilename);
        //   3. o título da aba;
        //   4. a lista da sidebar, que é renderizada pelo HTMX (evento reload-sidebar).
        applyRenameToUI: function (opts) {
            opts = opts || {};
            var filename = this.normalizeFilename(opts.newName);
            var input = opts.filenameInput || document.getElementById("file-name");
            var display = filename.split("/").pop() || filename;

            if (input) {
                input.value = display;
                if (input.dataset) input.dataset.filename = filename;
            }

            var base = opts.base || (window.location && window.location.pathname) || "/editor";
            try {
                window.history.replaceState(null, "", base + "?file=" + encodeURIComponent(filename));
            } catch (e) { /* ambiente sem history (testes em node) */ }

            // Preserva o prefixo que já está no título ("Editor - ", "Desenho - ", ...).
            var prefix = opts.titlePrefix || String(document.title || "").split(" - ")[0] || "Editor";
            try {
                document.title = prefix + " - " + display;
            } catch (e) { /* ambientes sem document */ }

            // A sidebar vive em outro elemento HTMX: pede para ela recarregar.
            try {
                document.body.dispatchEvent(new Event("reload-sidebar"));
            } catch (e) { /* ambiente sem body/Event */ }

            return filename;
        },

        // ── setupRenameListeners: liga eventos de rename num filenameInput ──
        setupRenameListeners: function (filenameInput, opts) {
            opts = opts || {};
            var self = this;
            filenameInput.addEventListener("keydown", function (e) {
                if (e.key === "Enter") {
                    e.preventDefault();
                    filenameInput.blur();
                }
            });
            filenameInput.addEventListener("blur", function () {
                self.doRenameContent(filenameInput, opts.getContent, opts.redirectBase, opts);
            });
        },

        // ── setupCtrlS: liga Ctrl+S para salvar ──
        setupCtrlS: function (saveFn) {
            document.addEventListener("keydown", function (e) {
                if ((e.ctrlKey || e.metaKey) && e.key === "s") {
                    e.preventDefault();
                    if (typeof saveFn === "function") saveFn();
                }
            });
        },

        // ── Paste: decide se o conteúdo colado deve ser interpretado como markdown ──
        //
        // Contexto: muitos programas (editores de código, leitores de PDF, terminais,
        // painéis de IA) colocam no clipboard um `text/html` que é apenas o MESMO
        // texto embrulhado em <div>/<p>, sem nenhuma formatação real. Se o HTML for
        // usado, as marcações cruas do markdown ("**negrito**", "*itálico*") entram
        // no editor como texto sujo. Nesse caso preferimos o texto puro e o deixamos
        // ser convertido (negrito vira negrito de verdade, sem os asteriscos).
        //
        // Quando o HTML tem formatação REAL (strong/b/em/h1/ul/table/link...), o HTML
        // ganha: ele é a fonte mais fiel e descartá-lo perderia a formatação.
        shouldPasteAsMarkdown: function (text, html) {
            if (!text) return false;
            if (html && this.htmlHasRealFormatting(html)) return false;
            return this.textHasMarkdownSyntax(text);
        },

        // Detecta formatação REAL dentro do HTML colado (tags que carregam semântica
        // visual). Tags puramente estruturais (div, p, br, span) NÃO contam: são as
        // que aparecem nos "HTML de fachada" descritos acima.
        htmlHasRealFormatting: function (html) {
            return /<(strong|b|em|i|u|s|strike|del|ins|mark|sub|sup|h[1-6]|ul|ol|li|dl|blockquote|pre|code|table|thead|tbody|tr|td|th|img|hr|a)\b/i
                .test(html);
        },

        // Detecta sintaxe markdown no texto puro: títulos, listas, ênfase, código,
        // riscado e links [texto](url).
        textHasMarkdownSyntax: function (text) {
            return /(?:^(?:#+\s+|\d+\.\s+|[-*+]\s+))|[*_`~]|\[.+\]\(.+\)/m.test(text);
        },

        // ── normalizePastedHtml: tira o "espaço morto" do conteúdo colado ──
        //
        // Por que: o CSS do editor é enxuto (p{margin:.3em}, p{line-height:1.7}),
        // então "grandes espaços entre parágrafos" ao colar NÃO vêm da margem — vêm
        // de lixo ESTRUTURAL do clipboard, e cada um vira uma LINHA EM BRANCO
        // inteira no editor. Casos reais já vistos:
        //   <p><br></p>            <div></div>          <div>&nbsp;</div>
        //   <p><span> </span></p>  <p><span>&nbsp;</span></p>
        //   <br><br>               <p>texto<br></p>     <p><br>texto</p>
        //   <p>a</p><br><p>b</p>   (br solto entre blocos)
        //   <li><p>único</p></li>  (lista "loose")
        //
        // A função roda em laço até estabilizar (remover o de dentro pode esvaziar o
        // de fora) e só REMOVE espaço vazio / desembrulha wrappers: nunca descarta
        // texto com conteúdo. Blocos com whitespace SIGNIFICATIVO (pre/code) e
        // células de tabela (td/th/tr) ficam de fora de propósito.
        normalizePastedHtml: function (html) {
            if (!html) return "";

            var out = String(html);

            // Nunca deixar script/style/meta/link entrarem na nota.
            out = out.replace(/<(script|style|meta|link)\b[^>]*>[\s\S]*?(?:<\/\1\s*>|$)/gi, "");

            for (var pass = 0; pass < 6; pass++) {
                var antes = out;

                // 1. Inline VAZIO: <span></span>, <span> </span>, <span>&nbsp;</span>.
                out = out.replace(
                    /<(?:span|font|b|i|u|em|strong|a|small|sub|sup|mark|label)\b[^>]*>(?:\s|&nbsp;|&#160;|\u00a0)*<\/(?:span|font|b|i|u|em|strong|a|small|sub|sup|mark|label)\s*>/gi,
                    "",
                );

                // 2. Bloco VAZIO: <p></p>, <p><br></p>, <div>&nbsp;</div>.
                out = out.replace(
                    /<(?:p|div|h[1-6]|li|blockquote|section|article|header|footer|figure)\b[^>]*>(?:\s|&nbsp;|&#160;|\u00a0|<br\s*\/?>)*<\/(?:p|div|h[1-6]|li|blockquote|section|article|header|footer|figure)\s*>/gi,
                    "",
                );

                // 3. Quebras consecutivas (com recheio) viram UMA quebra.
                out = out.replace(/(?:<br\s*\/?>\s*(?:&nbsp;|&#160;|\u00a0)?\s*){2,}/gi, "<br>");

                // 4. Quebra/espaço ÓRFÃO antes de abrir um bloco.
                out = out.replace(
                    /(?:\s|&nbsp;|&#160;|\u00a0|<br\s*\/?>)+(?=<(?:p|div|h[1-6]|ul|ol|li|blockquote|table|section|article|header|footer|figure)\b)/gi,
                    "",
                );

                // 5. Quebra/espaço ÓRFÃO logo depois de fechar um bloco.
                out = out.replace(
                    /(<\/(?:p|div|h[1-6]|ul|ol|li|blockquote|table|section|article|header|footer|figure)\s*>)(?:\s|&nbsp;|&#160;|\u00a0|<br\s*\/?>)+/gi,
                    "$1",
                );

                // 6. Quebra/espaço ÓRFÃO nas bordas do bloco (<p><br>texto</p> ou
                //    <p>texto<br></p>). Quebras NO MEIO do parágrafo são preservadas.
                out = out.replace(
                    /(<(?:p|div|h[1-6]|li|blockquote)\b[^>]*>)(?:\s|&nbsp;|&#160;|\u00a0|<br\s*\/?>)+/gi,
                    "$1",
                );
                out = out.replace(
                    /(?:\s|&nbsp;|&#160;|\u00a0|<br\s*\/?>)+(<\/(?:p|div|h[1-6]|li|blockquote)\s*>)/gi,
                    "$1",
                );

                if (out === antes) break;
            }

            // 7. Listas "loose": <li><p>único parágrafo</p></li> → <li>…</li>.
            //    O lookahead impede capturar através de um </p> (itens com 2+
            //    parágrafos são preservados).
            out = out.replace(
                /<li\b[^>]*>\s*<p\b[^>]*>((?:(?!<\/?p\b)[\s\S])*?)<\/p>\s*<\/li\s*>/gi,
                function (_m, conteudo) { return "<li>" + conteudo + "</li>"; },
            );

            // 8. Sobrou item de lista vazio → some.
            out = out.replace(/<li\b[^>]*>(?:\s|&nbsp;|<br\s*\/?>)*<\/li\s*>/gi, "");

            return out;
        },
    };

    window.deleteCurrentNote = function (filenameInput, confirmMsg) {
        return window.EditorCommon.deleteCurrentNote(filenameInput, confirmMsg);
    };
    window.duplicateCurrentNote = function (filenameInput, redirectBase, confirmMsg) {
        return window.EditorCommon.duplicateCurrentNote(filenameInput, redirectBase, confirmMsg);
    };

    // Close backlinks popover clicking outside
    document.addEventListener('click', function(event) {
        var popover = document.getElementById('backlinks-popover');
        var btn = document.getElementById('backlink-badge-btn');
        if (popover && !popover.classList.contains('hidden')) {
            if (btn && !btn.contains(event.target) && !popover.contains(event.target)) {
                popover.classList.add('hidden');
            }
        }
    });

})();
