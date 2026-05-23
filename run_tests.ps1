$ProjectRoot = "\\wsl.localhost\Ubuntu\home\giobon\ton618plus"
wsl bash -c "cd /home/giobon/ton618plus && /usr/local/go/bin/go test -tags sqlite_fts5 -count=1 ./internal/db/ ./internal/api/ ./internal/search/ ./internal/processor/ ./internal/config/ ./internal/capture/ ./internal/watcher/ 2>&1" | Tee-Object -FilePath test_output.txt
