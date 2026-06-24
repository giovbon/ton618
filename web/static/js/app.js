    // ── PDF Upload ──
    var activePdfUploadButton = null;

    document.addEventListener("change", function (e) {
        if (e.target && e.target.id === "pdf-file-input") {
            var file = e.target.files[0];
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
            e.target.value = "";
        }
    });

    // ── ZIP Upload ──
    document
        .getElementById("zip-file-input")
        .addEventListener("change", function (e) {
            var files = e.target.files;
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
                alert("Erro: " + err.message);
            };
            
            xhr.send(fd);
        });

    document
        .getElementById("mobile-menu-toggle")
        .addEventListener("click", function () {
            var menu = document.getElementById("mobile-menu");
            var expanded = this.getAttribute("aria-expanded") === "true";
            if (expanded) {
                menu.classList.add("hidden");
                this.setAttribute("aria-expanded", "false");
            } else {
                menu.classList.remove("hidden");
                this.setAttribute("aria-expanded", "true");
            }
        });

    // ── New Note com nome único ──

    // ── Captura de artigo/YouTube ──
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
    function toggleCriarMenu(e) {
        e.stopPropagation();
        const menu = document.getElementById('criar-menu');
        menu.classList.toggle('hidden');
    }
    function closeCriarMenu() {
        document.getElementById('criar-menu').classList.add('hidden');
    }
    // ── + Captura Dropdown ──
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
        if (wrapper && !wrapper.contains(e.target)) {
            closeCriarMenu();
        }
        const capWrapper = document.getElementById('captura-dropdown-wrapper');
        if (capWrapper && !capWrapper.contains(e.target)) {
            closeCapturaMenu();
        }
    });

    // ── Configurações (⚙️ Settings Modal) ──
    function openSettings() {
        document
            .getElementById("settings-modal")
            .classList.remove("hidden");
        switchSettingsTab("arquivamento");
    }

    function openSettingsTab(tabName) {
        openSettings();
        switchSettingsTab(tabName);
        // Load tab-specific data
        if (tabName === "marcadores") loadMarcadores();
        if (tabName === "stopwords") loadStopwords();
        if (tabName === "restaurar") loadArchives();
    }

    function closeSettings() {
        document.getElementById("settings-modal").classList.add("hidden");
    }
    function switchSettingsTab(name) {
        document.querySelectorAll(".settings-tab").forEach(function (el) {
            el.classList.add("hidden");
        });
        document.getElementById("tab-" + name).classList.remove("hidden");
        document
            .querySelectorAll(".settings-tab-btn")
            .forEach(function (el) {
                el.classList.remove("text-white", "border-sky-500");
                el.classList.add("text-zinc-400", "border-transparent");
            });
        var btn = document.querySelector('[data-tab="' + name + '"]');
        if (btn) {
            btn.classList.add("text-white", "border-sky-500");
            btn.classList.remove("text-zinc-400", "border-transparent");
        }
    }

    // ── Age slider: update display live ──
    document.addEventListener("DOMContentLoaded", function () {
        var slider = document.getElementById("filter-age-slider");
        if (!slider) return;
        function updateAgeDisplay() {
            var v = parseInt(slider.value);
            document.getElementById("age-value-display").textContent =
                v + (v === 1 ? " ano" : " anos");
        }
        slider.addEventListener("input", updateAgeDisplay);
        updateAgeDisplay();
    });

    function getFilterParams() {
        var byAge = document.getElementById("filter-by-age").checked;
        var byTag = document.getElementById("filter-by-tag").checked;
        var ageYears =
            parseInt(document.getElementById("filter-age-slider").value) ||
            0;
        var tagRaw = document
            .getElementById("filter-tag-names")
            .value.trim();
        return {
            byAge: byAge,
            byTag: byTag,
            ageYears: ageYears,
            tagRaw: tagRaw,
        };
    }

    function validateFilters() {
        var p = getFilterParams();
        if (!p.byAge && !p.byTag) {
            alert("Selecione pelo menos um filtro de exclusão.");
            return null;
        }
        if (p.byAge && (p.ageYears < 1 || p.ageYears > 10)) {
            alert("Selecione uma idade válida (1-10 anos).");
            return null;
        }
        if (p.byTag && !p.tagRaw) {
            alert("Digite pelo menos uma tag.");
            return null;
        }
        return p;
    }

    // ── Preview pagination state ──
    var previewFiles = [];
    var previewPageNum = 0;
    var previewPageSize = 100;

    function renderPreviewPage() {
        var start = previewPageNum * previewPageSize;
        var end = Math.min(start + previewPageSize, previewFiles.length);
        var pageItems = previewFiles.slice(start, end);

        var list = document.getElementById("preview-list");
        list.innerHTML = pageItems.map(buildPreviewItem).join("");

        document.getElementById("preview-count").textContent =
            previewFiles.length;

        if (previewFiles.length > 0) {
            document.getElementById("preview-page-info").textContent =
                start + 1 + "-" + end + " de " + previewFiles.length;
        }

        var pag = document.getElementById("preview-pagination");
        if (previewFiles.length > previewPageSize) {
            pag.classList.remove("hidden");
            document.getElementById("preview-prev-btn").disabled =
                previewPageNum === 0;
            document.getElementById("preview-next-btn").disabled =
                end >= previewFiles.length;
        } else {
            pag.classList.add("hidden");
        }
    }

    function previewPage(delta) {
        var newPage = previewPageNum + delta;
        if (newPage < 0) return;
        var start = newPage * previewPageSize;
        if (start >= previewFiles.length) return;
        previewPageNum = newPage;
        renderPreviewPage();
        document
            .getElementById("bulk-preview-area")
            .scrollIntoView({ behavior: "smooth", block: "nearest" });
    }

    var selectedFiles = {};
    var allPreviewFiles = [];

    function selectAllPreview(select) {
        allPreviewFiles.forEach(function (f) {
            selectedFiles[f] = select;
            var cb = document.querySelector(
                'input[data-file="' + f.replace(/"/g, '\\"') + '"]',
            );
            if (cb) cb.checked = select;
        });
        updateArchiveBtn();
    }

    function updateArchiveBtn() {
        var count = Object.keys(selectedFiles).filter(function (k) {
            return selectedFiles[k];
        }).length;
        var btn = document.getElementById("confirm-archive-btn");
        if (count > 0) {
            btn.disabled = false;
            btn.textContent = "📦 Arquivar (" + count + ")";
        } else {
            btn.disabled = true;
            btn.textContent = "📦 Arquivar Selecionadas";
        }
        var delBtn = document.getElementById("confirm-delete-btn");
        if (count > 0) {
            delBtn.disabled = false;
            delBtn.textContent = "🗑️ Excluir (" + count + ")";
        } else {
            delBtn.disabled = true;
            delBtn.textContent = "🗑️ Excluir Selecionadas";
        }
    }

    function toggleFile(filename) {
        selectedFiles[filename] = !selectedFiles[filename];
        updateArchiveBtn();
    }

    function buildPreviewItem(filename) {
        var display = filename.replace(/^notes\//, "").replace(/\.md$/, "");
        var icon = filename.indexOf("pdfs/") === 0 ? "📕" : "📄";
        var checked = selectedFiles[filename] ? "checked" : "";
        return (
            '<div class="flex items-center gap-2 text-xs text-zinc-300 py-1">' +
            '<input type="checkbox" data-file="' +
            filename.replace(/"/g, "&quot;") +
            '" ' +
            checked +
            " onchange=\"toggleFile('" +
            filename.replace(/'/g, "\\'") +
            '\')" class="accent-sky-500 w-3.5 h-3.5 rounded shrink-0 cursor-pointer" />' +
            '<span class="text-zinc-600 shrink-0">' +
            icon +
            "</span>" +
            "<span>" +
            display +
            "</span>" +
            "</div>"
        );
    }

    function getSelectedFiles() {
        return allPreviewFiles.filter(function (f) {
            return selectedFiles[f];
        });
    }

    function previewBulkDelete() {
        var p = validateFilters();
        if (!p) return;

        var btn = document.getElementById("bulk-preview-btn");
        btn.disabled = true;
        btn.textContent = "Buscando...";

        var formData = new URLSearchParams();
        formData.set("by_age", p.byAge ? "true" : "false");
        formData.set("age_years", p.ageYears.toString());
        formData.set("by_tag", p.byTag ? "true" : "false");
        formData.set("tag_name", p.tagRaw);
        formData.set("preview", "true");

        fetch("/api/bulk-delete", {
            method: "POST",
            headers: {
                "Content-Type": "application/x-www-form-urlencoded",
            },
            body: formData.toString(),
        })
            .then(function (r) {
                if (!r.ok)
                    return r.text().then(function (t) {
                        throw new Error(t);
                    });
                return r.json();
            })
            .then(function (data) {
                btn.disabled = false;
                btn.textContent = "🔍 Previsualizar Notas";

                var area = document.getElementById("bulk-preview-area");

                if (data.files && data.files.length > 0) {
                    allPreviewFiles = data.files;
                    previewFiles = data.files;
                    previewPageNum = 0;

                    // Inicializa selecao: todos marcados por padrao
                    allPreviewFiles.forEach(function (f) {
                        selectedFiles[f] = true;
                    });

                    renderPreviewPage();
                    updateArchiveBtn();
                    area.classList.remove("hidden");
                } else {
                    allPreviewFiles = [];
                    previewFiles = [];
                    previewPageNum = 0;
                    selectedFiles = {};
                    area.classList.add("hidden");
                    alert(
                        "Nenhuma nota encontrada com os filtros selecionados.",
                    );
                }
            })
            .catch(function (err) {
                btn.disabled = false;
                btn.textContent = "🔍 Previsualizar Notas";
                alert("Erro: " + err.message);
            });
    }

    function doBulkDelete() {
        var selected = getSelectedFiles();
        if (selected.length === 0) {
            alert("Nenhuma nota selecionada.");
            return;
        }

        if (
            !confirm(
                "Tem certeza que deseja EXCLUIR PERMANENTEMENTE " +
                selected.length +
                " nota(s)? Esta ação não pode ser desfeita.",
            )
        )
            return;

        var btn = document.getElementById("confirm-delete-btn");
        btn.disabled = true;
        btn.textContent = "Excluindo...";

        // Envia a lista de arquivos selecionados
        var formData = new URLSearchParams();
        selected.forEach(function (f) {
            formData.append("files", f);
        });

        fetch("/api/bulk-delete", {
            method: "POST",
            headers: {
                "Content-Type": "application/x-www-form-urlencoded",
            },
            body: formData.toString(),
        })
            .then(function (r) {
                if (!r.ok)
                    return r.text().then(function (t) {
                        throw new Error(t);
                    });
                return r.json();
            })
            .then(function (data) {
                btn.disabled = false;
                btn.textContent = "🗑️ Excluir Selecionadas";
                alert(
                    "Excluídas " + data.deleted + " nota(s) com sucesso!",
                );
                closeSettings();
                if (window.location.pathname === "/") {
                    window.location.reload();
                }
            })
            .catch(function (err) {
                btn.disabled = false;
                btn.textContent = "🗑️ Excluir Selecionadas";
                alert("Erro: " + err.message);
            });
    }

    // ── Bulk Archive ──
    function doBulkArchive() {
        var selected = getSelectedFiles();
        if (selected.length === 0) {
            alert("Nenhuma nota selecionada.");
            return;
        }

        if (
            !confirm(
                "Arquivar " +
                selected.length +
                " nota(s)? Elas serão zipadas e movidas para a pasta de arquivos mortos.",
            )
        )
            return;

        var btn = document.getElementById("confirm-archive-btn");
        btn.disabled = true;
        btn.textContent = "Arquivando...";

        var formData = new URLSearchParams();
        selected.forEach(function (f) {
            formData.append("files", f);
        });

        fetch("/api/bulk-archive", {
            method: "POST",
            headers: {
                "Content-Type": "application/x-www-form-urlencoded",
            },
            body: formData.toString(),
        })
            .then(function (r) {
                if (!r.ok)
                    return r.text().then(function (t) {
                        throw new Error(t);
                    });
                return r.json();
            })
            .then(function (data) {
                btn.disabled = false;
                btn.textContent = "📦 Arquivar Selecionadas";
                if (data.ok) {
                    alert(
                        "Arquivadas " +
                        data.archived +
                        " nota(s) com sucesso!\nArquivo: " +
                        data.archive,
                    );
                    closeSettings();
                    if (window.location.pathname === "/") {
                        window.location.reload();
                    }
                }
            })
            .catch(function (err) {
                btn.disabled = false;
                btn.textContent = "📦 Arquivar Selecionadas";
                alert("Erro ao arquivar: " + err.message);
            });
    }

    // ── List Archives ──
    function loadArchives() {
        var list = document.getElementById("archives-list");
        list.innerHTML =
            '<div class="text-center py-10"><div class="animate-spin h-5 w-5 border-2 border-zinc-600 border-t-transparent rounded-full mx-auto mb-3"></div><p class="text-xs text-zinc-600">Carregando...</p></div>';

        fetch("/api/archives")
            .then(function (r) {
                if (!r.ok) throw new Error("Erro ao carregar archives");
                return r.json();
            })
            .then(function (data) {
                var archives = data.archives || [];
                if (archives.length === 0) {
                    list.innerHTML =
                        '<div class="text-center py-10"><p class="text-xs text-zinc-500">Nenhum archive encontrado.</p></div>';
                    return;
                }
                var html = '<div class="space-y-2">';
                archives.forEach(function (a) {
                    var sizeStr =
                        a.size > 1024 * 1024
                            ? (a.size / 1024 / 1024).toFixed(1) + " MB"
                            : a.size > 1024
                                ? (a.size / 1024).toFixed(1) + " KB"
                                : a.size + " B";
                    var dateStr = a.modified
                        ? new Date(a.modified).toLocaleDateString("pt-BR")
                        : "?";
                    html +=
                        '<div class="flex items-center justify-between gap-3 bg-zinc-800/40 rounded-xl p-3 border border-zinc-700/50">' +
                        '<div class="flex-1 min-w-0">' +
                        '<div class="text-sm font-semibold text-zinc-200 truncate">💾 ' +
                        a.name +
                        "</div>" +
                        '<div class="text-[10px] text-zinc-500 mt-0.5">' +
                        a.file_count +
                        " arquivo(s) · " +
                        sizeStr +
                        " · " +
                        dateStr +
                        "</div>" +
                        "</div>" +
                        "<button onclick=\"restoreArchive('" +
                        a.name.replace(/'/g, "\\'") +
                        '\')" class="shrink-0 px-3 py-1.5 rounded-lg bg-emerald-600 hover:bg-emerald-700 disabled:opacity-50 disabled:cursor-not-allowed text-white text-xs font-bold transition-colors restore-btn">Restaurar</button>' +
                        "</div>";
                });
                html += "</div>";
                list.innerHTML = html;
            })
            .catch(function (err) {
                list.innerHTML =
                    '<div class="text-center py-10"><p class="text-xs text-red-400">Erro: ' +
                    err.message +
                    "</p></div>";
            });
    }

    function restoreArchive(name) {
        if (
            !confirm(
                'Restaurar o archive "' +
                name +
                '"? As notas serão extraídas de volta para o diretório de trabalho.',
            )
        )
            return;

        var btns = document.querySelectorAll(".restore-btn");
        btns.forEach(function (b) {
            b.disabled = true;
            b.textContent = "Restaurando...";
        });

        var formData = new URLSearchParams();
        formData.set("archive", name);

        fetch("/api/archive/restore", {
            method: "POST",
            headers: {
                "Content-Type": "application/x-www-form-urlencoded",
            },
            body: formData.toString(),
        })
            .then(function (r) {
                if (!r.ok)
                    return r.text().then(function (t) {
                        throw new Error(t);
                    });
                return r.json();
            })
            .then(function (data) {
                if (data.ok) {
                    alert(
                        "Archive restaurado com sucesso!\n" +
                        data.restored +
                        " arquivo(s) restaurados.",
                    );
                    closeSettings();
                    if (window.location.pathname === "/") {
                        window.location.reload();
                    }
                }
            })
            .catch(function (err) {
                alert("Erro ao restaurar: " + err.message);
                btns.forEach(function (b) {
                    b.disabled = false;
                    b.textContent = "Restaurar";
                });
            });
    }

    // ── Cleanup Orphan Images ──
    function cleanupOrphanImages() {
        var btn = document.getElementById("cleanup-images-btn");
        if (
            !confirm(
                "Remover imagens que não são mais referenciadas por nenhuma nota?",
            )
        )
            return;
        btn.disabled = true;
        btn.textContent = "Limpando...";
        fetch("/api/cleanup-images", { method: "POST" })
            .then(function (r) {
                return r.json();
            })
            .then(function (data) {
                btn.disabled = false;
                btn.textContent = "🧹 Limpar imagens órfãs";
                if (data.ok) {
                    var msg =
                        data.count > 0
                            ? data.count +
                            " imagem(ns) órfã(s) removida(s)!"
                            : "Nenhuma imagem órfã encontrada.";
                    alert(msg);
                } else {
                    alert("Erro: " + (data.error || "desconhecido"));
                }
            })
            .catch(function (err) {
                btn.disabled = false;
                btn.textContent = "🧹 Limpar imagens órfãs";
                alert("Erro: " + err.message);
            });
    }

    // ── Stopwords Customizadas ──
    function loadStopwords() {
        var container = document.getElementById("stopwords-list");
        var countEl = document.getElementById("stopwords-count");
        if (!container) return;
        container.innerHTML =
            '<span class="text-xs text-zinc-500">Carregando...</span>';

        fetch("/api/stopwords")
            .then(function (r) {
                return r.json();
            })
            .then(function (data) {
                var words = data.stopwords || [];
                if (words.length === 0) {
                    container.innerHTML =
                        '<span class="text-xs text-zinc-600">Nenhuma stopword personalizada ainda.</span>';
                    countEl.textContent = "0";
                    return;
                }
                var html = "";
                words.forEach(function (w) {
                    html +=
                        '<span class="inline-flex items-center gap-1 px-2.5 py-1 rounded-full bg-zinc-800 border border-zinc-700/50 text-xs text-zinc-300">' +
                        "<span>" +
                        w +
                        "</span>" +
                        "<button onclick=\"removeStopword('" +
                        w.replace(/'/g, "\\'") +
                        '\')" class="text-zinc-600 hover:text-red-400 transition-colors leading-none text-sm ml-0.5" title="Remover">&times;</button>' +
                        "</span>";
                });
                container.innerHTML = html;
                countEl.textContent = words.length;
            })
            .catch(function (err) {
                container.innerHTML =
                    '<span class="text-xs text-red-400">Erro ao carregar: ' +
                    err.message +
                    "</span>";
            });
    }

    function addStopword() {
        var input = document.getElementById("new-stopword-input");
        var word = input.value.trim().toLowerCase();
        if (!word) {
            alert("Digite uma palavra para adicionar.");
            return;
        }

        fetch("/api/stopwords/add", {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ word: word }),
        })
            .then(function (r) {
                return r.json();
            })
            .then(function (data) {
                if (data.success) {
                    input.value = "";
                    loadStopwords();
                } else {
                    alert("Erro ao adicionar stopword.");
                }
            })
            .catch(function (err) {
                alert("Erro: " + err.message);
            });
    }

    function removeStopword(word) {
        if (
            !confirm('Remover "' + word + '" das stopwords personalizadas?')
        )
            return;

        fetch("/api/stopwords/remove", {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ word: word }),
        })
            .then(function (r) {
                return r.json();
            })
            .then(function (data) {
                if (data.success) {
                    loadStopwords();
                } else {
                    alert("Erro ao remover stopword.");
                }
            })
            .catch(function (err) {
                alert("Erro: " + err.message);
            });
    }

    (function () {
        var auth = localStorage.getItem("ton_auth");

        // Redirect to login if not authenticated (skip for login page and static)
        var path = window.location.pathname;
        if (!auth && path !== "/login" && !path.startsWith("/static/")) {
            window.location.href = "/login";
            return;
        }

        // Inject auth header into HTMX requests
        if (typeof htmx !== "undefined") {
            document.body.addEventListener(
                "htmx:configRequest",
                function (evt) {
                    if (auth) evt.detail.headers["Authorization"] = auth;
                },
            );
        }

        // Wrap fetch to always include auth
        var origFetch = window.fetch;
        window.fetch = function (url, opts) {
            opts = opts || {};
            opts.headers = opts.headers || {};
            if (auth && !opts.headers["Authorization"]) {
                if (opts.headers instanceof Headers) {
                    opts.headers.set("Authorization", auth);
                } else {
                    opts.headers["Authorization"] = auth;
                }
            }
            return origFetch(url, opts);
        };
    })();

    // ── Marcadores (settings modal tab) ──
    var modalMarkers = [];

    function loadMarcadores() {
        var list = document.getElementById("modal-markers-list");
        if (!list) return;
        list.innerHTML =
            '<div class="text-center py-8"><div class="animate-spin h-5 w-5 border-2 border-zinc-600 border-t-transparent rounded-full mx-auto mb-3"></div><p class="text-xs text-zinc-600">Carregando...</p></div>';

        fetch("/api/todo-markers")
            .then(function (r) {
                if (!r.ok) throw new Error("Falha ao carregar");
                return r.json();
            })
            .then(function (data) {
                modalMarkers = data || [];
                renderModalMarkers();
            })
            .catch(function () {
                modalMarkers = [];
                renderModalMarkers();
            });
    }

    function renderModalMarkers() {
        var list = document.getElementById("modal-markers-list");
        if (!list) return;
        list.innerHTML = "";

        if (modalMarkers.length === 0) {
            list.innerHTML =
                '<div class="text-center py-8"><p class="text-xs text-zinc-500">Nenhum marcador configurado.</p></div>';
            return;
        }

        modalMarkers.forEach(function (m, idx) {
            var div = document.createElement("div");
            div.className =
                "marker-row flex items-center gap-3 px-4 py-3 bg-zinc-800/40 border border-zinc-700/50 rounded-lg";
            div.setAttribute("data-marker", m.marker);

            var activeAttr = m.active ? "checked" : "";

            div.innerHTML =
                '<label class="flex items-center gap-2 cursor-pointer">' +
                '<input type="checkbox" class="marker-active w-4 h-4 rounded border-zinc-600 bg-zinc-800 text-amber-500 focus:ring-amber-500" ' +
                activeAttr +
                " />" +
                "</label>" +
                '<span class="marker-badge px-3 py-1 text-[11px] font-bold rounded" style="background:' +
                m.color +
                "20; color:" +
                m.color +
                '">' +
                escapeHtml(m.marker) +
                "</span>" +
                '<input type="color" class="marker-color w-8 h-8 rounded cursor-pointer border-0 bg-transparent" value="' +
                m.color +
                '" />' +
                '<span class="flex-1 text-xs text-zinc-500">' +
                escapeHtml(m.marker) +
                ":</span>" +
                '<button class="marker-delete text-zinc-600 hover:text-red-400 transition-all text-sm px-2">✕</button>';

            list.appendChild(div);

            // Delete handler
            div.querySelector(".marker-delete").addEventListener("click", function () {
                modalMarkers.splice(idx, 1);
                renderModalMarkers();
            });

            // Change handlers
            div.querySelector(".marker-active").addEventListener("change", function () {
                m.active = this.checked;
            });
            div.querySelector(".marker-color").addEventListener("input", function () {
                m.color = this.value;
                div.querySelector(".marker-badge").style.background = m.color + "20";
                div.querySelector(".marker-badge").style.color = m.color;
            });
        });
    }

    function escapeHtml(str) {
        var div = document.createElement("div");
        div.textContent = str;
        return div.innerHTML;
    }

    async function updateTodosCount() {
        try {
            const response = await fetch("/api/todos?type=all&status=pending");
            if (!response.ok) return;
            const data = await response.json();
            const count = data.todos ? data.todos.length : 0;

            const navTodos = document.getElementById("nav-todos");
            const mobileNavTodos = document.getElementById("mobile-nav-todos");

            if (navTodos) {
                navTodos.textContent = count > 0 ? `🎯TODOs ${count}` : `🎯TODOs`;
            }
            if (mobileNavTodos) {
                mobileNavTodos.textContent = count > 0 ? `🎯 TODOs ${count}` : `🎯 TODOs`;
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

        // Add marker
        var addBtn = document.getElementById("modal-btn-add-marker");
        if (addBtn) {
            addBtn.addEventListener("click", function () {
                var input = document.getElementById("modal-new-marker");
                var colorSelect = document.getElementById("modal-new-color");
                var name = input.value.trim().toUpperCase();

                if (!name) return;
                if (modalMarkers.some(function (m) { return m.marker === name; })) {
                    document.getElementById("modal-save-status").textContent = "⚠️ Marcador já existe";
                    return;
                }

                modalMarkers.push({
                    marker: name,
                    color: colorSelect.value,
                    active: true,
                });
                input.value = "";
                renderModalMarkers();
                document.getElementById("modal-save-status").textContent = "";
            });
        }

        // Save
        var saveBtn = document.getElementById("modal-btn-save");
        if (saveBtn) {
            saveBtn.addEventListener("click", async function () {
                var btn = this;
                btn.textContent = "💾 Salvando...";
                btn.disabled = true;

                try {
                    var response = await fetch("/api/todo-markers", {
                        method: "POST",
                        headers: { "Content-Type": "application/json" },
                        body: JSON.stringify(modalMarkers),
                    });

                    if (!response.ok) throw new Error("Save failed");

                    document.getElementById("modal-save-status").textContent =
                        "✅ Configurações salvas!";
                    setTimeout(function () {
                        document.getElementById("modal-save-status").textContent = "";
                    }, 3000);
                } catch (e) {
                    document.getElementById("modal-save-status").textContent =
                        "❌ Erro ao salvar";
                }

                btn.textContent = "💾 Salvar configurações";
                btn.disabled = false;
            });
        }

        // Reset to defaults
        var resetBtn = document.getElementById("modal-btn-reset");
        if (resetBtn) {
            resetBtn.addEventListener("click", async function () {
                var defaults = [
                    { marker: "TODO", color: "#3b82f6", active: true },
                    { marker: "FIXME", color: "#f59e0b", active: true },
                    { marker: "BUG", color: "#ef4444", active: true },
                    { marker: "HACK", color: "#8b5cf6", active: false },
                    { marker: "NOTE", color: "#06b6d4", active: false },
                    { marker: "OPTIMIZE", color: "#10b981", active: false },
                    { marker: "REVIEW", color: "#f97316", active: false },
                ];
                modalMarkers = defaults;
                renderModalMarkers();
                document.getElementById("modal-save-status").textContent =
                    "⚠️ Padrão restaurado. Clique em Salvar para confirmar.";
            });
        }
    });
