package provider

import (
	"context"
	"errors"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework-validators/float64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/float64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &evaluationRuleResource{}
var _ resource.ResourceWithImportState = &evaluationRuleResource{}

func NewEvaluationRuleResource() resource.Resource {
	return &evaluationRuleResource{}
}

type evaluationRuleResource struct {
	publicAPIResource
}

type evaluationRuleResourceModel struct {
	ID                   types.String         `tfsdk:"id"`
	ProjectPublicKey     types.String         `tfsdk:"project_public_key"`
	ProjectSecretKey     types.String         `tfsdk:"project_secret_key"`
	Name                 types.String         `tfsdk:"name"`
	Enabled              types.Bool           `tfsdk:"enabled"`
	Sampling             types.Float64        `tfsdk:"sampling"`
	Filter               jsontypes.Normalized `tfsdk:"filter"`
	EvaluatorAssignments jsontypes.Normalized `tfsdk:"evaluator_assignments"`
}

type evaluationRuleAPI struct {
	ID                   string  `json:"id"`
	Name                 string  `json:"name"`
	Enabled              bool    `json:"enabled"`
	Sampling             float64 `json:"sampling"`
	Filter               []any   `json:"filter"`
	EvaluatorAssignments []any   `json:"evaluatorAssignments"`
}

func (r *evaluationRuleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_evaluation_rule"
}

func (r *evaluationRuleResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manage an evaluation rule in a project using project-scoped credentials. " +
			"An evaluation rule decides which data is scored and by which evaluators. " +
			"An enabled rule needs at least one evaluator assignment, and every assignment must map all variables of its evaluator. " +
			"Import format: project_public_key:project_secret_key:evaluation_rule_id.",
		Attributes: withProjectCredentials(map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "The unique identifier of the evaluation rule.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Name of the evaluation rule. Names are not unique within a project.",
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"enabled": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(true),
				Description: "Whether the rule evaluates matching data. Defaults to `true`.",
			},
			"sampling": schema.Float64Attribute{
				Optional:    true,
				Computed:    true,
				Default:     float64default.StaticFloat64(1),
				Description: "Fraction of matching data to evaluate, between 0 and 1. Defaults to `1`.",
				Validators: []validator.Float64{
					float64validator.Between(0, 1),
				},
			},
			"filter": schema.StringAttribute{
				Optional:    true,
				CustomType:  jsontypes.NormalizedType{},
				Description: "Conditions that select the data to evaluate as a JSON array of filter objects, for example `{type = \"stringOptions\", column = \"name\", operator = \"any of\", value = [\"chat\"]}`. When omitted, the rule matches all data.",
			},
			"evaluator_assignments": schema.StringAttribute{
				Optional:    true,
				CustomType:  jsontypes.NormalizedType{},
				Description: "Evaluators run by the rule as a JSON array of `{evaluatorId, variableMapping}` objects, where `variableMapping` is an array of `{variable, source, jsonPath}` objects. The array replaces all existing assignments.",
			},
		}),
	}
}

// evaluationRuleRequestBody builds the full create or update body. The API
// rejects null for filter and sampling, so unset lists are sent as empty.
func evaluationRuleRequestBody(plan evaluationRuleResourceModel) (map[string]any, error) {
	body := map[string]any{
		"name":     plan.Name.ValueString(),
		"enabled":  plan.Enabled.ValueBool(),
		"sampling": plan.Sampling.ValueFloat64(),
	}
	for key, attr := range map[string]jsontypes.Normalized{
		"filter":               plan.Filter,
		"evaluatorAssignments": plan.EvaluatorAssignments,
	} {
		value, err := jsonAttr(attr)
		if err != nil {
			return nil, err
		}
		if value == nil {
			value = []any{}
		}
		body[key] = value
	}
	return body, nil
}

// keepJSONList is keepJSON for lists whose unset form is reported as empty.
func keepJSONList(prior jsontypes.Normalized, remote []any) (jsontypes.Normalized, error) {
	if prior.IsNull() && len(remote) == 0 {
		return prior, nil
	}
	return keepJSON(prior, remote)
}

// evaluationRuleState maps an API rule onto prior (the plan or previous
// state), which supplies the credentials and the configured JSON spelling.
func evaluationRuleState(rule evaluationRuleAPI, prior evaluationRuleResourceModel) (evaluationRuleResourceModel, diag.Diagnostics) {
	var diags diag.Diagnostics
	state := evaluationRuleResourceModel{
		ID:               types.StringValue(rule.ID),
		ProjectPublicKey: prior.ProjectPublicKey,
		ProjectSecretKey: prior.ProjectSecretKey,
		Name:             types.StringValue(rule.Name),
		Enabled:          types.BoolValue(rule.Enabled),
		Sampling:         types.Float64Value(rule.Sampling),
	}
	var err error
	if state.Filter, err = keepJSONList(prior.Filter, rule.Filter); err != nil {
		diags.AddError("Error reading evaluation rule filter", err.Error())
	}
	if state.EvaluatorAssignments, err = keepJSONList(prior.EvaluatorAssignments, rule.EvaluatorAssignments); err != nil {
		diags.AddError("Error reading evaluation rule evaluator assignments", err.Error())
	}
	return state, diags
}

func (r *evaluationRuleResource) save(ctx context.Context, plan evaluationRuleResourceModel, id string) (evaluationRuleResourceModel, diag.Diagnostics) {
	var diags diag.Diagnostics
	body, err := evaluationRuleRequestBody(plan)
	if err != nil {
		diags.AddError("Error building evaluation rule request", err.Error())
		return plan, diags
	}
	reader, err := jsonBody(body)
	if err != nil {
		diags.AddError("Error building evaluation rule request", err.Error())
		return plan, diags
	}
	client := r.client(plan.ProjectPublicKey, plan.ProjectSecretKey)
	var rule evaluationRuleAPI
	if id == "" {
		err = apiCall(client.EvaluationRulesCreateWithBody(ctx, jsonContentType, reader)).decode(&rule)
	} else {
		err = apiCall(client.EvaluationRulesUpdateWithBody(ctx, id, jsonContentType, reader)).decode(&rule)
	}
	if err != nil {
		diags.AddError("Error saving evaluation rule", err.Error())
		return plan, diags
	}
	return evaluationRuleState(rule, plan)
}

func (r *evaluationRuleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan evaluationRuleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	state, diags := r.save(ctx, plan, "")
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *evaluationRuleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state evaluationRuleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var rule evaluationRuleAPI
	err := apiCall(r.client(state.ProjectPublicKey, state.ProjectSecretKey).EvaluationRulesGet(ctx, state.ID.ValueString())).decode(&rule)
	if errors.Is(err, errNotFound) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Error reading evaluation rule", err.Error())
		return
	}
	newState, diags := evaluationRuleState(rule, state)
	resp.Diagnostics.Append(diags...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *evaluationRuleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan evaluationRuleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	state, diags := r.save(ctx, plan, plan.ID.ValueString())
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *evaluationRuleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state evaluationRuleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	err := apiCall(r.client(state.ProjectPublicKey, state.ProjectSecretKey).EvaluationRulesDelete(ctx, state.ID.ValueString())).decode(nil)
	if err != nil && !errors.Is(err, errNotFound) {
		resp.Diagnostics.AddError("Error deleting evaluation rule", err.Error())
	}
}

func (r *evaluationRuleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	publicKey, secretKey, id, err := parseCredentialImportID(req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project_public_key"), publicKey)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project_secret_key"), secretKey)...)
}
