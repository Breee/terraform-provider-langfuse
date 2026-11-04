package langfuse

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/langfuse/terraform-provider-langfuse/langfuse/publicapi"
)

//go:generate mockgen -destination=./mocks/mock_llm_connections_client.go -package=mocks github.com/langfuse/terraform-provider-langfuse/langfuse LlmConnectionsClient

type LlmConnection struct {
	ID                string         `json:"id"`
	Provider          string         `json:"provider"`
	Adapter           string         `json:"adapter"`
	DisplaySecretKey  string         `json:"displaySecretKey"`
	BaseURL           string         `json:"baseURL,omitempty"`
	CustomModels      []string       `json:"customModels,omitempty"`
	WithDefaultModels bool           `json:"withDefaultModels"`
	ExtraHeaderKeys   []string       `json:"extraHeaderKeys,omitempty"`
	Config            map[string]any `json:"config,omitempty"`
	CreatedAt         string         `json:"createdAt"`
	UpdatedAt         string         `json:"updatedAt"`
}

type UpsertLlmConnectionRequest struct {
	Adapter           string            `json:"adapter"`
	Provider          string            `json:"provider"`
	SecretKey         string            `json:"secretKey"`
	BaseURL           string            `json:"baseURL,omitempty"`
	Config            map[string]any    `json:"config,omitempty"`
	CustomModels      []string          `json:"customModels,omitempty"`
	ExtraHeaders      map[string]string `json:"extraHeaders,omitempty"`
	WithDefaultModels *bool             `json:"withDefaultModels,omitempty"`
}

type ListLlmConnectionsResponse struct {
	Data []LlmConnection `json:"data"`
	Meta PaginationMeta  `json:"meta"`
}

type PaginationMeta struct {
	Page       int `json:"page"`
	Limit      int `json:"limit"`
	TotalItems int `json:"totalItems"`
	TotalPages int `json:"totalPages"`
}

type LlmConnectionsClient interface {
	ListLlmConnections(ctx context.Context, page, limit *int) (*ListLlmConnectionsResponse, error)
	UpsertLlmConnection(ctx context.Context, req *UpsertLlmConnectionRequest) (*LlmConnection, error)
	DeleteLlmConnection(ctx context.Context, id string) error
}

type llmConnectionsClientImpl struct {
	api *publicapi.ClientWithResponses
}

func NewLlmConnectionsClient(host, publicKey, privateKey string) LlmConnectionsClient {
	return &llmConnectionsClientImpl{api: newPublicAPIClient(host, publicKey, privateKey)}
}

func toLlmConnection(c *publicapi.LlmConnection) *LlmConnection {
	conn := &LlmConnection{
		ID:                c.ID,
		Provider:          c.Provider,
		Adapter:           c.Adapter,
		DisplaySecretKey:  c.DisplaySecretKey,
		CustomModels:      c.CustomModels,
		WithDefaultModels: c.WithDefaultModels,
		ExtraHeaderKeys:   c.ExtraHeaderKeys,
		CreatedAt:         c.CreatedAt.Format(time.RFC3339Nano),
		UpdatedAt:         c.UpdatedAt.Format(time.RFC3339Nano),
	}
	if c.BaseURL != nil {
		conn.BaseURL = *c.BaseURL
	}
	if c.Config != nil {
		conn.Config = *c.Config
	}
	return conn
}

func (c *llmConnectionsClientImpl) ListLlmConnections(ctx context.Context, page, limit *int) (*ListLlmConnectionsResponse, error) {
	resp, err := c.api.LlmConnectionsListWithResponse(ctx, &publicapi.LlmConnectionsListParams{Page: page, Limit: limit})
	if err != nil {
		return nil, fmt.Errorf("failed to make list llm connections request: %w", err)
	}
	list, err := payload(resp, resp.JSON200)
	if err != nil {
		return nil, err
	}

	out := &ListLlmConnectionsResponse{
		Data: make([]LlmConnection, 0, len(list.Data)),
		Meta: PaginationMeta{
			Page:       list.Meta.Page,
			Limit:      list.Meta.Limit,
			TotalItems: list.Meta.TotalItems,
			TotalPages: list.Meta.TotalPages,
		},
	}
	for i := range list.Data {
		out.Data = append(out.Data, *toLlmConnection(&list.Data[i]))
	}
	return out, nil
}

func (c *llmConnectionsClientImpl) UpsertLlmConnection(ctx context.Context, req *UpsertLlmConnectionRequest) (*LlmConnection, error) {
	body := publicapi.LlmConnectionsUpsertJSONRequestBody{
		Adapter:           publicapi.LlmAdapter(req.Adapter),
		Provider:          req.Provider,
		SecretKey:         req.SecretKey,
		WithDefaultModels: req.WithDefaultModels,
	}
	if req.BaseURL != "" {
		body.BaseURL = &req.BaseURL
	}
	if len(req.Config) > 0 {
		body.Config = &req.Config
	}
	if len(req.CustomModels) > 0 {
		body.CustomModels = &req.CustomModels
	}
	if len(req.ExtraHeaders) > 0 {
		body.ExtraHeaders = &req.ExtraHeaders
	}

	resp, err := c.api.LlmConnectionsUpsertWithResponse(ctx, body)
	if err != nil {
		return nil, fmt.Errorf("failed to make upsert llm connection request: %w", err)
	}
	conn := resp.JSON200
	if conn == nil {
		conn = resp.JSON201
	}
	conn, err = payload(resp, conn)
	if err != nil {
		return nil, err
	}
	return toLlmConnection(conn), nil
}

func (c *llmConnectionsClientImpl) DeleteLlmConnection(ctx context.Context, id string) error {
	resp, err := c.api.LlmConnectionsDeleteWithResponse(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to make request: %w", err)
	}
	if resp.StatusCode() == http.StatusNotFound {
		return nil
	}
	_, err = payload(resp, resp.JSON200)
	return err
}
