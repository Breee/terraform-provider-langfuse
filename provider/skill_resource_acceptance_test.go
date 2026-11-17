package provider

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

func TestAccLangfuseSkill(t *testing.T) {
	scope := testAccProjectScope(t)

	config := func(skillName, description, attrs string) string {
		return scope + `
resource "langfuse_skill" "full" {
  project_public_key = local.public_key
  project_secret_key = local.secret_key
  name               = "acc-skill"
  files = {
    "SKILL.md"     = "---\nname: ` + skillName + `\ndescription: ` + description + `\n---\n# Skill\n"
    "scripts/a.sh" = "echo hi\n"
  }
` + attrs + `
}

resource "langfuse_skill" "minimal" {
  project_public_key = local.public_key
  project_secret_key = local.secret_key
  name               = "acc-minimal"
  files = {
    "SKILL.md" = "---\nname: acc-minimal\ndescription: Minimal\n---\n"
  }
}
`
	}
	initial := config("acc-skill", "Initial", `
  commit_message = "first"
  labels         = ["production", "staging"]
  tags           = ["docs"]
`)
	relabeled := config("acc-skill", "Initial", `
  commit_message = "first"
  labels         = ["staging"]
  tags           = ["docs", "extra"]
`)
	revised := config("acc-skill", "Revised", `
  labels = ["staging"]
`)

	noop := []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}
	update := resource.ConfigPlanChecks{
		PreApply: []plancheck.PlanCheck{
			plancheck.ExpectResourceAction("langfuse_skill.full", plancheck.ResourceActionUpdate),
			plancheck.ExpectResourceAction("langfuse_skill.minimal", plancheck.ResourceActionNoop),
		},
		PostApplyPostRefresh: noop,
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckOrganizationsDestroyed,
		Steps: []resource.TestStep{
			{
				Config:      config("another-skill", "Initial", ""),
				ExpectError: regexp.MustCompile("Skill name mismatch"),
			},
			{
				Config:           initial,
				ConfigPlanChecks: resource.ConfigPlanChecks{PostApplyPostRefresh: noop},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("langfuse_skill.full", "id", "acc-skill"),
					resource.TestCheckResourceAttr("langfuse_skill.full", "version", "1"),
					resource.TestCheckResourceAttr("langfuse_skill.full", "description", "Initial"),
					resource.TestCheckResourceAttr("langfuse_skill.full", "labels.#", "2"),
					resource.TestCheckResourceAttr("langfuse_skill.minimal", "version", "1"),
					resource.TestCheckNoResourceAttr("langfuse_skill.minimal", "labels"),
					resource.TestCheckNoResourceAttr("langfuse_skill.minimal", "tags"),
				),
			},
			{
				ResourceName:      "langfuse_skill.full",
				ImportState:       true,
				ImportStateIdFunc: testAccCredentialImportID("langfuse_skill.full", "id"),
				ImportStateVerify: true,
			},
			{
				// Labels and tags change in place without a new version.
				Config:           relabeled,
				ConfigPlanChecks: update,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("langfuse_skill.full", "version", "1"),
					resource.TestCheckResourceAttr("langfuse_skill.full", "labels.#", "1"),
					resource.TestCheckResourceAttr("langfuse_skill.full", "tags.#", "2"),
				),
			},
			{
				// Changed files create version 2, which takes over the label; tags are cleared.
				Config:           revised,
				ConfigPlanChecks: update,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("langfuse_skill.full", "version", "2"),
					resource.TestCheckResourceAttr("langfuse_skill.full", "description", "Revised"),
					resource.TestCheckResourceAttr("langfuse_skill.full", "labels.#", "1"),
					resource.TestCheckNoResourceAttr("langfuse_skill.full", "tags"),
					resource.TestCheckNoResourceAttr("langfuse_skill.full", "commit_message"),
				),
			},
			{
				ResourceName:      "langfuse_skill.minimal",
				ImportState:       true,
				ImportStateIdFunc: testAccCredentialImportID("langfuse_skill.minimal", "id"),
				ImportStateVerify: true,
			},
		},
	})
}
