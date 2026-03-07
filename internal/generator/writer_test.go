package generator

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCountPanels(t *testing.T) {
	tests := []struct {
		name  string
		input map[string]interface{}
		want  int
	}{
		{
			name:  "with panels",
			input: map[string]interface{}{"panels": []interface{}{1, 2, 3}},
			want:  3,
		},
		{
			name:  "empty map",
			input: map[string]interface{}{},
			want:  0,
		},
		{
			name:  "no panels key",
			input: map[string]interface{}{"title": "test"},
			want:  0,
		},
		{
			name:  "panels wrong type",
			input: map[string]interface{}{"panels": "not a slice"},
			want:  0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := countPanels(tt.input)
			if got != tt.want {
				t.Errorf("countPanels() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestFormatSize(t *testing.T) {
	tests := []struct {
		input int
		want  string
	}{
		{0, "0"},
		{999, "999"},
		{1000, "1,000"},
		{1234567, "1,234,567"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := formatSize(tt.input)
			if got != tt.want {
				t.Errorf("formatSize(%d) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestTrimSlash(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"http://localhost:3000", "http://localhost:3000"},
		{"http://localhost:3000/", "http://localhost:3000"},
		{"http://localhost:3000///", "http://localhost:3000"},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := trimSlash(tt.input)
			if got != tt.want {
				t.Errorf("trimSlash(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestWriteDashboardDryRun(t *testing.T) {
	dir := t.TempDir()
	fpath := filepath.Join(dir, "test.json")

	dashboard := map[string]interface{}{
		"uid":    "test",
		"panels": []interface{}{map[string]interface{}{}, map[string]interface{}{}, map[string]interface{}{}},
	}

	size, err := WriteDashboard(dashboard, fpath, true)
	if err != nil {
		t.Fatalf("WriteDashboard(dryRun=true) error: %v", err)
	}
	if size <= 0 {
		t.Errorf("expected size > 0, got %d", size)
	}
	if _, err := os.Stat(fpath); !os.IsNotExist(err) {
		t.Error("expected file to not exist in dry run mode")
	}
}

func TestWriteDashboard(t *testing.T) {
	dir := t.TempDir()
	fpath := filepath.Join(dir, "test.json")

	dashboard := map[string]interface{}{
		"uid":    "test",
		"panels": []interface{}{map[string]interface{}{}, map[string]interface{}{}, map[string]interface{}{}},
	}

	size, err := WriteDashboard(dashboard, fpath, false)
	if err != nil {
		t.Fatalf("WriteDashboard() error: %v", err)
	}
	if size <= 0 {
		t.Errorf("expected size > 0, got %d", size)
	}

	data, err := os.ReadFile(fpath)
	if err != nil {
		t.Fatalf("reading written file: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("written file is not valid JSON: %v", err)
	}
	if parsed["uid"] != "test" {
		t.Errorf("uid = %v, want %q", parsed["uid"], "test")
	}
}

func TestPushToGrafana(t *testing.T) {
	var receivedAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/dashboards/db" {
			t.Errorf("expected /api/dashboards/db, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "success", "uid": "test"})
	}))
	defer srv.Close()

	dashboard := map[string]interface{}{"uid": "test", "panels": []interface{}{}}
	err := PushToGrafana(dashboard, srv.URL, "", "", "my-token")
	if err != nil {
		t.Fatalf("PushToGrafana() error: %v", err)
	}
	if receivedAuth != "Bearer my-token" {
		t.Errorf("Authorization = %q, want %q", receivedAuth, "Bearer my-token")
	}
}

func TestPushToGrafanaBasicAuth(t *testing.T) {
	var receivedAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "success", "uid": "test"})
	}))
	defer srv.Close()

	dashboard := map[string]interface{}{"uid": "test", "panels": []interface{}{}}
	err := PushToGrafana(dashboard, srv.URL, "admin", "secret", "")
	if err != nil {
		t.Fatalf("PushToGrafana() error: %v", err)
	}
	if !strings.HasPrefix(receivedAuth, "Basic ") {
		t.Errorf("Authorization = %q, want prefix %q", receivedAuth, "Basic ")
	}
}

func TestPushToGrafanaError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("internal error"))
	}))
	defer srv.Close()

	dashboard := map[string]interface{}{"uid": "test", "panels": []interface{}{}}
	err := PushToGrafana(dashboard, srv.URL, "", "", "tok")
	if err == nil {
		t.Fatal("expected error for 500 response, got nil")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("error %q should contain status code 500", err.Error())
	}
}

func TestPushToGrafanaTrailingSlash(t *testing.T) {
	var receivedPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "success", "uid": "test"})
	}))
	defer srv.Close()

	dashboard := map[string]interface{}{"uid": "test", "panels": []interface{}{}}
	err := PushToGrafana(dashboard, srv.URL+"/", "", "", "tok")
	if err != nil {
		t.Fatalf("PushToGrafana() error: %v", err)
	}
	if receivedPath != "/api/dashboards/db" {
		t.Errorf("path = %q, want /api/dashboards/db", receivedPath)
	}
}
