// editor-frontmatter.js — painel de frontmatter do editor
//
// Extraído de editor-init.js (item de modularização): concentra o toggle do
// painel, o autosize do textarea e as sugestões de tags. O IIFE apenas injeta
// o callback `onChange` (que marca a nota como suja e agenda o save).
// @ts-nocheck

/**
 * Inicializa o painel de frontmatter.
 * @param {{
 *   onChange: () => void
 * }} deps - onChange é chamado a cada edição do textarea (inclusive ao clicar
 *             numa tag sugerida, que dispara um evento "input" sintético).
 * @returns {{
 *   setValue: (text: string) => void,
 *   getValue: () => string,
 *   toggle: () => void
 * }}
 */
export function initFrontmatter(deps) {
    var area = /** @type {HTMLTextAreaElement} */(document.getElementById("frontmatter-area"));
    var toggleBtn = document.getElementById("toggle-fm-btn");
    var arrow = document.getElementById("fm-arrow");
    var tagSuggest = document.getElementById("tag-suggestions");
    var visible = false;

    function autosize() {
        if (!area) return;
        area.style.height = "auto";
        area.style.height = area.scrollHeight + "px";
    }

    function triggerInput(el) {
        el.dispatchEvent(new Event("input", { bubbles: true }));
    }

    function loadTagSuggestions() {
        fetch("/api/tags")
            .then(function (r) { return r.json(); })
            .then(function (data) {
                var list = document.getElementById("tag-suggestion-list");
                if (!list) return;
                var tags = data.tags || [];
                if (tags.length === 0) {
                    list.innerHTML = '<span class="text-[10px] text-zinc-700">Nenhuma tag indexada.</span>';
                    return;
                }
                list.innerHTML = "";
                tags.forEach(function (t) {
                    var btn = document.createElement("button");
                    btn.className = "text-[10px] font-bold text-zinc-500 bg-zinc-800/40 hover:bg-zinc-700/60 hover:text-zinc-300 px-2 py-0.5 rounded-sm transition-colors";
                    btn.textContent = "#" + t;
                    btn.onclick = function () { addTagToFrontmatter(t); };
                    list.appendChild(btn);
                });
            })
            .catch(function () {});
    }

    function addTagToFrontmatter(tag) {
        if (!area) return;
        var val = area.value;

        var flowMatch = val.match(/^tags:\s*\[([^\]]*)\]/m);
        if (flowMatch) {
            var existing = flowMatch[1].trim();
            var tags = existing ? existing.split(",").map(function (t) { return t.trim(); }).filter(Boolean) : [];
            if (tags.indexOf(tag) === -1) tags.push(tag);
            area.value = val.replace(/^tags:\s*\[[^\]]*\]/m, "tags: [" + tags.join(", ") + "]");
            triggerInput(area);
            return;
        }

        var lines = val.split("\n");
        var tagsIndex = -1;
        for (var i = 0; i < lines.length; i++) {
            if (lines[i].trim() === "tags:") {
                tagsIndex = i;
                break;
            }
        }
        if (tagsIndex !== -1) {
            var list = [];
            var lastIndex = tagsIndex;
            for (var j = tagsIndex + 1; j < lines.length; j++) {
                var line = lines[j];
                var match = line.match(/^\s*-\s*(.+)$/);
                if (match) {
                    list.push(match[1].trim());
                    lastIndex = j;
                } else if (line.trim() !== "" && line.indexOf(" ") !== 0) {
                    break;
                }
            }
            if (list.indexOf(tag) === -1) list.push(tag);
            lines.splice(tagsIndex, (lastIndex - tagsIndex) + 1, "tags: [" + list.join(", ") + "]");
            area.value = lines.join("\n");
            triggerInput(area);
            return;
        }

        area.value = val.trim() ? val.trim() + "\ntags: [" + tag + "]\n" : "tags: [" + tag + "]\n";
        triggerInput(area);
    }

    function toggle() {
        visible = !visible;
        if (area) area.classList.toggle("hidden", !visible);
        if (arrow) arrow.classList.toggle("rotate-90", visible);
        if (tagSuggest) tagSuggest.classList.toggle("hidden", !visible);
        if (visible) loadTagSuggestions();
    }

    if (toggleBtn) toggleBtn.addEventListener("click", toggle);
    if (area) {
        area.addEventListener("input", function () {
            autosize();
            deps.onChange();
        });
    }

    return {
        setValue: function (text) {
            if (!area) return;
            area.value = text;
            autosize();
        },
        getValue: function () {
            return area ? area.value : "";
        },
        toggle: toggle,
    };
}
