# 04 — Organization membership phantom attributes

Status: implemented

## Problem

`langfuse_organization_membership` exposes `id`, `status` and `username`
(`provider/organization_membership_resource.go`), populated from
`OrganizationMembership.ID/Status/Username`.

The Langfuse server never returns these fields. Verified in
`web/src/ee/features/admin-api/server/memberships.ts`: GET and PUT return only
`userId`, `role`, `email`, `name`; the spec agrees. The generated models make
this explicit. Today `status`/`username` are always empty strings and `id`
falls back to `userId`.

## Scope

1. Keep `id` as an alias of `user_id`, and document that behavior.
2. Deprecate `status` and `username` while retaining their empty-string values
   for existing state compatibility; remove them only in a future major version.
3. Expose the API-provided `name` as a computed-only attribute.
4. Regenerate `docs/resources/organization_membership.md` with `make docs`.

## Acceptance criteria

- Unsupported `status` and `username` are marked deprecated without changing
  their empty-string state behavior.
- `id` remains the user ID alias for new state; reads and updates preserve
  non-user-ID values already stored in legacy state.
- API-provided `name` is exposed as computed-only.
- Unit tests cover the schema and state behavior; regenerate resource docs.
