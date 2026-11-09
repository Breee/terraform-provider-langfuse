# Development

## Requirements

- Go 1.27.3 or newer for both the provider and `tools/` modules.
- Terraform CLI for `make docs`.
- Docker and a Langfuse Enterprise test environment for acceptance tests.

## Common commands

| Command | Purpose |
| --- | --- |
| `make build` | Build all Go packages. |
| `make test` | Run unit tests without external services. |
| `golangci-lint run` | Run the Go linter used by CI. |
| `make generate` | Refresh both upstream API specs, then regenerate clients, mocks, docs, and format examples. |
| `make generate-local` | Regenerate everything from vendored specs without refreshing them. |
| `make check-generated` | Run all generation and fail if committed outputs are stale. |
| `make update-openapi` | Refresh upstream specs only, without regenerating outputs. |
| `make generate-api` | Regenerate API clients only from the checked-in OpenAPI specs. |
| `make docs` | Regenerate Terraform docs and format examples only. |
| `make testacc` | Start the Docker test environment and run acceptance tests. |
| `make test-teardown` | Stop the Docker test environment and remove its volumes. |

`make generate` is the one-command API update workflow. Review both specs and
generated output before committing. If a strict overlay no longer applies,
generation fails visibly: review the API change, adjust the overlay, and rerun
`make generate-local` without downloading the specs again.
Both downloads must succeed before either vendored spec is replaced.

For changes that should not adopt upstream API updates, use
`make generate-local`. CI uses the same pinned inputs through
`make check-generated`; it never refreshes upstream specs.

Run `make build`, `make test`, `make generate-local`, and `golangci-lint run` before
submitting changes. Stage expected generated outputs before running
`make check-generated`; CI runs that drift check on a clean checkout.
CI can also be triggered manually from the GitHub Actions page.
Acceptance tests require a working Docker daemon
and an Enterprise license configured for the test environment; see
[`TESTING.md`](TESTING.md). Never commit credentials or Terraform state.

## API client workflow

The upstream OpenAPI documents are vendored in `openapi/`. Keep generated
specifications unchanged by hand; place required corrections in the adjacent
OpenAPI overlays. Generator version and directives are in `tools/go.mod` and
`tools/openapi.go`; generator options are in
`langfuse/{publicapi,adminapi}/oapi-codegen.yaml`.

When an API operation changes:

1. Run `make generate` to refresh Langfuse's published specifications and all
   generated outputs. Review the full spec diff.
2. Update the relevant overlay for Terraform-independent specification fixes.
3. If inputs need correction, run `make generate-local`, then review the API
   client, mock, documentation, and example changes.
4. Update handwritten client adapters, provider lifecycle logic, and tests
   as needed. Do not edit `*.gen.go` files directly.
5. Run `make build`, `make test`, `make check-generated`, and
   `golangci-lint run`.

The OpenAPI documents generate Go API clients, not complete Terraform
resources. Keep resource schemas and lifecycle/state behavior in `provider/`
unless a generated-resource migration proves compatibility for existing state,
imports, secrets, and deletion behavior. See
[`ai-docs/API_GENERATION.md`](ai-docs/API_GENERATION.md) for the complete
workflow and known limitations, and
[`ai-docs/openapi-provider-generation.md`](ai-docs/openapi-provider-generation.md)
for the generation assessment and migration constraints.

## Releases

On Mondays, the release workflow validates the provider and publishes a signed
GitHub release with GoReleaser artifacts when there are commits since the last
stable version tag. Releases can also be started by pushing a `vMAJOR.MINOR.PATCH`
tag. The workflow requires the repository secrets `GPG_PRIVATE_KEY` and
`PASSPHRASE`; configure them in GitHub Actions settings, never in the repo.
See `.github/workflows/release.yml` and `.goreleaser.yml`.

Registry publication is not completed by uploading GitHub release assets alone:

- Confirm the canonical namespace and update the provider address and examples
  before publication ([#4](https://github.com/Breee/terraform-provider-langfuse/issues/4)).
- Sign in to the [Terraform Registry](https://developer.hashicorp.com/terraform/registry/providers/publishing),
  register the signing public key, and authorize the repository.
- Submit the provider and signing public key using the
  [OpenTofu registry issue forms](https://github.com/opentofu/registry#adding-providers-modules-or-gpg-keys-to-the-opentofu-registry).
  OpenTofu requires its GitHub issue form UI; CLI/API submissions are not processed.
- Verify installation of a signed release with both Terraform and OpenTofu.

Signing setup is tracked in
[#1](https://github.com/Breee/terraform-provider-langfuse/issues/1).
Without `GPG_PRIVATE_KEY`, the workflow fails before creating a release tag.

Renovate scans Go modules (root and `tools/`), GitHub Actions, container images,
Terraform providers/modules, and Terraform CLI versions. It groups updates by
ecosystem on a weekly schedule; see `.github/renovate.json`.
Enable the Renovate GitHub App for this repository before expecting update PRs
([#5](https://github.com/Breee/terraform-provider-langfuse/issues/5)).
