# Terraform Provider for Langfuse

Manage Langfuse organizations, projects, API keys, memberships, and LLM
connections with Terraform or OpenTofu.

This repository maintains an independent Terraform provider fork. API clients
are generated from Langfuse OpenAPI specifications; Terraform schemas and
lifecycle behavior are maintained here.

## Requirements

- Terraform >= 1.5, or a compatible OpenTofu CLI.
- Go >= 1.27.3 for development.
- A Langfuse instance with the API permissions required by your resources.
- Langfuse Enterprise and an admin API key for organization management.

## Installation

**Registry publication is pending.** The historical `langfuse/langfuse`
registry address points to upstream, not this fork. Namespace selection and
Terraform/OpenTofu registration are tracked in
[#4](https://github.com/Breee/terraform-provider-langfuse/issues/4).

For local development:

```sh
git clone https://github.com/Breee/terraform-provider-langfuse
cd terraform-provider-langfuse
mkdir -p .work
go build -o .work/terraform-provider-langfuse
```

Use a development override in a dedicated Terraform CLI configuration file:

```hcl
provider_installation {
  dev_overrides {
    "langfuse/langfuse" = "/absolute/path/to/terraform-provider-langfuse/.work"
  }
  direct {}
}
```

Set `TF_CLI_CONFIG_FILE` to that file's absolute path. The address above is
only a local override; it does not install the upstream binary. Do not run
`terraform init` merely to install this development provider.

## Configuration

```hcl
terraform {
  required_providers {
    langfuse = {
      source = "langfuse/langfuse" # Local development override, not a registry install.
    }
  }
}

provider "langfuse" {
  host = "https://cloud.langfuse.com"
  # Organization management can read LANGFUSE_ADMIN_KEY from the environment.
}
```

Organization-scoped credentials are supplied on project and membership
resources. Project-scoped credentials are supplied on LLM connections.
`host` does not read `LANGFUSE_HOST`; configure it explicitly.

Sensitive values remain in Terraform state. Use an encrypted,
access-controlled state backend and never commit credentials or state.
An Enterprise license is configured on the Langfuse server, not the provider.

## Generated reference

The [provider overview](docs/index.md) covers authentication and state security.
Each resource page contains a configuration example, generated schema,
lifecycle limitations, and import guidance:

| Resource | Reference |
| --- | --- |
| Organizations | [Organization](docs/resources/organization.md) |
| Organization API keys | [Organization API key](docs/resources/organization_api_key.md) |
| Organization memberships | [Organization membership](docs/resources/organization_membership.md) |
| Projects | [Project](docs/resources/project.md) |
| Project API keys | [Project API key](docs/resources/project_api_key.md) |
| Project memberships | [Project membership](docs/resources/project_membership.md) |
| LLM connections | [LLM connection](docs/resources/llm_connection.md) |

Reference pages are generated from schema descriptions, templates, and examples;
do not edit `docs/` by hand.

## Development

```sh
make build
make test
make generate        # Refresh both API specs and regenerate every output.
make generate-local  # Regenerate from vendored specs without refreshing them.
make validate-docs
```

See [CONTRIBUTING.md](CONTRIBUTING.md) for the full development, generation,
release, and Renovate workflow; [TESTING.md](TESTING.md) for isolated acceptance
tests; [AGENTS.md](AGENTS.md) and [llms.txt](llms.txt) for agent entry points.

Weekly signed releases require repository signing secrets. Real-server
acceptance verification requires an Enterprise test license.
Remaining work and human setup are tracked in
[GitHub issues](https://github.com/Breee/terraform-provider-langfuse/issues).
