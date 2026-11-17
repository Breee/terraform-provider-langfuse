package provider

import (
	"context"
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework-timetypes/timetypes"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

const blobStoragePath = "/api/public/integrations/blob-storage"

func testBlobStorageIntegrationPlan() blobStorageIntegrationResourceModel {
	return blobStorageIntegrationResourceModel{
		ID:                     types.StringUnknown(),
		OrganizationPublicKey:  types.StringValue("pk"),
		OrganizationPrivateKey: types.StringValue("sk"),
		ProjectID:              types.StringValue("proj1"),
		Type:                   types.StringValue("S3"),
		BucketName:             types.StringValue("exports"),
		Endpoint:               types.StringNull(),
		Region:                 types.StringValue("eu-central-1"),
		AuthType:               types.StringValue("ACCESS_KEYS"),
		RoleArn:                types.StringNull(),
		AccessKeyID:            types.StringValue("key-id"),
		SecretAccessKey:        types.StringValue("key-secret"),
		Prefix:                 types.StringValue(""),
		ExportFrequency:        types.StringValue("daily"),
		Enabled:                types.BoolValue(true),
		ForcePathStyle:         types.BoolValue(false),
		FileType:               types.StringValue("JSONL"),
		ExportMode:             types.StringValue("FULL_HISTORY"),
		ExportStartDate:        timetypes.NewRFC3339Null(),
		Compressed:             types.BoolValue(true),
		ExportSource:           types.StringUnknown(),
		ExportFieldGroups:      types.SetUnknown(types.StringType),
		ExternalID:             types.StringUnknown(),
	}
}

func blobStorageIntegrationResponse(overrides map[string]any) map[string]any {
	response := map[string]any{
		"id": "proj1", "projectId": "proj1", "type": "S3", "bucketName": "exports", "endpoint": nil,
		"region": "eu-central-1", "authType": "ACCESS_KEYS", "accessKeyId": "key-id", "roleArn": nil,
		"externalId": nil, "prefix": "", "exportFrequency": "daily", "enabled": true, "forcePathStyle": false,
		"fileType": "JSONL", "exportMode": "FULL_HISTORY", "exportStartDate": nil, "compressed": true,
		"exportSource": "OBSERVATIONS_V2", "exportFieldGroups": []string{"core", "io"},
	}
	for k, v := range overrides {
		response[k] = v
	}
	return response
}

func TestBlobStorageIntegrationResourceLifecycle(t *testing.T) {
	ctx := context.Background()
	api := newFakeAPI(t)
	r := &blobStorageIntegrationResource{publicAPIResource{ClientFactory: api.factory()}}

	api.on("PUT "+blobStoragePath, 200, blobStorageIntegrationResponse(nil))
	createResp := resource.CreateResponse{State: testState(t, r, nil)}
	r.Create(ctx, resource.CreateRequest{Plan: testPlan(t, r, testBlobStorageIntegrationPlan())}, &createResp)
	if createResp.Diagnostics.HasError() {
		t.Fatalf("create: %v", createResp.Diagnostics)
	}
	var state blobStorageIntegrationResourceModel
	createResp.State.Get(ctx, &state)
	if state.ID.ValueString() != "proj1" || state.ExportSource.ValueString() != "OBSERVATIONS_V2" ||
		len(state.ExportFieldGroups.Elements()) != 2 || state.SecretAccessKey.ValueString() != "key-secret" ||
		!state.ExternalID.IsNull() {
		t.Fatalf("unexpected state after create: %+v", state)
	}
	want := map[string]any{
		"projectId": "proj1", "type": "S3", "bucketName": "exports", "endpoint": nil, "region": "eu-central-1",
		"authType": "ACCESS_KEYS", "roleArn": nil, "accessKeyId": "key-id", "secretAccessKey": "key-secret",
		"prefix": "", "exportFrequency": "daily", "enabled": true, "forcePathStyle": false, "fileType": "JSONL",
		"exportMode": "FULL_HISTORY", "exportStartDate": nil, "compressed": true,
	}
	if got := api.bodies["PUT "+blobStoragePath]; !reflect.DeepEqual(got, want) {
		t.Fatalf("create body\n got: %v\nwant: %v", got, want)
	}

	// Read finds the integration in the organization-wide list and picks up drift.
	api.on("GET "+blobStoragePath, 200, map[string]any{"data": []any{
		blobStorageIntegrationResponse(map[string]any{"id": "other", "projectId": "other"}),
		blobStorageIntegrationResponse(map[string]any{"exportFrequency": "weekly", "exportStartDate": "2026-01-01T00:00:00.000Z"}),
	}})
	readResp := resource.ReadResponse{State: createResp.State}
	r.Read(ctx, resource.ReadRequest{State: createResp.State}, &readResp)
	if readResp.Diagnostics.HasError() {
		t.Fatalf("read: %v", readResp.Diagnostics)
	}
	readResp.State.Get(ctx, &state)
	if state.ExportFrequency.ValueString() != "weekly" || state.ExportStartDate.ValueString() != "2026-01-01T00:00:00.000Z" ||
		state.SecretAccessKey.ValueString() != "key-secret" {
		t.Fatalf("unexpected state after read: %+v", state)
	}

	// Update sends the configured export source and field groups and omits an unset secret.
	plan := testBlobStorageIntegrationPlan()
	plan.AuthType = types.StringValue("ROLE_ARN")
	plan.RoleArn = types.StringValue("arn:aws:iam::123456789012:role/export")
	plan.AccessKeyID = types.StringNull()
	plan.SecretAccessKey = types.StringNull()
	plan.ExportSource = types.StringValue("OBSERVATIONS_V2")
	plan.ExportFieldGroups = testStringSet("core")
	api.on("PUT "+blobStoragePath, 200, blobStorageIntegrationResponse(map[string]any{
		"authType": "ROLE_ARN", "roleArn": "arn:aws:iam::123456789012:role/export", "accessKeyId": nil,
		"externalId": "ext-1", "exportFieldGroups": []string{"core"},
	}))
	updateResp := resource.UpdateResponse{State: readResp.State}
	r.Update(ctx, resource.UpdateRequest{Plan: testPlan(t, r, plan), State: readResp.State}, &updateResp)
	if updateResp.Diagnostics.HasError() {
		t.Fatalf("update: %v", updateResp.Diagnostics)
	}
	updateResp.State.Get(ctx, &state)
	if state.ExternalID.ValueString() != "ext-1" || !state.AccessKeyID.IsNull() || !state.SecretAccessKey.IsNull() {
		t.Fatalf("unexpected state after update: %+v", state)
	}
	body := api.bodies["PUT "+blobStoragePath]
	if _, sent := body["secretAccessKey"]; sent || body["exportSource"] != "OBSERVATIONS_V2" ||
		!reflect.DeepEqual(body["exportFieldGroups"], []any{"core"}) || body["accessKeyId"] != nil {
		t.Fatalf("unexpected update body: %v", body)
	}

	// Delete tolerates an integration that is already gone.
	api.on("DELETE "+blobStoragePath+"/proj1", 404, map[string]any{"message": "not found"})
	deleteResp := resource.DeleteResponse{State: updateResp.State}
	r.Delete(ctx, resource.DeleteRequest{State: updateResp.State}, &deleteResp)
	if deleteResp.Diagnostics.HasError() {
		t.Fatalf("delete: %v", deleteResp.Diagnostics)
	}

	// Read removes the resource when the list no longer contains it.
	api.on("GET "+blobStoragePath, 200, map[string]any{"data": []any{}})
	goneResp := resource.ReadResponse{State: updateResp.State}
	r.Read(ctx, resource.ReadRequest{State: updateResp.State}, &goneResp)
	if !goneResp.State.Raw.IsNull() {
		t.Fatal("expected the resource to be removed from state")
	}
}

func TestBlobStorageIntegrationResourceImport(t *testing.T) {
	ctx := context.Background()
	r := &blobStorageIntegrationResource{}

	resp := resource.ImportStateResponse{State: testState(t, r, nil)}
	r.ImportState(ctx, resource.ImportStateRequest{ID: "pk:sk:proj1"}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("import: %v", resp.Diagnostics)
	}
	var id, publicKey types.String
	resp.State.GetAttribute(ctx, path.Root("id"), &id)
	resp.State.GetAttribute(ctx, path.Root("organization_public_key"), &publicKey)
	if id.ValueString() != "proj1" || publicKey.ValueString() != "pk" {
		t.Fatalf("unexpected import state: %s %s", id, publicKey)
	}

	bad := resource.ImportStateResponse{State: testState(t, r, nil)}
	r.ImportState(ctx, resource.ImportStateRequest{ID: "proj1"}, &bad)
	if !bad.Diagnostics.HasError() {
		t.Fatal("expected an error for a malformed import ID")
	}
}
