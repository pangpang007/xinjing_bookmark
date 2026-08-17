.PHONY: run build test font docker

run:
	go run .

build:
	go build -ldflags="-s -w" -o bin/bookjie-api .

test:
	go test ./...

font:
	bash scripts/download-font.sh

docker:
	docker compose up -d --build
