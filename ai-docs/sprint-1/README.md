# Sprint 1: generated Langfuse API client

Goal: generate as much of the provider as possible from the Langfuse OpenAPI
specs, without hacks, and keep it maintainable.

## Implemented

- Vendored specs: `openapi/langfuse.yml`, `openapi/langfuse-organizations.yml`
  (`make update-openapi`).
- Spec corrections as OpenAPI Overlays: `openapi/*.overlay.yml` (201 status
  codes, verified against the Langfuse server source).
- oapi-codegen v2.8.0 pinned in `tools/`, configs in
  `langfuse/{publicapi,adminapi}/oapi-codegen.yaml`.
- `make generate` refreshes upstream specs and regenerates mocks, API clients,
  provider docs, and example formatting. `make generate-local` reproduces outputs
  from vendored specs; CI uses this path through `make check-generated`.
- `langfuse/*_client.go` reimplemented on the generated clients; interfaces,
  domain types and mocks unchanged.

## Open work (tracked as GitHub issues)

| # | Feature | Priority | Blocked by |
| --- | --- | --- | --- |
| 01 | [Acceptance-test the generated client](01-acceptance-test-generated-client.md) / [#7](https://github.com/Breee/terraform-provider-langfuse/issues/7) | P0 | Licensed Langfuse Enterprise test environment |
| 02 | [CI: generated-code drift check](02-ci-generated-code-drift-check.md) | Done | — |
| 03 | [HTTP-level unit tests for the adapter layer](03-adapter-http-unit-tests.md) / [#8](https://github.com/Breee/terraform-provider-langfuse/issues/8) | Done | — |
| 04 | [Organization membership phantom attributes](04-organization-membership-phantom-attributes.md) / [#6](https://github.com/Breee/terraform-provider-langfuse/issues/6) | Done | Real-server verification remains in 01 |
| 05 | [Upstream Langfuse spec fixes](05-upstream-spec-fixes.md) / [#3](https://github.com/Breee/terraform-provider-langfuse/issues/3) | P1 | Upstream review and release |
| 06 | [Generated models as domain types](06-generated-models-as-domain-types.md) / [#2](https://github.com/Breee/terraform-provider-langfuse/issues/2) | P2 | 01, 03 |
| 07 | [Evaluate Terraform schema generation](07-evaluate-schema-generation.md) / [#9](https://github.com/Breee/terraform-provider-langfuse/issues/9) | P3 | 06 |
| 08 | [Update the generation research doc](08-update-generation-docs.md) | Done | — |
| 09 | [Housekeeping and commit plan](09-housekeeping-and-commit.md) | Done | — |

## Human setup and publication

| Work | Issue |
| --- | --- |
| Choose the provider namespace and publish to Terraform and OpenTofu registries | [#4](https://github.com/Breee/terraform-provider-langfuse/issues/4) |
| Configure signing secrets and verify weekly signed releases and artifacts | [#1](https://github.com/Breee/terraform-provider-langfuse/issues/1) |
| Install/authorize Renovate and verify dependency groups | [#5](https://github.com/Breee/terraform-provider-langfuse/issues/5) |
