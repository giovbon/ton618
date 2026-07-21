const globalWindow = window;
export function CodeJar(editor, highlight, opt = {}) {
    const options = {
        tab: '\t',
        indentOn: /[({\[]$/,
        moveToNewLine: /^[)}\]]/,
        spellcheck: false,
        catchTab: true,
        preserveIdent: true,
        addClosing: true,
        history: true,
        window: globalWindow,
        autoclose: {
            open: `([{'"`,
            close: `)]}'"`
        },
        ...opt,
    };
    const window = options.window;
    const document = window.document;
    const listeners = [];
    const history = [];
    let at = -1;
    let focus = false;
    let onUpdate = () => void 0;
    let prev; // code content prior keydown event
    editor.setAttribute('contenteditable', 'plaintext-only');
    editor.setAttribute('spellcheck', options.spellcheck ? 'true' : 'false');
    editor.style.outline = 'none';
    editor.style.overflowWrap = 'break-word';
    editor.style.overflowY = 'auto';
    editor.style.whiteSpace = 'pre-wrap';
    const doHighlight = (editor, pos) => {
        highlight(editor, pos);
    };
    const matchFirefoxVersion = window.navigator.userAgent.match(/Firefox\/([0-9]+)\./);
    const firefoxVersion = matchFirefoxVersion
        ? parseInt(matchFirefoxVersion[1])
        : 0;
    let isLegacy = false; // true if plaintext-only is not supported
    if (editor.contentEditable !== "plaintext-only" || firefoxVersion >= 136)
        isLegacy = true;
    if (isLegacy)
        editor.setAttribute("contenteditable", "true");
    const debounceHighlight = debounce(() => {
        const pos = safeSave();
        if (!pos) return;
        doHighlight(editor, pos);
        safeRestore(pos);
    }, 30);
    let recording = false;
    const shouldRecord = (event) => {
        return !isUndo(event) && !isRedo(event)
            && event.key !== 'Meta'
            && event.key !== 'Control'
            && event.key !== 'Alt'
            && !event.key.startsWith('Arrow');
    };
    const debounceRecordHistory = debounce((event) => {
        if (shouldRecord(event)) {
            recordHistory();
            recording = false;
        }
    }, 300);
    const on = (type, fn) => {
        listeners.push([type, fn]);
        editor.addEventListener(type, fn);
    };
    on('keydown', event => {
        if (event.defaultPrevented)
            return;
        prev = toString();
        if (options.preserveIdent)
            handleNewLine(event);
        else
            legacyNewLineFix(event);
        if (options.catchTab)
            handleTabCharacters(event);
        if (options.addClosing)
            handleSelfClosingCharacters(event);
        if (options.history) {
            handleUndoRedo(event);
            if (shouldRecord(event) && !recording) {
                recordHistory();
                recording = true;
            }
        }
        if (isLegacy && !isCopy(event))
            safeRestore(safeSave());
    });
    on('keyup', event => {
        if (event.defaultPrevented)
            return;
        if (event.isComposing)
            return;
        if (prev !== toString())
            debounceHighlight();
        debounceRecordHistory(event);
        onUpdate(toString());
    });
    on('focus', _event => {
        focus = true;
    });
    on('blur', _event => {
        focus = false;
    });
    on('paste', event => {
        recordHistory();
        handlePaste(event);
        recordHistory();
        onUpdate(toString());
    });
    on('cut', event => {
        recordHistory();
        handleCut(event);
        recordHistory();
        onUpdate(toString());
    });
    // ─── Safe wrappers ──────────────────────────────────────
    function safeSave() {
        try {
            return save();
        } catch (_e) {
            return { start: 0, end: 0, dir: '->' };
        }
    }
    function safeRestore(pos) {
        try {
            restore(pos);
        } catch (_e) {
            // ignora — melhor que crashar o editor
        }
    }
    function save() {
        const s = getSelection();
        const pos = { start: 0, end: 0, dir: undefined };
        let { anchorNode, anchorOffset, focusNode, focusOffset } = s;
        if (!anchorNode || !focusNode)
            return { start: 0, end: 0, dir: '->' };
        if (anchorNode === editor && focusNode === editor) {
            pos.start = (anchorOffset > 0 && editor.textContent) ? editor.textContent.length : 0;
            pos.end = (focusOffset > 0 && editor.textContent) ? editor.textContent.length : 0;
            pos.dir = (focusOffset >= anchorOffset) ? '->' : '<-';
            return pos;
        }
        if (anchorNode.nodeType === Node.ELEMENT_NODE) {
            const node = document.createTextNode('');
            anchorNode.insertBefore(node, anchorNode.childNodes[anchorOffset]);
            anchorNode = node;
            anchorOffset = 0;
        }
        if (focusNode.nodeType === Node.ELEMENT_NODE) {
            const node = document.createTextNode('');
            focusNode.insertBefore(node, focusNode.childNodes[focusOffset]);
            focusNode = node;
            focusOffset = 0;
        }
        visit(editor, el => {
            if (el === anchorNode && el === focusNode) {
                pos.start += anchorOffset;
                pos.end += focusOffset;
                pos.dir = anchorOffset <= focusOffset ? '->' : '<-';
                return 'stop';
            }
            if (el === anchorNode) {
                pos.start += anchorOffset;
                if (!pos.dir) {
                    pos.dir = '->';
                } else {
                    return 'stop';
                }
            } else if (el === focusNode) {
                pos.end += focusOffset;
                if (!pos.dir) {
                    pos.dir = '<-';
                } else {
                    return 'stop';
                }
            }
            if (el.nodeType === Node.TEXT_NODE) {
                if (pos.dir != '->')
                    pos.start += el.nodeValue.length;
                if (pos.dir != '<-')
                    pos.end += el.nodeValue.length;
            }
        });
        editor.normalize();
        return pos;
    }
    function restore(pos) {
        const s = getSelection();
        let startNode, startOffset = 0;
        let endNode, endOffset = 0;
        if (!pos.dir) pos.dir = '->';
        if (pos.start < 0) pos.start = 0;
        if (pos.end < 0) pos.end = 0;
        if (pos.dir == '<-') {
            const { start, end } = pos;
            pos.start = end;
            pos.end = start;
        }
        let current = 0;
        visit(editor, el => {
            if (el.nodeType !== Node.TEXT_NODE) return;
            const len = (el.nodeValue || '').length;
            if (current + len > pos.start) {
                if (!startNode) {
                    startNode = el;
                    startOffset = pos.start - current;
                }
                if (current + len > pos.end) {
                    endNode = el;
                    endOffset = pos.end - current;
                    return 'stop';
                }
            }
            current += len;
        });
        if (!startNode) startNode = editor, startOffset = editor.childNodes.length;
        if (!endNode) endNode = editor, endOffset = editor.childNodes.length;
        if (pos.dir == '<-') {
            [startNode, startOffset, endNode, endOffset] = [endNode, endOffset, startNode, startOffset];
        }
        {
            const startEl = uneditable(startNode);
            if (startEl) {
                const node = document.createTextNode('');
                startEl.parentNode?.insertBefore(node, startEl);
                startNode = node;
                startOffset = 0;
            }
            const endEl = uneditable(endNode);
            if (endEl) {
                const node = document.createTextNode('');
                endEl.parentNode?.insertBefore(node, endEl);
                endNode = node;
                endOffset = 0;
            }
        }
        try {
            s.setBaseAndExtent(startNode, startOffset, endNode, endOffset);
        } catch (_e) {
            // nó pode ter sido removido pelo highlight — ignora
        }
        editor.normalize();
    }
    function uneditable(node) {
        while (node && node !== editor) {
            if (node.nodeType === Node.ELEMENT_NODE) {
                const el = node;
                if (el.getAttribute('contenteditable') == 'false') return el;
            }
            node = node.parentNode;
        }
    }
    // ─── Safe cursor helpers ────────────────────────────────
    function safeBeforeCursor() {
        try {
            const s = getSelection();
            if (!s || s.rangeCount === 0) return '';
            const r0 = s.getRangeAt(0);
            const r = document.createRange();
            r.selectNodeContents(editor);
            r.setEnd(r0.startContainer, r0.startOffset);
            return r.toString();
        } catch (_e) { return ''; }
    }
    function safeAfterCursor() {
        try {
            const s = getSelection();
            if (!s || s.rangeCount === 0) return '';
            const r0 = s.getRangeAt(0);
            const r = document.createRange();
            r.selectNodeContents(editor);
            r.setStart(r0.endContainer, r0.endOffset);
            return r.toString();
        } catch (_e) { return ''; }
    }
    function handleNewLine(event) {
        if (event.key === 'Enter') {
            const before = safeBeforeCursor();
            const after = safeAfterCursor();
            let [padding] = findPadding(before);
            let newLinePadding = padding;
            if (options.indentOn.test(before)) {
                newLinePadding += options.tab;
            }
            if (newLinePadding.length > 0) {
                preventDefault(event);
                event.stopPropagation();
                insert('\n' + newLinePadding);
            } else {
                legacyNewLineFix(event);
            }
            if (newLinePadding !== padding && options.moveToNewLine.test(after)) {
                const pos = safeSave();
                insert('\n' + padding);
                safeRestore(pos);
            }
        }
    }
    function legacyNewLineFix(event) {
        if (isLegacy && event.key === 'Enter') {
            preventDefault(event);
            event.stopPropagation();
            if (safeAfterCursor() == '') {
                insert('\n ');
                const pos = safeSave();
                pos.start = --pos.end;
                safeRestore(pos);
            } else {
                insert('\n');
            }
        }
    }
    function handleSelfClosingCharacters(event) {
        const open = options.autoclose.open;
        const close = options.autoclose.close;
        if (open.includes(event.key)) {
            preventDefault(event);
            const pos = safeSave();
            const wrapText = pos.start == pos.end ? '' : getSelection().toString();
            const text = event.key + wrapText + (close[open.indexOf(event.key)] ?? "");
            insert(text);
            pos.start++;
            pos.end++;
            safeRestore(pos);
        }
    }
    function handleTabCharacters(event) {
        if (event.key === 'Tab') {
            preventDefault(event);
            const text = toString();
            const pos = safeSave();
            let selStart = Math.min(pos.start, pos.end);
            let selEnd = Math.max(pos.start, pos.end);
            const isMultiLine = text.substring(selStart, selEnd).includes('\n');
            if (isMultiLine || event.shiftKey) {
                if (selEnd > selStart && text[selEnd - 1] === '\n') selEnd--;
                const startLineOffset = text.lastIndexOf('\n', selStart - 1) + 1;
                let endLineOffset = text.indexOf('\n', selEnd);
                if (endLineOffset === -1) endLineOffset = text.length;
                const targetText = text.substring(startLineOffset, endLineOffset);
                const lines = targetText.split('\n');
                let newLines = [];
                let cumulativeShift = 0;
                let newSelStart = selStart;
                let newSelEnd = selEnd;
                let currentOriginalOffset = startLineOffset;
                for (let i = 0; i < lines.length; i++) {
                    const line = lines[i];
                    const lineLength = line.length;
                    let newLine = line;
                    let change = 0;
                    if (event.shiftKey) {
                        if (line.startsWith(options.tab)) {
                            newLine = line.substring(options.tab.length);
                            change = -options.tab.length;
                        } else {
                            const spaceMatch = line.match(/^[ ]+/);
                            if (spaceMatch) {
                                const spacesCount = Math.min(spaceMatch[0].length, options.tab.length);
                                newLine = line.substring(spacesCount);
                                change = -spacesCount;
                            }
                        }
                    } else {
                        newLine = options.tab + line;
                        change = options.tab.length;
                    }
                    newLines.push(newLine);
                    if (selStart >= currentOriginalOffset && selStart <= currentOriginalOffset + lineLength) {
                        const relativePos = selStart - currentOriginalOffset;
                        const newLineStart = currentOriginalOffset + cumulativeShift;
                        if (relativePos === 0) newSelStart = newLineStart;
                        else newSelStart = newLineStart + Math.max(0, relativePos + change);
                    }
                    if (selEnd >= currentOriginalOffset && selEnd <= currentOriginalOffset + lineLength) {
                        const relativePos = selEnd - currentOriginalOffset;
                        const newLineStart = currentOriginalOffset + cumulativeShift;
                        if (relativePos === 0) newSelEnd = newLineStart;
                        else newSelEnd = newLineStart + Math.max(0, relativePos + change);
                    }
                    cumulativeShift += change;
                    currentOriginalOffset += lineLength + 1;
                }
                const newLinesText = newLines.join('\n');
                recordHistory();
                safeRestore({ start: startLineOffset, end: endLineOffset });
                insert(newLinesText);
                const newPos = {
                    start: pos.dir === '<-' ? newSelEnd : newSelStart,
                    end: pos.dir === '<-' ? newSelStart : newSelEnd,
                    dir: pos.dir
                };
                safeRestore(newPos);
                recordHistory();
                if (prev !== toString()) {
                    const pos2 = safeSave();
                    doHighlight(editor);
                    safeRestore(pos2);
                    onUpdate(toString());
                }
            } else {
                recordHistory();
                insert(options.tab);
                recordHistory();
                if (prev !== toString()) {
                    const pos3 = safeSave();
                    doHighlight(editor);
                    safeRestore(pos3);
                    onUpdate(toString());
                }
            }
        }
    }
    function handleUndoRedo(event) {
        if (isUndo(event)) {
            preventDefault(event);
            at--;
            const record = history[at];
            if (record) {
                editor.innerHTML = record.html;
                safeRestore(record.pos);
            }
            if (at < 0) at = 0;
        }
        if (isRedo(event)) {
            preventDefault(event);
            at++;
            const record = history[at];
            if (record) {
                editor.innerHTML = record.html;
                safeRestore(record.pos);
            }
            if (at >= history.length) at--;
        }
    }
    function recordHistory() {
        if (!focus) return;
        const html = editor.innerHTML;
        const pos = safeSave();
        const lastRecord = history[at];
        if (lastRecord) {
            if (lastRecord.html === html
                && lastRecord.pos.start === pos.start
                && lastRecord.pos.end === pos.end)
                return;
        }
        at++;
        history[at] = { html, pos };
        history.splice(at + 1);
        const maxHistory = 300;
        if (at > maxHistory) {
            at = maxHistory;
            history.splice(0, 1);
        }
    }
    function handlePaste(event) {
        if (event.defaultPrevented) return;
        preventDefault(event);
        const originalEvent = event.originalEvent ?? event;
        const text = originalEvent.clipboardData.getData('text/plain').replace(/\r\n?/g, '\n');
        const pos = safeSave();
        insert(text);
        doHighlight(editor);
        safeRestore({
            start: Math.min(pos.start, pos.end) + text.length,
            end: Math.min(pos.start, pos.end) + text.length,
            dir: '<-',
        });
    }
    function handleCut(event) {
        const pos = safeSave();
        const selection = getSelection();
        const originalEvent = event.originalEvent ?? event;
        originalEvent.clipboardData.setData('text/plain', selection.toString());
        document.execCommand('delete');
        doHighlight(editor);
        safeRestore({
            start: Math.min(pos.start, pos.end),
            end: Math.min(pos.start, pos.end),
            dir: '<-',
        });
        preventDefault(event);
    }
    function visit(editor, visitor) {
        const queue = [];
        if (editor.firstChild) queue.push(editor.firstChild);
        let el = queue.pop();
        while (el) {
            if (visitor(el) === 'stop') break;
            if (el.nextSibling) queue.push(el.nextSibling);
            if (el.firstChild) queue.push(el.firstChild);
            el = queue.pop();
        }
    }
    function isCtrl(event) { return event.metaKey || event.ctrlKey; }
    function isUndo(event) { return isCtrl(event) && !event.shiftKey && getKeyCode(event) === 'Z'; }
    function isRedo(event) { return isCtrl(event) && event.shiftKey && getKeyCode(event) === 'Z'; }
    function isCopy(event) { return isCtrl(event) && getKeyCode(event) === 'C'; }
    function getKeyCode(event) {
        let key = event.key || event.keyCode || event.which;
        if (!key) return undefined;
        return (typeof key === 'string' ? key : String.fromCharCode(key)).toUpperCase();
    }
    function insert(text) {
        text = text
            .replace(/&/g, '&amp;')
            .replace(/</g, '&lt;')
            .replace(/>/g, '&gt;')
            .replace(/"/g, '&quot;')
            .replace(/'/g, '&#039;');
        document.execCommand('insertHTML', false, text);
    }
    function debounce(cb, wait) {
        let timeout = 0;
        return (...args) => {
            clearTimeout(timeout);
            timeout = window.setTimeout(() => cb(...args), wait);
        };
    }
    function findPadding(text) {
        let i = text.length - 1;
        while (i >= 0 && text[i] !== '\n') i--;
        i++;
        let j = i;
        while (j < text.length && /[ \t]/.test(text[j])) j++;
        return [text.substring(i, j) || '', i, j];
    }
    function toString() { return editor.textContent || ''; }
    function preventDefault(event) { event.preventDefault(); }
    function getSelection() {
        // @ts-ignore
        return editor.getRootNode().getSelection();
    }
    return {
        updateOptions(newOptions) { Object.assign(options, newOptions); },
        updateCode(code, callOnUpdate = true) {
            editor.textContent = code;
            doHighlight(editor);
            callOnUpdate && onUpdate(code);
        },
        onUpdate(callback) { onUpdate = callback; },
        toString,
        save,
        restore,
        recordHistory,
        destroy() {
            for (let [type, fn] of listeners) {
                editor.removeEventListener(type, fn);
            }
        },
    };
}
