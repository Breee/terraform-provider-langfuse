# Development

## Requirements

- Go 1.24.2 or newer for the provider module.
- Go 1.25 or newer for tools in `tools/`.
- Terraform CLI for `make docs`.
- Docker and a Langfuse Enterprise test environment for acceptance tests.

## Common commands

| Command | Purpose |
| --- | --- |
| `make build` | Build all Go packages. |
| `make test` | Run unit tests without external services. |
| `golangci-lint run` | Run the Go linter used by CI. |
| `make generate` | Regenerate Go mocks from `go:generate` directives. |
| `make generate-api` | Regenerate API clients from the checked-in OpenAPI specs. |
| `make check-generated` | Regenerate API clients and fail if generated clients differ from the committed files. |
| `make docs` | Regenerate Terraform provider documentation and format examples. |
| `make update-openapi` | Download the latest Langfuse specs; review the resulting changes before using them. |
| `make testacc` | Start the Docker test environment and run acceptance tests. |
| `make test-teardown` | Stop the Docker test environment and remove its volumes. |

Run `make build`, `make test`, `make check-generated`, and `golangci-lint run`
before submitting Go changes. Run `make docs` when schemas, resources, or
examples change. Acceptance tests require a working Docker daemon and an
Enterprise license configured for the test environment; see
[`TESTING.md`](TESTING.md). Never commit credentials or Terraform state.

## API client workflow

The upstream OpenAPI documents are vendored in `openapi/`. Keep generated
specifications unchanged by hand; place required corrections in the adjacent
OpenAPI overlays. Generator version and directives are in `tools/go.mod` and
`tools/openapi.go`; generator options are in
`langfuse/{publicapi,adminapi}/oapi-codegen.yaml`.

When an API operation changes:

1. If intentionally refreshing Langfuse's published specification, run
   `make update-openapi` and review the full spec diff.
2. Update the relevant overlay for Terraform-independent specification fixes.
3. Run `make generate-api`, then review generated client changes.
4. Update the handwritten client adapters, provider lifecycle logic, and tests
   as needed. Do not edit `*.gen.go` files directly.
5. Run `make build`, `make test`, and `make check-generated`.

The OpenAPI documents generate Go API clients, not complete Terraform
resources. Keep resource schemas and lifecycle/state behavior in `provider/`
unless a generated-resource migration proves compatibility for existing state,
imports, secrets, and deletion behavior. See
[`ai-docs/openapi-provider-generation.md`](ai-docs/openapi-provider-generation.md)
for the generation assessment and migration constraints.
