package provider

import (
	"context"
	"errors"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/langfuse/terraform-provider-langfuse/langfuse"
	"github.com/langfuse/terraform-provider-langfuse/langfuse/mocks"
)

func TestOrganizationMembershipAuthenticatedImport(t *testing.T) {
	for _, tc := range []struct {
		name, id string
		apiError bool
		wantErr  bool
	}{
		{"complete credentials", "user,public,private", false, false},
		{"missing credential", "user,public,", false, true},
		{"invalid format", "user,public", false, true},
		{"missing membership", "user,public,private", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			ctrl := gomock.NewController(t)
			factory := mocks.NewMockClientFactory(ctrl)
			client := factory.OrganizationClient
			r := &organizationMembershipResource{ClientFactory: factory}
			schema := resource.SchemaResponse{}
			r.Schema(ctx, resource.SchemaRequest{}, &schema)
			if tc.id == "user,public,private" {
				if tc.apiError {
					client.EXPECT().GetMembership(ctx, "user").Return(nil, errors.New("membership unavailable"))
				} else {
					client.EXPECT().GetMembership(ctx, "user").Return(&langfuse.OrganizationMembership{
						UserID: "user", Email: "user@example.com", Role: "MEMBER", Name: "User",
					}, nil)
				}
			}
			resp := resource.ImportStateResponse{State: tfsdk.State{Schema: schema.Schema}}
			r.ImportState(ctx, resource.ImportStateRequest{ID: tc.id}, &resp)
			if resp.Diagnostics.HasError() != tc.wantErr {
				t.Fatalf("diagnostics = %v", resp.Diagnostics)
			}
			if tc.wantErr {
				return
			}
			var got organizationMembershipResourceModel
			if diags := resp.State.Get(ctx, &got); diags.HasError() {
				t.Fatal(diags)
			}
			if got.ID.ValueString() != "user" || got.UserID.ValueString() != "user" ||
				got.Name.ValueString() != "User" || got.OrganizationPublicKey.ValueString() != "public" ||
				got.OrganizationPrivateKey.ValueString() != "private" {
				t.Fatalf("imported identity or credentials missing")
			}
		})
	}
}
