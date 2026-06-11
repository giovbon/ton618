import React from 'react';
import { createRoot } from 'react-dom/client';
import { Tldraw } from 'tldraw';
import 'tldraw/tldraw.css';

// Gerenciador de assets customizado para fazer upload no servidor em vez de usar base64 inline
const customAssetStore = {
    async upload(asset, file) {
        const formData = new FormData();
        formData.append('file', file);
        
        const response = await fetch('/api/upload-image', {
            method: 'POST',
            body: formData,
        });
        
        if (!response.ok) {
            throw new Error(`Erro no upload: ${response.statusText}`);
        }
        
        const data = await response.json();
        if (!data.ok) {
            throw new Error(`Erro no upload: ${data.error || 'Erro desconhecido'}`);
        }
        
        return { src: data.url };
    },
    resolve(asset) {
        return asset.props.src;
    }
};

// Componente React que renderiza o canvas do tldraw
function DrawingEditor({ initialState, onChange, onReady }) {
    return (
        <div className="tldraw-editor-wrapper" style={{ width: '100%', height: '100%', position: 'relative' }}>
            <Tldraw
                autoFocus={true}
                assets={customAssetStore}
                onMount={(editor) => {
                    // Configura tema escuro para combinar com o visual premium do TON-618
                    editor.user.updateUserPreferences({ colorScheme: 'dark' });
                    
                    if (initialState) {
                        try {
                            editor.loadSnapshot(initialState);
                        } catch (e) {
                            console.error("Erro ao carregar o estado inicial do tldraw:", e);
                        }
                    }

                    if (onReady) {
                        onReady(editor);
                    }

                    // Escuta alterações no canvas para disparar o auto-save
                    const cleanup = editor.store.listen(() => {
                        if (onChange) {
                            try {
                                const snapshot = editor.getSnapshot();
                                onChange(snapshot);
                            } catch (e) {
                                console.error("Erro ao obter snapshot do tldraw:", e);
                            }
                        }
                    });

                    return cleanup;
                }}
            />
        </div>
    );
}

// Inicializa a aplicação React no container HTML
window.initTldraw = (containerEl, options) => {
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
        console.error("Falha ao inicializar o tldraw:", err);
        containerEl.innerHTML = `<div style="padding: 20px; color: #f87171; font-family: monospace;">Erro ao inicializar o canvas do tldraw: ${err.message}</div>`;
        return null;
    }
};
