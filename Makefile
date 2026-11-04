.PHONY: test testacc test-setup test-teardown

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
test-setup:
	@echo "Starting Langfuse test environment..."
	docker compose -f testdata/docker-compose.yml up -d
	@echo "Waiting for Langfuse to be ready..."
	./scripts/wait-for-langfuse.sh localhost 3000 600 /

# Tear down test environment
test-teardown:
	@echo "Stopping Langfuse test environment..."
	docker compose -f testdata/docker-compose.yml down -v

# Generate mocks
generate:
	go generate ./...

# Regenerate API clients from the vendored OpenAPI specs and overlays in openapi/
generate-api:
	cd tools; go generate -run oapi-codegen ./...

# Regenerate API clients and fail if committed generated files are stale.
check-generated: generate-api
	@git diff HEAD --exit-code -- langfuse/adminapi/adminapi.gen.go langfuse/publicapi/publicapi.gen.go || \
		(echo "Generated clients are stale. Run 'make generate-api' and commit the result."; exit 1)

# Refresh the vendored OpenAPI specs from Langfuse Cloud
update-openapi:
	curl -fsSL https://cloud.langfuse.com/generated/api/openapi.yml -o openapi/langfuse.yml
	curl -fsSL https://cloud.langfuse.com/generated/organizations-api/openapi.yml -o openapi/langfuse-organizations.yml

docs:
	cd tools; go generate -skip oapi-codegen ./...

.PHONY: docs generate-api check-generated update-openapi
