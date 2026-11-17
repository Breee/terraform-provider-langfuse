package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

func TestAccLangfuseModel(t *testing.T) {
	scope := testAccProjectScope(t)

	flat := func(pattern string) string {
		return scope + `
resource "langfuse_model" "flat" {
  project_public_key = local.public_key
  project_secret_key = local.secret_key
  model_name         = "acc-flat"
  match_pattern      = "` + pattern + `"
  unit               = "TOKENS"
  input_price        = 0.000001
  output_price       = 0.000002
}
`
	}
	tiered := `
resource "langfuse_model" "tiered" {
  project_public_key = local.public_key
  project_secret_key = local.secret_key
  model_name         = "acc-tiered"
  match_pattern      = "(?i)^(acc-tiered)$"
  start_date         = "2026-01-01T00:00:00Z"
  unit               = "TOKENS"
  tokenizer_id       = "openai"
  tokenizer_config   = jsonencode({ tokenizerModel = "gpt-4o", tokensPerMessage = 3, tokensPerName = 1 })
  pricing_tiers = jsonencode([
    { name = "Standard", isDefault = true, priority = 0, conditions = [], prices = { input = 0.000001 } },
    {
      name = "Long context", isDefault = false, priority = 1,
      conditions = [{ usageDetailPattern = "input", operator = "gt", value = 200000, caseSensitive = false }],
      prices     = { input = 0.000002 }
    },
  ])
}
`
	noop := resource.ConfigPlanChecks{
		PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckOrganizationsDestroyed,
		Steps: []resource.TestStep{
			{
				Config:           flat("(?i)^(acc-flat)$") + tiered,
				ConfigPlanChecks: noop,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("langfuse_model.flat", "id"),
					resource.TestCheckResourceAttr("langfuse_model.flat", "input_price", "0.000001"),
					resource.TestCheckNoResourceAttr("langfuse_model.flat", "pricing_tiers"),
					resource.TestCheckResourceAttrSet("langfuse_model.tiered", "pricing_tiers"),
					resource.TestCheckResourceAttr("langfuse_model.tiered", "tokenizer_id", "openai"),
				),
			},
			{
				Config: flat("(?i)^(acc-flat-v2)$") + tiered,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("langfuse_model.flat", plancheck.ResourceActionUpdate),
						plancheck.ExpectResourceAction("langfuse_model.tiered", plancheck.ResourceActionNoop),
					},
					PostApplyPostRefresh: noop.PostApplyPostRefresh,
				},
				Check: resource.TestCheckResourceAttr("langfuse_model.flat", "match_pattern", "(?i)^(acc-flat-v2)$"),
			},
			{
				ResourceName:      "langfuse_model.flat",
				ImportState:       true,
				ImportStateIdFunc: testAccCredentialImportID("langfuse_model.flat", "id"),
				ImportStateVerify: true,
			},
			{
				ResourceName:            "langfuse_model.tiered",
				ImportState:             true,
				ImportStateIdFunc:       testAccCredentialImportID("langfuse_model.tiered", "id"),
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"pricing_tiers", "tokenizer_config", "start_date"},
			},
		},
	})
}
