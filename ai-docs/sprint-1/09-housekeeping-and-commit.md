# 09 — Housekeeping and commit plan

Status: completed

The OpenAPI inputs, overlays, pinned generators, clients, adapters, and
development documentation are committed. Temporary generator output is ignored
under `.work/`. Both the provider and nested `tools` modules declare their
required Go version in `go.mod`; CI reads those files.

The supported regeneration entry point is `make generate`; CI verifies it with
`make check-generated`. See [`API_GENERATION.md`](../API_GENERATION.md) for
inputs, ordering, and limitations.
