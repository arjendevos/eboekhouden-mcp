// Package spec turns the embedded e-Boekhouden OpenAPI document into a list of
// operations, each with a JSON Schema suitable as an MCP tool input schema.
//
// To update to a newer API version, replace openapi.json with
// https://api.e-boekhouden.nl/openapi/v1.json.
package spec

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode"
)

//go:embed openapi.json
var openapiJSON []byte

// Param is a path or query parameter of an operation.
type Param struct {
	Name     string
	In       string // "path" or "query"
	Required bool
	// Filter is set for parameters that accept the API's filter syntax,
	// e.g. date[gte]=2025-01-01.
	Filter bool
}

// Operation is one API call, exposed as one MCP tool.
type Operation struct {
	Name        string // snake_case tool name, e.g. "get_mutations"
	Description string
	Method      string
	Path        string
	Params      []Param
	HasBody     bool
	InputSchema map[string]any
}

// ReadOnly reports whether the operation only reads data.
func (o Operation) ReadOnly() bool { return o.Method == "GET" }

// Destructive reports whether the operation deletes data.
func (o Operation) Destructive() bool { return o.Method == "DELETE" }

type document struct {
	Info struct {
		Description string `json:"description"`
	} `json:"info"`
	Paths      map[string]map[string]json.RawMessage `json:"paths"`
	Components struct {
		Schemas map[string]any `json:"schemas"`
	} `json:"components"`
}

type rawOperation struct {
	OperationID string `json:"operationId"`
	Description string `json:"description"`
	Parameters  []struct {
		Name        string         `json:"name"`
		In          string         `json:"in"`
		Description string         `json:"description"`
		Required    bool           `json:"required"`
		Schema      map[string]any `json:"schema"`
		Example     any            `json:"example"`
	} `json:"parameters"`
	RequestBody *struct {
		Required bool                      `json:"required"`
		Content  map[string]map[string]any `json:"content"`
	} `json:"requestBody"`
}

// Session handling is done by the client, not exposed as tools.
var skipped = map[string]bool{"StartSession": true, "EndSession": true}

const filterHelp = " Pass a plain value to match exactly, or prefix an operator in brackets: "

var filterOps = map[string]string{
	"filter-string": `[not_eq], [like], [not_like] (wildcards % and ?), e.g. "[like]%acme%".`,
	"filter-int64":  `[not_eq], [gt], [gte], [lt], [lte], [range], e.g. "[range]5,10".`,
	"filter-int32":  `[not_eq], [gt], [gte], [lt], [lte], [range], e.g. "[range]5,10".`,
	"filter-date":   `[not_eq], [gt], [gte], [lt], [lte], [range], e.g. "[gte]2025-01-01" or "[range]2025-01-01,2025-12-31".`,
}

// Reference returns the API's general documentation (filters, paging, VAT
// codes) as markdown.
func Reference() string {
	doc, err := load()
	if err != nil {
		return ""
	}
	return strings.ReplaceAll(doc.Info.Description, "\r\n", "\n")
}

// Operations returns all API operations, sorted by tool name.
func Operations() ([]Operation, error) {
	doc, err := load()
	if err != nil {
		return nil, err
	}
	var ops []Operation
	for path, methods := range doc.Paths {
		if !strings.HasPrefix(path, "/v1/") {
			continue
		}
		for method, raw := range methods {
			method = strings.ToUpper(method)
			switch method {
			case "GET", "POST", "PUT", "PATCH", "DELETE":
			default:
				continue
			}
			var ro rawOperation
			if err := json.Unmarshal(raw, &ro); err != nil {
				return nil, fmt.Errorf("%s %s: %w", method, path, err)
			}
			if ro.OperationID == "" || skipped[ro.OperationID] {
				continue
			}
			op, err := buildOperation(doc, method, path, ro)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", ro.OperationID, err)
			}
			ops = append(ops, op)
		}
	}
	sort.Slice(ops, func(i, j int) bool { return ops[i].Name < ops[j].Name })
	return ops, nil
}

func load() (*document, error) {
	var doc document
	if err := json.Unmarshal(openapiJSON, &doc); err != nil {
		return nil, fmt.Errorf("parse openapi.json: %w", err)
	}
	return &doc, nil
}

func buildOperation(doc *document, method, path string, ro rawOperation) (Operation, error) {
	op := Operation{
		Name:        snakeCase(ro.OperationID),
		Description: cleanDescription(ro.Description),
		Method:      method,
		Path:        path,
	}
	properties := map[string]any{}
	var required []string

	for _, p := range ro.Parameters {
		schema := resolve(doc, p.Schema, 0).(map[string]any)
		format, _ := schema["format"].(string)
		param := Param{Name: p.Name, In: p.In, Required: p.Required || p.In == "path"}
		desc := cleanDescription(p.Description)
		if help, ok := filterOps[format]; ok {
			param.Filter = true
			// Operators are passed inside the value, so it must be a string.
			schema = map[string]any{"type": "string"}
			desc = strings.TrimSpace(desc + filterHelp + help)
		}
		if p.In == "path" {
			schema = map[string]any{"type": "integer"}
		}
		if desc != "" {
			schema["description"] = desc
		}
		if p.Example != nil {
			schema["examples"] = []any{p.Example}
		}
		properties[p.Name] = schema
		if param.Required {
			required = append(required, p.Name)
		}
		op.Params = append(op.Params, param)
	}

	if rb := ro.RequestBody; rb != nil {
		content, ok := rb.Content["application/json"]
		if !ok {
			return op, fmt.Errorf("request body has no application/json schema")
		}
		body := resolve(doc, content["schema"], 0).(map[string]any)
		if _, ok := body["type"]; !ok {
			body["type"] = "object"
		}
		delete(body, "title")
		body["description"] = "The JSON request body."
		if method == "PATCH" {
			body["description"] = "The fields to change; omitted fields are left unchanged."
		}
		properties["body"] = body
		required = append(required, "body")
		op.HasBody = true
	}

	schema := map[string]any{
		"type":                 "object",
		"properties":           properties,
		"additionalProperties": false,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	op.InputSchema = schema
	return op, nil
}

// resolve returns a copy of v with every $ref inlined and descriptions cleaned
// up for use by a language model.
func resolve(doc *document, v any, depth int) any {
	if depth > 20 {
		return map[string]any{}
	}
	switch v := v.(type) {
	case map[string]any:
		if ref, ok := v["$ref"].(string); ok {
			name := strings.TrimPrefix(ref, "#/components/schemas/")
			target, ok := doc.Components.Schemas[name]
			if !ok {
				return map[string]any{}
			}
			return resolve(doc, target, depth+1)
		}
		out := make(map[string]any, len(v))
		for k, val := range v {
			switch k {
			case "properties":
				// Keys here are field names, not schema keywords.
				if props, ok := val.(map[string]any); ok {
					resolved := make(map[string]any, len(props))
					for name, p := range props {
						resolved[name] = resolve(doc, p, depth+1)
					}
					out[k] = resolved
					continue
				}
			case "description":
				if s, ok := val.(string); ok {
					if s = cleanDescription(s); s != "" {
						out[k] = s
					}
					continue
				}
			case "format":
				// "string" is not a real format and adds noise.
				if val == "string" {
					continue
				}
			case "example":
				// "example" is deprecated in JSON Schema; use "examples".
				out["examples"] = []any{val}
				continue
			}
			out[k] = resolve(doc, val, depth+1)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, val := range v {
			out[i] = resolve(doc, val, depth+1)
		}
		return out
	default:
		return v
	}
}

var (
	errorTable = regexp.MustCompile(`(?s)(<br\s*/?>)?\s*<div>\s*<h3>Error codes</h3>.*?</div>`)
	lineBreak  = regexp.MustCompile(`<br\s*/?>`)
	htmlTag    = regexp.MustCompile(`<[^>]+>`)
	blankLines = regexp.MustCompile(`\n{3,}`)
)

// cleanDescription strips the HTML the spec embeds in descriptions, including
// the per-field error code tables.
func cleanDescription(s string) string {
	s = errorTable.ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = lineBreak.ReplaceAllString(s, "\n")
	s = htmlTag.ReplaceAllString(s, "")
	s = blankLines.ReplaceAllString(s, "\n\n")
	return strings.TrimSpace(s)
}

// snakeCase converts "GetOutstandingInvoices" to "get_outstanding_invoices".
func snakeCase(s string) string {
	var b strings.Builder
	for i, r := range s {
		if unicode.IsUpper(r) {
			if i > 0 {
				b.WriteByte('_')
			}
			r = unicode.ToLower(r)
		}
		b.WriteRune(r)
	}
	return b.String()
}
