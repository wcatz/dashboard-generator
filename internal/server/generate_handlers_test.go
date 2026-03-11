package server

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/wcatz/dashboard-generator/web"
)

func TestGenerate(t *testing.T) {
	srv, _ := newTestServer(t)

	t.Run("POST generates all dashboards", func(t *testing.T) {
		w := doPost(t, srv, "/api/generate", "")
		assertStatus(t, w, 200)
		assertContains(t, w, "test-overview")
		assertContains(t, w, "test-network")
	})

	t.Run("GET returns 405", func(t *testing.T) {
		w := doGet(t, srv, "/api/generate")
		assertStatus(t, w, 405)
	})
}

func TestGenerateSingleDashboard(t *testing.T) {
	srv, _ := newTestServer(t)

	w := doPost(t, srv, "/api/generate?dashboard=test-overview", "")
	assertStatus(t, w, 200)
	assertContains(t, w, "test-overview")
	assertNotContains(t, w, "test-network")
}

func TestPushNoGrafanaURL(t *testing.T) {
	srv, _ := newTestServer(t)

	t.Run("POST without Grafana URL configured", func(t *testing.T) {
		w := doPost(t, srv, "/api/push", "")
		assertStatus(t, w, 200)
		assertContains(t, w, "no Grafana URL configured")
	})

	t.Run("GET returns 405", func(t *testing.T) {
		w := doGet(t, srv, "/api/push")
		assertStatus(t, w, 405)
	})
}

func TestPushWithMockGrafana(t *testing.T) {
	// Create a mock Grafana server that accepts dashboard pushes
	var pushCount int
	grafana := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/dashboards/db" && r.Method == http.MethodPost {
			pushCount++
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "success", "uid": "test"})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer grafana.Close()

	// Create server with Grafana URL set
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(cfgPath, []byte(testConfig), 0644); err != nil {
		t.Fatalf("writing config: %v", err)
	}
	outDir := filepath.Join(dir, "output")
	_ = os.MkdirAll(outDir, 0755)

	webFS, err := fs.Sub(web.EmbeddedFS, ".")
	if err != nil {
		t.Fatalf("creating web FS: %v", err)
	}
	srv, err := New(webFS, cfgPath, "", grafana.URL, "")
	if err != nil {
		t.Fatalf("creating server: %v", err)
	}

	w := doPost(t, srv, "/api/push", "")
	assertStatus(t, w, 200)
	// Should have pushed 2 dashboards (test-overview + test-network)
	if pushCount != 2 {
		t.Errorf("expected 2 pushes, got %d", pushCount)
	}
	assertContains(t, w, "success")
}

func TestPushSingleDashboard(t *testing.T) {
	var pushCount int
	grafana := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pushCount++
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "success", "uid": "test"})
	}))
	defer grafana.Close()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(cfgPath, []byte(testConfig), 0644); err != nil {
		t.Fatalf("writing config: %v", err)
	}
	_ = os.MkdirAll(filepath.Join(dir, "output"), 0755)

	webFS, err := fs.Sub(web.EmbeddedFS, ".")
	if err != nil {
		t.Fatalf("creating web FS: %v", err)
	}
	srv, err := New(webFS, cfgPath, "", grafana.URL, "")
	if err != nil {
		t.Fatalf("creating server: %v", err)
	}

	w := doPost(t, srv, "/api/push?dashboard=test-overview", "")
	assertStatus(t, w, 200)
	if pushCount != 1 {
		t.Errorf("expected 1 push, got %d", pushCount)
	}
}
