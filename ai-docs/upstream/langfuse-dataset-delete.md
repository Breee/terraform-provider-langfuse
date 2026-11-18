# Public API: no endpoint to delete a dataset

Project: langfuse/langfuse — version 4.77.0 (self-hosted). Caveat C01. No existing issue or discussion found (GitHub issue + discussion search "delete dataset api", 2026-11-18).

## What
A dataset can be created, listed and read through the public API, but not deleted. Dataset items and dataset runs have delete endpoints.

## Repro
```
curl -u $PK:$SK -H 'Content-Type: application/json' -d '{"name":"tmp"}' $HOST/api/public/v2/datasets
curl -u $PK:$SK -X DELETE -w '%{http_code}\n' $HOST/api/public/v2/datasets/tmp
```
Actual: `405`. Expected: `DELETE /api/public/v2/datasets/{datasetName}` removes the dataset (as the UI can).

## Why it matters
Infrastructure-as-code tools (Terraform provider) and CI clean-up cannot remove datasets they created; test projects accumulate datasets.
