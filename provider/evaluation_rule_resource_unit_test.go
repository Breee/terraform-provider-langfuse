package provider

import (
	"context"
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

const evaluationRulesPath = "/api/public/v2/evaluation-rules"

func testEvaluationRulePlan() evaluationRuleResourceModel {
	return evaluationRuleResourceModel{
		ID:                   types.StringUnknown(),
		ProjectPublicKey:     types.StringValue("pk"),
		ProjectSecretKey:     types.StringValue("sk"),
		Name:                 types.StringValue("chat"),
		Enabled:              types.BoolValue(true),
		Sampling:             types.Float64Value(0.5),
		Filter:               jsontypes.NewNormalizedValue(`[{"type":"stringOptions","column":"name","operator":"any of","value":["chat"]}]`),
		EvaluatorAssignments: jsontypes.NewNormalizedValue(`[{"evaluatorId":"ev1","variableMapping":[{"variable":"output","source":"output"}]}]`),
	}
}

func evaluationRuleResponse(overrides map[string]any) map[string]any {
	response := map[string]any{
		"id": "rule1", "name": "chat", "enabled": true, "sampling": 0.5, "createdBy": "API",
		"filter": []any{map[string]any{"type": "stringOptions", "column": "name", "operator": "any of", "value": []any{"chat"}}},
		"evaluatorAssignments": []any{map[string]any{
			"evaluatorId":     "ev1",
			"variableMapping": []any{map[string]any{"variable": "output", "source": "output", "jsonPath": nil}},
		}},
	}
	for k, v := range overrides {
		response[k] = v
	}
	return response
}

func TestEvaluationRuleResourceLifecycle(t *testing.T) {
	ctx := context.Background()
	api := newFakeAPI(t)
	r := &evaluationRuleResource{publicAPIResource{ClientFactory: api.factory()}}

	api.on("POST "+evaluationRulesPath, 201, evaluationRuleResponse(nil))
	createResp := resource.CreateResponse{State: testState(t, r, nil)}
	r.Create(ctx, resource.CreateRequest{Plan: testPlan(t, r, testEvaluationRulePlan())}, &createResp)
	if createResp.Diagnostics.HasError() {
		t.Fatalf("create: %v", createResp.Diagnostics)
	}
	var state evaluationRuleResourceModel
	createResp.State.Get(ctx, &state)
	// Server-added keys such as a null jsonPath are not reported as drift.
	if state.ID.ValueString() != "rule1" || state.Sampling.ValueFloat64() != 0.5 ||
		state.EvaluatorAssignments.ValueString() != testEvaluationRulePlan().EvaluatorAssignments.ValueString() ||
		state.Filter.ValueString() != testEvaluationRulePlan().Filter.ValueString() {
		t.Fatalf("unexpected state after create: %+v", state)
	}
	want := map[string]any{
		"name": "chat", "enabled": true, "sampling": 0.5,
		"filter": []any{map[string]any{"type": "stringOptions", "column": "name", "operator": "any of", "value": []any{"chat"}}},
		"evaluatorAssignments": []any{map[string]any{
			"evaluatorId":     "ev1",
			"variableMapping": []any{map[string]any{"variable": "output", "source": "output"}},
		}},
	}
	if got := api.bodies["POST "+evaluationRulesPath]; !reflect.DeepEqual(got, want) {
		t.Fatalf("create body\n got: %v\nwant: %v", got, want)
	}

	api.on("GET "+evaluationRulesPath+"/rule1", 200, evaluationRuleResponse(map[string]any{
		"name": "renamed", "enabled": false, "filter": []any{},
	}))
	readResp := resource.ReadResponse{State: createResp.State}
	r.Read(ctx, resource.ReadRequest{State: createResp.State}, &readResp)
	readResp.State.Get(ctx, &state)
	if readResp.Diagnostics.HasError() || state.Name.ValueString() != "renamed" || state.Enabled.ValueBool() ||
		state.Filter.ValueString() != `[]` {
		t.Fatalf("read should surface drift: %v %+v", readResp.Diagnostics, state)
	}

	// Unset lists are sent as empty arrays and stay null in state.
	plan := testEvaluationRulePlan()
	plan.ID = types.StringValue("rule1")
	plan.Enabled = types.BoolValue(false)
	plan.Sampling = types.Float64Value(1)
	plan.Filter = jsontypes.NewNormalizedNull()
	plan.EvaluatorAssignments = jsontypes.NewNormalizedNull()
	api.on("PATCH "+evaluationRulesPath+"/rule1", 200, evaluationRuleResponse(map[string]any{
		"enabled": false, "sampling": 1, "filter": []any{}, "evaluatorAssignments": []any{},
	}))
	updateResp := resource.UpdateResponse{State: testState(t, r, nil)}
	r.Update(ctx, resource.UpdateRequest{Plan: testPlan(t, r, plan)}, &updateResp)
	updateResp.State.Get(ctx, &state)
	if updateResp.Diagnostics.HasError() || !state.Filter.IsNull() || !state.EvaluatorAssignments.IsNull() ||
		state.Enabled.ValueBool() || state.Sampling.ValueFloat64() != 1 {
		t.Fatalf("update: %v %+v", updateResp.Diagnostics, state)
	}
	want = map[string]any{
		"name": "chat", "enabled": false, "sampling": float64(1), "filter": []any{}, "evaluatorAssignments": []any{},
	}
	if got := api.bodies["PATCH "+evaluationRulesPath+"/rule1"]; !reflect.DeepEqual(got, want) {
		t.Fatalf("update body\n got: %v\nwant: %v", got, want)
	}

	api.on("DELETE "+evaluationRulesPath+"/rule1", 200, map[string]any{"id": "rule1"})
	deleteResp := resource.DeleteResponse{State: updateResp.State}
	r.Delete(ctx, resource.DeleteRequest{State: updateResp.State}, &deleteResp)
	if deleteResp.Diagnostics.HasError() {
		t.Fatalf("delete: %v", deleteResp.Diagnostics)
	}
	api.on("DELETE "+evaluationRulesPath+"/rule1", 404, map[string]any{"message": "not found"})
	r.Delete(ctx, resource.DeleteRequest{State: updateResp.State}, &deleteResp)
	if deleteResp.Diagnostics.HasError() {
		t.Fatalf("delete of a missing rule should succeed: %v", deleteResp.Diagnostics)
	}

	api.on("GET "+evaluationRulesPath+"/rule1", 404, map[string]any{"message": "not found"})
	readResp = resource.ReadResponse{State: updateResp.State}
	r.Read(ctx, resource.ReadRequest{State: updateResp.State}, &readResp)
	if readResp.Diagnostics.HasError() || !readResp.State.Raw.IsNull() {
		t.Fatalf("read of a missing rule should remove it from state: %v", readResp.Diagnostics)
	}
}

func TestEvaluationRuleResourceImportAndErrors(t *testing.T) {
	ctx := context.Background()
	api := newFakeAPI(t)
	r := &evaluationRuleResource{publicAPIResource{ClientFactory: api.factory()}}

	api.on("POST "+evaluationRulesPath, 400, map[string]any{"message": "Missing mappings for evaluator variables: output"})
	createResp := resource.CreateResponse{State: testState(t, r, nil)}
	r.Create(ctx, resource.CreateRequest{Plan: testPlan(t, r, testEvaluationRulePlan())}, &createResp)
	if !createResp.Diagnostics.HasError() {
		t.Fatal("create should fail on 400")
	}

	importResp := resource.ImportStateResponse{State: testState(t, r, nil)}
	r.ImportState(ctx, resource.ImportStateRequest{ID: "rule1"}, &importResp)
	if !importResp.Diagnostics.HasError() {
		t.Fatal("import should reject an ID without credentials")
	}

	importResp = resource.ImportStateResponse{State: testState(t, r, nil)}
	r.ImportState(ctx, resource.ImportStateRequest{ID: "pk:sk:rule1"}, &importResp)
	if importResp.Diagnostics.HasError() {
		t.Fatalf("import: %v", importResp.Diagnostics)
	}

	// An imported rule has no prior JSON, so the remote lists are adopted.
	api.on("GET "+evaluationRulesPath+"/rule1", 200, evaluationRuleResponse(map[string]any{"filter": []any{}}))
	readResp := resource.ReadResponse{State: importResp.State}
	r.Read(ctx, resource.ReadRequest{State: importResp.State}, &readResp)
	var state evaluationRuleResourceModel
	readResp.State.Get(ctx, &state)
	if readResp.Diagnostics.HasError() || !state.Filter.IsNull() || state.EvaluatorAssignments.IsNull() ||
		state.ProjectSecretKey.ValueString() != "sk" {
		t.Fatalf("read after import: %v %+v", readResp.Diagnostics, state)
	}
}
