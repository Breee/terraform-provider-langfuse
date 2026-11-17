variable "organization_public_key" {
  type      = string
  sensitive = true
}

variable "organization_private_key" {
  type      = string
  sensitive = true
}

data "langfuse_user" "reviewer" {
  organization_public_key  = var.organization_public_key
  organization_private_key = var.organization_private_key
  email                    = "reviewer@example.com"
}

output "reviewer_id" {
  value = data.langfuse_user.reviewer.id
}
