package apitest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"aegion-dynamic/apismith/request"
)

func writeTempConfig(t *testing.T, baseURL string) string {
	t.Helper()
	dir := t.TempDir()
	spec := filepath.Join(dir, "openapi.yaml")
	specBody := `openapi: 3.0.3
info: {title: t, version: '1'}
servers: [{url: ` + baseURL + `}]
security: [{bearerAuth: []}]
paths:
  /users:
    post:
      operationId: createUser
      security: [{bearerAuth: []}]
      requestBody: {content: {application/json: {schema: {type: object}}}}
      responses: {'201': {description: created}}
    get:
      operationId: listUsers
      responses: {'200': {description: ok}}
  /users/{id}:
    get:
      operationId: getUser
      parameters: [{name: id, in: path, required: true, schema: {type: string}}]
      responses: {'200': {description: ok}}
  /auth/login:
    post:
      operationId: login
      security: []
      responses: {'200': {description: ok}}
components:
  securitySchemes: {bearerAuth: {type: http, scheme: bearer}}
`
	if err := os.WriteFile(spec, []byte(specBody), 0o644); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(dir, "environments.yaml")
	cfgBody := "openapi_spec: " + spec + "\ndefault_environment: dev\nenvironments:\n  - id: dev\n    name: DEV\n    base_url: " + baseURL + "\n    production: false\n    cognito: {region: ap-south-1}\n"
	if err := os.WriteFile(cfgPath, []byte(cfgBody), 0o644); err != nil {
		t.Fatal(err)
	}
	return cfgPath
}

func TestClientE2E(t *testing.T) {
	// fake backend
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/users":
			if r.Method == "POST" {
				var body map[string]any
				_ = json.NewDecoder(r.Body).Decode(&body)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(201)
				_ = json.NewEncoder(w).Encode(map[string]any{"id": "u-123", "email": body["email"]})
				return
			}
		case "/users/u-123":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "u-123", "email": "a@b.com"})
			return
		case "/auth/login":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"token": "ok"})
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	cfgPath := writeTempConfig(t, srv.URL)

	// bypass real Cognito with a fake JWT - SDK should attach it for auth'd routes
	client := New(t, WithConfigPath(cfgPath), WithJWT("fake-jwt"), WithBaseURL(srv.URL))

	t.Run("create", func(t *testing.T) {
		var id string
		client.POST(t, "/users", WithBody(map[string]any{"email": "a@b.com"})).
			ExpectStatus("201").
			ExpectJSON("$.email", "a@b.com").
			Capture("$.id", &id)
		if id != "u-123" {
			t.Fatalf("capture got %q want u-123", id)
		}
		// stash via closure for next subtest
		t.Logf("captured id=%s", id)
	})

	t.Run("get", func(t *testing.T) {
		client.GET(t, "/users/{id}", WithPath("id", "u-123")).
			ExpectStatusCode(200).
			ExpectJSON("$.id", "u-123")
	})

	t.Run("public_no_auth", func(t *testing.T) {
		// /auth/login is public per spec (security: []), SDK auto skips JWT
		client.POST(t, "/auth/login", WithBody(map[string]any{"email": "x"}), WithRequestNoAuth()).
			ExpectStatusCode(200).
			ExpectBodyContains("token")
	})

	t.Run("concrete_path_extract", func(t *testing.T) {
		// concrete path like /users/u-123 auto-extracts {id}
		client.GET(t, "/users/u-123").ExpectStatusCode(200)
	})

	t.Run("operationId", func(t *testing.T) {
		client.CallByID(t, "createUser", WithBody(map[string]any{"email": "b@b.com"})).
			ExpectStatus("201")
	})
}

func TestClientTypesafetyBodyStruct(t *testing.T) {
	type CreateReq struct {
		Email string `json:"email"`
		Name  string `json:"name"`
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var got CreateReq
		_ = json.NewDecoder(r.Body).Decode(&got)
		if got.Email != "typed@example.com" {
			t.Errorf("email %q", got.Email)
		}
		w.WriteHeader(201)
		_, _ = w.Write([]byte(`{"id":"1"}`))
	}))
	defer srv.Close()
	cfgPath := writeTempConfig(t, srv.URL)
	client := New(t, WithConfigPath(cfgPath), WithJWT("tok"), WithBaseURL(srv.URL))
	client.POST(t, "/users", WithBody(CreateReq{Email: "typed@example.com", Name: "Ada"})).
		ExpectStatusCode(201)
}

func TestResponseJSONPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"user":{"emails":["a@b.com","b@b.com"]},"count":2}`))
	}))
	defer srv.Close()
	cfgPath := writeTempConfig(t, srv.URL)
	client := New(t, WithConfigPath(cfgPath), WithJWT("tok"), WithBaseURL(srv.URL))
	resp := client.GET(t, "/users/u-123", WithPath("id", "u-123"))
	// hijack body for this unit test - directly test getPath
	resp.Raw() // ensure not unused
	if got := resp.JSON("$.user.emails[0]"); got != "a@b.com" {
		t.Fatalf("got %v", got)
	}
	if got := resp.JSON("count"); got.(float64) != 2 {
		t.Fatalf("got %v", got)
	}
	// use executor directly for this assertion
	exec := request.NewExecutor()
	_ = exec
}
