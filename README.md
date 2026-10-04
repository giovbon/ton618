
```yaml
services:
  ton618-pkm:
    image: giovbon/ton618_core:latest
    container_name: ton618
    restart: unless-stopped
    user: root
    ports:
      - "6180:6180"
    environment:
      - DOCS_DIR=/app/docs
      - DB_PATH=/app/data/ton618.db
      - STATE_DIR=/app/data
      - MODEL_DIR=/app/data/models  # modelo de embeddings baixado 1x no boot
      - WEB_DIR=/app/web
      - PORT=6180
      - TZ=America/Sao_Paulo
      - AUTH_PASS=${AUTH_PASS:-ton618_secret}
    volumes:
      - ./docs:/app/docs
      - ./data:/app/data
```
