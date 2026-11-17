package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/langfuse/terraform-provider-langfuse/langfuse/publicapi"
)

var _ resource.Resource = &organizationLlmConnectionResource{}
var _ resource.ResourceWithValidateConfig = &organizationLlmConnectionResource{}
var _ resource.ResourceWithImportState = &organizationLlmConnectionResource{}

func NewOrganizationLlmConnectionResource() resource.Resource {
	return &organizationLlmConnectionResource{}
}

type organizationLlmConnectionResource struct{ publicAPIResource }

type organizationLlmConnectionModel struct {
	ID                     types.String         `tfsdk:"id"`
	OrganizationPublicKey  types.String         `tfsdk:"organization_public_key"`
	OrganizationPrivateKey types.String         `tfsdk:"organization_private_key"`
	ProviderName           types.String         `tfsdk:"provider_name"`
	Adapter                types.String         `tfsdk:"adapter"`
	SecretKey              types.String         `tfsdk:"secret_key"`
	BaseURL                types.String         `tfsdk:"base_url"`
	CustomModels           types.List           `tfsdk:"custom_models"`
	ExtraHeaders           types.Map            `tfsdk:"extra_headers"`
	WithDefaultModels      types.Bool           `tfsdk:"with_default_models"`
	Config                 jsontypes.Normalized `tfsdk:"config"`
}

type organizationLlmConnectionAPI struct {
	ID                string   `json:"id"`
	Provider          string   `json:"provider"`
	Adapter           string   `json:"adapter"`
	BaseURL           *string  `json:"baseURL"`
	CustomModels      []string `json:"customModels"`
	WithDefaultModels bool     `json:"withDefaultModels"`
	Config            any      `json:"config"`
}

func (r *organizationLlmConnectionResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_organization_llm_connection"
}

func (r *organizationLlmConnectionResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages an organization-level LLM connection, which is available to every project of the organization. " +
			"Requires an organization-scoped API key. Langfuse upserts connections by provider_name; applying a name that " +
			"already exists updates that connection. secret_key and extra_headers are write-only at the API and are retained " +
			"in Terraform state; use a secure state backend. Changes made to them outside Terraform are not detected. " +
			"Import format: organization_public_key:organization_private_key:connection_id. After import, supply secret_key " +
			"and any extra_headers in configuration because the API cannot recover them.",
		Attributes: withOrganizationCredentials(map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "The unique identifier of the LLM connection.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"provider_name": schema.StringAttribute{
				Required:      true,
				Description:   "The unique name of the connection within the organization. It is the upsert key, so changing it replaces the resource.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"adapter": schema.StringAttribute{
				Required:    true,
				Description: "The LLM service type. Valid values: anthropic, openai, azure, bedrock, google-vertex-ai, google-ai-studio.",
				Validators: []validator.String{
					stringvalidator.OneOf("anthropic", "openai", "azure", "bedrock", "google-vertex-ai", "google-ai-studio"),
				},
			},
			"secret_key": schema.StringAttribute{
				Required:    true,
				Sensitive:   true,
				Description: "The API authentication key for the LLM provider.",
			},
			"base_url": schema.StringAttribute{
				Optional:    true,
				Description: "Base URL override for the LLM provider API. Langfuse rejects hosts it cannot resolve.",
			},
			"custom_models": schema.ListAttribute{
				Optional:    true,
				ElementType: types.StringType,
				Description: "Custom model identifiers offered by this connection.",
			},
			"extra_headers": schema.MapAttribute{
				Optional:    true,
				Sensitive:   true,
				ElementType: types.StringType,
				Description: "Additional HTTP headers sent with LLM API requests.",
			},
			"with_default_models": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(true),
				Description: "Whether to offer the adapter's default models. Defaults to true.",
			},
			"config": schema.StringAttribute{
				Optional:    true,
				CustomType:  jsontypes.NormalizedType{},
				Description: "Adapter-specific configuration as a JSON object. Required for bedrock (must contain `region`), optional for google-vertex-ai (must contain `location`), and not supported by other adapters.",
			},
		}),
	}
}

func (r *organizationLlmConnectionResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var data organizationLlmConnectionModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() || data.Adapter.IsUnknown() || data.Config.IsUnknown() {
		return
	}

	requiredKey := map[string]string{"bedrock": "region", "google-vertex-ai": "location"}[data.Adapter.ValueString()]
	if data.Config.IsNull() {
		if data.Adapter.ValueString() == "bedrock" {
			resp.Diagnostics.AddAttributeError(path.Root("config"), "Config required for bedrock adapter",
				"The bedrock adapter requires a config JSON object containing a \"region\" key.")
		}
		return
	}
	if requiredKey == "" {
		resp.Diagnostics.AddAttributeError(path.Root("config"), "Config not supported for this adapter",
			fmt.Sprintf("The %q adapter does not support a config value.", data.Adapter.ValueString()))
		return
	}
	var config map[string]any
	resp.Diagnostics.Append(data.Config.Unmarshal(&config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if _, ok := config[requiredKey]; !ok {
		resp.Diagnostics.AddAttributeError(path.Root("config"), "Incomplete adapter config",
			fmt.Sprintf("The %q adapter config must contain a %q key.", data.Adapter.ValueString(), requiredKey))
	}
}

func organizationLlmConnectionRequestBody(ctx context.Context, plan organizationLlmConnectionModel) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics
	body := map[string]any{
		"provider":          plan.ProviderName.ValueString(),
		"adapter":           plan.Adapter.ValueString(),
		"secretKey":         plan.SecretKey.ValueString(),
		"withDefaultModels": plan.WithDefaultModels.ValueBool(),
	}
	if !plan.BaseURL.IsNull() {
		body["baseURL"] = plan.BaseURL.ValueString()
	}
	if !plan.CustomModels.IsNull() {
		var models []string
		diags.Append(plan.CustomModels.ElementsAs(ctx, &models, false)...)
		body["customModels"] = models
	}
	if !plan.ExtraHeaders.IsNull() {
		var headers map[string]string
		diags.Append(plan.ExtraHeaders.ElementsAs(ctx, &headers, false)...)
		body["extraHeaders"] = headers
	}
	config, err := jsonAttr(plan.Config)
	if err != nil {
		diags.AddAttributeError(path.Root("config"), "Invalid JSON", err.Error())
	} else if config != nil {
		body["config"] = config
	}
	return body, diags
}

// organizationLlmConnectionState maps an API connection onto prior, which
// supplies the credentials and the write-only secret_key and extra_headers.
func organizationLlmConnectionState(ctx context.Context, api organizationLlmConnectionAPI, prior organizationLlmConnectionModel) (organizationLlmConnectionModel, diag.Diagnostics) {
	var diags diag.Diagnostics
	state := prior
	state.ID = types.StringValue(api.ID)
	state.ProviderName = types.StringValue(api.Provider)
	state.Adapter = types.StringValue(api.Adapter)
	state.BaseURL = stringOrNull(api.BaseURL)
	state.WithDefaultModels = types.BoolValue(api.WithDefaultModels)

	state.CustomModels = types.ListNull(types.StringType)
	if len(api.CustomModels) > 0 {
		models, d := types.ListValueFrom(ctx, types.StringType, api.CustomModels)
		diags.Append(d...)
		state.CustomModels = models
	}

	config, err := keepJSON(prior.Config, api.Config)
	if err != nil {
		diags.AddError("Error reading LLM connection config", err.Error())
	}
	state.Config = config
	return state, diags
}

// find pages through the organization's connections; it returns errNotFound
// when none matches because the API has no endpoint to read a single one.
func (r *organizationLlmConnectionResource) find(ctx context.Context, client publicapi.ClientInterface, id string) (organizationLlmConnectionAPI, error) {
	limit := llmConnectionsPageSize
	for page := 1; ; page++ {
		var list struct {
			Data []organizationLlmConnectionAPI `json:"data"`
			Meta struct {
				TotalPages int `json:"totalPages"`
			} `json:"meta"`
		}
		params := &publicapi.LlmConnectionsListOrganizationParams{Page: &page, Limit: &limit}
		if err := apiCall(client.LlmConnectionsListOrganization(ctx, params)).decode(&list); err != nil {
			return organizationLlmConnectionAPI{}, err
		}
		for _, connection := range list.Data {
			if connection.ID == id {
				return connection, nil
			}
		}
		if page >= list.Meta.TotalPages {
			return organizationLlmConnectionAPI{}, fmt.Errorf("LLM connection %q: %w", id, errNotFound)
		}
	}
}

func (r *organizationLlmConnectionResource) save(ctx context.Context, plan organizationLlmConnectionModel) (organizationLlmConnectionModel, diag.Diagnostics) {
	body, diags := organizationLlmConnectionRequestBody(ctx, plan)
	if diags.HasError() {
		return plan, diags
	}
	reader, err := jsonBody(body)
	if err != nil {
		diags.AddError("Error encoding LLM connection", err.Error())
		return plan, diags
	}

	client := r.client(plan.OrganizationPublicKey, plan.OrganizationPrivateKey)
	var api organizationLlmConnectionAPI
	if err := apiCall(client.LlmConnectionsUpsertOrganizationWithBody(ctx, jsonContentType, reader)).decode(&api); err != nil {
		diags.AddError("Error saving organization LLM connection", err.Error())
		return plan, diags
	}
	state, d := organizationLlmConnectionState(ctx, api, plan)
	diags.Append(d...)
	return state, diags
}

func (r *organizationLlmConnectionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan organizationLlmConnectionModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	state, diags := r.save(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *organizationLlmConnectionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state organizationLlmConnectionModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	client := r.client(state.OrganizationPublicKey, state.OrganizationPrivateKey)
	api, err := r.find(ctx, client, state.ID.ValueString())
	if errors.Is(err, errNotFound) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Error reading organization LLM connection", err.Error())
		return
	}

	newState, diags := organizationLlmConnectionState(ctx, api, state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *organizationLlmConnectionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan organizationLlmConnectionModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	state, diags := r.save(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *organizationLlmConnectionResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state organizationLlmConnectionModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	client := r.client(state.OrganizationPublicKey, state.OrganizationPrivateKey)
	err := apiCall(client.LlmConnectionsDeleteOrganization(ctx, state.ID.ValueString())).decode(nil)
	if err != nil && !errors.Is(err, errNotFound) {
		resp.Diagnostics.AddError("Error deleting organization LLM connection", err.Error())
	}
}

func (r *organizationLlmConnectionResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	publicKey, privateKey, id, err := parseCredentialImportID(req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("organization_public_key"), publicKey)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("organization_private_key"), privateKey)...)
}
