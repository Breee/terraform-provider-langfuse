package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework-validators/datasourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/langfuse/terraform-provider-langfuse/langfuse"
	"github.com/langfuse/terraform-provider-langfuse/langfuse/publicapi"
)

// publicAPIDataSource is embedded by data sources that call the public API
// with an API key pair taken from their configuration.
type publicAPIDataSource struct {
	ClientFactory langfuse.ClientFactory
}

func (d *publicAPIDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	clientFactory, ok := req.ProviderData.(langfuse.ClientFactory)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected langfuse.ClientFactory, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}
	d.ClientFactory = clientFactory
}

func (d *publicAPIDataSource) client(publicKey, secretKey types.String) publicapi.ClientInterface {
	return d.ClientFactory.NewPublicAPIClient(publicKey.ValueString(), secretKey.ValueString())
}

func dataSourceCredentials(attrs map[string]schema.Attribute, scope, secretName string) map[string]schema.Attribute {
	attrs[scope+"_public_key"] = schema.StringAttribute{
		Required:    true,
		Sensitive:   true,
		Description: "The " + scope + " public key used to authenticate API calls.",
	}
	attrs[scope+"_"+secretName+"_key"] = schema.StringAttribute{
		Required:    true,
		Sensitive:   true,
		Description: "The " + scope + " " + secretName + " key used to authenticate API calls.",
	}
	return attrs
}

func withProjectDataSourceCredentials(attrs map[string]schema.Attribute) map[string]schema.Attribute {
	return dataSourceCredentials(attrs, "project", "secret")
}

func withOrganizationDataSourceCredentials(attrs map[string]schema.Attribute) map[string]schema.Attribute {
	return dataSourceCredentials(attrs, "organization", "private")
}

func computedString(description string) schema.StringAttribute {
	return schema.StringAttribute{Computed: true, Description: description}
}

func computedJSON(description string) schema.StringAttribute {
	return schema.StringAttribute{Computed: true, CustomType: jsontypes.NormalizedType{}, Description: description}
}

// --- langfuse_project ---

func NewProjectDataSource() datasource.DataSource { return &projectDataSource{} }

type projectDataSource struct{ publicAPIDataSource }

type projectDataSourceModel struct {
	ProjectPublicKey types.String         `tfsdk:"project_public_key"`
	ProjectSecretKey types.String         `tfsdk:"project_secret_key"`
	ID               types.String         `tfsdk:"id"`
	Name             types.String         `tfsdk:"name"`
	OrganizationID   types.String         `tfsdk:"organization_id"`
	OrganizationName types.String         `tfsdk:"organization_name"`
	Metadata         jsontypes.Normalized `tfsdk:"metadata"`
}

func (d *projectDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project"
}

func (d *projectDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Look up the project that a project-scoped API key belongs to.",
		Attributes: withProjectDataSourceCredentials(map[string]schema.Attribute{
			"id":                computedString("The project ID."),
			"name":              computedString("The project name."),
			"organization_id":   computedString("The ID of the organization that owns the project."),
			"organization_name": computedString("The name of the organization that owns the project."),
			"metadata":          computedJSON("The project metadata as a JSON object."),
		}),
	}
}

func (d *projectDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data projectDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var list struct {
		Data []struct {
			ID           string `json:"id"`
			Name         string `json:"name"`
			Metadata     any    `json:"metadata"`
			Organization struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"organization"`
		} `json:"data"`
	}
	client := d.client(data.ProjectPublicKey, data.ProjectSecretKey)
	if err := apiCall(client.ProjectsGet(ctx)).decode(&list); err != nil {
		resp.Diagnostics.AddError("Error reading project", err.Error())
		return
	}
	if len(list.Data) != 1 {
		resp.Diagnostics.AddError("Error reading project", fmt.Sprintf("Expected exactly one project for the API key, got %d.", len(list.Data)))
		return
	}

	project := list.Data[0]
	metadata, err := jsonValue(project.Metadata)
	if err != nil {
		resp.Diagnostics.AddError("Error reading project metadata", err.Error())
		return
	}
	data.ID = types.StringValue(project.ID)
	data.Name = types.StringValue(project.Name)
	data.OrganizationID = types.StringValue(project.Organization.ID)
	data.OrganizationName = types.StringValue(project.Organization.Name)
	data.Metadata = metadata
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// --- langfuse_projects ---

func NewProjectsDataSource() datasource.DataSource { return &projectsDataSource{} }

type projectsDataSource struct{ publicAPIDataSource }

type projectsDataSourceModel struct {
	OrganizationPublicKey  types.String `tfsdk:"organization_public_key"`
	OrganizationPrivateKey types.String `tfsdk:"organization_private_key"`
	Projects               types.List   `tfsdk:"projects"`
}

var projectsItemType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"id":       types.StringType,
	"name":     types.StringType,
	"metadata": jsontypes.NormalizedType{},
}}

type projectsItem struct {
	ID       types.String         `tfsdk:"id"`
	Name     types.String         `tfsdk:"name"`
	Metadata jsontypes.Normalized `tfsdk:"metadata"`
}

func (d *projectsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_projects"
}

func (d *projectsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "List the projects of the organization that an organization-scoped API key belongs to.",
		Attributes: withOrganizationDataSourceCredentials(map[string]schema.Attribute{
			"projects": schema.ListNestedAttribute{
				Computed:    true,
				Description: "The projects of the organization.",
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"id":       computedString("The project ID."),
					"name":     computedString("The project name."),
					"metadata": computedJSON("The project metadata as a JSON object."),
				}},
			},
		}),
	}
}

func (d *projectsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data projectsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var list struct {
		Projects []struct {
			ID       string `json:"id"`
			Name     string `json:"name"`
			Metadata any    `json:"metadata"`
		} `json:"projects"`
	}
	client := d.client(data.OrganizationPublicKey, data.OrganizationPrivateKey)
	if err := apiCall(client.OrganizationsGetOrganizationProjects(ctx)).decode(&list); err != nil {
		resp.Diagnostics.AddError("Error listing projects", err.Error())
		return
	}

	items := make([]projectsItem, 0, len(list.Projects))
	for _, project := range list.Projects {
		metadata, err := jsonValue(project.Metadata)
		if err != nil {
			resp.Diagnostics.AddError("Error reading project metadata", err.Error())
			return
		}
		items = append(items, projectsItem{ID: types.StringValue(project.ID), Name: types.StringValue(project.Name), Metadata: metadata})
	}
	projects, diags := types.ListValueFrom(ctx, projectsItemType, items)
	resp.Diagnostics.Append(diags...)
	data.Projects = projects
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// --- langfuse_user ---

func NewUserDataSource() datasource.DataSource { return &userDataSource{} }

type userDataSource struct{ publicAPIDataSource }

type userDataSourceModel struct {
	OrganizationPublicKey  types.String `tfsdk:"organization_public_key"`
	OrganizationPrivateKey types.String `tfsdk:"organization_private_key"`
	Email                  types.String `tfsdk:"email"`
	ID                     types.String `tfsdk:"id"`
	Name                   types.String `tfsdk:"name"`
}

func (d *userDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user"
}

func (d *userDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Look up a member of the organization by email address through the SCIM API, for example to obtain " +
			"the user ID for an annotation queue assignment. Requires an organization-scoped API key.",
		Attributes: withOrganizationDataSourceCredentials(map[string]schema.Attribute{
			"email": schema.StringAttribute{Required: true, Description: "The email address of the user."},
			"id":    computedString("The user ID."),
			"name":  computedString("The display name of the user."),
		}),
	}
}

func (d *userDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data userDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var list struct {
		Resources []struct {
			ID   string `json:"id"`
			Name struct {
				Formatted *string `json:"formatted"`
			} `json:"name"`
		} `json:"Resources"`
	}
	filter := fmt.Sprintf("userName eq %q", data.Email.ValueString())
	client := d.client(data.OrganizationPublicKey, data.OrganizationPrivateKey)
	if err := apiCall(client.ScimListUsers(ctx, &publicapi.ScimListUsersParams{Filter: &filter})).decode(&list); err != nil {
		resp.Diagnostics.AddError("Error looking up user", err.Error())
		return
	}
	if len(list.Resources) != 1 {
		resp.Diagnostics.AddAttributeError(path.Root("email"), "User not found",
			fmt.Sprintf("Expected exactly one organization member with email %q, got %d.", data.Email.ValueString(), len(list.Resources)))
		return
	}

	data.ID = types.StringValue(list.Resources[0].ID)
	data.Name = stringOrNull(list.Resources[0].Name.Formatted)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// --- langfuse_prompt ---

func NewPromptDataSource() datasource.DataSource { return &promptDataSource{} }

type promptDataSource struct{ publicAPIDataSource }

var _ datasource.DataSourceWithConfigValidators = &promptDataSource{}

type promptDataSourceModel struct {
	ProjectPublicKey types.String         `tfsdk:"project_public_key"`
	ProjectSecretKey types.String         `tfsdk:"project_secret_key"`
	Name             types.String         `tfsdk:"name"`
	Version          types.Int64          `tfsdk:"version"`
	Label            types.String         `tfsdk:"label"`
	Type             types.String         `tfsdk:"type"`
	Prompt           types.String         `tfsdk:"prompt"`
	Config           jsontypes.Normalized `tfsdk:"config"`
	Labels           types.Set            `tfsdk:"labels"`
	Tags             types.Set            `tfsdk:"tags"`
	CommitMessage    types.String         `tfsdk:"commit_message"`
}

func (d *promptDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_prompt"
}

func (d *promptDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Read one version of a prompt. Select it by `version` or by `label`; without either, Langfuse returns " +
			"the version labelled `production`. Prompt dependencies are resolved in the returned content.",
		Attributes: withProjectDataSourceCredentials(map[string]schema.Attribute{
			"name": schema.StringAttribute{Required: true, Description: "The prompt name."},
			"version": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "The version to read. Conflicts with `label`. Always set to the version that was read.",
			},
			"label": schema.StringAttribute{
				Optional:    true,
				Description: "The label of the version to read, for example `production` or `latest`. Conflicts with `version`.",
			},
			"type":   computedString("The prompt type: `text` or `chat`."),
			"prompt": computedString("The prompt content. Chat prompts are returned as a JSON-encoded list of messages; decode with `jsondecode`."),
			"config": computedJSON("The prompt config as a JSON value."),
			"labels": schema.SetAttribute{
				Computed: true, ElementType: types.StringType, Description: "The labels of the version that was read.",
			},
			"tags": schema.SetAttribute{
				Computed: true, ElementType: types.StringType, Description: "The tags of the prompt.",
			},
			"commit_message": computedString("The commit message of the version that was read."),
		}),
	}
}

func (d *promptDataSource) ConfigValidators(context.Context) []datasource.ConfigValidator {
	return []datasource.ConfigValidator{
		datasourcevalidator.Conflicting(path.MatchRoot("version"), path.MatchRoot("label")),
	}
}

func (d *promptDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data promptDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	params := &publicapi.PromptsGetParams{Label: data.Label.ValueStringPointer()}
	if !data.Version.IsNull() {
		version := int(data.Version.ValueInt64())
		params.Version = &version
	}
	var prompt promptAPI
	client := d.client(data.ProjectPublicKey, data.ProjectSecretKey)
	if err := apiCall(client.PromptsGet(ctx, data.Name.ValueString(), params)).decode(&prompt); err != nil {
		resp.Diagnostics.AddError("Error reading prompt", err.Error())
		return
	}

	data.Version = types.Int64Value(prompt.Version)
	data.Type = types.StringValue(prompt.Type)
	data.CommitMessage = stringOrNull(prompt.CommitMessage)
	if text, ok := prompt.Prompt.(string); ok {
		data.Prompt = types.StringValue(text)
	} else {
		messages, err := jsonValue(prompt.Prompt)
		if err != nil {
			resp.Diagnostics.AddError("Error reading prompt content", err.Error())
			return
		}
		data.Prompt = types.StringValue(messages.ValueString())
	}
	config, err := jsonValue(prompt.Config)
	if err != nil {
		resp.Diagnostics.AddError("Error reading prompt config", err.Error())
		return
	}
	data.Config = config

	// A non-null prior makes promptSet return empty sets instead of null.
	empty := types.SetValueMust(types.StringType, nil)
	labels, diags := promptSet(ctx, prompt.Labels, empty)
	resp.Diagnostics.Append(diags...)
	tags, diags := promptSet(ctx, prompt.Tags, empty)
	resp.Diagnostics.Append(diags...)
	data.Labels, data.Tags = labels, tags
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// --- langfuse_score_config ---

func NewScoreConfigDataSource() datasource.DataSource { return &scoreConfigDataSource{} }

type scoreConfigDataSource struct{ publicAPIDataSource }

type scoreConfigDataSourceModel struct {
	ProjectPublicKey types.String  `tfsdk:"project_public_key"`
	ProjectSecretKey types.String  `tfsdk:"project_secret_key"`
	Name             types.String  `tfsdk:"name"`
	ID               types.String  `tfsdk:"id"`
	DataType         types.String  `tfsdk:"data_type"`
	Description      types.String  `tfsdk:"description"`
	MinValue         types.Float64 `tfsdk:"min_value"`
	MaxValue         types.Float64 `tfsdk:"max_value"`
	Categories       types.List    `tfsdk:"categories"`
}

func (d *scoreConfigDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_score_config"
}

func (d *scoreConfigDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Look up a score config by name, for example to reference it from an annotation queue. " +
			"Archived score configs are ignored; the lookup fails unless exactly one active score config has the name.",
		Attributes: withProjectDataSourceCredentials(map[string]schema.Attribute{
			"name":        schema.StringAttribute{Required: true, Description: "The name of the score config."},
			"id":          computedString("The score config ID."),
			"data_type":   computedString("The data type: `NUMERIC`, `CATEGORICAL`, `BOOLEAN`, or `TEXT`."),
			"description": computedString("The description of the score config."),
			"min_value":   schema.Float64Attribute{Computed: true, Description: "The minimum value of a numeric score config."},
			"max_value":   schema.Float64Attribute{Computed: true, Description: "The maximum value of a numeric score config."},
			"categories": schema.ListNestedAttribute{
				Computed:    true,
				Description: "The categories of a categorical or boolean score config.",
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"label": computedString("The category label."),
					"value": schema.Float64Attribute{Computed: true, Description: "The numeric value of the category."},
				}},
			},
		}),
	}
}

func (d *scoreConfigDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data scoreConfigDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	client := d.client(data.ProjectPublicKey, data.ProjectSecretKey)
	var matches []scoreConfigAPI
	limit := 100
	for page := 1; ; page++ {
		var list struct {
			Data []scoreConfigAPI `json:"data"`
			Meta struct {
				TotalPages int `json:"totalPages"`
			} `json:"meta"`
		}
		params := &publicapi.ScoreConfigsGetParams{Page: &page, Limit: &limit}
		if err := apiCall(client.ScoreConfigsGet(ctx, params)).decode(&list); err != nil {
			resp.Diagnostics.AddError("Error listing score configs", err.Error())
			return
		}
		for _, config := range list.Data {
			if config.Name == data.Name.ValueString() && !config.IsArchived {
				matches = append(matches, config)
			}
		}
		if page >= list.Meta.TotalPages {
			break
		}
	}
	if len(matches) != 1 {
		resp.Diagnostics.AddAttributeError(path.Root("name"), "Score config not found",
			fmt.Sprintf("Expected exactly one active score config named %q, got %d.", data.Name.ValueString(), len(matches)))
		return
	}

	config := matches[0]
	categories := make([]scoreConfigCategory, 0, len(config.Categories))
	for _, category := range config.Categories {
		categories = append(categories, scoreConfigCategory{
			Label: types.StringValue(category.Label),
			Value: types.Float64Value(category.Value),
		})
	}
	list, diags := types.ListValueFrom(ctx, scoreConfigCategoryType, categories)
	resp.Diagnostics.Append(diags...)

	data.ID = types.StringValue(config.ID)
	data.DataType = types.StringValue(config.DataType)
	data.Description = stringOrNull(config.Description)
	data.MinValue = float64OrNull(config.MinValue)
	data.MaxValue = float64OrNull(config.MaxValue)
	data.Categories = list
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// --- langfuse_dataset ---

func NewDatasetDataSource() datasource.DataSource { return &datasetDataSource{} }

type datasetDataSource struct{ publicAPIDataSource }

type datasetDataSourceModel struct {
	ProjectPublicKey     types.String         `tfsdk:"project_public_key"`
	ProjectSecretKey     types.String         `tfsdk:"project_secret_key"`
	Name                 types.String         `tfsdk:"name"`
	ID                   types.String         `tfsdk:"id"`
	Description          types.String         `tfsdk:"description"`
	Metadata             jsontypes.Normalized `tfsdk:"metadata"`
	InputSchema          jsontypes.Normalized `tfsdk:"input_schema"`
	ExpectedOutputSchema jsontypes.Normalized `tfsdk:"expected_output_schema"`
}

func (d *datasetDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dataset"
}

func (d *datasetDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Look up a dataset by name.",
		Attributes: withProjectDataSourceCredentials(map[string]schema.Attribute{
			"name":                   schema.StringAttribute{Required: true, Description: "The dataset name."},
			"id":                     computedString("The dataset ID."),
			"description":            computedString("The dataset description."),
			"metadata":               computedJSON("The dataset metadata as a JSON value."),
			"input_schema":           computedJSON("The JSON Schema that item inputs are validated against."),
			"expected_output_schema": computedJSON("The JSON Schema that expected outputs are validated against."),
		}),
	}
}

func (d *datasetDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data datasetDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var dataset datasetAPI
	client := d.client(data.ProjectPublicKey, data.ProjectSecretKey)
	if err := apiCall(client.DatasetsGet(ctx, data.Name.ValueString())).decode(&dataset); err != nil {
		resp.Diagnostics.AddError("Error reading dataset", err.Error())
		return
	}

	data.ID = types.StringValue(dataset.ID)
	data.Description = stringOrNull(dataset.Description)
	for _, field := range []struct {
		target *jsontypes.Normalized
		value  any
	}{
		{&data.Metadata, dataset.Metadata},
		{&data.InputSchema, dataset.InputSchema},
		{&data.ExpectedOutputSchema, dataset.ExpectedOutputSchema},
	} {
		value, err := jsonValue(field.value)
		if err != nil {
			resp.Diagnostics.AddError("Error reading dataset", err.Error())
			return
		}
		*field.target = value
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
