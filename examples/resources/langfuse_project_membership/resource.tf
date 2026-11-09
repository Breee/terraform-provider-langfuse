variable "project_id" {
  type = string
}

variable "member_email" {
  type        = string
  description = "Email of an existing organization member."
}

variable "organization_public_key" {
  type      = string
  sensitive = true
}

variable "organization_private_key" {
  type      = string
  sensitive = true
}

resource "langfuse_project_membership" "example" {
  project_id               = var.project_id
  email                    = var.member_email
  role                     = "MEMBER"
  organization_public_key  = var.organization_public_key
  organization_private_key = var.organization_private_key
}
