.PHONY: build install test testacc test-all test-prerequisites test-setup test-teardown generate generate-local generate-go generate-api check-generated update-openapi docs validate-docs

build:
	go build -v ./...

install: build
	go install -v ./...

# Run unit tests (fast, no external dependencies)
test:
	go test ./... -v -run "^Test[^A].*"

# Run acceptance tests (requires docker)
testacc: test-setup
	TF_ACC=1 LANGFUSE_HOST=http://localhost:3000 LANGFUSE_ADMIN_KEY=test_admin_key go test ./provider -v -run TestAcc

# Run all tests (unit + acceptance)
test-all: test testacc

# Set up test environment
test-prerequisites:
	@if [ -z "$$LANGFUSE_EE_LICENSE_KEY" ]; then \
		echo "Acceptance tests require LANGFUSE_EE_LICENSE_KEY. Export the Enterprise test license before running 'make testacc'." >&2; \
		exit 1; \
	fi
	docker compose version
	docker info > /dev/null

test-setup: test-prerequisites
	@echo "Starting Langfuse test environment..."
	docker compose -f testdata/docker-compose.yml up -d
	@echo "Waiting for Langfuse to be ready..."
	./scripts/wait-for-langfuse.sh localhost 3000 600 /

# Tear down test environment
test-teardown:
	@echo "Stopping Langfuse test environment..."
	docker compose -f testdata/docker-compose.yml down -v

# Refresh API inputs, then regenerate everything, including under make -j.
generate:
	$(MAKE) update-openapi
	$(MAKE) generate-local

# Reproduce all outputs without refreshing the vendored specifications.
generate-local:
	$(MAKE) generate-go
	$(MAKE) generate-api
	$(MAKE) docs

generate-go:
	go generate ./...

# Regenerate API clients from the vendored OpenAPI specs and overlays in openapi/
generate-api:
	cd tools && go generate -run oapi-codegen ./...

# Regenerate every checked-in generated output and fail if any are stale.
check-generated: generate-local
	@git diff --exit-code -- langfuse/mocks langfuse/adminapi langfuse/publicapi docs examples || \
		(echo "Generated files or examples are stale. Run 'make generate-local' and commit the result."; exit 1)
	@test -z "$$(git ls-files --others --exclude-standard -- langfuse/mocks langfuse/adminapi langfuse/publicapi docs examples)" || \
		(echo "Generation created untracked output. Review it and commit the result."; exit 1)

# Refresh the vendored OpenAPI specs from Langfuse Cloud
update-openapi:
	@set -eu; \
	tmpdir=$$(mktemp -d openapi/.download.XXXXXX); \
	trap 'rm -f "$$tmpdir/langfuse.yml" "$$tmpdir/langfuse-organizations.yml"; rmdir "$$tmpdir"' EXIT; \
	curl --fail --silent --show-error --location --retry 3 --connect-timeout 15 --max-time 120 \
		https://cloud.langfuse.com/generated/api/openapi.yml -o "$$tmpdir/langfuse.yml"; \
	curl --fail --silent --show-error --location --retry 3 --connect-timeout 15 --max-time 120 \
		https://cloud.langfuse.com/generated/organizations-api/openapi.yml -o "$$tmpdir/langfuse-organizations.yml"; \
	if ! test -s "$$tmpdir/langfuse.yml" || ! test -s "$$tmpdir/langfuse-organizations.yml"; then \
		echo "An OpenAPI download was empty; vendored specifications were not changed." >&2; exit 1; \
	fi; \
	mv "$$tmpdir/langfuse.yml" openapi/langfuse.yml; \
	mv "$$tmpdir/langfuse-organizations.yml" openapi/langfuse-organizations.yml

docs:
	cd tools && go generate -skip oapi-codegen ./...

validate-docs:
	cd tools && go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs validate --provider-dir .. --provider-name langfuse
