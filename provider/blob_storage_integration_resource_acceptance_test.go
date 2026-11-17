package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccLangfuseBlobStorageIntegration(t *testing.T) {
	scope := testAccProjectScope(t)
	const name = "langfuse_blob_storage_integration.export"

	config := func(body string) string {
		return scope + `
resource "langfuse_blob_storage_integration" "export" {
  organization_public_key  = langfuse_organization_api_key.scope.public_key
  organization_private_key = langfuse_organization_api_key.scope.secret_key
  project_id               = langfuse_project.scope.id
` + body + `
}
`
	}
	// The export stays disabled so the worker does not start export jobs.
	initial := config(`
  type              = "S3_COMPATIBLE"
  bucket_name       = "langfuse"
  endpoint          = "http://rustfs:9000"
  region            = "us-east-1"
  access_key_id     = "langfuse-test"
  secret_access_key = "langfuse-test-secret"
  force_path_style  = true
  prefix            = "acc-export/"
  export_frequency  = "daily"
  export_mode       = "FULL_HISTORY"
  file_type         = "JSONL"
  enabled           = false
`)
	updated := config(`
  type                = "S3_COMPATIBLE"
  bucket_name         = "langfuse"
  endpoint            = "http://rustfs:9000"
  region              = "us-east-1"
  access_key_id       = "langfuse-test"
  secret_access_key   = "langfuse-test-secret"
  force_path_style    = true
  export_frequency    = "weekly"
  export_mode         = "FROM_CUSTOM_DATE"
  export_start_date   = "2026-01-01T00:00:00Z"
  file_type           = "CSV"
  compressed          = false
  export_source       = "OBSERVATIONS_V2"
  export_field_groups = ["core", "io"]
  enabled             = false
`)
	cleared := config(`
  type             = "S3"
  bucket_name      = "langfuse"
  region           = "us-east-1"
  export_frequency = "weekly"
  export_mode      = "FULL_HISTORY"
  file_type        = "CSV"
  enabled          = false
`)

	noop := []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}
	update := resource.ConfigPlanChecks{
		PreApply:             []plancheck.PlanCheck{plancheck.ExpectResourceAction(name, plancheck.ResourceActionUpdate)},
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
					resource.TestCheckResourceAttrPair(name, "id", "langfuse_project.scope", "id"),
					resource.TestCheckResourceAttr(name, "auth_type", "ACCESS_KEYS"),
					resource.TestCheckResourceAttr(name, "compressed", "true"),
					resource.TestCheckResourceAttrSet(name, "export_source"),
					resource.TestCheckResourceAttrSet(name, "export_field_groups.#"),
				),
			},
			{
				Config:           updated,
				ConfigPlanChecks: update,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(name, "prefix", ""),
					resource.TestCheckResourceAttr(name, "export_frequency", "weekly"),
					resource.TestCheckResourceAttr(name, "compressed", "false"),
					resource.TestCheckResourceAttr(name, "export_field_groups.#", "2"),
				),
			},
			{
				ResourceName: name,
				ImportState:  true,
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					rs, ok := s.RootModule().Resources[name]
					if !ok {
						return "", fmt.Errorf("resource %s not found", name)
					}
					attrs := rs.Primary.Attributes
					return attrs["organization_public_key"] + ":" + attrs["organization_private_key"] + ":" + attrs["id"], nil
				},
				ImportStateVerify: true,
				// The API never returns the secret; the date is normalised to milliseconds.
				ImportStateVerifyIgnore: []string{"secret_access_key", "export_start_date"},
			},
			{
				Config:           cleared,
				ConfigPlanChecks: update,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr(name, "endpoint"),
					resource.TestCheckNoResourceAttr(name, "access_key_id"),
					resource.TestCheckNoResourceAttr(name, "export_start_date"),
					resource.TestCheckResourceAttr(name, "force_path_style", "false"),
				),
			},
		},
	})
}
