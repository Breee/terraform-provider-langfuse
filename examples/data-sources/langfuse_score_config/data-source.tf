variable "project_public_key" {
  type      = string
  sensitive = true
}

variable "project_secret_key" {
  type      = string
  sensitive = true
}

data "langfuse_score_config" "accuracy" {
  project_public_key = var.project_public_key
  project_secret_key = var.project_secret_key
  name               = "accuracy"
}

output "accuracy_score_config_id" {
  value = data.langfuse_score_config.accuracy.id
}
