package openapi

import (
	"path/filepath"
	"testing"
)

func TestLookup(t *testing.T) {
	cat, err := Load(filepath.Join("testdata", "fixture.yaml"))
	if err != nil {
		t.Fatal(err)
	}

	ep, params, err := cat.Lookup("createUser")
	if err != nil {
		t.Fatal(err)
	}
	if ep.ID != "POST /users" || len(params) != 0 {
		t.Fatalf("createUser: %+v params=%v", ep, params)
	}

	ep, params, err = cat.Lookup("GET /users/{id}")
	if err != nil {
		t.Fatal(err)
	}
	if ep.OperationID != "getUser" {
		t.Fatalf("template lookup: %+v", ep)
	}

	ep, params, err = cat.LookupMethodPath("GET", "/users/abc-123")
	if err != nil {
		t.Fatal(err)
	}
	if ep.Path != "/users/{id}" || params["id"] != "abc-123" {
		t.Fatalf("concrete path: ep=%+v params=%v", ep, params)
	}

	if _, _, err := cat.Lookup("GET /missing"); err == nil {
		t.Fatal("expected missing operation error")
	}
}

func TestFilter(t *testing.T) {
	cat, err := Load(filepath.Join("testdata", "fixture.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	auth := cat.Filter("auth", "")
	if len(auth) != 1 || auth[0].Path != "/auth/login" {
		t.Fatalf("tag filter: %+v", auth)
	}
	login := cat.Filter("", "login")
	if len(login) != 1 {
		t.Fatalf("search: %+v", login)
	}
}
