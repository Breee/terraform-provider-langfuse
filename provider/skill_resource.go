package provider

import (
	"context"
	"errors"
	"maps"
	"regexp"
	"slices"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/langfuse/terraform-provider-langfuse/langfuse/publicapi"
)

var _ resource.Resource = &skillResource{}
var _ resource.ResourceWithImportState = &skillResource{}
var _ resource.ResourceWithModifyPlan = &skillResource{}

const skillManifest = "SKILL.md"

var (
	skillNamePattern        = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
	skillFrontmatterPattern = regexp.MustCompile(`(?m)^name:[ \t]*["']?([^"'\r\n#]*?)["']?[ \t]*(?:#.*)?\r?$`)
)

func NewSkillResource() resource.Resource {
	return &skillResource{}
}

type skillResource struct {
	publicAPIResource
}

type skillResourceModel struct {
	ID               types.String `tfsdk:"id"`
	ProjectPublicKey types.String `tfsdk:"project_public_key"`
	ProjectSecretKey types.String `tfsdk:"project_secret_key"`
	Name             types.String `tfsdk:"name"`
	Files            types.Map    `tfsdk:"files"`
	CommitMessage    types.String `tfsdk:"commit_message"`
	Labels           types.Set    `tfsdk:"labels"`
	Tags             types.Set    `tfsdk:"tags"`
	Description      types.String `tfsdk:"description"`
	Version          types.Int64  `tfsdk:"version"`
}

type skillAPI struct {
	Name          string   `json:"name"`
	Description   string   `json:"description"`
	Version       int64    `json:"version"`
	Tags          []string `json:"tags"`
	Labels        []string `json:"labels"`
	CommitMessage *string  `json:"commitMessage"`
	Files         []struct {
		Path    string  `json:"path"`
		Content *string `json:"content"`
	} `json:"files"`
}

func (r *skillResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_skill"
}

func (r *skillResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manage a skill in a project using project-scoped credentials. " +
			"A skill is a versioned set of text files with a root `SKILL.md` whose YAML frontmatter holds the skill `name` and `description`. " +
			"The resource tracks one version: changing the files or the commit message creates a new version, while labels and tags are changed in place. " +
			"Versions created outside Terraform are not tracked. Destroying the resource deletes all versions of the skill. " +
			"Import format: project_public_key:project_secret_key:skill_name (imports the latest version).",
		Attributes: withProjectCredentials(map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "The name of the skill.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Name of the skill: lowercase letters, numbers, and single hyphens. It must equal the `name` in the `SKILL.md` frontmatter. Changing it forces a new skill.",
				Validators: []validator.String{
					stringvalidator.RegexMatches(skillNamePattern, "must contain only lowercase letters, numbers, and single hyphens"),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"files": schema.MapAttribute{
				ElementType: types.StringType,
				Required:    true,
				Description: "Files of the skill as a map of path to UTF-8 text content. Must contain a root `SKILL.md` that starts with YAML frontmatter holding `name` and `description`.",
			},
			"commit_message": schema.StringAttribute{
				Optional:    true,
				Description: "Commit message of the tracked version.",
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
				Description: "Tags of the skill. Tags apply to all versions of the skill.",
			},
			"description": schema.StringAttribute{
				Computed:    true,
				Description: "Description of the skill, read from the `SKILL.md` frontmatter.",
			},
			"version": schema.Int64Attribute{
				Computed:    true,
				Description: "Version number tracked by this resource.",
			},
		}),
	}
}

// skillFrontmatterName returns the name declared in the SKILL.md frontmatter,
// or "" when it cannot be determined.
func skillFrontmatterName(manifest string) string {
	rest, ok := strings.CutPrefix(strings.TrimPrefix(manifest, "\ufeff"), "---")
	if !ok {
		return ""
	}
	frontmatter, _, ok := strings.Cut(rest, "\n---")
	if !ok {
		return ""
	}
	match := skillFrontmatterPattern.FindStringSubmatch(frontmatter)
	if match == nil {
		return ""
	}
	return strings.TrimSpace(match[1])
}

func skillNeedsVersion(plan, state skillResourceModel) bool {
	return !plan.Files.Equal(state.Files) || !plan.CommitMessage.Equal(state.CommitMessage)
}

func (r *skillResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.Plan.Raw.IsNull() {
		return
	}
	var plan skillResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !plan.Files.IsUnknown() && !plan.Name.IsUnknown() {
		manifest, ok := plan.Files.Elements()[skillManifest].(types.String)
		switch {
		case !ok:
			resp.Diagnostics.AddAttributeError(path.Root("files"), "Missing SKILL.md",
				"files must contain a root SKILL.md.")
		case !manifest.IsUnknown():
			if name := skillFrontmatterName(manifest.ValueString()); name != "" && name != plan.Name.ValueString() {
				resp.Diagnostics.AddAttributeError(path.Root("name"), "Skill name mismatch",
					"name is \""+plan.Name.ValueString()+"\" but the SKILL.md frontmatter declares \""+name+"\".")
			}
		}
	}
	if req.State.Raw.IsNull() || resp.Diagnostics.HasError() {
		return
	}
	var state skillResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	version, description := state.Version, state.Description
	if skillNeedsVersion(plan, state) {
		version, description = types.Int64Unknown(), types.StringUnknown()
	}
	resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root("version"), version)...)
	resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root("description"), description)...)
}

func skillLabels(labels []string) []string {
	return slices.DeleteFunc(slices.Clone(labels), func(l string) bool { return l == promptLatestLabel })
}

func sameStrings(a, b []string) bool {
	a, b = slices.Clone(a), slices.Clone(b)
	slices.Sort(a)
	slices.Sort(b)
	return slices.Equal(a, b)
}

// skillState maps an API skill version onto prior (the plan or previous
// state), which supplies the credentials. files is the file map to store.
func skillState(ctx context.Context, s skillAPI, prior skillResourceModel, files types.Map) (skillResourceModel, diag.Diagnostics) {
	var diags, d diag.Diagnostics
	state := skillResourceModel{
		ID:               types.StringValue(s.Name),
		ProjectPublicKey: prior.ProjectPublicKey,
		ProjectSecretKey: prior.ProjectSecretKey,
		Name:             types.StringValue(s.Name),
		Files:            files,
		CommitMessage:    stringOrNull(s.CommitMessage),
		Description:      types.StringValue(s.Description),
		Version:          types.Int64Value(s.Version),
	}
	state.Labels, d = promptSet(ctx, skillLabels(s.Labels), prior.Labels)
	diags.Append(d...)
	state.Tags, d = promptSet(ctx, s.Tags, prior.Tags)
	diags.Append(d...)
	return state, diags
}

// apply brings the tags and labels of the skill version in current in line
// with the plan and returns the resulting version.
func (r *skillResource) apply(ctx context.Context, plan skillResourceModel, current skillAPI) (skillAPI, diag.Diagnostics) {
	client := r.client(plan.ProjectPublicKey, plan.ProjectSecretKey)
	tags, diags := promptStrings(ctx, plan.Tags)
	labels, d := promptStrings(ctx, plan.Labels)
	diags.Append(d...)
	if diags.HasError() {
		return current, diags
	}
	name, version := current.Name, int(current.Version)
	if !sameStrings(tags, current.Tags) {
		reader, err := jsonBody(map[string]any{"tags": tags})
		if err == nil {
			// The response describes the latest version, not necessarily the tracked one.
			err = apiCall(client.SkillsUpdateWithBody(ctx, name, jsonContentType, reader)).decode(nil)
		}
		if err != nil {
			diags.AddError("Error updating skill tags", err.Error())
			return current, diags
		}
		current.Tags = tags
	}
	if !sameStrings(labels, skillLabels(current.Labels)) {
		reader, err := jsonBody(map[string]any{"labels": labels})
		if err == nil {
			err = apiCall(client.SkillsSetLabelsWithBody(ctx, name, version, jsonContentType, reader)).decode(nil)
		}
		if err != nil {
			diags.AddError("Error updating skill labels", err.Error())
			return current, diags
		}
		current.Labels = labels
	}
	return current, diags
}

// createVersion uploads the planned files as a new version and applies tags
// and labels to it.
func (r *skillResource) createVersion(ctx context.Context, plan skillResourceModel) (skillResourceModel, diag.Diagnostics) {
	contents := map[string]string{}
	diags := plan.Files.ElementsAs(ctx, &contents, false)
	if diags.HasError() {
		return plan, diags
	}
	files := make([]map[string]string, 0, len(contents))
	for _, filePath := range slices.Sorted(maps.Keys(contents)) {
		files = append(files, map[string]string{"path": filePath, "content": contents[filePath]})
	}
	body := map[string]any{"files": files}
	if !plan.CommitMessage.IsNull() {
		body["commitMessage"] = plan.CommitMessage.ValueString()
	}
	reader, err := jsonBody(body)
	if err != nil {
		diags.AddError("Error building skill request", err.Error())
		return plan, diags
	}
	client := r.client(plan.ProjectPublicKey, plan.ProjectSecretKey)
	var skill skillAPI
	if err := apiCall(client.SkillsCreateVersionWithBody(ctx, jsonContentType, reader)).decode(&skill); err != nil {
		diags.AddError("Error creating skill version", err.Error())
		return plan, diags
	}
	if skill.Name != plan.Name.ValueString() {
		// The frontmatter named another skill; undo the stray version.
		detail := "name is \"" + plan.Name.ValueString() + "\" but the SKILL.md frontmatter created a version of \"" + skill.Name + "\"."
		if err := apiCall(client.SkillsDeleteVersion(ctx, skill.Name, int(skill.Version))).decode(nil); err != nil {
			detail += " Removing version " + types.Int64Value(skill.Version).String() + " of that skill failed: " + err.Error()
		}
		diags.AddAttributeError(path.Root("name"), "Skill name mismatch", detail)
		return plan, diags
	}
	skill, d := r.apply(ctx, plan, skill)
	diags.Append(d...)
	state, d := skillState(ctx, skill, plan, plan.Files)
	diags.Append(d...)
	return state, diags
}

func (r *skillResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan skillResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	state, diags := r.createVersion(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if state.Version.IsUnknown() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *skillResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state skillResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	includeContent := true
	params := &publicapi.SkillsGetParams{IncludeContent: &includeContent}
	if state.Version.IsNull() {
		// Import does not know a version yet and adopts the newest one.
		label := promptLatestLabel
		params.Label = &label
	} else {
		version := int(state.Version.ValueInt64())
		params.Version = &version
	}
	var skill skillAPI
	err := apiCall(r.client(state.ProjectPublicKey, state.ProjectSecretKey).SkillsGet(ctx, state.ID.ValueString(), params)).decode(&skill)
	if errors.Is(err, errNotFound) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Error reading skill", err.Error())
		return
	}
	contents := map[string]string{}
	for _, file := range skill.Files {
		if file.Content != nil {
			contents[file.Path] = *file.Content
		} else {
			contents[file.Path] = ""
		}
	}
	files, diags := types.MapValueFrom(ctx, types.StringType, contents)
	resp.Diagnostics.Append(diags...)
	newState, diags := skillState(ctx, skill, state, files)
	resp.Diagnostics.Append(diags...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *skillResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, prior skillResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if skillNeedsVersion(plan, prior) {
		state, diags := r.createVersion(ctx, plan)
		resp.Diagnostics.Append(diags...)
		if state.Version.IsUnknown() {
			return
		}
		resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
		return
	}

	tags, diags := promptStrings(ctx, prior.Tags)
	resp.Diagnostics.Append(diags...)
	labels, diags := promptStrings(ctx, prior.Labels)
	resp.Diagnostics.Append(diags...)
	skill, diags := r.apply(ctx, plan, skillAPI{
		Name:          prior.Name.ValueString(),
		Description:   prior.Description.ValueString(),
		Version:       prior.Version.ValueInt64(),
		Tags:          tags,
		Labels:        labels,
		CommitMessage: prior.CommitMessage.ValueStringPointer(),
	})
	resp.Diagnostics.Append(diags...)
	state, diags := skillState(ctx, skill, plan, plan.Files)
	resp.Diagnostics.Append(diags...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *skillResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state skillResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	client := r.client(state.ProjectPublicKey, state.ProjectSecretKey)
	name := state.ID.ValueString()
	var list struct {
		Data []struct {
			Name          string `json:"name"`
			LatestVersion int    `json:"latestVersion"`
		} `json:"data"`
	}
	if err := apiCall(client.SkillsList(ctx, &publicapi.SkillsListParams{Name: &name})).decode(&list); err != nil {
		resp.Diagnostics.AddError("Error listing skill versions", err.Error())
		return
	}
	for _, skill := range list.Data {
		if skill.Name != name {
			continue
		}
		for version := skill.LatestVersion; version >= 1; version-- {
			err := apiCall(client.SkillsDeleteVersion(ctx, name, version)).decode(nil)
			if err != nil && !errors.Is(err, errNotFound) {
				resp.Diagnostics.AddError("Error deleting skill version", err.Error())
				return
			}
		}
	}
}

func (r *skillResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	publicKey, secretKey, name, err := parseCredentialImportID(req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), name)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project_public_key"), publicKey)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project_secret_key"), secretKey)...)
}
