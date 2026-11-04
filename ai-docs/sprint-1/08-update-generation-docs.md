# 08 — Update the generation research doc

Status: todo

## Problem

`ai-docs/openapi-provider-generation.md` (research date 2026-09-11) is outdated
and partly wrong.

## Corrections

- "No `/api/admin/organizations` operations" is wrong: the organizations spec
  exists at `https://cloud.langfuse.com/generated/organizations-api/openapi.yml`
  (OAS 3.0.1, Bearer auth, self-hosted instance management).
- Self-hosted Langfuse ≥ v4.36.0 serves its own spec at `/api/openapi.yaml`.
- Missing: `const` in a 3.0.1 spec breaks OpenAPI Generator validation;
  lossless lowering to `enum` fixes it; relabelling as 3.1 fails on the YAML
  alias limit.
- Missing: OpenAPI Generator v7.25.0 result — output compiles, resources are
  empty stubs (`Create` returns "Not Supported").
- Missing: the six 201-vs-200 status-code defects (see 05).
- Speakeasy: requires authentication; no public OSS/free pricing found.
- Recommendation changed: generated Go client (oapi-codegen v2.8.0, overlays,
  vendored specs, committed output, drift check) with hand-written Terraform
  resources; schema generation tracked in 07.

## Scope

Rewrite the Recommendation, Input assessment and Decision gate sections; add a
"Procedure C: oapi-codegen client" section describing `make update-openapi`,
overlays, `make generate-api`, and the adapter layer. Keep A/B as evaluated
alternatives with their outcomes.

## Acceptance criteria

- Every claim in the doc matches the repo state and verified sources.
- Commands in the doc run as written.
