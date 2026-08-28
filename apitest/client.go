package apitest

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"aegion-dynamic/api-console/auth/cognito"
	"aegion-dynamic/api-console/environments"
	"aegion-dynamic/api-console/openapi"
	"aegion-dynamic/api-console/request"
)

// Client is the e2e test client. It wraps OpenAPI validation, environment
// routing, JWT minting, and HTTP execution so tests read like go test:
//
//	client := apitest.New(t)
//	var id string
//	client.POST(t, "/users", apitest.WithBody(map[string]any{"email": "a@b.com"})).
//		ExpectStatus(201).Capture("$.id", &id)
//
// JWT is minted lazily once per Client and reused. Go handles orchestration
// via t.Run / t.Parallel - this package does not reimplement a test runner.
type Client struct {
	tb      testing.TB
	cfg     *environments.Config
	catalog *openapi.Catalog
	exec    *request.Executor
	env     *environments.Environment
	envID   string
	jwt     string
	jwtErr  error
	didAuth bool
}

// Option configures Client creation.
type Option func(*clientConfig)

type clientConfig struct {
	envID      string
	configPath string
	baseURL    string // override for tests (httptest)
	jwt        string // bypass Cognito for tests
	executor   *request.Executor
	noAuth     bool
}

// WithEnv selects environment id (dev/staging). Also respects APITEST_ENV / CONSOLE_DEFAULT_ENV.
func WithEnv(id string) Option { return func(c *clientConfig) { c.envID = id } }

// WithConfigPath overrides environments.yaml path.
func WithConfigPath(p string) Option { return func(c *clientConfig) { c.configPath = p } }

// WithBaseURL overrides env BaseURL (useful with httptest.NewServer).
func WithBaseURL(url string) Option { return func(c *clientConfig) { c.baseURL = url } }

// WithJWT bypasses Cognito and uses this token (or "" for no auth).
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

// New creates a Client bound to the caller's testing.TB. It loads
// config/environments.yaml + OpenAPI spec exactly like `apismith call`.
// JWT is not fetched until the first authenticated request.
func New(tb testing.TB, opts ...Option) *Client {
	tb.Helper()
	cc := &clientConfig{}
	for _, o := range opts {
		o(cc)
	}
	// resolve env id: explicit > APITEST_ENV > CONSOLE_DEFAULT_ENV
	if cc.envID == "" {
		cc.envID = strings.TrimSpace(os.Getenv("APITEST_ENV"))
	}
	cfgPath := cc.configPath
	if cfgPath == "" {
		cfgPath = strings.TrimSpace(os.Getenv("APITEST_CONFIG"))
		if cfgPath == "" {
			cfgPath = strings.TrimSpace(os.Getenv("CONSOLE_CONFIG"))
		}
	}
	cfg, catalog, envID, env := loadRuntime(tb, cfgPath, cc.envID)
	if cc.baseURL != "" && env != nil {
		// copy env so we don't mutate shared config
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
		envID:   envID,
	}
	if cc.noAuth {
		c.didAuth = true
		c.jwt = ""
	} else if cc.jwt != "" {
		c.jwt = cc.jwt
		c.didAuth = true
	}
	// if WithJWT("") was not used, jwt remains lazy
	return c
}

func loadRuntime(tb testing.TB, configPath, envID string) (*environments.Config, *openapi.Catalog, string, *environments.Environment) {
	tb.Helper()
	if configPath == "" {
		configPath = "config/environments.yaml"
		// fallback: when running `go test ./...` from apitest/ dir, try parent
		if _, err := os.Stat(configPath); err != nil {
			if _, err2 := os.Stat("../config/environments.yaml"); err2 == nil {
				configPath = "../config/environments.yaml"
			} else if _, err3 := os.Stat("../../config/environments.yaml"); err3 == nil {
				configPath = "../../config/environments.yaml"
			}
		}
	}
	cfg, err := environments.Load(configPath)
	if err != nil {
		tb.Fatalf("apitest: load config %s: %v", configPath, err)
	}
	if envID == "" {
		envID = strings.TrimSpace(os.Getenv("APITEST_ENV"))
	}
	if envID == "" {
		envID = cfg.DefaultEnvironment
	}
	env := cfg.Find(envID)
	if env == nil {
		tb.Fatalf("apitest: unknown environment %q (have %v)", envID, envIDs(cfg))
	}
	catalog, err := openapi.Load(cfg.OpenAPISpec)
	if err != nil {
		tb.Fatalf("apitest: load OpenAPI %s: %v", cfg.OpenAPISpec, err)
	}
	return cfg, catalog, envID, env
}

func envIDs(cfg *environments.Config) []string {
	out := make([]string, 0, len(cfg.Environments))
	for _, e := range cfg.Environments {
		out = append(out, e.ID)
	}
	return out
}

// EnvID returns the selected environment id.
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
	// allow env var bypass for CI that mints externally
	if tok := strings.TrimSpace(os.Getenv("APITEST_JWT")); tok != "" {
		c.jwt = tok
		return c.jwt
	}
	if c.cfg == nil || c.env == nil {
		return ""
	}
	// check if any credential is missing - let caller decide no-auth for public endpoints
	cc, err := c.cfg.CognitoConfigFor(c.envID, "", "", "")
	if err != nil {
		c.jwtErr = err
		return ""
	}
	// skip if spec says public? caller handles via WithNoAuth; we always try if credentials look valid
	if cc.UserPoolID == "" || cc.ClientID == "" || cc.Username == "" || cc.Password == "" {
		// no credentials - leave jwt empty, public endpoints will still work
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

// Do executes an operation. Method/path are validated against the OpenAPI
// catalog exactly like `apismith call` - unknown ops fail the test immediately.
// Use CallOptions to set body/query/path params. Returns *Response for chaining.
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
	// validate against spec (like cli/call.go:resolveCall)
	ep, extracted, err := c.catalog.LookupMethodPath(method, path)
	if err != nil {
		// also try operationId lookup if method looks like an op id (no slash)
		if !strings.Contains(path, "/") && strings.TrimSpace(method) != "" && len(opts) == 0 {
			// fallback: single arg as operationId
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
	// merge extracted path params (from concrete path like /users/123)
	for k, v := range extracted {
		if _, ok := co.Path[k]; !ok {
			co.Path[k] = v
		}
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
	// headers/path/query: ensure non-nil for executor
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
	if co.NoAuth || !ep.AuthRequired {
		authMode = request.AuthNone
	} else {
		jwt = c.ensureJWT()
		if c.jwtErr != nil {
			tb.Fatalf("apitest: jwt mint failed (env=%s): %v", c.envID, c.jwtErr)
		}
	}
	in := request.ExecuteInput{
		Environment: c.envID,
		Method:      ep.Method,
		Path:        ep.Path, // use templated path from spec, executor does substitution
		PathParams:  co.Path,
		Query:       co.Query,
		Headers:     co.Headers,
		Body:        bodyStr,
		AuthMode:    authMode,
		JWT:         jwt,
	}
	out := c.exec.Execute(in, c.env.BaseURL, c.env.Production)
	resp := &Response{tb: tb, in: in, ep: ep, out: out, client: c}
	// pretty json like cli/call.go:106
	if strings.Contains(strings.ToLower(out.ContentType), "json") {
		resp.prettyBody = request.PrettyJSON(out.Body)
	} else {
		resp.prettyBody = out.Body
	}
	return resp
}

// Convenience verbs - each validates against spec and is typesafe at call-site.
// Body is any Go value (struct/map) marshalled to JSON, so refactoring is compiler-checked.

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

// CallByID uses operationId instead of METHOD PATH.
func (c *Client) CallByID(tb testing.TB, operationID string, opts ...CallOption) *Response {
	tb.Helper()
	ep, _, err := c.catalog.Lookup(operationID)
	if err != nil {
		tb.Fatalf("apitest: %v", err)
	}
	return c.Do(tb, ep.Method, ep.Path, opts...)
}
