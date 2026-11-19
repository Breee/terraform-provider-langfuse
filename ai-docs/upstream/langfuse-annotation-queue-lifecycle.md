# Public API: annotation queues cannot be updated or deleted; assignments cannot be listed

Tracking issue: #31. Upstream link: not filed yet.

Project: langfuse/langfuse — version 4.77.0 (self-hosted). Caveats C02, C03. No existing issue or discussion found (search "annotation queue delete api", "annotation queue update api", 2026-11-18).

## What
`/api/public/annotation-queues` offers list, create and get. There is no update (name, description, scoreConfigIds) and no delete. `/api/public/annotation-queues/{queueId}/assignments` offers POST and DELETE but no GET.

## Repro
```
curl -u $PK:$SK -X DELETE -w '%{http_code}\n' $HOST/api/public/annotation-queues/$QUEUE_ID
curl -u $PK:$SK -X PATCH  -w '%{http_code}\n' -H 'Content-Type: application/json' -d '{"description":"x"}' $HOST/api/public/annotation-queues/$QUEUE_ID
curl -u $PK:$SK -w '%{http_code}\n' $HOST/api/public/annotation-queues/$QUEUE_ID/assignments
```
Actual (4.77.0, verified 2026-11-19): all three return `405 {"message":"Method not allowed","error":"MethodNotAllowedError"}`. Expected: `PATCH` and `DELETE` on a queue, `GET` on its assignments.

## Why it matters
Queue names are unique, so a queue created by automation can neither be corrected nor removed; a changed description needs a new queue under a new name. Without a list of assignments a client cannot detect that an assignment was removed in the UI.
