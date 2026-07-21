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

            function updateHighlight() {
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
            var name = filenameInput.dataset.filename || filenameInput.value.trim();
            return this.normalizeFilename(name);
        },

        // ── getDisplayName: extrai nome de exibição de um filename ──
        getDisplayName: function (filename) {
            return filename.split("/").pop().replace(/\.md$/i, "");
        },

        // ── deleteCurrentNote: genérico para todos os tipos de nota ──
        deleteCurrentNote: function (filenameInput, confirmMsg) {
            var filename = this.getCurrentFilename(filenameInput);
            if (!filename) return;
            if (!confirm(confirmMsg || 'Excluir definitivamente "' + filename + '"?')) return;
            var fd = new FormData();
            fd.append("filename", filename);
            fetch("/file/delete", { method: "POST", body: fd, headers: this.getAuthHeaders() })
                .then(function () { window.location.href = "/"; })
                .catch(function () { window.location.href = "/"; });
        },

        // ── duplicateCurrentNote: genérico para todos os tipos de nota ──
        duplicateCurrentNote: function (filenameInput, redirectBase, confirmMsg) {
            var filename = this.getCurrentFilename(filenameInput);
            if (!filename) return;
            if (!confirm(confirmMsg || 'Duplicar "' + filename + '"?')) return;

            var fd = new FormData();
            fd.append("filename", filename);

            fetch("/api/note/duplicate", { method: "POST", body: fd, headers: this.getAuthHeaders() })
                .then(function (r) {
                    if (!r.ok) return r.text().then(function (t) { throw new Error(t); });
                    return r.json();
                })
                .then(function (data) {
                    if (data && data.new_filename) {
                        window.location.href = (redirectBase || "/editor") + "?file=" + encodeURIComponent(data.new_filename);
                    }
                })
                .catch(function (err) {
                    alert("Erro ao duplicar: " + err.message);
                });
        },

        // ── doRenameContent: renomeia + salva conteúdo, comum a todos os tipos ──
        doRenameContent: async function (filenameInput, getContentFn, redirectBase, opts) {
            opts = opts || {};
            var newName = filenameInput.value.trim();
            if (newName.endsWith(".md")) newName = newName.slice(0, -3);

            var currentFilename = this.getCurrentFilename(filenameInput);
            var currentDisplayName = this.getDisplayName(currentFilename);

            if (!newName || newName === currentDisplayName) return;

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
                    if (!renameResp.ok) throw new Error("Erro ao renomear no servidor");
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
                filenameInput.dataset.filename = fullNewName;
                window.location.href = (redirectBase || "/editor") + "?file=" + encodeURIComponent(fullNewName);
            } catch (e) {
                console.error(e);
                alert("Erro ao renomear: " + (e.message || "desconhecido"));
                filenameInput.value = currentDisplayName;
                if (opts.setStatus) opts.setStatus("dirty");
            }
        },

        // ── setupRenameListeners: liga eventos de rename num filenameInput ──
        setupRenameListeners: function (filenameInput, opts) {
            opts = opts || {};
            var self = this;
            filenameInput.addEventListener("keydown", function (e) {
                if (e.key === "Enter") {
                    e.preventDefault();
                    self.doRenameContent(filenameInput, opts.getContent, opts.redirectBase, opts);
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
        }
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
