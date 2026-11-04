# 02 — CI: generated-code drift check

Status: todo

## Problem

Generated clients are committed. Nothing in CI proves they match the vendored
specs, overlays and pinned generator, so hand edits or forgotten regeneration
would go unnoticed. The `tools` module now declares `go 1.25.0` while CI's
`setup-go` reads `go.mod` (`go 1.24.2`).

## Scope

1. Add a `generate` job to `.github/workflows/ci.yml`, mirroring the existing
   `docs` job:
   - `make generate-api`
   - `git diff --compact-summary --exit-code` with a message telling the author
     to run `make generate-api` and commit.
2. Confirm the `docs` job still works: `make docs` now runs
   `go generate -skip oapi-codegen ./...` in `tools/`, which needs Go 1.25
   (toolchain auto-switch or `go-version-file: tools/go.mod`).
3. Confirm `golangci-lint` (no repo config, defaults) ignores `*.gen.go`
   ("Code generated … DO NOT EDIT" header) and is clean on the adapters.
4. Add a `gomod` ecosystem for `/tools` to `.github/dependabot.yml` so
   oapi-codegen updates arrive as PRs (regeneration diff reviewed in the PR).
5. Optional: scheduled workflow running `make update-openapi generate-api` and
   opening a PR when the upstream spec changes.

## Acceptance criteria

- A PR that edits `langfuse/publicapi/publicapi.gen.go` by hand fails CI.
- A PR that changes an overlay without regenerating fails CI.
- `lint`, `docs`, `build`, `test-unit` stay green.
