package server

import "testing"

func TestVariableAdd(t *testing.T) {
	srv, _ := newTestServer(t)

	t.Run("success", func(t *testing.T) {
		w := doPost(t, srv, "/api/variable/add",
			"name=test_var&type=query&datasource=primary&query=label_values(up)&multi=true&include_all=true")
		assertStatus(t, w, 200)
		assertContains(t, w, "added")
	})

	t.Run("empty name", func(t *testing.T) {
		w := doPost(t, srv, "/api/variable/add", "name=&type=query")
		assertStatus(t, w, 200)
		assertContains(t, w, "required")
	})

	t.Run("invalid name", func(t *testing.T) {
		w := doPost(t, srv, "/api/variable/add", "name=123bad&type=query")
		assertStatus(t, w, 200)
		assertContains(t, w, "invalid")
	})

	t.Run("GET not allowed", func(t *testing.T) {
		w := doGet(t, srv, "/api/variable/add")
		assertStatus(t, w, 405)
	})
}

func TestVariableDelete(t *testing.T) {
	srv, _ := newTestServer(t)

	t.Run("delete existing", func(t *testing.T) {
		// Add a variable first, then delete it.
		w := doPost(t, srv, "/api/variable/add", "name=to_delete&type=custom&values=a,b,c")
		assertStatus(t, w, 200)
		assertContains(t, w, "added")

		w = doPost(t, srv, "/api/variable/delete", "name=to_delete")
		assertStatus(t, w, 200)
		assertContains(t, w, "deleted")
	})

	t.Run("missing name", func(t *testing.T) {
		w := doPost(t, srv, "/api/variable/delete", "name=")
		assertStatus(t, w, 200)
		assertContains(t, w, "required")
	})
}

func TestVariablesPage(t *testing.T) {
	srv, _ := newTestServer(t)

	w := doGet(t, srv, "/variables")
	assertStatus(t, w, 200)
	assertContains(t, w, "instance")
}


func TestHandleVariableSnippet(t *testing.T) {
	srv, _ := newTestServer(t)

	rec := doPost(t, srv, "/api/datasources/variable-snippet", "datasource=primary&labels=job&labels=instance")
	assertStatus(t, rec, 200)
	assertContains(t, rec, "variables:")
	assertContains(t, rec, "label_values(job)")
	assertContains(t, rec, "label_values(instance)")
	assertContains(t, rec, "datasource: primary")
}

func TestHandleVariableSnippetNoLabels(t *testing.T) {
	srv, _ := newTestServer(t)

	rec := doPost(t, srv, "/api/datasources/variable-snippet", "datasource=primary")
	assertStatus(t, rec, 200)
	assertContains(t, rec, "select at least one")
}
