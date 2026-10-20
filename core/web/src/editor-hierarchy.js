// editor-hierarchy.js — barra de hierarquia do editor (pai/filhas)
//
// Extraído de editor-init.js (item de modularização): o IIFE apenas orquestra.
// O backend já valida a gravação (auto-pai e ciclo → HTTP 400) na mesma rota
// usada pela célula do Tabulator (`POST /api/notes/update-property`).
// @ts-nocheck

/**
 * Nome de exibição de um arquivo. Espelha domain.DisplayName (backend):
 * remove a pasta, o prefixo interno "captura-" e a extensão ".md".
 * @param {string} file
 * @returns {string}
 */
function displayName(file) {
    var base = String(file || "").split("/").pop() || "";
    base = base.replace(/^captura-/, "");
    return base.replace(/\.md$/, "");
}

/**
 * Inicializa a barra de hierarquia: o botão "Nota-mãe" define o campo `pai:` do
 * frontmatter numa lista pesquisável e o badge de filhas abre o popover.
 * @param {{
 *   getFilename: () => string,
 *   authHeaders: () => Record<string, string>
 * }} deps
 * @returns {{ notify: (msg: string) => void }}
 */
export function initHierarchy(deps) {
    var bar = document.getElementById("hierarchy-bar");
    if (!bar) return { notify: function () {} };

    var btn = document.getElementById("parent-btn");
    var clearBtn = document.getElementById("parent-clear-btn");
    var nameEl = document.getElementById("parent-name");
    var picker = document.getElementById("parent-picker");
    var search = /** @type {HTMLInputElement} */(document.getElementById("parent-search"));
    var listEl = document.getElementById("parent-list");
    var errorEl = document.getElementById("parent-error");
    var msgEl = document.getElementById("hierarchy-msg");
    var childrenBtn = document.getElementById("children-badge-btn");
    var childrenPop = document.getElementById("children-popover");

    var allNotes = null; // cache: [{ file, name }] vindo de /api/notes
    var filtered = [];   // itens visíveis no picker
    var active = -1;     // índice destacado (teclado)
    var msgTimer = null;

    // ── Aviso/erro ──
    function notify(msg) {
        if (!msgEl || !msg) return;
        msgEl.textContent = msg;
        msgEl.classList.remove("hidden");
        if (msgTimer) clearTimeout(msgTimer);
        msgTimer = setTimeout(function () { msgEl.classList.add("hidden"); }, 4000);
    }

    function setError(msg) {
        if (!errorEl) return;
        errorEl.textContent = msg || "";
        errorEl.classList.toggle("hidden", !msg);
    }

    // ── Chip do pai ──
    function showParent(name) {
        if (nameEl) {
            nameEl.textContent = name || "—";
            nameEl.className = "font-bold " + (name ? "text-sky-400" : "text-zinc-600");
        }
        if (clearBtn) clearBtn.classList.toggle("hidden", !name);
    }

    // ── Lista de notas candidatas ──
    function loadNotes() {
        if (allNotes) return Promise.resolve(allNotes);
        return fetch("/api/notes")
            .then(function (r) { return r.json(); })
            .then(function (data) {
                allNotes = (data.notes || []).map(function (n) {
                    return { file: n.arquivo, name: displayName(n.arquivo) };
                });
                return allNotes;
            })
            .catch(function () {
                allNotes = [];
                return allNotes;
            });
    }

    function renderList() {
        var q = (search.value || "").trim().toLowerCase();
        var self = displayName(deps.getFilename());
        filtered = allNotes.filter(function (n) {
            if (n.name === self) return false; // não pode ser pai de si mesma
            return !q || n.name.toLowerCase().indexOf(q) >= 0;
        }).slice(0, 200);

        listEl.innerHTML = "";
        if (filtered.length === 0) {
            var empty = document.createElement("div");
            empty.className = "px-3 py-2 text-[10px] font-mono text-zinc-600";
            empty.textContent = allNotes.length === 0 ? "Nenhuma nota disponível." : "Nada encontrado.";
            listEl.appendChild(empty);
            active = -1;
            return;
        }

        filtered.forEach(function (n, i) {
            var b = document.createElement("button");
            b.type = "button";
            b.className = "wikilink-item";
            b.dataset.index = String(i);

            var icon = document.createElement("span");
            icon.className = "wiki-icon";
            icon.textContent = "↑";

            var label = document.createElement("span");
            label.className = "truncate";
            label.textContent = n.name;

            b.appendChild(icon);
            b.appendChild(label);
            b.addEventListener("click", function () { selectIndex(i); });
            listEl.appendChild(b);
        });
        active = -1;
    }

    function move(delta) {
        if (filtered.length === 0) return;
        active = (active + delta + filtered.length) % filtered.length;
        var items = listEl.querySelectorAll(".wikilink-item");
        for (var i = 0; i < items.length; i++) {
            items[i].classList.toggle("active", i === active);
        }
        var target = items[active];
        if (target && target.scrollIntoView) target.scrollIntoView({ block: "nearest" });
    }

    function selectIndex(i) {
        var n = filtered[i];
        if (!n) return;
        applyParent(n.name);
    }

    function openPicker() {
        picker.classList.remove("hidden");
        setError("");
        search.value = "";
        listEl.innerHTML = '<div class="px-3 py-2 text-[10px] font-mono text-zinc-600">Carregando…</div>';
        loadNotes().then(renderList);
        setTimeout(function () { search.focus(); }, 0);
    }

    function closePicker() {
        picker.classList.add("hidden");
        setError("");
    }

    // ── Gravação do `pai:` ──
    function applyParent(value) {
        setError("");
        var file = deps.getFilename();
        fetch("/api/notes/update-property", {
            method: "POST",
            headers: Object.assign({ "Content-Type": "application/json" }, deps.authHeaders()),
            body: JSON.stringify({ file: file, key: "pai", value: value }),
        }).then(function (resp) {
            if (resp.ok) {
                showParent(value);
                closePicker();
                notify(value ? "Pai definido: " + value : "Nota movida para a raiz");
                return null;
            }
            return resp.text().then(function (t) {
                throw new Error((t || "Erro ao gravar").trim());
            });
        }).catch(function (err) {
            setError(err && err.message ? err.message : "Erro ao gravar o pai");
        });
    }

    // ── Wiring ──
    if (btn) {
        btn.addEventListener("click", function (e) {
            e.stopPropagation();
            if (picker.classList.contains("hidden")) openPicker();
            else closePicker();
        });
    }
    if (clearBtn) {
        clearBtn.addEventListener("click", function (e) {
            e.stopPropagation();
            applyParent("");
        });
    }
    if (search) {
        search.addEventListener("input", renderList);
        search.addEventListener("keydown", function (e) {
            if (e.key === "Escape") {
                e.preventDefault();
                closePicker();
            } else if (e.key === "ArrowDown") {
                e.preventDefault();
                move(1);
            } else if (e.key === "ArrowUp") {
                e.preventDefault();
                move(-1);
            } else if (e.key === "Enter") {
                e.preventDefault();
                selectIndex(active >= 0 ? active : 0);
            }
        });
    }
    if (childrenBtn && childrenPop) {
        childrenBtn.addEventListener("click", function (e) {
            e.stopPropagation();
            childrenPop.classList.toggle("hidden");
        });
    }
    document.addEventListener("mousedown", function (e) {
        if (picker && !picker.classList.contains("hidden") &&
            !picker.contains(e.target) && e.target !== btn && !(btn && btn.contains(e.target))) {
            closePicker();
        }
        if (childrenPop && !childrenPop.classList.contains("hidden") &&
            !childrenPop.contains(e.target) && e.target !== childrenBtn && !(childrenBtn && childrenBtn.contains(e.target))) {
            childrenPop.classList.add("hidden");
        }
    });

    return { notify: notify };
}
