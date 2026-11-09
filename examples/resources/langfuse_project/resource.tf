variable "organization_id" {
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

resource "langfuse_project" "example" {
  name                     = "Example Project"
  organization_id          = var.organization_id
  organization_public_key  = var.organization_public_key
  organization_private_key = var.organization_private_key
  retention_days           = 90
  metadata = {
    environment = "development"
  }
}
