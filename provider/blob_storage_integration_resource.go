package provider

import (
	"context"
	"errors"

	"github.com/hashicorp/terraform-plugin-framework-timetypes/timetypes"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &blobStorageIntegrationResource{}
var _ resource.ResourceWithImportState = &blobStorageIntegrationResource{}

func NewBlobStorageIntegrationResource() resource.Resource {
	return &blobStorageIntegrationResource{}
}

type blobStorageIntegrationResource struct {
	publicAPIResource
}

type blobStorageIntegrationResourceModel struct {
	ID                     types.String      `tfsdk:"id"`
	OrganizationPublicKey  types.String      `tfsdk:"organization_public_key"`
	OrganizationPrivateKey types.String      `tfsdk:"organization_private_key"`
	ProjectID              types.String      `tfsdk:"project_id"`
	Type                   types.String      `tfsdk:"type"`
	BucketName             types.String      `tfsdk:"bucket_name"`
	Endpoint               types.String      `tfsdk:"endpoint"`
	Region                 types.String      `tfsdk:"region"`
	AuthType               types.String      `tfsdk:"auth_type"`
	RoleArn                types.String      `tfsdk:"role_arn"`
	AccessKeyID            types.String      `tfsdk:"access_key_id"`
	SecretAccessKey        types.String      `tfsdk:"secret_access_key"`
	Prefix                 types.String      `tfsdk:"prefix"`
	ExportFrequency        types.String      `tfsdk:"export_frequency"`
	Enabled                types.Bool        `tfsdk:"enabled"`
	ForcePathStyle         types.Bool        `tfsdk:"force_path_style"`
	FileType               types.String      `tfsdk:"file_type"`
	ExportMode             types.String      `tfsdk:"export_mode"`
	ExportStartDate        timetypes.RFC3339 `tfsdk:"export_start_date"`
	Compressed             types.Bool        `tfsdk:"compressed"`
	ExportSource           types.String      `tfsdk:"export_source"`
	ExportFieldGroups      types.Set         `tfsdk:"export_field_groups"`
	ExternalID             types.String      `tfsdk:"external_id"`
}

type blobStorageIntegrationAPI struct {
	ID                string   `json:"id"`
	ProjectID         string   `json:"projectId"`
	Type              string   `json:"type"`
	BucketName        string   `json:"bucketName"`
	Endpoint          *string  `json:"endpoint"`
	Region            string   `json:"region"`
	AuthType          string   `json:"authType"`
	RoleArn           *string  `json:"roleArn"`
	AccessKeyID       *string  `json:"accessKeyId"`
	ExternalID        *string  `json:"externalId"`
	Prefix            string   `json:"prefix"`
	ExportFrequency   string   `json:"exportFrequency"`
	Enabled           bool     `json:"enabled"`
	ForcePathStyle    bool     `json:"forcePathStyle"`
	FileType          string   `json:"fileType"`
	ExportMode        string   `json:"exportMode"`
	ExportStartDate   *string  `json:"exportStartDate"`
	Compressed        bool     `json:"compressed"`
	ExportSource      string   `json:"exportSource"`
	ExportFieldGroups []string `json:"exportFieldGroups"`
}

func (r *blobStorageIntegrationResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_blob_storage_integration"
}

func (r *blobStorageIntegrationResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages the scheduled blob storage export of a Langfuse project. " +
			"A project has at most one integration, so its ID equals the project ID. " +
			"Requires an organization-scoped API key.",
		Attributes: withOrganizationCredentials(map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "The integration ID, which equals the project ID.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"project_id": schema.StringAttribute{
				Required:      true,
				Description:   "The ID of the project whose data is exported.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"type": schema.StringAttribute{
				Required:    true,
				Description: "The storage type: `S3`, `S3_COMPATIBLE`, `AZURE_BLOB_STORAGE`, or `GOOGLE_CLOUD_STORAGE`.",
				Validators: []validator.String{
					stringvalidator.OneOf("S3", "S3_COMPATIBLE", "AZURE_BLOB_STORAGE", "GOOGLE_CLOUD_STORAGE"),
				},
			},
			"bucket_name": schema.StringAttribute{
				Required:    true,
				Description: "The bucket (or container) to export to.",
			},
			"endpoint": schema.StringAttribute{
				Optional:    true,
				Description: "The storage endpoint URL. Required for `S3_COMPATIBLE` and `AZURE_BLOB_STORAGE`.",
			},
			"region": schema.StringAttribute{
				Required:    true,
				Description: "The storage region.",
			},
			"auth_type": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("ACCESS_KEYS"),
				Description: "The authentication type: `ACCESS_KEYS` (default) or `ROLE_ARN`.",
				Validators:  []validator.String{stringvalidator.OneOf("ACCESS_KEYS", "ROLE_ARN")},
			},
			"role_arn": schema.StringAttribute{
				Optional:    true,
				Description: "The AWS role ARN to assume when `auth_type` is `ROLE_ARN`.",
			},
			"access_key_id": schema.StringAttribute{
				Optional:    true,
				Description: "The access key ID used when `auth_type` is `ACCESS_KEYS`.",
			},
			"secret_access_key": schema.StringAttribute{
				Optional:  true,
				Sensitive: true,
				Description: "The secret access key used when `auth_type` is `ACCESS_KEYS`. " +
					"The API never returns it, so it cannot be imported and drift is not detected.",
			},
			"prefix": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString(""),
				Description: "The object key prefix for exported files. Defaults to an empty prefix.",
			},
			"export_frequency": schema.StringAttribute{
				Required:    true,
				Description: "How often data is exported: `every_20_minutes`, `hourly`, `daily`, or `weekly`.",
				Validators: []validator.String{
					stringvalidator.OneOf("every_20_minutes", "hourly", "daily", "weekly"),
				},
			},
			"enabled": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(true),
				Description: "Whether the export is active. Defaults to `true`.",
			},
			"force_path_style": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
				Description: "Whether to use path-style bucket addressing. Defaults to `false`.",
			},
			"file_type": schema.StringAttribute{
				Required:    true,
				Description: "The export file format: `JSON`, `CSV`, `JSONL`, or `PARQUET`.",
				Validators:  []validator.String{stringvalidator.OneOf("JSON", "CSV", "JSONL", "PARQUET")},
			},
			"export_mode": schema.StringAttribute{
				Required:    true,
				Description: "Which data to export: `FULL_HISTORY`, `FROM_TODAY`, or `FROM_CUSTOM_DATE`.",
				Validators: []validator.String{
					stringvalidator.OneOf("FULL_HISTORY", "FROM_TODAY", "FROM_CUSTOM_DATE"),
				},
			},
			"export_start_date": schema.StringAttribute{
				CustomType:  timetypes.RFC3339Type{},
				Optional:    true,
				Description: "The RFC 3339 start date used when `export_mode` is `FROM_CUSTOM_DATE`.",
			},
			"compressed": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(true),
				Description: "Whether exported files are compressed. Defaults to `true`.",
			},
			"export_source": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Description: "The data source to export: `LEGACY_TRACES_OBSERVATIONS`, `OBSERVATIONS_V2`, or " +
					"`LEGACY_TRACES_AND_ENRICHED_OBSERVATIONS`. Defaults to the server default.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
				Validators: []validator.String{
					stringvalidator.OneOf("LEGACY_TRACES_OBSERVATIONS", "OBSERVATIONS_V2", "LEGACY_TRACES_AND_ENRICHED_OBSERVATIONS"),
				},
			},
			"export_field_groups": schema.SetAttribute{
				ElementType: types.StringType,
				Optional:    true,
				Computed:    true,
				Description: "The field groups to include in the export. Defaults to the server default. " +
					"Setting it requires `export_source`.",
				PlanModifiers: []planmodifier.Set{setplanmodifier.UseStateForUnknown()},
			},
			"external_id": schema.StringAttribute{
				Computed:      true,
				Description:   "The external ID Langfuse presents when assuming `role_arn`.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
		}),
	}
}

func (r *blobStorageIntegrationResource) upsert(ctx context.Context, plan blobStorageIntegrationResourceModel) (blobStorageIntegrationResourceModel, diag.Diagnostics) {
	var diags diag.Diagnostics
	body := map[string]any{
		"projectId":       plan.ProjectID.ValueString(),
		"type":            plan.Type.ValueString(),
		"bucketName":      plan.BucketName.ValueString(),
		"endpoint":        plan.Endpoint.ValueStringPointer(),
		"region":          plan.Region.ValueString(),
		"authType":        plan.AuthType.ValueString(),
		"roleArn":         plan.RoleArn.ValueStringPointer(),
		"accessKeyId":     plan.AccessKeyID.ValueStringPointer(),
		"prefix":          plan.Prefix.ValueString(),
		"exportFrequency": plan.ExportFrequency.ValueString(),
		"enabled":         plan.Enabled.ValueBool(),
		"forcePathStyle":  plan.ForcePathStyle.ValueBool(),
		"fileType":        plan.FileType.ValueString(),
		"exportMode":      plan.ExportMode.ValueString(),
		"exportStartDate": plan.ExportStartDate.ValueStringPointer(),
		"compressed":      plan.Compressed.ValueBool(),
	}
	if !plan.SecretAccessKey.IsNull() {
		body["secretAccessKey"] = plan.SecretAccessKey.ValueString()
	}
	if !plan.ExportSource.IsNull() && !plan.ExportSource.IsUnknown() {
		body["exportSource"] = plan.ExportSource.ValueString()
	}
	if !plan.ExportFieldGroups.IsNull() && !plan.ExportFieldGroups.IsUnknown() {
		var groups []string
		diags.Append(plan.ExportFieldGroups.ElementsAs(ctx, &groups, false)...)
		body["exportFieldGroups"] = groups
	}
	if diags.HasError() {
		return plan, diags
	}
	reader, err := jsonBody(body)
	if err != nil {
		diags.AddError("Error encoding blob storage integration", err.Error())
		return plan, diags
	}
	var integration blobStorageIntegrationAPI
	client := r.client(plan.OrganizationPublicKey, plan.OrganizationPrivateKey)
	err = apiCall(client.BlobStorageIntegrationsUpsertBlobStorageIntegrationWithBody(ctx, jsonContentType, reader)).decode(&integration)
	if err != nil {
		diags.AddError("Error saving blob storage integration", err.Error())
		return plan, diags
	}
	return blobStorageIntegrationState(ctx, integration, plan)
}

func blobStorageIntegrationState(ctx context.Context, api blobStorageIntegrationAPI, prior blobStorageIntegrationResourceModel) (blobStorageIntegrationResourceModel, diag.Diagnostics) {
	var diags diag.Diagnostics
	state := prior
	state.ID = types.StringValue(api.ID)
	state.ProjectID = types.StringValue(api.ProjectID)
	state.Type = types.StringValue(api.Type)
	state.BucketName = types.StringValue(api.BucketName)
	state.Endpoint = stringOrNull(api.Endpoint)
	state.Region = types.StringValue(api.Region)
	state.AuthType = types.StringValue(api.AuthType)
	state.RoleArn = stringOrNull(api.RoleArn)
	state.AccessKeyID = stringOrNull(api.AccessKeyID)
	state.Prefix = types.StringValue(api.Prefix)
	state.ExportFrequency = types.StringValue(api.ExportFrequency)
	state.Enabled = types.BoolValue(api.Enabled)
	state.ForcePathStyle = types.BoolValue(api.ForcePathStyle)
	state.FileType = types.StringValue(api.FileType)
	state.ExportMode = types.StringValue(api.ExportMode)
	state.Compressed = types.BoolValue(api.Compressed)
	state.ExportSource = types.StringValue(api.ExportSource)
	state.ExternalID = stringOrNull(api.ExternalID)

	state.ExportStartDate = timetypes.NewRFC3339Null()
	if api.ExportStartDate != nil {
		date, d := timetypes.NewRFC3339Value(*api.ExportStartDate)
		diags.Append(d...)
		state.ExportStartDate = date
	}
	groups, d := types.SetValueFrom(ctx, types.StringType, api.ExportFieldGroups)
	diags.Append(d...)
	state.ExportFieldGroups = groups
	return state, diags
}

func (r *blobStorageIntegrationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan blobStorageIntegrationResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	state, diags := r.upsert(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Read lists the organization's integrations because the API only exposes
// the sync status, not the configuration, for a single integration.
func (r *blobStorageIntegrationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state blobStorageIntegrationResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var list struct {
		Data []blobStorageIntegrationAPI `json:"data"`
	}
	client := r.client(state.OrganizationPublicKey, state.OrganizationPrivateKey)
	if err := apiCall(client.BlobStorageIntegrationsGetBlobStorageIntegrations(ctx)).decode(&list); err != nil {
		resp.Diagnostics.AddError("Error reading blob storage integration", err.Error())
		return
	}
	for _, integration := range list.Data {
		if integration.ID == state.ID.ValueString() {
			newState, diags := blobStorageIntegrationState(ctx, integration, state)
			resp.Diagnostics.Append(diags...)
			resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
			return
		}
	}
	resp.State.RemoveResource(ctx)
}

func (r *blobStorageIntegrationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan blobStorageIntegrationResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	state, diags := r.upsert(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *blobStorageIntegrationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state blobStorageIntegrationResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	client := r.client(state.OrganizationPublicKey, state.OrganizationPrivateKey)
	err := apiCall(client.BlobStorageIntegrationsDeleteBlobStorageIntegration(ctx, state.ID.ValueString())).decode(nil)
	if err != nil && !errors.Is(err, errNotFound) {
		resp.Diagnostics.AddError("Error deleting blob storage integration", err.Error())
	}
}

func (r *blobStorageIntegrationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	publicKey, privateKey, id, err := parseCredentialImportID(req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("organization_public_key"), publicKey)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("organization_private_key"), privateKey)...)
}
