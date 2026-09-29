// editor-toc.js — Módulo de Table of Contents (TOC) bidirecional do editor
// Exporta as funções de leitura e escrita do TOC.
// @ts-nocheck

/**
 * @typedef {{ pos: number, node: object, level: number, text: string }} Heading
 */

/**
 * Coleta os títulos do documento na ordem em que aparecem.
 * Ignora títulos sem texto (não aparecem no TOC e causariam dessincronia
 * se entrassem no pareamento linha↔título do applyToc).
 * @param {object} editor - instância do TipTap Editor
 * @returns {Heading[]}
 */
export function collectHeadings(editor) {
    var headings = [];
    if (!editor || editor.isDestroyed) return headings;
    editor.state.doc.descendants(function (node, pos) {
        if (node.type.name !== "heading") return;
        var text = node.textContent.trim();
        if (text) headings.push({ pos: pos, node: node, level: node.attrs.level, text: text });
    });
    return headings;
}

/**
 * Converte número de tabs de indentação em nível de heading (1–6).
 * @param {number} tabs
 * @returns {number}
 */
export function tocLevel(tabs) {
    return Math.min(Math.max(tabs + 1, 1), 6);
}

/**
 * Atualiza o textarea do TOC com os títulos atuais do documento.
 * Só escreve se o valor mudou — atribuir o mesmo valor move o cursor para o
 * fim enquanto o usuário digita no TOC.
 * @param {object} editor
 * @param {HTMLTextAreaElement} tocEl
 */
export function updateToc(editor, tocEl) {
    if (!editor || editor.isDestroyed || !tocEl) return;
    var next = collectHeadings(editor).map(function (h) {
        var indent = "";
        for (var i = 1; i < h.level; i++) indent += "\t";
        return indent + h.text;
    }).join("\n");
    if (tocEl.value !== next) tocEl.value = next;
    tocEl.style.height = "auto";
    tocEl.style.height = tocEl.scrollHeight + "px";
}

/**
 * Aplica o texto do TOC no documento — espelha a caixa nos dois sentidos:
 *   · linha editada  → renomeia o título
 *   · tabulação      → nível do título (1..6)
 *   · linha apagada  → o título correspondente é REMOVIDO da nota
 *   · linha nova     → cria um título novo no fim da nota
 *
 * As posições são as do documento ORIGINAL, então as mudanças são aplicadas
 * de trás para frente — assim uma alteração nunca invalida a posição da anterior.
 * @param {object} editor
 * @param {string|null} tocText
 */
export function applyToc(editor, tocText) {
    if (!editor || editor.isDestroyed) return;

    var parsed = String(tocText == null ? "" : tocText).split("\n")
        .map(function (l) {
            var tabs = 0;
            while (tabs < l.length && l.charAt(tabs) === "\t") tabs++;
            return { tabs: tabs, text: l.slice(tabs).trim() };
        })
        .filter(function (p) { return p.text; });

    var headings = collectHeadings(editor);
    var limit = Math.min(parsed.length, headings.length);

    // Sem mudança real? Não gera transação (nem marca a nota como suja).
    var changed = parsed.length !== headings.length;
    for (var c = 0; !changed && c < limit; c++) {
        if (parsed[c].text !== headings[c].text ||
            tocLevel(parsed[c].tabs) !== headings[c].level) {
            changed = true;
        }
    }
    if (!changed) return;

    var tr = editor.state.tr;

    // 1. Sobraram títulos no documento (TOC com menos linhas) → removê-los.
    for (var d = headings.length - 1; d >= parsed.length; d--) {
        tr.delete(headings[d].pos, headings[d].pos + headings[d].node.nodeSize);
    }

    // 2. Nível e texto dos títulos que continuam (de trás para frente).
    for (var u = limit - 1; u >= 0; u--) {
        var entry = headings[u];
        var node = entry.node;
        var level = tocLevel(parsed[u].tabs);

        if (entry.level !== level) {
            var attrs = Object.assign({}, node.attrs, { level: level });
            tr.setNodeMarkup(entry.pos, null, attrs);
        }

        if (parsed[u].text !== entry.text) {
            var from = entry.pos + 1;
            var to = entry.pos + node.nodeSize - 1;
            var textNode = editor.state.schema.text(parsed[u].text);
            if (to > from) tr.replaceWith(from, to, textNode);
            else tr.insert(from, textNode);
        }
    }

    // 3. TOC com mais linhas → títulos novos no fim da nota.
    for (var n = limit; n < parsed.length; n++) {
        tr.insert(tr.doc.content.size, editor.state.schema.nodes.heading.create(
            { level: tocLevel(parsed[n].tabs) },
            editor.state.schema.text(parsed[n].text)
        ));
    }

    if (tr.steps.length) editor.view.dispatch(tr);
}
