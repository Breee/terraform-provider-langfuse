package provider

import (
	"context"
	"reflect"
	"slices"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

const (
	skillsPath        = "/api/public/skills"
	testSkillManifest = "---\nname: my-skill\ndescription: Does things\n---\n# My skill\n"
)

func testSkillFiles(files map[string]string) types.Map {
	elements := map[string]attr.Value{}
	for path, content := range files {
		elements[path] = types.StringValue(content)
	}
	return types.MapValueMust(types.StringType, elements)
}

func testSkillPlan() skillResourceModel {
	return skillResourceModel{
		ID:               types.StringUnknown(),
		ProjectPublicKey: types.StringValue("pk"),
		ProjectSecretKey: types.StringValue("sk"),
		Name:             types.StringValue("my-skill"),
		Files:            testSkillFiles(map[string]string{skillManifest: testSkillManifest, "scripts/a.sh": "echo hi"}),
		CommitMessage:    types.StringValue("first"),
		Labels:           testStringSet("production"),
		Tags:             testStringSet("docs"),
		Description:      types.StringUnknown(),
		Version:          types.Int64Unknown(),
	}
}

func skillResponse(overrides map[string]any) map[string]any {
	response := map[string]any{
		"id": "uuid", "name": "my-skill", "description": "Does things", "version": 1,
		"tags": []any{}, "labels": []any{"latest"}, "commitMessage": "first",
		"files": []any{
			map[string]any{"path": "scripts/a.sh", "content": "echo hi"},
			map[string]any{"path": skillManifest, "content": testSkillManifest},
		},
	}
	for k, v := range overrides {
		response[k] = v
	}
	return response
}

func TestSkillResourceLifecycle(t *testing.T) {
	ctx := context.Background()
	api := newFakeAPI(t)
	r := &skillResource{publicAPIResource{ClientFactory: api.factory()}}
	tagsKey, labelsKey := "PATCH "+skillsPath+"/my-skill", "PATCH "+skillsPath+"/my-skill/versions/"

	api.on("POST "+skillsPath, 201, skillResponse(nil))
	api.on(tagsKey, 200, skillResponse(nil))
	api.on(labelsKey+"1", 200, skillResponse(nil))
	createResp := resource.CreateResponse{State: testState(t, r, nil)}
	r.Create(ctx, resource.CreateRequest{Plan: testPlan(t, r, testSkillPlan())}, &createResp)
	if createResp.Diagnostics.HasError() {
		t.Fatalf("create: %v", createResp.Diagnostics)
	}
	var state skillResourceModel
	createResp.State.Get(ctx, &state)
	if state.ID.ValueString() != "my-skill" || state.Version.ValueInt64() != 1 || state.Description.ValueString() != "Does things" ||
		!state.Labels.Equal(testStringSet("production")) || !state.Tags.Equal(testStringSet("docs")) {
		t.Fatalf("unexpected state after create: %+v", state)
	}
	wantFiles := map[string]any{
		"commitMessage": "first",
		"files": []any{
			map[string]any{"path": skillManifest, "content": testSkillManifest},
			map[string]any{"path": "scripts/a.sh", "content": "echo hi"},
		},
	}
	if got := api.bodies["POST "+skillsPath]; !reflect.DeepEqual(got, wantFiles) {
		t.Fatalf("create body\n got: %v\nwant: %v", got, wantFiles)
	}
	if got := api.bodies[tagsKey]; !reflect.DeepEqual(got, map[string]any{"tags": []any{"docs"}}) {
		t.Fatalf("tags body: %v", got)
	}
	if got := api.bodies[labelsKey+"1"]; !reflect.DeepEqual(got, map[string]any{"labels": []any{"production"}}) {
		t.Fatalf("labels body: %v", got)
	}

	// Read pins the tracked version and surfaces drift in files, labels, and tags.
	api.on("GET "+skillsPath+"/my-skill?version=1&includeContent=true", 200, skillResponse(map[string]any{
		"tags": []any{"docs", "extra"}, "labels": []any{"latest"},
		"files": []any{map[string]any{"path": skillManifest, "content": testSkillManifest}},
	}))
	readResp := resource.ReadResponse{State: createResp.State}
	r.Read(ctx, resource.ReadRequest{State: createResp.State}, &readResp)
	readResp.State.Get(ctx, &state)
	if readResp.Diagnostics.HasError() || len(state.Files.Elements()) != 1 || len(state.Labels.Elements()) != 0 ||
		!state.Tags.Equal(testStringSet("docs", "extra")) {
		t.Fatalf("read should surface drift: %v %+v", readResp.Diagnostics, state)
	}

	// Changing only labels and tags keeps the version.
	api.calls = nil
	plan := testSkillPlan()
	plan.ID, plan.Version, plan.Description = types.StringValue("my-skill"), types.Int64Value(1), types.StringValue("Does things")
	plan.Labels, plan.Tags = testStringSet("staging"), types.SetNull(types.StringType)
	updateResp := resource.UpdateResponse{State: testState(t, r, nil)}
	r.Update(ctx, resource.UpdateRequest{Plan: testPlan(t, r, plan), State: createResp.State}, &updateResp)
	updateResp.State.Get(ctx, &state)
	if updateResp.Diagnostics.HasError() || state.Version.ValueInt64() != 1 || !state.Tags.IsNull() ||
		!state.Labels.Equal(testStringSet("staging")) || state.Description.ValueString() != "Does things" {
		t.Fatalf("label update: %v %+v", updateResp.Diagnostics, state)
	}
	if !slices.Equal(api.calls, []string{tagsKey, labelsKey + "1"}) {
		t.Fatalf("label update calls: %v", api.calls)
	}
	if got := api.bodies[tagsKey]; !reflect.DeepEqual(got, map[string]any{"tags": []any{}}) {
		t.Fatalf("tags body: %v", got)
	}

	// Changing files creates a new version; inherited tags are not sent again.
	api.calls = nil
	plan.Files = testSkillFiles(map[string]string{skillManifest: testSkillManifest})
	plan.CommitMessage = types.StringNull()
	plan.Version, plan.Description = types.Int64Unknown(), types.StringUnknown()
	api.on("POST "+skillsPath, 201, skillResponse(map[string]any{"version": 2, "commitMessage": nil}))
	api.on(labelsKey+"2", 200, skillResponse(nil))
	versionResp := resource.UpdateResponse{State: testState(t, r, nil)}
	r.Update(ctx, resource.UpdateRequest{Plan: testPlan(t, r, plan), State: updateResp.State}, &versionResp)
	versionResp.State.Get(ctx, &state)
	if versionResp.Diagnostics.HasError() || state.Version.ValueInt64() != 2 || !state.CommitMessage.IsNull() {
		t.Fatalf("version update: %v %+v", versionResp.Diagnostics, state)
	}
	if !slices.Equal(api.calls, []string{"POST " + skillsPath, labelsKey + "2"}) {
		t.Fatalf("version update calls: %v", api.calls)
	}
	if _, ok := api.bodies["POST "+skillsPath]["commitMessage"]; ok {
		t.Fatal("unset commit message should be omitted")
	}

	// Delete removes every version, tolerating versions that are already gone.
	api.calls = nil
	api.on("GET "+skillsPath+"?name=my-skill", 200, map[string]any{"data": []any{map[string]any{"name": "my-skill", "latestVersion": 2}}})
	api.on("DELETE "+skillsPath+"/my-skill/versions/2", 200, map[string]any{"deleted": true})
	api.on("DELETE "+skillsPath+"/my-skill/versions/1", 404, map[string]any{"message": "Skill version not found"})
	deleteResp := resource.DeleteResponse{State: versionResp.State}
	r.Delete(ctx, resource.DeleteRequest{State: versionResp.State}, &deleteResp)
	if deleteResp.Diagnostics.HasError() || len(api.calls) != 3 {
		t.Fatalf("delete: %v %v", deleteResp.Diagnostics, api.calls)
	}

	api.on("GET "+skillsPath+"/my-skill?version=2&includeContent=true", 404, map[string]any{"message": "Skill version not found"})
	readResp = resource.ReadResponse{State: versionResp.State}
	r.Read(ctx, resource.ReadRequest{State: versionResp.State}, &readResp)
	if readResp.Diagnostics.HasError() || !readResp.State.Raw.IsNull() {
		t.Fatalf("read of a missing skill should remove it from state: %v", readResp.Diagnostics)
	}
}

func TestSkillResourceNameMismatch(t *testing.T) {
	ctx := context.Background()
	api := newFakeAPI(t)
	r := &skillResource{publicAPIResource{ClientFactory: api.factory()}}

	// A name that plan-time validation could not read still must not leave a stray version behind.
	api.on("POST "+skillsPath, 201, skillResponse(map[string]any{"name": "other", "version": 4}))
	api.on("DELETE "+skillsPath+"/other/versions/4", 200, map[string]any{"deleted": true})
	createResp := resource.CreateResponse{State: testState(t, r, nil)}
	r.Create(ctx, resource.CreateRequest{Plan: testPlan(t, r, testSkillPlan())}, &createResp)
	if !createResp.Diagnostics.HasError() || !createResp.State.Raw.IsNull() {
		t.Fatalf("create should fail without state: %v", createResp.Diagnostics)
	}
	if !slices.Contains(api.calls, "DELETE "+skillsPath+"/other/versions/4") {
		t.Fatalf("stray version should be deleted: %v", api.calls)
	}
}

func TestSkillResourceModifyPlan(t *testing.T) {
	ctx := context.Background()
	r := &skillResource{}
	prior := testSkillPlan()
	prior.ID, prior.Version, prior.Description = types.StringValue("my-skill"), types.Int64Value(3), types.StringValue("Does things")

	relabeled := testSkillPlan()
	relabeled.Labels = types.SetNull(types.StringType)
	changed := testSkillPlan()
	changed.Files = testSkillFiles(map[string]string{skillManifest: testSkillManifest})
	noManifest := testSkillPlan()
	noManifest.Files = testSkillFiles(map[string]string{"README.md": "x"})
	otherName := testSkillPlan()
	otherName.Name = types.StringValue("other")

	for name, tc := range map[string]struct {
		plan        skillResourceModel
		create      bool
		wantErr     bool
		wantVersion types.Int64
	}{
		"create":         {testSkillPlan(), true, false, types.Int64Unknown()},
		"labels only":    {relabeled, false, false, types.Int64Value(3)},
		"files changed":  {changed, false, false, types.Int64Unknown()},
		"no manifest":    {noManifest, true, true, types.Int64Unknown()},
		"name mismatch":  {otherName, true, true, types.Int64Unknown()},
		"update no file": {noManifest, false, true, types.Int64Unknown()},
	} {
		t.Run(name, func(t *testing.T) {
			req := resource.ModifyPlanRequest{State: testState(t, r, prior), Plan: testPlan(t, r, tc.plan)}
			if tc.create {
				req.State = testState(t, r, nil)
			}
			resp := resource.ModifyPlanResponse{Plan: req.Plan}
			r.ModifyPlan(ctx, req, &resp)
			if resp.Diagnostics.HasError() != tc.wantErr {
				t.Fatalf("wantErr=%v, got %v", tc.wantErr, resp.Diagnostics)
			}
			var planned skillResourceModel
			resp.Plan.Get(ctx, &planned)
			if !tc.wantErr && !planned.Version.Equal(tc.wantVersion) {
				t.Fatalf("version: got %v, want %v", planned.Version, tc.wantVersion)
			}
		})
	}
}

func TestSkillFrontmatterName(t *testing.T) {
	for manifest, want := range map[string]string{
		testSkillManifest: "my-skill",
		"---\r\ndescription: x\r\nname: \"quoted\"\r\n---\r\nbody": "quoted",
		"---\nname: 'single' # comment\n---\n":                     "single",
		"---\ndescription: x\n---\nname: body":                     "",
		"name: no-frontmatter":                                     "",
		"---\nname: unterminated":                                  "",
	} {
		if got := skillFrontmatterName(manifest); got != want {
			t.Errorf("skillFrontmatterName(%q) = %q, want %q", manifest, got, want)
		}
	}
}

func TestSkillResourceImport(t *testing.T) {
	ctx := context.Background()
	api := newFakeAPI(t)
	r := &skillResource{publicAPIResource{ClientFactory: api.factory()}}

	importResp := resource.ImportStateResponse{State: testState(t, r, nil)}
	r.ImportState(ctx, resource.ImportStateRequest{ID: "my-skill"}, &importResp)
	if !importResp.Diagnostics.HasError() {
		t.Fatal("import should reject an ID without credentials")
	}
	importResp = resource.ImportStateResponse{State: testState(t, r, nil)}
	r.ImportState(ctx, resource.ImportStateRequest{ID: "pk:sk:my-skill"}, &importResp)
	if importResp.Diagnostics.HasError() {
		t.Fatalf("import: %v", importResp.Diagnostics)
	}

	// Without a known version the latest one is adopted.
	api.on("GET "+skillsPath+"/my-skill?label=latest&includeContent=true", 200, skillResponse(map[string]any{
		"version": 5, "labels": []any{"latest", "production"},
	}))
	readResp := resource.ReadResponse{State: importResp.State}
	r.Read(ctx, resource.ReadRequest{State: importResp.State}, &readResp)
	var state skillResourceModel
	readResp.State.Get(ctx, &state)
	if readResp.Diagnostics.HasError() || state.Version.ValueInt64() != 5 || state.Name.ValueString() != "my-skill" ||
		!state.Labels.Equal(testStringSet("production")) || !state.Tags.IsNull() || len(state.Files.Elements()) != 2 {
		t.Fatalf("read after import: %v %+v", readResp.Diagnostics, state)
	}
}
