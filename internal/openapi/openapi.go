// Package openapi generates OpenAPI 3.0 documents from extracted endpoints.
package openapi

import (
	"sort"
	"strings"

	projctx "github.com/AnshGajera/CTX/internal/context"
)

// Document is a minimal OpenAPI 3.0 document.
type Document struct {
	OpenAPI string              `json:"openapi"`
	Info    Info                `json:"info"`
	Paths   map[string]PathItem `json:"paths"`
}

// Info holds API metadata.
type Info struct {
	Title   string `json:"title"`
	Version string `json:"version"`
}

// PathItem maps lowercase methods to operations.
type PathItem map[string]Operation

// Operation is one HTTP operation.
type Operation struct {
	Summary     string              `json:"summary,omitempty"`
	OperationID string              `json:"operationId,omitempty"`
	Tags        []string            `json:"tags,omitempty"`
	Parameters  []Parameter         `json:"parameters,omitempty"`
	Responses   map[string]Response `json:"responses"`
}

// Parameter is a path/query parameter.
type Parameter struct {
	Name     string `json:"name"`
	In       string `json:"in"`
	Required bool   `json:"required"`
	Schema   Schema `json:"schema"`
}

// Schema is a primitive schema.
type Schema struct {
	Type string `json:"type"`
}

// Response is a response shell.
type Response struct {
	Description string `json:"description"`
}

// Build converts endpoints to an OpenAPI document. Express-style :params
// become {params}; unknown methods map to x- extensions and are skipped
// unless they are standard verbs or ALL.
func Build(projectName string, endpoints []projctx.APIEndpoint) *Document {
	doc := &Document{
		OpenAPI: "3.0.0",
		Info:    Info{Title: projectName, Version: "1.0.0"},
		Paths:   map[string]PathItem{},
	}
	for _, e := range endpoints {
		methods := []string{strings.ToLower(e.Method)}
		if e.Method == "ALL" {
			methods = []string{"get", "post", "put", "patch", "delete"}
		}
		oasPath := toOASPath(e.Path)
		item, ok := doc.Paths[oasPath]
		if !ok {
			item = PathItem{}
		}
		var params []Parameter
		for _, p := range e.Parameters {
			in := p.In
			if in == "" {
				in = "path"
			}
			typ := p.Type
			if typ == "" {
				typ = "string"
			}
			params = append(params, Parameter{Name: p.Name, In: in, Required: p.Required || in == "path", Schema: Schema{Type: typ}})
		}
		// Also surface :params missed by extractors.
		for _, name := range oasParamNames(oasPath) {
			found := false
			for _, p := range params {
				if p.Name == name {
					found = true
					break
				}
			}
			if !found {
				params = append(params, Parameter{Name: name, In: "path", Required: true, Schema: Schema{Type: "string"}})
			}
		}
		sort.Slice(params, func(i, j int) bool { return params[i].Name < params[j].Name })
		op := Operation{
			Summary:     e.Method + " " + e.Path,
			OperationID: e.Handler,
			Responses:   map[string]Response{"200": {Description: "OK"}},
		}
		if tag := tagFor(e.Path); tag != "" {
			op.Tags = []string{tag}
		}
		if len(params) > 0 {
			op.Parameters = params
		}
		for _, m := range methods {
			if !validMethod(m) {
				continue
			}
			item[m] = op
		}
		doc.Paths[oasPath] = item
	}
	return doc
}

func validMethod(m string) bool {
	switch m {
	case "get", "post", "put", "patch", "delete", "head", "options", "trace":
		return true
	}
	return false
}

func toOASPath(p string) string {
	var out strings.Builder
	for i := 0; i < len(p); i++ {
		if p[i] == ':' {
			j := i + 1
			for j < len(p) && (isPathChar(p[j])) {
				j++
			}
			out.WriteString("{" + p[i+1:j] + "}")
			i = j - 1
			continue
		}
		out.WriteByte(p[i])
	}
	s := out.String()
	if s == "" {
		return "/"
	}
	return s
}

func isPathChar(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == '-'
}

func oasParamNames(oasPath string) []string {
	var out []string
	for i := 0; i < len(oasPath); i++ {
		if oasPath[i] == '{' {
			j := i + 1
			for j < len(oasPath) && oasPath[j] != '}' {
				j++
			}
			if j < len(oasPath) {
				out = append(out, oasPath[i+1:j])
			}
			i = j
		}
	}
	return out
}

func tagFor(path string) string {
	trimmed := strings.Trim(path, "/")
	if trimmed == "" {
		return "root"
	}
	seg := strings.Split(trimmed, "/")[0]
	seg = strings.Trim(seg, "{}:")
	if seg == "api" {
		parts := strings.Split(trimmed, "/")
		if len(parts) > 1 {
			seg = strings.Trim(parts[1], "{}:")
		}
	}
	return seg
}
