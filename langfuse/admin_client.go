package langfuse

import (
	"context"
	"fmt"

	"github.com/langfuse/terraform-provider-langfuse/langfuse/adminapi"
)

type Organization struct {
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	Metadata map[string]string `json:"metadata"`
}

type OrganizationApiKey struct {
	ID        string `json:"id"`
	PublicKey string `json:"publicKey"`
	SecretKey string `json:"secretKey"`
}

type CreateOrganizationRequest struct {
	Name     string            `json:"name"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

type UpdateOrganizationRequest struct {
	Name     string            `json:"name"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

//go:generate go run github.com/golang/mock/mockgen -destination=./mocks/mock_admin_client.go -package=mocks github.com/langfuse/terraform-provider-langfuse/langfuse AdminClient

type AdminClient interface {
	ListOrganizations(ctx context.Context) ([]*Organization, error)
	GetOrganization(ctx context.Context, orgID string) (*Organization, error)
	CreateOrganization(ctx context.Context, request *CreateOrganizationRequest) (*Organization, error)
	UpdateOrganization(ctx context.Context, orgID string, request *UpdateOrganizationRequest) (*Organization, error)
	DeleteOrganization(ctx context.Context, orgID string) error
	GetOrganizationApiKey(ctx context.Context, orgID string, apiKeyID string) (*OrganizationApiKey, error)
	CreateOrganizationApiKey(ctx context.Context, orgID string) (*OrganizationApiKey, error)
	DeleteOrganizationApiKey(ctx context.Context, orgID string, apiKeyID string) error
}

type adminClientImpl struct {
	api *adminapi.ClientWithResponses
}

func NewAdminClient(host, apiKey string) AdminClient {
	api, err := adminapi.NewClientWithResponses(host, adminapi.WithRequestEditorFn(bearerAuth(apiKey)))
	if err != nil {
		// Unreachable: only client options can fail, and WithRequestEditorFn never does.
		panic(err)
	}
	return &adminClientImpl{api: api}
}

func toOrganization(o *adminapi.Organization) (*Organization, error) {
	metadata, err := stringMap(o.Metadata)
	if err != nil {
		return nil, err
	}
	return &Organization{ID: o.ID, Name: o.Name, Metadata: metadata}, nil
}

func (c *adminClientImpl) ListOrganizations(ctx context.Context) ([]*Organization, error) {
	resp, err := c.api.OrganizationsGetAllWithResponse(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	list, err := payload(resp, resp.JSON200)
	if err != nil {
		return nil, err
	}

	orgs := make([]*Organization, 0, len(list.Organizations))
	for i := range list.Organizations {
		org, err := toOrganization(&list.Organizations[i])
		if err != nil {
			return nil, err
		}
		orgs = append(orgs, org)
	}
	return orgs, nil
}

func (c *adminClientImpl) GetOrganization(ctx context.Context, orgID string) (*Organization, error) {
	resp, err := c.api.OrganizationsGetByIDWithResponse(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	org, err := payload(resp, resp.JSON200)
	if err != nil {
		return nil, err
	}
	return toOrganization(org)
}

func (c *adminClientImpl) CreateOrganization(ctx context.Context, request *CreateOrganizationRequest) (*Organization, error) {
	resp, err := c.api.OrganizationsCreateWithResponse(ctx, adminapi.OrganizationsCreateJSONRequestBody{
		Name:     request.Name,
		Metadata: anyMapPtr(request.Metadata),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	org, err := payload(resp, resp.JSON201)
	if err != nil {
		return nil, err
	}
	return toOrganization(org)
}

func (c *adminClientImpl) UpdateOrganization(ctx context.Context, orgID string, request *UpdateOrganizationRequest) (*Organization, error) {
	resp, err := c.api.OrganizationsUpdateWithResponse(ctx, orgID, adminapi.OrganizationsUpdateJSONRequestBody{
		Name:     request.Name,
		Metadata: anyMapPtr(request.Metadata),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	org, err := payload(resp, resp.JSON200)
	if err != nil {
		return nil, err
	}
	return toOrganization(org)
}

func (c *adminClientImpl) DeleteOrganization(ctx context.Context, orgID string) error {
	resp, err := c.api.OrganizationsDeleteWithResponse(ctx, orgID)
	if err != nil {
		return fmt.Errorf("failed to make request: %w", err)
	}
	deleted, err := payload(resp, resp.JSON200)
	if err != nil {
		return err
	}
	if !deleted.Success {
		return fmt.Errorf("failed to delete organization with ID %s", orgID)
	}
	return nil
}

func (c *adminClientImpl) GetOrganizationApiKey(ctx context.Context, orgID string, apiKeyID string) (*OrganizationApiKey, error) {
	resp, err := c.api.OrganizationsGetAPIKeysWithResponse(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	list, err := payload(resp, resp.JSON200)
	if err != nil {
		return nil, err
	}
	for _, key := range list.APIKeys {
		if key.ID == apiKeyID {
			return &OrganizationApiKey{ID: key.ID, PublicKey: key.PublicKey}, nil
		}
	}

	return nil, fmt.Errorf("cannot find API key with ID %s in organization %s", apiKeyID, orgID)
}

func (c *adminClientImpl) CreateOrganizationApiKey(ctx context.Context, orgID string) (*OrganizationApiKey, error) {
	resp, err := c.api.OrganizationsCreateAPIKeyWithResponse(ctx, orgID, adminapi.OrganizationsCreateAPIKeyJSONRequestBody{})
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	key, err := payload(resp, resp.JSON201)
	if err != nil {
		return nil, err
	}
	return &OrganizationApiKey{ID: key.ID, PublicKey: key.PublicKey, SecretKey: key.SecretKey}, nil
}

func (c *adminClientImpl) DeleteOrganizationApiKey(ctx context.Context, orgID string, apiKeyID string) error {
	resp, err := c.api.OrganizationsDeleteAPIKeyWithResponse(ctx, orgID, apiKeyID)
	if err != nil {
		return fmt.Errorf("failed to make request: %w", err)
	}
	deleted, err := payload(resp, resp.JSON200)
	if err != nil {
		return err
	}
	if !deleted.Success {
		return fmt.Errorf("failed to delete API key with ID %s in organization %s", apiKeyID, orgID)
	}
	return nil
}
