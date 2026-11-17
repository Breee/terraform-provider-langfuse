variable "project_public_key" {
  type      = string
  sensitive = true
}

variable "project_secret_key" {
  type      = string
  sensitive = true
}

resource "langfuse_dataset" "support_faq" {
  project_public_key = var.project_public_key
  project_secret_key = var.project_secret_key
  name               = "support/faq"
}

resource "langfuse_dataset_item" "password_reset" {
  project_public_key = var.project_public_key
  project_secret_key = var.project_secret_key
  dataset_name       = langfuse_dataset.support_faq.name
  input              = jsonencode({ question = "How do I reset my password?" })
  expected_output    = jsonencode("Use the forgot password link on the sign-in page.")
  metadata           = jsonencode({ source = "help-center" })
}

# A stable ID keeps the item addressable from SDKs and experiments.
resource "langfuse_dataset_item" "refund_policy" {
  project_public_key = var.project_public_key
  project_secret_key = var.project_secret_key
  dataset_name       = langfuse_dataset.support_faq.name
  id                 = "refund-policy"
  input              = jsonencode({ question = "What is the refund policy?" })
}
