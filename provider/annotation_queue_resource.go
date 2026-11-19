package provider

import (
	"context"
	"errors"

	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &annotationQueueResource{}
var _ resource.ResourceWithImportState = &annotationQueueResource{}
var _ resource.ResourceWithModifyPlan = &annotationQueueResource{}

func NewAnnotationQueueResource() resource.Resource {
	return &annotationQueueResource{}
}

type annotationQueueResource struct {
	publicAPIResource
}

type annotationQueueResourceModel struct {
	ID               types.String `tfsdk:"id"`
	ProjectPublicKey types.String `tfsdk:"project_public_key"`
	ProjectSecretKey types.String `tfsdk:"project_secret_key"`
	Name             types.String `tfsdk:"name"`
	Description      types.String `tfsdk:"description"`
	ScoreConfigIDs   types.Set    `tfsdk:"score_config_ids"`
}

type annotationQueueAPI struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Description    *string  `json:"description"`
	ScoreConfigIDs []string `json:"scoreConfigIds"`
}

func (r *annotationQueueResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_annotation_queue"
}

const (
	annotationQueueDescription = "Manage an annotation queue in a project using project-scoped credentials. " +
		"Import format: project_public_key:project_secret_key:annotation_queue_id."
	annotationQueueNote = "Annotation queues can only be created and read through the Langfuse public API; they cannot be updated or deleted. " +
		"Every change therefore forces a new queue, and because queue names are unique and the previous queue cannot be deleted, a change must also rename the queue (the plan fails otherwise). " +
		"Destroying this resource removes it from the Terraform state with a warning and leaves the queue and its items in Langfuse; delete it in the Langfuse UI."
)

func (r *annotationQueueResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description:         annotationQueueDescription + " Note: " + annotationQueueNote,
		MarkdownDescription: annotationQueueDescription + noteCallout(annotationQueueNote),
		Attributes: withProjectCredentials(map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "The unique identifier of the annotation queue.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Name of the annotation queue, unique within the project. Changing it forces a new queue; the previous one is left in Langfuse.",
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"description": schema.StringAttribute{
				Optional:    true,
				Description: "Description of the annotation queue. Changing it forces a new queue and requires a new name.",
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"score_config_ids": schema.SetAttribute{
				Required:    true,
				ElementType: types.StringType,
				Description: "IDs of the score configs annotators fill in for each item. At least one is required. Changing them forces a new queue and requires a new name.",
				Validators: []validator.Set{
					setvalidator.SizeAtLeast(1),
				},
				PlanModifiers: []planmodifier.Set{
					setplanmodifier.RequiresReplace(),
				},
			},
		}),
	}
}

// ModifyPlan rejects a replacement that keeps the name: the previous queue
// cannot be deleted, so creating its successor would fail on the unique name.
func (r *annotationQueueResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.State.Raw.IsNull() || req.Plan.Raw.IsNull() {
		return
	}
	var state, plan annotationQueueResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() || plan.Name.IsUnknown() || !plan.Name.Equal(state.Name) {
		return
	}
	if plan.Description.IsUnknown() || plan.ScoreConfigIDs.IsUnknown() {
		return
	}
	if !plan.Description.Equal(state.Description) || !plan.ScoreConfigIDs.Equal(state.ScoreConfigIDs) {
		resp.Diagnostics.AddAttributeError(
			path.Root("name"),
			"Annotation queue cannot be changed in place",
			"The Langfuse API can neither update nor delete annotation queues. Changing description or score_config_ids creates a new queue, "+
				"which needs a new name because the existing queue keeps the current one.",
		)
	}
}

// annotationQueueState maps an API queue onto prior (the plan or previous
// state), which supplies the credentials.
func annotationQueueState(ctx context.Context, q annotationQueueAPI, prior annotationQueueResourceModel) (annotationQueueResourceModel, diag.Diagnostics) {
	ids, diags := types.SetValueFrom(ctx, types.StringType, q.ScoreConfigIDs)
	return annotationQueueResourceModel{
		ID:               types.StringValue(q.ID),
		ProjectPublicKey: prior.ProjectPublicKey,
		ProjectSecretKey: prior.ProjectSecretKey,
		Name:             types.StringValue(q.Name),
		Description:      stringOrNull(q.Description),
		ScoreConfigIDs:   ids,
	}, diags
}

func (r *annotationQueueResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan annotationQueueResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var ids []string
	resp.Diagnostics.Append(plan.ScoreConfigIDs.ElementsAs(ctx, &ids, false)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body := map[string]any{"name": plan.Name.ValueString(), "scoreConfigIds": ids}
	if !plan.Description.IsNull() {
		body["description"] = plan.Description.ValueString()
	}
	reader, err := jsonBody(body)
	if err != nil {
		resp.Diagnostics.AddError("Error building annotation queue request", err.Error())
		return
	}
	var queue annotationQueueAPI
	if err := apiCall(r.client(plan.ProjectPublicKey, plan.ProjectSecretKey).AnnotationQueuesCreateQueueWithBody(ctx, jsonContentType, reader)).decode(&queue); err != nil {
		resp.Diagnostics.AddError("Error creating annotation queue", err.Error())
		return
	}
	state, diags := annotationQueueState(ctx, queue, plan)
	resp.Diagnostics.Append(diags...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *annotationQueueResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state annotationQueueResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var queue annotationQueueAPI
	err := apiCall(r.client(state.ProjectPublicKey, state.ProjectSecretKey).AnnotationQueuesGetQueue(ctx, state.ID.ValueString())).decode(&queue)
	if errors.Is(err, errNotFound) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Error reading annotation queue", err.Error())
		return
	}
	newState, diags := annotationQueueState(ctx, queue, state)
	resp.Diagnostics.Append(diags...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

// Update only stores new credentials; every other change forces replacement.
func (r *annotationQueueResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan annotationQueueResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete only removes the queue from state because the API has no delete endpoint.
func (r *annotationQueueResource) Delete(_ context.Context, _ resource.DeleteRequest, resp *resource.DeleteResponse) {
	resp.Diagnostics.AddWarning(
		"Annotation queue left in Langfuse",
		"The Langfuse API has no delete endpoint for annotation queues. The queue was removed from Terraform state but still exists in Langfuse; delete it in the Langfuse UI if it is no longer needed.",
	)
}

func (r *annotationQueueResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	publicKey, secretKey, id, err := parseCredentialImportID(req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project_public_key"), publicKey)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project_secret_key"), secretKey)...)
}
