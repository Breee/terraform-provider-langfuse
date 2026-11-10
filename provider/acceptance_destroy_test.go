package provider

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestCheckOrganizationsDestroyed(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		body       string
		resourceID string
		wantError  string
	}{
		{"absent", http.StatusOK, `{"organizations":[]}`, "org-1", ""},
		{"still present", http.StatusOK, `{"organizations":[{"id":"org-1","name":"test","metadata":{}}]}`, "org-1", "still exists"},
		{"API error", http.StatusInternalServerError, `{"error":"unavailable"}`, "org-1", "list organizations"},
		{"missing state", http.StatusOK, `{"organizations":[]}`, "", "no organization"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet || r.URL.Path != "/api/admin/organizations" {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()
			t.Setenv("LANGFUSE_HOST", server.URL)
			t.Setenv("LANGFUSE_ADMIN_KEY", "test")

			state := &terraform.State{Modules: []*terraform.ModuleState{{
				Path:      []string{"root"},
				Resources: map[string]*terraform.ResourceState{},
			}}}
			if tt.resourceID != "" {
				state.RootModule().Resources["langfuse_organization.test"] = &terraform.ResourceState{
					Type:    "langfuse_organization",
					Primary: &terraform.InstanceState{ID: tt.resourceID},
				}
			}
			err := testAccCheckOrganizationsDestroyed(state)
			if tt.wantError == "" && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantError != "" && (err == nil || !strings.Contains(err.Error(), tt.wantError)) {
				t.Fatalf("error = %v, want %q", err, tt.wantError)
			}
		})
	}
}
