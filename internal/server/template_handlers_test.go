package server

import (
	"net/http"
	"testing"
)

func TestHandleTemplatesPage(t *testing.T) {
	srv, _ := newTestServer(t)
	w := doGet(t, srv, "/templates")
	assertStatus(t, w, http.StatusOK)
	assertContains(t, w, "templates")
	// Sidebar active link should be present
	assertContains(t, w, "active")
	// At least one built-in template should appear
	assertContains(t, w, "Minimal")
}

func TestHandleTemplatePreview(t *testing.T) {
	srv, _ := newTestServer(t)
	w := doGet(t, srv, "/api/template/preview?name=Minimal")
	assertStatus(t, w, http.StatusOK)
	// Template content must contain valid YAML anchors
	assertContains(t, w, "datasources")
	assertContains(t, w, "dashboards")
}

func TestHandleTemplatePreview_MissingName(t *testing.T) {
	srv, _ := newTestServer(t)
	w := doGet(t, srv, "/api/template/preview")
	assertStatus(t, w, http.StatusBadRequest)
}

func TestHandleTemplatePreview_UnknownTemplate(t *testing.T) {
	srv, _ := newTestServer(t)
	w := doGet(t, srv, "/api/template/preview?name=DoesNotExist")
	assertStatus(t, w, http.StatusNotFound)
}

func TestHandleTemplateLoad_ValidTemplate(t *testing.T) {
	srv, cfgPath := newTestServer(t)

	// Record config path used by this server
	if srv.ConfigPath() != cfgPath {
		t.Fatalf("server config path mismatch: got %s, want %s", srv.ConfigPath(), cfgPath)
	}

	w := doPost(t, srv, "/api/template/load", "template=Minimal")
	// Should succeed (200 with HX-Redirect or redirect body) — not 400/500
	if w.Code >= 400 {
		t.Errorf("expected success status, got %d\nbody: %s", w.Code, w.Body.String())
	}
	// Validate-before-write: config file should be updated with the template content
	// (we only check the server reloaded cleanly by making another request)
	w2 := doGet(t, srv, "/")
	assertStatus(t, w2, http.StatusOK)
}

func TestHandleTemplateLoad_UnknownTemplate(t *testing.T) {
	srv, _ := newTestServer(t)
	w := doPost(t, srv, "/api/template/load", "template=NoSuchTemplate")
	assertStatus(t, w, http.StatusNotFound)
}

func TestHandleTemplateLoad_MissingName(t *testing.T) {
	srv, _ := newTestServer(t)
	w := doPost(t, srv, "/api/template/load", "")
	assertStatus(t, w, http.StatusBadRequest)
}

func TestHandleTemplateCreate_Success(t *testing.T) {
	srv, _ := newTestServer(t)
	// Write to a temp path inside the test's temp dir
	outPath := t.TempDir() + "/created.yaml"
	w := doPost(t, srv, "/api/template/create",
		"template=Minimal&path="+outPath)
	assertStatus(t, w, http.StatusOK)
	assertContains(t, w, "successfully")
}

func TestHandleTemplateCreate_PathTraversal(t *testing.T) {
	srv, _ := newTestServer(t)
	w := doPost(t, srv, "/api/template/create",
		"template=Minimal&path=../../etc/passwd")
	assertStatus(t, w, http.StatusBadRequest)
}

func TestHandleTemplateCreate_MissingTemplate(t *testing.T) {
	srv, _ := newTestServer(t)
	w := doPost(t, srv, "/api/template/create", "path=/tmp/x.yaml")
	assertStatus(t, w, http.StatusBadRequest)
}

func TestHandleTemplateCreate_MissingPath(t *testing.T) {
	srv, _ := newTestServer(t)
	w := doPost(t, srv, "/api/template/create", "template=Minimal")
	assertStatus(t, w, http.StatusBadRequest)
}

func TestHandleTemplateCreate_NoOverwrite(t *testing.T) {
	srv, _ := newTestServer(t)
	outPath := t.TempDir() + "/exists.yaml"

	// Create it once
	doPost(t, srv, "/api/template/create", "template=Minimal&path="+outPath)

	// Second attempt without overwrite should conflict
	w := doPost(t, srv, "/api/template/create", "template=Minimal&path="+outPath)
	assertStatus(t, w, http.StatusConflict)
}

func TestHandleTemplateCreate_WithOverwrite(t *testing.T) {
	srv, _ := newTestServer(t)
	outPath := t.TempDir() + "/overwrite.yaml"

	doPost(t, srv, "/api/template/create", "template=Minimal&path="+outPath)

	w := doPost(t, srv, "/api/template/create", "template=Minimal&path="+outPath+"&overwrite=true")
	assertStatus(t, w, http.StatusOK)
	assertContains(t, w, "successfully")
}
