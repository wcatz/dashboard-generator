package server

import (
	"net/url"
	"os"
	"testing"
)

func TestConfigReload(t *testing.T) {
	srv, _ := newTestServer(t)

	t.Run("POST succeeds", func(t *testing.T) {
		w := doPost(t, srv, "/api/config/reload", "")
		assertStatus(t, w, 200)
		assertContains(t, w, "config reloaded")
	})

	t.Run("GET returns 405", func(t *testing.T) {
		w := doGet(t, srv, "/api/config/reload")
		assertStatus(t, w, 405)
	})
}

func TestConfigSave(t *testing.T) {
	srv, cfgPath := newTestServer(t)

	t.Run("valid YAML", func(t *testing.T) {
		raw, err := os.ReadFile(cfgPath)
		if err != nil {
			t.Fatalf("reading config: %v", err)
		}
		w := doPost(t, srv, "/api/config/save", "content="+url.QueryEscape(string(raw)))
		assertStatus(t, w, 200)
		assertContains(t, w, "saved and reloaded")
	})

	t.Run("empty content", func(t *testing.T) {
		w := doPost(t, srv, "/api/config/save", "content=")
		assertStatus(t, w, 200)
		assertContains(t, w, "empty content")
	})

	t.Run("invalid YAML", func(t *testing.T) {
		w := doPost(t, srv, "/api/config/save", "content="+url.QueryEscape(`": invalid`))
		assertStatus(t, w, 200)
		assertContains(t, w, "invalid YAML")
	})

	t.Run("GET returns 405", func(t *testing.T) {
		w := doGet(t, srv, "/api/config/save")
		assertStatus(t, w, 405)
	})
}
