# Caveats

Format: `CNN — date — (severity, owner) — symptom, versions` + what the provider does today + report.
Rule: see `../AGENTS.md` ("never hack around limitations"). Entries marked **UNAPPROVED** were built before that rule existed and need a user decision.

All observed on Langfuse 4.77.0 (self-hosted, `testdata/docker-compose.yml`), spec `openapi/langfuse.yml`.

- C01 — 2026-11-18 — (high, upstream) Datasets have no delete in the public API: `DELETE /api/public/v2/datasets/{name}` → 405; upstream `main` spec has list/create/get only. Provider: `langfuse_dataset` destroy removes it from state with a warning, the dataset stays. **UNAPPROVED.** Report: `upstream/langfuse-dataset-delete.md`.
- C02 — 2026-11-18 — (high, upstream) Annotation queues have no update and no delete in the public API, and names are unique (400 on duplicate). Provider: every attribute is replace-only, a plan-time error demands a rename, destroy only warns; old queues are orphaned. **UNAPPROVED.** Report: `upstream/langfuse-annotation-queue-lifecycle.md`.
- C03 — 2026-11-18 — (medium, upstream) Annotation queue assignments have create/delete but no read or list. Provider: `langfuse_annotation_queue_assignment` Read is a no-op → no drift detection, no import. **UNAPPROVED.** Same report as C02.
- C04 — 2026-11-18 — (medium, upstream) Score configs have no delete. Provider: destroy PATCHes `isArchived: true`; the name stays taken by the archived config. **UNAPPROVED.** Report: `upstream/langfuse-score-config-delete.md`.
- C05 — 2026-11-18 — (medium, upstream) `null` does not clear fields consistently. Dataset upsert ignores `null` description/metadata; dataset item upsert ignores `null` input/expectedOutput/metadata; `PATCH /score-configs/{id}` rejects `null` description (400); `PATCH /v2/evaluation-rules/{id}` rejects `null` filter/sampling (400). Provider: sends `""` / `{}` / `[]` / `1` and maps them back to null, or forces replacement (dataset item). **UNAPPROVED.** Report: `upstream/langfuse-null-clearing.md`.
- C06 — 2026-11-18 — (low, upstream) Prompt labels can only be added via `PATCH /v2/prompts/{name}/versions/{v}`; removing a label needs a new version. Provider: label removal creates a new prompt version. Documented in the resource docs.
- C07 — 2026-11-18 — (low, upstream) `GET /integrations/blob-storage/{id}` returns sync status, not the configuration. Provider: Read filters the organization-wide list.
- C08 — 2026-11-18 — (low, test stack) Code evaluators are rejected locally: 412 "Code evaluations are not enabled for this deployment". `langfuse_evaluator` with `type = "code"` is unit-tested only.
- C09 — 2026-11-18 — (low, upstream) The spec declares wrong success codes for several operations (real 201 where 200 is documented and vice versa), so the generated `JSON200` fields are unreliable. Provider: `apiCall(...).decode` accepts any 2xx. Tracked in #3.
- C10 — 2026-11-18 — (low, upstream) Users created through SCIM get organization role `NONE` and cannot be assigned to annotation queues (404 "User not found or not authorized"). Use `langfuse_organization_membership`.
