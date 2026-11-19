# Public API: no endpoint to delete a score config

Tracking issue: #32. Upstream link: not filed yet.

Project: langfuse/langfuse — version 4.77.0 (self-hosted). Caveat C04. No existing issue or discussion found (search "delete score config api", 2026-11-18).

## What
Score configs can be created, read and patched (`isArchived`), but not deleted.

## Repro
```
curl -u $PK:$SK -X DELETE -w '%{http_code}\n' $HOST/api/public/score-configs/$CONFIG_ID
```
Actual (4.77.0, verified 2026-11-19): `405 {"message":"Method not allowed","error":"MethodNotAllowedError"}`. Expected: delete of a config that no score references, or a documented statement that archiving is the only supported removal.

## Why it matters
Automation can only archive. Archived configs keep their name, so test projects fill up with archived configs.
