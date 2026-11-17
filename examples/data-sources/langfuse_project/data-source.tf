variable "project_public_key" {
  type      = string
  sensitive = true
}

variable "project_secret_key" {
  type      = string
  sensitive = true
}

data "langfuse_project" "current" {
  project_public_key = var.project_public_key
  project_secret_key = var.project_secret_key
}

output "project_id" {
  value = data.langfuse_project.current.id
}
