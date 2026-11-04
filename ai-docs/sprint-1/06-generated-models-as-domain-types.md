# 06 — Generated models as domain types

Status: todo (after 01 and 03)

## Problem

The HTTP layer is generated, but `langfuse/` still hand-writes domain types
(`Project`, `OrganizationMembership`, `LlmConnection`, request structs, …) and
maps them to/from generated models. That mapping layer is the largest remaining
hand-written surface between the spec and the Terraform resources.

## Scope

1. Change the `AdminClient`, `OrganizationClient` and `LlmConnectionsClient`
   interfaces to accept and return `publicapi`/`adminapi` models directly
   (or thin aliases), resource by resource.
2. Move type conversion (pointers, `map[string]any` metadata, enums such as
   `publicapi.MembershipRole` / `LlmAdapter`) into the provider resources where
   Terraform types are built anyway.
3. Delete the now-unused domain structs; regenerate mocks (`make generate`);
   update provider unit tests.
4. Use generated enum values for schema validators (e.g. adapter and role
   `OneOf` lists) so new upstream values arrive via regeneration.

## Constraints

- `langfuse` is a public package since `0715904`; removing exported types is a
  breaking change for external importers. Ship in a major version or keep
  aliases for one release.
- No Terraform schema or state change in this feature.

## Acceptance criteria

- No hand-written struct in `langfuse/` duplicates a generated model.
- Unit and acceptance tests pass; empty plan against existing state.
