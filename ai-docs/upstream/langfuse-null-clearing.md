# Public API: `null` does not clear optional fields consistently

Project: langfuse/langfuse — version 4.77.0 (self-hosted). Caveat C05. No existing issue found (2026-11-18).

## What
| Request | Field sent as `null` | Actual |
|---|---|---|
| `POST /api/public/v2/datasets` (upsert) | `description`, `metadata` | ignored, old value kept |
| `POST /api/public/dataset-items` (upsert) | `input`, `expectedOutput`, `metadata` | ignored, old value kept |
| `PATCH /api/public/score-configs/{id}` | `description` | 400 |
| `PATCH /api/public/v2/evaluation-rules/{id}` | `filter`, `sampling` | 400 |

Elsewhere `null` clears the field (dataset `inputSchema`, evaluator `description`, blob storage `endpoint`).

## Expected
One documented rule: `null` clears an optional field, an omitted field is left unchanged.

## Why it matters
Clients cannot unset these fields; they must send `""`, `{}` or `[]` instead, or recreate the object.
