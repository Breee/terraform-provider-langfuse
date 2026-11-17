package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccLangfuseOrganizationLlmConnection(t *testing.T) {
	scope := testAccProjectScope(t)
	const openai = "langfuse_organization_llm_connection.openai"
	const bedrock = "langfuse_organization_llm_connection.bedrock"

	config := func(openaiAttrs, bedrockRegion string) string {
		return scope + `
resource "langfuse_organization_llm_connection" "openai" {
  organization_public_key  = langfuse_organization_api_key.scope.public_key
  organization_private_key = langfuse_organization_api_key.scope.secret_key
  provider_name            = "acc-openai"
  adapter                  = "openai"
` + openaiAttrs + `
}

resource "langfuse_organization_llm_connection" "bedrock" {
  organization_public_key  = langfuse_organization_api_key.scope.public_key
  organization_private_key = langfuse_organization_api_key.scope.secret_key
  provider_name            = "acc-bedrock"
  adapter                  = "bedrock"
  secret_key               = jsonencode({ accessKeyId = "acc", secretAccessKey = "acc-secret" })
  config                   = jsonencode({ region = "` + bedrockRegion + `" })
}
`
	}
	initial := config(`
  secret_key          = "sk-acc-initial"
  custom_models       = ["acc-model-a", "acc-model-b"]
  extra_headers       = { "X-Acc" = "one" }
  with_default_models = false
`, "us-east-1")
	updated := config(`
  secret_key = "sk-acc-rotated"
`, "eu-central-1")

	importID := func(name string) resource.ImportStateIdFunc {
		return func(s *terraform.State) (string, error) {
			rs, ok := s.RootModule().Resources[name]
			if !ok {
				return "", fmt.Errorf("resource %s not found", name)
			}
			attrs := rs.Primary.Attributes
			return attrs["organization_public_key"] + ":" + attrs["organization_private_key"] + ":" + attrs["id"], nil
		}
	}
	noop := []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckOrganizationsDestroyed,
		Steps: []resource.TestStep{
			{
				Config:           initial,
				ConfigPlanChecks: resource.ConfigPlanChecks{PostApplyPostRefresh: noop},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(openai, "id"),
					resource.TestCheckResourceAttr(openai, "custom_models.#", "2"),
					resource.TestCheckResourceAttr(openai, "with_default_models", "false"),
					resource.TestCheckNoResourceAttr(openai, "config"),
					resource.TestCheckResourceAttr(bedrock, "with_default_models", "true"),
				),
			},
			{
				ResourceName:      openai,
				ImportState:       true,
				ImportStateIdFunc: importID(openai),
				ImportStateVerify: true,
				// The API never returns the secret or the header values.
				ImportStateVerifyIgnore: []string{"secret_key", "extra_headers"},
			},
			{
				ResourceName:      bedrock,
				ImportState:       true,
				ImportStateIdFunc: importID(bedrock),
				ImportStateVerify: true,
				// config is compared textually and the API returns it compact.
				ImportStateVerifyIgnore: []string{"secret_key", "config"},
			},
			{
				Config: updated,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(openai, plancheck.ResourceActionUpdate),
						plancheck.ExpectResourceAction(bedrock, plancheck.ResourceActionUpdate),
					},
					PostApplyPostRefresh: noop,
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr(openai, "custom_models"),
					resource.TestCheckNoResourceAttr(openai, "extra_headers"),
					resource.TestCheckResourceAttr(openai, "with_default_models", "true"),
					resource.TestCheckResourceAttr(bedrock, "config", `{"region":"eu-central-1"}`),
				),
			},
		},
	})
}
