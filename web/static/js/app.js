// @ts-check

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

// ── New Note com nome único ──

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
    // Codifica a URL em base64 para evitar WAF (SSRF falso positivo)
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
    
    // Desabilitar interações no body durante o carregamento
    document.body.style.pointerEvents = "none";
}

function hideProgressBar() {
    var container = document.getElementById("upload-progress-container");
    var bar = document.getElementById("upload-progress-bar");
    var glow = document.getElementById("upload-progress-glow");
    if (!container || !bar || !glow) return;

    container.classList.remove("opacity-100");
    container.classList.add("opacity-0");
    
    // Habilitar novamente as interações no body
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

// ── Auth Integration ──
function logout() {
    localStorage.removeItem("ton_auth");
    document.cookie = "ton_auth=;path=/;max-age=0";
    window.location.href = "/login";
}

// ── + Criar Dropdown ──
/**
 * @param {Event} e 
 */
function toggleCriarMenu(e) {
    e.stopPropagation();
    const menu = document.getElementById('criar-menu');
    if (menu) menu.classList.toggle('hidden');
}
function closeCriarMenu() {
    const menu = document.getElementById('criar-menu');
    if (menu) menu.classList.add('hidden');
}
// ── + Captura Dropdown ──
/**
 * @param {Event} e 
 */
function toggleCapturaMenu(e) {
    e.stopPropagation();
    const menu = document.getElementById('captura-menu');
    if (menu) menu.classList.toggle('hidden');
}
function closeCapturaMenu() {
    const menu = document.getElementById('captura-menu');
    if (menu) menu.classList.add('hidden');
}
document.addEventListener('click', function(e) {
    const wrapper = document.getElementById('criar-dropdown-wrapper');
    const target = /** @type {Node} */ (e.target);
    if (wrapper && !wrapper.contains(target)) {
        closeCriarMenu();
    }
    const capWrapper = document.getElementById('captura-dropdown-wrapper');
    if (capWrapper && !capWrapper.contains(target)) {
        closeCapturaMenu();
    }
});

// ── Configurações (⚙️ Settings Modal) ──
// Registrado inline no layout.templ para evitar condições de corrida com o Alpine.js

// ── Age slider: update display live ──
document.addEventListener("DOMContentLoaded", function () {
    /** @type {HTMLInputElement | null} */
    var slider = document.querySelector("#filter-age-slider");
    if (!slider) return;
    function updateAgeDisplay() {
        if (!slider) return;
        var v = parseInt(slider.value);
        const display = document.getElementById("age-value-display");
        if (display) display.textContent = v + (v === 1 ? " ano" : " anos");
    }
    slider.addEventListener("input", updateAgeDisplay);
    updateAgeDisplay();
});

// ── Stopwords Customizadas (Gerenciado via HTMX) ──
(function () {
    var auth = localStorage.getItem("ton_auth");

    // Fallback: recupera token do cookie quando vindo de uma origem diferente
    // (ex: IP local vs URL Cloudflare - localStorage e isolado por origem)
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

    // Redirect to login if not authenticated (skip for login page and static)
    var path = window.location.pathname;
    if (!auth && path !== "/login" && !path.startsWith("/static/")) {
        window.location.href = "/login";
        return;
    }

    // Inject auth header into HTMX requests (same-origin only)
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
                    Alpine.processTree(evt.detail.target);
                }
            }
        );
    }

    // Wrap fetch to always include auth for same-origin requests
    var origFetch = window.fetch;
    window.fetch = function (url, opts) {
        opts = opts || {};
        opts.headers = opts.headers || {};

        // Check if target is same-origin
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

// ── Marcadores (HTMX HATEOAS) ──

async function updateTodosCount() {
    try {
        const response = await fetch("/api/todos?type=all&status=pending&format=json");
        if (!response.ok) return;
        const data = await response.json();
        const count = data.todos ? data.todos.length : 0;

        const navTodos = document.getElementById("nav-todos");
        const mobileNavTodos = document.getElementById("mobile-nav-todos");

        if (navTodos) {
            navTodos.textContent = count > 0 ? `🚧 Task ${count}` : `🚧 Task`;
        }
        if (mobileNavTodos) {
            mobileNavTodos.textContent = count > 0 ? `🚧 Task ${count}` : `🚧 Task`;
        }
    } catch (e) {
        console.error("Error updating todos count:", e);
    }
}

document.addEventListener("DOMContentLoaded", function () {
    updateTodosCount();

    // Highlight active nav link on /todos page
    if (window.location.pathname === "/todos") {
        const navTodos = document.getElementById("nav-todos");
        const mobileNavTodos = document.getElementById("mobile-nav-todos");
        if (navTodos) {
            navTodos.className = "bg-amber-500/10 text-amber-400 px-2.5 py-1 rounded-lg border border-amber-500/20 transition-colors";
        }
        if (mobileNavTodos) {
            mobileNavTodos.className = "block rounded-lg px-3 py-2 bg-amber-500/10 text-amber-400 border border-amber-500/20 transition-colors";
        }
    }
});

