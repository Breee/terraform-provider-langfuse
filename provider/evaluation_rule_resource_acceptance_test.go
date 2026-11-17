package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

func TestAccLangfuseEvaluationRule(t *testing.T) {
	scope := testAccProjectScope(t) + `
resource "langfuse_evaluator" "polite" {
  project_public_key = local.public_key
  project_secret_key = local.secret_key
  name               = "acc-polite"
  type               = "llm_as_judge"
  prompt             = jsonencode([{ role = "user", content = "Is {{output}} polite?" }])
  output_definition  = jsonencode({ dataType = "BOOLEAN" })
}

resource "langfuse_evaluation_rule" "draft" {
  project_public_key = local.public_key
  project_secret_key = local.secret_key
  name               = "acc-draft"
  enabled            = false
}
`
	config := func(attrs string) string {
		return scope + `
resource "langfuse_evaluation_rule" "full" {
  project_public_key = local.public_key
  project_secret_key = local.secret_key
` + attrs + `
}
`
	}
	assignments := `
  evaluator_assignments = jsonencode([{
    evaluatorId     = langfuse_evaluator.polite.id
    variableMapping = [{ variable = "output", source = "output" }]
  }])
`
	initial := config(`
  name     = "acc-rule"
  sampling = 0.25
  filter   = jsonencode([{ type = "stringOptions", column = "name", operator = "any of", value = ["chat"] }])
` + assignments)
	unfiltered := config(`
  name = "acc-rule-renamed"
` + assignments)
	drafted := config(`
  name    = "acc-rule-renamed"
  enabled = false
`)

	noop := []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}
	update := resource.ConfigPlanChecks{
		PreApply: []plancheck.PlanCheck{
			plancheck.ExpectResourceAction("langfuse_evaluation_rule.full", plancheck.ResourceActionUpdate),
			plancheck.ExpectResourceAction("langfuse_evaluation_rule.draft", plancheck.ResourceActionNoop),
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
					resource.TestCheckResourceAttrSet("langfuse_evaluation_rule.full", "id"),
					resource.TestCheckResourceAttr("langfuse_evaluation_rule.full", "enabled", "true"),
					resource.TestCheckResourceAttr("langfuse_evaluation_rule.full", "sampling", "0.25"),
					resource.TestCheckResourceAttr("langfuse_evaluation_rule.draft", "enabled", "false"),
					resource.TestCheckResourceAttr("langfuse_evaluation_rule.draft", "sampling", "1"),
					resource.TestCheckNoResourceAttr("langfuse_evaluation_rule.draft", "filter"),
					resource.TestCheckNoResourceAttr("langfuse_evaluation_rule.draft", "evaluator_assignments"),
				),
			},
			{
				ResourceName:            "langfuse_evaluation_rule.full",
				ImportState:             true,
				ImportStateIdFunc:       testAccCredentialImportID("langfuse_evaluation_rule.full", "id"),
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"filter", "evaluator_assignments"},
			},
			{
				// Removing the filter and sampling resets them to match everything.
				Config:           unfiltered,
				ConfigPlanChecks: update,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("langfuse_evaluation_rule.full", "name", "acc-rule-renamed"),
					resource.TestCheckResourceAttr("langfuse_evaluation_rule.full", "sampling", "1"),
					resource.TestCheckNoResourceAttr("langfuse_evaluation_rule.full", "filter"),
				),
			},
			{
				// Disabling the rule allows removing all assignments.
				Config:           drafted,
				ConfigPlanChecks: update,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("langfuse_evaluation_rule.full", "enabled", "false"),
					resource.TestCheckNoResourceAttr("langfuse_evaluation_rule.full", "evaluator_assignments"),
				),
			},
			{
				ResourceName:      "langfuse_evaluation_rule.draft",
				ImportState:       true,
				ImportStateIdFunc: testAccCredentialImportID("langfuse_evaluation_rule.draft", "id"),
				ImportStateVerify: true,
			},
		},
	})
}
