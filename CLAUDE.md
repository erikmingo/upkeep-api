# upkeep-api

Backend for Upkeep (companion: `erikmingo/upkeep-ios`). Workflow = the `up-*` skills in `~/workspace/upkeep/upkeep-skills` (`/up-start #N` → `/up-commit` → `/up-pr` → `/up-babysit`); conventions in `up-developer/SKILL.md`; board = GitHub Issues + the shared "Upkeep" Project.

Stack: Go 1.27, stdlib `net/http`, config via caarlos0/env, Postgres via `compose.yaml` + pgx + sqlc (`internal/db`, queries in `internal/db/queries/*.sql`, `make generate`) + goose (`internal/db/migrations`, run at startup and via `make migrate up|down|status`). Seeding: seedling blueprints in `internal/seed` (shared by tests via `seedlingpgx.WithTx` and `make seed`). See `docs/stack-research.md`. `make run` / `make test` / `make lint` / `make db`; `.env` from `.env.example`; `CGO_ENABLED=0` always.
