.PHONY: run test lint db

-include .env
export

run:
	go run ./cmd/api

test:
	go test ./...

lint:
	go vet ./...
	@test -z "$$(gofmt -l .)" || { gofmt -l .; echo "gofmt: files above need formatting"; exit 1; }

db:
	docker compose up -d --wait db
