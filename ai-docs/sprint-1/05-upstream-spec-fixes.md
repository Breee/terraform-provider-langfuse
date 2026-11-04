# 05 — Upstream Langfuse spec fixes

Status: todo

## Problem

The published specs disagree with the server. We correct this locally with
overlays (`openapi/langfuse.overlay.yml`,
`openapi/langfuse-organizations.overlay.yml`); every overlay action is debt to
remove once upstream is fixed.

## Verified defects (langfuse/langfuse `main`, server source)

| Operation | Spec | Server |
| --- | --- | --- |
| `POST /api/public/projects` | 200 | 201 |
| `POST /api/public/projects/{projectId}/apiKeys` | 200 | 201 |
| `POST /api/public/scim/Users` | 200 | 201 |
| `PUT /api/public/llm-connections` | 200 | 201 on create, 200 on update |
| `POST /api/admin/organizations` | 200 | 201 |
| `POST /api/admin/organizations/{organizationId}/apiKeys` | 200 | 201 |

Also:

- The public spec declares `openapi: 3.0.1` but uses 14 OAS 3.1 `const`
  keywords. oapi-codegen v2.8.0 tolerates this; strict 3.0 validators
  (OpenAPI Generator) reject it. Upstream fix: emit `enum: [value]`.
- Error response bodies are declared as `type: string` but are JSON objects
  (`{error, message}` / `{message}`), with inconsistent keys across routes.

## Scope

1. Open an issue/PR on langfuse/langfuse. The spec is exported via
   `pnpm run openapi:export` and post-processed in
   `web/scripts/openapi/postprocess-openapi.ts`; fix the status codes at the
   source and lower `const` → `enum` there, with a validation step in CI
   (`openapi:check`).
2. After a Langfuse release ships the fix: `make update-openapi`, delete the
   matching overlay actions (`strict: true` flags actions that no longer take
   effect), `make generate-api`.

## Acceptance criteria

- Upstream issue/PR linked here.
- Overlay files shrink to zero actions over time; empty overlays and their
  config entries are removed.
