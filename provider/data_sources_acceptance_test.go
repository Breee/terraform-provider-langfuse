package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

func TestAccLangfuseDataSources(t *testing.T) {
	config := testAccProjectScope(t) + `
resource "langfuse_organization_membership" "member" {
  email                    = "acc-lookup-${langfuse_organization.scope.id}@example.com"
  role                     = "MEMBER"
  organization_public_key  = langfuse_organization_api_key.scope.public_key
  organization_private_key = langfuse_organization_api_key.scope.secret_key
}

resource "langfuse_prompt" "greeting" {
  project_public_key = local.public_key
  project_secret_key = local.secret_key
  name               = "acc/greeting"
  prompt             = "Hello {{name}}"
  config             = jsonencode({ temperature = 0.2 })
  labels             = ["production"]
  tags               = ["acc"]
}

resource "langfuse_score_config" "quality" {
  project_public_key = local.public_key
  project_secret_key = local.secret_key
  name               = "acc-quality"
  data_type          = "NUMERIC"
  min_value          = 0
  max_value          = 1
}

resource "langfuse_dataset" "faq" {
  project_public_key = local.public_key
  project_secret_key = local.secret_key
  name               = "acc/faq"
  description        = "lookup"
}

data "langfuse_project" "current" {
  project_public_key = local.public_key
  project_secret_key = local.secret_key
}

data "langfuse_projects" "all" {
  organization_public_key  = langfuse_organization_api_key.scope.public_key
  organization_private_key = langfuse_organization_api_key.scope.secret_key
  depends_on               = [langfuse_project.scope]
}

data "langfuse_user" "member" {
  organization_public_key  = langfuse_organization_api_key.scope.public_key
  organization_private_key = langfuse_organization_api_key.scope.secret_key
  email                    = langfuse_organization_membership.member.email
}

data "langfuse_prompt" "production" {
  project_public_key = local.public_key
  project_secret_key = local.secret_key
  name               = langfuse_prompt.greeting.name
}

data "langfuse_prompt" "pinned" {
  project_public_key = local.public_key
  project_secret_key = local.secret_key
  name               = langfuse_prompt.greeting.name
  version            = langfuse_prompt.greeting.version
}

data "langfuse_score_config" "quality" {
  project_public_key = local.public_key
  project_secret_key = local.secret_key
  name               = langfuse_score_config.quality.name
}

data "langfuse_dataset" "faq" {
  project_public_key = local.public_key
  project_secret_key = local.secret_key
  name               = langfuse_dataset.faq.name
}
`

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckOrganizationsDestroyed,
		Steps: []resource.TestStep{
			{
				Config: config,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair("data.langfuse_project.current", "id", "langfuse_project.scope", "id"),
					resource.TestCheckResourceAttrPair("data.langfuse_project.current", "name", "langfuse_project.scope", "name"),
					resource.TestCheckResourceAttrPair("data.langfuse_project.current", "organization_id", "langfuse_organization.scope", "id"),
					resource.TestCheckResourceAttr("data.langfuse_projects.all", "projects.#", "1"),
					resource.TestCheckResourceAttrPair("data.langfuse_projects.all", "projects.0.id", "langfuse_project.scope", "id"),
					resource.TestCheckResourceAttrPair("data.langfuse_user.member", "id", "langfuse_organization_membership.member", "user_id"),
					resource.TestCheckResourceAttr("data.langfuse_prompt.production", "prompt", "Hello {{name}}"),
					resource.TestCheckResourceAttr("data.langfuse_prompt.production", "type", "text"),
					resource.TestCheckResourceAttrPair("data.langfuse_prompt.production", "version", "langfuse_prompt.greeting", "version"),
					resource.TestCheckTypeSetElemAttr("data.langfuse_prompt.production", "labels.*", "production"),
					resource.TestCheckTypeSetElemAttr("data.langfuse_prompt.production", "tags.*", "acc"),
					resource.TestCheckResourceAttr("data.langfuse_prompt.pinned", "prompt", "Hello {{name}}"),
					resource.TestCheckResourceAttrPair("data.langfuse_score_config.quality", "id", "langfuse_score_config.quality", "id"),
					resource.TestCheckResourceAttr("data.langfuse_score_config.quality", "data_type", "NUMERIC"),
					resource.TestCheckResourceAttr("data.langfuse_score_config.quality", "max_value", "1"),
					resource.TestCheckResourceAttrPair("data.langfuse_dataset.faq", "id", "langfuse_dataset.faq", "id"),
					resource.TestCheckResourceAttr("data.langfuse_dataset.faq", "description", "lookup"),
				),
			},
		},
	})
}
