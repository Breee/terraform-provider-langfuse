package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

func TestAccLangfusePrompt(t *testing.T) {
	scope := testAccProjectScope(t)

	config := func(text, labels string) string {
		return scope + `
resource "langfuse_prompt" "text" {
  project_public_key = local.public_key
  project_secret_key = local.secret_key
  name               = "acc/text"
  prompt             = "` + text + `"
  labels             = ` + labels + `
  tags               = ["acc"]
  commit_message     = "from terraform"
  config             = jsonencode({ model = "gpt-4o", temperature = 0.2 })
}

resource "langfuse_prompt" "chat" {
  project_public_key = local.public_key
  project_secret_key = local.secret_key
  name               = "acc-chat"
  type               = "chat"
  prompt = jsonencode([
    { role = "system", content = "You are {{persona}}." },
    { type = "placeholder", name = "history" },
  ])
}
`
	}
	noop := []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}
	updateText := resource.ConfigPlanChecks{
		PreApply: []plancheck.PlanCheck{
			plancheck.ExpectResourceAction("langfuse_prompt.text", plancheck.ResourceActionUpdate),
			plancheck.ExpectResourceAction("langfuse_prompt.chat", plancheck.ResourceActionNoop),
		},
		PostApplyPostRefresh: noop,
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckOrganizationsDestroyed,
		Steps: []resource.TestStep{
			{
				Config:           config("Hello {{name}}", `["production"]`),
				ConfigPlanChecks: resource.ConfigPlanChecks{PostApplyPostRefresh: noop},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("langfuse_prompt.text", "id", "acc/text"),
					resource.TestCheckResourceAttr("langfuse_prompt.text", "version", "1"),
					resource.TestCheckResourceAttr("langfuse_prompt.text", "type", "text"),
					resource.TestCheckResourceAttr("langfuse_prompt.chat", "version", "1"),
					resource.TestCheckNoResourceAttr("langfuse_prompt.chat", "labels"),
					resource.TestCheckNoResourceAttr("langfuse_prompt.chat", "config"),
				),
			},
			{
				// Adding a label keeps the version.
				Config:           config("Hello {{name}}", `["production", "staging"]`),
				ConfigPlanChecks: updateText,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("langfuse_prompt.text", "version", "1"),
					resource.TestCheckResourceAttr("langfuse_prompt.text", "labels.#", "2"),
				),
			},
			{
				// Changing the content creates a new version that takes over the labels.
				Config:           config("Hi {{name}}", `["production", "staging"]`),
				ConfigPlanChecks: updateText,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("langfuse_prompt.text", "version", "2"),
					resource.TestCheckResourceAttr("langfuse_prompt.text", "prompt", "Hi {{name}}"),
					resource.TestCheckResourceAttr("langfuse_prompt.text", "labels.#", "2"),
				),
			},
			{
				// Labels cannot be removed from a version, so this creates a new one.
				Config:           config("Hi {{name}}", `["production"]`),
				ConfigPlanChecks: updateText,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("langfuse_prompt.text", "version", "3"),
					resource.TestCheckResourceAttr("langfuse_prompt.text", "labels.#", "1"),
				),
			},
			{
				ResourceName:            "langfuse_prompt.text",
				ImportState:             true,
				ImportStateIdFunc:       testAccCredentialImportID("langfuse_prompt.text", "id"),
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"config"},
			},
			{
				ResourceName:            "langfuse_prompt.chat",
				ImportState:             true,
				ImportStateIdFunc:       testAccCredentialImportID("langfuse_prompt.chat", "id"),
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"prompt"},
			},
		},
	})
}
