package apitest

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"aegion-dynamic/apismith/auth/cognito"
	"aegion-dynamic/apismith/environments"
	"aegion-dynamic/apismith/openapi"
	"aegion-dynamic/apismith/request"
)

// Client is the e2e test client. Zero yaml dependency - single env from env vars.
// Base URL and Cognito creds come from env vars, JWT minted lazily once.
//
// Required env vars (single env):
//   APITEST_BASE_URL  e.g. http://localhost:8080/api/v1  (fallback: CONSOLE_BASE_URL, API_BASE_URL)
//   COGNITO_USER_POOL_ID, COGNITO_CLIENT_ID, COGNITO_USERNAME, COGNITO_PASSWORD
//   Optional: COGNITO_CLIENT_SECRET, COGNITO_REGION (default ap-south-1), COGNITO_ENDPOINT, APITEST_JWT (bypass)
//   Optional: APITEST_OPENAPI_SPEC - if set, validates METHOD PATH against spec; if unset, no validation.
//
// Usage: api := apitest.New(t)
//        api.POST(t, "/users", apitest.WithBody(...)).ExpectStatus("201")
type Client struct {
	tb      testing.TB
	cfg     *environments.Config // nil when env-only
	catalog *openapi.Catalog     // nil when no spec
	exec    *request.Executor
	env     *environments.Environment // single env, id="apitest"
	envID   string
	jwt     string
	jwtErr  error
	didAuth bool
}

// Option configures Client creation.
type Option func(*clientConfig)

type clientConfig struct {
	baseURL  string // override for tests (httptest)
	jwt      string // bypass Cognito for tests
	executor *request.Executor
	noAuth   bool
}

// WithBaseURL overrides base URL (useful with httptest.NewServer).
func WithBaseURL(url string) Option { return func(c *clientConfig) { c.baseURL = url } }

// WithJWT bypasses Cognito and uses this token.
func WithJWT(tok string) Option {
	return func(c *clientConfig) {
		c.jwt = tok
		c.noAuth = false
	}
}

// WithClientNoAuth disables JWT injection client-wide.
func WithClientNoAuth() Option { return func(c *clientConfig) { c.noAuth = true } }

// WithExecutor injects a custom http client (e.g. for mocking).
func WithExecutor(e *request.Executor) Option { return func(c *clientConfig) { c.executor = e } }

// New creates a Client bound to the caller's testing.TB. No yaml required.
// Env vars are the single source of truth; APITEST_BASE_URL selects target.
func New(tb testing.TB, opts ...Option) *Client {
	tb.Helper()
	cc := &clientConfig{}
	for _, o := range opts {
		o(cc)
	}
	cfg, catalog, env := loadEnv(tb)
	if cc.baseURL != "" && env != nil {
		cp := *env
		cp.BaseURL = cc.baseURL
		env = &cp
	}
	exec := cc.executor
	if exec == nil {
		exec = request.NewExecutor()
	}
	c := &Client{
		tb:      tb,
		cfg:     cfg,
		catalog: catalog,
		exec:    exec,
		env:     env,
		envID:   env.ID,
	}
	if cc.noAuth {
		c.didAuth = true
		c.jwt = ""
	} else if cc.jwt != "" {
		c.jwt = cc.jwt
		c.didAuth = true
	}
	return c
}

func loadEnv(tb testing.TB) (*environments.Config, *openapi.Catalog, *environments.Environment) {
	tb.Helper()
	// Prefer explicit yaml only if caller set it - otherwise env-only (no .dev/.staging)
	if p := strings.TrimSpace(os.Getenv("APITEST_CONFIG")); p != "" {
		if cfg, cat, env := tryLoadYAML(tb, p); cfg != nil {
			return cfg, cat, env
		}
	}
	if p := strings.TrimSpace(os.Getenv("CONSOLE_CONFIG")); p != "" {
		if cfg, cat, env := tryLoadYAML(tb, p); cfg != nil {
			return cfg, cat, env
		}
	}
	// Env-only single env
	baseURL := firstNonEmpty(
		os.Getenv("APITEST_BASE_URL"),
		os.Getenv("API_BASE_URL"),
		os.Getenv("CONSOLE_BASE_URL"),
		os.Getenv("BASE_URL"),
	)
	if baseURL == "" {
		// fallback for local dev without env set - keep tests runnable with httptest override
		baseURL = "http://localhost:8080/api/v1"
		tb.Logf("apitest: APITEST_BASE_URL not set, defaulting to %s (set APITEST_BASE_URL to override)", baseURL)
	}
	env := &environments.Environment{
		ID:      "apitest",
		Name:    "apitest",
		BaseURL: baseURL,
		Cognito: environments.CognitoConfig{
			Region:     firstNonEmpty(os.Getenv("COGNITO_REGION"), os.Getenv("AWS_REGION"), "ap-south-1"),
			UserPoolID: os.Getenv("COGNITO_USER_POOL_ID"),
			ClientID:   os.Getenv("COGNITO_CLIENT_ID"),
			Endpoint:   os.Getenv("COGNITO_ENDPOINT"),
			AuthFlow:   firstNonEmpty(os.Getenv("COGNITO_AUTH_FLOW"), os.Getenv("CONSOLE_AUTH_FLOW"), "srp"),
		},
	}
	creds := environments.Credentials{
		Username:     firstNonEmpty(os.Getenv("COGNITO_USERNAME"), os.Getenv("CONSOLE_USERNAME")),
		Password:     firstNonEmpty(os.Getenv("COGNITO_PASSWORD"), os.Getenv("CONSOLE_PASSWORD")),
		ClientSecret: os.Getenv("COGNITO_CLIENT_SECRET"),
	}
	cfg := &environments.Config{
		DefaultEnvironment: "apitest",
		Environments:       []environments.Environment{*env},
		Credentials:        map[string]environments.Credentials{"": creds, "apitest": creds},
		OpenAPISpec:        strings.TrimSpace(os.Getenv("APITEST_OPENAPI_SPEC")),
	}
	// Also respect CONSOLE_OPENAPI_SPEC
	if cfg.OpenAPISpec == "" {
		cfg.OpenAPISpec = strings.TrimSpace(os.Getenv("CONSOLE_OPENAPI_SPEC"))
	}
	// Try to load catalog if spec path provided or default file exists
	var catalog *openapi.Catalog
	specCandidates := []string{
		cfg.OpenAPISpec,
		"openapi/openapi.yaml",
		"../openapi/openapi.yaml",
		"../framework-backend/nimbus_openapi_spec.yaml",
	}
	for _, p := range specCandidates {
		if p == "" {
			continue
		}
		if _, err := os.Stat(p); err == nil {
			if cat, err := openapi.Load(p); err == nil {
				catalog = cat
				break
			}
		}
	}
	// catalog remains nil if no spec found -> no validation, direct passthrough
	return cfg, catalog, env
}

func tryLoadYAML(tb testing.TB, path string) (*environments.Config, *openapi.Catalog, *environments.Environment) {
	tb.Helper()
	if _, err := os.Stat(path); err != nil {
		tb.Logf("apitest: config %s not found, using env vars", path)
		return nil, nil, nil
	}
	cfg, err := environments.Load(path)
	if err != nil {
		tb.Logf("apitest: load %s failed: %v, using env vars", path, err)
		return nil, nil, nil
	}
	envID := cfg.DefaultEnvironment
	env := cfg.Find(envID)
	if env == nil && len(cfg.Environments) > 0 {
		env = &cfg.Environments[0]
		envID = env.ID
	}
	var catalog *openapi.Catalog
	if cfg.OpenAPISpec != "" {
		catalog, _ = openapi.Load(cfg.OpenAPISpec)
	}
	return cfg, catalog, env
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// EnvID returns the single env id ("apitest").
func (c *Client) EnvID() string { return c.envID }

// BaseURL returns the target base URL.
func (c *Client) BaseURL() string {
	if c.env != nil {
		return c.env.BaseURL
	}
	return ""
}

// ensureJWT fetches Cognito token if needed.
func (c *Client) ensureJWT() string {
	if c.didAuth {
		return c.jwt
	}
	c.didAuth = true
	if tok := strings.TrimSpace(os.Getenv("APITEST_JWT")); tok != "" {
		c.jwt = tok
		return c.jwt
	}
	if c.cfg == nil || c.env == nil {
		return ""
	}
	cc, err := c.cfg.CognitoConfigFor(c.envID, "", "", "")
	if err != nil {
		c.jwtErr = err
		return ""
	}
	if cc.UserPoolID == "" || cc.ClientID == "" || cc.Username == "" || cc.Password == "" {
		return ""
	}
	tokens, err := cognito.Generate(context.Background(), cc)
	if err != nil {
		c.jwtErr = err
		return ""
	}
	c.jwt = tokens.AccessToken
	return c.jwt
}

// Do executes a request. If OpenAPI spec loaded, validates METHOD PATH; otherwise passes through.
func (c *Client) Do(tb testing.TB, method, path string, opts ...CallOption) *Response {
	tb.Helper()
	co := &callConfig{
		Headers: map[string]string{},
		Path:    map[string]string{},
		Query:   map[string]string{},
	}
	for _, o := range opts {
		o(co)
	}
	var ep *openapi.Endpoint
	var extracted map[string]string
	if c.catalog != nil {
		var err error
		ep, extracted, err = c.catalog.LookupMethodPath(method, path)
		if err != nil {
			if !strings.Contains(path, "/") && strings.TrimSpace(method) != "" && len(opts) == 0 {
				ep2, _, err2 := c.catalog.Lookup(method)
				if err2 == nil {
					ep = ep2
					extracted = map[string]string{}
					err = nil
				}
			}
			if err != nil {
				tb.Fatalf("apitest: %v (env=%s)", err, c.envID)
			}
		}
		for k, v := range extracted {
			if _, ok := co.Path[k]; !ok {
				co.Path[k] = v
			}
		}
	} else {
		// no catalog - use method/path as-is
		ep = &openapi.Endpoint{Method: strings.ToUpper(method), Path: path, AuthRequired: true}
		extracted = map[string]string{}
		// if path contains concrete segments like /users/123, try to leave as-is; executor will use exact path
	}
	bodyStr := ""
	if co.Body != nil {
		switch v := co.Body.(type) {
		case string:
			bodyStr = v
		case []byte:
			bodyStr = string(v)
		default:
			b, err := json.Marshal(v)
			if err != nil {
				tb.Fatalf("apitest: marshal body: %v", err)
			}
			bodyStr = string(b)
		}
	} else if co.BodyFile != "" {
		b, err := os.ReadFile(co.BodyFile)
		if err != nil {
			tb.Fatalf("apitest: read bodyFile %s: %v", co.BodyFile, err)
		}
		bodyStr = string(b)
	}
	if co.Path == nil {
		co.Path = map[string]string{}
	}
	if co.Query == nil {
		co.Query = map[string]string{}
	}
	if co.Headers == nil {
		co.Headers = map[string]string{}
	}
	authMode := request.AuthJWT
	jwt := ""
	if co.NoAuth || (ep != nil && !ep.AuthRequired) {
		authMode = request.AuthNone
	} else {
		jwt = c.ensureJWT()
		if c.jwtErr != nil {
			tb.Fatalf("apitest: jwt mint failed: %v", c.jwtErr)
		}
	}
	in := request.ExecuteInput{
		Environment: c.envID,
		Method:      ep.Method,
		Path:        ep.Path,
		PathParams:  co.Path,
		Query:       co.Query,
		Headers:     co.Headers,
		Body:        bodyStr,
		AuthMode:    authMode,
		JWT:         jwt,
	}
	out := c.exec.Execute(in, c.env.BaseURL, c.env.Production)
	resp := &Response{tb: tb, in: in, ep: ep, out: out, client: c}
	if strings.Contains(strings.ToLower(out.ContentType), "json") {
		resp.prettyBody = request.PrettyJSON(out.Body)
	} else {
		resp.prettyBody = out.Body
	}
	return resp
}

// Convenience verbs
func (c *Client) GET(tb testing.TB, path string, opts ...CallOption) *Response {
	tb.Helper()
	return c.Do(tb, "GET", path, opts...)
}
func (c *Client) POST(tb testing.TB, path string, opts ...CallOption) *Response {
	tb.Helper()
	return c.Do(tb, "POST", path, opts...)
}
func (c *Client) PUT(tb testing.TB, path string, opts ...CallOption) *Response {
	tb.Helper()
	return c.Do(tb, "PUT", path, opts...)
}
func (c *Client) PATCH(tb testing.TB, path string, opts ...CallOption) *Response {
	tb.Helper()
	return c.Do(tb, "PATCH", path, opts...)
}
func (c *Client) DELETE(tb testing.TB, path string, opts ...CallOption) *Response {
	tb.Helper()
	return c.Do(tb, "DELETE", path, opts...)
}

// CallByID uses operationId (requires catalog; fails if no spec).
func (c *Client) CallByID(tb testing.TB, operationID string, opts ...CallOption) *Response {
	tb.Helper()
	if c.catalog == nil {
		tb.Fatalf("apitest: CallByID requires APITEST_OPENAPI_SPEC to be set")
	}
	ep, _, err := c.catalog.Lookup(operationID)
	if err != nil {
		tb.Fatalf("apitest: %v", err)
	}
	return c.Do(tb, ep.Method, ep.Path, opts...)
}
