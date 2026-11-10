# Testing Guide

This document describes how to run tests for the Terraform Langfuse Provider.

## Unit Tests

Unit tests use mocks and don't require external dependencies. They test individual resource logic in isolation:

```bash
make test
# or
go test ./... -v -run "^Test[^A].*"
```

Unit tests are located in `*_unit_test.go` files and include:
- Metadata validation
- Schema validation  
- CRUD operations with mocked dependencies
- Fast execution (< 1 second per test)

## Acceptance Tests

Acceptance tests run against a real Langfuse instance using Docker Compose.

### Test Strategy

The current acceptance suite exercises four resource types in dependency order:

1. **Organization** (foundational resource)
2. **Organization API Key** (requires organization)  
3. **Project** (requires organization API key)
4. **Project API Key** (requires project and organization API key)

It also tests organization and project imports, updates, and project API-key
replacement. Organization membership, project membership, and LLM connections
are not covered by the current acceptance suite. The destroy check verifies
that test organizations disappear from the API, but does not independently
verify deletion of their child resources.

### E2E coverage design

The target is a real-server lifecycle for every resource, not just a successful
Terraform apply:

- Create each of the seven resources, read it back through Langfuse's API, and
  verify the Terraform state and an unchanged follow-up plan.
- Update supported fields and verify the remote values; assert replacement for
  immutable fields and preserve write-only credentials across refreshes.
- Import each resource that supports import, refresh it, and verify subsequent
  management with the documented import format.
- Delete resources in dependency order and independently verify their absence
  through the appropriate API. Poll boundedly for asynchronous project deletion;
  treat authentication, transport and unexpected API errors as failures.
- Exercise out-of-band deletion or changes and invalid credentials without
  touching any non-test instance. Use unique resource names and retain useful
  diagnostics on failure.
- Run the suite against a pinned disposable stack with isolated credentials;
  require a dedicated CI license secret before enabling it in automation.

### Prerequisites

- Docker and Docker Compose
- **Langfuse Enterprise License Key** - Required for admin API access
  - Set the `LANGFUSE_EE_LICENSE_KEY` environment variable
  - Never commit or print the key

### Running Acceptance Tests

First, set your enterprise license key:

```bash
export LANGFUSE_EE_LICENSE_KEY="your_actual_license_key_here"
```

Then run the tests using the Makefile:

```bash
make testacc
```

The command checks for the Enterprise license and a working Docker daemon
before starting containers. It never loads credentials from `testing/.env` or
targets the live instance used by local example configurations. Acceptance
tests create and delete resources; only use an isolated test environment.
The default Compose project is `terraform-provider-langfuse-acceptance`; set
`COMPOSE_PROJECT_NAME` to isolate concurrent runs. `make test-teardown` removes
that project's containers and volumes, so use the same project name for setup
and teardown. Runs also need exclusive use of loopback port 3000.

This will:
1. Start the Langfuse test environment using Docker Compose
2. Wait for the web and worker services to be healthy
3. Run the acceptance tests
4. Leave the environment running (use `make test-teardown` to clean up)

To run both unit and acceptance tests:

```bash
make test-all
```

### Manual Steps

If you prefer to run steps manually:

```bash
# 1. Start the test environment
make test-setup

# 2. Run acceptance tests
TF_ACC=1 LANGFUSE_HOST=http://localhost:3000 LANGFUSE_ADMIN_KEY=test_admin_key \
  go test ./provider -v -run TestAcc

# 3. Clean up
make test-teardown
```

### Environment Variables

The acceptance tests require these environment variables:

- `TF_ACC=1` - Enables acceptance testing
- `LANGFUSE_HOST` - Base URL of the Langfuse instance (default: http://localhost:3000)
- `LANGFUSE_ADMIN_KEY` - Admin API key for authentication

### Test Infrastructure

The test setup uses:

- **PostgreSQL 15** - Primary database
- **ClickHouse** - Analytics database, pinned to the version used by Langfuse's official Compose stack
- **Redis 7** - Caching layer
- **RustFS** - S3-compatible object storage
- **Langfuse web and worker** - Main application and background processing

RustFS has a readiness check; an initializer creates the S3 bucket before
Langfuse starts. The web service binds to loopback only. The disposable worker
uses a shorter project-deletion rate limit so repeated lifecycle tests finish
without waiting for Langfuse's production default.

## Test Structure

### Unit Tests
- Located in `provider/*_unit_test.go` files
- Use gomock for mocking dependencies  
- Fast execution, no external dependencies
- Test individual resource logic in isolation

### Acceptance Tests  
- Located in `provider/provider_acceptance_test.go`
- Use the `terraform-plugin-testing` framework
- Test against real Terraform configurations and live Langfuse instance
- Verify actual resource lifecycle (Create, Read, Update, Delete, Import)

### Test Utilities
- `testdata/docker-compose.yml` - Test environment definition

## Troubleshooting

### Tests fail with "connection refused"
Check whether the Langfuse web service is healthy:

```bash
curl 'http://localhost:3000/api/public/health?failIfDatabaseUnavailable=true'
```

### Docker containers won't start
Check Docker logs:

```bash
docker compose -p "${COMPOSE_PROJECT_NAME:-terraform-provider-langfuse-acceptance}" -f testdata/docker-compose.yml logs
```

### Port conflicts
If port 3000 is already in use, you can modify the docker-compose.yml to use a different port and update the `LANGFUSE_HOST` environment variable accordingly.
