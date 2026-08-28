// Package apitest provides a Go SDK for e2e API testing that builds directly
// on top of `testing.T` / `go test` instead of reimplementing a runner.
//
// It reuses apismith's existing layers:
//
//	env loading    -> environments.Load (config/environments.yaml + env vars)
//	OpenAPI truth  -> openapi.Load / Catalog.Lookup (same as `apismith call`)
//	JWT minting    -> auth/cognito.Generate (SRP/password, cached per Client)
//	HTTP execution -> request.Executor (URL building, header masking, timing)
//
// Usage:
//
//	func TestUserLifecycle(t *testing.T) {
//	    api := apitest.New(t) // APITEST_ENV=staging go test ./e2e -run TestUser -v
//	    var id string
//	    api.POST(t, "/users", apitest.WithBody(map[string]any{"email": "a@b.com"})).
//	        ExpectStatus("201").Capture("$.id", &id)
//	    api.GET(t, "/users/{id}", apitest.WithPath("id", id)).
//	        ExpectStatusCode(200).ExpectJSON("$.email", "a@b.com")
//	}
//
// Typed bodies give compiler safety when you share backend's oapi-codegen structs:
//
//	import backend "framework-backend/api"
//	api.POST(t, "/users", apitest.WithBody(backend.CreateUserRequest{Email: "a@b.com"}))
//
// Go handles orchestration: t.Run, t.Parallel, table-driven tests, -run regex,
// -count, -json output for CI. JWT is minted lazily and reused.
package apitest
