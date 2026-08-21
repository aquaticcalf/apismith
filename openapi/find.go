package openapi

import (
	"fmt"
	"strings"
)

// Filter returns endpoints matching an optional tag and free-text search
// (method, path, operationId, summary).
func (c *Catalog) Filter(tag, search string) []Endpoint {
	if c == nil {
		return nil
	}
	tag = strings.TrimSpace(tag)
	search = strings.ToLower(strings.TrimSpace(search))
	out := make([]Endpoint, 0, len(c.Endpoints))
	for _, ep := range c.Endpoints {
		if tag != "" && !hasTag(ep, tag) {
			continue
		}
		if search != "" && !endpointMatchesSearch(ep, search) {
			continue
		}
		out = append(out, ep)
	}
	return out
}

// Lookup finds an operation by operationId or "METHOD PATH".
// Concrete paths such as /users/123 are matched against templated spec
// paths and the extracted path parameters are returned.
func (c *Catalog) Lookup(ref string) (*Endpoint, map[string]string, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil, nil, fmt.Errorf("operation is required")
	}
	if method, path, ok := splitMethodPath(ref); ok {
		return c.LookupMethodPath(method, path)
	}
	return c.lookupOperationID(ref)
}

// LookupMethodPath finds an operation by HTTP method and path (template or concrete).
func (c *Catalog) LookupMethodPath(method, path string) (*Endpoint, map[string]string, error) {
	if c == nil {
		return nil, nil, fmt.Errorf("catalog is nil")
	}
	method = strings.ToUpper(strings.TrimSpace(method))
	path = normalizePath(path)
	if method == "" || path == "" {
		return nil, nil, fmt.Errorf("method and path are required")
	}
	if !validMethod(method) {
		return nil, nil, fmt.Errorf("unknown HTTP method %q", method)
	}

	var exact *Endpoint
	var templated []match
	for i := range c.Endpoints {
		ep := &c.Endpoints[i]
		if ep.Method != method {
			continue
		}
		if normalizePath(ep.Path) == path {
			exact = ep
			break
		}
		if params, ok := matchTemplate(ep.Path, path); ok {
			templated = append(templated, match{ep: ep, params: params})
		}
	}
	if exact != nil {
		return exact, map[string]string{}, nil
	}
	if len(templated) == 1 {
		return templated[0].ep, templated[0].params, nil
	}
	if len(templated) > 1 {
		return nil, nil, fmt.Errorf("ambiguous path %s %s; matches %s", method, path, joinIDs(templated))
	}
	return nil, nil, fmt.Errorf("operation not in OpenAPI spec: %s %s", method, path)
}

type match struct {
	ep     *Endpoint
	params map[string]string
}

func (c *Catalog) lookupOperationID(id string) (*Endpoint, map[string]string, error) {
	want := strings.ToLower(id)
	var found []*Endpoint
	for i := range c.Endpoints {
		ep := &c.Endpoints[i]
		if strings.ToLower(ep.OperationID) == want {
			found = append(found, ep)
		}
	}
	switch len(found) {
	case 1:
		return found[0], map[string]string{}, nil
	case 0:
		return nil, nil, fmt.Errorf("operation not in OpenAPI spec: %s (use METHOD PATH or operationId)", id)
	default:
		return nil, nil, fmt.Errorf("ambiguous operationId %q", id)
	}
}

func splitMethodPath(ref string) (method, path string, ok bool) {
	parts := strings.Fields(ref)
	if len(parts) < 2 || !validMethod(parts[0]) {
		return "", "", false
	}
	return strings.ToUpper(parts[0]), strings.Join(parts[1:], " "), true
}

func validMethod(m string) bool {
	switch strings.ToUpper(m) {
	case "GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS":
		return true
	default:
		return false
	}
}

func normalizePath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return p
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	if len(p) > 1 {
		p = strings.TrimRight(p, "/")
	}
	return p
}

func matchTemplate(template, actual string) (map[string]string, bool) {
	tParts := splitPath(template)
	aParts := splitPath(actual)
	if len(tParts) != len(aParts) {
		return nil, false
	}
	params := map[string]string{}
	hasParam := false
	for i := range tParts {
		t, a := tParts[i], aParts[i]
		if name, ok := templateParam(t); ok {
			params[name] = a
			hasParam = true
			continue
		}
		if t != a {
			return nil, false
		}
	}
	if !hasParam {
		return nil, false
	}
	return params, true
}

func splitPath(p string) []string {
	p = strings.Trim(normalizePath(p), "/")
	if p == "" {
		return nil
	}
	return strings.Split(p, "/")
}

func templateParam(seg string) (string, bool) {
	if strings.HasPrefix(seg, "{") && strings.HasSuffix(seg, "}") && len(seg) > 2 {
		return seg[1 : len(seg)-1], true
	}
	return "", false
}

func hasTag(ep Endpoint, tag string) bool {
	want := strings.ToLower(tag)
	for _, t := range ep.Tags {
		if strings.ToLower(t) == want {
			return true
		}
	}
	return false
}

func endpointMatchesSearch(ep Endpoint, search string) bool {
	haystacks := []string{
		strings.ToLower(ep.Method),
		strings.ToLower(ep.Path),
		strings.ToLower(ep.OperationID),
		strings.ToLower(ep.Summary),
		strings.ToLower(ep.ID),
	}
	for _, h := range haystacks {
		if strings.Contains(h, search) {
			return true
		}
	}
	for _, t := range ep.Tags {
		if strings.Contains(strings.ToLower(t), search) {
			return true
		}
	}
	return false
}

func joinIDs(matches []match) string {
	ids := make([]string, 0, len(matches))
	for _, m := range matches {
		ids = append(ids, m.ep.ID)
	}
	return strings.Join(ids, ", ")
}
