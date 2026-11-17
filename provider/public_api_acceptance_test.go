package provider

import (
	"fmt"
	"math/rand"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// testAccProjectScope skips unless acceptance tests are enabled and returns a
// configuration prefix that creates an organization, a project, and a project
// API key exposed as local.public_key and local.secret_key.
func testAccProjectScope(t *testing.T) string {
	t.Helper()
	if os.Getenv("TF_ACC") != "1" {
		t.Skip("TF_ACC not set - skipping acceptance test")
	}
	testAccPreCheck(t)
	suffix := rand.Intn(1000000)

	return fmt.Sprintf(`
provider "langfuse" {
  host          = "%s"
  admin_api_key = "%s"
}

resource "langfuse_organization" "scope" {
  name = "acc-org-%d"
}

resource "langfuse_organization_api_key" "scope" {
  organization_id = langfuse_organization.scope.id
}

resource "langfuse_project" "scope" {
  name                     = "acc-project-%d"
  organization_id          = langfuse_organization.scope.id
  organization_public_key  = langfuse_organization_api_key.scope.public_key
  organization_private_key = langfuse_organization_api_key.scope.secret_key
}

resource "langfuse_project_api_key" "scope" {
  project_id               = langfuse_project.scope.id
  organization_public_key  = langfuse_organization_api_key.scope.public_key
  organization_private_key = langfuse_organization_api_key.scope.secret_key
}

locals {
  public_key = langfuse_project_api_key.scope.public_key
  secret_key = langfuse_project_api_key.scope.secret_key
}
`, os.Getenv("LANGFUSE_HOST"), os.Getenv("LANGFUSE_ADMIN_KEY"), suffix, suffix)
}

// testAccCredentialImportID builds "<public_key>:<secret_key>:<attribute>" for a resource in state.
func testAccCredentialImportID(resourceName, attribute string) func(*terraform.State) (string, error) {
	return func(s *terraform.State) (string, error) {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return "", fmt.Errorf("resource %s not found in state", resourceName)
		}
		a := rs.Primary.Attributes
		return a["project_public_key"] + ":" + a["project_secret_key"] + ":" + a[attribute], nil
	}
}
