// editor-slash.js — Módulo de slash commands do editor TipTap
// Exporta as funções e dados que o IIFE principal (editor-init.js) precisa.
// @ts-nocheck

/**
 * @typedef {{
 *   id: string,
 *   icon: string,
 *   label: string,
 *   keywords: string[],
 *   action: Function,
 *   tableOnly?: boolean
 * }} SlashCommand
 */

// ── Fábrica de ação simples ──
// Criada em runtime (não pode ser exportada como constante aqui porque precisa
// da referência ao `editor` do IIFE pai). O IIFE injeta os deps via init().

var _editorRef = null;
var _slashPosRef = null;
var _setSlashPos = null;
var _slashFilterRef = null;
var _setSlashFilter = null;
var _hideSlashMenuFn = null;

/**
 * Inicializa o módulo com as referências ao estado compartilhado do IIFE.
 * @param {{ getEditor: Function, getSlashPos: Function, setSlashPos: Function,
 *           getSlashFilter: Function, setSlashFilter: Function,
 *           hideSlashMenu: Function, organizeHeadings: Function }} deps
 */
export function initSlash(deps) {
    _editorRef = deps.getEditor;
    _slashPosRef = deps.getSlashPos;
    _setSlashPos = deps.setSlashPos;
    _slashFilterRef = deps.getSlashFilter;
    _setSlashFilter = deps.setSlashFilter;
    _hideSlashMenuFn = deps.hideSlashMenu;
    _organizeHeadings = deps.organizeHeadings;
}

function makeSlashAction(cmd, attrs) {
    return function () {
        _hideSlashMenuFn();
        var editor = _editorRef();
        var slashPos = _slashPosRef();
        if (slashPos !== null && editor) {
            var $to = editor.state.selection.$from;
            editor.chain().focus().deleteRange({ from: slashPos, to: $to.pos }).run();
            _setSlashPos(null);
            _setSlashFilter("");
        }
        var chain = editor.chain().focus();
        switch (cmd) {
            case "bold": chain.toggleBold(); break;
            case "italic": chain.toggleItalic(); break;
            case "heading": chain.toggleHeading(attrs); break;
            case "bulletList": chain.toggleBulletList(); break;
            case "orderedList": chain.toggleOrderedList(); break;
            case "taskList": chain.toggleTaskList(); break;
            case "blockquote": chain.toggleBlockquote(); break;
            case "codeBlock": chain.toggleCodeBlock(); break;
            case "horizontalRule": chain.setHorizontalRule(); break;
            case "addRowBefore": chain.addRowBefore(); break;
            case "addRowAfter": chain.addRowAfter(); break;
            case "addColumnBefore": chain.addColumnBefore(); break;
            case "addColumnAfter": chain.addColumnAfter(); break;
            case "deleteRow": chain.deleteRow(); break;
            case "deleteColumn": chain.deleteColumn(); break;
            default: return;
        }
        chain.run();
    };
}

/** @type {SlashCommand[]} */
export var SLASH_COMMANDS = [
    { id: "task", icon: "☑", label: "Checklist (Tarefa)", keywords: ["task", "todo", "check", "tarefa", "checklist", "caixa", "mark", "item"], action: null },
    { id: "h1", icon: "H1", label: "Título 1 (H1)", keywords: ["h1", "titulo 1", "heading 1", "header 1", "t1"], action: null },
    { id: "h2", icon: "H2", label: "Título 2 (H2)", keywords: ["h2", "titulo 2", "heading 2", "header 2", "t2"], action: null },
    { id: "h3", icon: "H3", label: "Título 3 (H3)", keywords: ["h3", "titulo 3", "heading 3", "header 3", "t3"], action: null },
    { id: "bullet", icon: "•", label: "Lista com marcadores", keywords: ["bullet", "lista", "list", "ul", "ponto", "marcadores"], action: null },
    { id: "ordered", icon: "1.", label: "Lista numerada", keywords: ["ordered", "numerada", "numero", "ol", "1.", "lista numerada"], action: null },
    { id: "quote", icon: "❝", label: "Citação", keywords: ["quote", "citacao", "blockquote", ">"], action: null },
    { id: "code", icon: "⎔", label: "Bloco de código", keywords: ["code", "codigo", "codeblock", "bloco de codigo", "script"], action: null },
    {
        id: "table", icon: "⊞", label: "Tabela", keywords: ["table", "tabela", "grid", "grade"],
        action: function () {
            _hideSlashMenuFn();
            var editor = _editorRef();
            var slashPos = _slashPosRef();
            if (slashPos !== null && editor) {
                var $to = editor.state.selection.$from;
                editor.chain().focus().deleteRange({ from: slashPos, to: $to.pos })
                    .insertTable({ rows: 3, cols: 3, withHeaderRow: true }).run();
                _setSlashPos(null);
                _setSlashFilter("");
            }
        }
    },
    { id: "hr", icon: "—", label: "Linha horizontal", keywords: ["hr", "linha", "divider", "divisoria", "horizontal"], action: null },
    {
        id: "image", icon: "🖼", label: "Imagem", keywords: ["image", "imagem", "foto", "img", "picture"],
        action: function () {
            _hideSlashMenuFn();
            var editor = _editorRef();
            var slashPos = _slashPosRef();
            if (slashPos !== null && editor) {
                var $to = editor.state.selection.$from;
                editor.chain().focus().deleteRange({ from: slashPos, to: $to.pos }).run();
                _setSlashPos(null);
                _setSlashFilter("");
            }
            document.getElementById("editor-image-input").click();
        }
    },
    {
        id: "organize", icon: "☰", label: "Organizar títulos", keywords: ["organize", "organizar", "titulos", "headers", "hierarquia"],
        action: function () {
            _hideSlashMenuFn();
            var editor = _editorRef();
            var slashPos = _slashPosRef();
            if (slashPos !== null && editor) {
                editor.chain().focus().deleteRange({ from: slashPos, to: editor.state.selection.$from.pos }).run();
                _setSlashPos(null);
                _setSlashFilter("");
            }
            _organizeHeadings();
        }
    },
    { id: "addRowBefore", icon: "➕⬆️", label: "Adicionar linha acima", keywords: ["linha", "above", "acima", "row"], tableOnly: true, action: null },
    { id: "addRowAfter", icon: "➕⬇️", label: "Adicionar linha abaixo", keywords: ["linha", "below", "abaixo", "row"], tableOnly: true, action: null },
    { id: "addColumnBefore", icon: "➕⬅️", label: "Adicionar coluna antes", keywords: ["coluna", "before", "antes", "column"], tableOnly: true, action: null },
    { id: "addColumnAfter", icon: "➕➡️", label: "Adicionar coluna depois", keywords: ["coluna", "after", "depois", "column"], tableOnly: true, action: null },
    { id: "deleteRow", icon: "🗑️⬇️", label: "Excluir linha", keywords: ["excluir", "deletar", "linha", "delete", "row"], tableOnly: true, action: null },
    { id: "deleteColumn", icon: "🗑️➡️", label: "Excluir coluna", keywords: ["excluir", "deletar", "coluna", "delete", "column"], tableOnly: true, action: null },
    {
        id: "deleteTable", icon: "🗑️", label: "Excluir tabela", keywords: ["excluir", "deletar", "tabela", "delete", "table"], tableOnly: true,
        action: function () {
            _hideSlashMenuFn();
            var editor = _editorRef();
            var slashPos = _slashPosRef();
            if (slashPos !== null && editor) {
                editor.chain().focus().deleteRange({ from: slashPos, to: editor.state.selection.$from.pos }).run();
                _setSlashPos(null);
                _setSlashFilter("");
            }
            if (editor && confirm("Excluir esta tabela?")) {
                editor.chain().focus().deleteTable().run();
            }
        }
    },
];

/**
 * Preenche as actions dos comandos simples (que precisam de makeSlashAction).
 * Chamado após initSlash.
 */
export function buildSlashActions() {
    var actionMap = {
        task: makeSlashAction("taskList"),
        h1: makeSlashAction("heading", { level: 1 }),
        h2: makeSlashAction("heading", { level: 2 }),
        h3: makeSlashAction("heading", { level: 3 }),
        bullet: makeSlashAction("bulletList"),
        ordered: makeSlashAction("orderedList"),
        quote: makeSlashAction("blockquote"),
        code: makeSlashAction("codeBlock"),
        hr: makeSlashAction("horizontalRule"),
        addRowBefore: makeSlashAction("addRowBefore"),
        addRowAfter: makeSlashAction("addRowAfter"),
        addColumnBefore: makeSlashAction("addColumnBefore"),
        addColumnAfter: makeSlashAction("addColumnAfter"),
        deleteRow: makeSlashAction("deleteRow"),
        deleteColumn: makeSlashAction("deleteColumn"),
    };
    SLASH_COMMANDS.forEach(function (c) {
        if (actionMap[c.id]) c.action = actionMap[c.id];
    });
}

/**
 * Pontua relevância de um comando para um dado filtro.
 * @param {SlashCommand} cmd
 * @param {string} query
 * @returns {number}
 */
export function scoreSlashCommand(cmd, query) {
    if (!query) return 100;
    var q = query.toLowerCase().trim();
    if (cmd.keywords) {
        for (var k = 0; k < cmd.keywords.length; k++) {
            var kw = cmd.keywords[k].toLowerCase();
            if (kw === q) return 1000;
            if (kw.startsWith(q)) return 800 - k * 10;
        }
    }
    var labelLower = cmd.label.toLowerCase();
    if (labelLower.startsWith(q)) return 700;
    if (cmd.keywords) {
        for (var k = 0; k < cmd.keywords.length; k++) {
            if (cmd.keywords[k].toLowerCase().indexOf(q) !== -1) return 500 - k * 10;
        }
    }
    var labelIdx = labelLower.indexOf(q);
    if (labelIdx !== -1) return 300 - labelIdx;
    return 0;
}
