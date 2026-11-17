package provider

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

func TestAccLangfuseAnnotationQueue(t *testing.T) {
	scope := testAccProjectScope(t) + `
resource "langfuse_score_config" "quality" {
  project_public_key = local.public_key
  project_secret_key = local.secret_key
  name               = "acc-quality"
  data_type          = "NUMERIC"
}

resource "langfuse_score_config" "correct" {
  project_public_key = local.public_key
  project_secret_key = local.secret_key
  name               = "acc-correct"
  data_type          = "BOOLEAN"
}

resource "langfuse_organization_membership" "annotator" {
  email                    = "acc-annotator-${langfuse_organization.scope.id}@example.com"
  role                     = "MEMBER"
  organization_public_key  = langfuse_organization_api_key.scope.public_key
  organization_private_key = langfuse_organization_api_key.scope.secret_key
}

resource "langfuse_annotation_queue_assignment" "annotator" {
  project_public_key = local.public_key
  project_secret_key = local.secret_key
  queue_id           = langfuse_annotation_queue.review.id
  user_id            = langfuse_organization_membership.annotator.user_id
}
`
	config := func(attrs string) string {
		return scope + `
resource "langfuse_annotation_queue" "review" {
  project_public_key = local.public_key
  project_secret_key = local.secret_key
` + attrs + `
}
`
	}
	initial := config(`
  name             = "acc-review"
  description      = "initial"
  score_config_ids = [langfuse_score_config.quality.id, langfuse_score_config.correct.id]
`)
	sameName := config(`
  name             = "acc-review"
  description      = "changed"
  score_config_ids = [langfuse_score_config.quality.id, langfuse_score_config.correct.id]
`)
	renamed := config(`
  name             = "acc-review-v2"
  score_config_ids = [langfuse_score_config.quality.id]
`)

	noop := []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckOrganizationsDestroyed,
		Steps: []resource.TestStep{
			{
				Config:           initial,
				ConfigPlanChecks: resource.ConfigPlanChecks{PostApplyPostRefresh: noop},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("langfuse_annotation_queue.review", "id"),
					resource.TestCheckResourceAttr("langfuse_annotation_queue.review", "score_config_ids.#", "2"),
					resource.TestCheckResourceAttrSet("langfuse_annotation_queue_assignment.annotator", "id"),
				),
			},
			{
				ResourceName:      "langfuse_annotation_queue.review",
				ImportState:       true,
				ImportStateIdFunc: testAccCredentialImportID("langfuse_annotation_queue.review", "id"),
				ImportStateVerify: true,
			},
			{
				// The previous queue cannot be deleted, so a change without a rename is rejected at plan time.
				Config:      sameName,
				ExpectError: regexp.MustCompile("Annotation queue cannot be changed in place"),
			},
			{
				Config: renamed,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("langfuse_annotation_queue.review", plancheck.ResourceActionDestroyBeforeCreate),
						plancheck.ExpectResourceAction("langfuse_annotation_queue_assignment.annotator", plancheck.ResourceActionDestroyBeforeCreate),
					},
					PostApplyPostRefresh: noop,
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("langfuse_annotation_queue.review", "name", "acc-review-v2"),
					resource.TestCheckNoResourceAttr("langfuse_annotation_queue.review", "description"),
					resource.TestCheckResourceAttr("langfuse_annotation_queue.review", "score_config_ids.#", "1"),
				),
			},
		},
	})
}
