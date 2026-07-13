(function() {
    /** @type {string[]} */
    let availableNotes = [];
    /** @type {string[]} */
    let filteredNotes = [];
    /** @type {boolean} */
    let autocompleteVisible = false;
    /** @type {number} */
    let selectedIndex = 0;
    /** @type {HTMLInputElement | null} */
    let activeInput = null;
    
    // Grab the autocomplete container from the DOM
    /** @type {HTMLElement | null} */
    const autocompleteContainer = document.getElementById('agenda-autocomplete');

    /**
     * @param {string|number|boolean} str
     * @returns {string}
     */
    function escapeHtml(str) {
        return String(str)
            .replace(/&/g, '&amp;')
            .replace(/</g, '&lt;')
            .replace(/>/g, '&gt;')
            .replace(/"/g, '&quot;')
            .replace(/'/g, '&#39;');
    }

    /**
     * @returns {Promise<void>}
     */
    async function fetchNotesForAutocomplete() {
        if (availableNotes.length > 0) return;
        try {
            const res = await fetch('/api/notes');
            if (res.ok) {
                const data = await res.json();
                availableNotes = (data.notes || []).map((/** @type {{arquivo: string}} */ n) => {
                    const filename = n.arquivo.split('/').pop() || n.arquivo;
                    return filename.replace(/\.md$/i, '');
                });
            }
        } catch (e) {
            console.error("Erro ao carregar notas para autocomplete", e);
        }
    }

    /**
     * @param {HTMLElement} targetInput 
     */
    function positionAutocomplete(targetInput) {
        if (!autocompleteContainer) return;
        const rect = targetInput.getBoundingClientRect();
        autocompleteContainer.style.top = `${rect.bottom + 4}px`;
        autocompleteContainer.style.left = `${rect.left}px`;
        autocompleteContainer.style.width = `${rect.width}px`;
    }

    /**
     * @param {string} query 
     */
    function showAutocomplete(query) {
        if (!autocompleteContainer) return;

        const q = query.toLowerCase().trim();
        filteredNotes = availableNotes.filter(name => name.toLowerCase().includes(q));
        
        if (filteredNotes.length === 0) {
            hideAutocomplete();
            return;
        }
        
        selectedIndex = Math.min(selectedIndex, filteredNotes.length - 1);
        if (selectedIndex < 0) selectedIndex = 0;
        
        autocompleteContainer.innerHTML = '';
        filteredNotes.forEach((name, idx) => {
            const btn = document.createElement('button');
            btn.className = `w-full text-left px-3 py-2 text-[13px] text-zinc-300 hover:bg-zinc-800 rounded flex items-center gap-2 transition-colors ${idx === selectedIndex ? 'bg-zinc-800 text-white font-medium' : ''}`;
            btn.innerHTML = `<span class="text-sky-400 text-[11px] font-bold shrink-0">[[]]</span><span class="truncate">${escapeHtml(name)}</span>`;
            
            btn.addEventListener('mousedown', (e) => {
                e.preventDefault();
                e.stopPropagation();
            });
            btn.addEventListener('click', (e) => {
                e.preventDefault();
                e.stopPropagation();
                selectNote(name);
            });
            autocompleteContainer.appendChild(btn);
        });
        
        autocompleteContainer.classList.remove('hidden');
        autocompleteVisible = true;
    }
    
    function hideAutocomplete() {
        if (autocompleteContainer) {
            autocompleteContainer.classList.add('hidden');
            autocompleteContainer.innerHTML = '';
        }
        autocompleteVisible = false;
    }
    
    /**
     * @param {string} name 
     */
    function selectNote(name) {
        if (!activeInput) return;
        const text = activeInput.value;
        const cursor = activeInput.selectionStart || 0;
        const textBefore = text.substring(0, cursor);
        const bracketPos = textBefore.lastIndexOf('[[');
        if (bracketPos !== -1) {
            const textAfter = text.substring(cursor);
            const newValue = textBefore.substring(0, bracketPos) + `[[${name}]] ` + textAfter;
            activeInput.value = newValue;
            const newCursorPos = bracketPos + name.length + 5;
            activeInput.setSelectionRange(newCursorPos, newCursorPos);
        }
        hideAutocomplete();
        activeInput.focus();
        
        // Trigger input event to re-evaluate date preview after selecting a note
        activeInput.dispatchEvent(new Event('input'));
    }

    function updateAutocompleteSelection() {
        if (!autocompleteContainer) return;
        const buttons = autocompleteContainer.querySelectorAll('button');
        buttons.forEach((btn, idx) => {
            if (idx === selectedIndex) {
                btn.classList.add('bg-zinc-800', 'text-white', 'font-medium');
                btn.scrollIntoView({ block: 'nearest' });
            } else {
                btn.classList.remove('bg-zinc-800', 'text-white', 'font-medium');
            }
        });
    }

    // Expose configuration function globally
    // @ts-ignore
    window.setupAutocomplete = function(/** @type {HTMLInputElement} */ targetInput) {
        targetInput.addEventListener('input', async (e) => {
            activeInput = targetInput;
            const text = targetInput.value;
            if (!text.trim()) {
                hideAutocomplete();
                return;
            }

            // Handle Wiki Link autocomplete detection
            const cursor = targetInput.selectionStart || 0;
            const textBefore = text.substring(0, cursor);
            const bracketPos = textBefore.lastIndexOf('[[');
            
            if (bracketPos !== -1) {
                const textAfterBracket = textBefore.substring(bracketPos + 2);
                if (!textAfterBracket.includes(']]')) {
                    await fetchNotesForAutocomplete();
                    positionAutocomplete(targetInput);
                    showAutocomplete(textAfterBracket);
                } else {
                    hideAutocomplete();
                }
            } else {
                hideAutocomplete();
            }
        });

        targetInput.addEventListener('keydown', (e) => {
            if (!autocompleteVisible || activeInput !== targetInput) return;
            
            if (e.key === 'ArrowDown') {
                e.preventDefault();
                selectedIndex = (selectedIndex + 1) % filteredNotes.length;
                updateAutocompleteSelection();
            } else if (e.key === 'ArrowUp') {
                e.preventDefault();
                selectedIndex = (selectedIndex - 1 + filteredNotes.length) % filteredNotes.length;
                updateAutocompleteSelection();
            } else if (e.key === 'Enter' || e.key === 'Tab') {
                e.preventDefault();
                if (filteredNotes[selectedIndex]) {
                    selectNote(filteredNotes[selectedIndex]);
                }
            } else if (e.key === 'Escape') {
                e.preventDefault();
                hideAutocomplete();
            }
        });
    };

    // Close autocomplete when clicking outside
    document.addEventListener('click', (e) => {
        if (autocompleteVisible && autocompleteContainer && !autocompleteContainer.contains(/** @type {Node} */ (e.target)) && e.target !== activeInput) {
            hideAutocomplete();
        }
    });

    // Expose checking state globally for editor modal integration
    // @ts-ignore
    window.isAutocompleteVisible = function() {
        return autocompleteVisible;
    };
})();
