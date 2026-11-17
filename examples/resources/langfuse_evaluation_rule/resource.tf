variable "project_public_key" {
  type      = string
  sensitive = true
}

variable "project_secret_key" {
  type      = string
  sensitive = true
}

resource "langfuse_evaluator" "politeness" {
  project_public_key = var.project_public_key
  project_secret_key = var.project_secret_key
  name               = "politeness"
  type               = "llm_as_judge"
  prompt             = jsonencode([{ role = "user", content = "Is {{output}} polite?" }])
  output_definition  = jsonencode({ dataType = "BOOLEAN" })
}

resource "langfuse_evaluation_rule" "example" {
  project_public_key = var.project_public_key
  project_secret_key = var.project_secret_key
  name               = "chat-politeness"
  enabled            = true
  sampling           = 0.25

  # Omit to evaluate all data.
  filter = jsonencode([
    { type = "stringOptions", column = "name", operator = "any of", value = ["chat"] },
  ])

  evaluator_assignments = jsonencode([
    {
      evaluatorId     = langfuse_evaluator.politeness.id
      variableMapping = [{ variable = "output", source = "output" }]
    },
  ])
}
