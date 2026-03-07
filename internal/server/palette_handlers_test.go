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
