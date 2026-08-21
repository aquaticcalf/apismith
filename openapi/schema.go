package openapi

import (
	"encoding/json"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
)

// SchemaView is a JSON-serialisable, frontend-friendly slice of an OpenAPI schema.
type SchemaView struct {
	Type        string                 `json:"type,omitempty"`
	Format      string                 `json:"format,omitempty"`
	Description string                 `json:"description,omitempty"`
	Example     any                    `json:"example,omitempty"`
	Enum        []any                  `json:"enum,omitempty"`
	Required    []string               `json:"required,omitempty"`
	Properties  map[string]*SchemaView `json:"properties,omitempty"`
	Items       *SchemaView            `json:"items,omitempty"`
	Ref         string                 `json:"ref,omitempty"`
	Nullable    bool                   `json:"nullable,omitempty"`
}

func viewFromRef(ref *openapi3.SchemaRef, visiting map[*openapi3.Schema]bool) *SchemaView {
	if ref == nil {
		return nil
	}
	view := &SchemaView{Ref: ref.Ref}
	if ref.Value == nil {
		return view
	}
	return viewFromSchema(ref.Value, visiting, view)
}

func viewFromSchema(s *openapi3.Schema, visiting map[*openapi3.Schema]bool, into *SchemaView) *SchemaView {
	if s == nil {
		return into
	}
	if visiting == nil {
		visiting = map[*openapi3.Schema]bool{}
	}
	if visiting[s] {
		if into == nil {
			into = &SchemaView{}
		}
		into.Type = "object"
		into.Description = "(recursive)"
		return into
	}
	visiting[s] = true
	defer delete(visiting, s)

	if into == nil {
		into = &SchemaView{}
	}

	// allOf: merge properties from each constituent.
	if len(s.AllOf) > 0 {
		return viewFromSchema(mergeAllOf(s), visiting, into)
	}
	if len(s.OneOf) > 0 {
		return viewFromRef(s.OneOf[0], visiting)
	}
	if len(s.AnyOf) > 0 {
		return viewFromRef(s.AnyOf[0], visiting)
	}

	into.Type = schemaType(s)
	into.Format = s.Format
	into.Description = s.Description
	into.Example = s.Example
	into.Nullable = s.Nullable
	into.Required = append([]string{}, s.Required...)
	if len(s.Enum) > 0 {
		into.Enum = append([]any{}, s.Enum...)
	}
	if s.Items != nil {
		into.Items = viewFromRef(s.Items, visiting)
	}
	if len(s.Properties) > 0 {
		into.Properties = make(map[string]*SchemaView, len(s.Properties))
		for name, prop := range s.Properties {
			into.Properties[name] = viewFromRef(prop, visiting)
		}
		if into.Type == "" {
			into.Type = "object"
		}
	}
	return into
}

func mergeAllOf(s *openapi3.Schema) *openapi3.Schema {
	out := &openapi3.Schema{
		Type:       s.Type,
		Format:     s.Format,
		Required:   append([]string{}, s.Required...),
		Properties: openapi3.Schemas{},
		Example:    s.Example,
	}
	for name, prop := range s.Properties {
		out.Properties[name] = prop
	}
	for _, part := range s.AllOf {
		if part == nil || part.Value == nil {
			continue
		}
		p := part.Value
		if out.Type == nil && p.Type != nil {
			out.Type = p.Type
		}
		if out.Format == "" {
			out.Format = p.Format
		}
		if out.Example == nil {
			out.Example = p.Example
		}
		out.Required = append(out.Required, p.Required...)
		for name, prop := range p.Properties {
			out.Properties[name] = prop
		}
	}
	return out
}

func schemaType(s *openapi3.Schema) string {
	if s == nil {
		return ""
	}
	if s.Type == nil {
		if len(s.Properties) > 0 {
			return "object"
		}
		return ""
	}
	types := []string(*s.Type)
	for _, t := range types {
		if t != "null" {
			return t
		}
	}
	if len(types) > 0 {
		return types[0]
	}
	return ""
}

func exampleFromSchemaView(s *SchemaView) any {
	if s == nil {
		return nil
	}
	if s.Example != nil {
		return s.Example
	}
	switch s.Type {
	case "object", "":
		if len(s.Properties) == 0 {
			if s.Type == "object" {
				return map[string]any{}
			}
			return nil
		}
		obj := map[string]any{}
		for name, prop := range s.Properties {
			if v := exampleFromSchemaView(prop); v != nil {
				obj[name] = v
			} else {
				obj[name] = defaultFor(prop)
			}
		}
		return obj
	case "array":
		item := exampleFromSchemaView(s.Items)
		if item == nil {
			item = defaultFor(s.Items)
		}
		return []any{item}
	case "string":
		if len(s.Enum) > 0 {
			return s.Enum[0]
		}
		return defaultString(s.Format)
	case "integer", "number":
		if len(s.Enum) > 0 {
			return s.Enum[0]
		}
		return 0
	case "boolean":
		return false
	default:
		return nil
	}
}

func defaultFor(s *SchemaView) any {
	if s == nil {
		return nil
	}
	if s.Example != nil {
		return s.Example
	}
	switch s.Type {
	case "string":
		return defaultString(s.Format)
	case "integer", "number":
		return 0
	case "boolean":
		return false
	case "array":
		return []any{}
	case "object":
		return map[string]any{}
	default:
		return nil
	}
}

func defaultString(format string) string {
	switch format {
	case "email":
		return "user@example.com"
	case "uuid":
		return "00000000-0000-0000-0000-000000000000"
	case "date-time":
		return "2026-01-01T00:00:00Z"
	case "date":
		return "2026-01-01"
	case "uri", "url":
		return "https://example.com"
	case "password":
		return ""
	default:
		return ""
	}
}

func mustJSON(v any) string {
	if v == nil {
		return ""
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}
