variable "project_public_key" {
  type      = string
  sensitive = true
}

variable "project_secret_key" {
  type      = string
  sensitive = true
}

variable "organization_public_key" {
  type      = string
  sensitive = true
}

variable "organization_private_key" {
  type      = string
  sensitive = true
}

variable "annotation_queue_id" {
  type = string
}

# The user needs access to the project, here through an organization role.
resource "langfuse_organization_membership" "annotator" {
  email                    = "annotator@example.com"
  role                     = "MEMBER"
  organization_public_key  = var.organization_public_key
  organization_private_key = var.organization_private_key
}

resource "langfuse_annotation_queue_assignment" "annotator" {
  project_public_key = var.project_public_key
  project_secret_key = var.project_secret_key
  queue_id           = var.annotation_queue_id
  user_id            = langfuse_organization_membership.annotator.user_id
}
