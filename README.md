repomix --remove-comments --compress --ignore "**/*_test.go,**/*.html,web/**/*"

# TON-618 v2 — Personal Knowledge Manager

**TON-618 v2** é um motor de busca pessoal (PKM) que indexa arquivos Markdown, combina busca textual **FTS5** e fornece um editor rico com suporte a Typst, planilhas, diagramas e desenhos. Frontend em **HTMX + Templ** com inicialização em **< 100ms**.

---

## 🚀 Instalação Rápida (Docker — Raspberry Pi / Self-host)

A imagem é multi-arch: suporta **AMD64** (x86) e **ARM64** (Raspberry Pi 4/5, Apple Silicon).

### 1. Pré-requisitos

```bash
# Docker
curl -fsSL https://get.docker.com | sh
sudo usermod -aG docker $USER
newgrp docker
```

### 2. Iniciar com Docker Compose

```bash
# Cria os diretórios locais
mkdir -p ~/ton618/{docs,data}
cd ~/ton618

# Baixa o docker-compose.yml
curl -fsSL https://raw.githubusercontent.com/giovbon/ton618plus/main/docker-compose.yml -o docker-compose.yml

# Edite as credenciais antes de subir:
nano docker-compose.yml  # altere AUTH_USER e AUTH_PASS

# Sobe o container
docker compose up -d
```

Acesse: **http://\<ip-do-pi\>:6180**

### 3. Apontar para seus documentos

No `docker-compose.yml`, mapeie o volume `docs` para onde seus arquivos Markdown estão:

```yaml
volumes:
  - /home/pi/meus-documentos:/app/docs   # ← sua pasta de notas
  - ./data:/app/data
```

---

## ⚙️ Variáveis de Ambiente

| Variável | Padrão | Descrição |
|---------|--------|-----------|
| `DOCS_DIR` | `/app/docs` | Diretório com os arquivos Markdown |
| `DB_PATH` | `/app/data/ton618.db` | Caminho do banco SQLite |
| `PORT` | `6180` | Porta HTTP |
| `AUTH_USER` | `admin` | Usuário de autenticação básica |
| `AUTH_PASS` | `ton618_secret` | Senha de autenticação básica |
| `POLL_INTERVAL_SEC` | `30` | Intervalo de re-indexação em segundos |
| `TZ` | `America/Sao_Paulo` | Timezone |

---

## 🐳 Deploy Multi-Arch (para publicar no Docker Hub)

```bash
# AMD64 + ARM64 (padrão)
./deploy.sh latest

# Apenas ARM64 (mais rápido — ideal para Pi)
./deploy.sh latest 1

# Tag específica
./deploy.sh v1.2.0
```

---

## 🔧 Build local (sem Docker)

```bash
# Dependências: Go 1.24+, Node 20+, templ
./run.sh
```

---

## 📦 Recursos

- 🔍 Busca full-text (SQLite FTS5)
- 📝 Editor Markdown rico (TipTap)
- 📊 Planilhas (JSSpreadsheet)
- 🎨 Desenhos (Excalidraw)
- 📘 Typst (compilação de documentos)
- 🧜 Diagramas Mermaid
- 🏷️ TODOs indexados por arquivo/seção
- 🔑 Extração automática de keywords (RAKE)
- 🌐 Frontend HTMX — sem frameworks JS pesados