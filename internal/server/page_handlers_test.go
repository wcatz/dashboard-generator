package server

import (
	"net/http"
	"testing"
)

func TestPageHandlers(t *testing.T) {
	srv, _ := newTestServer(t)

	tests := []struct {
		name     string
		route    string
		contains []string
	}{
		{
			name:     "index",
			route:    "/",
			contains: []string{"test overview", "test network", "dashboards"},
		},
		{
			name:     "datasources",
			route:    "/datasources",
			contains: []string{"primary", "secondary", "datasources"},
		},
		{
			name:     "variables",
			route:    "/variables",
			contains: []string{"instance", "variables"},
		},
		{
			name:     "palettes",
			route:    "/palettes",
			contains: []string{"default", "palettes"},
		},
		{
			name:     "references",
			route:    "/references",
			contains: []string{"rate_interval", "host"},
		},
		{
			name:     "editor",
			route:    "/editor",
			contains: []string{"editor"},
		},
		{
			name:     "metrics",
			route:    "/metrics",
			contains: []string{"metrics"},
		},
		{
			name:     "preview",
			route:    "/preview",
			contains: []string{"preview", "test-overview"},
		},
		{
			name:     "profiles",
			route:    "/profiles",
			contains: []string{"core", "profiles"},
		},
		{
			name:     "settings",
			route:    "/settings",
			contains: []string{"settings", "30s"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := doGet(t, srv, tt.route)
			assertStatus(t, w, http.StatusOK)
			for _, substr := range tt.contains {
				assertContains(t, w, substr)
			}
		})
	}
}

func TestIndexPageStats(t *testing.T) {
	srv, _ := newTestServer(t)

	w := doGet(t, srv, "/")
	assertStatus(t, w, http.StatusOK)

	// 3 total panels (2 from test-overview + 1 from test-network)
	assertContains(t, w, "3")
	// 2 datasources
	assertContains(t, w, "2")
	// 1 variable
	assertContains(t, w, "1")
}

func TestIndexNotFoundPath(t *testing.T) {
	srv, _ := newTestServer(t)

	w := doGet(t, srv, "/nonexistent")
	assertStatus(t, w, http.StatusNotFound)
}
