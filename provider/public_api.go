package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/langfuse/terraform-provider-langfuse/langfuse"
	"github.com/langfuse/terraform-provider-langfuse/langfuse/publicapi"
)

const jsonContentType = "application/json"

// errNotFound is returned by apiResult.decode when the API answers 404.
var errNotFound = errors.New("not found")

// apiResult wraps the raw result of a generated client call so status handling
// does not depend on the success codes declared in the OpenAPI document.
type apiResult struct {
	resp *http.Response
	err  error
}

func apiCall(resp *http.Response, err error) apiResult {
	return apiResult{resp: resp, err: err}
}

// decode unmarshals a 2xx body into out (when out is non-nil) and turns every
// other status into an error carrying the status code and response body.
func (r apiResult) decode(out any) error {
	if r.err != nil {
		return r.err
	}
	defer func() { _ = r.resp.Body.Close() }()
	body, err := io.ReadAll(r.resp.Body)
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}
	if r.resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("%w: %s", errNotFound, string(body))
	}
	if r.resp.StatusCode < 200 || r.resp.StatusCode > 299 {
		return fmt.Errorf("request failed with status code %d, response body: %s", r.resp.StatusCode, string(body))
	}
	if out == nil || len(body) == 0 {
		return nil
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("decode response body: %w", err)
	}
	return nil
}

func jsonBody(v any) (io.Reader, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("encode request body: %w", err)
	}
	return bytes.NewReader(b), nil
}

// publicAPIResource is embedded by resources that call the public API with a
// project- or organization-scoped API key pair taken from their configuration.
type publicAPIResource struct {
	ClientFactory langfuse.ClientFactory
}

func (r *publicAPIResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	clientFactory, ok := req.ProviderData.(langfuse.ClientFactory)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected langfuse.ClientFactory, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}
	r.ClientFactory = clientFactory
}

func (r *publicAPIResource) client(publicKey, secretKey types.String) publicapi.ClientInterface {
	return r.ClientFactory.NewPublicAPIClient(publicKey.ValueString(), secretKey.ValueString())
}

// withProjectCredentials adds the project API key attributes shared by all project-scoped resources.
func withProjectCredentials(attrs map[string]schema.Attribute) map[string]schema.Attribute {
	attrs["project_public_key"] = schema.StringAttribute{
		Required:    true,
		Sensitive:   true,
		Description: "The project public key used to authenticate API calls.",
	}
	attrs["project_secret_key"] = schema.StringAttribute{
		Required:    true,
		Sensitive:   true,
		Description: "The project secret key used to authenticate API calls.",
	}
	return attrs
}

// parseCredentialImportID splits "<public_key>:<secret_key>:<identifier>".
// The identifier is returned verbatim and may itself contain colons.
func parseCredentialImportID(id string) (publicKey, secretKey, identifier string, err error) {
	parts := strings.SplitN(id, ":", 3)
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return "", "", "", errors.New("import ID must be in the format: <public_key>:<secret_key>:<identifier>")
	}
	return parts[0], parts[1], parts[2], nil
}

// jsonValue converts a decoded JSON value to a normalized JSON string attribute; nil becomes null.
func jsonValue(v any) (jsontypes.Normalized, error) {
	if v == nil {
		return jsontypes.NewNormalizedNull(), nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return jsontypes.NewNormalizedNull(), err
	}
	return jsontypes.NewNormalizedValue(string(b)), nil
}

// jsonAttr decodes a JSON string attribute; null and unknown decode to nil.
func jsonAttr(v jsontypes.Normalized) (any, error) {
	if v.IsNull() || v.IsUnknown() {
		return nil, nil
	}
	var out any
	if err := json.Unmarshal([]byte(v.ValueString()), &out); err != nil {
		return nil, err
	}
	return out, nil
}

func stringOrNull(v *string) types.String {
	if v == nil {
		return types.StringNull()
	}
	return types.StringValue(*v)
}

func float64OrNull(v *float64) types.Float64 {
	if v == nil {
		return types.Float64Null()
	}
	return types.Float64Value(*v)
}

// jsonSubset reports whether every value in want is present in got. Objects in
// got may carry extra keys (server-side defaults and generated identifiers);
// arrays must have the same length and order.
func jsonSubset(want, got any) bool {
	switch w := want.(type) {
	case map[string]any:
		g, ok := got.(map[string]any)
		if !ok {
			return false
		}
		for k, wv := range w {
			gv, exists := g[k]
			if !exists && wv != nil {
				return false
			}
			if !jsonSubset(wv, gv) {
				return false
			}
		}
		return true
	case []any:
		g, ok := got.([]any)
		if !ok || len(g) != len(w) {
			return false
		}
		for i := range w {
			if !jsonSubset(w[i], g[i]) {
				return false
			}
		}
		return true
	default:
		return want == got
	}
}

// keepJSON returns prior when the remote value still satisfies it, so that
// server-side defaults do not show up as drift, and the remote value otherwise.
func keepJSON(prior jsontypes.Normalized, remote any) (jsontypes.Normalized, error) {
	// Round-trip so typed values compare as plain decoded JSON.
	b, err := json.Marshal(remote)
	if err != nil {
		return jsontypes.NewNormalizedNull(), err
	}
	var plain any
	if err := json.Unmarshal(b, &plain); err != nil {
		return jsontypes.NewNormalizedNull(), err
	}
	if !prior.IsNull() && !prior.IsUnknown() {
		want, err := jsonAttr(prior)
		if err == nil && jsonSubset(want, plain) {
			return prior, nil
		}
	}
	return jsonValue(plain)
}
