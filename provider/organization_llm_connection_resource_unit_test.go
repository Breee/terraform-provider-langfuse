package provider

import (
	"context"
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

const organizationLlmConnectionsPath = "/api/public/organizations/llm-connections"

func testOrganizationLlmConnectionPlan() organizationLlmConnectionModel {
	return organizationLlmConnectionModel{
		ID:                     types.StringUnknown(),
		OrganizationPublicKey:  types.StringValue("pk"),
		OrganizationPrivateKey: types.StringValue("sk"),
		ProviderName:           types.StringValue("shared"),
		Adapter:                types.StringValue("openai"),
		SecretKey:              types.StringValue("llm-secret"),
		BaseURL:                types.StringNull(),
		CustomModels:           types.ListNull(types.StringType),
		ExtraHeaders:           types.MapNull(types.StringType),
		WithDefaultModels:      types.BoolValue(true),
		Config:                 jsontypes.NewNormalizedNull(),
	}
}

func organizationLlmConnectionResponse(overrides map[string]any) map[string]any {
	response := map[string]any{
		"id": "conn1", "provider": "shared", "adapter": "openai", "displaySecretKey": "...cret", "baseURL": nil,
		"customModels": []string{}, "withDefaultModels": true, "extraHeaderKeys": []string{}, "config": nil,
	}
	for k, v := range overrides {
		response[k] = v
	}
	return response
}

func TestOrganizationLlmConnectionResourceLifecycle(t *testing.T) {
	ctx := context.Background()
	api := newFakeAPI(t)
	r := &organizationLlmConnectionResource{publicAPIResource{ClientFactory: api.factory()}}

	api.on("PUT "+organizationLlmConnectionsPath, 201, organizationLlmConnectionResponse(nil))
	createResp := resource.CreateResponse{State: testState(t, r, nil)}
	r.Create(ctx, resource.CreateRequest{Plan: testPlan(t, r, testOrganizationLlmConnectionPlan())}, &createResp)
	if createResp.Diagnostics.HasError() {
		t.Fatalf("create: %v", createResp.Diagnostics)
	}
	var state organizationLlmConnectionModel
	createResp.State.Get(ctx, &state)
	if state.ID.ValueString() != "conn1" || state.SecretKey.ValueString() != "llm-secret" ||
		!state.CustomModels.IsNull() || !state.Config.IsNull() || !state.BaseURL.IsNull() {
		t.Fatalf("unexpected state after create: %+v", state)
	}
	want := map[string]any{"provider": "shared", "adapter": "openai", "secretKey": "llm-secret", "withDefaultModels": true}
	if got := api.bodies["PUT "+organizationLlmConnectionsPath]; !reflect.DeepEqual(got, want) {
		t.Fatalf("create body\n got: %v\nwant: %v", got, want)
	}

	// Read pages through the list, picks up drift, and keeps the write-only values.
	api.on("GET "+organizationLlmConnectionsPath+"?page=1&limit=100", 200, map[string]any{
		"data": []any{organizationLlmConnectionResponse(map[string]any{"id": "other"})},
		"meta": map[string]any{"totalPages": 2},
	})
	api.on("GET "+organizationLlmConnectionsPath+"?page=2&limit=100", 200, map[string]any{
		"data": []any{organizationLlmConnectionResponse(map[string]any{"customModels": []string{"m1"}, "withDefaultModels": false})},
		"meta": map[string]any{"totalPages": 2},
	})
	readResp := resource.ReadResponse{State: createResp.State}
	r.Read(ctx, resource.ReadRequest{State: createResp.State}, &readResp)
	if readResp.Diagnostics.HasError() {
		t.Fatalf("read: %v", readResp.Diagnostics)
	}
	readResp.State.Get(ctx, &state)
	if len(state.CustomModels.Elements()) != 1 || state.WithDefaultModels.ValueBool() || state.SecretKey.ValueString() != "llm-secret" {
		t.Fatalf("unexpected state after read: %+v", state)
	}

	// Update sends every configured field; the stored config keeps its configured form.
	plan := testOrganizationLlmConnectionPlan()
	plan.ID = types.StringValue("conn1")
	plan.Adapter = types.StringValue("bedrock")
	plan.BaseURL = types.StringValue("https://llm.example.com")
	plan.CustomModels = types.ListValueMust(types.StringType, []attr.Value{types.StringValue("m1")})
	plan.ExtraHeaders = types.MapValueMust(types.StringType, map[string]attr.Value{"X-Team": types.StringValue("a")})
	plan.WithDefaultModels = types.BoolValue(false)
	plan.Config = jsontypes.NewNormalizedValue(`{"region":"eu-central-1"}`)
	api.on("PUT "+organizationLlmConnectionsPath, 200, organizationLlmConnectionResponse(map[string]any{
		"adapter": "bedrock", "baseURL": "https://llm.example.com", "customModels": []string{"m1"},
		"withDefaultModels": false, "extraHeaderKeys": []string{"X-Team"}, "config": map[string]any{"region": "eu-central-1"},
	}))
	updateResp := resource.UpdateResponse{State: readResp.State}
	r.Update(ctx, resource.UpdateRequest{Plan: testPlan(t, r, plan), State: readResp.State}, &updateResp)
	if updateResp.Diagnostics.HasError() {
		t.Fatalf("update: %v", updateResp.Diagnostics)
	}
	updateResp.State.Get(ctx, &state)
	if state.Config.ValueString() != `{"region":"eu-central-1"}` || state.BaseURL.ValueString() != "https://llm.example.com" ||
		len(state.ExtraHeaders.Elements()) != 1 {
		t.Fatalf("unexpected state after update: %+v", state)
	}
	want = map[string]any{
		"provider": "shared", "adapter": "bedrock", "secretKey": "llm-secret", "withDefaultModels": false,
		"baseURL": "https://llm.example.com", "customModels": []any{"m1"}, "extraHeaders": map[string]any{"X-Team": "a"},
		"config": map[string]any{"region": "eu-central-1"},
	}
	if got := api.bodies["PUT "+organizationLlmConnectionsPath]; !reflect.DeepEqual(got, want) {
		t.Fatalf("update body\n got: %v\nwant: %v", got, want)
	}

	// Delete tolerates a connection that is already gone.
	api.on("DELETE "+organizationLlmConnectionsPath+"/conn1", 404, map[string]any{"message": "not found"})
	deleteResp := resource.DeleteResponse{State: updateResp.State}
	r.Delete(ctx, resource.DeleteRequest{State: updateResp.State}, &deleteResp)
	if deleteResp.Diagnostics.HasError() {
		t.Fatalf("delete: %v", deleteResp.Diagnostics)
	}

	// Read removes the resource when no page contains it.
	api.on("GET "+organizationLlmConnectionsPath+"?page=1&limit=100", 200, map[string]any{
		"data": []any{}, "meta": map[string]any{"totalPages": 0},
	})
	goneResp := resource.ReadResponse{State: updateResp.State}
	r.Read(ctx, resource.ReadRequest{State: updateResp.State}, &goneResp)
	if !goneResp.State.Raw.IsNull() {
		t.Fatal("expected the resource to be removed from state")
	}
}

func TestOrganizationLlmConnectionResourceErrors(t *testing.T) {
	ctx := context.Background()
	api := newFakeAPI(t)
	r := &organizationLlmConnectionResource{publicAPIResource{ClientFactory: api.factory()}}

	api.on("PUT "+organizationLlmConnectionsPath, 403, map[string]any{"error": "Access denied"})
	createResp := resource.CreateResponse{State: testState(t, r, nil)}
	r.Create(ctx, resource.CreateRequest{Plan: testPlan(t, r, testOrganizationLlmConnectionPlan())}, &createResp)
	if !createResp.Diagnostics.HasError() {
		t.Fatal("expected create to report the API error")
	}

	plan := testOrganizationLlmConnectionPlan()
	plan.ID = types.StringValue("conn1")
	state := testState(t, r, plan)
	api.on("GET "+organizationLlmConnectionsPath+"?page=1&limit=100", 500, map[string]any{"error": "boom"})
	readResp := resource.ReadResponse{State: state}
	r.Read(ctx, resource.ReadRequest{State: state}, &readResp)
	if !readResp.Diagnostics.HasError() {
		t.Fatal("expected read to report the API error")
	}

	api.on("DELETE "+organizationLlmConnectionsPath+"/conn1", 500, map[string]any{"error": "boom"})
	deleteResp := resource.DeleteResponse{State: state}
	r.Delete(ctx, resource.DeleteRequest{State: state}, &deleteResp)
	if !deleteResp.Diagnostics.HasError() {
		t.Fatal("expected delete to report the API error")
	}
}

func TestOrganizationLlmConnectionResourceValidateConfig(t *testing.T) {
	r := &organizationLlmConnectionResource{}
	tests := map[string]struct {
		adapter string
		config  jsontypes.Normalized
		wantErr bool
	}{
		"openai without config":      {"openai", jsontypes.NewNormalizedNull(), false},
		"openai with config":         {"openai", jsontypes.NewNormalizedValue(`{"region":"x"}`), true},
		"bedrock without config":     {"bedrock", jsontypes.NewNormalizedNull(), true},
		"bedrock without region":     {"bedrock", jsontypes.NewNormalizedValue(`{}`), true},
		"bedrock with region":        {"bedrock", jsontypes.NewNormalizedValue(`{"region":"x"}`), false},
		"vertex without config":      {"google-vertex-ai", jsontypes.NewNormalizedNull(), false},
		"vertex without location":    {"google-vertex-ai", jsontypes.NewNormalizedValue(`{}`), true},
		"vertex with location":       {"google-vertex-ai", jsontypes.NewNormalizedValue(`{"location":"x"}`), false},
		"unknown config is deferred": {"bedrock", jsontypes.NewNormalizedUnknown(), false},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			model := testOrganizationLlmConnectionPlan()
			model.Adapter = types.StringValue(tt.adapter)
			model.Config = tt.config
			var resp resource.ValidateConfigResponse
			r.ValidateConfig(context.Background(), resource.ValidateConfigRequest{Config: tfsdk.Config(testState(t, r, model))}, &resp)
			if resp.Diagnostics.HasError() != tt.wantErr {
				t.Fatalf("wantErr=%v, got %v", tt.wantErr, resp.Diagnostics)
			}
		})
	}
}

func TestOrganizationLlmConnectionResourceImport(t *testing.T) {
	ctx := context.Background()
	r := &organizationLlmConnectionResource{}

	resp := resource.ImportStateResponse{State: testState(t, r, nil)}
	r.ImportState(ctx, resource.ImportStateRequest{ID: "pk:sk:conn1"}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("import: %v", resp.Diagnostics)
	}
	var id, privateKey types.String
	resp.State.GetAttribute(ctx, path.Root("id"), &id)
	resp.State.GetAttribute(ctx, path.Root("organization_private_key"), &privateKey)
	if id.ValueString() != "conn1" || privateKey.ValueString() != "sk" {
		t.Fatalf("unexpected import state: %s %s", id, privateKey)
	}

	bad := resource.ImportStateResponse{State: testState(t, r, nil)}
	r.ImportState(ctx, resource.ImportStateRequest{ID: "conn1"}, &bad)
	if !bad.Diagnostics.HasError() {
		t.Fatal("expected an error for a malformed import ID")
	}
}
