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

const (
	evaluatorTypeLlmAsJudge = "llm_as_judge"
	evaluatorTypeCode       = "code"
)

var _ resource.Resource = &evaluatorResource{}
var _ resource.ResourceWithImportState = &evaluatorResource{}
var _ resource.ResourceWithValidateConfig = &evaluatorResource{}

func NewEvaluatorResource() resource.Resource {
	return &evaluatorResource{}
}

type evaluatorResource struct {
	publicAPIResource
}

type evaluatorResourceModel struct {
	ID                 types.String         `tfsdk:"id"`
	ProjectPublicKey   types.String         `tfsdk:"project_public_key"`
	ProjectSecretKey   types.String         `tfsdk:"project_secret_key"`
	Name               types.String         `tfsdk:"name"`
	Description        types.String         `tfsdk:"description"`
	Type               types.String         `tfsdk:"type"`
	Prompt             jsontypes.Normalized `tfsdk:"prompt"`
	ModelProvider      types.String         `tfsdk:"model_provider"`
	Model              types.String         `tfsdk:"model"`
	VariableMapping    jsontypes.Normalized `tfsdk:"variable_mapping"`
	OutputDefinition   jsontypes.Normalized `tfsdk:"output_definition"`
	SourceCode         types.String         `tfsdk:"source_code"`
	SourceCodeLanguage types.String         `tfsdk:"source_code_language"`
	Version            types.Int64          `tfsdk:"version"`
	Status             types.String         `tfsdk:"status"`
}

type evaluatorAPI struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Description *string `json:"description"`
	Type        string  `json:"type"`
	Prompt      any     `json:"prompt"`
	ModelConfig *struct {
		Provider string `json:"provider"`
		Model    string `json:"model"`
	} `json:"modelConfig"`
	VariableMapping    any     `json:"variableMapping"`
	OutputDefinition   any     `json:"outputDefinition"`
	SourceCode         *string `json:"sourceCode"`
	SourceCodeLanguage *string `json:"sourceCodeLanguage"`
	Version            int64   `json:"version"`
	Status             string  `json:"status"`
}

func (r *evaluatorResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_evaluator"
}

func (r *evaluatorResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manage an evaluator in a project using project-scoped credentials. " +
			"An evaluator describes how to score data; evaluation rules decide which data is scored. " +
			"Changing the definition (prompt, model, variable mapping, output definition, or source code) creates a new evaluator version; changing only the name or description does not. " +
			"Code evaluators are only accepted by deployments that have code evaluations enabled. " +
			"Import format: project_public_key:project_secret_key:evaluator_id.",
		Attributes: withProjectCredentials(map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "The unique identifier of the evaluator.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Name of the evaluator.",
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"description": schema.StringAttribute{
				Optional:    true,
				Description: "Description of the evaluator.",
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"type": schema.StringAttribute{
				Required:    true,
				Description: "Type of the evaluator: `llm_as_judge` or `code`. Changing it forces a new evaluator.",
				Validators: []validator.String{
					stringvalidator.OneOf(evaluatorTypeLlmAsJudge, evaluatorTypeCode),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"prompt": schema.StringAttribute{
				Optional:    true,
				CustomType:  jsontypes.NormalizedType{},
				Description: "Chat prompt of an `llm_as_judge` evaluator as a JSON array of `{role, content}` messages. Variables use the `{{variable}}` syntax. Required for `llm_as_judge`.",
			},
			"model_provider": schema.StringAttribute{
				Optional:    true,
				Description: "LLM connection provider used by an `llm_as_judge` evaluator. When omitted, the project's default evaluation model is used.",
				Validators: []validator.String{
					stringvalidator.AlsoRequires(path.MatchRoot("model")),
				},
			},
			"model": schema.StringAttribute{
				Optional:    true,
				Description: "Model used by an `llm_as_judge` evaluator. Requires `model_provider`.",
				Validators: []validator.String{
					stringvalidator.AlsoRequires(path.MatchRoot("model_provider")),
				},
			},
			"variable_mapping": schema.StringAttribute{
				Optional:    true,
				CustomType:  jsontypes.NormalizedType{},
				Description: "Default mapping of prompt variables as a JSON array of `{variable, source, jsonPath}` objects. `source` is one of `input`, `output`, `metadata`, `tool_calls`, `expected_output`, `experiment_item_metadata`.",
			},
			"output_definition": schema.StringAttribute{
				Optional:    true,
				CustomType:  jsontypes.NormalizedType{},
				Description: "Score produced by an `llm_as_judge` evaluator as a JSON object with `dataType` (`NUMERIC`, `BOOLEAN`, or `CATEGORICAL`) and its type-specific fields such as `minValue`, `maxValue`, `categories`, `shouldAllowMultipleMatches`, `scoreReasoningInstructions`, and `scoreValueInstructions`. Required for `llm_as_judge`.",
			},
			"source_code": schema.StringAttribute{
				Optional:    true,
				Description: "Source code of a `code` evaluator. Required for `code`.",
			},
			"source_code_language": schema.StringAttribute{
				Optional:    true,
				Description: "Language of a `code` evaluator: `PYTHON` or `TYPESCRIPT`. Required for `code`.",
				Validators: []validator.String{
					stringvalidator.OneOf("PYTHON", "TYPESCRIPT"),
				},
			},
			"version": schema.Int64Attribute{
				Computed:    true,
				Description: "Current version of the evaluator definition.",
			},
			"status": schema.StringAttribute{
				Computed:    true,
				Description: "Status of the evaluator: `active` or `paused`. Langfuse pauses an evaluator that has no usable model.",
			},
		}),
	}
}

func (r *evaluatorResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var config evaluatorResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() || config.Type.IsUnknown() || config.Type.IsNull() {
		return
	}
	llm := map[string]bool{
		"prompt":            config.Prompt.IsNull(),
		"output_definition": config.OutputDefinition.IsNull(),
		"model_provider":    config.ModelProvider.IsNull(),
		"model":             config.Model.IsNull(),
		"variable_mapping":  config.VariableMapping.IsNull(),
	}
	code := map[string]bool{
		"source_code":          config.SourceCode.IsNull(),
		"source_code_language": config.SourceCodeLanguage.IsNull(),
	}
	required, forbidden := []string{"prompt", "output_definition"}, code
	missing := llm
	if config.Type.ValueString() == evaluatorTypeCode {
		required, forbidden, missing = []string{"source_code", "source_code_language"}, llm, code
	}
	for _, name := range required {
		if missing[name] {
			resp.Diagnostics.AddAttributeError(path.Root(name), "Missing evaluator attribute",
				name+" is required when type is "+config.Type.ValueString()+".")
		}
	}
	for name, isNull := range forbidden {
		if !isNull {
			resp.Diagnostics.AddAttributeError(path.Root(name), "Invalid evaluator attribute",
				name+" cannot be set when type is "+config.Type.ValueString()+".")
		}
	}
}

// evaluatorRequestBody builds the full create or update body. Sending an
// unchanged definition on update does not create a new version.
func evaluatorRequestBody(plan evaluatorResourceModel) (map[string]any, error) {
	body := map[string]any{
		"name":        plan.Name.ValueString(),
		"description": plan.Description.ValueStringPointer(),
		"type":        plan.Type.ValueString(),
	}
	if plan.Type.ValueString() == evaluatorTypeCode {
		body["sourceCode"] = plan.SourceCode.ValueString()
		body["sourceCodeLanguage"] = plan.SourceCodeLanguage.ValueString()
		return body, nil
	}
	var err error
	if body["prompt"], err = jsonAttr(plan.Prompt); err != nil {
		return nil, err
	}
	if body["outputDefinition"], err = jsonAttr(plan.OutputDefinition); err != nil {
		return nil, err
	}
	if body["variableMapping"], err = jsonAttr(plan.VariableMapping); err != nil {
		return nil, err
	}
	body["modelConfig"] = nil
	if !plan.ModelProvider.IsNull() {
		body["modelConfig"] = map[string]any{
			"provider": plan.ModelProvider.ValueString(),
			"model":    plan.Model.ValueString(),
		}
	}
	return body, nil
}

// evaluatorState maps an API evaluator onto prior (the plan or previous
// state), which supplies the credentials and the configured JSON spelling.
func evaluatorState(e evaluatorAPI, prior evaluatorResourceModel) (evaluatorResourceModel, diag.Diagnostics) {
	var diags diag.Diagnostics
	state := evaluatorResourceModel{
		ID:                 types.StringValue(e.ID),
		ProjectPublicKey:   prior.ProjectPublicKey,
		ProjectSecretKey:   prior.ProjectSecretKey,
		Name:               types.StringValue(e.Name),
		Description:        types.StringNull(),
		Type:               types.StringValue(e.Type),
		ModelProvider:      types.StringNull(),
		Model:              types.StringNull(),
		SourceCode:         stringOrNull(e.SourceCode),
		SourceCodeLanguage: stringOrNull(e.SourceCodeLanguage),
		Version:            types.Int64Value(e.Version),
		Status:             types.StringValue(e.Status),
	}
	if e.Description != nil && *e.Description != "" {
		state.Description = types.StringValue(*e.Description)
	}
	if e.ModelConfig != nil {
		state.ModelProvider = types.StringValue(e.ModelConfig.Provider)
		state.Model = types.StringValue(e.ModelConfig.Model)
	}
	var err error
	if state.Prompt, err = keepJSON(prior.Prompt, e.Prompt); err != nil {
		diags.AddError("Error reading evaluator prompt", err.Error())
	}
	if state.VariableMapping, err = keepJSON(prior.VariableMapping, e.VariableMapping); err != nil {
		diags.AddError("Error reading evaluator variable mapping", err.Error())
	}
	if state.OutputDefinition, err = keepJSON(prior.OutputDefinition, e.OutputDefinition); err != nil {
		diags.AddError("Error reading evaluator output definition", err.Error())
	}
	return state, diags
}

func (r *evaluatorResource) save(ctx context.Context, plan evaluatorResourceModel, id string) (evaluatorResourceModel, diag.Diagnostics) {
	var diags diag.Diagnostics
	body, err := evaluatorRequestBody(plan)
	if err != nil {
		diags.AddError("Error building evaluator request", err.Error())
		return plan, diags
	}
	reader, err := jsonBody(body)
	if err != nil {
		diags.AddError("Error building evaluator request", err.Error())
		return plan, diags
	}
	client := r.client(plan.ProjectPublicKey, plan.ProjectSecretKey)
	var evaluator evaluatorAPI
	if id == "" {
		err = apiCall(client.EvaluatorsCreateWithBody(ctx, jsonContentType, reader)).decode(&evaluator)
	} else {
		err = apiCall(client.EvaluatorsUpdateWithBody(ctx, id, jsonContentType, reader)).decode(&evaluator)
	}
	if err != nil {
		diags.AddError("Error saving evaluator", err.Error())
		return plan, diags
	}
	return evaluatorState(evaluator, plan)
}

func (r *evaluatorResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan evaluatorResourceModel
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

func (r *evaluatorResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state evaluatorResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var evaluator evaluatorAPI
	err := apiCall(r.client(state.ProjectPublicKey, state.ProjectSecretKey).EvaluatorsGet(ctx, state.ID.ValueString())).decode(&evaluator)
	if errors.Is(err, errNotFound) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Error reading evaluator", err.Error())
		return
	}
	newState, diags := evaluatorState(evaluator, state)
	resp.Diagnostics.Append(diags...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *evaluatorResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan evaluatorResourceModel
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

func (r *evaluatorResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state evaluatorResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	err := apiCall(r.client(state.ProjectPublicKey, state.ProjectSecretKey).EvaluatorsDelete(ctx, state.ID.ValueString())).decode(nil)
	if err != nil && !errors.Is(err, errNotFound) {
		resp.Diagnostics.AddError("Error deleting evaluator", err.Error())
	}
}

func (r *evaluatorResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	publicKey, secretKey, id, err := parseCredentialImportID(req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project_public_key"), publicKey)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project_secret_key"), secretKey)...)
}
