# Agent instructions

- This repository is the Langfuse **Terraform provider**. Keep changes scoped
  here; do not confuse it with the separate Crossplane provider.
- Follow [`CONTRIBUTING.md`](CONTRIBUTING.md) for the development flow.
- Follow [`ai-docs/API_GENERATION.md`](ai-docs/API_GENERATION.md) for generated
  API inputs, outputs, and known boundaries.
- `openapi/*.yml` are vendored inputs; keep source specs unchanged by hand and
  put corrections in overlays.
- Never hand-edit generated files. Change inputs/configuration, then run
  `make generate` to refresh upstream specs and regenerate all outputs.
  Use `make generate-local` to regenerate from vendored specs without refreshing
  them; CI uses this path through `make check-generated`.
- Terraform resource schemas and lifecycle/state compatibility remain
  hand-maintained in `provider/`; the generated clients do not generate complete
  resources.
- Run `make build`, `make test`, `make check-generated`, and `golangci-lint run`
  for Go changes.
- Do not commit credentials, local `.env` files, Terraform state, or generated
  local binaries.
