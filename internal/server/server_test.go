package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/arjendevos/eboekhouden-mcp/internal/eboekhouden"
)

type request struct {
	Method, Path, Query, Auth, Body string
}

// fakeAPI records requests and hands out a new session token per session call.
type fakeAPI struct {
	mu       sync.Mutex
	requests []request
	sessions int
	// expire makes the next authenticated request fail with 401.
	expire bool
}

func (f *fakeAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	f.mu.Lock()
	defer f.mu.Unlock()
	f.requests = append(f.requests, request{r.Method, r.URL.Path, r.URL.RawQuery, r.Header.Get("Authorization"), string(body)})

	w.Header().Set("Content-Type", "application/json")
	switch {
	case r.Method == "POST" && r.URL.Path == "/v1/session":
		f.sessions++
		json.NewEncoder(w).Encode(map[string]any{"token": "session-" + string(rune('0'+f.sessions)), "expiresIn": 3600})
	case f.expire:
		f.expire = false
		w.WriteHeader(http.StatusUnauthorized)
	case r.Method == "PATCH":
		w.WriteHeader(http.StatusNoContent)
	case r.URL.Path == "/v1/mutation/999":
		w.WriteHeader(http.StatusNotFound)
		io.WriteString(w, `{"code":"MUT_404","title":"Not found"}`)
	default:
		io.WriteString(w, `{"items":[],"count":0}`)
	}
}

func (f *fakeAPI) last() request {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.requests[len(f.requests)-1]
}

func connect(t *testing.T, opts Options) (*mcp.ClientSession, *fakeAPI) {
	t.Helper()
	api := &fakeAPI{}
	ts := httptest.NewServer(api)
	t.Cleanup(ts.Close)

	srv, err := New(&eboekhouden.Client{BaseURL: ts.URL, APIToken: "secret", Source: "test"}, opts)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	st, ct := mcp.NewInMemoryTransports()
	if _, err := srv.Connect(ctx, st, nil); err != nil {
		t.Fatal(err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cs.Close() })
	return cs, api
}

func call(t *testing.T, cs *mcp.ClientSession, name string, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return res
}

func text(res *mcp.CallToolResult) string {
	return res.Content[0].(*mcp.TextContent).Text
}

func TestListTools(t *testing.T) {
	cs, _ := connect(t, Options{})
	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, tool := range res.Tools {
		names[tool.Name] = true
		if strings.Contains(tool.Description, "<") {
			t.Errorf("%s: description contains HTML: %q", tool.Name, tool.Description)
		}
	}
	for _, want := range []string{"get_api_reference", "get_mutations", "create_mutation", "create_invoice", "get_relations", "update_relation", "delete_product", "get_balances"} {
		if !names[want] {
			t.Errorf("missing tool %s", want)
		}
	}
	if names["start_session"] || names["end_session"] {
		t.Error("session tools must not be exposed")
	}
}

func TestReadOnly(t *testing.T) {
	cs, _ := connect(t, Options{ReadOnly: true})
	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range res.Tools {
		if !tool.Annotations.ReadOnlyHint {
			t.Errorf("read-only mode exposes %s", tool.Name)
		}
	}
}

func TestQueryAndFilters(t *testing.T) {
	cs, api := connect(t, Options{})
	res := call(t, cs, "get_mutations", map[string]any{
		"limit":       50,
		"date":        "[range]2025-01-01,2025-12-31",
		"description": "huur",
	})
	if res.IsError {
		t.Fatal(text(res))
	}
	got := api.last()
	if got.Path != "/v1/mutation" || got.Auth != "session-1" {
		t.Errorf("got %+v", got)
	}
	want := "date%5Brange%5D=2025-01-01%2C2025-12-31&description=huur&limit=50"
	if got.Query != want {
		t.Errorf("query = %s, want %s", got.Query, want)
	}
}

func TestPathParamAndBody(t *testing.T) {
	cs, api := connect(t, Options{})
	res := call(t, cs, "update_relation", map[string]any{"id": 42, "body": map[string]any{"name": "Acme BV"}})
	if res.IsError || text(res) != "OK" {
		t.Fatalf("got %v %q", res.IsError, text(res))
	}
	got := api.last()
	if got.Method != "PATCH" || got.Path != "/v1/relation/42" || got.Body != `{"name":"Acme BV"}` {
		t.Errorf("got %+v", got)
	}
}

func TestSessionRenewedOn401(t *testing.T) {
	cs, api := connect(t, Options{})
	call(t, cs, "get_ledgers", nil)
	api.mu.Lock()
	api.expire = true
	api.mu.Unlock()
	res := call(t, cs, "get_ledgers", nil)
	if res.IsError {
		t.Fatal(text(res))
	}
	if api.sessions != 2 || api.last().Auth != "session-2" {
		t.Errorf("sessions = %d, last = %+v", api.sessions, api.last())
	}
}

func TestAPIErrorReturnedToModel(t *testing.T) {
	cs, _ := connect(t, Options{})
	res := call(t, cs, "get_mutation", map[string]any{"id": 999})
	if !res.IsError || !strings.Contains(text(res), "MUT_404") {
		t.Errorf("got %v %q", res.IsError, text(res))
	}
}
