package provider

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/langfuse/terraform-provider-langfuse/langfuse"
)

// fakeAPI is a scripted Langfuse API: handlers are keyed by "METHOD /path".
type fakeAPI struct {
	t        *testing.T
	server   *httptest.Server
	handlers map[string]func(body map[string]any) (int, any)
	calls    []string
	bodies   map[string]map[string]any
}

func newFakeAPI(t *testing.T) *fakeAPI {
	t.Helper()
	f := &fakeAPI{t: t, handlers: map[string]func(map[string]any) (int, any){}, bodies: map[string]map[string]any{}}
	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Method + " " + r.URL.Path
		if r.URL.RawQuery != "" && f.handlers[key+"?"+r.URL.RawQuery] != nil {
			key += "?" + r.URL.RawQuery
		}
		f.calls = append(f.calls, key)
		if user, pass, ok := r.BasicAuth(); !ok || user != "pk" || pass != "sk" {
			t.Errorf("%s: unexpected credentials", key)
		}
		var body map[string]any
		raw, _ := io.ReadAll(r.Body)
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &body); err != nil {
				t.Errorf("%s: invalid JSON body: %v", key, err)
			}
		}
		f.bodies[key] = body
		handler, ok := f.handlers[key]
		if !ok {
			t.Errorf("unexpected request: %s", key)
			w.WriteHeader(http.StatusNotImplemented)
			return
		}
		status, response := handler(body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(response)
	}))
	t.Cleanup(f.server.Close)
	return f
}

func (f *fakeAPI) on(key string, status int, response any) {
	f.handlers[key] = func(map[string]any) (int, any) { return status, response }
}

func (f *fakeAPI) factory() langfuse.ClientFactory {
	return langfuse.NewClientFactory(f.server.URL, "")
}

func testResourceSchema(t *testing.T, r resource.Resource) tfsdk.State {
	t.Helper()
	ctx := context.Background()
	var resp resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("schema: %v", resp.Diagnostics)
	}
	if diags := resp.Schema.ValidateImplementation(ctx); diags.HasError() {
		t.Fatalf("schema implementation: %v", diags)
	}
	return tfsdk.State{Schema: resp.Schema, Raw: tftypes.NewValue(resp.Schema.Type().TerraformType(ctx), nil)}
}

// testState builds resource state from a model struct; a nil model yields empty state.
func testState(t *testing.T, r resource.Resource, model any) tfsdk.State {
	t.Helper()
	state := testResourceSchema(t, r)
	if model != nil {
		if diags := state.Set(context.Background(), model); diags.HasError() {
			t.Fatalf("set state: %v", diags)
		}
	}
	return state
}

func testPlan(t *testing.T, r resource.Resource, model any) tfsdk.Plan {
	t.Helper()
	state := testState(t, r, model)
	return tfsdk.Plan(state)
}

func TestAPIResultDecode(t *testing.T) {
	response := func(status int, body string) *http.Response {
		return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body))}
	}
	var out struct {
		ID string `json:"id"`
	}
	if err := apiCall(response(201, `{"id":"a"}`), nil).decode(&out); err != nil || out.ID != "a" {
		t.Fatalf("201 should decode: %v %+v", err, out)
	}
	if err := apiCall(response(204, ``), nil).decode(&out); err != nil {
		t.Fatalf("empty body should succeed: %v", err)
	}
	if err := apiCall(response(404, `gone`), nil).decode(&out); !errors.Is(err, errNotFound) {
		t.Fatalf("404 should be errNotFound, got %v", err)
	}
	if err := apiCall(response(400, `bad input`), nil).decode(&out); err == nil || !strings.Contains(err.Error(), "400") || !strings.Contains(err.Error(), "bad input") {
		t.Fatalf("400 should carry status and body, got %v", err)
	}
	if err := apiCall(nil, errors.New("boom")).decode(&out); err == nil || err.Error() != "boom" {
		t.Fatalf("transport error should pass through, got %v", err)
	}
}

func TestParseCredentialImportID(t *testing.T) {
	pk, sk, id, err := parseCredentialImportID("pk:sk:name:with:colons")
	if err != nil || pk != "pk" || sk != "sk" || id != "name:with:colons" {
		t.Fatalf("unexpected result: %q %q %q %v", pk, sk, id, err)
	}
	for _, bad := range []string{"", "pk", "pk:sk", ":sk:id", "pk::id", "pk:sk:"} {
		if _, _, _, err := parseCredentialImportID(bad); err == nil {
			t.Errorf("%q should be rejected", bad)
		}
	}
}

func TestKeepJSON(t *testing.T) {
	decode := func(s string) any {
		var v any
		if err := json.Unmarshal([]byte(s), &v); err != nil {
			t.Fatal(err)
		}
		return v
	}
	prior := jsontypes.NewNormalizedValue(`[{"name":"a","prices":{"input":1}}]`)

	kept, err := keepJSON(prior, decode(`[{"id":"x","name":"a","conditions":[],"prices":{"input":1}}]`))
	if err != nil || kept.ValueString() != prior.ValueString() {
		t.Fatalf("server defaults must not replace prior: %v %s", err, kept.ValueString())
	}
	for name, remote := range map[string]string{
		"changed value": `[{"name":"a","prices":{"input":2}}]`,
		"extra element": `[{"name":"a","prices":{"input":1}},{"name":"b"}]`,
		"missing key":   `[{"name":"a"}]`,
	} {
		got, err := keepJSON(prior, decode(remote))
		if err != nil || got.ValueString() == prior.ValueString() {
			t.Errorf("%s must surface as drift: %v %s", name, err, got.ValueString())
		}
	}
	null, err := keepJSON(jsontypes.NewNormalizedNull(), nil)
	if err != nil || !null.IsNull() {
		t.Fatalf("nil remote should be null: %v %v", err, null)
	}
}
