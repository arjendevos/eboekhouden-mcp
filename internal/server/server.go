// Package server exposes the e-Boekhouden API as MCP tools.
package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/arjendevos/eboekhouden-mcp/internal/eboekhouden"
	"github.com/arjendevos/eboekhouden-mcp/internal/spec"
)

// Options configures the server.
type Options struct {
	Version string
	// ReadOnly registers only tools that read data.
	ReadOnly bool
}

const instructions = `Tools for the Dutch bookkeeping service e-Boekhouden.nl.
List tools return {"items": [...], "count": n} where count is the number of items in this page, not the total: if count equals limit, fetch the next page with offset. limit is 1-2000 (default 100).
Dates are YYYY-MM-DD. Call get_api_reference for filter syntax, mutation types and the valid VAT codes before creating mutations or invoices.`

// New builds an MCP server with one tool per API operation.
func New(client *eboekhouden.Client, opts Options) (*mcp.Server, error) {
	ops, err := spec.Operations()
	if err != nil {
		return nil, err
	}
	s := mcp.NewServer(&mcp.Implementation{Name: "eboekhouden", Version: opts.Version}, &mcp.ServerOptions{Instructions: instructions})

	s.AddTool(&mcp.Tool{
		Name:        "get_api_reference",
		Description: "Get the e-Boekhouden API reference: filter syntax, paging, date format, mutation types and VAT codes per country.",
		InputSchema: map[string]any{"type": "object", "properties": map[string]any{}},
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true, OpenWorldHint: ptr(false)},
	}, func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return textResult(spec.Reference(), false), nil
	})

	for _, op := range ops {
		if opts.ReadOnly && !op.ReadOnly() {
			continue
		}
		s.AddTool(&mcp.Tool{
			Name:        op.Name,
			Description: op.Description,
			InputSchema: op.InputSchema,
			Annotations: &mcp.ToolAnnotations{
				ReadOnlyHint:    op.ReadOnly(),
				DestructiveHint: ptr(op.Destructive()),
				IdempotentHint:  op.ReadOnly() || op.Method == "PATCH" || op.Method == "DELETE",
				OpenWorldHint:   ptr(true),
			},
		}, handler(client, op))
	}
	return s, nil
}

func handler(client *eboekhouden.Client, op spec.Operation) mcp.ToolHandler {
	return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args map[string]any
		if raw := req.Params.Arguments; len(raw) > 0 {
			if err := json.Unmarshal(raw, &args); err != nil {
				return textResult("invalid arguments: "+err.Error(), true), nil
			}
		}
		path, query, err := buildRequest(op, args)
		if err != nil {
			return textResult(err.Error(), true), nil
		}
		var body any
		if op.HasBody {
			body = args["body"]
		}

		resp, err := client.Do(ctx, op.Method, path, query, body)
		var apiErr *eboekhouden.APIError
		if errors.As(err, &apiErr) {
			// Hand validation errors back to the model so it can correct itself.
			return textResult(err.Error(), true), nil
		}
		if err != nil {
			return nil, err
		}
		if len(resp) == 0 {
			return textResult("OK", false), nil
		}
		return textResult(string(resp), false), nil
	}
}

// buildRequest fills in path parameters and builds the query string.
func buildRequest(op spec.Operation, args map[string]any) (string, url.Values, error) {
	path := op.Path
	query := url.Values{}
	for _, p := range op.Params {
		v, ok := args[p.Name]
		if !ok || v == nil {
			if p.Required {
				return "", nil, fmt.Errorf("missing required parameter %q", p.Name)
			}
			continue
		}
		s := formatValue(v)
		if p.In == "path" {
			path = strings.ReplaceAll(path, "{"+p.Name+"}", url.PathEscape(s))
			continue
		}
		key := p.Name
		// "[gte]2025-01-01" becomes date[gte]=2025-01-01.
		if p.Filter && strings.HasPrefix(s, "[") {
			if end := strings.Index(s, "]"); end > 0 {
				key += s[:end+1]
				s = s[end+1:]
			}
		}
		query.Set(key, s)
	}
	return path, query, nil
}

func formatValue(v any) string {
	switch v := v.(type) {
	case string:
		return v
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(v)
	default:
		b, _ := json.Marshal(v)
		return string(b)
	}
}

func textResult(text string, isError bool) *mcp.CallToolResult {
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}, IsError: isError}
}

func ptr[T any](v T) *T { return &v }
