.PHONY: run test lint db generate migrate

# pure-Go project; also sidesteps the broken Xcode clang on this Mac
export CGO_ENABLED=0

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

generate:
	go generate ./...

migrate:
	go run ./cmd/migrate $(filter-out $@,$(MAKECMDGOALS))

up down status:
	@:
