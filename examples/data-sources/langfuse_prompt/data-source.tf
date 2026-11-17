variable "project_public_key" {
  type      = string
  sensitive = true
}

variable "project_secret_key" {
  type      = string
  sensitive = true
}

# Without version or label, the version labelled "production" is read.
data "langfuse_prompt" "support" {
  project_public_key = var.project_public_key
  project_secret_key = var.project_secret_key
  name               = "support/answer"
}

data "langfuse_prompt" "staging" {
  project_public_key = var.project_public_key
  project_secret_key = var.project_secret_key
  name               = "support/answer"
  label              = "staging"
}

output "support_prompt" {
  value = data.langfuse_prompt.support.prompt
}
