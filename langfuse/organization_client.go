package langfuse

import (
	"context"
	"errors"
	"fmt"

	"github.com/langfuse/terraform-provider-langfuse/langfuse/publicapi"
)

var (
	ErrMembershipNotFound        = errors.New("membership not found")
	ErrProjectMembershipNotFound = errors.New("project membership not found")
)

type Project struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	RetentionDays int32             `json:"retentionDays"`
	Metadata      map[string]string `json:"metadata"`
}

type ProjectApiKey struct {
	ID        string  `json:"id"`
	PublicKey string  `json:"publicKey"`
	SecretKey string  `json:"secretKey"`
	Note      *string `json:"note"`
}

// CreateProjectApiKeyRequest is the JSON body for POST /api/public/projects/{projectId}/apiKeys.
type CreateProjectApiKeyRequest struct {
	Note *string `json:"note,omitempty"`
}

type CreateProjectRequest struct {
	Name          string            `json:"name"`
	RetentionDays int32             `json:"retention"`
	Metadata      map[string]string `json:"metadata,omitempty"`
}

type UpdateProjectRequest struct {
	Name          string            `json:"name"`
	RetentionDays int32             `json:"retention"`
	Metadata      map[string]string `json:"metadata,omitempty"`
}

type OrganizationMembership struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	Role     string `json:"role"`
	Status   string `json:"status"`
	UserID   string `json:"userId"`
	Username string `json:"username"`
}

type SCIMUserRequest struct {
	UserName string `json:"userName"`
	Emails   []struct {
		Value   string `json:"value"`
		Primary bool   `json:"primary"`
	} `json:"emails"`
	Password string `json:"password,omitempty"`
	Active   bool   `json:"active"`
}

type SCIMUserResponse struct {
	ID       string `json:"id"`
	UserName string `json:"userName"`
	Emails   []struct {
		Value   string `json:"value"`
		Primary bool   `json:"primary"`
	} `json:"emails"`
	Active bool `json:"active"`
}

type UpdateMembershipRequest struct {
	UserID string `json:"userId,omitempty"` // User ID from SCIM
	Email  string `json:"email,omitempty"`  // Or email
	Role   string `json:"role"`
}

// Project Membership types
type ProjectMembership struct {
	UserID string `json:"userId"`
	Role   string `json:"role"`
	Email  string `json:"email"`
	Name   string `json:"name"`
}

type CreateProjectMembershipRequest struct {
	UserID string `json:"userId"`
	Role   string `json:"role"`
}

//go:generate go run github.com/golang/mock/mockgen -destination=./mocks/mock_organization_client.go -package=mocks github.com/langfuse/terraform-provider-langfuse/langfuse OrganizationClient

type OrganizationClient interface {
	ListProjects(ctx context.Context) ([]*Project, error)
	GetProject(ctx context.Context, projectID string) (*Project, error)
	CreateProject(ctx context.Context, request *CreateProjectRequest) (*Project, error)
	UpdateProject(ctx context.Context, projectID string, request *UpdateProjectRequest) (*Project, error)
	DeleteProject(ctx context.Context, projectID string) error
	GetProjectApiKey(ctx context.Context, projectID string, apiKeyID string) (*ProjectApiKey, error)
	CreateProjectApiKey(ctx context.Context, projectID string, request *CreateProjectApiKeyRequest) (*ProjectApiKey, error)
	DeleteProjectApiKey(ctx context.Context, projectID string, apiKeyID string) error
	ListMemberships(ctx context.Context) ([]OrganizationMembership, error)
	GetMembership(ctx context.Context, membershipID string) (*OrganizationMembership, error)
	UpdateMembership(ctx context.Context, membershipID string, request *UpdateMembershipRequest) (*OrganizationMembership, error)
	RemoveMember(ctx context.Context, membershipID string) error
	CreateSCIMUser(ctx context.Context, request *SCIMUserRequest) (*SCIMUserResponse, error)
	// Project membership methods
	ListProjectMemberships(ctx context.Context, projectID string) ([]ProjectMembership, error)
	GetProjectMembership(ctx context.Context, projectID, membershipID string) (*ProjectMembership, error)
	CreateOrUpdateProjectMembership(ctx context.Context, projectID string, request *CreateProjectMembershipRequest) (*ProjectMembership, error)
	DeleteProjectMembership(ctx context.Context, projectID, userID string) error
}

type organizationClientImpl struct {
	api *publicapi.ClientWithResponses
}

func NewOrganizationClient(host, publicKey, privateKey string) OrganizationClient {
	return &organizationClientImpl{api: newPublicAPIClient(host, publicKey, privateKey)}
}

func toProject(id, name string, metadata map[string]any, retentionDays *int) (*Project, error) {
	md, err := stringMap(metadata)
	if err != nil {
		return nil, err
	}
	proj := &Project{ID: id, Name: name, Metadata: md}
	if retentionDays != nil {
		proj.RetentionDays = int32(*retentionDays)
	}
	return proj, nil
}

func (c *organizationClientImpl) ListProjects(ctx context.Context) ([]*Project, error) {
	resp, err := c.api.OrganizationsGetOrganizationProjectsWithResponse(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	list, err := payload(resp, resp.JSON200)
	if err != nil {
		return nil, err
	}

	projects := make([]*Project, 0, len(list.Projects))
	for _, p := range list.Projects {
		var metadata map[string]any
		if p.Metadata != nil {
			metadata = *p.Metadata
		}
		// This endpoint does not return retentionDays.
		proj, err := toProject(p.ID, p.Name, metadata, nil)
		if err != nil {
			return nil, err
		}
		projects = append(projects, proj)
	}
	return projects, nil
}

func (c *organizationClientImpl) GetProject(ctx context.Context, projectID string) (*Project, error) {
	// Note: this endpoint does not return `retentionDays`, so the returned value will always be 0
	projects, err := c.ListProjects(ctx)
	if err != nil {
		return nil, err
	}
	for _, proj := range projects {
		if proj.ID == projectID {
			return proj, nil
		}
	}
	return nil, fmt.Errorf("cannot find project with ID %s", projectID)
}

func (c *organizationClientImpl) CreateProject(ctx context.Context, request *CreateProjectRequest) (*Project, error) {
	resp, err := c.api.ProjectsCreateWithResponse(ctx, publicapi.ProjectsCreateJSONRequestBody{
		Name:      request.Name,
		Retention: int(request.RetentionDays),
		Metadata:  anyMapPtr(request.Metadata),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	proj, err := payload(resp, resp.JSON201)
	if err != nil {
		return nil, err
	}
	return toProject(proj.ID, proj.Name, proj.Metadata, proj.RetentionDays)
}

func (c *organizationClientImpl) UpdateProject(ctx context.Context, projectID string, request *UpdateProjectRequest) (*Project, error) {
	retention := int(request.RetentionDays)
	resp, err := c.api.ProjectsUpdateWithResponse(ctx, projectID, publicapi.ProjectsUpdateJSONRequestBody{
		Name:      request.Name,
		Retention: &retention,
		Metadata:  anyMapPtr(request.Metadata),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	proj, err := payload(resp, resp.JSON200)
	if err != nil {
		return nil, err
	}
	return toProject(proj.ID, proj.Name, proj.Metadata, proj.RetentionDays)
}

func (c *organizationClientImpl) DeleteProject(ctx context.Context, projectID string) error {
	resp, err := c.api.ProjectsDeleteWithResponse(ctx, projectID)
	if err != nil {
		return fmt.Errorf("failed to make request: %w", err)
	}
	deleted, err := payload(resp, resp.JSON202)
	if err != nil {
		return err
	}
	if !deleted.Success {
		return fmt.Errorf("failed to delete project with ID %s: %s", projectID, deleted.Message)
	}
	return nil
}

func (c *organizationClientImpl) GetProjectApiKey(ctx context.Context, projectID string, apiKeyID string) (*ProjectApiKey, error) {
	resp, err := c.api.ProjectsGetAPIKeysWithResponse(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	list, err := payload(resp, resp.JSON200)
	if err != nil {
		return nil, err
	}
	for _, key := range list.APIKeys {
		if key.ID == apiKeyID {
			return &ProjectApiKey{ID: key.ID, PublicKey: key.PublicKey, Note: key.Note}, nil
		}
	}

	return nil, fmt.Errorf("cannot find API key with ID %s in project %s", apiKeyID, projectID)
}

func (c *organizationClientImpl) CreateProjectApiKey(ctx context.Context, projectID string, request *CreateProjectApiKeyRequest) (*ProjectApiKey, error) {
	var body publicapi.ProjectsCreateAPIKeyJSONRequestBody
	if request != nil {
		body.Note = request.Note
	}
	resp, err := c.api.ProjectsCreateAPIKeyWithResponse(ctx, projectID, body)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	key, err := payload(resp, resp.JSON201)
	if err != nil {
		return nil, err
	}
	return &ProjectApiKey{ID: key.ID, PublicKey: key.PublicKey, SecretKey: key.SecretKey, Note: key.Note}, nil
}

func (c *organizationClientImpl) DeleteProjectApiKey(ctx context.Context, projectID string, apiKeyID string) error {
	resp, err := c.api.ProjectsDeleteAPIKeyWithResponse(ctx, projectID, apiKeyID)
	if err != nil {
		return fmt.Errorf("failed to make request: %w", err)
	}
	deleted, err := payload(resp, resp.JSON200)
	if err != nil {
		return err
	}
	if !deleted.Success {
		return fmt.Errorf("failed to delete API key with ID %s in project %s", apiKeyID, projectID)
	}
	return nil
}

func (c *organizationClientImpl) ListMemberships(ctx context.Context) ([]OrganizationMembership, error) {
	resp, err := c.api.OrganizationsGetOrganizationMembershipsWithResponse(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	list, err := payload(resp, resp.JSON200)
	if err != nil {
		return nil, err
	}

	memberships := make([]OrganizationMembership, 0, len(list.Memberships))
	for _, m := range list.Memberships {
		memberships = append(memberships, OrganizationMembership{UserID: m.UserID, Email: m.Email, Role: string(m.Role), Name: m.Name})
	}
	return memberships, nil
}

func (c *organizationClientImpl) GetMembership(ctx context.Context, membershipID string) (*OrganizationMembership, error) {
	memberships, err := c.ListMemberships(ctx)
	if err != nil {
		return nil, err
	}

	for _, membership := range memberships {
		// The API may not return the membership ID field, so check both ID and UserID
		if membership.ID == membershipID || membership.UserID == membershipID {
			return &membership, nil
		}
	}

	return nil, fmt.Errorf("%w: ID %s", ErrMembershipNotFound, membershipID)
}

func (c *organizationClientImpl) UpdateMembership(ctx context.Context, membershipID string, request *UpdateMembershipRequest) (*OrganizationMembership, error) {
	// Retrieve current membership to get the user ID
	currentMembership, err := c.GetMembership(ctx, membershipID)
	if err != nil {
		return nil, fmt.Errorf("failed to get current membership: %w", err)
	}

	// Prepare update request with user ID
	userIDToUpdate := request.UserID
	if userIDToUpdate == "" {
		userIDToUpdate = currentMembership.UserID
	}

	resp, err := c.api.OrganizationsUpdateOrganizationMembershipWithResponse(ctx, publicapi.MembershipRequest{
		UserID: userIDToUpdate,
		Role:   publicapi.MembershipRole(request.Role),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to update membership: %w", err)
	}
	updated, err := payload(resp, resp.JSON200)
	if err != nil {
		return nil, err
	}

	// The PUT response has no membership ID, so preserve it from the original request
	return &OrganizationMembership{
		ID:     membershipID,
		UserID: updated.UserID,
		Email:  updated.Email,
		Role:   string(updated.Role),
		Name:   updated.Name,
	}, nil
}

func (c *organizationClientImpl) RemoveMember(ctx context.Context, membershipID string) error {
	resp, err := c.api.OrganizationsDeleteOrganizationMembershipWithResponse(ctx, publicapi.DeleteMembershipRequest{UserID: membershipID})
	if err != nil {
		return fmt.Errorf("failed to make request: %w", err)
	}
	_, err = payload(resp, resp.JSON200)
	return err
}

func (c *organizationClientImpl) CreateSCIMUser(ctx context.Context, request *SCIMUserRequest) (*SCIMUserResponse, error) {
	active := true
	body := publicapi.ScimCreateUserJSONRequestBody{UserName: request.UserName, Active: &active}
	if request.Password != "" {
		body.Password = &request.Password
	}
	if len(request.Emails) > 0 {
		emails := make([]publicapi.ScimEmail, 0, len(request.Emails))
		for _, e := range request.Emails {
			emails = append(emails, publicapi.ScimEmail{Value: e.Value, Primary: e.Primary})
		}
		body.Emails = &emails
	}

	resp, err := c.api.ScimCreateUserWithResponse(ctx, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create SCIM user: %w", err)
	}
	user, err := payload(resp, resp.JSON201)
	if err != nil {
		return nil, err
	}

	scimUser := &SCIMUserResponse{ID: user.ID, UserName: user.UserName}
	for _, e := range user.Emails {
		scimUser.Emails = append(scimUser.Emails, struct {
			Value   string `json:"value"`
			Primary bool   `json:"primary"`
		}{Value: e.Value, Primary: e.Primary})
	}
	return scimUser, nil
}

// Project membership methods

func toProjectMembership(m *publicapi.MembershipResponse) ProjectMembership {
	return ProjectMembership{UserID: m.UserID, Role: string(m.Role), Email: m.Email, Name: m.Name}
}

func (c *organizationClientImpl) ListProjectMemberships(ctx context.Context, projectID string) ([]ProjectMembership, error) {
	resp, err := c.api.OrganizationsGetProjectMembershipsWithResponse(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	list, err := payload(resp, resp.JSON200)
	if err != nil {
		return nil, err
	}

	memberships := make([]ProjectMembership, 0, len(list.Memberships))
	for i := range list.Memberships {
		memberships = append(memberships, toProjectMembership(&list.Memberships[i]))
	}
	return memberships, nil
}

func (c *organizationClientImpl) GetProjectMembership(ctx context.Context, projectID, membershipID string) (*ProjectMembership, error) {
	memberships, err := c.ListProjectMemberships(ctx, projectID)
	if err != nil {
		return nil, err
	}

	for _, membership := range memberships {
		if membership.UserID == membershipID {
			return &membership, nil
		}
	}

	return nil, fmt.Errorf("%w: user %s in project %s", ErrProjectMembershipNotFound, membershipID, projectID)
}

func (c *organizationClientImpl) CreateOrUpdateProjectMembership(ctx context.Context, projectID string, request *CreateProjectMembershipRequest) (*ProjectMembership, error) {
	resp, err := c.api.OrganizationsUpdateProjectMembershipWithResponse(ctx, projectID, publicapi.MembershipRequest{
		UserID: request.UserID,
		Role:   publicapi.MembershipRole(request.Role),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create/update project membership: %w", err)
	}
	updated, err := payload(resp, resp.JSON200)
	if err != nil {
		return nil, err
	}
	membership := toProjectMembership(updated)
	return &membership, nil
}

func (c *organizationClientImpl) DeleteProjectMembership(ctx context.Context, projectID, userID string) error {
	resp, err := c.api.OrganizationsDeleteProjectMembershipWithResponse(ctx, projectID, publicapi.DeleteMembershipRequest{UserID: userID})
	if err != nil {
		return fmt.Errorf("failed to make request: %w", err)
	}
	_, err = payload(resp, resp.JSON200)
	return err
}
