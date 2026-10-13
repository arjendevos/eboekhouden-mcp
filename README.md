# eboekhouden-mcp

An [MCP](https://modelcontextprotocol.io) server for [e-Boekhouden.nl](https://www.e-boekhouden.nl), written in Go. It lets Claude (or any MCP client) read and book in your administration: mutations, relations, invoices, ledgers and balances, cost centers, products and members.

The tools are generated from e-Boekhouden's own OpenAPI spec (`internal/spec/openapi.json`), so every endpoint of the REST API is available with its real field descriptions. To pick up new API features, replace that file with <https://api.e-boekhouden.nl/openapi/v1.json> and rebuild.

## Install

```sh
go install github.com/pointers-agency/eboekhouden-mcp@latest
```

or build from a checkout with `go build -o bin/eboekhouden-mcp .`

## Configure

Create an API token in the settings of your e-Boekhouden account, then:

| Variable | |
|---|---|
| `EBOEKHOUDEN_API_TOKEN` | Required. Your API token. |
| `EBOEKHOUDEN_READ_ONLY` | `true` to expose only the tools that read data. |
| `EBOEKHOUDEN_SOURCE` | Name shown in e-Boekhouden for this integration (max 10 chars, default `MCP`). |
| `EBOEKHOUDEN_BASE_URL` | Override the API URL (default `https://api.e-boekhouden.nl`). |

### Claude Code

```sh
claude mcp add eboekhouden -e EBOEKHOUDEN_API_TOKEN=... -- eboekhouden-mcp
```

### Claude Desktop

```json
{
  "mcpServers": {
    "eboekhouden": {
      "command": "/path/to/eboekhouden-mcp",
      "env": { "EBOEKHOUDEN_API_TOKEN": "...", "EBOEKHOUDEN_READ_ONLY": "true" }
    }
  }
}
```

## Tools

One tool per API operation, named after it: `get_mutations`, `create_mutation`, `get_relations`, `update_relation`, `create_invoice`, `get_outstanding_invoices`, `get_balances`, and so on (38 in total). Plus `get_api_reference`, which returns the API's notes on filters, mutation types and VAT codes.

Search parameters accept e-Boekhouden's filter operators inside the value: `"[gte]2025-01-01"`, `"[range]2025-01-01,2025-12-31"`, `"[like]%acme%"`.

The server handles sessions itself: it exchanges the API token for a session token and renews it when it expires.

## Develop

```sh
go test ./...
```
