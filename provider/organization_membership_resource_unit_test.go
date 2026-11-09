package provider

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	resschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/langfuse/terraform-provider-langfuse/langfuse"
	"github.com/langfuse/terraform-provider-langfuse/langfuse/mocks"
)

func TestOrganizationMembershipResourceMetadata(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	r := NewOrganizationMembershipResource().(*organizationMembershipResource)

	var resp resource.MetadataResponse
	r.Metadata(ctx, resource.MetadataRequest{ProviderTypeName: "langfuse"}, &resp)

	expected := "langfuse_organization_membership"
	if resp.TypeName != expected {
		t.Fatalf("unexpected type name. got %q, want %q", resp.TypeName, expected)
	}
}

func TestOrganizationMembershipResourceSchema(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	r := NewOrganizationMembershipResource().(*organizationMembershipResource)

	var schemaResp resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &schemaResp)

	if schemaResp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics from Schema: %v", schemaResp.Diagnostics)
	}

	if diags := schemaResp.Schema.ValidateImplementation(ctx); diags.HasError() {
		t.Fatalf("schema implementation validation failed: %v", diags)
	}

	schema := schemaResp.Schema

	expectedAttributes := []string{
		"id", "email", "role", "status", "user_id", "username",
		"organization_public_key", "organization_private_key",
	}

	for _, expectedAttr := range expectedAttributes {
		if _, exists := schema.Attributes[expectedAttr]; !exists {
			t.Errorf("expected attribute %q not found in schema", expectedAttr)
		}
	}

	idAttr, ok := schema.Attributes["id"].(resschema.StringAttribute)
	if !ok || !idAttr.Computed {
		t.Fatal("'id' must be a computed string attribute")
	}
	if idAttr.Description != "The user ID of the organization membership." {
		t.Errorf("unexpected id description: %q", idAttr.Description)
	}

	for _, attributeName := range []string{"status", "username"} {
		attr, ok := schema.Attributes[attributeName].(resschema.StringAttribute)
		if !ok || attr.DeprecationMessage == "" {
			t.Errorf("%q must be a deprecated string attribute", attributeName)
		}
	}

	nameAttr, ok := schema.Attributes["name"].(resschema.StringAttribute)
	if !ok || nameAttr.Optional || !nameAttr.Computed {
		t.Fatal("'name' must be a computed-only string attribute")
	}
}

func TestOrganizationMembershipResourceReadPreservesLegacyIDAndEmptyDeprecatedFields(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ctrl := gomock.NewController(t)
	clientFactory := mocks.NewMockClientFactory(ctrl)
	r := NewOrganizationMembershipResource().(*organizationMembershipResource)
	r.ClientFactory = clientFactory

	var schemaResp resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &schemaResp)
	if schemaResp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics from Schema: %v", schemaResp.Diagnostics)
	}

	const userID = "user-123"
	const legacyID = "membership-456"
	clientFactory.OrganizationClient.EXPECT().
		GetMembership(ctx, legacyID).
		Return(&langfuse.OrganizationMembership{
			ID:       "membership-456",
			UserID:   userID,
			Email:    "person@example.com",
			Role:     "MEMBER",
			Status:   "",
			Username: "",
			Name:     "Person Example",
		}, nil)

	priorState := tfsdk.State{Schema: schemaResp.Schema}
	priorStateDiags := priorState.Set(ctx, organizationMembershipResourceModel{
		ID:                     types.StringValue(legacyID),
		Email:                  types.StringValue("person@example.com"),
		Role:                   types.StringValue("MEMBER"),
		Status:                 types.StringValue(""),
		UserID:                 types.StringValue(userID),
		Username:               types.StringValue(""),
		OrganizationPublicKey:  types.StringValue("public"),
		OrganizationPrivateKey: types.StringValue("private"),
	})
	if priorStateDiags.HasError() {
		t.Fatalf("failed to create prior state: %v", priorStateDiags)
	}

	resp := resource.ReadResponse{State: tfsdk.State{Schema: schemaResp.Schema}}
	r.Read(ctx, resource.ReadRequest{State: priorState}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics from Read: %v", resp.Diagnostics)
	}

	var state organizationMembershipResourceModel
	stateDiags := resp.State.Get(ctx, &state)
	if stateDiags.HasError() {
		t.Fatalf("failed to read resulting state: %v", stateDiags)
	}
	if state.ID.ValueString() != legacyID {
		t.Errorf("read must preserve legacy id %q, got %q", legacyID, state.ID.ValueString())
	}
	if state.UserID.ValueString() != userID {
		t.Errorf("unexpected user_id: got %q, want %q", state.UserID.ValueString(), userID)
	}
	if state.Name.ValueString() != "Person Example" {
		t.Errorf("unexpected name: %q", state.Name.ValueString())
	}
	for _, attr := range []struct {
		name  string
		value types.String
	}{
		{name: "status", value: state.Status},
		{name: "username", value: state.Username},
	} {
		if attr.value.IsNull() || attr.value.ValueString() != "" {
			t.Errorf("%s must remain a non-null empty string, got %#v", attr.name, attr.value)
		}
	}
}

func TestOrganizationMembershipResourceCreateUsesUserIDAsResourceID(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ctrl := gomock.NewController(t)
	clientFactory := mocks.NewMockClientFactory(ctrl)
	r := NewOrganizationMembershipResource().(*organizationMembershipResource)
	r.ClientFactory = clientFactory

	var schemaResp resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &schemaResp)
	if schemaResp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics from Schema: %v", schemaResp.Diagnostics)
	}

	const (
		email  = "person@example.com"
		userID = "user-123"
	)
	clientFactory.OrganizationClient.EXPECT().
		ListMemberships(ctx).
		Return([]langfuse.OrganizationMembership{
			{ID: "membership-456", UserID: userID, Email: email},
		}, nil)
	clientFactory.OrganizationClient.EXPECT().
		UpdateMembership(ctx, "membership-456", &langfuse.UpdateMembershipRequest{
			UserID: userID,
			Role:   "MEMBER",
		}).
		Return(&langfuse.OrganizationMembership{
			ID:       "membership-456",
			UserID:   userID,
			Email:    email,
			Role:     "MEMBER",
			Status:   "",
			Username: "",
			Name:     "Person Example",
		}, nil)

	planValue := map[string]tftypes.Value{
		"id":                       tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
		"email":                    tftypes.NewValue(tftypes.String, email),
		"role":                     tftypes.NewValue(tftypes.String, "MEMBER"),
		"status":                   tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
		"user_id":                  tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
		"username":                 tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
		"name":                     tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
		"organization_public_key":  tftypes.NewValue(tftypes.String, "public"),
		"organization_private_key": tftypes.NewValue(tftypes.String, "private"),
	}
	req := resource.CreateRequest{
		Plan: tfsdk.Plan{
			Schema: schemaResp.Schema,
			Raw:    tftypes.NewValue(schemaResp.Schema.Type().TerraformType(ctx), planValue),
		},
	}
	resp := resource.CreateResponse{State: tfsdk.State{Schema: schemaResp.Schema}}

	r.Create(ctx, req, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics from Create: %v", resp.Diagnostics)
	}

	var state organizationMembershipResourceModel
	stateDiags := resp.State.Get(ctx, &state)
	if stateDiags.HasError() {
		t.Fatalf("failed to read resulting state: %v", stateDiags)
	}
	if state.ID.ValueString() != userID || state.UserID.ValueString() != userID {
		t.Errorf("id and user_id must both remain the user ID; got id=%q user_id=%q", state.ID.ValueString(), state.UserID.ValueString())
	}
	if state.Name.ValueString() != "Person Example" {
		t.Errorf("unexpected name: %q", state.Name.ValueString())
	}
	if state.Status.IsNull() || state.Status.ValueString() != "" {
		t.Errorf("status must remain a non-null empty string, got %#v", state.Status)
	}
	if state.Username.IsNull() || state.Username.ValueString() != "" {
		t.Errorf("username must remain a non-null empty string, got %#v", state.Username)
	}
}

func TestOrganizationMembershipResourceUpdatePreservesLegacyResourceID(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ctrl := gomock.NewController(t)
	clientFactory := mocks.NewMockClientFactory(ctrl)
	r := NewOrganizationMembershipResource().(*organizationMembershipResource)
	r.ClientFactory = clientFactory

	var schemaResp resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &schemaResp)
	if schemaResp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics from Schema: %v", schemaResp.Diagnostics)
	}

	const userID = "user-123"
	const legacyID = "membership-456"
	clientFactory.OrganizationClient.EXPECT().
		UpdateMembership(ctx, legacyID, &langfuse.UpdateMembershipRequest{Role: "ADMIN"}).
		Return(&langfuse.OrganizationMembership{
			ID:       "membership-456",
			UserID:   userID,
			Email:    "person@example.com",
			Role:     "ADMIN",
			Status:   "",
			Username: "",
			Name:     "Person Example",
		}, nil)

	stateValue := map[string]tftypes.Value{
		"id":                       tftypes.NewValue(tftypes.String, legacyID),
		"email":                    tftypes.NewValue(tftypes.String, "person@example.com"),
		"role":                     tftypes.NewValue(tftypes.String, "MEMBER"),
		"status":                   tftypes.NewValue(tftypes.String, ""),
		"user_id":                  tftypes.NewValue(tftypes.String, userID),
		"username":                 tftypes.NewValue(tftypes.String, ""),
		"name":                     tftypes.NewValue(tftypes.String, "Person Example"),
		"organization_public_key":  tftypes.NewValue(tftypes.String, "public"),
		"organization_private_key": tftypes.NewValue(tftypes.String, "private"),
	}
	planValue := map[string]tftypes.Value{
		"id":                       tftypes.NewValue(tftypes.String, userID),
		"email":                    tftypes.NewValue(tftypes.String, "person@example.com"),
		"role":                     tftypes.NewValue(tftypes.String, "ADMIN"),
		"status":                   tftypes.NewValue(tftypes.String, ""),
		"user_id":                  tftypes.NewValue(tftypes.String, userID),
		"username":                 tftypes.NewValue(tftypes.String, ""),
		"name":                     tftypes.NewValue(tftypes.String, "Person Example"),
		"organization_public_key":  tftypes.NewValue(tftypes.String, "public"),
		"organization_private_key": tftypes.NewValue(tftypes.String, "private"),
	}
	objectType := schemaResp.Schema.Type().TerraformType(ctx)
	req := resource.UpdateRequest{
		Plan: tfsdk.Plan{
			Schema: schemaResp.Schema,
			Raw:    tftypes.NewValue(objectType, planValue),
		},
		State: tfsdk.State{
			Schema: schemaResp.Schema,
			Raw:    tftypes.NewValue(objectType, stateValue),
		},
	}
	resp := resource.UpdateResponse{State: tfsdk.State{Schema: schemaResp.Schema}}

	r.Update(ctx, req, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics from Update: %v", resp.Diagnostics)
	}

	var state organizationMembershipResourceModel
	stateDiags := resp.State.Get(ctx, &state)
	if stateDiags.HasError() {
		t.Fatalf("failed to read resulting state: %v", stateDiags)
	}
	if state.ID.ValueString() != legacyID {
		t.Errorf("update must preserve legacy id %q, got %q", legacyID, state.ID.ValueString())
	}
	if state.UserID.ValueString() != userID {
		t.Errorf("unexpected user_id: got %q, want %q", state.UserID.ValueString(), userID)
	}
	if state.Name.ValueString() != "Person Example" {
		t.Errorf("unexpected name: %q", state.Name.ValueString())
	}
}

func TestOrganizationMembershipResource_Create_InvalidRole(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Create resource
	r := NewOrganizationMembershipResource().(*organizationMembershipResource)

	// Create request
	req := resource.CreateRequest{}
	resp := &resource.CreateResponse{}

	// Set up plan data with invalid role
	planValue := map[string]tftypes.Value{
		"id":                       tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
		"email":                    tftypes.NewValue(tftypes.String, "test@example.com"),
		"role":                     tftypes.NewValue(tftypes.String, "INVALID_ROLE"),
		"status":                   tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
		"user_id":                  tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
		"username":                 tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
		"name":                     tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
		"organization_public_key":  tftypes.NewValue(tftypes.String, "test-public"),
		"organization_private_key": tftypes.NewValue(tftypes.String, "test-private"),
	}

	schemaResp := resource.SchemaResponse{}
	r.Schema(ctx, resource.SchemaRequest{}, &schemaResp)

	req.Plan = tfsdk.Plan{
		Schema: schemaResp.Schema,
		Raw:    tftypes.NewValue(schemaResp.Schema.Type().TerraformType(ctx), planValue),
	}

	// Call Create
	r.Create(ctx, req, resp)

	// Assert error occurred
	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for invalid role, but got none")
	}

	errorSummary := resp.Diagnostics.Errors()[0].Summary()
	if errorSummary != "Invalid Role" {
		t.Fatalf("unexpected error summary. got %q, want %q", errorSummary, "Invalid Role")
	}
}

func TestOrganizationMembershipResource_Update_InvalidRole(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Create resource
	r := NewOrganizationMembershipResource().(*organizationMembershipResource)

	// Create request
	req := resource.UpdateRequest{}
	resp := &resource.UpdateResponse{}

	// Set up plan data with invalid role
	planValue := map[string]tftypes.Value{
		"id":                       tftypes.NewValue(tftypes.String, "membership-123"),
		"email":                    tftypes.NewValue(tftypes.String, "test@example.com"),
		"role":                     tftypes.NewValue(tftypes.String, "SUPER_ADMIN"),
		"status":                   tftypes.NewValue(tftypes.String, "ACTIVE"),
		"user_id":                  tftypes.NewValue(tftypes.String, "user-123"),
		"username":                 tftypes.NewValue(tftypes.String, "testuser"),
		"name":                     tftypes.NewValue(tftypes.String, "Test User"),
		"organization_public_key":  tftypes.NewValue(tftypes.String, "test-public"),
		"organization_private_key": tftypes.NewValue(tftypes.String, "test-private"),
	}

	stateValue := map[string]tftypes.Value{
		"id":                       tftypes.NewValue(tftypes.String, "membership-123"),
		"email":                    tftypes.NewValue(tftypes.String, "test@example.com"),
		"role":                     tftypes.NewValue(tftypes.String, "MEMBER"),
		"status":                   tftypes.NewValue(tftypes.String, "ACTIVE"),
		"user_id":                  tftypes.NewValue(tftypes.String, "user-123"),
		"username":                 tftypes.NewValue(tftypes.String, "testuser"),
		"name":                     tftypes.NewValue(tftypes.String, "Test User"),
		"organization_public_key":  tftypes.NewValue(tftypes.String, "test-public"),
		"organization_private_key": tftypes.NewValue(tftypes.String, "test-private"),
	}

	schemaResp := resource.SchemaResponse{}
	r.Schema(ctx, resource.SchemaRequest{}, &schemaResp)

	req.Plan = tfsdk.Plan{
		Schema: schemaResp.Schema,
		Raw:    tftypes.NewValue(schemaResp.Schema.Type().TerraformType(ctx), planValue),
	}

	req.State = tfsdk.State{
		Schema: schemaResp.Schema,
		Raw:    tftypes.NewValue(schemaResp.Schema.Type().TerraformType(ctx), stateValue),
	}

	// Call Update
	r.Update(ctx, req, resp)

	// Assert error occurred
	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for invalid role, but got none")
	}

	errorSummary := resp.Diagnostics.Errors()[0].Summary()
	if errorSummary != "Invalid Role" {
		t.Fatalf("unexpected error summary. got %q, want %q", errorSummary, "Invalid Role")
	}
}
