// ── Lucide Icons Rendering (DEPRECATED - ícones são inline SVG via server) ──
// Mantido como noop para compatibilidade com código legado que chama renderLucideIcons()
export function renderLucideIcons() {
    // Icons now rendered server-side in icons.templ - no client-side rendering needed
}

// @ts-ignore
window.renderLucideIcons = renderLucideIcons;

if (typeof document !== 'undefined') {
    document.addEventListener("DOMContentLoaded", function () {
        renderLucideIcons();
    });
    // @ts-ignore
    if (typeof htmx !== 'undefined') {
        document.body.addEventListener("htmx:afterSwap", function () {
            renderLucideIcons();
        });
    }
}

// ── Auth Header helper (para XHR/XMLHttpRequest) ──
// O restante do app injeta o header Authorization via fetch/HTMX,
// mas uploads via XHR (PDF/anexo) não fazem isso automaticamente.
// Sem esse header, quando o cookie ton_auth expira (24h) mas o
// localStorage ainda tem o token, o servidor responde 401 e o
// upload falha com "Erro ao criar anexo.".
function getAuthHeader() {
    var auth = localStorage.getItem("ton_auth");
    if (!auth) {
        var cookieMatch = document.cookie.match(/(?:^|;\s*)ton_auth=([^;]+)/);
        if (cookieMatch) {
            var cookieVal = decodeURIComponent(cookieMatch[1]);
            auth = cookieVal.startsWith("Basic ")
                ? cookieVal
                : "Basic " + cookieVal;
        }
    }
    return auth;
}

// ── PDF Upload ──
/** @type {HTMLElement | null} */
var activePdfUploadButton = null;

document.addEventListener("change", function (e) {
    const target = /** @type {HTMLInputElement} */ (e.target);
    if (target && target.id === "pdf-file-input") {
        var file = target.files ? target.files[0] : null;
        if (!file) return;
        var link =
            activePdfUploadButton ||
            document.getElementById("pdf-upload-link");
        setButtonLoading(link, true, "Processando...", "📕");
        showProgressBar(0, false);
        var fd = new FormData();
        fd.append("file", file);
        
        var xhr = new XMLHttpRequest();
        xhr.open("POST", "/upload", true);
        var auth = getAuthHeader();
        if (auth) {
            xhr.setRequestHeader("Authorization", auth);
        }
        
        xhr.upload.addEventListener("progress", function (evt) {
            if (evt.lengthComputable) {
                var pct = Math.round((evt.loaded / evt.total) * 100);
                showProgressBar(pct, false);
            }
        });
        
        xhr.onload = function () {
            if (xhr.status >= 200 && xhr.status < 300) {
                window.location.href = "/";
            } else {
                setButtonLoading(link, false, "PDF", "📕");
                hideProgressBar();
                alert("Erro ao fazer upload do PDF.");
            }
        };
        
        xhr.onerror = function () {
            setButtonLoading(link, false, "PDF", "📕");
            hideProgressBar();
            alert("Erro ao fazer upload do PDF.");
        };
        
        xhr.send(fd);
        target.value = "";
    }
});

// ── ZIP Upload ──
const zipInput = document.getElementById("zip-file-input");
if (zipInput) {
    zipInput.addEventListener("change", function (e) {
        const target = /** @type {HTMLInputElement} */ (e.target);
        var files = target.files;
        if (!files || files.length === 0) return;
        var fd = new FormData();
        for (var i = 0; i < files.length; i++) {
            fd.append("files", files[i]);
        }
        var link = document.getElementById("zip-upload-link");
        setButtonLoading(link, true, "ZIPando...", "📦");
        showProgressBar(0, false);

        var xhr = new XMLHttpRequest();
        xhr.open("POST", "/api/upload-attachment", true);
        var auth = getAuthHeader();
        if (auth) {
            xhr.setRequestHeader("Authorization", auth);
        }
        
        xhr.upload.addEventListener("progress", function (evt) {
            if (evt.lengthComputable) {
                var pct = Math.round((evt.loaded / evt.total) * 100);
                showProgressBar(pct, false);
            }
        });
        
        xhr.onload = function () {
            if (xhr.status >= 200 && xhr.status < 300) {
                window.location.href = "/";
            } else {
                setButtonLoading(link, false, "ANEXO", "📦");
                hideProgressBar();
                alert("Erro ao criar anexo.");
            }
        };
        
        xhr.onerror = function () {
            setButtonLoading(link, false, "ANEXO", "📦");
            hideProgressBar();
            alert("Erro ao fazer upload.");
        };
        
        xhr.send(fd);
    });
}

const menuToggle = document.getElementById("mobile-menu-toggle");
if (menuToggle) {
    menuToggle.addEventListener("click", function () {
        var menu = document.getElementById("mobile-menu");
        if (!menu) return;
        var expanded = menuToggle.getAttribute("aria-expanded") === "true";
        if (expanded) {
            menu.classList.add("hidden");
            menuToggle.setAttribute("aria-expanded", "false");
        } else {
            menu.classList.remove("hidden");
            menuToggle.setAttribute("aria-expanded", "true");
        }
    });
}

// ── Captura de artigo/YouTube ──
/**
 * Prompts user for a URL and posts it to the capture API.
 * 
 * @param {HTMLElement | null} button 
 * @returns {boolean}
 */
function promptCapture(button) {
    var url = prompt("Insira a URL do artigo ou video do YouTube:");
    if (!url) return false;
    if (button) {
        setButtonLoading(button, true, "Capturando...", "🌐");
    }
    showProgressBar(100, true);
    var encodedUrl = btoa(encodeURIComponent(url));
    fetch("/api/capture", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ url: encodedUrl }),
    })
        .then(function (r) {
            if (!r.ok)
                return r.text().then(function (t) {
                    var trimmed = t.trim();
                    if (trimmed.indexOf("<!DOCTYPE") === 0 || trimmed.indexOf("<html") === 0 || trimmed.indexOf("<!doctype") === 0) {
                        var match = t.match(/<title>([\s\S]*?)<\/title>/i);
                        if (match && match[1]) {
                            throw new Error("Servidor retornou HTML: " + match[1].trim());
                        }
                        throw new Error("Servidor retornou HTML (possivel bloqueio de WAF/Proxy)");
                    }
                    throw new Error(t);
                });
            return r.json();
        })
        .then(function (data) {
            window.location.href =
                "/editor?file=" + encodeURIComponent(data.filename);
        })
        .catch(function (err) {
            if (button) {
                setButtonLoading(button, false, "CAPTURA", "🌐");
            }
            hideProgressBar();
            alert("Erro ao capturar: " + err.message);
        });
    return false;
}

// @ts-ignore
window.promptCapture = promptCapture;

/**
 * Sets button loading state with spinner.
 * 
 * @param {HTMLElement | null} el 
 * @param {boolean} loading 
 * @param {string} text 
 * @param {string} [icon] 
 */
function setButtonLoading(el, loading, text, icon) {
    if (!el) return;
    if (loading) {
        el.classList.add("button-loading");
        el.innerHTML = '<span class="loading-spinner"></span>' + text;
    } else {
        el.classList.remove("button-loading");
        el.innerHTML = icon ? icon + " " + text : text;
    }
}

/**
 * Displays upload progress bar.
 * 
 * @param {number} pct 
 * @param {boolean} isIndeterminate 
 */
function showProgressBar(pct, isIndeterminate) {
    var container = document.getElementById("upload-progress-container");
    var bar = document.getElementById("upload-progress-bar");
    var glow = document.getElementById("upload-progress-glow");
    if (!container || !bar || !glow) return;

    container.classList.remove("opacity-0");
    container.classList.add("opacity-100");

    if (isIndeterminate) {
        bar.style.width = "100%";
        glow.classList.remove("hidden");
    } else {
        bar.style.width = pct + "%";
        if (pct >= 100) {
            glow.classList.remove("hidden");
        } else {
            glow.classList.add("hidden");
        }
    }
    
    document.body.style.pointerEvents = "none";
}

function hideProgressBar() {
    var container = document.getElementById("upload-progress-container");
    var bar = document.getElementById("upload-progress-bar");
    var glow = document.getElementById("upload-progress-glow");
    if (!container || !bar || !glow) return;

    container.classList.remove("opacity-100");
    container.classList.add("opacity-0");
    
    document.body.style.pointerEvents = "";

    setTimeout(function () {
        if (container.classList.contains("opacity-0")) {
            bar.style.width = "0%";
            glow.classList.add("hidden");
        }
    }, 300);
}

/**
 * @returns {boolean}
 */
function createNewNote() {
    window.location.href = "/editor";
    return false;
}

// @ts-ignore
window.createNewNote = createNewNote;

// ── Auth Integration ──
function logout() {
    localStorage.removeItem("ton_auth");
    document.cookie = "ton_auth=;path=/;max-age=0";
    window.location.href = "/login";
}

// @ts-ignore
window.logout = logout;

// ── Stopwords Customizadas (Gerenciado via HTMX) ──
(function () {
    var auth = localStorage.getItem("ton_auth");

    if (!auth) {
        var cookieMatch = document.cookie.match(/(?:^|;\s*)ton_auth=([^;]+)/);
        if (cookieMatch) {
            var cookieVal = decodeURIComponent(cookieMatch[1]);
            var basicToken = cookieVal.startsWith("Basic ")
                ? cookieVal
                : "Basic " + cookieVal;
            localStorage.setItem("ton_auth", basicToken);
            auth = basicToken;
        }
    }

    var path = window.location.pathname;
    if (!auth && path !== "/login" && !path.startsWith("/static/")) {
        window.location.href = "/login";
        return;
    }

    // @ts-ignore
    if (typeof htmx !== "undefined") {
        document.body.addEventListener(
            "htmx:configRequest",
            function (evt) {
                // @ts-ignore
                var isLocal = evt.detail.path.startsWith('/') || (!evt.detail.path.startsWith('http://') && !evt.detail.path.startsWith('https://')) || evt.detail.path.startsWith(window.location.origin);
                // @ts-ignore
                if (isLocal && auth) evt.detail.headers["Authorization"] = auth;
            },
        );
        document.body.addEventListener(
            "htmx:afterSwap",
            function (evt) {
                // @ts-ignore
                if (typeof Alpine !== "undefined" && evt.detail.target) {
                    // @ts-ignore
                    Alpine.initTree(evt.detail.target);
                }
            }
        );
    }

    var origFetch = window.fetch;
    window.fetch = function (url, opts) {
        opts = opts || {};
        opts.headers = opts.headers || {};

        var isSameOrigin = false;
        if (typeof url === 'string') {
            if (url.startsWith('/') || (!url.startsWith('http://') && !url.startsWith('https://')) || url.startsWith(window.location.origin)) {
                isSameOrigin = true;
            }
        } else if (url instanceof URL) {
            if (url.origin === window.location.origin) {
                isSameOrigin = true;
            }
        } else if (url && typeof url.url === 'string') {
            if (url.url.startsWith('/') || (!url.url.startsWith('http://') && !url.url.startsWith('https://')) || url.url.startsWith(window.location.origin)) {
                isSameOrigin = true;
            }
        }

        if (isSameOrigin && auth && !opts.headers["Authorization"]) {
            if (opts.headers instanceof Headers) {
                opts.headers.set("Authorization", auth);
            } else {
                opts.headers["Authorization"] = auth;
            }
        }
        return origFetch(url, opts);
    };
})();

// ── Marcadores / contagem de Tasks ──
//
// ⚠️ NÃO reintroduzir aqui contagem/atualização do badge de Tasks do cabeçalho.
//
// Existia neste arquivo uma função `updateTodosCount()` que buscava
// `/api/todos?type=all&status=pending&format=json` e sobrescrevia o conteúdo de
// `#nav-todos` com `innerHTML = ...`. Isso DESTRUÍA o `<span id="todos-badge">`
// (junto com os atributos hx-get/hx-trigger) em toda carga de página, com três
// efeitos: (1) o número exibido vinha daquele endpoint antigo, que ignora
// `todo_markers.count_in_badge` e `todo_markers.active` — ou seja, o checkbox
// "Contar" das configurações não tinha efeito no desktop; (2) o swap
// out-of-band (`hx-swap-oob="innerHTML:#todos-badge"`) e o evento
// `todos-updated` não tinham mais onde ser aplicados, então o badge era o único
// elemento da UI que nunca se atualizava sem reload; (3) no mobile o número era
// injetado no rótulo `TASK N`, duplicando a contagem ao lado do badge real.
//
// A responsabilidade é do HTMX hoje: o container vive em `layout/navbar.templ`
// (`hx-get="/api/todos/count"` + `hx-trigger="load, todos-updated from:body"`) e
// quem quiser atualizar o número dispara o evento `todos-updated` no `body`.
// O destaque do link ativo do cabeçalho também é feito no próprio navbar.templ.
