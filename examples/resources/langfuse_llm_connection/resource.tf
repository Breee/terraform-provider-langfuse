variable "project_public_key" {
  type      = string
  sensitive = true
}

variable "project_secret_key" {
  type      = string
  sensitive = true
}

variable "llm_secret_key" {
  type      = string
  sensitive = true
}

resource "langfuse_llm_connection" "example" {
  project_public_key  = var.project_public_key
  project_secret_key  = var.project_secret_key
  provider_name       = "openai-development"
  adapter             = "openai"
  secret_key          = var.llm_secret_key
  with_default_models = true
}
