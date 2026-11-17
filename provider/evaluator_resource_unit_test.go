package provider

import (
	"context"
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

const evaluatorsPath = "/api/public/v2/evaluators"

func testEvaluatorPlan() evaluatorResourceModel {
	return evaluatorResourceModel{
		ID:                 types.StringUnknown(),
		ProjectPublicKey:   types.StringValue("pk"),
		ProjectSecretKey:   types.StringValue("sk"),
		Name:               types.StringValue("helpfulness"),
		Description:        types.StringValue("desc"),
		Type:               types.StringValue(evaluatorTypeLlmAsJudge),
		Prompt:             jsontypes.NewNormalizedValue(`[{"role":"user","content":"Judge {{output}}"}]`),
		ModelProvider:      types.StringValue("openai"),
		Model:              types.StringValue("gpt-4o"),
		VariableMapping:    jsontypes.NewNormalizedValue(`[{"variable":"output","source":"output"}]`),
		OutputDefinition:   jsontypes.NewNormalizedValue(`{"dataType":"NUMERIC","minValue":0,"maxValue":1}`),
		SourceCode:         types.StringNull(),
		SourceCodeLanguage: types.StringNull(),
		Version:            types.Int64Unknown(),
		Status:             types.StringUnknown(),
	}
}

func evaluatorResponse(overrides map[string]any) map[string]any {
	response := map[string]any{
		"id": "ev1", "name": "helpfulness", "description": "desc", "type": "llm_as_judge",
		"status": "paused", "version": 1,
		"prompt":           []any{map[string]any{"role": "user", "content": "Judge {{output}}"}},
		"variables":        []any{"output"},
		"variableMapping":  []any{map[string]any{"variable": "output", "source": "output"}},
		"modelConfig":      map[string]any{"provider": "openai", "model": "gpt-4o"},
		"outputDefinition": map[string]any{"dataType": "NUMERIC", "minValue": 0, "maxValue": 1},
	}
	for k, v := range overrides {
		response[k] = v
	}
	return response
}

func TestEvaluatorResourceLifecycle(t *testing.T) {
	ctx := context.Background()
	api := newFakeAPI(t)
	r := &evaluatorResource{publicAPIResource{ClientFactory: api.factory()}}

	api.on("POST "+evaluatorsPath, 201, evaluatorResponse(nil))
	createResp := resource.CreateResponse{State: testState(t, r, nil)}
	r.Create(ctx, resource.CreateRequest{Plan: testPlan(t, r, testEvaluatorPlan())}, &createResp)
	if createResp.Diagnostics.HasError() {
		t.Fatalf("create: %v", createResp.Diagnostics)
	}
	var state evaluatorResourceModel
	createResp.State.Get(ctx, &state)
	if state.ID.ValueString() != "ev1" || state.Version.ValueInt64() != 1 || state.Status.ValueString() != "paused" ||
		state.Prompt.ValueString() != testEvaluatorPlan().Prompt.ValueString() || !state.SourceCode.IsNull() {
		t.Fatalf("unexpected state after create: %+v", state)
	}
	want := map[string]any{
		"name": "helpfulness", "description": "desc", "type": "llm_as_judge",
		"prompt":           []any{map[string]any{"role": "user", "content": "Judge {{output}}"}},
		"variableMapping":  []any{map[string]any{"variable": "output", "source": "output"}},
		"modelConfig":      map[string]any{"provider": "openai", "model": "gpt-4o"},
		"outputDefinition": map[string]any{"dataType": "NUMERIC", "minValue": float64(0), "maxValue": float64(1)},
	}
	if got := api.bodies["POST "+evaluatorsPath]; !reflect.DeepEqual(got, want) {
		t.Fatalf("create body\n got: %v\nwant: %v", got, want)
	}

	api.on("GET "+evaluatorsPath+"/ev1", 200, evaluatorResponse(map[string]any{
		"name": "renamed", "version": 2, "outputDefinition": map[string]any{"dataType": "BOOLEAN"},
	}))
	readResp := resource.ReadResponse{State: createResp.State}
	r.Read(ctx, resource.ReadRequest{State: createResp.State}, &readResp)
	readResp.State.Get(ctx, &state)
	if readResp.Diagnostics.HasError() || state.Name.ValueString() != "renamed" || state.Version.ValueInt64() != 2 ||
		state.OutputDefinition.ValueString() != `{"dataType":"BOOLEAN"}` {
		t.Fatalf("read should surface drift: %v %+v", readResp.Diagnostics, state)
	}

	// Optional fields are cleared with explicit nulls.
	plan := testEvaluatorPlan()
	plan.ID = types.StringValue("ev1")
	plan.Description = types.StringNull()
	plan.ModelProvider = types.StringNull()
	plan.Model = types.StringNull()
	plan.VariableMapping = jsontypes.NewNormalizedNull()
	api.on("PATCH "+evaluatorsPath+"/ev1", 200, evaluatorResponse(map[string]any{
		"description": nil, "modelConfig": nil, "variableMapping": nil, "version": 3,
	}))
	updateResp := resource.UpdateResponse{State: testState(t, r, nil)}
	r.Update(ctx, resource.UpdateRequest{Plan: testPlan(t, r, plan)}, &updateResp)
	updateResp.State.Get(ctx, &state)
	if updateResp.Diagnostics.HasError() || !state.Description.IsNull() || !state.Model.IsNull() ||
		!state.VariableMapping.IsNull() || state.Version.ValueInt64() != 3 {
		t.Fatalf("update: %v %+v", updateResp.Diagnostics, state)
	}
	want["description"], want["modelConfig"], want["variableMapping"] = nil, nil, nil
	if got := api.bodies["PATCH "+evaluatorsPath+"/ev1"]; !reflect.DeepEqual(got, want) {
		t.Fatalf("update body\n got: %v\nwant: %v", got, want)
	}

	api.on("DELETE "+evaluatorsPath+"/ev1", 200, map[string]any{"id": "ev1"})
	deleteResp := resource.DeleteResponse{State: updateResp.State}
	r.Delete(ctx, resource.DeleteRequest{State: updateResp.State}, &deleteResp)
	if deleteResp.Diagnostics.HasError() {
		t.Fatalf("delete: %v", deleteResp.Diagnostics)
	}
	api.on("DELETE "+evaluatorsPath+"/ev1", 404, map[string]any{"message": "Evaluator not found"})
	r.Delete(ctx, resource.DeleteRequest{State: updateResp.State}, &deleteResp)
	if deleteResp.Diagnostics.HasError() {
		t.Fatalf("delete of a missing evaluator should succeed: %v", deleteResp.Diagnostics)
	}

	api.on("GET "+evaluatorsPath+"/ev1", 404, map[string]any{"message": "Evaluator not found"})
	goneResp := resource.ReadResponse{State: updateResp.State}
	r.Read(ctx, resource.ReadRequest{State: updateResp.State}, &goneResp)
	if goneResp.Diagnostics.HasError() || !goneResp.State.Raw.IsNull() {
		t.Fatalf("read of a missing evaluator should remove it from state: %v", goneResp.Diagnostics)
	}
}

func testCodeEvaluatorPlan() evaluatorResourceModel {
	plan := testEvaluatorPlan()
	plan.Type = types.StringValue(evaluatorTypeCode)
	plan.Prompt = jsontypes.NewNormalizedNull()
	plan.ModelProvider = types.StringNull()
	plan.Model = types.StringNull()
	plan.VariableMapping = jsontypes.NewNormalizedNull()
	plan.OutputDefinition = jsontypes.NewNormalizedNull()
	plan.SourceCode = types.StringValue("def evaluate(): return 1")
	plan.SourceCodeLanguage = types.StringValue("PYTHON")
	return plan
}

func TestEvaluatorResourceCode(t *testing.T) {
	ctx := context.Background()
	api := newFakeAPI(t)
	r := &evaluatorResource{publicAPIResource{ClientFactory: api.factory()}}

	api.on("POST "+evaluatorsPath, 201, map[string]any{
		"id": "ev2", "name": "helpfulness", "description": "desc", "type": "code", "status": "active", "version": 1,
		"sourceCode": "def evaluate(): return 1", "sourceCodeLanguage": "PYTHON",
	})
	createResp := resource.CreateResponse{State: testState(t, r, nil)}
	r.Create(ctx, resource.CreateRequest{Plan: testPlan(t, r, testCodeEvaluatorPlan())}, &createResp)
	var state evaluatorResourceModel
	createResp.State.Get(ctx, &state)
	if createResp.Diagnostics.HasError() || state.SourceCodeLanguage.ValueString() != "PYTHON" || !state.Prompt.IsNull() {
		t.Fatalf("create: %v %+v", createResp.Diagnostics, state)
	}
	want := map[string]any{
		"name": "helpfulness", "description": "desc", "type": "code",
		"sourceCode": "def evaluate(): return 1", "sourceCodeLanguage": "PYTHON",
	}
	if got := api.bodies["POST "+evaluatorsPath]; !reflect.DeepEqual(got, want) {
		t.Fatalf("create body\n got: %v\nwant: %v", got, want)
	}
}

func TestEvaluatorResourceValidateConfig(t *testing.T) {
	ctx := context.Background()
	r := &evaluatorResource{}
	noPrompt := testEvaluatorPlan()
	noPrompt.Prompt = jsontypes.NewNormalizedNull()
	llmWithCode := testEvaluatorPlan()
	llmWithCode.SourceCode = types.StringValue("x")
	codeWithPrompt := testCodeEvaluatorPlan()
	codeWithPrompt.Prompt = jsontypes.NewNormalizedValue(`[]`)
	codeNoLanguage := testCodeEvaluatorPlan()
	codeNoLanguage.SourceCodeLanguage = types.StringNull()

	for name, tc := range map[string]struct {
		config  evaluatorResourceModel
		wantErr bool
	}{
		"llm":              {testEvaluatorPlan(), false},
		"code":             {testCodeEvaluatorPlan(), false},
		"llm no prompt":    {noPrompt, true},
		"llm with code":    {llmWithCode, true},
		"code with prompt": {codeWithPrompt, true},
		"code no language": {codeNoLanguage, true},
	} {
		t.Run(name, func(t *testing.T) {
			state := testState(t, r, tc.config)
			resp := resource.ValidateConfigResponse{}
			r.ValidateConfig(ctx, resource.ValidateConfigRequest{Config: tfsdk.Config(state)}, &resp)
			if resp.Diagnostics.HasError() != tc.wantErr {
				t.Fatalf("wantErr=%v, got %v", tc.wantErr, resp.Diagnostics)
			}
		})
	}
}

func TestEvaluatorResourceImportAndErrors(t *testing.T) {
	ctx := context.Background()
	api := newFakeAPI(t)
	r := &evaluatorResource{publicAPIResource{ClientFactory: api.factory()}}

	api.on("POST "+evaluatorsPath, 412, map[string]any{"message": "Code evaluations are not enabled for this deployment."})
	createResp := resource.CreateResponse{State: testState(t, r, nil)}
	r.Create(ctx, resource.CreateRequest{Plan: testPlan(t, r, testCodeEvaluatorPlan())}, &createResp)
	if !createResp.Diagnostics.HasError() {
		t.Fatal("create should fail on 412")
	}

	importResp := resource.ImportStateResponse{State: testState(t, r, nil)}
	r.ImportState(ctx, resource.ImportStateRequest{ID: "ev1"}, &importResp)
	if !importResp.Diagnostics.HasError() {
		t.Fatal("import should reject an ID without credentials")
	}

	importResp = resource.ImportStateResponse{State: testState(t, r, nil)}
	r.ImportState(ctx, resource.ImportStateRequest{ID: "pk:sk:ev1"}, &importResp)
	var state evaluatorResourceModel
	importResp.State.Get(ctx, &state)
	if importResp.Diagnostics.HasError() || state.ID.ValueString() != "ev1" || state.ProjectSecretKey.ValueString() != "sk" {
		t.Fatalf("import: %v %+v", importResp.Diagnostics, state)
	}
}
