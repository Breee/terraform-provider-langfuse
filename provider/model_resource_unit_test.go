package provider

import (
	"context"
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework-timetypes/timetypes"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func testModelPlan() modelResourceModel {
	return modelResourceModel{
		ID:               types.StringUnknown(),
		ProjectPublicKey: types.StringValue("pk"),
		ProjectSecretKey: types.StringValue("sk"),
		ModelName:        types.StringValue("my-model"),
		MatchPattern:     types.StringValue("(?i)^my-model$"),
		StartDate:        timetypes.NewRFC3339ValueMust("2026-01-01T00:00:00Z"),
		Unit:             types.StringValue("TOKENS"),
		InputPrice:       types.Float64Value(0.001),
		OutputPrice:      types.Float64Value(0.002),
		TotalPrice:       types.Float64Null(),
		TokenizerID:      types.StringNull(),
		TokenizerConfig:  jsontypes.NewNormalizedNull(),
		PricingTiers:     jsontypes.NewNormalizedNull(),
	}
}

func flatModelResponse(matchPattern string) map[string]any {
	return map[string]any{
		"id": "m1", "modelName": "my-model", "matchPattern": matchPattern,
		"startDate": "2026-01-01T00:00:00.000Z", "unit": "TOKENS",
		"inputPrice": 0.001, "outputPrice": 0.002, "totalPrice": nil,
		"tokenizerId": nil, "tokenizerConfig": nil, "isLangfuseManaged": false,
		"pricingTiers": []any{map[string]any{
			"id": "m1_tier_default", "name": "Standard", "isDefault": true, "priority": 0,
			"conditions": []any{}, "prices": map[string]any{"input": 0.001, "output": 0.002},
		}},
	}
}

func TestModelResourceLifecycle(t *testing.T) {
	ctx := context.Background()
	api := newFakeAPI(t)
	r := &modelResource{publicAPIResource{ClientFactory: api.factory()}}

	api.on("POST /api/public/models", 200, flatModelResponse("(?i)^my-model$"))
	createResp := resource.CreateResponse{State: testState(t, r, nil)}
	r.Create(ctx, resource.CreateRequest{Plan: testPlan(t, r, testModelPlan())}, &createResp)
	if createResp.Diagnostics.HasError() {
		t.Fatalf("create: %v", createResp.Diagnostics)
	}
	var state modelResourceModel
	createResp.State.Get(ctx, &state)
	if state.ID.ValueString() != "m1" || state.InputPrice.ValueFloat64() != 0.001 || !state.PricingTiers.IsNull() {
		t.Fatalf("unexpected state after create: %+v", state)
	}
	body := api.bodies["POST /api/public/models"]
	want := map[string]any{
		"modelName": "my-model", "matchPattern": "(?i)^my-model$", "unit": "TOKENS", "startDate": "2026-01-01T00:00:00Z",
		"inputPrice": 0.001, "outputPrice": 0.002, "totalPrice": nil, "tokenizerId": nil, "tokenizerConfig": nil,
	}
	if !reflect.DeepEqual(body, want) {
		t.Fatalf("create body\n got: %v\nwant: %v", body, want)
	}

	api.on("GET /api/public/models/m1", 200, flatModelResponse("(?i)^drifted$"))
	readResp := resource.ReadResponse{State: createResp.State}
	r.Read(ctx, resource.ReadRequest{State: createResp.State}, &readResp)
	readResp.State.Get(ctx, &state)
	if readResp.Diagnostics.HasError() || state.MatchPattern.ValueString() != "(?i)^drifted$" {
		t.Fatalf("read should surface drift: %v %+v", readResp.Diagnostics, state)
	}

	plan := testModelPlan()
	plan.ID = types.StringValue("m1")
	plan.MatchPattern = types.StringValue("(?i)^updated$")
	api.on("PUT /api/public/models/m1", 200, flatModelResponse("(?i)^updated$"))
	updateResp := resource.UpdateResponse{State: testState(t, r, nil)}
	r.Update(ctx, resource.UpdateRequest{Plan: testPlan(t, r, plan)}, &updateResp)
	updateResp.State.Get(ctx, &state)
	if updateResp.Diagnostics.HasError() || state.MatchPattern.ValueString() != "(?i)^updated$" {
		t.Fatalf("update: %v %+v", updateResp.Diagnostics, state)
	}

	api.on("GET /api/public/models/m1", 404, map[string]any{"message": "No model with this id found."})
	goneResp := resource.ReadResponse{State: updateResp.State}
	r.Read(ctx, resource.ReadRequest{State: updateResp.State}, &goneResp)
	if goneResp.Diagnostics.HasError() || !goneResp.State.Raw.IsNull() {
		t.Fatalf("read of a deleted model should remove it from state: %v", goneResp.Diagnostics)
	}

	api.on("DELETE /api/public/models/m1", 404, map[string]any{"message": "No model with this id found."})
	deleteResp := resource.DeleteResponse{State: updateResp.State}
	r.Delete(ctx, resource.DeleteRequest{State: updateResp.State}, &deleteResp)
	if deleteResp.Diagnostics.HasError() {
		t.Fatalf("deleting an already deleted model should succeed: %v", deleteResp.Diagnostics)
	}
}

func TestModelResourceErrors(t *testing.T) {
	ctx := context.Background()
	api := newFakeAPI(t)
	r := &modelResource{publicAPIResource{ClientFactory: api.factory()}}

	api.on("POST /api/public/models", 400, map[string]any{"message": "Invalid request data"})
	createResp := resource.CreateResponse{State: testState(t, r, nil)}
	r.Create(ctx, resource.CreateRequest{Plan: testPlan(t, r, testModelPlan())}, &createResp)
	if !createResp.Diagnostics.HasError() {
		t.Fatal("create should fail on 400")
	}

	var importResp resource.ImportStateResponse
	importResp.State = testState(t, r, nil)
	r.ImportState(ctx, resource.ImportStateRequest{ID: "only-an-id"}, &importResp)
	if !importResp.Diagnostics.HasError() {
		t.Fatal("import should reject an ID without credentials")
	}
}

func TestModelStatePricingForm(t *testing.T) {
	tiers := []map[string]any{
		{"id": "t0", "name": "Standard", "isDefault": true, "priority": 0, "conditions": []any{}, "prices": map[string]any{"input": 0.001}},
		{"id": "t1", "name": "Large", "isDefault": false, "priority": 1, "conditions": []any{}, "prices": map[string]any{"input": 0.002}},
	}
	price := 0.001
	remote := modelAPI{ID: "m1", InputPrice: &price, PricingTiers: tiers}

	// Imported multi-tier models must be tracked as tiers, without generated tier IDs.
	state, diags := modelState(remote, modelResourceModel{PricingTiers: jsontypes.NewNormalizedNull()})
	if diags.HasError() || state.PricingTiers.IsNull() || !state.InputPrice.IsNull() {
		t.Fatalf("expected tiered state: %v %+v", diags, state)
	}
	if got := state.PricingTiers.ValueString(); got != `[{"conditions":[],"isDefault":true,"name":"Standard","prices":{"input":0.001},"priority":0},{"conditions":[],"isDefault":false,"name":"Large","prices":{"input":0.002},"priority":1}]` {
		t.Fatalf("unexpected tiers: %s", got)
	}

	// A configured value that the remote still satisfies is kept verbatim.
	configured := jsontypes.NewNormalizedValue(`[{"name":"Standard","isDefault":true,"priority":0,"conditions":[],"prices":{"input":0.001}},{"name":"Large","priority":1,"conditions":[],"prices":{"input":0.002}}]`)
	state, _ = modelState(remote, modelResourceModel{PricingTiers: configured})
	if state.PricingTiers.ValueString() != configured.ValueString() {
		t.Fatalf("configured tiers should be kept: %s", state.PricingTiers.ValueString())
	}
}
