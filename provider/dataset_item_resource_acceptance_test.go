package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

func TestAccLangfuseDatasetItem(t *testing.T) {
	scope := testAccProjectScope(t)

	config := func(generated, named string) string {
		return scope + `
resource "langfuse_dataset" "faq" {
  project_public_key = local.public_key
  project_secret_key = local.secret_key
  name               = "acc-faq"
  input_schema       = jsonencode({ type = "object", properties = { question = { type = "string" } }, required = ["question"] })
}

resource "langfuse_dataset_item" "generated" {
  project_public_key = local.public_key
  project_secret_key = local.secret_key
  dataset_name       = langfuse_dataset.faq.name
` + generated + `
}

resource "langfuse_dataset_item" "named" {
  project_public_key = local.public_key
  project_secret_key = local.secret_key
  dataset_name       = langfuse_dataset.faq.name
  id                 = "acc-refund-policy"
` + named + `
}
`
	}
	initial := config(`
  input           = jsonencode({ question = "How do I reset my password?" })
  expected_output = jsonencode("Use the forgot password link.")
  metadata        = jsonencode({ source = "docs" })
`, `
  input = jsonencode({ question = "What is the refund policy?" })
`)
	updated := config(`
  input           = jsonencode({ question = "How do I reset my password?", locale = "en" })
  expected_output = jsonencode({ answer = "Use the forgot password link." })
  metadata        = jsonencode({ source = "docs" })
`, `
  input  = jsonencode({ question = "What is the refund policy?" })
  status = "ARCHIVED"
`)
	removed := config(`
  input = jsonencode({ question = "How do I reset my password?", locale = "en" })
`, `
  input = jsonencode({ question = "What is the refund policy?" })
`)

	noop := []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}
	action := func(generated, named plancheck.ResourceActionType) resource.ConfigPlanChecks {
		return resource.ConfigPlanChecks{
			PreApply: []plancheck.PlanCheck{
				plancheck.ExpectResourceAction("langfuse_dataset_item.generated", generated),
				plancheck.ExpectResourceAction("langfuse_dataset_item.named", named),
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
					resource.TestCheckResourceAttrSet("langfuse_dataset_item.generated", "id"),
					resource.TestCheckResourceAttr("langfuse_dataset_item.generated", "status", "ACTIVE"),
					resource.TestCheckResourceAttr("langfuse_dataset_item.named", "id", "acc-refund-policy"),
					resource.TestCheckNoResourceAttr("langfuse_dataset_item.named", "expected_output"),
				),
			},
			{
				Config:           updated,
				ConfigPlanChecks: action(plancheck.ResourceActionUpdate, plancheck.ResourceActionUpdate),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("langfuse_dataset_item.named", "status", "ARCHIVED"),
				),
			},
			{
				ResourceName:            "langfuse_dataset_item.generated",
				ImportState:             true,
				ImportStateIdFunc:       testAccCredentialImportID("langfuse_dataset_item.generated", "id"),
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"input", "expected_output", "metadata"},
			},
			{
				ResourceName:            "langfuse_dataset_item.named",
				ImportState:             true,
				ImportStateIdFunc:       testAccCredentialImportID("langfuse_dataset_item.named", "id"),
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"input"},
			},
			{
				// The API cannot clear a value, so removing one replaces the item.
				Config:           removed,
				ConfigPlanChecks: action(plancheck.ResourceActionDestroyBeforeCreate, plancheck.ResourceActionUpdate),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr("langfuse_dataset_item.generated", "expected_output"),
					resource.TestCheckNoResourceAttr("langfuse_dataset_item.generated", "metadata"),
					resource.TestCheckResourceAttr("langfuse_dataset_item.named", "status", "ACTIVE"),
				),
			},
		},
	})
}
