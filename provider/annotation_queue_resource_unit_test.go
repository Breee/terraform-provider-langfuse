package provider

import (
	"context"
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func testAnnotationQueuePlan() annotationQueueResourceModel {
	return annotationQueueResourceModel{
		ID:               types.StringUnknown(),
		ProjectPublicKey: types.StringValue("pk"),
		ProjectSecretKey: types.StringValue("sk"),
		Name:             types.StringValue("review"),
		Description:      types.StringNull(),
		ScoreConfigIDs:   testStringSet("sc1", "sc2"),
	}
}

func TestAnnotationQueueResourceLifecycle(t *testing.T) {
	ctx := context.Background()
	api := newFakeAPI(t)
	r := &annotationQueueResource{publicAPIResource{ClientFactory: api.factory()}}
	queue := map[string]any{"id": "q1", "name": "review", "description": nil, "scoreConfigIds": []string{"sc2", "sc1"}}

	api.on("POST /api/public/annotation-queues", 200, queue)
	createResp := resource.CreateResponse{State: testState(t, r, nil)}
	r.Create(ctx, resource.CreateRequest{Plan: testPlan(t, r, testAnnotationQueuePlan())}, &createResp)
	if createResp.Diagnostics.HasError() {
		t.Fatalf("create: %v", createResp.Diagnostics)
	}
	var state annotationQueueResourceModel
	createResp.State.Get(ctx, &state)
	if state.ID.ValueString() != "q1" || !state.Description.IsNull() || !state.ScoreConfigIDs.Equal(testStringSet("sc1", "sc2")) {
		t.Fatalf("unexpected state after create: %+v", state)
	}
	got := api.bodies["POST /api/public/annotation-queues"]
	if _, ok := got["description"]; ok || got["name"] != "review" || len(got["scoreConfigIds"].([]any)) != 2 {
		t.Fatalf("unexpected create body: %v", got)
	}

	queue["description"] = "drifted"
	api.on("GET /api/public/annotation-queues/q1", 200, queue)
	readResp := resource.ReadResponse{State: createResp.State}
	r.Read(ctx, resource.ReadRequest{State: createResp.State}, &readResp)
	readResp.State.Get(ctx, &state)
	if readResp.Diagnostics.HasError() || state.Description.ValueString() != "drifted" {
		t.Fatalf("read should surface drift: %v %+v", readResp.Diagnostics, state)
	}

	// Destroy cannot delete, so it warns without calling the API.
	calls := len(api.calls)
	deleteResp := resource.DeleteResponse{State: createResp.State}
	r.Delete(ctx, resource.DeleteRequest{State: createResp.State}, &deleteResp)
	if deleteResp.Diagnostics.HasError() || deleteResp.Diagnostics.WarningsCount() != 1 || len(api.calls) != calls {
		t.Fatalf("delete should only warn: %v %v", deleteResp.Diagnostics, api.calls[calls:])
	}

	api.on("GET /api/public/annotation-queues/q1", 404, map[string]any{"message": "Annotation queue not found"})
	goneResp := resource.ReadResponse{State: createResp.State}
	r.Read(ctx, resource.ReadRequest{State: createResp.State}, &goneResp)
	if goneResp.Diagnostics.HasError() || !goneResp.State.Raw.IsNull() {
		t.Fatalf("read of a missing queue should remove it from state: %v", goneResp.Diagnostics)
	}

	importResp := resource.ImportStateResponse{State: testState(t, r, nil)}
	r.ImportState(ctx, resource.ImportStateRequest{ID: "only-an-id"}, &importResp)
	if !importResp.Diagnostics.HasError() {
		t.Fatal("import should reject an ID without credentials")
	}
}

func TestAnnotationQueueResourceModifyPlan(t *testing.T) {
	ctx := context.Background()
	r := &annotationQueueResource{}
	state := testAnnotationQueuePlan()
	state.ID = types.StringValue("q1")

	tests := map[string]struct {
		change  func(*annotationQueueResourceModel)
		wantErr bool
	}{
		"no change":               {func(*annotationQueueResourceModel) {}, false},
		"credentials only":        {func(m *annotationQueueResourceModel) { m.ProjectSecretKey = types.StringValue("rotated") }, false},
		"description, same name":  {func(m *annotationQueueResourceModel) { m.Description = types.StringValue("new") }, true},
		"score configs same name": {func(m *annotationQueueResourceModel) { m.ScoreConfigIDs = testStringSet("sc1") }, true},
		"description and rename": {func(m *annotationQueueResourceModel) {
			m.Description = types.StringValue("new")
			m.Name = types.StringValue("review-v2")
		}, false},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			plan := state
			tt.change(&plan)
			resp := resource.ModifyPlanResponse{Plan: testPlan(t, r, plan)}
			r.ModifyPlan(ctx, resource.ModifyPlanRequest{State: testState(t, r, state), Plan: testPlan(t, r, plan)}, &resp)
			if resp.Diagnostics.HasError() != tt.wantErr {
				t.Fatalf("error = %v, want %v: %v", resp.Diagnostics.HasError(), tt.wantErr, resp.Diagnostics)
			}
		})
	}

	// Create and destroy plans are never rejected.
	resp := resource.ModifyPlanResponse{Plan: testPlan(t, r, state)}
	r.ModifyPlan(ctx, resource.ModifyPlanRequest{State: testState(t, r, nil), Plan: testPlan(t, r, state)}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("create plan: %v", resp.Diagnostics)
	}
}

func TestAnnotationQueueAssignmentResource(t *testing.T) {
	ctx := context.Background()
	api := newFakeAPI(t)
	r := &annotationQueueAssignmentResource{publicAPIResource{ClientFactory: api.factory()}}
	plan := annotationQueueAssignmentResourceModel{
		ID:               types.StringUnknown(),
		ProjectPublicKey: types.StringValue("pk"),
		ProjectSecretKey: types.StringValue("sk"),
		QueueID:          types.StringValue("q1"),
		UserID:           types.StringValue("u1"),
	}
	userBody := map[string]any{"userId": "u1"}

	api.on("POST /api/public/annotation-queues/q1/assignments", 404, map[string]any{"message": "User not found or not authorized for this project"})
	createResp := resource.CreateResponse{State: testState(t, r, nil)}
	r.Create(ctx, resource.CreateRequest{Plan: testPlan(t, r, plan)}, &createResp)
	if !createResp.Diagnostics.HasError() {
		t.Fatal("create should fail when the user has no project access")
	}

	api.on("POST /api/public/annotation-queues/q1/assignments", 200, map[string]any{"userId": "u1", "queueId": "q1", "projectId": "p1"})
	createResp = resource.CreateResponse{State: testState(t, r, nil)}
	r.Create(ctx, resource.CreateRequest{Plan: testPlan(t, r, plan)}, &createResp)
	var state annotationQueueAssignmentResourceModel
	createResp.State.Get(ctx, &state)
	if createResp.Diagnostics.HasError() || state.ID.ValueString() != "q1/u1" {
		t.Fatalf("create: %v %+v", createResp.Diagnostics, state)
	}
	if got := api.bodies["POST /api/public/annotation-queues/q1/assignments"]; !reflect.DeepEqual(got, userBody) {
		t.Fatalf("unexpected create body: %v", got)
	}

	// Read cannot call the API and keeps the state.
	calls := len(api.calls)
	readResp := resource.ReadResponse{State: createResp.State}
	r.Read(ctx, resource.ReadRequest{State: createResp.State}, &readResp)
	if readResp.Diagnostics.HasError() || readResp.State.Raw.IsNull() || len(api.calls) != calls {
		t.Fatalf("read should keep the state: %v", readResp.Diagnostics)
	}

	api.on("DELETE /api/public/annotation-queues/q1/assignments", 200, map[string]any{"success": true})
	deleteResp := resource.DeleteResponse{State: createResp.State}
	r.Delete(ctx, resource.DeleteRequest{State: createResp.State}, &deleteResp)
	if deleteResp.Diagnostics.HasError() {
		t.Fatalf("delete: %v", deleteResp.Diagnostics)
	}
	if got := api.bodies["DELETE /api/public/annotation-queues/q1/assignments"]; !reflect.DeepEqual(got, userBody) {
		t.Fatalf("unexpected delete body: %v", got)
	}

	// A queue that is already gone is not an error.
	api.on("DELETE /api/public/annotation-queues/q1/assignments", 404, map[string]any{"message": "Annotation queue not found"})
	deleteResp = resource.DeleteResponse{State: createResp.State}
	r.Delete(ctx, resource.DeleteRequest{State: createResp.State}, &deleteResp)
	if deleteResp.Diagnostics.HasError() {
		t.Fatalf("delete of a missing queue: %v", deleteResp.Diagnostics)
	}
}
