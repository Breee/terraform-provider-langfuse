variable "organization_public_key" {
  type      = string
  sensitive = true
}

variable "organization_private_key" {
  type      = string
  sensitive = true
}

data "langfuse_projects" "all" {
  organization_public_key  = var.organization_public_key
  organization_private_key = var.organization_private_key
}

output "project_names" {
  value = [for project in data.langfuse_projects.all.projects : project.name]
}
