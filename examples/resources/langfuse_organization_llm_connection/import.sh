# The API cannot return the LLM secret or extra header values after import.
# Supply those values in configuration before applying.
terraform import langfuse_organization_llm_connection.example "${ORGANIZATION_PUBLIC_KEY}:${ORGANIZATION_PRIVATE_KEY}:${CONNECTION_ID}"
