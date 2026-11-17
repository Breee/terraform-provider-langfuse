package provider

import (
	"context"
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func testScoreConfigPlan() scoreConfigResourceModel {
	return scoreConfigResourceModel{
		ID:               types.StringUnknown(),
		ProjectPublicKey: types.StringValue("pk"),
		ProjectSecretKey: types.StringValue("sk"),
		Name:             types.StringValue("accuracy"),
		DataType:         types.StringValue("NUMERIC"),
		Description:      types.StringValue("desc"),
		MinValue:         types.Float64Value(0),
		MaxValue:         types.Float64Null(),
		Categories:       types.ListNull(scoreConfigCategoryType),
		IsArchived:       types.BoolValue(false),
	}
}

func scoreConfigResponse(overrides map[string]any) map[string]any {
	response := map[string]any{
		"id": "sc1", "name": "accuracy", "dataType": "NUMERIC", "description": "desc",
		"minValue": 0, "maxValue": nil, "categories": nil, "isArchived": false,
	}
	for k, v := range overrides {
		response[k] = v
	}
	return response
}

func TestScoreConfigResourceLifecycle(t *testing.T) {
	ctx := context.Background()
	api := newFakeAPI(t)
	r := &scoreConfigResource{publicAPIResource{ClientFactory: api.factory()}}

	api.on("POST /api/public/score-configs", 200, scoreConfigResponse(nil))
	createResp := resource.CreateResponse{State: testState(t, r, nil)}
	r.Create(ctx, resource.CreateRequest{Plan: testPlan(t, r, testScoreConfigPlan())}, &createResp)
	if createResp.Diagnostics.HasError() {
		t.Fatalf("create: %v", createResp.Diagnostics)
	}
	var state scoreConfigResourceModel
	createResp.State.Get(ctx, &state)
	if state.ID.ValueString() != "sc1" || state.MinValue.ValueFloat64() != 0 || !state.MaxValue.IsNull() || !state.Categories.IsNull() {
		t.Fatalf("unexpected state after create: %+v", state)
	}
	want := map[string]any{"name": "accuracy", "dataType": "NUMERIC", "description": "desc", "minValue": 0.0}
	if got := api.bodies["POST /api/public/score-configs"]; !reflect.DeepEqual(got, want) {
		t.Fatalf("create body\n got: %v\nwant: %v", got, want)
	}

	api.on("GET /api/public/score-configs/sc1", 200, scoreConfigResponse(map[string]any{"name": "drifted", "isArchived": true}))
	readResp := resource.ReadResponse{State: createResp.State}
	r.Read(ctx, resource.ReadRequest{State: createResp.State}, &readResp)
	readResp.State.Get(ctx, &state)
	if readResp.Diagnostics.HasError() || state.Name.ValueString() != "drifted" || !state.IsArchived.ValueBool() {
		t.Fatalf("read should surface drift: %v %+v", readResp.Diagnostics, state)
	}

	// Clearing the description sends an empty string, which reads back as null.
	plan := testScoreConfigPlan()
	plan.ID = types.StringValue("sc1")
	plan.Description = types.StringNull()
	api.on("PATCH /api/public/score-configs/sc1", 200, scoreConfigResponse(map[string]any{"description": ""}))
	updateResp := resource.UpdateResponse{State: testState(t, r, nil)}
	r.Update(ctx, resource.UpdateRequest{Plan: testPlan(t, r, plan)}, &updateResp)
	updateResp.State.Get(ctx, &state)
	if updateResp.Diagnostics.HasError() || !state.Description.IsNull() {
		t.Fatalf("update: %v %+v", updateResp.Diagnostics, state)
	}
	want = map[string]any{"name": "accuracy", "description": "", "isArchived": false, "minValue": 0.0}
	if got := api.bodies["PATCH /api/public/score-configs/sc1"]; !reflect.DeepEqual(got, want) {
		t.Fatalf("update body\n got: %v\nwant: %v", got, want)
	}

	// Destroy archives because the API has no delete endpoint.
	deleteResp := resource.DeleteResponse{State: updateResp.State}
	r.Delete(ctx, resource.DeleteRequest{State: updateResp.State}, &deleteResp)
	if deleteResp.Diagnostics.HasError() {
		t.Fatalf("delete: %v", deleteResp.Diagnostics)
	}
	want = map[string]any{"isArchived": true}
	if got := api.bodies["PATCH /api/public/score-configs/sc1"]; !reflect.DeepEqual(got, want) {
		t.Fatalf("delete body\n got: %v\nwant: %v", got, want)
	}

	api.on("GET /api/public/score-configs/sc1", 404, map[string]any{"message": "Score config not found"})
	goneResp := resource.ReadResponse{State: updateResp.State}
	r.Read(ctx, resource.ReadRequest{State: updateResp.State}, &goneResp)
	if goneResp.Diagnostics.HasError() || !goneResp.State.Raw.IsNull() {
		t.Fatalf("read of a missing score config should remove it from state: %v", goneResp.Diagnostics)
	}
}

func TestScoreConfigResourceCreateArchived(t *testing.T) {
	ctx := context.Background()
	api := newFakeAPI(t)
	r := &scoreConfigResource{publicAPIResource{ClientFactory: api.factory()}}

	plan := testScoreConfigPlan()
	plan.IsArchived = types.BoolValue(true)
	api.on("POST /api/public/score-configs", 200, scoreConfigResponse(nil))
	api.on("PATCH /api/public/score-configs/sc1", 200, scoreConfigResponse(map[string]any{"isArchived": true}))
	createResp := resource.CreateResponse{State: testState(t, r, nil)}
	r.Create(ctx, resource.CreateRequest{Plan: testPlan(t, r, plan)}, &createResp)
	var state scoreConfigResourceModel
	createResp.State.Get(ctx, &state)
	if createResp.Diagnostics.HasError() || !state.IsArchived.ValueBool() {
		t.Fatalf("create archived: %v %+v", createResp.Diagnostics, state)
	}

	// An already archived config needs no API call on destroy.
	calls := len(api.calls)
	deleteResp := resource.DeleteResponse{State: createResp.State}
	r.Delete(ctx, resource.DeleteRequest{State: createResp.State}, &deleteResp)
	if deleteResp.Diagnostics.HasError() || len(api.calls) != calls {
		t.Fatalf("delete of an archived config should be a no-op: %v %v", deleteResp.Diagnostics, api.calls[calls:])
	}
}

func TestScoreConfigResourceErrors(t *testing.T) {
	ctx := context.Background()
	api := newFakeAPI(t)
	r := &scoreConfigResource{publicAPIResource{ClientFactory: api.factory()}}

	api.on("POST /api/public/score-configs", 400, map[string]any{"message": "Invalid request data"})
	createResp := resource.CreateResponse{State: testState(t, r, nil)}
	r.Create(ctx, resource.CreateRequest{Plan: testPlan(t, r, testScoreConfigPlan())}, &createResp)
	if !createResp.Diagnostics.HasError() {
		t.Fatal("create should fail on 400")
	}

	importResp := resource.ImportStateResponse{State: testState(t, r, nil)}
	r.ImportState(ctx, resource.ImportStateRequest{ID: "only-an-id"}, &importResp)
	if !importResp.Diagnostics.HasError() {
		t.Fatal("import should reject an ID without credentials")
	}
}

func TestScoreConfigStateCategories(t *testing.T) {
	ctx := context.Background()
	categories := []scoreConfigCategoryAPI{{Label: "True", Value: 1}, {Label: "False", Value: 0}}

	// Server-generated BOOLEAN categories are not tracked.
	state, diags := scoreConfigState(ctx, scoreConfigAPI{ID: "sc1", DataType: "BOOLEAN", Categories: categories}, scoreConfigResourceModel{})
	if diags.HasError() || !state.Categories.IsNull() {
		t.Fatalf("boolean categories should be null: %v %v", diags, state.Categories)
	}

	state, diags = scoreConfigState(ctx, scoreConfigAPI{ID: "sc1", DataType: "CATEGORICAL", Categories: categories}, scoreConfigResourceModel{})
	if diags.HasError() || len(state.Categories.Elements()) != 2 {
		t.Fatalf("categorical categories should be tracked: %v %v", diags, state.Categories)
	}
	first := state.Categories.Elements()[0].(types.Object).Attributes()
	if !reflect.DeepEqual(first, map[string]attr.Value{"label": types.StringValue("True"), "value": types.Float64Value(1)}) {
		t.Fatalf("unexpected category: %v", first)
	}

	body, diags := scoreConfigRequestBody(ctx, state, false)
	if diags.HasError() || !reflect.DeepEqual(body["categories"], categories) {
		t.Fatalf("categories should round-trip into the request: %v %v", diags, body)
	}
}
