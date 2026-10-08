# upkeep-api

Backend for Upkeep (companion: `erikmingo/upkeep-ios`). Workflow = the `up-*` skills in `~/workspace/upkeep/upkeep-skills` (`/up-start #N` → `/up-commit` → `/up-pr` → `/up-babysit`); conventions in `up-developer/SKILL.md`; board = GitHub Issues + the shared "Upkeep" Project.

Stack: Go 1.27, stdlib `net/http`, config via caarlos0/env, Postgres via `compose.yaml` (see `docs/stack-research.md`). `make run` / `make test` / `make lint` / `make db`; `.env` from `.env.example`.
