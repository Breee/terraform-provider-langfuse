variable "project_public_key" {
  type      = string
  sensitive = true
}

variable "project_secret_key" {
  type      = string
  sensitive = true
}

resource "langfuse_evaluator" "example" {
  project_public_key = var.project_public_key
  project_secret_key = var.project_secret_key
  name               = "helpfulness"
  description        = "Rates how helpful an answer is."
  type               = "llm_as_judge"

  # Omit both to use the project's default evaluation model.
  model_provider = "openai"
  model          = "gpt-4o"

  prompt = jsonencode([
    { role = "system", content = "You are a strict judge." },
    { role = "user", content = "Rate how helpful {{output}} is as an answer to {{input}}." },
  ])

  variable_mapping = jsonencode([
    { variable = "input", source = "input" },
    { variable = "output", source = "output" },
  ])

  output_definition = jsonencode({
    dataType                   = "NUMERIC"
    minValue                   = 0
    maxValue                   = 1
    scoreReasoningInstructions = "Explain the rating in one sentence."
  })
}
