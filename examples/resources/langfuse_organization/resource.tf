resource "langfuse_organization" "example" {
  name = "Example Organization"
  metadata = {
    environment = "development"
  }
}
