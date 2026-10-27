// calc-engine.js — Motor de avaliação do bloco de cálculo (```calc)
//
// Avalia expressões aritméticas linha a linha, com escopo sequencial por bloco:
//
//   var1 = 10        → 10
//   var2 = 20        → 20
//   var1 + var2      → 30
//
// Decisões de projeto (docs/DECISIONS.md §6.31):
//  - SEM eval/new Function: tokenizer + parser Pratt + interpretador de AST.
//    O app tem CSP restritiva e não executa código — este módulo só interpreta
//    a gramática abaixo, que é fechada (sem I/O, sem laços, sem funções do usuário).
//  - Toda expressão termina: limite de 200 símbolos e profundidade 32 por linha,
//    1000 linhas por bloco. Não há caminho para laço infinito.
//  - Erros em pt-BR com posição, para exibir inline na linha.
//  - Resultado efêmero: o motor só alimenta a decoração do editor; nada aqui é
//    gravado no markdown da nota (mesma regra do bloco consulta, §6.29.4).
//
// Gramática aceita:
//   número      123 · 12,5 (aceita vírgula OU ponto) · .5 · 1e6 · 2E+9
//   operadores  + - * / % (módulo) ^ (potência, associativa à direita)
//   variáveis   nome = expr (atribui e mostra o valor) · uso: nome
//   constantes  pi · e
//   funções     sqrt, cbrt, abs, round, floor, ceil, min, max, pow, log, ln,
//               sin, cos, tan (ângulos em radianos) · med(v1, v2, …) = média
//               aritmética com quantos valores quiser (mínimo 1)
//   comentário  linha que começa com #
//
// @ts-nocheck — parser próprio: tipagem dinâmica proposital (mesmo critério
// de editor-slash.js); validado por testes unitários (tests/calc-engine.unit.cjs).

var MAX_TOKENS = 200;
var MAX_DEPTH = 32;
var MAX_BLOCK_LINES = 1000;
// Teto por chamada (proteção de parsing); na prática o limite real de
// argumentos vem do teto de símbolos da linha (MAX_TOKENS).
var MAX_CALL_ARGS = 64;
var MIN_VARIADIC_ARGS = 1;

/** Monta um Error de cálculo em pt-BR, com posição (base 1) quando há. */
function calcError(message, pos) {
    var full = pos === undefined || pos === null
        ? message
        : message + " (posição " + (pos + 1) + ")";
    var err = new Error(full);
    err.name = "CalcError";
    return err;
}

var RE_IDENT_START = /[\p{L}_]/u;
var RE_IDENT_PART = /[\p{L}\p{N}_]/u;

function isDigit(c) {
    return c >= "0" && c <= "9";
}

// ── Tokenizer ──────────────────────────────────────────────────────────────

function tokenize(src) {
    var tokens = [];
    var i = 0;
    var n = src.length;
    while (i < n) {
        if (tokens.length > MAX_TOKENS) {
            throw calcError("linha muito longa (máximo de " + MAX_TOKENS + " símbolos)", i);
        }
        var c = src.charAt(i);
        if (c === " " || c === "\t") {
            i++;
            continue;
        }
        // Número: dígitos, opcionalmente com separador decimal único e
        // notação científica opcional.
        if (isDigit(c) || ((c === "." || c === ",") && isDigit(src.charAt(i + 1)))) {
            var numStart = i;
            while (i < n && isDigit(src.charAt(i))) i++;
            var usedSep = false;
            while (
                !usedSep &&
                i < n &&
                (src.charAt(i) === "." || src.charAt(i) === ",") &&
                isDigit(src.charAt(i + 1))
            ) {
                usedSep = true;
                i++;
                while (i < n && isDigit(src.charAt(i))) i++;
            }
            if (i < n && (src.charAt(i) === "e" || src.charAt(i) === "E")) {
                var j = i + 1;
                if (j < n && (src.charAt(j) === "+" || src.charAt(j) === "-")) j++;
                if (j < n && isDigit(src.charAt(j))) {
                    j++;
                    while (j < n && isDigit(src.charAt(j))) j++;
                    i = j;
                }
            }
            tokens.push({
                t: "num",
                v: parseFloat(src.slice(numStart, i).replace(",", ".")),
                pos: numStart,
            });
            continue;
        }
        // Identificador (aceita letras com acento)
        if (RE_IDENT_START.test(c)) {
            var idStart = i;
            i++;
            while (i < n && RE_IDENT_PART.test(src.charAt(i))) i++;
            tokens.push({ t: "ident", v: src.slice(idStart, i), pos: idStart });
            continue;
        }
        // Operadores e delimitadores
        if ("+-*/%^()=,".indexOf(c) !== -1) {
            tokens.push({ t: c, pos: i });
            i++;
            continue;
        }
        throw calcError('caractere não permitido "' + c + '"', i);
    }
    tokens.push({ t: "eof", pos: n });
    return tokens;
}

// ── Parser (precedence climbing / Pratt) ───────────────────────────────────

var PRECEDENCE = { "+": 10, "-": 10, "*": 20, "/": 20, "%": 20, "^": 40 };

function parseTokens(tokens) {
    var idx = 0;
    var depth = 0;

    function peek() {
        return tokens[idx];
    }
    function next() {
        return tokens[idx++];
    }

    function parseExpr(minPrec) {
        depth++;
        if (depth > MAX_DEPTH) {
            throw calcError("expressão muito aninhada (máximo de " + MAX_DEPTH + " níveis)");
        }
        var left = parseUnary();
        for (;;) {
            var tk = peek();
            var prec = Object.prototype.hasOwnProperty.call(PRECEDENCE, tk.t)
                ? PRECEDENCE[tk.t]
                : undefined;
            if (prec === undefined || prec < minPrec) break;
            next();
            // ^ é associativa à direita (2^3^2 = 2^(3^2)); os demais, à esquerda.
            var right = parseExpr(tk.t === "^" ? prec : prec + 1);
            left = { k: "bin", op: tk.t, l: left, r: right };
        }
        depth--;
        return left;
    }

    function parseUnary() {
        var tk = peek();
        if (tk.t === "-" || tk.t === "+") {
            next();
            // Unário mais frouxo que ^ (−2^2 = −4), mais firme que + − * / %.
            var e = parseExpr(30);
            return tk.t === "-" ? { k: "neg", e: e } : e;
        }
        return parsePrimary();
    }

    function parsePrimary() {
        var tk = next();
        if (tk.t === "num") {
            return { k: "num", v: tk.v };
        }
        if (tk.t === "ident") {
            if (peek().t === "(") {
                next();
                var args = [];
                if (peek().t !== ")") {
                    args.push(parseExpr(0));
                    while (peek().t === ",") {
                        next();
                        args.push(parseExpr(0));
                    }
                }
                var close = next();
                if (close.t !== ")") throw calcError('esperado ")"', close.pos);
                if (args.length > MAX_CALL_ARGS) {
                    throw calcError(
                        '"' + tk.v + '" recebe muitos valores (máximo de ' + MAX_CALL_ARGS + ")",
                        tk.pos
                    );
                }
                return { k: "call", name: tk.v, args: args };
            }
            return { k: "var", name: tk.v, pos: tk.pos };
        }
        if (tk.t === "(") {
            var inner = parseExpr(0);
            var closeParen = next();
            if (closeParen.t !== ")") throw calcError('esperado ")"', closeParen.pos);
            return inner;
        }
        if (tk.t === "eof") throw calcError("expressão incompleta", tk.pos);
        throw calcError('símbolo inesperado "' + tk.t + '"', tk.pos);
    }

    function assertEnd() {
        var tk = peek();
        if (tk.t === "eof") return;
        if (tk.t === "num" || tk.t === "ident") {
            throw calcError('falta um operador antes de "' + String(tk.v) + '"', tk.pos);
        }
        throw calcError('símbolo inesperado "' + tk.t + '"', tk.pos);
    }

    // Atribuição no início da linha: "nome = expr"
    if (tokens[0].t === "ident" && tokens[1] && tokens[1].t === "=") {
        var name = tokens[0].v;
        idx = 2;
        var rhs = parseExpr(0);
        assertEnd();
        return { assign: name, expr: rhs };
    }
    var expr = parseExpr(0);
    assertEnd();
    return { assign: null, expr: expr };
}

// ── Interpretador ──────────────────────────────────────────────────────────

var CONSTANTS = { pi: Math.PI, e: Math.E };

var FUNCTIONS = {
    sqrt: { arity: 1, fn: function (a) { return Math.sqrt(a[0]); } },
    cbrt: { arity: 1, fn: function (a) { return Math.cbrt(a[0]); } },
    abs: { arity: 1, fn: function (a) { return Math.abs(a[0]); } },
    round: { arity: 1, fn: function (a) { return Math.round(a[0]); } },
    floor: { arity: 1, fn: function (a) { return Math.floor(a[0]); } },
    ceil: { arity: 1, fn: function (a) { return Math.ceil(a[0]); } },
    min: { arity: 2, fn: function (a) { return Math.min(a[0], a[1]); } },
    max: { arity: 2, fn: function (a) { return Math.max(a[0], a[1]); } },
    pow: { arity: 2, fn: function (a) { return Math.pow(a[0], a[1]); } },
    log: { arity: 1, fn: function (a) { return Math.log10(a[0]); } },
    ln: { arity: 1, fn: function (a) { return Math.log(a[0]); } },
    sin: { arity: 1, fn: function (a) { return Math.sin(a[0]); } },
    cos: { arity: 1, fn: function (a) { return Math.cos(a[0]); } },
    tan: { arity: 1, fn: function (a) { return Math.tan(a[0]); } },
    // Aridade variável: med(10, 20, 30) … quantos valores queira (≥ 1).
    med: {
        variadic: true,
        minArgs: MIN_VARIADIC_ARGS,
        fn: function (a) {
            var sum = 0;
            for (var i = 0; i < a.length; i++) sum += a[i];
            return sum / a.length;
        },
    },
};

function finite(v) {
    if (Number.isNaN(v)) {
        throw calcError("resultado inválido (NaN) — confira raízes de negativos e logaritmos");
    }
    if (!Number.isFinite(v)) {
        throw calcError("resultado fora do intervalo (infinito) — confira divisões e potências");
    }
    return v;
}

function evalNode(node, scope) {
    switch (node.k) {
        case "num":
            return node.v;
        case "neg":
            return -evalNode(node.e, scope);
        case "var": {
            if (scope && Object.prototype.hasOwnProperty.call(scope, node.name)) {
                return scope[node.name];
            }
            if (Object.prototype.hasOwnProperty.call(CONSTANTS, node.name)) {
                return CONSTANTS[node.name];
            }
            throw calcError(
                '"' + node.name + '" não foi definida — defina antes com ' + node.name + " = valor",
                node.pos
            );
        }
        case "bin": {
            var l = evalNode(node.l, scope);
            var r = evalNode(node.r, scope);
            var out;
            switch (node.op) {
                case "+": out = l + r; break;
                case "-": out = l - r; break;
                case "*": out = l * r; break;
                case "/":
                    if (r === 0) throw calcError("divisão por zero");
                    out = l / r;
                    break;
                case "%":
                    if (r === 0) throw calcError("módulo por zero");
                    out = l % r;
                    break;
                case "^":
                    out = Math.pow(l, r);
                    break;
                default:
                    throw calcError('operador desconhecido "' + node.op + '"');
            }
            return finite(out);
        }
        case "call": {
            if (!Object.prototype.hasOwnProperty.call(FUNCTIONS, node.name)) {
                throw calcError(
                    'função desconhecida "' + node.name + '" — disponíveis: ' +
                        Object.keys(FUNCTIONS).join(", "),
                    undefined
                );
            }
            var spec = FUNCTIONS[node.name];
            var args = [];
            for (var ai = 0; ai < node.args.length; ai++) {
                args.push(evalNode(node.args[ai], scope));
            }
            if (spec.variadic) {
                if (args.length < spec.minArgs) {
                    throw calcError(
                        '"' + node.name + '" espera ao menos ' + spec.minArgs +
                            (spec.minArgs === 1 ? " valor, recebeu " : " valores, recebeu ") +
                            args.length
                    );
                }
            } else if (args.length !== spec.arity) {
                throw calcError(
                    '"' + node.name + '" espera ' + spec.arity +
                        (spec.arity === 1 ? " valor, recebeu " : " valores, recebeu ") +
                        args.length
                );
            }
            return finite(spec.fn(args));
        }
        default:
            throw calcError("expressão inválida");
    }
}

// ── API ────────────────────────────────────────────────────────────────────

/**
 * Avalia uma linha do bloco.
 * @param {string} line Linha original (comentaristas `#` e vazias retornam null)
 * @param {Object} scope Escopo compartilhado do bloco (mutado em atribuições)
 * @returns {null | { value: number, name: string | null } | { error: string }}
 *   null = nada a exibir; {value} = resultado; {error} = mensagem em pt-BR.
 */
export function evaluateLine(line, scope) {
    var trimmed = line.replace(/^\s+|\s+$/g, "");
    if (trimmed === "" || trimmed.charAt(0) === "#") return null;
    var target = scope || Object.create(null);
    try {
        var tokens = tokenize(line);
        var statement = parseTokens(tokens);
        var value = evalNode(statement.expr, target);
        if (statement.assign !== null) {
            target[statement.assign] = value;
        }
        return { value: value, name: statement.assign };
    } catch (err) {
        return {
            error: err && err.message ? err.message : "erro ao avaliar a linha",
        };
    }
}

/**
 * Avalia todas as linhas de um bloco em ordem (as atribuições alimentam as
 * linhas seguintes).
 * @param {string[]} lines
 * @returns {Array<null | { value: number, name: string | null } | { error: string }>}
 */
export function evaluateBlock(lines) {
    var scope = Object.create(null);
    var results = [];
    for (var i = 0; i < lines.length; i++) {
        if (i >= MAX_BLOCK_LINES) {
            results.push({ error: "bloco muito grande (máximo de " + MAX_BLOCK_LINES + " linhas)" });
            continue;
        }
        results.push(evaluateLine(lines[i], scope));
    }
    return results;
}

/**
 * Formata um resultado para exibição: inteiro sem casas; decimal com vírgula
 * (pt-BR) e sem separador de milhar. O texto nunca é reinterpretado — os
 * resultados são efêmeros e não voltam para o markdown.
 * @param {number} v
 * @returns {string}
 */
export function formatResult(v) {
    if (Number.isInteger(v)) return String(v);
    if (!Number.isFinite(v)) return "—";
    return v.toLocaleString("pt-BR", { useGrouping: false, maximumFractionDigits: 12 });
}
