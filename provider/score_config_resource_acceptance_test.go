package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

func TestAccLangfuseScoreConfig(t *testing.T) {
	scope := testAccProjectScope(t)

	config := func(numeric, categories string) string {
		return scope + `
resource "langfuse_score_config" "numeric" {
  project_public_key = local.public_key
  project_secret_key = local.secret_key
  data_type          = "NUMERIC"
` + numeric + `
}

resource "langfuse_score_config" "categorical" {
  project_public_key = local.public_key
  project_secret_key = local.secret_key
  name               = "acc-tone"
  data_type          = "CATEGORICAL"
  categories         = ` + categories + `
}

resource "langfuse_score_config" "boolean" {
  project_public_key = local.public_key
  project_secret_key = local.secret_key
  name               = "acc-boolean"
  data_type          = "BOOLEAN"
}

resource "langfuse_score_config" "text" {
  project_public_key = local.public_key
  project_secret_key = local.secret_key
  name               = "acc-text"
  data_type          = "TEXT"
}
`
	}
	initial := config(`
  name        = "acc-accuracy"
  description = "initial"
  min_value   = 0
  max_value   = 1
`, `[{ label = "formal", value = 0 }, { label = "casual", value = 1 }]`)
	updated := config(`
  name        = "acc-accuracy-v2"
  min_value   = -1
  max_value   = 5
  is_archived = true
`, `[{ label = "formal", value = 0 }, { label = "neutral", value = 1 }, { label = "casual", value = 2 }]`)
	unbounded := config(`
  name      = "acc-accuracy-v2"
  max_value = 5
`, `[{ label = "formal", value = 0 }, { label = "neutral", value = 1 }, { label = "casual", value = 2 }]`)

	noop := []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}
	action := func(numeric, categorical plancheck.ResourceActionType) resource.ConfigPlanChecks {
		return resource.ConfigPlanChecks{
			PreApply: []plancheck.PlanCheck{
				plancheck.ExpectResourceAction("langfuse_score_config.numeric", numeric),
				plancheck.ExpectResourceAction("langfuse_score_config.categorical", categorical),
				plancheck.ExpectResourceAction("langfuse_score_config.boolean", plancheck.ResourceActionNoop),
				plancheck.ExpectResourceAction("langfuse_score_config.text", plancheck.ResourceActionNoop),
			},
			PostApplyPostRefresh: noop,
		}
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckOrganizationsDestroyed,
		Steps: []resource.TestStep{
			{
				Config:           initial,
				ConfigPlanChecks: resource.ConfigPlanChecks{PostApplyPostRefresh: noop},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("langfuse_score_config.numeric", "id"),
					resource.TestCheckResourceAttr("langfuse_score_config.numeric", "description", "initial"),
					resource.TestCheckResourceAttr("langfuse_score_config.numeric", "is_archived", "false"),
					resource.TestCheckResourceAttr("langfuse_score_config.categorical", "categories.#", "2"),
					resource.TestCheckNoResourceAttr("langfuse_score_config.boolean", "categories"),
				),
			},
			{
				Config:           updated,
				ConfigPlanChecks: action(plancheck.ResourceActionUpdate, plancheck.ResourceActionUpdate),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("langfuse_score_config.numeric", "name", "acc-accuracy-v2"),
					resource.TestCheckNoResourceAttr("langfuse_score_config.numeric", "description"),
					resource.TestCheckResourceAttr("langfuse_score_config.numeric", "min_value", "-1"),
					resource.TestCheckResourceAttr("langfuse_score_config.numeric", "is_archived", "true"),
					resource.TestCheckResourceAttr("langfuse_score_config.categorical", "categories.2.label", "casual"),
				),
			},
			{
				ResourceName:      "langfuse_score_config.numeric",
				ImportState:       true,
				ImportStateIdFunc: testAccCredentialImportID("langfuse_score_config.numeric", "id"),
				ImportStateVerify: true,
			},
			{
				ResourceName:      "langfuse_score_config.categorical",
				ImportState:       true,
				ImportStateIdFunc: testAccCredentialImportID("langfuse_score_config.categorical", "id"),
				ImportStateVerify: true,
			},
			{
				ResourceName:      "langfuse_score_config.boolean",
				ImportState:       true,
				ImportStateIdFunc: testAccCredentialImportID("langfuse_score_config.boolean", "id"),
				ImportStateVerify: true,
			},
			{
				// The API cannot clear a bound, so removing min_value replaces the config.
				Config:           unbounded,
				ConfigPlanChecks: action(plancheck.ResourceActionDestroyBeforeCreate, plancheck.ResourceActionNoop),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr("langfuse_score_config.numeric", "min_value"),
					resource.TestCheckResourceAttr("langfuse_score_config.numeric", "is_archived", "false"),
				),
			},
		},
	})
}
