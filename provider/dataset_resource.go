package provider

import (
	"context"
	"errors"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &datasetResource{}
var _ resource.ResourceWithImportState = &datasetResource{}

func NewDatasetResource() resource.Resource {
	return &datasetResource{}
}

type datasetResource struct {
	publicAPIResource
}

type datasetResourceModel struct {
	ID                   types.String         `tfsdk:"id"`
	ProjectPublicKey     types.String         `tfsdk:"project_public_key"`
	ProjectSecretKey     types.String         `tfsdk:"project_secret_key"`
	Name                 types.String         `tfsdk:"name"`
	Description          types.String         `tfsdk:"description"`
	Metadata             jsontypes.Normalized `tfsdk:"metadata"`
	InputSchema          jsontypes.Normalized `tfsdk:"input_schema"`
	ExpectedOutputSchema jsontypes.Normalized `tfsdk:"expected_output_schema"`
}

type datasetAPI struct {
	ID                   string  `json:"id"`
	Name                 string  `json:"name"`
	Description          *string `json:"description"`
	Metadata             any     `json:"metadata"`
	InputSchema          any     `json:"inputSchema"`
	ExpectedOutputSchema any     `json:"expectedOutputSchema"`
}

func (r *datasetResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dataset"
}

func (r *datasetResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manage a dataset in a project using project-scoped credentials. " +
			"The Langfuse API creates or updates a dataset by name, so applying this resource adopts an existing dataset with the same name. " +
			"The API has no delete endpoint for datasets: destroying this resource only removes it from state and leaves the dataset, its items, and its runs in Langfuse. " +
			"Import format: project_public_key:project_secret_key:dataset_name.",
		Attributes: withProjectCredentials(map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "The unique identifier of the dataset.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Name of the dataset, unique within the project. Changing it forces a new dataset; the previous one is left in Langfuse.",
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"description": schema.StringAttribute{
				Optional:    true,
				Description: "Description of the dataset.",
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"metadata": schema.StringAttribute{
				Optional:    true,
				CustomType:  jsontypes.NormalizedType{},
				Description: "Metadata of the dataset as a JSON string. Removing it resets the metadata to an empty object because the API cannot clear it.",
			},
			"input_schema": schema.StringAttribute{
				Optional:    true,
				CustomType:  jsontypes.NormalizedType{},
				Description: "JSON Schema, as a JSON string, that the input of every dataset item is validated against.",
			},
			"expected_output_schema": schema.StringAttribute{
				Optional:    true,
				CustomType:  jsontypes.NormalizedType{},
				Description: "JSON Schema, as a JSON string, that the expected output of every dataset item is validated against.",
			},
		}),
	}
}

// datasetRequestBody builds the upsert body. The API ignores a null
// description or metadata, so an update clears them with empty values instead.
func datasetRequestBody(plan datasetResourceModel, update bool) (map[string]any, error) {
	body := map[string]any{"name": plan.Name.ValueString()}
	if !plan.Description.IsNull() || update {
		body["description"] = plan.Description.ValueString()
	}
	metadata, err := jsonAttr(plan.Metadata)
	if err != nil {
		return nil, err
	}
	if metadata != nil {
		body["metadata"] = metadata
	} else if update {
		body["metadata"] = map[string]any{}
	}
	if body["inputSchema"], err = jsonAttr(plan.InputSchema); err != nil {
		return nil, err
	}
	if body["expectedOutputSchema"], err = jsonAttr(plan.ExpectedOutputSchema); err != nil {
		return nil, err
	}
	return body, nil
}

// datasetState maps an API dataset onto prior (the plan or previous state),
// which supplies the credentials.
func datasetState(d datasetAPI, prior datasetResourceModel) (datasetResourceModel, diag.Diagnostics) {
	var diags diag.Diagnostics
	state := datasetResourceModel{
		ID:               types.StringValue(d.ID),
		ProjectPublicKey: prior.ProjectPublicKey,
		ProjectSecretKey: prior.ProjectSecretKey,
		Name:             types.StringValue(d.Name),
		Description:      types.StringNull(),
	}
	if d.Description != nil && *d.Description != "" {
		state.Description = types.StringValue(*d.Description)
	}
	// Cleared metadata is stored as an empty object.
	if m, ok := d.Metadata.(map[string]any); ok && len(m) == 0 && prior.Metadata.IsNull() {
		d.Metadata = nil
	}
	var err error
	if state.Metadata, err = jsonValue(d.Metadata); err != nil {
		diags.AddError("Error reading dataset metadata", err.Error())
	}
	if state.InputSchema, err = jsonValue(d.InputSchema); err != nil {
		diags.AddError("Error reading dataset input schema", err.Error())
	}
	if state.ExpectedOutputSchema, err = jsonValue(d.ExpectedOutputSchema); err != nil {
		diags.AddError("Error reading dataset expected output schema", err.Error())
	}
	return state, diags
}

func (r *datasetResource) upsert(ctx context.Context, plan datasetResourceModel, update bool) (datasetResourceModel, diag.Diagnostics) {
	var diags diag.Diagnostics
	body, err := datasetRequestBody(plan, update)
	if err != nil {
		diags.AddError("Error building dataset request", err.Error())
		return plan, diags
	}
	reader, err := jsonBody(body)
	if err != nil {
		diags.AddError("Error building dataset request", err.Error())
		return plan, diags
	}
	var dataset datasetAPI
	if err := apiCall(r.client(plan.ProjectPublicKey, plan.ProjectSecretKey).DatasetsCreateWithBody(ctx, jsonContentType, reader)).decode(&dataset); err != nil {
		diags.AddError("Error saving dataset", err.Error())
		return plan, diags
	}
	return datasetState(dataset, plan)
}

func (r *datasetResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan datasetResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	state, diags := r.upsert(ctx, plan, false)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *datasetResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state datasetResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var dataset datasetAPI
	err := apiCall(r.client(state.ProjectPublicKey, state.ProjectSecretKey).DatasetsGet(ctx, state.Name.ValueString())).decode(&dataset)
	if errors.Is(err, errNotFound) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Error reading dataset", err.Error())
		return
	}
	newState, diags := datasetState(dataset, state)
	resp.Diagnostics.Append(diags...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *datasetResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan datasetResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	state, diags := r.upsert(ctx, plan, true)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Delete only removes the dataset from state because the API has no delete endpoint.
func (r *datasetResource) Delete(_ context.Context, _ resource.DeleteRequest, resp *resource.DeleteResponse) {
	resp.Diagnostics.AddWarning(
		"Dataset left in Langfuse",
		"The Langfuse API has no delete endpoint for datasets. The dataset was removed from Terraform state but still exists in Langfuse; delete it in the Langfuse UI if it is no longer needed.",
	)
}

func (r *datasetResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	publicKey, secretKey, name, err := parseCredentialImportID(req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), name)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project_public_key"), publicKey)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project_secret_key"), secretKey)...)
}
