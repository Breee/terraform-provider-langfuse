variable "project_public_key" {
  type      = string
  sensitive = true
}

variable "project_secret_key" {
  type      = string
  sensitive = true
}

resource "langfuse_prompt" "summarize" {
  project_public_key = var.project_public_key
  project_secret_key = var.project_secret_key
  name               = "support/summarize"
  prompt             = "Summarize the following ticket in three sentences:\n\n{{ticket}}"
  labels             = ["production"]
  tags               = ["support"]
  commit_message     = "Managed by Terraform"
  config = jsonencode({
    model       = "gpt-4o"
    temperature = 0.2
  })
}

resource "langfuse_prompt" "assistant" {
  project_public_key = var.project_public_key
  project_secret_key = var.project_secret_key
  name               = "support/assistant"
  type               = "chat"
  labels             = ["production"]
  prompt = jsonencode([
    { role = "system", content = "You are a support assistant for {{product}}." },
    { type = "placeholder", name = "history" },
    { role = "user", content = "{{question}}" },
  ])
}
