variable "project_public_key" {
  type      = string
  sensitive = true
}

variable "project_secret_key" {
  type      = string
  sensitive = true
}

# Flat prices per unit.
resource "langfuse_model" "flat" {
  project_public_key = var.project_public_key
  project_secret_key = var.project_secret_key
  model_name         = "internal-llm"
  match_pattern      = "(?i)^(internal-llm)$"
  unit               = "TOKENS"
  input_price        = 0.000001
  output_price       = 0.000002
}

# Tiered prices: usage above 200k input tokens is billed at a higher rate.
resource "langfuse_model" "tiered" {
  project_public_key = var.project_public_key
  project_secret_key = var.project_secret_key
  model_name         = "internal-llm-long-context"
  match_pattern      = "(?i)^(internal-llm-long-context)$"
  start_date         = "2026-01-01T00:00:00Z"
  unit               = "TOKENS"
  tokenizer_id       = "openai"
  tokenizer_config = jsonencode({
    tokenizerModel   = "gpt-4o"
    tokensPerMessage = 3
    tokensPerName    = 1
  })
  pricing_tiers = jsonencode([
    {
      name       = "Standard"
      isDefault  = true
      priority   = 0
      conditions = []
      prices     = { input = 0.000001, output = 0.000002 }
    },
    {
      name      = "Long context"
      isDefault = false
      priority  = 1
      conditions = [{
        usageDetailPattern = "input"
        operator           = "gt"
        value              = 200000
        caseSensitive      = false
      }]
      prices = { input = 0.000002, output = 0.000004 }
    },
  ])
}
