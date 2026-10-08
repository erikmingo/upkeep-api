# Go stack research (#1)

Target: a small JSON API behind the iOS app, with realistic seed data that is cheap to spin up for dev, demos and tests. Bias: stdlib first, one dependency per slot, every pick swappable.

Go version: **1.27** (Aug 2026). Relevant: `encoding/json` now backed by v2 (faster unmarshal, stricter UTF-8/duplicate-key handling), `net/http` ServeMux has had method + path-param routing since 1.22, `go.mod` `tool` directive pins CLIs. The machine has go1.23.2; a `go 1.27` line in `go.mod` makes the toolchain auto-download.

## Candidates per slot

### HTTP router

| Candidate | For | Against |
|---|---|---|
| **stdlib `net/http` ServeMux** | `GET /users/{id}` patterns, no dep, plain `http.Handler`, 2026 benchmarks within ~4% of the frameworks on a real socket | No route groups, no middleware chain helper, no struct binding |
| chi | Stays on `http.Handler`; groups + middleware chain | Only buys groups/chains; ~7 allocs/req for ctx params |
| gin / echo | Binding, validation, batteries | Own handler type, own ecosystem; speed gap vs stdlib is gone |

### DB access

| Candidate | For | Against |
|---|---|---|
| **sqlc** + **pgx/v5** | Write SQL, get typed Go; no runtime reflection; fastest; `seedling-gen` reads its config directly | Codegen step; joins returning ad-hoc shapes need a query each |
| bun | SQL-first query builder, light | Smaller community; still a runtime builder |
| ent | Schema-as-code, graph-y | Heavy codegen + its own migration story; overkill at this size |
| GORM | Biggest community | Reflection, N+1 traps, magic |
| sqlx | Thin `database/sql` helpers | Hand-written scanning, no type checking of SQL |

Engine: **PostgreSQL**. SQLite (modernc, pure Go) is tempting for zero infra but the hosted path for a phone-backed API is Postgres, sqlc's Postgres engine is its best one, and one `docker compose up` is the whole local cost.

### Migrations

| Candidate | For | Against |
|---|---|---|
| **goose v3** | Plain SQL files in `embed.FS`, Go-function migrations when SQL can't, Provider API (`provider.Up(ctx)` at startup or from a CLI), active (v3.28, Sep 2026) | — |
| golang-migrate | Widest driver list | SQL only, slower cadence |
| atlas | Declarative diffing, linting | HCL schema DSL, single-vendor |
| tern | pgx-native | Tiny community |

### Seeding / factories / fake data (the main focus)

Three layers, each a separate pick:

| Layer | Candidate | For | Against |
|---|---|---|---|
| Fake values | **gofakeit v7** | 300+ generators, zero deps, seedable `NewFaker` for reproducible runs, `Struct()` via `fake:` tags, v7.17 Sep 2026 | — |
| | jaswdr/faker | PHP-Faker port | Smaller surface, no struct tags |
| FK-aware builders | **seedling** | Builds only the rows a test asks for and inserts FK parents in order; `seedling-gen sqlc` generates blueprints from `sqlc.yaml`; `WithTx` rolls back per test; deterministic data; v0.4.x Aug 2026, MIT | Young (v0.x); you fill in the `Insert` callbacks |
| | bluele/factory-go | factory_bot-style sequences/sub-factories | In-memory only, no FK/DB awareness, low activity |
| | standin | Generates fixture funcs from structs | No DB/FK story |
| Static datasets | testfixtures | Checked-in YAML loaded into the DB | Drifts from the schema; brittle at scale. Use only if a demo needs a fixed named dataset |

Dev seeding reuses the same blueprints: `cmd/seed` opens the DB, runs the seedling session with a fixed seed, done. One definition of "a realistic user" for tests, dev and demos.

### Config

| Candidate | For | Against |
|---|---|---|
| **caarlos0/env** | Env → struct via tags, zero deps, `required`/defaults, `time.Duration` | Env only (that is the point: 12-factor, same in Docker) |
| kelseyhightower/envconfig | Same idea | Less active |
| koanf / viper | Files + env + remote + reload | Nothing here needs files |

### Testing helpers

| Candidate | For | Against |
|---|---|---|
| **stdlib `testing` + `net/http/httptest`** | Handler tests need nothing else; `httptest.NewTestServer` + `synctest` in 1.27 | — |
| **seedling `WithTx`** | Per-test transaction rollback against the compose Postgres | — |
| testcontainers-go (postgres module) | Hermetic DB per test run | Docker-in-CI plumbing; add when CI exists, not before |
| testify | `assert`/`require` sugar | Plain `if got != want { t.Fatalf }` reads fine; skip |

## Recommendation

| Slot | Pick |
|---|---|
| Runtime | Go 1.27, `go.mod` `tool` directive for sqlc / goose / seedling-gen |
| HTTP | `net/http` ServeMux + hand-rolled middleware funcs |
| DB | PostgreSQL via `jackc/pgx/v5` + `sqlc` |
| Migrations | `pressly/goose/v3`, SQL files in `embed.FS`, run via Provider at startup (dev) and `go run ./cmd/migrate` (ops) |
| Seeding | `gofakeit/v7` values + `seedling` blueprints generated from `sqlc.yaml`; `cmd/seed` for dev data |
| Config | `caarlos0/env` |
| Tests | stdlib + `httptest`; DB tests use `seedling.WithTx` against the compose Postgres |

Rejected on principle: any framework that owns the handler signature (gin, echo), any ORM with runtime reflection (GORM), any config loader that reads files (viper, koanf).

Upgrade paths, if ever needed: ServeMux → chi (handlers unchanged); compose Postgres in tests → testcontainers (same `DATABASE_URL` contract); goose SQL → goose Go migration (same tool).

## Follow-up tickets (to create)

1. **Scaffold the API** — extends the #2 spike: `go.mod` at 1.27, `cmd/api` with ServeMux + `/health`, config via caarlos0/env, Dockerfile, `compose.yaml` with Postgres, Makefile targets `run` / `test` / `lint`.
2. **DB layer** — pgx + sqlc + goose; first migration (`users`); `sqlc.yaml`; `go generate` wiring; `cmd/migrate`.
3. **Seeding** — gofakeit + seedling blueprints from `sqlc.yaml`; `cmd/seed` with a fixed seed; test helper using `WithTx`.
4. **Test harness + CI** — handler tests via `httptest`, DB tests against compose Postgres, GitHub Actions workflow running `go vet` + `go test`.

## Sources

- Routers: [Go web frameworks 2026 (glukhov.org)](https://glukhov.org/app-architecture/api-architecture/go-web-frameworks-stdlib-chi-gin-echo-fiber/), [chi/gin/echo/fiber (pistack, Jul 2026)](https://www.pistack.xyz/posts/2026-07-31-go-http-routers-chi-gin-echo-fiber/)
- DB: [GORM vs Ent vs Bun vs sqlc (glukhov.org)](https://www.glukhov.org/app-architecture/data-access/comparing-go-orms-gorm-ent-bun-sqlc/), [sqlc on pkg.go.dev](https://pkg.go.dev/github.com/sqlc-dev/sqlc)
- Migrations: [goose v3](https://pkg.go.dev/github.com/pressly/goose/v3), [goose/dbmate/atlas guide (pistack, May 2026)](https://www.pistack.xyz/posts/2026-05-14-self-hosted-lightweight-database-migrations-goose-dbmate-atlas-guide/), [atlas on picking a migration tool](https://www.atlasgo.io/blog/2022/12/01/picking-database-migration-tool)
- Seeding: [seedling](https://pkg.go.dev/github.com/mhiro2/seedling), [gofakeit v7](https://pkg.go.dev/github.com/brianvoe/gofakeit/v7), [factory-go](https://pkg.go.dev/github.com/bluele/factory-go), [standin](https://pkg.go.dev/github.com/mickamy/standin)
- Config: [viper vs koanf (ITNEXT)](https://itnext.io/golang-configuration-management-library-viper-vs-koanf-eea60a652a22), [caarlos0/env](https://best-of-web.builder.io/library/caarlos0/env)
- Go 1.27: [release notes](https://go.dev/doc/go1.27), [testcontainers postgres module](https://pkg.go.dev/github.com/testcontainers/testcontainers-go/modules/postgres)
