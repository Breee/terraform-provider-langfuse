//go:build generate

package tools

import (
	_ "github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen"
)

// Generate the Langfuse API clients from the vendored specs in ../openapi.
//go:generate go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen -config ../langfuse/publicapi/oapi-codegen.yaml ../openapi/langfuse.yml
//go:generate go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen -config ../langfuse/adminapi/oapi-codegen.yaml ../openapi/langfuse-organizations.yml
