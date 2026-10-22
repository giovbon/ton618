/**
 * app-uploads.unit.cjs — cobre os uploads via XMLHttpRequest do app.js.
 *
 * O app.js é um script de browser (sem exports): avaliamos o arquivo real em um
 * ambiente fake mínimo e disparamos os mesmos eventos que a página dispara.
 * Assim o teste protege o comportamento (URL, auth, FormData, progresso e
 * estados de erro) antes e depois de extrair o helper comum.
 */
const test = require("node:test");
const assert = require("node:assert/strict");
const path = require("node:path");

// ── DOM fake mínimo ──
function makeClassList() {
    const classes = new Set();
    return {
        add: (c) => classes.add(c),
        remove: (c) => classes.delete(c),
        contains: (c) => classes.has(c),
        _classes: classes,
    };
}

function makeElement(id) {
    return {
        id,
        files: null,
        value: "",
        innerHTML: "",
        style: {},
        attrs: {},
        classList: makeClassList(),
        listeners: {},
        addEventListener(type, fn) {
            this.listeners[type] = fn;
        },
        setAttribute(key, value) {
            this.attrs[key] = value;
        },
        getAttribute(key) {
            return this.attrs[key];
        },
    };
}

const elements = new Map();
function getEl(id) {
    if (!elements.has(id)) elements.set(id, makeElement(id));
    return elements.get(id);
}

const documentListeners = {};
const alerts = [];

global.window = {
    location: { pathname: "/", href: "", origin: "http://localhost:6180" },
    EditorCommon: { getAuthHeaders: () => ({ Authorization: "Basic token" }) },
    fetch: () => Promise.resolve({ ok: true, json: () => Promise.resolve({}) }),
};

global.localStorage = {
    getItem: () => "Basic token",
    setItem: () => {},
    removeItem: () => {},
};

global.document = {
    cookie: "",
    activeElement: null,
    body: { style: {}, addEventListener: () => {} },
    getElementById: getEl,
    addEventListener(type, fn) {
        (documentListeners[type] || (documentListeners[type] = [])).push(fn);
    },
};

class FakeFormData {
    constructor() {
        this.entries = [];
    }
    append(key, value) {
        this.entries.push([key, value]);
    }
}

class FakeXHR {
    static instances = [];
    constructor() {
        FakeXHR.instances.push(this);
        this.headers = {};
        this.uploadListeners = {};
        this.upload = {
            addEventListener: (type, fn) => {
                this.uploadListeners[type] = fn;
            },
        };
    }
    open(method, url, async) {
        this.method = method;
        this.url = url;
        this.async = async;
    }
    setRequestHeader(key, value) {
        this.headers[key] = value;
    }
    send(body) {
        this.body = body;
        this.sent = true;
    }
    fireLoad(status) {
        this.status = status;
        this.onload();
    }
    fireError() {
        this.onerror();
    }
}

global.FormData = FakeFormData;
global.XMLHttpRequest = FakeXHR;
global.alert = (message) => alerts.push(message);

require(path.join(__dirname, "..", "src", "app.js"));

function reset() {
    FakeXHR.instances.length = 0;
    alerts.length = 0;
    global.window.location.href = "";
    global.window.EditorCommon = { getAuthHeaders: () => ({ Authorization: "Basic token" }) };
    global.document.body.style = {};
    for (const el of elements.values()) {
        el.innerHTML = "";
        el.value = "";
        el.files = null;
        el.style = {};
        el.classList = makeClassList();
    }
}

function firePdfChange(files) {
    const input = getEl("pdf-file-input");
    input.files = files;
    input.value = "selecionado";
    for (const fn of documentListeners.change || []) fn({ target: input });
    return FakeXHR.instances[FakeXHR.instances.length - 1];
}

function fireZipChange(files) {
    const input = getEl("zip-file-input");
    input.files = files;
    input.listeners.change({ target: input });
    return FakeXHR.instances[FakeXHR.instances.length - 1];
}

test("upload de PDF: envia arquivo, auth, progresso e redireciona no sucesso", () => {
    reset();
    const file = { name: "manual.pdf" };
    const xhr = firePdfChange([file]);

    assert.equal(xhr.method, "POST");
    assert.equal(xhr.url, "/upload");
    assert.equal(xhr.async, true);
    assert.equal(xhr.headers.Authorization, "Basic token");
    assert.deepEqual(xhr.body.entries, [["file", file]]);
    assert.equal(
        getEl("pdf-upload-link").innerHTML,
        '<span class="loading-spinner"></span>Processando...',
    );

    xhr.uploadListeners.progress({ lengthComputable: true, loaded: 50, total: 100 });
    assert.equal(getEl("upload-progress-bar").style.width, "50%");

    xhr.fireLoad(200);
    assert.equal(global.window.location.href, "/");
});

test("upload de PDF: falha HTTP restaura o botão e alerta", () => {
    reset();
    const xhr = firePdfChange([{ name: "manual.pdf" }]);

    xhr.fireLoad(500);

    assert.equal(getEl("pdf-upload-link").innerHTML, "📕 PDF");
    assert.deepEqual(alerts, ["Erro ao fazer upload do PDF."]);
    assert.equal(getEl("upload-progress-container").classList.contains("opacity-0"), true);
    assert.equal(global.window.location.href, "");
});

test("upload de PDF: erro de rede alerta e restaura o botão", () => {
    reset();
    const xhr = firePdfChange([{ name: "manual.pdf" }]);

    xhr.fireError();

    assert.equal(getEl("pdf-upload-link").innerHTML, "📕 PDF");
    assert.deepEqual(alerts, ["Erro ao fazer upload do PDF."]);
});

test("upload de PDF: sem arquivo não dispara XHR", () => {
    reset();
    firePdfChange([]);
    assert.equal(FakeXHR.instances.length, 0);
});

test("upload de anexo: envia todos os arquivos e redireciona no sucesso", () => {
    reset();
    const a = { name: "a.zip" };
    const b = { name: "b.zip" };
    const xhr = fireZipChange([a, b]);

    assert.equal(xhr.method, "POST");
    assert.equal(xhr.url, "/api/upload-attachment");
    assert.equal(xhr.headers.Authorization, "Basic token");
    assert.deepEqual(xhr.body.entries, [
        ["files", a],
        ["files", b],
    ]);
    assert.equal(
        getEl("zip-upload-link").innerHTML,
        '<span class="loading-spinner"></span>ZIPando...',
    );

    xhr.fireLoad(201);
    assert.equal(global.window.location.href, "/");
});

test("upload de anexo: falha HTTP usa a mensagem de criação de anexo", () => {
    reset();
    const xhr = fireZipChange([{ name: "a.zip" }]);

    xhr.fireLoad(400);

    assert.equal(getEl("zip-upload-link").innerHTML, "📦 ANEXO");
    assert.deepEqual(alerts, ["Erro ao criar anexo."]);
});

test("upload de anexo: erro de rede usa a mensagem genérica", () => {
    reset();
    const xhr = fireZipChange([{ name: "a.zip" }]);

    xhr.fireError();

    assert.equal(getEl("zip-upload-link").innerHTML, "📦 ANEXO");
    assert.deepEqual(alerts, ["Erro ao fazer upload."]);
});

test("upload de anexo: sem arquivos não dispara XHR", () => {
    reset();
    fireZipChange([]);
    assert.equal(FakeXHR.instances.length, 0);
});

test("uploads não enviam Authorization quando EditorCommon não está disponível", () => {
    reset();
    global.window.EditorCommon = undefined;

    const pdf = firePdfChange([{ name: "manual.pdf" }]);
    assert.equal(pdf.headers.Authorization, undefined);

    const zip = fireZipChange([{ name: "a.zip" }]);
    assert.equal(zip.headers.Authorization, undefined);
});
