---
name: project-management
description: Use before creating, updating, starting, or closing any work item in terraform-provider-langfuse (Breee/terraform-provider-langfuse) — issues, bugs, features, enhancements, caveats, upstream reports, sprints. Defines issue types, required sections, GitHub Project fields, and the WHAT/WHY/HOW/DONE/GOOD/PROOF/REVIEW workflow.
---

# Project management for terraform-provider-langfuse

Source of truth: GitHub issues in `Breee/terraform-provider-langfuse` + GitHub Project **Terraform Provider Langfuse** (`gh project … 3 --owner Breee`, https://github.com/users/Breee/projects/3). Milestones are not used.

## Issue types (one label each, template in `.github/ISSUE_TEMPLATE/`)

| Label | Use when | Must contain |
|---|---|---|
| `enhancement` | new resource, data source, or changed behaviour | endpoints, affected files, current vs wanted |
| `bug` | the provider behaves wrong | repro, actual vs expected, versions |
| `caveat` | known, documented limitation (`ai-docs/caveats.md` C-number) | symptom, versions, severity |
| `upstream` | bug/limit in Langfuse or a dependency (hard rule: no workarounds) | upstream link, draft `ai-docs/upstream/*.md`, caveat C-number |

Every issue body has, in this order: **WHAT**, **WHY**, **HOW**, **DONE** (checkable criteria), **GOOD** (quality bar), **PROOF** (commands that prove DONE). See `AGENTS.md` ("Hard rules for issues"). No DONE/GOOD → not ready, Status stays Todo.

Title: imperative, specific (`Add langfuse_model resource`), no ticket prefixes except `upstream:`.

## Project fields

| Field | Values | Rule |
|---|---|---|
| Status | Todo → In Progress → Done | exactly one item In Progress per agent |
| Sprint | iteration (2 weeks) | set when pulled into a sprint; unset = backlog |

Blocked: label `blocked` + comment with blocker and link.

## Workflow

1. **Create** with `gh issue create -R Breee/terraform-provider-langfuse --label <type> --body-file <file>` with a body following the template; `gh project item-add 3 --owner Breee --url <issue>`.
2. **Start**: Sprint set, Status In Progress.
3. **Commit**: `<type>: <what> (#N)`, small steps.
4. **Verify**: `make generate-local`, `make test`, `git add -A && make check-generated`, `golangci-lint run`, `make validate-docs`, acceptance test against the local stack (`TESTING.md`). `make test` must show no FAIL before committing.
5. **Review**: spawn a subagent with the issue body + diff/commits + proof; it checks every DONE item and GOOD. Fix findings.
6. **Close**: comment with
   ```
   PROOF
   commits: abc123, def456
   $ <command>
   <decisive output>
   REVIEW
   subagent: <findings> → <resolution>
   ```
   then `gh issue close`, Status Done. Never include credentials or license keys.
7. **Change of plan** (postpone, not planned, descoped): update field/label + comment with reason immediately; descoped work gets its own issue.
8. **Limitation hit** (missing endpoint, ignored `null`, wrong status code): stop, follow `AGENTS.md` ("never hack around limitations") — caveat entry, upstream draft, `caveat`/`upstream` issue, user decides.

## Useful commands

```
gh project item-list 3 --owner Breee --format json -L 300
gh project field-list 3 --owner Breee --format json
gh issue list -R Breee/terraform-provider-langfuse -l caveat
```
Field values are set via `gh project item-edit --id <item> --project-id <PVT_…> --field-id <…> --single-select-option-id <…>` (or `--iteration-id`).
