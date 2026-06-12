import React, { useEffect, useRef, useCallback } from 'react';
import { createRoot } from 'react-dom/client';
import { Excalidraw, serializeAsJSON, restore } from '@excalidraw/excalidraw';
import '../node_modules/@excalidraw/excalidraw/dist/prod/index.css';

function DrawingEditor({ initialState, onChange, onReady }) {
    const excalidrawRef = useRef(null);
    const onChangeRef = useRef(onChange);
    onChangeRef.current = onChange;

    const handleChange = useCallback((elements, appState, files) => {
        if (!onChangeRef.current) return;
        try {
            const json = serializeAsJSON(elements, appState, files, 'local');
            const snapshot = JSON.parse(json);
            onChangeRef.current(snapshot);
        } catch (e) {
            console.error('Erro ao serializar Excalidraw:', e);
        }
    }, []);

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
