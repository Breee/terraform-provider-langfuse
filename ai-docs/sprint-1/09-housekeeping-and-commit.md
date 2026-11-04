# 09 — Housekeeping and commit plan

Status: todo (commit after 01 passes)

## Open items

- `.work/openapi-generator/` holds the OpenAPI Generator spike output
  (untracked). Delete it (needs confirmation) or add `.work/` to `.gitignore`.
- `go.mod`: adding `github.com/oapi-codegen/runtime` v1.7.0 raised several
  `golang.org/x/*` minimums via MVS; review the diff.
- `tools/go.mod` is now `go 1.25.0` (oapi-codegen v2.8.0 requirement); the
  provider module stays on `go 1.24.2`.
- Docker runs for generation/tests should use `-u $(id -u):$(id -g)` and
  `GOFLAGS=-buildvcs=false`; consider a `make` target that wraps this.

## Commit plan (conventional commits, stage files explicitly, no `git add -A`)

1. `build: vendor Langfuse OpenAPI specs and pin oapi-codegen`
   — `openapi/langfuse.yml`, `openapi/langfuse-organizations.yml`,
   `tools/openapi.go`, `tools/go.mod`, `tools/go.sum`, `Makefile`
2. `fix: correct Langfuse spec status codes via overlays`
   — `openapi/*.overlay.yml`
3. `feat: generate Langfuse API clients`
   — `langfuse/publicapi/*`, `langfuse/adminapi/*`, `go.mod`, `go.sum`
4. `refactor: implement langfuse clients on generated API clients`
   — `langfuse/admin_client.go`, `langfuse/organization_client.go`,
   `langfuse/llm_connections_client.go`, `langfuse/client_factory.go`,
   `langfuse/utils.go`
   — note the removed exported types `ListOrganizationsResponse` and
   `DeleteProjectMembershipRequest` in the body.
5. `docs: add sprint-1 feature docs` — `ai-docs/`
