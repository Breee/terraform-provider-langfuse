# 04 — Organization membership phantom attributes

Status: todo (confirm with 01 first)

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

1. Decide per attribute:
   - `id`: keep, documented as the user ID (current effective behaviour).
   - `status`, `username`: deprecate (framework `DeprecationMessage`), then
     remove in the next major version.
   - Consider adding `name` (returned by the API, not exposed today).
2. Remove `ID`, `Status`, `Username` from `langfuse.OrganizationMembership`
   once no longer read; regenerate mocks.
3. Update `docs/resources/organization_membership.md` via `make docs`.

## Acceptance criteria

- No attribute is documented that the API cannot provide.
- Existing state upgrades without a diff (`status`/`username` empty → null or
  deprecated no-op; add a state upgrader if the schema version changes).
- Unit and acceptance tests updated.
