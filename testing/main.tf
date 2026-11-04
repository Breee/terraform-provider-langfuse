terraform {
  required_providers {
    langfuse = {
      source = "langfuse/langfuse"
    }
  }
}

variable "langfuse_host" {
  type = string
}

variable "litellm_base_url" {
  type = string
}

variable "litellm_key" {
  type      = string
  sensitive = true
}

variable "member_email" {
  type    = string
  default = "admin@localhost.localdomain"
}

# admin_api_key comes from LANGFUSE_ADMIN_KEY.
provider "langfuse" {
  host = var.langfuse_host
}

variable "name_suffix" {
  type = string
}

locals {
  name = "tf-testing-${var.name_suffix}"
}

resource "langfuse_organization" "this" {
  name = local.name
  metadata = {
    purpose = "terraform-provider-langfuse testing"
  }
}

resource "langfuse_organization_api_key" "this" {
  organization_id = langfuse_organization.this.id
}

resource "langfuse_organization_membership" "this" {
  email                    = var.member_email
  role                     = "MEMBER"
  organization_public_key  = langfuse_organization_api_key.this.public_key
  organization_private_key = langfuse_organization_api_key.this.secret_key
}

resource "langfuse_project" "this" {
  name                     = local.name
  retention_days           = 30
  organization_id          = langfuse_organization.this.id
  organization_public_key  = langfuse_organization_api_key.this.public_key
  organization_private_key = langfuse_organization_api_key.this.secret_key
  metadata = {
    purpose = "terraform-provider-langfuse testing"
  }
}

resource "langfuse_project_api_key" "this" {
  project_id               = langfuse_project.this.id
  organization_public_key  = langfuse_organization_api_key.this.public_key
  organization_private_key = langfuse_organization_api_key.this.secret_key
  note                     = "testing"
}

resource "langfuse_project_membership" "this" {
  project_id               = langfuse_project.this.id
  email                    = var.member_email
  role                     = "VIEWER"
  organization_public_key  = langfuse_organization_api_key.this.public_key
  organization_private_key = langfuse_organization_api_key.this.secret_key

  depends_on = [langfuse_organization_membership.this]
}

resource "langfuse_llm_connection" "litellm" {
  project_public_key  = langfuse_project_api_key.this.public_key
  project_secret_key  = langfuse_project_api_key.this.secret_key
  provider_name       = "litellm"
  adapter             = "openai"
  secret_key          = var.litellm_key
  base_url            = "${var.litellm_base_url}/v1"
  custom_models       = ["bedrock/global.openai.gpt-5.5"]
  with_default_models = false
}

resource "langfuse_llm_connection" "plain" {
  project_public_key  = langfuse_project_api_key.this.public_key
  project_secret_key  = langfuse_project_api_key.this.secret_key
  provider_name       = "plain-openai"
  adapter             = "openai"
  secret_key          = "sk-dummy"
  with_default_models = true
}

output "organization_id" {
  value = langfuse_organization.this.id
}

output "project_id" {
  value = langfuse_project.this.id
}

output "organization_membership_status" {
  value = langfuse_organization_membership.this.status
}

output "project_membership_user_id" {
  value = langfuse_project_membership.this.user_id
}

output "llm_connection_id" {
  value = langfuse_llm_connection.litellm.id
}
