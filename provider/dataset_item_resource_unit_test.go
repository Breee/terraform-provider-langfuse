package provider

import (
	"context"
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func testDatasetItemPlan() datasetItemResourceModel {
	return datasetItemResourceModel{
		ID:                  types.StringUnknown(),
		ProjectPublicKey:    types.StringValue("pk"),
		ProjectSecretKey:    types.StringValue("sk"),
		DatasetName:         types.StringValue("faq"),
		Input:               jsontypes.NewNormalizedValue(`{"question":"hi"}`),
		ExpectedOutput:      jsontypes.NewNormalizedValue(`"hello"`),
		Metadata:            jsontypes.NewNormalizedNull(),
		SourceTraceID:       types.StringNull(),
		SourceObservationID: types.StringNull(),
		Status:              types.StringValue("ACTIVE"),
	}
}

func datasetItemResponse(overrides map[string]any) map[string]any {
	response := map[string]any{
		"id": "item1", "datasetName": "faq", "input": map[string]any{"question": "hi"},
		"expectedOutput": "hello", "metadata": nil, "sourceTraceId": nil, "sourceObservationId": nil, "status": "ACTIVE",
	}
	for k, v := range overrides {
		response[k] = v
	}
	return response
}

func TestDatasetItemResourceLifecycle(t *testing.T) {
	ctx := context.Background()
	api := newFakeAPI(t)
	r := &datasetItemResource{publicAPIResource{ClientFactory: api.factory()}}

	api.on("POST /api/public/dataset-items", 200, datasetItemResponse(nil))
	createResp := resource.CreateResponse{State: testState(t, r, nil)}
	r.Create(ctx, resource.CreateRequest{Plan: testPlan(t, r, testDatasetItemPlan())}, &createResp)
	if createResp.Diagnostics.HasError() {
		t.Fatalf("create: %v", createResp.Diagnostics)
	}
	var state datasetItemResourceModel
	createResp.State.Get(ctx, &state)
	if state.ID.ValueString() != "item1" || state.ExpectedOutput.ValueString() != `"hello"` || !state.Metadata.IsNull() {
		t.Fatalf("unexpected state after create: %+v", state)
	}
	want := map[string]any{
		"datasetName": "faq", "status": "ACTIVE", "input": map[string]any{"question": "hi"}, "expectedOutput": "hello",
	}
	if got := api.bodies["POST /api/public/dataset-items"]; !reflect.DeepEqual(got, want) {
		t.Fatalf("create body\n got: %v\nwant: %v", got, want)
	}

	api.on("GET /api/public/dataset-items/item1", 200, datasetItemResponse(map[string]any{"status": "ARCHIVED"}))
	readResp := resource.ReadResponse{State: createResp.State}
	r.Read(ctx, resource.ReadRequest{State: createResp.State}, &readResp)
	readResp.State.Get(ctx, &state)
	if readResp.Diagnostics.HasError() || state.Status.ValueString() != "ARCHIVED" {
		t.Fatalf("read should surface drift: %v %+v", readResp.Diagnostics, state)
	}

	// An update upserts by ID.
	plan := testDatasetItemPlan()
	plan.ID = types.StringValue("item1")
	plan.Metadata = jsontypes.NewNormalizedValue(`{"k":1}`)
	api.on("POST /api/public/dataset-items", 200, datasetItemResponse(map[string]any{"metadata": map[string]any{"k": 1}}))
	updateResp := resource.UpdateResponse{State: testState(t, r, nil)}
	r.Update(ctx, resource.UpdateRequest{Plan: testPlan(t, r, plan)}, &updateResp)
	updateResp.State.Get(ctx, &state)
	if updateResp.Diagnostics.HasError() || state.Metadata.ValueString() != `{"k":1}` {
		t.Fatalf("update: %v %+v", updateResp.Diagnostics, state)
	}
	if got := api.bodies["POST /api/public/dataset-items"]; got["id"] != "item1" || !reflect.DeepEqual(got["metadata"], map[string]any{"k": 1.0}) {
		t.Fatalf("unexpected update body: %v", got)
	}

	api.on("DELETE /api/public/dataset-items/item1", 200, map[string]any{"message": "deleted"})
	deleteResp := resource.DeleteResponse{State: updateResp.State}
	r.Delete(ctx, resource.DeleteRequest{State: updateResp.State}, &deleteResp)
	if deleteResp.Diagnostics.HasError() {
		t.Fatalf("delete: %v", deleteResp.Diagnostics)
	}

	// An item that is already gone is not an error.
	api.on("DELETE /api/public/dataset-items/item1", 404, map[string]any{"message": "not found"})
	deleteResp = resource.DeleteResponse{State: updateResp.State}
	r.Delete(ctx, resource.DeleteRequest{State: updateResp.State}, &deleteResp)
	if deleteResp.Diagnostics.HasError() {
		t.Fatalf("delete of a missing item: %v", deleteResp.Diagnostics)
	}

	api.on("GET /api/public/dataset-items/item1", 404, map[string]any{"message": "not found"})
	goneResp := resource.ReadResponse{State: updateResp.State}
	r.Read(ctx, resource.ReadRequest{State: updateResp.State}, &goneResp)
	if goneResp.Diagnostics.HasError() || !goneResp.State.Raw.IsNull() {
		t.Fatalf("read of a missing item should remove it from state: %v", goneResp.Diagnostics)
	}
}

func TestDatasetItemResourceErrors(t *testing.T) {
	ctx := context.Background()
	api := newFakeAPI(t)
	r := &datasetItemResource{publicAPIResource{ClientFactory: api.factory()}}

	api.on("POST /api/public/dataset-items", 409, map[string]any{"message": "already exists in another dataset"})
	createResp := resource.CreateResponse{State: testState(t, r, nil)}
	r.Create(ctx, resource.CreateRequest{Plan: testPlan(t, r, testDatasetItemPlan())}, &createResp)
	if !createResp.Diagnostics.HasError() {
		t.Fatal("create should fail on 409")
	}

	api.on("DELETE /api/public/dataset-items/item1", 500, map[string]any{"message": "boom"})
	plan := testDatasetItemPlan()
	plan.ID = types.StringValue("item1")
	deleteResp := resource.DeleteResponse{State: testState(t, r, plan)}
	r.Delete(ctx, resource.DeleteRequest{State: testState(t, r, plan)}, &deleteResp)
	if !deleteResp.Diagnostics.HasError() {
		t.Fatal("delete should fail on 500")
	}

	importResp := resource.ImportStateResponse{State: testState(t, r, nil)}
	r.ImportState(ctx, resource.ImportStateRequest{ID: "only-an-id"}, &importResp)
	if !importResp.Diagnostics.HasError() {
		t.Fatal("import should reject an ID without credentials")
	}
}
