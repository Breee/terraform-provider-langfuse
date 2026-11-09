# Avoid putting literal credentials in shell history.
terraform import langfuse_project_membership.example "${PROJECT_ID},${USER_ID},${ORGANIZATION_PUBLIC_KEY},${ORGANIZATION_PRIVATE_KEY}"
