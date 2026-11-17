package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

func TestAccLangfuseEvaluator(t *testing.T) {
	scope := testAccProjectScope(t)

	config := func(name, attrs string) string {
		return scope + `
resource "langfuse_evaluator" "full" {
  project_public_key = local.public_key
  project_secret_key = local.secret_key
  name               = "` + name + `"
  type               = "llm_as_judge"
` + attrs + `
}

resource "langfuse_evaluator" "minimal" {
  project_public_key = local.public_key
  project_secret_key = local.secret_key
  name               = "acc-minimal"
  type               = "llm_as_judge"
  prompt             = jsonencode([{ role = "user", content = "Is {{output}} polite?" }])
  output_definition  = jsonencode({ dataType = "BOOLEAN" })
}
`
	}
	initial := config("acc-helpfulness", `
  description    = "initial"
  model_provider = "openai"
  model          = "gpt-4o"
  prompt = jsonencode([
    { role = "system", content = "You are a strict judge." },
    { role = "user", content = "Rate {{output}} for {{input}}." },
  ])
  variable_mapping = jsonencode([
    { variable = "input", source = "input" },
    { variable = "output", source = "output", jsonPath = "$.answer" },
  ])
  output_definition = jsonencode({ dataType = "NUMERIC", minValue = 0, maxValue = 1, scoreReasoningInstructions = "Explain briefly." })
`)
	definition := `
  prompt            = jsonencode([{ role = "user", content = "Classify {{output}}." }])
  output_definition = jsonencode({ dataType = "CATEGORICAL", categories = ["good", "bad"], shouldAllowMultipleMatches = false })
`
	redefined := config("acc-helpfulness", definition)
	renamed := config("acc-helpfulness-renamed", definition+`
  description = "renamed"
`)

	noop := []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}
	update := resource.ConfigPlanChecks{
		PreApply: []plancheck.PlanCheck{
			plancheck.ExpectResourceAction("langfuse_evaluator.full", plancheck.ResourceActionUpdate),
			plancheck.ExpectResourceAction("langfuse_evaluator.minimal", plancheck.ResourceActionNoop),
		},
		PostApplyPostRefresh: noop,
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckOrganizationsDestroyed,
		Steps: []resource.TestStep{
			{
				Config:           initial,
				ConfigPlanChecks: resource.ConfigPlanChecks{PostApplyPostRefresh: noop},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("langfuse_evaluator.full", "id"),
					resource.TestCheckResourceAttr("langfuse_evaluator.full", "version", "1"),
					resource.TestCheckResourceAttr("langfuse_evaluator.full", "status", "paused"),
					resource.TestCheckResourceAttr("langfuse_evaluator.full", "model", "gpt-4o"),
					resource.TestCheckNoResourceAttr("langfuse_evaluator.minimal", "model"),
					resource.TestCheckNoResourceAttr("langfuse_evaluator.minimal", "variable_mapping"),
				),
			},
			{
				ResourceName:            "langfuse_evaluator.full",
				ImportState:             true,
				ImportStateIdFunc:       testAccCredentialImportID("langfuse_evaluator.full", "id"),
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"prompt", "variable_mapping", "output_definition"},
			},
			{
				// A changed definition creates a new version and clears the optional fields.
				Config:           redefined,
				ConfigPlanChecks: update,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("langfuse_evaluator.full", "version", "2"),
					resource.TestCheckNoResourceAttr("langfuse_evaluator.full", "description"),
					resource.TestCheckNoResourceAttr("langfuse_evaluator.full", "model_provider"),
					resource.TestCheckNoResourceAttr("langfuse_evaluator.full", "variable_mapping"),
				),
			},
			{
				// A metadata-only change keeps the version.
				Config:           renamed,
				ConfigPlanChecks: update,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("langfuse_evaluator.full", "name", "acc-helpfulness-renamed"),
					resource.TestCheckResourceAttr("langfuse_evaluator.full", "description", "renamed"),
					resource.TestCheckResourceAttr("langfuse_evaluator.full", "version", "2"),
				),
			},
			{
				ResourceName:            "langfuse_evaluator.minimal",
				ImportState:             true,
				ImportStateIdFunc:       testAccCredentialImportID("langfuse_evaluator.minimal", "id"),
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"prompt", "output_definition"},
			},
		},
	})
}
