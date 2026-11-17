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
  min_value          = 0
  max_value          = 1
}

# Annotation queues cannot be updated or deleted through the API: any change
# needs a new name, and the previous queue stays in Langfuse.
resource "langfuse_annotation_queue" "review" {
  project_public_key = var.project_public_key
  project_secret_key = var.project_secret_key
  name               = "expert-review"
  description        = "Traces flagged for review by a domain expert."
  score_config_ids   = [langfuse_score_config.accuracy.id]
}
