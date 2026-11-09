resource "langfuse_organization" "example" {
  name = "Example Organization"
}

resource "langfuse_organization_api_key" "example" {
  organization_id = langfuse_organization.example.id
}

output "organization_secret_key" {
  value     = langfuse_organization_api_key.example.secret_key
  sensitive = true
}
