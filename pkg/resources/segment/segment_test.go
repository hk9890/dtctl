package segment

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dynatrace-oss/dtctl/pkg/client"
)

func TestNewHandler(t *testing.T) {
	c, err := client.New("https://test.dynatrace.com", "test-token")
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	h := NewHandler(c)

	if h == nil {
		t.Fatal("NewHandler() returned nil")
	}
	if h.client == nil {
		t.Error("Handler.client is nil")
	}
}

func TestList(t *testing.T) {
	tests := []struct {
		name          string
		statusCode    int
		responseBody  interface{}
		expectError   bool
		errorContains string
		validate      func(*testing.T, *FilterSegmentList)
	}{
		{
			name:       "successful list",
			statusCode: 200,
			responseBody: FilterSegmentList{
				FilterSegments: []FilterSegment{
					{
						UID:      "seg-uid-001",
						Name:     "k8s-alpha",
						IsPublic: true,
						Owner:    "user@example.invalid",
					},
					{
						UID:      "seg-uid-002",
						Name:     "prod-logs",
						IsPublic: false,
						Owner:    "admin@example.invalid",
					},
				},
				TotalCount: 2,
			},
			expectError: false,
			validate: func(t *testing.T, result *FilterSegmentList) {
				if len(result.FilterSegments) != 2 {
					t.Errorf("expected 2 segments, got %d", len(result.FilterSegments))
				}
				if result.FilterSegments[0].UID != "seg-uid-001" {
					t.Errorf("expected first segment UID 'seg-uid-001', got %q", result.FilterSegments[0].UID)
				}
				if result.FilterSegments[1].Name != "prod-logs" {
					t.Errorf("expected second segment name 'prod-logs', got %q", result.FilterSegments[1].Name)
				}
			},
		},
		{
			name:       "empty list",
			statusCode: 200,
			responseBody: FilterSegmentList{
				FilterSegments: []FilterSegment{},
				TotalCount:     0,
			},
			expectError: false,
			validate: func(t *testing.T, result *FilterSegmentList) {
				if len(result.FilterSegments) != 0 {
					t.Errorf("expected 0 segments, got %d", len(result.FilterSegments))
				}
			},
		},
		{
			name:          "server error",
			statusCode:    500,
			responseBody:  "internal server error",
			expectError:   true,
			errorContains: "status 500",
		},
		{
			name:          "forbidden",
			statusCode:    403,
			responseBody:  "access denied",
			expectError:   true,
			errorContains: "status 403",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/platform/storage/management/v1/filter-segments" {
					t.Errorf("expected path '/platform/storage/management/v1/filter-segments', got %q", r.URL.Path)
				}
				// Simulate API constraint: page-size must not be combined with page-key
				if r.URL.Query().Get("page-size") != "" && r.URL.Query().Get("page-key") != "" {
					w.WriteHeader(http.StatusBadRequest)
					w.Write([]byte(`{"error":{"code":400,"message":"Constraints violated."}}`))
					return
				}
				w.WriteHeader(tt.statusCode)
				if str, ok := tt.responseBody.(string); ok {
					w.Write([]byte(str))
				} else {
					json.NewEncoder(w).Encode(tt.responseBody)
				}
			}))
			defer server.Close()

			c, err := client.NewForTesting(server.URL, "test-token")
			if err != nil {
				t.Fatalf("failed to create client: %v", err)
			}
			h := NewHandler(c)

			result, err := h.List()

			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("expected error containing %q, got %q", tt.errorContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if tt.validate != nil {
					tt.validate(t, result)
				}
			}
		})
	}
}

func TestListPagination(t *testing.T) {
	page := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate API constraint: page-size must not be combined with page-key
		if r.URL.Query().Get("page-size") != "" && r.URL.Query().Get("page-key") != "" {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"error":{"code":400,"message":"Constraints violated."}}`))
			return
		}

		page++
		switch page {
		case 1:
			if r.URL.Query().Get("page-key") != "" {
				t.Error("first request should not have page-key")
			}
			json.NewEncoder(w).Encode(FilterSegmentList{
				FilterSegments: []FilterSegment{
					{UID: "seg-001", Name: "segment-1"},
				},
				TotalCount:  2,
				NextPageKey: "page2token",
			})
		case 2:
			if r.URL.Query().Get("page-key") != "page2token" {
				t.Errorf("expected page-key 'page2token', got %q", r.URL.Query().Get("page-key"))
			}
			json.NewEncoder(w).Encode(FilterSegmentList{
				FilterSegments: []FilterSegment{
					{UID: "seg-002", Name: "segment-2"},
				},
				TotalCount: 2,
			})
		default:
			t.Errorf("unexpected page request: %d", page)
		}
	}))
	defer server.Close()

	c, err := client.NewForTesting(server.URL, "test-token")
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	h := NewHandler(c)

	result, err := h.List()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.FilterSegments) != 2 {
		t.Errorf("expected 2 segments after pagination, got %d", len(result.FilterSegments))
	}
	if result.FilterSegments[0].UID != "seg-001" {
		t.Errorf("expected first segment UID 'seg-001', got %q", result.FilterSegments[0].UID)
	}
	if result.FilterSegments[1].UID != "seg-002" {
		t.Errorf("expected second segment UID 'seg-002', got %q", result.FilterSegments[1].UID)
	}
}

func TestGet(t *testing.T) {
	tests := []struct {
		name          string
		uid           string
		statusCode    int
		responseBody  interface{}
		expectError   bool
		errorContains string
		validate      func(*testing.T, *FilterSegment)
	}{
		{
			name:       "successful get",
			uid:        "seg-uid-001",
			statusCode: 200,
			responseBody: FilterSegment{
				UID:         "seg-uid-001",
				Name:        "k8s-alpha",
				Description: "Kubernetes cluster alpha",
				IsPublic:    true,
				Owner:       "user@example.invalid",
				Version:     3,
				Includes: []Include{
					{DataType: "all", Filter: `k8s.cluster.name = "alpha"`},
					{DataType: "logs", Filter: `dt.system.bucket = "custom-logs"`},
				},
				Variables: &Variables{
					Query:   `data record(ns="namespace-a"), record(ns="namespace-b")`,
					Columns: []string{"ns"},
				},
				AllowedOperations: []string{"READ", "WRITE", "DELETE"},
			},
			expectError: false,
			validate: func(t *testing.T, seg *FilterSegment) {
				if seg.UID != "seg-uid-001" {
					t.Errorf("expected UID 'seg-uid-001', got %q", seg.UID)
				}
				if seg.Name != "k8s-alpha" {
					t.Errorf("expected name 'k8s-alpha', got %q", seg.Name)
				}
				if len(seg.Includes) != 2 {
					t.Errorf("expected 2 includes, got %d", len(seg.Includes))
				}
				if seg.Variables == nil {
					t.Error("expected variables to be non-nil")
				} else if len(seg.Variables.Columns) != 1 {
					t.Errorf("expected 1 variable column, got %d", len(seg.Variables.Columns))
				}
			},
		},
		{
			name:          "segment not found",
			uid:           "non-existent",
			statusCode:    404,
			responseBody:  "not found",
			expectError:   true,
			errorContains: "not found",
		},
		{
			name:          "server error",
			uid:           "seg-uid-001",
			statusCode:    500,
			responseBody:  "internal error",
			expectError:   true,
			errorContains: "status 500",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				expectedPath := fmt.Sprintf("/platform/storage/management/v1/filter-segments/%s", tt.uid)
				if r.URL.Path != expectedPath {
					t.Errorf("expected path %q, got %q", expectedPath, r.URL.Path)
				}
				w.WriteHeader(tt.statusCode)
				if str, ok := tt.responseBody.(string); ok {
					w.Write([]byte(str))
				} else {
					json.NewEncoder(w).Encode(tt.responseBody)
				}
			}))
			defer server.Close()

			c, err := client.NewForTesting(server.URL, "test-token")
			if err != nil {
				t.Fatalf("failed to create client: %v", err)
			}
			h := NewHandler(c)

			result, err := h.Get(tt.uid)

			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("expected error containing %q, got %q", tt.errorContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if tt.validate != nil {
					tt.validate(t, result)
				}
			}
		})
	}
}

func TestCreate(t *testing.T) {
	tests := []struct {
		name          string
		input         FilterSegment
		statusCode    int
		responseBody  interface{}
		expectError   bool
		errorContains string
		validate      func(*testing.T, *FilterSegment)
	}{
		{
			name: "successful create",
			input: FilterSegment{
				Name:     "new-segment",
				IsPublic: true,
				Includes: []Include{{DataType: "logs", Filter: `status = "ERROR"`}},
			},
			statusCode: 201,
			responseBody: FilterSegment{
				UID:      "seg-new-001",
				Name:     "new-segment",
				IsPublic: true,
				Owner:    "user@example.invalid",
				Version:  1,
				Includes: []Include{{DataType: "logs", Filter: `status = "ERROR"`}},
			},
			expectError: false,
			validate: func(t *testing.T, seg *FilterSegment) {
				if seg.UID != "seg-new-001" {
					t.Errorf("expected UID 'seg-new-001', got %q", seg.UID)
				}
				if seg.Name != "new-segment" {
					t.Errorf("expected name 'new-segment', got %q", seg.Name)
				}
			},
		},
		{
			name: "invalid definition",
			input: FilterSegment{
				Name: "",
			},
			statusCode:    400,
			responseBody:  "invalid segment definition",
			expectError:   true,
			errorContains: "invalid segment definition",
		},
		{
			name: "access denied",
			input: FilterSegment{
				Name: "denied-segment",
			},
			statusCode:    403,
			responseBody:  "access denied",
			expectError:   true,
			errorContains: "access denied",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "POST" {
					t.Errorf("expected POST method, got %s", r.Method)
				}
				if r.URL.Path != "/platform/storage/management/v1/filter-segments" {
					t.Errorf("expected path '/platform/storage/management/v1/filter-segments', got %q", r.URL.Path)
				}
				w.WriteHeader(tt.statusCode)
				if str, ok := tt.responseBody.(string); ok {
					w.Write([]byte(str))
				} else {
					json.NewEncoder(w).Encode(tt.responseBody)
				}
			}))
			defer server.Close()

			c, err := client.NewForTesting(server.URL, "test-token")
			if err != nil {
				t.Fatalf("failed to create client: %v", err)
			}
			h := NewHandler(c)

			inputJSON, _ := json.Marshal(tt.input)
			result, err := h.Create(inputJSON)

			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("expected error containing %q, got %q", tt.errorContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if tt.validate != nil {
					tt.validate(t, result)
				}
			}
		})
	}
}

func TestUpdate(t *testing.T) {
	tests := []struct {
		name          string
		uid           string
		statusCode    int
		responseBody  string
		expectError   bool
		errorContains string
	}{
		{
			name:        "successful update",
			uid:         "seg-uid-001",
			statusCode:  200,
			expectError: false,
		},
		{
			name:          "segment not found",
			uid:           "non-existent",
			statusCode:    404,
			responseBody:  "not found",
			expectError:   true,
			errorContains: "not found",
		},
		{
			name:          "version conflict",
			uid:           "seg-uid-001",
			statusCode:    409,
			responseBody:  "version conflict",
			expectError:   true,
			errorContains: "version conflict",
		},
		{
			name:          "invalid definition",
			uid:           "seg-uid-001",
			statusCode:    400,
			responseBody:  "invalid",
			expectError:   true,
			errorContains: "invalid segment definition",
		},
		{
			name:          "access denied",
			uid:           "seg-uid-001",
			statusCode:    403,
			responseBody:  "access denied",
			expectError:   true,
			errorContains: "access denied",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "PUT" {
					t.Errorf("expected PUT method, got %s", r.Method)
				}
				expectedPath := fmt.Sprintf("/platform/storage/management/v1/filter-segments/%s", tt.uid)
				if r.URL.Path != expectedPath {
					t.Errorf("expected path %q, got %q", expectedPath, r.URL.Path)
				}
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.responseBody))
			}))
			defer server.Close()

			c, err := client.NewForTesting(server.URL, "test-token")
			if err != nil {
				t.Fatalf("failed to create client: %v", err)
			}
			h := NewHandler(c)

			updateData := []byte(`{"name":"updated-segment","isPublic":true}`)
			err = h.Update(tt.uid, updateData)

			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("expected error containing %q, got %q", tt.errorContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestDelete(t *testing.T) {
	tests := []struct {
		name          string
		uid           string
		statusCode    int
		responseBody  string
		expectError   bool
		errorContains string
	}{
		{
			name:        "successful delete",
			uid:         "seg-uid-001",
			statusCode:  204,
			expectError: false,
		},
		{
			name:          "segment not found",
			uid:           "non-existent",
			statusCode:    404,
			responseBody:  "not found",
			expectError:   true,
			errorContains: "not found",
		},
		{
			name:          "access denied",
			uid:           "seg-uid-001",
			statusCode:    403,
			responseBody:  "access denied",
			expectError:   true,
			errorContains: "access denied",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "DELETE" {
					t.Errorf("expected DELETE method, got %s", r.Method)
				}
				expectedPath := fmt.Sprintf("/platform/storage/management/v1/filter-segments/%s", tt.uid)
				if r.URL.Path != expectedPath {
					t.Errorf("expected path %q, got %q", expectedPath, r.URL.Path)
				}
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.responseBody))
			}))
			defer server.Close()

			c, err := client.NewForTesting(server.URL, "test-token")
			if err != nil {
				t.Fatalf("failed to create client: %v", err)
			}
			h := NewHandler(c)

			err = h.Delete(tt.uid)

			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("expected error containing %q, got %q", tt.errorContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestGetRaw(t *testing.T) {
	t.Run("successful get raw", func(t *testing.T) {
		expectedSegment := FilterSegment{
			UID:      "seg-uid-001",
			Name:     "test-segment",
			IsPublic: true,
			Owner:    "user@example.invalid",
			Version:  1,
			Includes: []Include{
				{DataType: "logs", Filter: `status = "ERROR"`},
			},
		}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(200)
			json.NewEncoder(w).Encode(expectedSegment)
		}))
		defer server.Close()

		c, err := client.NewForTesting(server.URL, "test-token")
		if err != nil {
			t.Fatalf("failed to create client: %v", err)
		}
		h := NewHandler(c)

		raw, err := h.GetRaw("seg-uid-001")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify it's valid JSON
		var seg FilterSegment
		if err := json.Unmarshal(raw, &seg); err != nil {
			t.Fatalf("failed to unmarshal raw JSON: %v", err)
		}

		if seg.UID != expectedSegment.UID {
			t.Errorf("expected UID %q, got %q", expectedSegment.UID, seg.UID)
		}
		if seg.Name != expectedSegment.Name {
			t.Errorf("expected name %q, got %q", expectedSegment.Name, seg.Name)
		}
	})

	t.Run("get raw with non-existent segment", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(404)
			w.Write([]byte("not found"))
		}))
		defer server.Close()

		c, err := client.NewForTesting(server.URL, "test-token")
		if err != nil {
			t.Fatalf("failed to create client: %v", err)
		}
		h := NewHandler(c)

		_, err = h.GetRaw("non-existent")
		if err == nil {
			t.Error("expected error, got nil")
		}
	})
}
