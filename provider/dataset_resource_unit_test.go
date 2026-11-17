package provider

import (
	"context"
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func testDatasetPlan() datasetResourceModel {
	return datasetResourceModel{
		ID:                   types.StringUnknown(),
		ProjectPublicKey:     types.StringValue("pk"),
		ProjectSecretKey:     types.StringValue("sk"),
		Name:                 types.StringValue("support/faq"),
		Description:          types.StringValue("desc"),
		Metadata:             jsontypes.NewNormalizedValue(`{"team":"support"}`),
		InputSchema:          jsontypes.NewNormalizedValue(`{"type":"object"}`),
		ExpectedOutputSchema: jsontypes.NewNormalizedNull(),
	}
}

func datasetResponse(overrides map[string]any) map[string]any {
	response := map[string]any{
		"id": "ds1", "name": "support/faq", "description": "desc",
		"metadata": map[string]any{"team": "support"}, "inputSchema": map[string]any{"type": "object"},
		"expectedOutputSchema": nil,
	}
	for k, v := range overrides {
		response[k] = v
	}
	return response
}

func TestDatasetResourceLifecycle(t *testing.T) {
	ctx := context.Background()
	api := newFakeAPI(t)
	r := &datasetResource{publicAPIResource{ClientFactory: api.factory()}}

	api.on("POST /api/public/v2/datasets", 200, datasetResponse(nil))
	createResp := resource.CreateResponse{State: testState(t, r, nil)}
	r.Create(ctx, resource.CreateRequest{Plan: testPlan(t, r, testDatasetPlan())}, &createResp)
	if createResp.Diagnostics.HasError() {
		t.Fatalf("create: %v", createResp.Diagnostics)
	}
	var state datasetResourceModel
	createResp.State.Get(ctx, &state)
	if state.ID.ValueString() != "ds1" || state.Metadata.ValueString() != `{"team":"support"}` || !state.ExpectedOutputSchema.IsNull() {
		t.Fatalf("unexpected state after create: %+v", state)
	}
	want := map[string]any{
		"name": "support/faq", "description": "desc", "metadata": map[string]any{"team": "support"},
		"inputSchema": map[string]any{"type": "object"}, "expectedOutputSchema": nil,
	}
	if got := api.bodies["POST /api/public/v2/datasets"]; !reflect.DeepEqual(got, want) {
		t.Fatalf("create body\n got: %v\nwant: %v", got, want)
	}

	api.on("GET /api/public/v2/datasets/support/faq", 200, datasetResponse(map[string]any{"description": "drifted"}))
	readResp := resource.ReadResponse{State: createResp.State}
	r.Read(ctx, resource.ReadRequest{State: createResp.State}, &readResp)
	readResp.State.Get(ctx, &state)
	if readResp.Diagnostics.HasError() || state.Description.ValueString() != "drifted" {
		t.Fatalf("read should surface drift: %v %+v", readResp.Diagnostics, state)
	}

	// The API ignores null, so clearing sends empty values that read back as null.
	plan := testDatasetPlan()
	plan.ID = types.StringValue("ds1")
	plan.Description = types.StringNull()
	plan.Metadata = jsontypes.NewNormalizedNull()
	plan.InputSchema = jsontypes.NewNormalizedNull()
	api.on("POST /api/public/v2/datasets", 200, datasetResponse(map[string]any{"description": "", "metadata": map[string]any{}, "inputSchema": nil}))
	updateResp := resource.UpdateResponse{State: testState(t, r, nil)}
	r.Update(ctx, resource.UpdateRequest{Plan: testPlan(t, r, plan)}, &updateResp)
	updateResp.State.Get(ctx, &state)
	if updateResp.Diagnostics.HasError() || !state.Description.IsNull() || !state.Metadata.IsNull() || !state.InputSchema.IsNull() {
		t.Fatalf("update: %v %+v", updateResp.Diagnostics, state)
	}
	want = map[string]any{
		"name": "support/faq", "description": "", "metadata": map[string]any{},
		"inputSchema": nil, "expectedOutputSchema": nil,
	}
	if got := api.bodies["POST /api/public/v2/datasets"]; !reflect.DeepEqual(got, want) {
		t.Fatalf("update body\n got: %v\nwant: %v", got, want)
	}

	// Destroy cannot delete, so it warns without calling the API.
	calls := len(api.calls)
	deleteResp := resource.DeleteResponse{State: updateResp.State}
	r.Delete(ctx, resource.DeleteRequest{State: updateResp.State}, &deleteResp)
	if deleteResp.Diagnostics.HasError() || deleteResp.Diagnostics.WarningsCount() != 1 || len(api.calls) != calls {
		t.Fatalf("delete should only warn: %v %v", deleteResp.Diagnostics, api.calls[calls:])
	}

	api.on("GET /api/public/v2/datasets/support/faq", 404, map[string]any{"message": "Dataset not found"})
	goneResp := resource.ReadResponse{State: updateResp.State}
	r.Read(ctx, resource.ReadRequest{State: updateResp.State}, &goneResp)
	if goneResp.Diagnostics.HasError() || !goneResp.State.Raw.IsNull() {
		t.Fatalf("read of a missing dataset should remove it from state: %v", goneResp.Diagnostics)
	}
}

func TestDatasetResourceImportAndErrors(t *testing.T) {
	ctx := context.Background()
	api := newFakeAPI(t)
	r := &datasetResource{publicAPIResource{ClientFactory: api.factory()}}

	api.on("POST /api/public/v2/datasets", 400, map[string]any{"message": "Invalid request data"})
	createResp := resource.CreateResponse{State: testState(t, r, nil)}
	r.Create(ctx, resource.CreateRequest{Plan: testPlan(t, r, testDatasetPlan())}, &createResp)
	if !createResp.Diagnostics.HasError() {
		t.Fatal("create should fail on 400")
	}

	importResp := resource.ImportStateResponse{State: testState(t, r, nil)}
	r.ImportState(ctx, resource.ImportStateRequest{ID: "only-a-name"}, &importResp)
	if !importResp.Diagnostics.HasError() {
		t.Fatal("import should reject an ID without credentials")
	}

	importResp = resource.ImportStateResponse{State: testState(t, r, nil)}
	r.ImportState(ctx, resource.ImportStateRequest{ID: "pk:sk:support/faq"}, &importResp)
	var state datasetResourceModel
	importResp.State.Get(ctx, &state)
	if importResp.Diagnostics.HasError() || state.Name.ValueString() != "support/faq" || state.ProjectPublicKey.ValueString() != "pk" {
		t.Fatalf("import: %v %+v", importResp.Diagnostics, state)
	}
}
