package server

import "testing"

func TestPreviewAPI(t *testing.T) {
	srv, _ := newTestServer(t)

	tests := []struct {
		name     string
		path     string
		wantCode int
		contains []string
	}{
		{
			name:     "single dashboard overview",
			path:     "/api/preview?uid=test-overview",
			wantCode: 200,
			contains: []string{"test overview", "stat", "timeseries"},
		},
		{
			name:     "single dashboard network",
			path:     "/api/preview?uid=test-network",
			wantCode: 200,
			contains: []string{"test network"},
		},
		{
			name:     "missing uid parameter",
			path:     "/api/preview",
			wantCode: 200,
			contains: []string{"select a dashboard"},
		},
		{
			name:     "nonexistent dashboard",
			path:     "/api/preview?uid=nonexistent",
			wantCode: 200,
			contains: []string{"not found"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := doGet(t, srv, tt.path)
			assertStatus(t, w, tt.wantCode)
			for _, s := range tt.contains {
				assertContains(t, w, s)
			}
		})
	}
}

func TestPreviewAllDashboards(t *testing.T) {
	srv, _ := newTestServer(t)

	w := doGet(t, srv, "/api/preview?uid=all")
	assertStatus(t, w, 200)
	assertContains(t, w, "all dashboards")
	assertContains(t, w, "test overview")
	assertContains(t, w, "test network")
}
