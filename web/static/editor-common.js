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
