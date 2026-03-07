package server

import "testing"

func TestPaletteColorSet(t *testing.T) {
	srv, _ := newTestServer(t)

	tests := []struct {
		name       string
		method     string
		form       string
		wantStatus int
		contains   string
	}{
		{
			name:       "valid color",
			method:     "POST",
			form:       "palette=default&color=yellow&hex=%23FFFF00",
			wantStatus: 200,
		},
		{
			name:       "missing fields",
			method:     "POST",
			form:       "",
			wantStatus: 200,
			contains:   "missing",
		},
		{
			name:       "method not allowed",
			method:     "GET",
			wantStatus: 405,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.method == "GET" {
				rec := doGet(t, srv, "/api/palette/color/set")
				assertStatus(t, rec, tt.wantStatus)
				if tt.contains != "" {
					assertContains(t, rec, tt.contains)
				}
				return
			}
			rec := doPost(t, srv, "/api/palette/color/set", tt.form)
			assertStatus(t, rec, tt.wantStatus)
			if tt.contains != "" {
				assertContains(t, rec, tt.contains)
			}
		})
	}
}

func TestPaletteColorDelete(t *testing.T) {
	srv, _ := newTestServer(t)

	// Add a color first
	rec := doPost(t, srv, "/api/palette/color/set", "palette=default&color=temp&hex=%23000000")
	assertStatus(t, rec, 200)

	// Delete it
	rec = doPost(t, srv, "/api/palette/color/delete", "palette=default&color=temp")
	assertStatus(t, rec, 200)

	// Missing fields
	rec = doPost(t, srv, "/api/palette/color/delete", "palette=default")
	assertStatus(t, rec, 200)
	assertContains(t, rec, "missing")
}

func TestPaletteCreate(t *testing.T) {
	srv, _ := newTestServer(t)

	tests := []struct {
		name     string
		form     string
		contains string
	}{
		{
			name: "valid create",
			form: "name=test-palette",
		},
		{
			name:     "missing name",
			form:     "",
			contains: "missing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := doPost(t, srv, "/api/palette/create", tt.form)
			assertStatus(t, rec, 200)
			if tt.contains != "" {
				assertContains(t, rec, tt.contains)
			}
		})
	}
}

func TestPaletteDelete(t *testing.T) {
	srv, _ := newTestServer(t)

	// Create a palette to delete
	rec := doPost(t, srv, "/api/palette/create", "name=deleteme")
	assertStatus(t, rec, 200)

	// Delete it
	rec = doPost(t, srv, "/api/palette/delete", "name=deleteme")
	assertStatus(t, rec, 200)
}

func TestPaletteActivate(t *testing.T) {
	srv, _ := newTestServer(t)

	tests := []struct {
		name     string
		form     string
		contains string
	}{
		{
			name: "valid activate",
			form: "name=default",
		},
		{
			name:     "missing name",
			form:     "",
			contains: "missing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := doPost(t, srv, "/api/palette/activate", tt.form)
			assertStatus(t, rec, 200)
			if tt.contains != "" {
				assertContains(t, rec, tt.contains)
			}
		})
	}
}


func TestHandlePaletteColorRename(t *testing.T) {
	srv, _ := newTestServer(t)

	rec := doPost(t, srv, "/api/palette/color/rename", "palette=default&color=blue&new_name=sky")
	assertStatus(t, rec, 200)

	// Verify config: sky exists with blue's old hex, blue is gone
	cfg := srv.Config()
	pal := cfg.Palettes["default"]
	if pal["sky"] != "#5794F2" {
		t.Errorf("sky = %q, want #5794F2", pal["sky"])
	}
	if _, ok := pal["blue"]; ok {
		t.Error("old name 'blue' should not exist after rename")
	}
}

func TestHandlePaletteColorRenameMissing(t *testing.T) {
	srv, _ := newTestServer(t)

	tests := []struct {
		name string
		form string
	}{
		{"missing palette", "color=blue&new_name=sky"},
		{"missing color", "palette=default&new_name=sky"},
		{"missing new_name", "palette=default&color=blue"},
		{"all empty", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := doPost(t, srv, "/api/palette/color/rename", tt.form)
			assertStatus(t, rec, 200)
			assertContains(t, rec, "missing")
		})
	}
}

func TestHandlePaletteColorRenameDuplicate(t *testing.T) {
	srv, _ := newTestServer(t)

	// Rename blue -> red; red already exists
	rec := doPost(t, srv, "/api/palette/color/rename", "palette=default&color=blue&new_name=red")
	assertStatus(t, rec, 200)
	assertContains(t, rec, "already exists")
}

func TestHandlePaletteCreateSuccess(t *testing.T) {
	srv, _ := newTestServer(t)

	rec := doPost(t, srv, "/api/palette/create", "name=dark")
	assertStatus(t, rec, 200)

	// Verify palette exists in config
	cfg := srv.Config()
	if _, ok := cfg.Palettes["dark"]; !ok {
		t.Error("palette 'dark' should exist after create")
	}
}

func TestHandlePaletteCreateDuplicate(t *testing.T) {
	srv, _ := newTestServer(t)

	rec := doPost(t, srv, "/api/palette/create", "name=default")
	assertStatus(t, rec, 200)
	assertContains(t, rec, "already exists")
}

func TestHandlePaletteActivateSuccess(t *testing.T) {
	srv, _ := newTestServer(t)

	rec := doPost(t, srv, "/api/palette/activate", "name=default")
	assertStatus(t, rec, 200)
}

func TestHandlePaletteActivateNotFound(t *testing.T) {
	srv, _ := newTestServer(t)

	rec := doPost(t, srv, "/api/palette/activate", "name=nonexistent")
	assertStatus(t, rec, 200)
	assertContains(t, rec, "not found")
}


func TestHandlePaletteDeleteSuccess(t *testing.T) {
	srv, _ := newTestServer(t)

	// Create a palette to delete
	rec := doPost(t, srv, "/api/palette/create", "name=extra")
	assertStatus(t, rec, 200)

	// Delete it
	rec = doPost(t, srv, "/api/palette/delete", "name=extra")
	assertStatus(t, rec, 200)

	// Verify it's gone from config
	cfg := srv.Config()
	if _, ok := cfg.Palettes["extra"]; ok {
		t.Error("palette 'extra' should not exist after delete")
	}
}

func TestHandlePaletteDeleteMissing(t *testing.T) {
	srv, _ := newTestServer(t)

	rec := doPost(t, srv, "/api/palette/delete", "")
	assertStatus(t, rec, 200)
	assertContains(t, rec, "missing")
}
