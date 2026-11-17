package provider

import (
	"context"
	"errors"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework-timetypes/timetypes"
	"github.com/hashicorp/terraform-plugin-framework-validators/resourcevalidator"
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

var _ resource.Resource = &modelResource{}
var _ resource.ResourceWithConfigValidators = &modelResource{}
var _ resource.ResourceWithImportState = &modelResource{}

func NewModelResource() resource.Resource {
	return &modelResource{}
}

type modelResource struct {
	publicAPIResource
}

type modelResourceModel struct {
	ID               types.String         `tfsdk:"id"`
	ProjectPublicKey types.String         `tfsdk:"project_public_key"`
	ProjectSecretKey types.String         `tfsdk:"project_secret_key"`
	ModelName        types.String         `tfsdk:"model_name"`
	MatchPattern     types.String         `tfsdk:"match_pattern"`
	StartDate        timetypes.RFC3339    `tfsdk:"start_date"`
	Unit             types.String         `tfsdk:"unit"`
	InputPrice       types.Float64        `tfsdk:"input_price"`
	OutputPrice      types.Float64        `tfsdk:"output_price"`
	TotalPrice       types.Float64        `tfsdk:"total_price"`
	TokenizerID      types.String         `tfsdk:"tokenizer_id"`
	TokenizerConfig  jsontypes.Normalized `tfsdk:"tokenizer_config"`
	PricingTiers     jsontypes.Normalized `tfsdk:"pricing_tiers"`
}

type modelAPI struct {
	ID              string           `json:"id"`
	ModelName       string           `json:"modelName"`
	MatchPattern    string           `json:"matchPattern"`
	StartDate       *string          `json:"startDate"`
	Unit            *string          `json:"unit"`
	InputPrice      *float64         `json:"inputPrice"`
	OutputPrice     *float64         `json:"outputPrice"`
	TotalPrice      *float64         `json:"totalPrice"`
	TokenizerID     *string          `json:"tokenizerId"`
	TokenizerConfig any              `json:"tokenizerConfig"`
	PricingTiers    []map[string]any `json:"pricingTiers"`
}

func (r *modelResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_model"
}

func (r *modelResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manage a custom model definition (name matching and pricing) in a project using project-scoped credentials. " +
			"Set either the flat prices (input_price, output_price, total_price) or pricing_tiers, not both. " +
			"Langfuse-managed built-in models cannot be modified or deleted through the API and cannot be managed by this resource. " +
			"Import format: project_public_key:project_secret_key:model_id.",
		Attributes: withProjectCredentials(map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "The unique identifier of the model.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"model_name": schema.StringAttribute{
				Required:    true,
				Description: "Name of the model definition. Usage is attributed to the newest definition whose match_pattern matches.",
			},
			"match_pattern": schema.StringAttribute{
				Required:    true,
				Description: "Regular expression matched against the model name of ingested generations, for example `(?i)^(gpt-4o)$`.",
			},
			"start_date": schema.StringAttribute{
				CustomType:  timetypes.RFC3339Type{},
				Optional:    true,
				Description: "RFC 3339 timestamp from which this definition applies. Omit to apply it to all generations.",
			},
			"unit": schema.StringAttribute{
				Required:    true,
				Description: "Unit of usage. Valid values: CHARACTERS, TOKENS, MILLISECONDS, SECONDS, IMAGES, REQUESTS.",
				Validators: []validator.String{
					stringvalidator.OneOf("CHARACTERS", "TOKENS", "MILLISECONDS", "SECONDS", "IMAGES", "REQUESTS"),
				},
			},
			"input_price": schema.Float64Attribute{
				Optional:    true,
				Description: "Price in USD per input unit.",
			},
			"output_price": schema.Float64Attribute{
				Optional:    true,
				Description: "Price in USD per output unit.",
			},
			"total_price": schema.Float64Attribute{
				Optional:    true,
				Description: "Price in USD per total unit. Cannot be combined with input_price or output_price.",
			},
			"tokenizer_id": schema.StringAttribute{
				Optional:    true,
				Description: "Tokenizer used to count tokens when usage is not ingested. Valid values: openai, claude.",
				Validators: []validator.String{
					stringvalidator.OneOf("openai", "claude"),
				},
			},
			"tokenizer_config": schema.StringAttribute{
				CustomType:  jsontypes.NormalizedType{},
				Optional:    true,
				Description: "Tokenizer configuration as a JSON object string.",
			},
			"pricing_tiers": schema.StringAttribute{
				CustomType: jsontypes.NormalizedType{},
				Optional:   true,
				Description: "Pricing tiers as a JSON array string. Each tier has name, isDefault, priority, conditions, and prices. " +
					"Exactly one tier must be the default tier with priority 0 and no conditions.",
			},
		}),
	}
}

func (r *modelResource) ConfigValidators(_ context.Context) []resource.ConfigValidator {
	tiers := path.MatchRoot("pricing_tiers")
	return []resource.ConfigValidator{
		resourcevalidator.AtLeastOneOf(path.MatchRoot("input_price"), path.MatchRoot("output_price"), path.MatchRoot("total_price"), tiers),
		resourcevalidator.Conflicting(tiers, path.MatchRoot("input_price")),
		resourcevalidator.Conflicting(tiers, path.MatchRoot("output_price")),
		resourcevalidator.Conflicting(tiers, path.MatchRoot("total_price")),
	}
}

func modelRequestBody(plan modelResourceModel) (map[string]any, error) {
	body := map[string]any{
		"modelName":       plan.ModelName.ValueString(),
		"matchPattern":    plan.MatchPattern.ValueString(),
		"unit":            plan.Unit.ValueString(),
		"startDate":       plan.StartDate.ValueStringPointer(),
		"inputPrice":      plan.InputPrice.ValueFloat64Pointer(),
		"outputPrice":     plan.OutputPrice.ValueFloat64Pointer(),
		"totalPrice":      plan.TotalPrice.ValueFloat64Pointer(),
		"tokenizerId":     plan.TokenizerID.ValueStringPointer(),
		"tokenizerConfig": nil,
	}
	tokenizerConfig, err := jsonAttr(plan.TokenizerConfig)
	if err != nil {
		return nil, err
	}
	body["tokenizerConfig"] = tokenizerConfig
	tiers, err := jsonAttr(plan.PricingTiers)
	if err != nil {
		return nil, err
	}
	if tiers != nil {
		body["pricingTiers"] = tiers
	}
	return body, nil
}

// modelState maps an API model onto prior (the plan or previous state), which
// supplies the credentials and decides whether prices are tracked flat or tiered.
func modelState(m modelAPI, prior modelResourceModel) (modelResourceModel, diag.Diagnostics) {
	var diags diag.Diagnostics
	state := modelResourceModel{
		ID:               types.StringValue(m.ID),
		ProjectPublicKey: prior.ProjectPublicKey,
		ProjectSecretKey: prior.ProjectSecretKey,
		ModelName:        types.StringValue(m.ModelName),
		MatchPattern:     types.StringValue(m.MatchPattern),
		StartDate:        timetypes.NewRFC3339Null(),
		Unit:             stringOrNull(m.Unit),
		InputPrice:       types.Float64Null(),
		OutputPrice:      types.Float64Null(),
		TotalPrice:       types.Float64Null(),
		TokenizerID:      stringOrNull(m.TokenizerID),
		PricingTiers:     jsontypes.NewNormalizedNull(),
	}
	if m.StartDate != nil {
		startDate, d := timetypes.NewRFC3339Value(*m.StartDate)
		diags.Append(d...)
		state.StartDate = startDate
	}
	tokenizerConfig, err := keepJSON(prior.TokenizerConfig, m.TokenizerConfig)
	if err != nil {
		diags.AddError("Error mapping tokenizer_config", err.Error())
	}
	state.TokenizerConfig = tokenizerConfig

	// The API always reports a default tier, even for flat prices, so the
	// remote shape alone cannot tell which form the configuration uses.
	if !prior.PricingTiers.IsNull() || len(m.PricingTiers) > 1 {
		for _, tier := range m.PricingTiers {
			delete(tier, "id")
		}
		tiers, err := keepJSON(prior.PricingTiers, m.PricingTiers)
		if err != nil {
			diags.AddError("Error mapping pricing_tiers", err.Error())
		}
		state.PricingTiers = tiers
	} else {
		state.InputPrice = float64OrNull(m.InputPrice)
		state.OutputPrice = float64OrNull(m.OutputPrice)
		state.TotalPrice = float64OrNull(m.TotalPrice)
	}
	return state, diags
}

func (r *modelResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan modelResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body, err := modelRequestBody(plan)
	if err != nil {
		resp.Diagnostics.AddError("Error building model request", err.Error())
		return
	}
	reader, err := jsonBody(body)
	if err != nil {
		resp.Diagnostics.AddError("Error building model request", err.Error())
		return
	}
	var model modelAPI
	if err := apiCall(r.client(plan.ProjectPublicKey, plan.ProjectSecretKey).ModelsCreateWithBody(ctx, jsonContentType, reader)).decode(&model); err != nil {
		resp.Diagnostics.AddError("Error creating model", err.Error())
		return
	}
	state, diags := modelState(model, plan)
	resp.Diagnostics.Append(diags...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *modelResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state modelResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var model modelAPI
	err := apiCall(r.client(state.ProjectPublicKey, state.ProjectSecretKey).ModelsGet(ctx, state.ID.ValueString())).decode(&model)
	if errors.Is(err, errNotFound) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Error reading model", err.Error())
		return
	}
	newState, diags := modelState(model, state)
	resp.Diagnostics.Append(diags...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *modelResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan modelResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body, err := modelRequestBody(plan)
	if err != nil {
		resp.Diagnostics.AddError("Error building model request", err.Error())
		return
	}
	reader, err := jsonBody(body)
	if err != nil {
		resp.Diagnostics.AddError("Error building model request", err.Error())
		return
	}
	var model modelAPI
	if err := apiCall(r.client(plan.ProjectPublicKey, plan.ProjectSecretKey).ModelsUpsertWithBody(ctx, plan.ID.ValueString(), jsonContentType, reader)).decode(&model); err != nil {
		resp.Diagnostics.AddError("Error updating model", err.Error())
		return
	}
	state, diags := modelState(model, plan)
	resp.Diagnostics.Append(diags...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *modelResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state modelResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	err := apiCall(r.client(state.ProjectPublicKey, state.ProjectSecretKey).ModelsDelete(ctx, state.ID.ValueString())).decode(nil)
	if err != nil && !errors.Is(err, errNotFound) {
		resp.Diagnostics.AddError("Error deleting model", err.Error())
	}
}

func (r *modelResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	publicKey, secretKey, id, err := parseCredentialImportID(req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project_public_key"), publicKey)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project_secret_key"), secretKey)...)
}
