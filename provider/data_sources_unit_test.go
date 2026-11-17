package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// readDataSource runs Read with the given configuration (a map of attribute
// name to string or int64 value) and decodes the resulting state into out.
func readDataSource(t *testing.T, d datasource.DataSource, config map[string]any, out any) bool {
	t.Helper()
	ctx := context.Background()
	var schemaResp datasource.SchemaResponse
	d.Schema(ctx, datasource.SchemaRequest{}, &schemaResp)
	if diags := schemaResp.Schema.ValidateImplementation(ctx); diags.HasError() {
		t.Fatalf("schema implementation: %v", diags)
	}

	objectType := schemaResp.Schema.Type().TerraformType(ctx).(tftypes.Object)
	values := map[string]tftypes.Value{}
	for name, attrType := range objectType.AttributeTypes {
		values[name] = tftypes.NewValue(attrType, nil)
		if v, ok := config[name]; ok {
			values[name] = tftypes.NewValue(attrType, v)
		}
	}
	raw := tftypes.NewValue(objectType, values)

	resp := datasource.ReadResponse{State: tfsdk.State{Schema: schemaResp.Schema, Raw: tftypes.NewValue(objectType, nil)}}
	d.Read(ctx, datasource.ReadRequest{Config: tfsdk.Config{Schema: schemaResp.Schema, Raw: raw}}, &resp)
	if resp.Diagnostics.HasError() {
		return false
	}
	if diags := resp.State.Get(ctx, out); diags.HasError() {
		t.Fatalf("decode state: %v", diags)
	}
	return true
}

var (
	testProjectKeys      = map[string]any{"project_public_key": "pk", "project_secret_key": "sk"}
	testOrganizationKeys = map[string]any{"organization_public_key": "pk", "organization_private_key": "sk"}
)

func withConfig(base map[string]any, extra map[string]any) map[string]any {
	config := map[string]any{}
	for k, v := range base {
		config[k] = v
	}
	for k, v := range extra {
		config[k] = v
	}
	return config
}

func TestProjectDataSources(t *testing.T) {
	api := newFakeAPI(t)
	base := publicAPIDataSource{ClientFactory: api.factory()}

	api.on("GET /api/public/projects", 200, map[string]any{"data": []any{map[string]any{
		"id": "p1", "name": "demo", "metadata": map[string]any{"team": "a"},
		"organization": map[string]any{"id": "o1", "name": "acme"},
	}}})
	var project projectDataSourceModel
	if !readDataSource(t, &projectDataSource{base}, testProjectKeys, &project) {
		t.Fatal("project read failed")
	}
	if project.ID.ValueString() != "p1" || project.OrganizationName.ValueString() != "acme" || project.Metadata.ValueString() != `{"team":"a"}` {
		t.Fatalf("unexpected project: %+v", project)
	}

	api.on("GET /api/public/projects", 200, map[string]any{"data": []any{}})
	if readDataSource(t, &projectDataSource{base}, testProjectKeys, &project) {
		t.Fatal("expected an error when the key resolves to no project")
	}

	api.on("GET /api/public/organizations/projects", 200, map[string]any{"projects": []any{
		map[string]any{"id": "p1", "name": "demo", "metadata": nil},
		map[string]any{"id": "p2", "name": "other", "metadata": map[string]any{"k": "v"}},
	}})
	var projects projectsDataSourceModel
	if !readDataSource(t, &projectsDataSource{base}, testOrganizationKeys, &projects) {
		t.Fatal("projects read failed")
	}
	var items []projectsItem
	projects.Projects.ElementsAs(context.Background(), &items, false)
	if len(items) != 2 || items[1].Name.ValueString() != "other" || !items[0].Metadata.IsNull() {
		t.Fatalf("unexpected projects: %+v", items)
	}

	api.on("GET /api/public/organizations/projects", 403, map[string]any{"error": "denied"})
	if readDataSource(t, &projectsDataSource{base}, testOrganizationKeys, &projects) {
		t.Fatal("expected the API error to be reported")
	}
}

func TestUserDataSource(t *testing.T) {
	api := newFakeAPI(t)
	d := &userDataSource{publicAPIDataSource{ClientFactory: api.factory()}}
	config := withConfig(testOrganizationKeys, map[string]any{"email": "a@example.com"})
	const key = "GET /api/public/scim/Users?filter=userName+eq+%22a%40example.com%22"

	api.on(key, 200, map[string]any{"Resources": []any{
		map[string]any{"id": "u1", "userName": "a@example.com", "name": map[string]any{"formatted": "Ada"}},
	}})
	var user userDataSourceModel
	if !readDataSource(t, d, config, &user) {
		t.Fatalf("user read failed; calls: %v", api.calls)
	}
	if user.ID.ValueString() != "u1" || user.Name.ValueString() != "Ada" {
		t.Fatalf("unexpected user: %+v", user)
	}

	api.on(key, 200, map[string]any{"Resources": []any{}})
	if readDataSource(t, d, config, &user) {
		t.Fatal("expected an error for an unknown email")
	}
}

func TestPromptDataSource(t *testing.T) {
	api := newFakeAPI(t)
	d := &promptDataSource{publicAPIDataSource{ClientFactory: api.factory()}}

	api.on("GET /api/public/v2/prompts/greet", 200, map[string]any{
		"name": "greet", "version": 3, "type": "text", "prompt": "Hello {{name}}", "config": map[string]any{},
		"labels": []string{"production", "latest"}, "tags": []string{}, "commitMessage": nil,
	})
	var prompt promptDataSourceModel
	if !readDataSource(t, d, withConfig(testProjectKeys, map[string]any{"name": "greet"}), &prompt) {
		t.Fatal("prompt read failed")
	}
	if prompt.Version.ValueInt64() != 3 || prompt.Prompt.ValueString() != "Hello {{name}}" || prompt.Config.ValueString() != "{}" ||
		len(prompt.Labels.Elements()) != 2 || prompt.Tags.IsNull() || !prompt.CommitMessage.IsNull() {
		t.Fatalf("unexpected prompt: %+v", prompt)
	}

	chat := map[string]any{
		"name": "greet", "version": 2, "type": "chat", "config": nil, "labels": []string{"staging"}, "tags": []string{"t"},
		"prompt": []any{map[string]any{"role": "system", "content": "Be kind"}}, "commitMessage": "tune",
	}
	api.on("GET /api/public/v2/prompts/greet?version=2", 200, chat)
	if !readDataSource(t, d, withConfig(testProjectKeys, map[string]any{"name": "greet", "version": int64(2)}), &prompt) {
		t.Fatal("prompt read by version failed")
	}
	if prompt.Type.ValueString() != "chat" || prompt.Prompt.ValueString() != `[{"content":"Be kind","role":"system"}]` ||
		!prompt.Config.IsNull() || prompt.CommitMessage.ValueString() != "tune" {
		t.Fatalf("unexpected chat prompt: %+v", prompt)
	}

	api.on("GET /api/public/v2/prompts/greet?label=staging", 200, chat)
	if !readDataSource(t, d, withConfig(testProjectKeys, map[string]any{"name": "greet", "label": "staging"}), &prompt) {
		t.Fatal("prompt read by label failed")
	}
	if prompt.Version.ValueInt64() != 2 {
		t.Fatalf("unexpected version: %s", prompt.Version)
	}

	api.on("GET /api/public/v2/prompts/missing", 404, map[string]any{"message": "not found"})
	if readDataSource(t, d, withConfig(testProjectKeys, map[string]any{"name": "missing"}), &prompt) {
		t.Fatal("expected an error for a missing prompt")
	}
	if len(d.ConfigValidators(context.Background())) != 1 {
		t.Fatal("expected version and label to be declared as conflicting")
	}
}

func TestScoreConfigDataSource(t *testing.T) {
	api := newFakeAPI(t)
	d := &scoreConfigDataSource{publicAPIDataSource{ClientFactory: api.factory()}}
	config := func(name string) map[string]any {
		return withConfig(testProjectKeys, map[string]any{"name": name})
	}
	scoreConfig := func(id, name string, archived bool) map[string]any {
		return map[string]any{
			"id": id, "name": name, "dataType": "CATEGORICAL", "isArchived": archived, "description": "d",
			"minValue": nil, "maxValue": nil, "categories": []any{map[string]any{"label": "good", "value": 1}},
		}
	}
	api.on("GET /api/public/score-configs?page=1&limit=100", 200, map[string]any{
		"data": []any{scoreConfig("a-old", "quality", true), scoreConfig("dup-1", "dup", false)},
		"meta": map[string]any{"totalPages": 2},
	})
	api.on("GET /api/public/score-configs?page=2&limit=100", 200, map[string]any{
		"data": []any{scoreConfig("a-new", "quality", false), scoreConfig("dup-2", "dup", false)},
		"meta": map[string]any{"totalPages": 2},
	})

	var got scoreConfigDataSourceModel
	if !readDataSource(t, d, config("quality"), &got) {
		t.Fatal("score config read failed")
	}
	if got.ID.ValueString() != "a-new" || got.DataType.ValueString() != "CATEGORICAL" || len(got.Categories.Elements()) != 1 ||
		!got.MinValue.IsNull() || got.Description.ValueString() != "d" {
		t.Fatalf("unexpected score config: %+v", got)
	}
	if readDataSource(t, d, config("dup"), &got) {
		t.Fatal("expected an error for an ambiguous name")
	}
	if readDataSource(t, d, config("missing"), &got) {
		t.Fatal("expected an error for an unknown name")
	}
}

func TestDatasetDataSource(t *testing.T) {
	api := newFakeAPI(t)
	d := &datasetDataSource{publicAPIDataSource{ClientFactory: api.factory()}}

	api.on("GET /api/public/v2/datasets/suite/a", 200, map[string]any{
		"id": "d1", "name": "suite/a", "description": nil, "metadata": map[string]any{"owner": "qa"},
		"inputSchema": map[string]any{"type": "object"}, "expectedOutputSchema": nil,
	})
	var dataset datasetDataSourceModel
	if !readDataSource(t, d, withConfig(testProjectKeys, map[string]any{"name": "suite/a"}), &dataset) {
		t.Fatalf("dataset read failed; calls: %v", api.calls)
	}
	if dataset.ID.ValueString() != "d1" || !dataset.Description.IsNull() || dataset.Metadata.ValueString() != `{"owner":"qa"}` ||
		dataset.InputSchema.ValueString() != `{"type":"object"}` || !dataset.ExpectedOutputSchema.IsNull() {
		t.Fatalf("unexpected dataset: %+v", dataset)
	}

	api.on("GET /api/public/v2/datasets/missing", 404, map[string]any{"message": "not found"})
	if readDataSource(t, d, withConfig(testProjectKeys, map[string]any{"name": "missing"}), &dataset) {
		t.Fatal("expected an error for a missing dataset")
	}
}

func TestDataSourcesConfigure(t *testing.T) {
	var d publicAPIDataSource
	var resp datasource.ConfigureResponse
	d.Configure(context.Background(), datasource.ConfigureRequest{ProviderData: "wrong"}, &resp)
	if !resp.Diagnostics.HasError() {
		t.Fatal("expected an error for unexpected provider data")
	}
}
