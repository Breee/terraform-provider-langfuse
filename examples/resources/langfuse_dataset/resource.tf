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
  description        = "Frequently asked support questions with reference answers."
  metadata           = jsonencode({ team = "support" })

  # Every dataset item must match these JSON Schemas.
  input_schema = jsonencode({
    type       = "object"
    properties = { question = { type = "string" } }
    required   = ["question"]
  })
  expected_output_schema = jsonencode({ type = "string" })
}
