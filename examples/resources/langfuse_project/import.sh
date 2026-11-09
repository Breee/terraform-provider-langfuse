# Avoid putting literal credentials in shell history.
terraform import langfuse_project.example "${PROJECT_ID},${ORGANIZATION_ID},${ORGANIZATION_PUBLIC_KEY},${ORGANIZATION_PRIVATE_KEY}"
