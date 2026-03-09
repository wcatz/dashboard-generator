package server

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wcatz/dashboard-generator/web"
)

// testConfig is a minimal but complete YAML config for testing all handler paths.
const testConfig = `
generator:
  schema_version: 39
  refresh: "30s"
  time_range:
    from: "now-6h"
    to: "now"
  output_dir: "output"
  editable: true
  graph_tooltip: 1
  timezone: "utc"

datasources:
  primary:
    type: prometheus
    uid: prom_primary
    url: "http://localhost:9090"
    is_default: true
  secondary:
    type: prometheus
    uid: prom_secondary
    url: "http://localhost:9091"

palettes:
  default:
    green: "#73BF69"
    red: "#F2495C"
    blue: "#5794F2"

active_palette: default

thresholds:
  percent_usage:
    - { color: "$green", value: 0 }
    - { color: "$red", value: 80 }

selectors:
  host: '{instance=~"$instance"}'

constants:
  rate_interval: "5m"

variables:
  instance:
    type: query
    datasource: primary
    query: 'label_values(up, instance)'
    multi: true
    include_all: true
    refresh: 2
    sort: 1

profiles:
  core:
    dashboards: [test-overview]

dashboards:
  test-overview:
    uid: test-overview
    title: test overview
    filename: test-overview.json
    tags: [test]
    icon: apps
    description: "test dashboard"
    variables: [instance]
    sections:
      - title: overview
        panels:
          - type: stat
            title: up
            query: 'up'
          - type: timeseries
            title: cpu
            query: 'rate(process_cpu_seconds_total[5m])'
            width: 12
  test-network:
    uid: test-network
    title: test network
    filename: test-network.json
    tags: [test]
    icon: cloud
    description: "network dashboard"
    variables: [instance]
    sections:
      - title: traffic
        panels:
          - type: timeseries
            title: bytes in
            query: 'rate(node_network_receive_bytes_total[5m])'
`

// newTestServer creates a Server backed by a temp config file and the real embedded FS.
// Returns the server, the temp config file path, and a cleanup function.
func newTestServer(t *testing.T) (*Server, string) {
	t.Helper()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(cfgPath, []byte(testConfig), 0644); err != nil {
		t.Fatalf("writing test config: %v", err)
	}

	// Create output dir so generate handler can write files
	outDir := filepath.Join(dir, "output")
	_ = os.MkdirAll(outDir, 0755)

	webFS, err := fs.Sub(web.EmbeddedFS, ".")
	if err != nil {
		t.Fatalf("creating web FS: %v", err)
	}

	srv, err := New(webFS, cfgPath, "", "")
	if err != nil {
		t.Fatalf("creating test server: %v", err)
	}

	return srv, cfgPath
}

// doGet performs a GET request against the server and returns the recorder.
func doGet(t *testing.T, srv *Server, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	return w
}

// doPost performs a POST request with form data against the server.
func doPost(t *testing.T, srv *Server, path string, formData string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(formData))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	// Set Origin to match Host for CSRF check
	req.Header.Set("Origin", "http://"+req.Host)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	return w
}

// assertStatus checks the response status code.
func assertStatus(t *testing.T, w *httptest.ResponseRecorder, want int) {
	t.Helper()
	if w.Code != want {
		t.Errorf("got status %d, want %d\nbody: %s", w.Code, want, w.Body.String()[:min(500, w.Body.Len())])
	}
}

// assertContains checks that the response body contains the given substring.
func assertContains(t *testing.T, w *httptest.ResponseRecorder, substr string) {
	t.Helper()
	if !strings.Contains(w.Body.String(), substr) {
		t.Errorf("response body does not contain %q\nbody prefix: %s", substr, w.Body.String()[:min(500, w.Body.Len())])
	}
}

// assertNotContains checks that the response body does NOT contain the given substring.
func assertNotContains(t *testing.T, w *httptest.ResponseRecorder, substr string) {
	t.Helper()
	if strings.Contains(w.Body.String(), substr) {
		t.Errorf("response body should not contain %q but does", substr)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
