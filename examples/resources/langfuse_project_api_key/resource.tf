variable "project_id" {
  type = string
}

variable "organization_public_key" {
  type      = string
  sensitive = true
}

variable "organization_private_key" {
  type      = string
  sensitive = true
}

resource "langfuse_project_api_key" "example" {
  project_id               = var.project_id
  organization_public_key  = var.organization_public_key
  organization_private_key = var.organization_private_key
  note                     = "Managed by Terraform"
}

output "project_secret_key" {
  value     = langfuse_project_api_key.example.secret_key
  sensitive = true
}
