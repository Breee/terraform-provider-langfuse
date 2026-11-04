# Agent instructions

- This repository is the Langfuse **Terraform provider**. Keep changes scoped
  here; do not confuse it with the separate Crossplane provider.
- Follow [`CONTRIBUTING.md`](CONTRIBUTING.md) for development and validation.
- `openapi/*.yml` are vendored inputs; keep source specs unchanged by hand and
  put corrections in overlays.
- Never hand-edit generated `*.gen.go` clients. Change specs, overlays, or
  pinned generator configuration, then run `make generate-api`.
- Terraform resource schemas and lifecycle/state compatibility remain
  hand-maintained in `provider/`; the generated clients do not generate complete
  resources.
- Run `make build`, `make test`, and `make check-generated` for Go changes.
  Run `make docs` when provider schemas or examples change. Run the linter
  before committing or pushing Go changes.
- Do not commit credentials, local `.env` files, Terraform state, or generated
  local binaries.
