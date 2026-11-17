package provider

import (
	"context"
	"errors"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &annotationQueueAssignmentResource{}

func NewAnnotationQueueAssignmentResource() resource.Resource {
	return &annotationQueueAssignmentResource{}
}

type annotationQueueAssignmentResource struct {
	publicAPIResource
}

type annotationQueueAssignmentResourceModel struct {
	ID               types.String `tfsdk:"id"`
	ProjectPublicKey types.String `tfsdk:"project_public_key"`
	ProjectSecretKey types.String `tfsdk:"project_secret_key"`
	QueueID          types.String `tfsdk:"queue_id"`
	UserID           types.String `tfsdk:"user_id"`
}

func (r *annotationQueueAssignmentResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_annotation_queue_assignment"
}

func (r *annotationQueueAssignmentResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Assign a user to an annotation queue using project-scoped credentials. " +
			"The user must have access to the project, for example through an organization membership with a role other than NONE. " +
			"The Langfuse API can only create and delete assignments: there is no endpoint to read them, " +
			"so an assignment removed outside Terraform is not detected and the resource cannot be imported.",
		Attributes: withProjectCredentials(map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Identifier of the assignment in the form queue_id/user_id.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"queue_id": schema.StringAttribute{
				Required:    true,
				Description: "ID of the annotation queue. Changing it forces a new assignment.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"user_id": schema.StringAttribute{
				Required:    true,
				Description: "ID of the user to assign. Changing it forces a new assignment.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
		}),
	}
}

func (r *annotationQueueAssignmentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan annotationQueueAssignmentResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	reader, err := jsonBody(map[string]any{"userId": plan.UserID.ValueString()})
	if err != nil {
		resp.Diagnostics.AddError("Error building annotation queue assignment request", err.Error())
		return
	}
	client := r.client(plan.ProjectPublicKey, plan.ProjectSecretKey)
	if err := apiCall(client.AnnotationQueuesCreateQueueAssignmentWithBody(ctx, plan.QueueID.ValueString(), jsonContentType, reader)).decode(nil); err != nil {
		resp.Diagnostics.AddError("Error creating annotation queue assignment", err.Error())
		return
	}
	plan.ID = types.StringValue(plan.QueueID.ValueString() + "/" + plan.UserID.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Read keeps the state as is because the API has no endpoint to read assignments.
func (r *annotationQueueAssignmentResource) Read(_ context.Context, _ resource.ReadRequest, _ *resource.ReadResponse) {
}

// Update only stores new credentials; every other change forces replacement.
func (r *annotationQueueAssignmentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan annotationQueueAssignmentResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *annotationQueueAssignmentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state annotationQueueAssignmentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	reader, err := jsonBody(map[string]any{"userId": state.UserID.ValueString()})
	if err != nil {
		resp.Diagnostics.AddError("Error building annotation queue assignment request", err.Error())
		return
	}
	client := r.client(state.ProjectPublicKey, state.ProjectSecretKey)
	err = apiCall(client.AnnotationQueuesDeleteQueueAssignmentWithBody(ctx, state.QueueID.ValueString(), jsonContentType, reader)).decode(nil)
	if err != nil && !errors.Is(err, errNotFound) {
		resp.Diagnostics.AddError("Error deleting annotation queue assignment", err.Error())
	}
}
