variable "member_email" {
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

resource "langfuse_organization_membership" "example" {
  email                    = var.member_email
  role                     = "MEMBER"
  organization_public_key  = var.organization_public_key
  organization_private_key = var.organization_private_key
}
