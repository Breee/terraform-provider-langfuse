package langfuse

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

type adapterCall func(context.Context, AdminClient, OrganizationClient, LlmConnectionsClient) error

func adapterServer(t *testing.T, handler http.HandlerFunc) (AdminClient, OrganizationClient, LlmConnectionsClient) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	host := server.URL + "/langfuse"
	return NewAdminClient(host, "admin-token"), NewOrganizationClient(host, "public", "private"), NewLlmConnectionsClient(host, "public", "private")
}

func checkJSON(t *testing.T, r *http.Request, want string) {
	t.Helper()
	var got, expected any
	if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
		t.Errorf("decode request: %v", err)
		return
	}
	if err := json.Unmarshal([]byte(want), &expected); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, expected) {
		t.Errorf("body = %#v; want %#v", got, expected)
	}
}

func TestHTTPAdapterContracts(t *testing.T) {
	const org = `{"id":"org","name":"Example","metadata":{"team":"platform"}}`
	const project = `{"id":"project","name":"Example","metadata":{"team":"platform"},"retentionDays":null}`
	const key = `{"id":"key","publicKey":"public","secretKey":"secret","createdAt":"2026-01-01T00:00:00Z","displaySecretKey":"***"}`
	const member = `{"userId":"user","email":"user@example.com","role":"ADMIN","name":"User"}`
	const connection = `{"id":"connection","provider":"openai","adapter":"openai","displaySecretKey":"***","baseURL":"https://example.com","config":{"region":"eu"},"customModels":["model"],"withDefaultModels":false,"extraHeaderKeys":["x-test"],"createdAt":"2026-01-01T01:00:00.123+01:00","updatedAt":"2026-01-01T00:00:00Z"}`
	for _, tc := range []struct {
		name, method, path, body, response string
		status                             int
		call                               adapterCall
	}{
		{"create organization", "POST", "/api/admin/organizations", `{"name":"Example"}`, org, 201,
			func(ctx context.Context, a AdminClient, _ OrganizationClient, _ LlmConnectionsClient) error {
				got, err := a.CreateOrganization(ctx, &CreateOrganizationRequest{Name: "Example", Metadata: map[string]string{}})
				if err == nil && (got.ID != "org" || got.Metadata["team"] != "platform") {
					t.Errorf("organization = %#v", got)
				}
				return err
			}},
		{"update organization", "PUT", "/api/admin/organizations/org", `{"name":"Example","metadata":{"team":"platform"}}`, org, 200,
			func(ctx context.Context, a AdminClient, _ OrganizationClient, _ LlmConnectionsClient) error {
				_, err := a.UpdateOrganization(ctx, "org", &UpdateOrganizationRequest{Name: "Example", Metadata: map[string]string{"team": "platform"}})
				return err
			}},
		{"create organization key", "POST", "/api/admin/organizations/org/apiKeys", `{}`, key, 201,
			func(ctx context.Context, a AdminClient, _ OrganizationClient, _ LlmConnectionsClient) error {
				got, err := a.CreateOrganizationApiKey(ctx, "org")
				if err == nil && (got.SecretKey != "secret" || got.ID != "key") {
					t.Errorf("key = %#v", got)
				}
				return err
			}},
		{"create project", "POST", "/api/public/projects", `{"name":"Example","retention":0}`, project, 201,
			func(ctx context.Context, _ AdminClient, o OrganizationClient, _ LlmConnectionsClient) error {
				got, err := o.CreateProject(ctx, &CreateProjectRequest{Name: "Example"})
				if err == nil && (got.RetentionDays != 0 || got.Metadata["team"] != "platform") {
					t.Errorf("project = %#v", got)
				}
				return err
			}},
		{"update project zero retention", "PUT", "/api/public/projects/project", `{"name":"Example","retention":0}`, project, 200,
			func(ctx context.Context, _ AdminClient, o OrganizationClient, _ LlmConnectionsClient) error {
				_, err := o.UpdateProject(ctx, "project", &UpdateProjectRequest{Name: "Example", Metadata: map[string]string{}})
				return err
			}},
		{"delete project", "DELETE", "/api/public/projects/project", "", `{"success":true,"message":"queued"}`, 202,
			func(ctx context.Context, _ AdminClient, o OrganizationClient, _ LlmConnectionsClient) error {
				return o.DeleteProject(ctx, "project")
			}},
		{"create project key", "POST", "/api/public/projects/project/apiKeys", `{}`, key, 201,
			func(ctx context.Context, _ AdminClient, o OrganizationClient, _ LlmConnectionsClient) error {
				got, err := o.CreateProjectApiKey(ctx, "project", nil)
				if err == nil && got.SecretKey != "secret" {
					t.Errorf("key = %#v", got)
				}
				return err
			}},
		{"create SCIM user", "POST", "/api/public/scim/Users", `{"userName":"user@example.com","active":true,"name":{},"emails":[{"value":"user@example.com","primary":true,"type":""}]}`, `{"id":"user","userName":"user@example.com","emails":[{"value":"user@example.com","primary":true,"type":""}],"active":true}`, 201,
			func(ctx context.Context, _ AdminClient, o OrganizationClient, _ LlmConnectionsClient) error {
				req := &SCIMUserRequest{UserName: "user@example.com"}
				req.Emails = append(req.Emails, struct {
					Value   string `json:"value"`
					Primary bool   `json:"primary"`
				}{Value: req.UserName, Primary: true})
				got, err := o.CreateSCIMUser(ctx, req)
				if err == nil && (got.ID != "user" || got.UserName != req.UserName || len(got.Emails) != 1) {
					t.Errorf("SCIM user = %#v", got)
				}
				return err
			}},
		{"delete organization membership", "DELETE", "/api/public/organizations/memberships", `{"userId":"user"}`, `{"success":false,"message":"removed"}`, 200,
			func(ctx context.Context, _ AdminClient, o OrganizationClient, _ LlmConnectionsClient) error {
				return o.RemoveMember(ctx, "user")
			}},
		{"upsert project membership", "PUT", "/api/public/projects/project/memberships", `{"userId":"user","role":"ADMIN"}`, member, 200,
			func(ctx context.Context, _ AdminClient, o OrganizationClient, _ LlmConnectionsClient) error {
				got, err := o.CreateOrUpdateProjectMembership(ctx, "project", &CreateProjectMembershipRequest{UserID: "user", Role: "ADMIN"})
				if err == nil && got.Name != "User" {
					t.Errorf("member = %#v", got)
				}
				return err
			}},
		{"delete project membership", "DELETE", "/api/public/projects/project/memberships", `{"userId":"user"}`, `{"success":false,"message":"removed"}`, 200,
			func(ctx context.Context, _ AdminClient, o OrganizationClient, _ LlmConnectionsClient) error {
				return o.DeleteProjectMembership(ctx, "project", "user")
			}},
		{"create connection", "PUT", "/api/public/llm-connections", `{"provider":"openai","adapter":"openai","secretKey":"secret"}`, connection, 201,
			func(ctx context.Context, _ AdminClient, _ OrganizationClient, l LlmConnectionsClient) error {
				got, err := l.UpsertLlmConnection(ctx, &UpsertLlmConnectionRequest{Provider: "openai", Adapter: "openai", SecretKey: "secret", Config: map[string]any{}, CustomModels: []string{}, ExtraHeaders: map[string]string{}})
				if err == nil && (got.BaseURL != "https://example.com" || got.Config["region"] != "eu" || got.CreatedAt != "2026-01-01T01:00:00.123+01:00") {
					t.Errorf("connection = %#v", got)
				}
				return err
			}},
		{"update connection", "PUT", "/api/public/llm-connections", `{"provider":"openai","adapter":"openai","secretKey":"secret","baseURL":"https://example.com","config":{"region":"eu"},"customModels":["model"],"extraHeaders":{"x-test":"value"},"withDefaultModels":false}`, connection, 200,
			func(ctx context.Context, _ AdminClient, _ OrganizationClient, l LlmConnectionsClient) error {
				useDefaults := false
				_, err := l.UpsertLlmConnection(ctx, &UpsertLlmConnectionRequest{Provider: "openai", Adapter: "openai", SecretKey: "secret", BaseURL: "https://example.com", Config: map[string]any{"region": "eu"}, CustomModels: []string{"model"}, ExtraHeaders: map[string]string{"x-test": "value"}, WithDefaultModels: &useDefaults})
				return err
			}},
		{"delete missing connection", "DELETE", "/api/public/llm-connections/connection", "", `{"message":"not found"}`, 404,
			func(ctx context.Context, _ AdminClient, _ OrganizationClient, l LlmConnectionsClient) error {
				return l.DeleteLlmConnection(ctx, "connection")
			}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			a, o, l := adapterServer(t, func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != tc.method || r.URL.Path != "/langfuse"+tc.path {
					t.Errorf("request = %s %s", r.Method, r.URL.Path)
				}
				if strings.HasPrefix(tc.path, "/api/admin/") {
					if r.Header.Get("Authorization") != "Bearer admin-token" {
						t.Error("missing admin bearer auth")
					}
				} else if user, password, ok := r.BasicAuth(); !ok || user != "public" || password != "private" {
					t.Error("missing public basic auth")
				}
				if tc.body != "" {
					checkJSON(t, r, tc.body)
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.response))
			})
			if err := tc.call(context.Background(), a, o, l); err != nil {
				t.Fatal(err)
			}
			if calls != 1 {
				t.Errorf("requests = %d; want 1", calls)
			}
		})
	}
}

func TestHTTPAdapterErrors(t *testing.T) {
	for _, tc := range []struct {
		name string
		call adapterCall
	}{
		{"admin", func(ctx context.Context, a AdminClient, _ OrganizationClient, _ LlmConnectionsClient) error {
			_, err := a.ListOrganizations(ctx)
			return err
		}},
		{"public", func(ctx context.Context, _ AdminClient, o OrganizationClient, _ LlmConnectionsClient) error {
			_, err := o.ListProjects(ctx)
			return err
		}},
		{"connections", func(ctx context.Context, _ AdminClient, _ OrganizationClient, l LlmConnectionsClient) error {
			_, err := l.ListLlmConnections(ctx, nil, nil)
			return err
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, status := range []int{400, 401, 403, 404, 500} {
				a, o, l := adapterServer(t, func(w http.ResponseWriter, _ *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(status)
					_, _ = w.Write([]byte(`{"message":"denied"}`))
				})
				err := tc.call(context.Background(), a, o, l)
				want := fmt.Sprintf(`request failed with status code %d, response body: {"message":"denied"}`, status)
				if err == nil || err.Error() != want {
					t.Errorf("status %d: error = %v", status, err)
				}
			}
			a, o, l := adapterServer(t, func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{invalid`))
			})
			if err := tc.call(context.Background(), a, o, l); err == nil || !strings.Contains(err.Error(), "request:") {
				t.Errorf("parse error = %v", err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			if err := tc.call(ctx, a, o, l); !errors.Is(err, context.Canceled) {
				t.Errorf("transport error = %v", err)
			}
		})
	}
}

func TestHTTPCreateStatusCodes(t *testing.T) {
	for _, tc := range []struct {
		name, response string
		call           adapterCall
	}{
		{"organization", `{"id":"org","name":"Example"}`, func(ctx context.Context, a AdminClient, _ OrganizationClient, _ LlmConnectionsClient) error {
			_, err := a.CreateOrganization(ctx, &CreateOrganizationRequest{Name: "Example"})
			return err
		}},
		{"organization key", `{"id":"key","publicKey":"public","secretKey":"secret"}`, func(ctx context.Context, a AdminClient, _ OrganizationClient, _ LlmConnectionsClient) error {
			_, err := a.CreateOrganizationApiKey(ctx, "org")
			return err
		}},
		{"project", `{"id":"project","name":"Example"}`, func(ctx context.Context, _ AdminClient, o OrganizationClient, _ LlmConnectionsClient) error {
			_, err := o.CreateProject(ctx, &CreateProjectRequest{Name: "Example"})
			return err
		}},
		{"project key", `{"id":"key","publicKey":"public","secretKey":"secret"}`, func(ctx context.Context, _ AdminClient, o OrganizationClient, _ LlmConnectionsClient) error {
			_, err := o.CreateProjectApiKey(ctx, "project", nil)
			return err
		}},
		{"SCIM user", `{"id":"user","userName":"user@example.com"}`, func(ctx context.Context, _ AdminClient, o OrganizationClient, _ LlmConnectionsClient) error {
			_, err := o.CreateSCIMUser(ctx, &SCIMUserRequest{UserName: "user@example.com"})
			return err
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, o, l := adapterServer(t, func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tc.response))
			})
			err := tc.call(context.Background(), a, o, l)
			if err == nil || !strings.Contains(err.Error(), "status code 200") {
				t.Fatalf("unexpected create status accepted: %v", err)
			}
		})
	}
}

func TestHTTPConnectionPagination(t *testing.T) {
	_, _, l := adapterServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/langfuse/api/public/llm-connections" || r.URL.Query().Get("page") != "2" || r.URL.Query().Get("limit") != "10" {
			t.Errorf("pagination URL = %s", r.URL)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"connection","provider":"openai","adapter":"openai","displaySecretKey":"***","customModels":[],"withDefaultModels":true,"extraHeaderKeys":[],"createdAt":"2026-01-01T00:00:00Z","updatedAt":"2026-01-01T00:00:00Z"}],"meta":{"page":2,"limit":10,"totalItems":11,"totalPages":2}}`))
	})
	page, limit := 2, 10
	got, err := l.ListLlmConnections(context.Background(), &page, &limit)
	if err != nil {
		t.Fatal(err)
	}
	if got.Meta != (PaginationMeta{Page: 2, Limit: 10, TotalItems: 11, TotalPages: 2}) || len(got.Data) != 1 || got.Data[0].ID != "connection" || !got.Data[0].WithDefaultModels {
		t.Errorf("list response = %#v", got)
	}
}

func TestHTTPMembershipUpdate(t *testing.T) {
	calls := 0
	_, o, _ := adapterServer(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case "GET":
			_, _ = w.Write([]byte(`{"memberships":[{"userId":"user","email":"user@example.com","role":"MEMBER","name":"User"}]}`))
		case "PUT":
			checkJSON(t, r, `{"userId":"user","role":"ADMIN"}`)
			_, _ = w.Write([]byte(`{"userId":"user","email":"user@example.com","role":"ADMIN","name":"User"}`))
		default:
			t.Errorf("unexpected method %s", r.Method)
		}
	})
	got, err := o.UpdateMembership(context.Background(), "user", &UpdateMembershipRequest{Role: "ADMIN"})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 || got.ID != "user" || got.UserID != "user" || got.Role != "ADMIN" || got.Name != "User" {
		t.Errorf("membership = %#v; calls = %d", got, calls)
	}
}

func TestHTTPMetadataMapping(t *testing.T) {
	for _, value := range []string{`"platform"`, `42`, `null`, `{"nested":true}`} {
		a, o, _ := adapterServer(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if strings.Contains(r.URL.Path, "/api/admin/") {
				_, _ = w.Write([]byte(`{"id":"org","name":"Example","metadata":{"team":` + value + `}}`))
			} else {
				_, _ = w.Write([]byte(`{"projects":[{"id":"project","name":"Example","metadata":{"team":` + value + `}}]}`))
			}
		})
		_, adminErr := a.GetOrganization(context.Background(), "org")
		_, publicErr := o.GetProject(context.Background(), "project")
		for _, err := range []error{adminErr, publicErr} {
			if value == `"platform"` && err != nil {
				t.Fatal(err)
			}
			if value != `"platform"` && (err == nil || !strings.Contains(err.Error(), "expected string")) {
				t.Errorf("metadata %s: error = %v", value, err)
			}
		}
	}
}
