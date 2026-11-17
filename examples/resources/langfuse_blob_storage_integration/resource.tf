variable "organization_public_key" {
  type      = string
  sensitive = true
}

variable "organization_private_key" {
  type      = string
  sensitive = true
}

variable "project_id" {
  type = string
}

variable "export_secret_access_key" {
  type      = string
  sensitive = true
}

resource "langfuse_blob_storage_integration" "example" {
  organization_public_key  = var.organization_public_key
  organization_private_key = var.organization_private_key
  project_id               = var.project_id

  type              = "S3"
  bucket_name       = "langfuse-exports"
  region            = "eu-central-1"
  access_key_id     = "AKIAIOSFODNN7EXAMPLE"
  secret_access_key = var.export_secret_access_key
  prefix            = "langfuse/"

  export_frequency = "daily"
  export_mode      = "FROM_TODAY"
  file_type        = "JSONL"
}
