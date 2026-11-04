# 03 — HTTP-level unit tests for the adapter layer

Status: todo

## Problem

`langfuse/` has no tests. Provider unit tests use gomock, so status-code
handling, auth headers, request bodies and model mapping in
`langfuse/*_client.go` are only covered by acceptance tests (which need an
Enterprise license).

## Scope

Add `langfuse/*_client_test.go` using `net/http/httptest` servers that return
responses shaped like the real server (see 05 for verified status codes).

Cover at least:

- Auth: Basic auth for public clients, `Bearer` for the admin client.
- Base URL with a path prefix (`https://host/langfuse`) is preserved.
- 201 on create endpoints, 202 on project delete, 200/201 on LLM connection upsert.
- `DeleteLlmConnection` treats 404 as success.
- Non-success responses produce
  `request failed with status code N, response body: <body>`.
- Request bodies: omitted empty metadata/config/custom models/extra headers;
  `retention` always sent on project update; SCIM `active: true`.
- Mapping: metadata `map[string]any` → `map[string]string` (error on
  non-string values), `retentionDays` nil → 0, membership ID preserved on update.

## Acceptance criteria

- Tests run with `make test` (no Docker services, no license).
- Each behaviour change listed in 01 has a test.
