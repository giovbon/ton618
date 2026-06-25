#!/bin/bash
cd /home/giobon/ton618plus
export PATH=$PATH:/usr/local/go/bin:/home/giobon/go/bin
go run github.com/a-h/templ/cmd/templ@latest generate 2>&1
go build -tags sqlite_fts5 -ldflags="-s -w" -o ton618 ./cmd/server/ 2>&1
echo "EXIT: $?"
