package langfuse

import (
	"context"
	"fmt"
	"net/http"
)

// apiResponse is implemented by every generated *Response type.
type apiResponse interface {
	StatusCode() int
	GetBody() []byte
}

// payload returns the decoded success body, or an error carrying the status and raw body of any other response.
func payload[T any](resp apiResponse, value *T) (*T, error) {
	if value == nil {
		return nil, fmt.Errorf("request failed with status code %d, response body: %s", resp.StatusCode(), string(resp.GetBody()))
	}
	return value, nil
}

func basicAuth(publicKey, privateKey string) func(context.Context, *http.Request) error {
	return func(_ context.Context, req *http.Request) error {
		req.SetBasicAuth(publicKey, privateKey)
		return nil
	}
}

func bearerAuth(token string) func(context.Context, *http.Request) error {
	return func(_ context.Context, req *http.Request) error {
		req.Header.Set("Authorization", "Bearer "+token)
		return nil
	}
}

func stringMap(m map[string]any) (map[string]string, error) {
	if m == nil {
		return nil, nil
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		s, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("metadata value for key %q is %T, expected string", k, v)
		}
		out[k] = s
	}
	return out, nil
}

// anyMapPtr keeps the previous omitempty behaviour: nil and empty maps are not sent.
func anyMapPtr(m map[string]string) *map[string]any {
	if len(m) == 0 {
		return nil
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return &out
}
