# 01 — Acceptance-test the generated client

Status: blocked (`LANGFUSE_EE_LICENSE_KEY` not set)

## Problem

Unit tests mock the `langfuse` client interfaces, so the new HTTP layer (generated
clients + adapters) has never talked to a real Langfuse server. The admin API
used by the acceptance suite requires an Enterprise license.

## Scope

Run `make testacc` (Docker Compose stack in `testdata/`) against the generated
client and confirm every resource lifecycle still works.

## Behaviour changes to verify explicitly

| Area | Before | After |
| --- | --- | --- |
| Create endpoints (project, project API key, SCIM user, LLM connection, organization, organization API key) | Any 2xx accepted | Exact status from spec + overlay (201) |
| Organization / project membership delete | Success heuristic on `success`/message text | HTTP 200 = success |
| SCIM user create body | `emails[]` without `type`, no `name` | Adds `"type": ""` per email and `"name": {}` (spec-required, ignored by server) |
| LLM connection `createdAt`/`updatedAt` | Raw server string | `time.RFC3339Nano` re-formatted (not used by the provider) |
| Org API key create body | No body | `{}` |
| Transport/parse errors | `failed to make request: …` | Same prefix; parse errors now also surface here |

Error text for non-success responses is unchanged
(`request failed with status code N, response body: …`), which
`provider/organization_resource.go` relies on for
"Cannot delete organization with existing projects".

## Acceptance criteria

- `make testacc` passes with the stack from `testdata/docker-compose.yml`.
- Each create/read/update/delete/import step of `TestAccLangfuseWorkflow` passes.
- Any failure is traced to either a spec defect (fix via overlay + upstream, see
  05) or an adapter mapping bug (fix in `langfuse/*_client.go`).

## Notes

- The user exports the license key in their shell; it must never be pasted into
  chat or committed.
- The compose file uses `langfuse/langfuse:latest`; pin it to the version the
  vendored spec was taken from to make results reproducible.
