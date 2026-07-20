.PHONY: build-core build-docker build-commercial test

build-core:
	cd core && go build -o ton618 ./cmd/server

build-docker:
	docker compose build

build-commercial:
	cd core && go build -tags commercial -o ton618-pro ./cmd/server

test:
	cd core && go test ./... -count=1

clean:
	rm -f core/ton618 core/ton618-pro
