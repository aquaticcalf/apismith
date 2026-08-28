// Package apitest provides e2e API testing on top of `testing.T` with zero yaml dependency.
// Single env from env vars only (no .dev/.staging, no environments.yaml).
//
// Env vars (single target):
//   APITEST_BASE_URL  e.g. http://localhost:8080/api/v1  (also API_BASE_URL, CONSOLE_BASE_URL)
//   COGNITO_USER_POOL_ID, COGNITO_CLIENT_ID, COGNITO_USERNAME, COGNITO_PASSWORD
//   Optional: COGNITO_CLIENT_SECRET, COGNITO_REGION/AWS_REGION, COGNITO_ENDPOINT, APITEST_JWT (bypass)
//   Optional: APITEST_OPENAPI_SPEC - if set, validates METHOD PATH; if unset, no validation (passthrough).
//
// Usage:
//
//	func TestUserLifecycle(t *testing.T) {
//	    api := apitest.New(t) // reads env vars, mints JWT lazily
//	    var id string
//	    api.POST(t, "/users", apitest.WithBody(map[string]any{"email":"a@b.com"})).ExpectStatus("201").Capture("$.id",&id)
//	    api.GET(t, "/users/{id}", apitest.WithPath("id", id)).ExpectStatusCode(200)
//	}
package apitest
