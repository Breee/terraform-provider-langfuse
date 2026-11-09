# API and provider generation

## What is generated

| Output | Inputs | Generator |
| --- | --- | --- |
| `langfuse/mocks/*.go` | Interfaces and `go:generate` directives in `langfuse/` | `github.com/golang/mock/mockgen`, pinned by the root `go.mod` |
| `langfuse/publicapi/publicapi.gen.go` | `openapi/langfuse.yml`, its overlay, and operation selection | `oapi-codegen` v2.8.0, configured in `langfuse/publicapi/oapi-codegen.yaml` |
| `langfuse/adminapi/adminapi.gen.go` | `openapi/langfuse-organizations.yml` and its overlay | `oapi-codegen` v2.8.0, configured in `langfuse/adminapi/oapi-codegen.yaml` |
| `docs/` | Provider schemas/descriptions, `templates/index.md.tmpl`, `examples/provider/`, and `examples/resources/<resource_type>/` | `terraform-plugin-docs` from `tools/go.mod` |
| `examples/*.tf` formatting | Terraform examples | `terraform fmt` |

Generated files are committed. Edit their source inputs or generator
configuration, not generated output.
`resource.tf` supplies each resource example and `import.sh` supplies its import
commands. The default generator templates render resource descriptions and all
schema attributes. `make validate-docs` checks the rendered registry pages against
the actual provider schema.

## Normal development flow

Run:

```sh
make generate
# Review changes, then stage expected generated outputs.
make check-generated
```

`make generate` first downloads both published OpenAPI specs with bounded
timeouts and retries. Neither vendored spec is replaced unless both downloads
succeed and are nonempty. It then runs `make generate-local`, which executes
every repository generator in order (also when invoked with `make -j`):

1. `make generate-go` runs root-module `go generate ./...` for mocks.
2. `make generate-api` runs both `oapi-codegen` directives from the `tools`
   module. Its version and dependencies are pinned in `tools/go.mod` and
   `tools/go.sum`.
3. `make docs` formats examples and regenerates provider documentation using
   the pinned `terraform-plugin-docs` tool.

`make generate-local` skips spec downloads; use it for ordinary provider changes
or to regenerate after editing overlays. Go modules and Terraform tooling must
already be available to run entirely offline.

`make check-generated` runs `make generate-local`, then checks tracked outputs under
`langfuse/mocks`, `langfuse/{publicapi,adminapi}`, `docs/`, and `examples/`
against the Git index, and fails if generation creates untracked files in those
directories. When generated outputs are expected to change, stage those outputs
before running the check. CI runs from a clean checkout, so its index represents
the committed version. CI reads the Go version from `tools/go.mod` and installs
Terraform.

## Updating the Langfuse API inputs

`make generate` is the one-command refresh-and-regenerate entry point.
`make generate-local` and `make check-generated` use the vendored specifications
without downloading upstream changes, keeping CI reproducible.

To intentionally adopt a newer Langfuse API:

1. Run `make generate` to refresh both specifications and all generated outputs.
2. Review the entire `openapi/` diff for breaking or unrelated API changes.
3. Update the adjacent OpenAPI overlays for server/spec mismatches. Overlays
   currently correct create response codes and malformed error schemas; retain
   strict overlay application so obsolete actions fail visibly.
4. If overlays needed changes, run `make generate-local`. Review client, mock, docs, and
   example changes.
5. Update handwritten adapters in `langfuse/`, Terraform resource logic in
   `provider/`, and HTTP/unit/acceptance tests when endpoint behavior changes.
6. Run `make build`, `make test`, `make check-generated`, and
   `golangci-lint run`. Run acceptance tests against a configured Langfuse
   Enterprise test environment before release.

For input-only inspection, `make update-openapi` downloads just the specs.
Neither command rewrites overlays or hides generator errors. If upstream
changes invalidate a strict overlay, fix that overlay and rerun
`make generate-local`. Do not work around the error by disabling strict mode.

## Generation boundary and known limitations

OpenAPI currently generates typed HTTP clients and models, **not** complete
Terraform resources. Resource schemas, plan modifiers, state/import behavior,
secret preservation, list-based reads, idempotent upserts, and membership
workflows remain in `provider/` because the Langfuse specs do not fully express
those Terraform lifecycle semantics.

The adapters in `langfuse/` translate between generated API models and the
provider's stable domain interfaces. This is deliberate compatibility
isolation, but it duplicates some API models. Removing that layer requires
tests for existing state, imports, sensitive/write-only values, and lifecycle
behavior; do not make it an incidental generator cleanup.

Remaining quality work is tracked in the repository issues: HTTP-level adapter
tests, real-server acceptance coverage, membership attributes unsupported by
the API, and upstream Langfuse specification corrections.

## CI invariants

- Generator versions are pinned in Go modules; do not use an unpinned global
  generator binary.
- Every generated artifact is reproducible from vendored inputs with
  `make generate-local`.
- A spec or overlay change that changes generated code must include that output.
- CI never downloads upstream specs; adopting API changes is the developer
  workflow provided by `make generate`.
- The OpenAPI specs and overlays are not a substitute for testing Terraform
  plan/state compatibility.
