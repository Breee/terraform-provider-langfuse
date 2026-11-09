# The API cannot return the LLM secret or extra header values after import.
# Supply those values in configuration before applying.
terraform import langfuse_llm_connection.example "${PROJECT_PUBLIC_KEY}:${PROJECT_SECRET_KEY}:${CONNECTION_ID}"
