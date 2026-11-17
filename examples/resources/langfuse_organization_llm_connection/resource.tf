variable "organization_public_key" {
  type      = string
  sensitive = true
}

variable "organization_private_key" {
  type      = string
  sensitive = true
}

variable "llm_secret_key" {
  type      = string
  sensitive = true
}

# Available to every project of the organization.
resource "langfuse_organization_llm_connection" "example" {
  organization_public_key  = var.organization_public_key
  organization_private_key = var.organization_private_key
  provider_name            = "openai-shared"
  adapter                  = "openai"
  secret_key               = var.llm_secret_key
  custom_models            = ["gpt-4o-mini"]
}

resource "langfuse_organization_llm_connection" "bedrock" {
  organization_public_key  = var.organization_public_key
  organization_private_key = var.organization_private_key
  provider_name            = "bedrock-shared"
  adapter                  = "bedrock"
  secret_key               = var.llm_secret_key
  config                   = jsonencode({ region = "eu-central-1" })
}
