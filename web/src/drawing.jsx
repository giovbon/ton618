import React, { useEffect, useRef, useCallback } from 'react';
import { createRoot } from 'react-dom/client';
import { Excalidraw, serializeAsJSON, restore, getSceneVersion } from '@excalidraw/excalidraw';
import '../node_modules/@excalidraw/excalidraw/dist/prod/index.css';

const safeGetSceneVersion = (elements) => {
    if (typeof getSceneVersion === 'function') {
        return getSceneVersion(elements);
    }
    return (elements || []).reduce((acc, el) => acc + (el.version || 0), 0) + (elements || []).length;
};

function DrawingEditor({ initialState, onChange, onReady }) {
    const excalidrawRef = useRef(null);
    const onChangeRef = useRef(onChange);
    onChangeRef.current = onChange;

    // Parse do estado inicial (pode ser string JSON ou objeto já parseado)
    let initialData = null;
    if (initialState) {
        try {
            const raw = typeof initialState === 'string' ? JSON.parse(initialState) : initialState;
            // serializeAsJSON salva no formato { type, version, source, elements, appState, files }
            if (raw.elements || raw.appState) {
                initialData = restore(raw, null, null);
            }
        } catch (e) {
            console.error('Erro ao restaurar estado do Excalidraw:', e);
        }
    }

    const initialElements = initialData?.elements || [];
    const initialFiles = initialData?.files || {};

    const lastVersionRef = useRef(null);
    const lastFilesCountRef = useRef(null);

    if (lastVersionRef.current === null) {
        lastVersionRef.current = safeGetSceneVersion(initialElements);
    }
    if (lastFilesCountRef.current === null) {
        lastFilesCountRef.current = Object.keys(initialFiles).length;
    }

    const handleChange = useCallback((elements, appState, files) => {
        if (!onChangeRef.current) return;

        const currentVersion = safeGetSceneVersion(elements);
        const currentFilesCount = Object.keys(files || {}).length;

        if (currentVersion === lastVersionRef.current && currentFilesCount === lastFilesCountRef.current) {
            return;
        }

        lastVersionRef.current = currentVersion;
        lastFilesCountRef.current = currentFilesCount;

        try {
            const json = serializeAsJSON(elements, appState, files, 'local');
            const snapshot = JSON.parse(json);
            onChangeRef.current(snapshot);
        } catch (e) {
            console.error('Erro ao serializar Excalidraw:', e);
        }
    }, []);

    return (
        <div style={{ width: '100%', height: '100%', position: 'relative' }}>
            <Excalidraw
                excalidrawAPI={(api) => {
                    excalidrawRef.current = api;
                    if (onReady) {
                        onReady(api);
                    }
                }}
                initialData={initialData}
                onChange={handleChange}
                theme="dark"
            />
        </div>
    );
}

// Inicializa a aplicação React no container HTML
window.initDrawing = (containerEl, options) => {
    try {
        const root = createRoot(containerEl);
        root.render(
            <DrawingEditor
                initialState={options.initialState}
                onChange={options.onChange}
                onReady={options.onReady}
            />
        );
        return root;
    } catch (err) {
        console.error("Falha ao inicializar o Excalidraw:", err);
        containerEl.innerHTML = `<div style="padding: 20px; color: #f87171; font-family: monospace;">Erro ao inicializar o canvas do Excalidraw: ${err.message}</div>`;
        return null;
    }
};
