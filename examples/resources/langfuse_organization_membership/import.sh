# Avoid putting literal credentials in shell history.
# Legacy user-ID-only imports do not populate credentials; use this full format.
terraform import langfuse_organization_membership.example "${USER_ID},${ORGANIZATION_PUBLIC_KEY},${ORGANIZATION_PRIVATE_KEY}"
