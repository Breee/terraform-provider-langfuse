# 02 — CI: generated-code drift check

Status: drift check implemented; full CI acceptance pending

## Problem

Generated clients are committed. CI must verify they match the vendored specs,
overlays, and pinned generator. The `tools` module declares `go 1.25.0`, while
the provider module declares `go 1.24.2`; jobs running tools should use
`tools/go.mod` so the required version is explicit.

## Scope

1. Add a `generated-clients` job to `.github/workflows/ci.yml`, mirroring the
   existing `docs` job:
   - `make check-generated`
   - Fail with a message telling the author to run `make generate-api` and commit.
2. The `docs` and `generated-clients` jobs use `tools/go.mod` for Go 1.25 and
   cache `tools/go.sum`.
3. Confirm `golangci-lint` (no repo config, defaults) ignores `*.gen.go`
   ("Code generated … DO NOT EDIT" header) and is clean on the adapters.
4. Add a `gomod` ecosystem for `/tools` to `.github/dependabot.yml` so
   oapi-codegen updates arrive as PRs (regeneration diff reviewed in the PR).
5. Optional: scheduled workflow running `make update-openapi generate-api` and
   opening a PR when the upstream spec changes.

## Acceptance criteria

- A PR that edits either generated client by hand fails `make check-generated`.
- A PR that changes an overlay and produces stale generated clients fails the
  `generated-clients` CI job.
- `lint`, `docs`, `build`, `test-unit` stay green.

`Makefile` now defines `check-generated`, and CI runs it with the Go version
from `tools/go.mod`. The full workflow result remains to be confirmed after
these changes are pushed.
