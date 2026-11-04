# Sprint 1: generated Langfuse API client

Goal: generate as much of the provider as possible from the Langfuse OpenAPI
specs, without hacks, and keep it maintainable.

## Done so far (uncommitted)

- Vendored specs: `openapi/langfuse.yml`, `openapi/langfuse-organizations.yml`
  (`make update-openapi`).
- Spec corrections as OpenAPI Overlays: `openapi/*.overlay.yml` (201 status
  codes, verified against the Langfuse server source).
- oapi-codegen v2.8.0 pinned in `tools/`, configs in
  `langfuse/{publicapi,adminapi}/oapi-codegen.yaml`, `make generate-api`
  (deterministic, verified twice).
- `langfuse/*_client.go` reimplemented on the generated clients; interfaces,
  domain types and mocks unchanged. Build, vet and unit tests pass in Docker.

## Backlog

| # | Feature | Priority | Blocked by |
| --- | --- | --- | --- |
| 01 | [Acceptance-test the generated client](01-acceptance-test-generated-client.md) | P0 | `LANGFUSE_EE_LICENSE_KEY` |
| 02 | [CI: generated-code drift check](02-ci-generated-code-drift-check.md) | P0 | — |
| 03 | [HTTP-level unit tests for the adapter layer](03-adapter-http-unit-tests.md) | P1 | — |
| 04 | [Organization membership phantom attributes](04-organization-membership-phantom-attributes.md) | P1 | 01 |
| 05 | [Upstream Langfuse spec fixes](05-upstream-spec-fixes.md) | P1 | — |
| 06 | [Generated models as domain types](06-generated-models-as-domain-types.md) | P2 | 01, 03 |
| 07 | [Evaluate Terraform schema generation](07-evaluate-schema-generation.md) | P3 | 06 |
| 08 | [Update the generation research doc](08-update-generation-docs.md) | P1 | — |
| 09 | [Housekeeping and commit plan](09-housekeeping-and-commit.md) | P0 | 01 |
