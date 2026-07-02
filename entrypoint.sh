#!/bin/sh
# Garante que os diretórios montados como volume tenham permissão de escrita.
# Necessário porque o Docker monta volumes com as permissões do host,
# que podem não corresponder ao UID do appuser dentro do container.
chmod -R 777 /app/data /app/docs 2>/dev/null || true

exec ./ton618
