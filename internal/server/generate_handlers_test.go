package server

import (
	"testing"
)

func TestGenerate(t *testing.T) {
	srv, _ := newTestServer(t)

	t.Run("POST generates all dashboards", func(t *testing.T) {
		w := doPost(t, srv, "/api/generate", "")
		assertStatus(t, w, 200)
		assertContains(t, w, "test-overview.json")
		assertContains(t, w, "test-network.json")
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
	assertContains(t, w, "test-overview.json")
	assertNotContains(t, w, "test-network.json")
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
