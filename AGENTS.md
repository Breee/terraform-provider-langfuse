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

## Hard rule: never hack around limitations

If the Langfuse API, the OpenAPI spec, or a tool has a limitation that may be a
bug or a design gap (missing endpoint, ignored `null`, wrong status code):

1. Stop. Do not work around it (no warn-only destroy, sentinel values, no-op
   reads, or retries that hide the error).
2. Record it in [`ai-docs/caveats.md`](ai-docs/caveats.md) (next C-number) with
   the exact symptom and versions.
3. Write a submittable report in `ai-docs/upstream/<project>-<slug>.md` (repro,
   expected vs actual, versions). Search for an existing upstream issue or
   discussion first and link it.
4. Tell the user and let them decide. A workaround ships only after the user
   approves it; note the approval in the caveat entry.

## Hard rules for issues

Load the `project-management` skill (`.opencode/skills/project-management/`)
before creating, starting, or closing work.

Every issue must contain these sections, in this order:

- **WHAT**: the concrete change, including affected resources, endpoints, and files.
- **WHY**: the user or maintenance problem it solves.
- **HOW**: the intended approach, design decisions, and known constraints.
- **DONE**: a checklist of objectively verifiable completion criteria.
- **GOOD**: the quality bar beyond done, such as an empty follow-up plan, import
  support, drift handling, secrets kept out of state and logs, and documented
  limitations.
- **PROOF**: how completion is demonstrated. Before closing, post the evidence
  as an issue comment: the commit, the commands run, and their decisive output
  (acceptance test results against the local stack, `make check-generated`,
  `golangci-lint run`). Never include credentials or license keys.

Additional rules:

- Do not close an issue without posted proof; "should work" is not proof.
- Every new resource or data source ships with examples under `examples/`,
  unit tests, and an acceptance test run against the local stack.
- If the API cannot support part of the Terraform lifecycle (for example, no
  delete endpoint), state the limitation in the issue and in the resource docs.
