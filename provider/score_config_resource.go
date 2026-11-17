package provider

import (
	"context"
	"errors"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/float64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &scoreConfigResource{}
var _ resource.ResourceWithImportState = &scoreConfigResource{}

func NewScoreConfigResource() resource.Resource {
	return &scoreConfigResource{}
}

type scoreConfigResource struct {
	publicAPIResource
}

type scoreConfigResourceModel struct {
	ID               types.String  `tfsdk:"id"`
	ProjectPublicKey types.String  `tfsdk:"project_public_key"`
	ProjectSecretKey types.String  `tfsdk:"project_secret_key"`
	Name             types.String  `tfsdk:"name"`
	DataType         types.String  `tfsdk:"data_type"`
	Description      types.String  `tfsdk:"description"`
	MinValue         types.Float64 `tfsdk:"min_value"`
	MaxValue         types.Float64 `tfsdk:"max_value"`
	Categories       types.List    `tfsdk:"categories"`
	IsArchived       types.Bool    `tfsdk:"is_archived"`
}

type scoreConfigCategory struct {
	Label types.String  `tfsdk:"label"`
	Value types.Float64 `tfsdk:"value"`
}

type scoreConfigCategoryAPI struct {
	Label string  `json:"label"`
	Value float64 `json:"value"`
}

type scoreConfigAPI struct {
	ID          string                   `json:"id"`
	Name        string                   `json:"name"`
	DataType    string                   `json:"dataType"`
	Description *string                  `json:"description"`
	MinValue    *float64                 `json:"minValue"`
	MaxValue    *float64                 `json:"maxValue"`
	Categories  []scoreConfigCategoryAPI `json:"categories"`
	IsArchived  bool                     `json:"isArchived"`
}

var scoreConfigCategoryType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"label": types.StringType,
	"value": types.Float64Type,
}}

func (r *scoreConfigResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_score_config"
}

func (r *scoreConfigResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	// The API rejects null for a bound that is already set, so removing one
	// can only be done by creating a new score config.
	replaceWhenRemoved := float64planmodifier.RequiresReplaceIf(
		func(_ context.Context, req planmodifier.Float64Request, resp *float64planmodifier.RequiresReplaceIfFuncResponse) {
			resp.RequiresReplace = req.ConfigValue.IsNull() && !req.StateValue.IsNull()
		},
		"Removing a bound requires a new score config because the API cannot clear it.",
		"Removing a bound requires a new score config because the API cannot clear it.",
	)
	resp.Schema = schema.Schema{
		Description: "Manage a score config (the schema that scores of a given name must follow) in a project using project-scoped credentials. " +
			"The Langfuse API has no delete endpoint for score configs: destroying this resource archives the score config (is_archived = true) and removes it from state. " +
			"Replacing the resource therefore leaves the previous score config behind as archived. " +
			"Import format: project_public_key:project_secret_key:score_config_id.",
		Attributes: withProjectCredentials(map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "The unique identifier of the score config.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Name of the score config. Scores with this name are validated against it.",
			},
			"data_type": schema.StringAttribute{
				Required:    true,
				Description: "Data type of the scores. Valid values: NUMERIC, BOOLEAN, CATEGORICAL, TEXT. Changing it forces a new score config.",
				Validators: []validator.String{
					stringvalidator.OneOf("NUMERIC", "BOOLEAN", "CATEGORICAL", "TEXT"),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"description": schema.StringAttribute{
				Optional:    true,
				Description: "Description shown to annotators.",
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"min_value": schema.Float64Attribute{
				Optional:      true,
				Description:   "Minimum value for NUMERIC scores. Removing it forces a new score config.",
				PlanModifiers: []planmodifier.Float64{replaceWhenRemoved},
			},
			"max_value": schema.Float64Attribute{
				Optional:      true,
				Description:   "Maximum value for NUMERIC scores. Removing it forces a new score config.",
				PlanModifiers: []planmodifier.Float64{replaceWhenRemoved},
			},
			"categories": schema.ListNestedAttribute{
				Optional: true,
				Description: "Categories for CATEGORICAL scores. Required for CATEGORICAL and not allowed for other data types; " +
					"BOOLEAN score configs get True/False categories from Langfuse automatically.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"label": schema.StringAttribute{
							Required:    true,
							Description: "Label of the category.",
						},
						"value": schema.Float64Attribute{
							Required:    true,
							Description: "Numeric value of the category.",
						},
					},
				},
			},
			"is_archived": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
				Description: "Whether the score config is archived. Archived score configs cannot be used for new scores. Defaults to false.",
			},
		}),
	}
}

func scoreConfigCategories(ctx context.Context, list types.List) ([]scoreConfigCategoryAPI, diag.Diagnostics) {
	if list.IsNull() || list.IsUnknown() {
		return nil, nil
	}
	var categories []scoreConfigCategory
	diags := list.ElementsAs(ctx, &categories, false)
	out := make([]scoreConfigCategoryAPI, 0, len(categories))
	for _, c := range categories {
		out = append(out, scoreConfigCategoryAPI{Label: c.Label.ValueString(), Value: c.Value.ValueFloat64()})
	}
	return out, diags
}

// scoreConfigRequestBody builds the create body, or the update body when
// update is set. The update endpoint only accepts fields that have a value.
func scoreConfigRequestBody(ctx context.Context, plan scoreConfigResourceModel, update bool) (map[string]any, diag.Diagnostics) {
	body := map[string]any{"name": plan.Name.ValueString()}
	if update {
		// An empty description is the only way to clear one.
		body["description"] = plan.Description.ValueString()
		body["isArchived"] = plan.IsArchived.ValueBool()
	} else {
		body["dataType"] = plan.DataType.ValueString()
		if !plan.Description.IsNull() {
			body["description"] = plan.Description.ValueString()
		}
	}
	if !plan.MinValue.IsNull() {
		body["minValue"] = plan.MinValue.ValueFloat64()
	}
	if !plan.MaxValue.IsNull() {
		body["maxValue"] = plan.MaxValue.ValueFloat64()
	}
	categories, diags := scoreConfigCategories(ctx, plan.Categories)
	if categories != nil {
		body["categories"] = categories
	}
	return body, diags
}

// scoreConfigState maps an API score config onto prior (the plan or previous
// state), which supplies the credentials.
func scoreConfigState(ctx context.Context, c scoreConfigAPI, prior scoreConfigResourceModel) (scoreConfigResourceModel, diag.Diagnostics) {
	var diags diag.Diagnostics
	state := scoreConfigResourceModel{
		ID:               types.StringValue(c.ID),
		ProjectPublicKey: prior.ProjectPublicKey,
		ProjectSecretKey: prior.ProjectSecretKey,
		Name:             types.StringValue(c.Name),
		DataType:         types.StringValue(c.DataType),
		Description:      types.StringNull(),
		MinValue:         float64OrNull(c.MinValue),
		MaxValue:         float64OrNull(c.MaxValue),
		Categories:       types.ListNull(scoreConfigCategoryType),
		IsArchived:       types.BoolValue(c.IsArchived),
	}
	if c.Description != nil && *c.Description != "" {
		state.Description = types.StringValue(*c.Description)
	}
	// BOOLEAN score configs report server-generated True/False categories that
	// cannot be configured, so categories are only tracked for CATEGORICAL.
	if c.DataType == "CATEGORICAL" && c.Categories != nil {
		categories := make([]scoreConfigCategory, 0, len(c.Categories))
		for _, category := range c.Categories {
			categories = append(categories, scoreConfigCategory{
				Label: types.StringValue(category.Label),
				Value: types.Float64Value(category.Value),
			})
		}
		list, d := types.ListValueFrom(ctx, scoreConfigCategoryType, categories)
		diags.Append(d...)
		state.Categories = list
	}
	return state, diags
}

func (r *scoreConfigResource) update(ctx context.Context, id string, plan scoreConfigResourceModel) (scoreConfigAPI, diag.Diagnostics) {
	var config scoreConfigAPI
	body, diags := scoreConfigRequestBody(ctx, plan, true)
	if diags.HasError() {
		return config, diags
	}
	reader, err := jsonBody(body)
	if err != nil {
		diags.AddError("Error building score config request", err.Error())
		return config, diags
	}
	if err := apiCall(r.client(plan.ProjectPublicKey, plan.ProjectSecretKey).ScoreConfigsUpdateWithBody(ctx, id, jsonContentType, reader)).decode(&config); err != nil {
		diags.AddError("Error updating score config", err.Error())
	}
	return config, diags
}

func (r *scoreConfigResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan scoreConfigResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body, diags := scoreConfigRequestBody(ctx, plan, false)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	reader, err := jsonBody(body)
	if err != nil {
		resp.Diagnostics.AddError("Error building score config request", err.Error())
		return
	}
	var config scoreConfigAPI
	if err := apiCall(r.client(plan.ProjectPublicKey, plan.ProjectSecretKey).ScoreConfigsCreateWithBody(ctx, jsonContentType, reader)).decode(&config); err != nil {
		resp.Diagnostics.AddError("Error creating score config", err.Error())
		return
	}
	state, diags := scoreConfigState(ctx, config, plan)
	resp.Diagnostics.Append(diags...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
	if resp.Diagnostics.HasError() || !plan.IsArchived.ValueBool() {
		return
	}

	// The create endpoint cannot archive, so an archived config needs a second call.
	config, diags = r.update(ctx, config.ID, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	state, diags = scoreConfigState(ctx, config, plan)
	resp.Diagnostics.Append(diags...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *scoreConfigResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state scoreConfigResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var config scoreConfigAPI
	err := apiCall(r.client(state.ProjectPublicKey, state.ProjectSecretKey).ScoreConfigsGetByID(ctx, state.ID.ValueString())).decode(&config)
	if errors.Is(err, errNotFound) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Error reading score config", err.Error())
		return
	}
	newState, diags := scoreConfigState(ctx, config, state)
	resp.Diagnostics.Append(diags...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *scoreConfigResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan scoreConfigResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	config, diags := r.update(ctx, plan.ID.ValueString(), plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	state, diags := scoreConfigState(ctx, config, plan)
	resp.Diagnostics.Append(diags...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Delete archives the score config because the API has no delete endpoint.
func (r *scoreConfigResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state scoreConfigResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() || state.IsArchived.ValueBool() {
		return
	}
	reader, err := jsonBody(map[string]any{"isArchived": true})
	if err != nil {
		resp.Diagnostics.AddError("Error building score config request", err.Error())
		return
	}
	err = apiCall(r.client(state.ProjectPublicKey, state.ProjectSecretKey).ScoreConfigsUpdateWithBody(ctx, state.ID.ValueString(), jsonContentType, reader)).decode(nil)
	if err != nil && !errors.Is(err, errNotFound) {
		resp.Diagnostics.AddError("Error archiving score config", err.Error())
	}
}

func (r *scoreConfigResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	publicKey, secretKey, id, err := parseCredentialImportID(req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project_public_key"), publicKey)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project_secret_key"), secretKey)...)
}
