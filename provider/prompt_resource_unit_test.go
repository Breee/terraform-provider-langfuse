package provider

import (
	"context"
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func testStringSet(values ...string) types.Set {
	elements := make([]attr.Value, 0, len(values))
	for _, v := range values {
		elements = append(elements, types.StringValue(v))
	}
	return types.SetValueMust(types.StringType, elements)
}

func testPromptPlan() promptResourceModel {
	return promptResourceModel{
		ID:               types.StringUnknown(),
		ProjectPublicKey: types.StringValue("pk"),
		ProjectSecretKey: types.StringValue("sk"),
		Name:             types.StringValue("greeting"),
		Type:             types.StringValue("text"),
		Prompt:           types.StringValue("Hello {{name}}"),
		Config:           jsontypes.NewNormalizedValue(`{"model":"gpt-4o"}`),
		Labels:           testStringSet("production"),
		Tags:             types.SetNull(types.StringType),
		CommitMessage:    types.StringNull(),
		Version:          types.Int64Unknown(),
	}
}

func promptResponse(version int, prompt any, labels ...string) map[string]any {
	return map[string]any{
		"name": "greeting", "version": version, "type": "text", "prompt": prompt,
		"config": map[string]any{"model": "gpt-4o"}, "labels": append(labels, "latest"), "tags": []any{}, "commitMessage": nil,
	}
}

func TestPromptResourceLifecycle(t *testing.T) {
	ctx := context.Background()
	api := newFakeAPI(t)
	r := &promptResource{publicAPIResource{ClientFactory: api.factory()}}

	api.on("POST /api/public/v2/prompts", 201, promptResponse(1, "Hello {{name}}", "production"))
	createResp := resource.CreateResponse{State: testState(t, r, nil)}
	r.Create(ctx, resource.CreateRequest{Plan: testPlan(t, r, testPromptPlan())}, &createResp)
	if createResp.Diagnostics.HasError() {
		t.Fatalf("create: %v", createResp.Diagnostics)
	}
	var state promptResourceModel
	createResp.State.Get(ctx, &state)
	if state.ID.ValueString() != "greeting" || state.Version.ValueInt64() != 1 || !state.Labels.Equal(testStringSet("production")) || !state.Tags.IsNull() {
		t.Fatalf("unexpected state after create: %+v", state)
	}
	want := map[string]any{
		"name": "greeting", "type": "text", "prompt": "Hello {{name}}", "commitMessage": nil,
		"config": map[string]any{"model": "gpt-4o"}, "labels": []any{"production"}, "tags": []any{},
	}
	if got := api.bodies["POST /api/public/v2/prompts"]; !reflect.DeepEqual(got, want) {
		t.Fatalf("create body\n got: %v\nwant: %v", got, want)
	}

	// Read pins the tracked version and surfaces a label that moved away.
	api.on("GET /api/public/v2/prompts/greeting?version=1&resolve=false", 200, promptResponse(1, "Hello {{name}}"))
	readResp := resource.ReadResponse{State: createResp.State}
	r.Read(ctx, resource.ReadRequest{State: createResp.State}, &readResp)
	var drifted promptResourceModel
	readResp.State.Get(ctx, &drifted)
	if readResp.Diagnostics.HasError() || len(drifted.Labels.Elements()) != 0 {
		t.Fatalf("read should surface label drift: %v %+v", readResp.Diagnostics, drifted)
	}

	// Adding a label patches the tracked version.
	plan := testPromptPlan()
	plan.ID, plan.Version = types.StringValue("greeting"), types.Int64Value(1)
	plan.Labels = testStringSet("production", "staging")
	api.on("PATCH /api/public/v2/prompts/greeting/versions/1", 200, promptResponse(1, "Hello {{name}}", "production", "staging"))
	updateResp := resource.UpdateResponse{State: testState(t, r, nil)}
	r.Update(ctx, resource.UpdateRequest{Plan: testPlan(t, r, plan), State: createResp.State}, &updateResp)
	updateResp.State.Get(ctx, &state)
	if updateResp.Diagnostics.HasError() || state.Version.ValueInt64() != 1 || len(state.Labels.Elements()) != 2 {
		t.Fatalf("label update: %v %+v", updateResp.Diagnostics, state)
	}

	// Changing the content posts a new version.
	plan.Prompt, plan.Version = types.StringValue("Hi {{name}}"), types.Int64Unknown()
	api.on("POST /api/public/v2/prompts", 201, promptResponse(2, "Hi {{name}}", "production", "staging"))
	versionResp := resource.UpdateResponse{State: testState(t, r, nil)}
	r.Update(ctx, resource.UpdateRequest{Plan: testPlan(t, r, plan), State: updateResp.State}, &versionResp)
	versionResp.State.Get(ctx, &state)
	if versionResp.Diagnostics.HasError() || state.Version.ValueInt64() != 2 || state.Prompt.ValueString() != "Hi {{name}}" {
		t.Fatalf("content update: %v %+v", versionResp.Diagnostics, state)
	}

	api.on("GET /api/public/v2/prompts/greeting?version=2&resolve=false", 404, map[string]any{"message": "Prompt not found"})
	goneResp := resource.ReadResponse{State: versionResp.State}
	r.Read(ctx, resource.ReadRequest{State: versionResp.State}, &goneResp)
	if goneResp.Diagnostics.HasError() || !goneResp.State.Raw.IsNull() {
		t.Fatalf("read of a deleted prompt should remove it from state: %v", goneResp.Diagnostics)
	}

	api.on("DELETE /api/public/v2/prompts/greeting", 404, map[string]any{"message": "Prompt not found"})
	deleteResp := resource.DeleteResponse{State: versionResp.State}
	r.Delete(ctx, resource.DeleteRequest{State: versionResp.State}, &deleteResp)
	if deleteResp.Diagnostics.HasError() {
		t.Fatalf("deleting an already deleted prompt should succeed: %v", deleteResp.Diagnostics)
	}
}

func TestPromptResourceImportReadsLatest(t *testing.T) {
	ctx := context.Background()
	api := newFakeAPI(t)
	r := &promptResource{publicAPIResource{ClientFactory: api.factory()}}

	importResp := resource.ImportStateResponse{State: testState(t, r, nil)}
	r.ImportState(ctx, resource.ImportStateRequest{ID: "pk:sk:greeting"}, &importResp)
	if importResp.Diagnostics.HasError() {
		t.Fatalf("import: %v", importResp.Diagnostics)
	}
	response := promptResponse(4, []any{map[string]any{"role": "system", "content": "hi"}})
	response["type"], response["config"] = "chat", map[string]any{}
	api.on("GET /api/public/v2/prompts/greeting?label=latest&resolve=false", 200, response)
	readResp := resource.ReadResponse{State: importResp.State}
	r.Read(ctx, resource.ReadRequest{State: importResp.State}, &readResp)
	var state promptResourceModel
	readResp.State.Get(ctx, &state)
	if readResp.Diagnostics.HasError() || state.Version.ValueInt64() != 4 || state.Prompt.ValueString() != `[{"content":"hi","role":"system"}]` ||
		!state.Config.IsNull() || !state.Labels.IsNull() {
		t.Fatalf("import read: %v %+v", readResp.Diagnostics, state)
	}

	badResp := resource.ImportStateResponse{State: testState(t, r, nil)}
	r.ImportState(ctx, resource.ImportStateRequest{ID: "only-a-name"}, &badResp)
	if !badResp.Diagnostics.HasError() {
		t.Fatal("import should reject an ID without credentials")
	}
}

func TestPromptRequestBodyChat(t *testing.T) {
	ctx := context.Background()
	plan := testPromptPlan()
	plan.Type = types.StringValue("chat")
	if _, diags := promptRequestBody(ctx, plan); !diags.HasError() {
		t.Fatal("a chat prompt that is not JSON should be rejected")
	}
	plan.Prompt = types.StringValue(`[{"role":"system","content":"hi"}]`)
	body, diags := promptRequestBody(ctx, plan)
	want := []any{map[string]any{"role": "system", "content": "hi"}}
	if diags.HasError() || !reflect.DeepEqual(body["prompt"], want) {
		t.Fatalf("chat prompt should be sent as an array: %v %v", diags, body["prompt"])
	}
}

func TestPromptNeedsVersion(t *testing.T) {
	state := testPromptPlan()
	state.Version = types.Int64Value(1)
	mutate := func(f func(*promptResourceModel)) promptResourceModel {
		plan := state
		f(&plan)
		return plan
	}
	cases := map[string]struct {
		plan promptResourceModel
		want bool
	}{
		"unchanged":        {state, false},
		"label added":      {mutate(func(p *promptResourceModel) { p.Labels = testStringSet("production", "staging") }), false},
		"config reordered": {mutate(func(p *promptResourceModel) { p.Config = jsontypes.NewNormalizedValue(`{ "model": "gpt-4o" }`) }), false},
		"label removed":    {mutate(func(p *promptResourceModel) { p.Labels = types.SetNull(types.StringType) }), true},
		"label swapped":    {mutate(func(p *promptResourceModel) { p.Labels = testStringSet("staging") }), true},
		"prompt changed":   {mutate(func(p *promptResourceModel) { p.Prompt = types.StringValue("x") }), true},
		"prompt unknown":   {mutate(func(p *promptResourceModel) { p.Prompt = types.StringUnknown() }), true},
		"config changed":   {mutate(func(p *promptResourceModel) { p.Config = jsontypes.NewNormalizedValue(`{"model":"x"}`) }), true},
		"config removed":   {mutate(func(p *promptResourceModel) { p.Config = jsontypes.NewNormalizedNull() }), true},
		"tags changed":     {mutate(func(p *promptResourceModel) { p.Tags = testStringSet("a") }), true},
		"message changed":  {mutate(func(p *promptResourceModel) { p.CommitMessage = types.StringValue("m") }), true},
	}
	for name, tc := range cases {
		if got := promptNeedsVersion(tc.plan, state); got != tc.want {
			t.Errorf("%s: got %v, want %v", name, got, tc.want)
		}
	}
}
