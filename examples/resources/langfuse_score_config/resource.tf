variable "project_public_key" {
  type      = string
  sensitive = true
}

variable "project_secret_key" {
  type      = string
  sensitive = true
}

resource "langfuse_score_config" "accuracy" {
  project_public_key = var.project_public_key
  project_secret_key = var.project_secret_key
  name               = "accuracy"
  data_type          = "NUMERIC"
  description        = "How accurate the answer is, from 0 (wrong) to 1 (correct)."
  min_value          = 0
  max_value          = 1
}

resource "langfuse_score_config" "tone" {
  project_public_key = var.project_public_key
  project_secret_key = var.project_secret_key
  name               = "tone"
  data_type          = "CATEGORICAL"
  categories = [
    { label = "formal", value = 0 },
    { label = "casual", value = 1 },
  ]
}

# BOOLEAN score configs get their True/False categories from Langfuse.
resource "langfuse_score_config" "hallucination" {
  project_public_key = var.project_public_key
  project_secret_key = var.project_secret_key
  name               = "hallucination"
  data_type          = "BOOLEAN"
}
