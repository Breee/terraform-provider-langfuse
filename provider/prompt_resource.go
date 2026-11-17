package provider

import (
	"context"
	"errors"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/langfuse/terraform-provider-langfuse/langfuse/publicapi"
)

var _ resource.Resource = &promptResource{}
var _ resource.ResourceWithImportState = &promptResource{}
var _ resource.ResourceWithModifyPlan = &promptResource{}

// promptLatestLabel is assigned by Langfuse to the newest version and cannot be managed.
const promptLatestLabel = "latest"

func NewPromptResource() resource.Resource {
	return &promptResource{}
}

type promptResource struct {
	publicAPIResource
}

type promptResourceModel struct {
	ID               types.String         `tfsdk:"id"`
	ProjectPublicKey types.String         `tfsdk:"project_public_key"`
	ProjectSecretKey types.String         `tfsdk:"project_secret_key"`
	Name             types.String         `tfsdk:"name"`
	Type             types.String         `tfsdk:"type"`
	Prompt           types.String         `tfsdk:"prompt"`
	Config           jsontypes.Normalized `tfsdk:"config"`
	Labels           types.Set            `tfsdk:"labels"`
	Tags             types.Set            `tfsdk:"tags"`
	CommitMessage    types.String         `tfsdk:"commit_message"`
	Version          types.Int64          `tfsdk:"version"`
}

type promptAPI struct {
	Name          string   `json:"name"`
	Version       int64    `json:"version"`
	Type          string   `json:"type"`
	Prompt        any      `json:"prompt"`
	Config        any      `json:"config"`
	Labels        []string `json:"labels"`
	Tags          []string `json:"tags"`
	CommitMessage *string  `json:"commitMessage"`
}

func (r *promptResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_prompt"
}

func (r *promptResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manage a prompt in a project using project-scoped credentials. " +
			"Prompt versions are immutable in Langfuse: changing prompt, config, tags, or commit_message, or removing a label, creates a new version and moves the configured labels to it. " +
			"Adding labels updates the current version in place. Earlier versions are kept until the resource is destroyed, which deletes the prompt with all of its versions. " +
			"The resource tracks the version it created; versions created outside Terraform are left alone. " +
			"Import format: project_public_key:project_secret_key:prompt_name (imports the latest version).",
		Attributes: withProjectCredentials(map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "The name of the prompt.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Name of the prompt, unique within the project. Use slashes to place it in folders. Changing it forces a new prompt.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"type": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("text"),
				Description: "Type of the prompt. Valid values: text, chat. Defaults to text. Changing it forces a new prompt.",
				Validators: []validator.String{
					stringvalidator.OneOf("text", "chat"),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"prompt": schema.StringAttribute{
				Required: true,
				Description: "Content of the prompt. For text prompts, the prompt text. " +
					"For chat prompts, a JSON array string of messages (`{role, content}`) and placeholders (`{type = \"placeholder\", name}`).",
			},
			"config": schema.StringAttribute{
				CustomType:  jsontypes.NormalizedType{},
				Optional:    true,
				Description: "Arbitrary configuration stored with the prompt version (for example model parameters) as a JSON object string.",
			},
			"labels": schema.SetAttribute{
				ElementType: types.StringType,
				Optional:    true,
				Description: "Labels of the tracked version, for example `production`. A label points to one version at a time. The `latest` label is managed by Langfuse and cannot be set.",
				Validators: []validator.Set{
					setvalidator.ValueStringsAre(stringvalidator.NoneOf(promptLatestLabel)),
				},
			},
			"tags": schema.SetAttribute{
				ElementType: types.StringType,
				Optional:    true,
				Description: "Tags of the prompt. Tags apply to all versions of the prompt.",
			},
			"commit_message": schema.StringAttribute{
				Optional:    true,
				Description: "Commit message of the tracked version.",
			},
			"version": schema.Int64Attribute{
				Computed:    true,
				Description: "Version number tracked by this resource.",
			},
		}),
	}
}

// promptNeedsVersion reports whether moving from state to plan requires a new
// prompt version. Only adding labels can be applied to an existing version.
func promptNeedsVersion(plan, state promptResourceModel) bool {
	if !plan.Prompt.Equal(state.Prompt) || !plan.Tags.Equal(state.Tags) || !plan.CommitMessage.Equal(state.CommitMessage) {
		return true
	}
	if plan.Config.IsUnknown() || plan.Config.IsNull() != state.Config.IsNull() {
		return true
	}
	if !plan.Config.IsNull() {
		equal, diags := plan.Config.StringSemanticEquals(context.Background(), state.Config)
		if diags.HasError() || !equal {
			return true
		}
	}
	if plan.Labels.IsUnknown() {
		return true
	}
	planned := map[string]bool{}
	for _, label := range plan.Labels.Elements() {
		planned[label.String()] = true
	}
	for _, label := range state.Labels.Elements() {
		if !planned[label.String()] {
			return true
		}
	}
	return false
}

func (r *promptResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.State.Raw.IsNull() || req.Plan.Raw.IsNull() {
		return
	}
	var plan, state promptResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	version := state.Version
	if promptNeedsVersion(plan, state) {
		version = types.Int64Unknown()
	}
	resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root("version"), version)...)
}

func promptStrings(ctx context.Context, set types.Set) ([]string, diag.Diagnostics) {
	values := []string{}
	if set.IsNull() || set.IsUnknown() {
		return values, nil
	}
	diags := set.ElementsAs(ctx, &values, false)
	return values, diags
}

func promptRequestBody(ctx context.Context, plan promptResourceModel) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics
	body := map[string]any{
		"name":          plan.Name.ValueString(),
		"type":          plan.Type.ValueString(),
		"prompt":        plan.Prompt.ValueString(),
		"commitMessage": plan.CommitMessage.ValueStringPointer(),
	}
	if plan.Type.ValueString() == "chat" {
		messages, err := jsonAttr(jsontypes.NewNormalizedValue(plan.Prompt.ValueString()))
		if err != nil {
			diags.AddAttributeError(path.Root("prompt"), "Invalid chat prompt", "The prompt of a chat prompt must be a JSON array string: "+err.Error())
			return nil, diags
		}
		body["prompt"] = messages
	}
	config, err := jsonAttr(plan.Config)
	if err != nil {
		diags.AddAttributeError(path.Root("config"), "Invalid config", err.Error())
		return nil, diags
	}
	if config != nil {
		body["config"] = config
	}
	labels, d := promptStrings(ctx, plan.Labels)
	diags.Append(d...)
	body["labels"] = labels
	tags, d := promptStrings(ctx, plan.Tags)
	diags.Append(d...)
	body["tags"] = tags
	return body, diags
}

// promptSet maps remote values to a set, keeping null when nothing is
// configured and nothing is set remotely.
func promptSet(ctx context.Context, values []string, prior types.Set) (types.Set, diag.Diagnostics) {
	if len(values) == 0 && prior.IsNull() {
		return types.SetNull(types.StringType), nil
	}
	if values == nil {
		values = []string{}
	}
	return types.SetValueFrom(ctx, types.StringType, values)
}

// promptState maps an API prompt version onto prior (the plan or previous
// state), which supplies the credentials and the configured JSON formatting.
func promptState(ctx context.Context, p promptAPI, prior promptResourceModel) (promptResourceModel, diag.Diagnostics) {
	var diags diag.Diagnostics
	state := promptResourceModel{
		ID:               types.StringValue(p.Name),
		ProjectPublicKey: prior.ProjectPublicKey,
		ProjectSecretKey: prior.ProjectSecretKey,
		Name:             types.StringValue(p.Name),
		Type:             types.StringValue(p.Type),
		Config:           jsontypes.NewNormalizedNull(),
		CommitMessage:    stringOrNull(p.CommitMessage),
		Version:          types.Int64Value(p.Version),
	}

	if text, ok := p.Prompt.(string); ok {
		state.Prompt = types.StringValue(text)
	} else {
		priorPrompt := jsontypes.NewNormalizedNull()
		if !prior.Prompt.IsNull() && !prior.Prompt.IsUnknown() {
			priorPrompt = jsontypes.NewNormalizedValue(prior.Prompt.ValueString())
		}
		messages, err := keepJSON(priorPrompt, p.Prompt)
		if err != nil {
			diags.AddError("Error mapping prompt", err.Error())
		}
		state.Prompt = types.StringValue(messages.ValueString())
	}

	// Langfuse stores an empty object when no config is given.
	if object, ok := p.Config.(map[string]any); !prior.Config.IsNull() || !ok || len(object) > 0 {
		config, err := keepJSON(prior.Config, p.Config)
		if err != nil {
			diags.AddError("Error mapping config", err.Error())
		}
		state.Config = config
	}

	labels := make([]string, 0, len(p.Labels))
	for _, label := range p.Labels {
		if label != promptLatestLabel {
			labels = append(labels, label)
		}
	}
	var d diag.Diagnostics
	state.Labels, d = promptSet(ctx, labels, prior.Labels)
	diags.Append(d...)
	state.Tags, d = promptSet(ctx, p.Tags, prior.Tags)
	diags.Append(d...)
	return state, diags
}

func (r *promptResource) createVersion(ctx context.Context, plan promptResourceModel) (promptResourceModel, diag.Diagnostics) {
	body, diags := promptRequestBody(ctx, plan)
	if diags.HasError() {
		return plan, diags
	}
	reader, err := jsonBody(body)
	if err != nil {
		diags.AddError("Error building prompt request", err.Error())
		return plan, diags
	}
	var prompt promptAPI
	if err := apiCall(r.client(plan.ProjectPublicKey, plan.ProjectSecretKey).PromptsCreateWithBody(ctx, jsonContentType, reader)).decode(&prompt); err != nil {
		diags.AddError("Error creating prompt version", err.Error())
		return plan, diags
	}
	state, d := promptState(ctx, prompt, plan)
	diags.Append(d...)
	return state, diags
}

func (r *promptResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan promptResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	state, diags := r.createVersion(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *promptResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state promptResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// Dependencies are left unresolved so the stored prompt is compared.
	resolve := false
	params := &publicapi.PromptsGetParams{Resolve: &resolve}
	if state.Version.IsNull() {
		// Import does not know a version yet and adopts the newest one.
		label := promptLatestLabel
		params.Label = &label
	} else {
		version := int(state.Version.ValueInt64())
		params.Version = &version
	}
	var prompt promptAPI
	err := apiCall(r.client(state.ProjectPublicKey, state.ProjectSecretKey).PromptsGet(ctx, state.ID.ValueString(), params)).decode(&prompt)
	if errors.Is(err, errNotFound) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Error reading prompt", err.Error())
		return
	}
	newState, diags := promptState(ctx, prompt, state)
	resp.Diagnostics.Append(diags...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *promptResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, prior promptResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if promptNeedsVersion(plan, prior) {
		state, diags := r.createVersion(ctx, plan)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
		return
	}

	labels, diags := promptStrings(ctx, plan.Labels)
	resp.Diagnostics.Append(diags...)
	reader, err := jsonBody(map[string]any{"newLabels": labels})
	if err != nil {
		resp.Diagnostics.AddError("Error building prompt request", err.Error())
		return
	}
	var prompt promptAPI
	client := r.client(plan.ProjectPublicKey, plan.ProjectSecretKey)
	if err := apiCall(client.PromptVersionUpdateWithBody(ctx, prior.Name.ValueString(), int(prior.Version.ValueInt64()), jsonContentType, reader)).decode(&prompt); err != nil {
		resp.Diagnostics.AddError("Error updating prompt labels", err.Error())
		return
	}
	state, diags := promptState(ctx, prompt, plan)
	resp.Diagnostics.Append(diags...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *promptResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state promptResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	err := apiCall(r.client(state.ProjectPublicKey, state.ProjectSecretKey).PromptsDelete(ctx, state.ID.ValueString(), nil)).decode(nil)
	if err != nil && !errors.Is(err, errNotFound) {
		resp.Diagnostics.AddError("Error deleting prompt", err.Error())
	}
}

func (r *promptResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	publicKey, secretKey, name, err := parseCredentialImportID(req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), name)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project_public_key"), publicKey)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project_secret_key"), secretKey)...)
}
