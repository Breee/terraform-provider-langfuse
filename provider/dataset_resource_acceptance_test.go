package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

func TestAccLangfuseDataset(t *testing.T) {
	scope := testAccProjectScope(t)

	config := func(attrs string) string {
		return scope + `
resource "langfuse_dataset" "full" {
  project_public_key = local.public_key
  project_secret_key = local.secret_key
  name               = "acc/support faq"
` + attrs + `
}

resource "langfuse_dataset" "minimal" {
  project_public_key = local.public_key
  project_secret_key = local.secret_key
  name               = "acc-minimal"
}
`
	}
	initial := config(`
  description  = "initial"
  metadata     = jsonencode({ team = "support" })
  input_schema = jsonencode({ type = "object", properties = { question = { type = "string" } }, required = ["question"] })
`)
	updated := config(`
  description            = "updated"
  metadata               = jsonencode({ team = "platform", tier = 1 })
  expected_output_schema = jsonencode({ type = "string" })
`)
	cleared := config("")

	noop := []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}
	update := resource.ConfigPlanChecks{
		PreApply: []plancheck.PlanCheck{
			plancheck.ExpectResourceAction("langfuse_dataset.full", plancheck.ResourceActionUpdate),
			plancheck.ExpectResourceAction("langfuse_dataset.minimal", plancheck.ResourceActionNoop),
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
					resource.TestCheckResourceAttrSet("langfuse_dataset.full", "id"),
					resource.TestCheckResourceAttr("langfuse_dataset.full", "description", "initial"),
					resource.TestCheckNoResourceAttr("langfuse_dataset.full", "expected_output_schema"),
					resource.TestCheckNoResourceAttr("langfuse_dataset.minimal", "metadata"),
				),
			},
			{
				Config:           updated,
				ConfigPlanChecks: update,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("langfuse_dataset.full", "description", "updated"),
					resource.TestCheckNoResourceAttr("langfuse_dataset.full", "input_schema"),
					resource.TestCheckResourceAttrSet("langfuse_dataset.full", "expected_output_schema"),
				),
			},
			{
				ResourceName:            "langfuse_dataset.full",
				ImportState:             true,
				ImportStateIdFunc:       testAccCredentialImportID("langfuse_dataset.full", "name"),
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"metadata", "expected_output_schema"},
			},
			{
				ResourceName:      "langfuse_dataset.minimal",
				ImportState:       true,
				ImportStateIdFunc: testAccCredentialImportID("langfuse_dataset.minimal", "name"),
				ImportStateVerify: true,
			},
			{
				Config:           cleared,
				ConfigPlanChecks: update,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr("langfuse_dataset.full", "description"),
					resource.TestCheckNoResourceAttr("langfuse_dataset.full", "metadata"),
					resource.TestCheckNoResourceAttr("langfuse_dataset.full", "expected_output_schema"),
				),
			},
		},
	})
}
