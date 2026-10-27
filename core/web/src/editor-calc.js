// editor-calc.js — Nó `calcBlock` do editor: bloco de cálculo (/calculator)
//
// O bloco serializa como um fenced block ```calc no markdown (só as expressões)
// e é reparseado na abertura da nota. Os RESULTADOS nunca entram no markdown:
// eles vivem em decorações ProseMirror (efêmeros por decisão — §6.31, mesma
// regra do bloco consulta §6.29.4) e são recalculados a cada mudança de estado.
//
// Layout: cada linha é um <p> e o resultado é um widget no fim da linha; o CSS
// do editor (.ProseMirror .calc-block) usa flex + margin-left:auto para alinhar
// o resultado à direita, na mesma linha da expressão.
//
// @ts-nocheck — ver critério de editor-slash.js; a lógica pura está em
// calc-engine.js e é coberta por tests/calc-engine.unit.cjs.

import { Node } from "@tiptap/core";
import { Plugin, PluginKey } from "@tiptap/pm/state";
import { Decoration, DecorationSet } from "@tiptap/pm/view";
import { evaluateBlock, formatResult } from "./calc-engine.js";

var calcPluginKey = new PluginKey("calcResults");

// Cache por objeto `doc`: só recalcula quando o documento muda.
var resultsCache = new WeakMap();

/** @param {{ error?: string, value?: number }} result */
function resultWidget(result) {
    var span = document.createElement("span");
    // Fora do fluxo de edição: o cursor não entra no resultado e ele não é
    // serializado (decorations não fazem parte do estado).
    span.setAttribute("contenteditable", "false");
    if (result.error) {
        span.className = "calc-error";
        span.textContent = "⚠ " + result.error;
        span.title = result.error;
    } else {
        var formatted = formatResult(result.value);
        span.className = "calc-result";
        span.textContent = "= " + formatted;
        span.title = "resultado: " + formatted;
    }
    return span;
}

function calcDecorations(state) {
    var cached = resultsCache.get(state.doc);
    if (cached) return cached;

    var decos = [];
    state.doc.descendants(function (node, pos) {
        if (node.type.name !== "calcBlock") return true;

        var lines = [];
        node.forEach(function (row) {
            lines.push(row.textContent);
        });
        var results = evaluateBlock(lines);

        node.forEach(function (row, offset, index) {
            var result = results[index];
            if (!result) return;
            // pos aponta para antes do calcBlock; o conteúdo começa em pos+1.
            // row.nodeSize inclui os dois tokens do <p>; -1 aponta para o fim
            // do CONTEÚDO (dentro do <p>), senão o widget vira irmão do <p> e
            // o resultado cai numa linha própria abaixo.
            var end = pos + 1 + offset + row.nodeSize - 1;
            decos.push(
                Decoration.widget(
                    end,
                    (function (r) {
                        return function () {
                            return resultWidget(r);
                        };
                    })(result),
                    { side: 1 }
                )
            );
        });
        return false;
    });

    var set = DecorationSet.create(state.doc, decos);
    resultsCache.set(state.doc, set);
    return set;
}

export var CalcBlock = Node.create({
    name: "calcBlock",
    group: "block",
    content: "paragraph+",
    defining: true,

    parseHTML: function () {
        return [
            {
                // O fenced block ```calc do markdown vira <pre><code
                // class="language-calc">; o transformamos em <div
                // class="calc-block"> no parse.updateDOM abaixo, então aqui só
                // precisamos reconhecer o nosso próprio div (inclusive ao colar
                // HTML copiado de outra nota).
                tag: "div",
                getAttrs: function (el) {
                    var element = /** @type {Element} */ (el);
                    return element.classList && element.classList.contains("calc-block")
                        ? null
                        : false;
                },
            },
        ];
    },

    renderHTML: function ({ HTMLAttributes }) {
        return ["div", { ...HTMLAttributes, class: "calc-block" }, 0];
    },

    addStorage: function () {
        return {
            markdown: {
                // ```calc\nlinha1\nlinha2\n``` — só as expressões, nunca resultados.
                serialize: function (state, node) {
                    state.write("```calc\n");
                    node.forEach(function (row) {
                        state.text(row.textContent, false);
                        state.write("\n");
                    });
                    state.write("```");
                    state.closeBlock(node);
                },
                parse: {
                    // Converte <pre><code class="language-calc"> em <div
                    // class="calc-block"> com um <p> por linha, ANTES do parser
                    // do ProseMirror ver o DOM (assim não há conflito com a regra
                    // <pre> do codeBlock).
                    updateDOM: function (element) {
                        var pres = element.querySelectorAll("pre");
                        pres.forEach(function (pre) {
                            var code = pre.firstElementChild;
                            if (!code || !code.classList.contains("language-calc")) return;
                            var block = document.createElement("div");
                            block.className = "calc-block";
                            var raw = (code.textContent || "")
                                .replace(/\r\n/g, "\n")
                                .replace(/\n$/, "");
                            raw.split("\n").forEach(function (lineText) {
                                var p = document.createElement("p");
                                p.textContent = lineText;
                                block.appendChild(p);
                            });
                            pre.replaceWith(block);
                        });
                    },
                },
            },
        };
    },

    addProseMirrorPlugins: function () {
        return [
            new Plugin({
                key: calcPluginKey,
                props: {
                    decorations: function (state) {
                        return calcDecorations(state);
                    },
                },
            }),
        ];
    },
});
