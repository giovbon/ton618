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
            const iconEl = navTodos.querySelector("svg, i");
            const iconHtml = iconEl ? iconEl.outerHTML : '';
            navTodos.innerHTML = `${iconHtml} Task${count > 0 ? ' ' + count : ''}`;
        }
        if (mobileNavTodos) {
            const iconEl = mobileNavTodos.querySelector("svg, i");
            const iconHtml = iconEl ? iconEl.outerHTML : '';
            const span = mobileNavTodos.querySelector("span");
            if (span) {
                span.textContent = count > 0 ? `TASK ${count}` : `TASK`;
            } else {
                mobileNavTodos.innerHTML = `${iconHtml} <span class="text-[11px] font-bold tracking-wider">TASK${count > 0 ? ' ' + count : ''}</span>`;
            }
        }
        renderLucideIcons();
    } catch (e) {
        console.error("Error updating todos count:", e);
    }
}

document.addEventListener("DOMContentLoaded", function () {
    updateTodosCount();

    if (window.location.pathname === "/todos") {
        const navTodos = document.getElementById("nav-todos");
        const mobileNavTodos = document.getElementById("mobile-nav-todos");
        if (navTodos) {
            navTodos.className = "px-3 py-1.5 rounded-lg bg-amber-950/40 border border-amber-500/30 text-amber-400 flex items-center gap-1.5 transition-all";
        }
        if (mobileNavTodos) {
            mobileNavTodos.className = "flex items-center gap-2.5 px-3 py-1.5 rounded-lg text-amber-400 bg-amber-950/40 border border-amber-500/20 transition-colors";
        }
    }
});
