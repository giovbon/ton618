#!/usr/bin/env bash
echo "USER: $(whoami)  HOME: $HOME"
echo "--- processos ton618 ---"
ps aux | grep -i ton618 | grep -v grep
echo "--- cwd dos processos ---"
for p in $(pgrep -f ton618 2>/dev/null); do
  echo -n "pid $p -> "
  readlink /proc/$p/cwd 2>/dev/null
done
echo "--- bancos encontrados (home + /tmp + /opt) ---"
find "$HOME" /tmp /opt -maxdepth 6 -name "*.db" -size +100k 2>/dev/null | head -20
echo "--- pastas de dados do app ---"
ls -la "$HOME/.local/share" 2>/dev/null | head -20
