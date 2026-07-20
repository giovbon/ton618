# TON-618 Desktop

Launcher desktop para o TON-618 (Wails + core separado).

## Estrutura

```
desktop/
├── main.go        ← Launcher Wails (inicia core + WebView)
├── go.mod         ← Módulo Go
└── wails.json     ← Config Wails
```

## Como funciona

O desktop é um **shell** que:
1. Localiza o binário `core-server` ao lado do executável
2. Inicia o core como processo filho
3. Abre uma WebView apontando para `http://127.0.0.1:6180`
4. Ao fechar, encerra o core gracefulmente

## Build

```bash
# 1. Build do core primeiro
cd ../core && go build -o core-server ./cmd/server

# 2. Build do desktop
cd ../desktop
cp ../core/core-server .   # coloca o core ao lado
go install github.com/wailsapp/wails/v2/cmd/wails@latest
wails build

# 3. Resultado em desktop/build/bin/TON-618
```

## Atualização

Para atualizar o core sem rebuildar o desktop:

```bash
cd ../core && git pull && go build -o core-server ./cmd/server
cp core-server ../desktop/build/bin/
```

O desktop sempre usa o core-server que está ao lado dele.

## Auto-update

O desktop verifica no GitHub Releases se há nova versão.
Bindings expostos no frontend:

- `window.runtime.CheckUpdate()` → { hasUpdate, latestVersion, downloadURL }
- `window.runtime.ApplyUpdate(url)` → baixa e substitui o core-server
