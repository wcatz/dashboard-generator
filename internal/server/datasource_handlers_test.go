package server

import "testing"

func TestHandleDatasourceAdd(t *testing.T) {
	srv, _ := newTestServer(t)

	rec := doPost(t, srv, "/api/datasource/add", "name=staging&url=http://staging:9090")
	assertStatus(t, rec, 200)

	cfg := srv.Config()
	ds, ok := cfg.Datasources["staging"]
	if !ok {
		t.Fatal("datasource 'staging' not found after add")
	}
	if ds.URL != "http://staging:9090" {
		t.Errorf("expected URL http://staging:9090, got %s", ds.URL)
	}
}

func TestHandleDatasourceAddSanitizesName(t *testing.T) {
	srv, _ := newTestServer(t)

	rec := doPost(t, srv, "/api/datasource/add", "name=My+Staging+DS&url=http://staging:9090")
	assertStatus(t, rec, 200)

	cfg := srv.Config()
	if _, ok := cfg.Datasources["my-staging-ds"]; !ok {
		t.Errorf("expected sanitized name 'my-staging-ds' in config, got keys: %v", keys(cfg.Datasources))
	}
}

func TestHandleDatasourceAddMissingName(t *testing.T) {
	srv, _ := newTestServer(t)

	rec := doPost(t, srv, "/api/datasource/add", "url=http://staging:9090")
	assertStatus(t, rec, 200)
	assertContains(t, rec, "required")
}

func TestHandleDatasourceAddMissingURL(t *testing.T) {
	srv, _ := newTestServer(t)

	rec := doPost(t, srv, "/api/datasource/add", "name=staging")
	assertStatus(t, rec, 200)
	assertContains(t, rec, "required")
}

func TestHandleDatasourceDelete(t *testing.T) {
	srv, _ := newTestServer(t)

	rec := doPost(t, srv, "/api/datasource/delete", "name=secondary")
	assertStatus(t, rec, 200)

	cfg := srv.Config()
	if _, ok := cfg.Datasources["secondary"]; ok {
		t.Error("datasource 'secondary' should have been deleted")
	}
}

func TestHandleDatasourceDeleteMissing(t *testing.T) {
	srv, _ := newTestServer(t)

	rec := doPost(t, srv, "/api/datasource/delete", "name=nonexistent")
	assertStatus(t, rec, 200)
	assertContains(t, rec, "not found")
}

func TestHandleDatasourceDeleteEmptyName(t *testing.T) {
	srv, _ := newTestServer(t)

	rec := doPost(t, srv, "/api/datasource/delete", "name=")
	assertStatus(t, rec, 400)
}

func TestHandleDatasourceURL(t *testing.T) {
	srv, _ := newTestServer(t)

	rec := doPost(t, srv, "/api/datasource/url", "name=primary&url=http://new:9090")
	assertStatus(t, rec, 200)

	cfg := srv.Config()
	ds, ok := cfg.Datasources["primary"]
	if !ok {
		t.Fatal("datasource 'primary' not found after URL update")
	}
	if ds.URL != "http://new:9090" {
		t.Errorf("expected URL http://new:9090, got %s", ds.URL)
	}
}

func TestHandleDatasourceURLMissingFields(t *testing.T) {
	srv, _ := newTestServer(t)

	tests := []struct {
		name string
		form string
	}{
		{"missing both", ""},
		{"missing url", "name=primary"},
		{"missing name", "url=http://new:9090"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := doPost(t, srv, "/api/datasource/url", tt.form)
			assertStatus(t, rec, 200)
			assertContains(t, rec, "required")
		})
	}
}

// keys returns the map keys for diagnostic messages.
func keys[K comparable, V any](m map[K]V) []K {
	out := make([]K, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
