# upkeep-api

```sh
cp .env.example .env     # PORT, DATABASE_URL
make db                  # Postgres 17 on localhost:5433 (5432 is usually taken by the TFA env)
make run                 # http://localhost:8099/health
make test && make lint     # DB test runs only when TEST_DATABASE_URL is set (make db, then: psql $DATABASE_URL -c 'create database upkeep_test')
make generate              # sqlc → internal/db (after editing internal/db/queries/*.sql or a migration)
make migrate status        # also: make migrate up | down
make seed                  # reset users to 20 fixed fake rows (truncates!)
docker build -t upkeep-api . && docker run --rm -p 8080:8080 -e DATABASE_URL=... upkeep-api
```

From a phone on the same Wi-Fi: `http://$(ipconfig getifaddr en0):8099/health`.

Migrations in `internal/db/migrations` (goose, embedded) run at API startup. Build with `CGO_ENABLED=0` (the Makefile sets it): the project is pure Go and this Mac's Xcode clang is broken.
