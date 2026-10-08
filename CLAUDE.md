# upkeep-api

Backend for Upkeep (companion: `erikmingo/upkeep-ios`). Workflow = the `up-*` skills in `~/workspace/upkeep/upkeep-skills` (`/up-start #N` → `/up-commit` → `/up-pr` → `/up-babysit`); conventions in `up-developer/SKILL.md`; board = GitHub Issues + the shared "Upkeep" Project.

Stack: Go 1.27, stdlib `net/http` (see `docs/stack-research.md` for the rest). Run `go run ./cmd/api`; test `go test ./...`; lint `go vet ./...`.
