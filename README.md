# eboekhouden-mcp

[![CI](https://github.com/arjendevos/eboekhouden-mcp/actions/workflows/ci.yml/badge.svg)](https://github.com/arjendevos/eboekhouden-mcp/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/arjendevos/eboekhouden-mcp)](https://github.com/arjendevos/eboekhouden-mcp/releases/latest)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

An [MCP](https://modelcontextprotocol.io) server for [e-Boekhouden.nl](https://www.e-boekhouden.nl). It lets Claude, Cursor, VS Code or any other MCP client work with your bookkeeping: look up mutations, relations and invoices, check ledger balances, and book new entries.

> **Nederlands:** een MCP-server voor e-Boekhouden.nl. Koppel je administratie aan Claude of een andere AI-assistent en vraag bijvoorbeeld *"Welke facturen staan nog open?"* of *"Wat is het saldo van de bank per 31 december?"*

Some things you can ask once it's connected:

- "Which sales invoices are still outstanding?"
- "What were my biggest expenses last quarter, per ledger?"
- "Show the balance of every ledger at the end of 2025."
- "Create a relation for Acme BV, Keizersgracht 1, Amsterdam."
- "Book this receipt: €48.40 incl. 21% VAT for software, paid today."

It's a single small binary, written in Go. The tools are generated from e-Boekhouden's own OpenAPI spec, so every endpoint of their REST API is available, with their field descriptions.

## Quick start

### 1. Create an API token

In e-Boekhouden, open the settings of your account and create an **API token**. Copy it; it is shown only once.

### 2. Install

Pick one:

**Docker** (nothing else to install):

```sh
docker pull ghcr.io/arjendevos/eboekhouden-mcp
```

**Download a binary** for macOS, Linux or Windows from the [latest release](https://github.com/arjendevos/eboekhouden-mcp/releases/latest), unpack it and put `eboekhouden-mcp` somewhere on your `PATH`. On macOS, if you downloaded it with a browser, allow it to run with `xattr -d com.apple.quarantine eboekhouden-mcp`.

**Go** (1.25 or newer):

```sh
go install github.com/arjendevos/eboekhouden-mcp@latest
```

### 3. Add it to your AI client

<details open>
<summary><b>Claude Code</b></summary>

```sh
claude mcp add eboekhouden -e EBOEKHOUDEN_API_TOKEN=your-token -- eboekhouden-mcp
```

or with Docker:

```sh
claude mcp add eboekhouden -e EBOEKHOUDEN_API_TOKEN=your-token -- docker run -i --rm -e EBOEKHOUDEN_API_TOKEN ghcr.io/arjendevos/eboekhouden-mcp
```

</details>

<details>
<summary><b>Claude Desktop</b></summary>

Open *Settings → Developer → Edit Config* and add:

```json
{
  "mcpServers": {
    "eboekhouden": {
      "command": "/full/path/to/eboekhouden-mcp",
      "env": { "EBOEKHOUDEN_API_TOKEN": "your-token" }
    }
  }
}
```

or with Docker:

```json
{
  "mcpServers": {
    "eboekhouden": {
      "command": "docker",
      "args": ["run", "-i", "--rm", "-e", "EBOEKHOUDEN_API_TOKEN", "ghcr.io/arjendevos/eboekhouden-mcp"],
      "env": { "EBOEKHOUDEN_API_TOKEN": "your-token" }
    }
  }
}
```

Restart Claude Desktop afterwards.

</details>

<details>
<summary><b>Cursor</b></summary>

Add to `~/.cursor/mcp.json`:

```json
{
  "mcpServers": {
    "eboekhouden": {
      "command": "eboekhouden-mcp",
      "env": { "EBOEKHOUDEN_API_TOKEN": "your-token" }
    }
  }
}
```

</details>

<details>
<summary><b>VS Code</b></summary>

Add to `.vscode/mcp.json`:

```json
{
  "servers": {
    "eboekhouden": {
      "command": "eboekhouden-mcp",
      "env": { "EBOEKHOUDEN_API_TOKEN": "${input:eboekhouden-token}" }
    }
  },
  "inputs": [
    { "id": "eboekhouden-token", "type": "promptString", "description": "e-Boekhouden API token", "password": true }
  ]
}
```

</details>

## Configuration

All settings are environment variables.

| Variable | Description |
|---|---|
| `EBOEKHOUDEN_API_TOKEN` | **Required.** Your e-Boekhouden API token. |
| `EBOEKHOUDEN_READ_ONLY` | Set to `true` to expose only the tools that read data. Recommended to start with. |
| `EBOEKHOUDEN_SOURCE` | Name for this integration as shown in e-Boekhouden (max 10 characters, default `MCP`). |
| `EBOEKHOUDEN_BASE_URL` | Override the API address (default `https://api.e-boekhouden.nl`). |

## Tools

| Area | Tools |
|---|---|
| Mutations (boekingen) | `get_mutations`, `get_mutation`, `create_mutation`, `get_outstanding_invoices` |
| Relations (relaties) | `get_relations`, `get_relation`, `create_relation`, `update_relation` |
| Invoices (facturen) | `get_invoices`, `get_invoice`, `create_invoice`, `get_invoice_templates`, `get_email_templates` |
| Ledgers (grootboekrekeningen) | `get_ledgers`, `get_ledger`, `create_ledger`, `update_ledger`, `get_balance`, `get_balances` |
| Cost centers (kostenplaatsen) | `get_cost_centers`, `get_cost_center`, `create_cost_center`, `update_cost_center`, `delete_cost_center` |
| Products (artikelen) | `get_products`, `get_product`, `create_product`, `update_product`, `delete_product`, `get_product_group`, `get_units` |
| Members (leden, for associations) | `get_members`, `get_member`, `create_member`, `update_member` |
| Administrations (for accountants) | `get_administrations`, `get_linked_administrations` |
| Reference | `get_api_reference`: filter syntax, mutation types and VAT codes |

Tools are annotated as read-only, or as destructive for deletes, so clients can ask for confirmation before changing anything.

**Filters.** Search parameters accept e-Boekhouden's filter operators inside the value: `"2025-01-01"` (equals), `"[gte]2025-01-01"`, `"[range]2025-01-01,2025-12-31"`, `"[like]%acme%"`.

**Paging.** List tools return up to `limit` items (1–2000, default 100); use `offset` for the next page.

## Safety

With write tools enabled, an AI can create mutations, relations, invoices and more in your real administration. Start with `EBOEKHOUDEN_READ_ONLY=true`, keep your client's tool confirmations on, and check what gets booked.

Your API token stays on your machine: the server only sends it to `api.e-boekhouden.nl` to open a session, and closes the session when it stops.

## How it works

The server embeds e-Boekhouden's OpenAPI document ([`internal/spec/openapi.json`](internal/spec/openapi.json)) and creates one MCP tool per operation, with input schemas built from the spec. It exchanges your API token for a short-lived session token and renews it when it expires.

To pick up new API features, replace that file with the latest <https://api.e-boekhouden.nl/openapi/v1.json> and rebuild.

## Development

```sh
git clone https://github.com/arjendevos/eboekhouden-mcp
cd eboekhouden-mcp
go test ./...
go build -o bin/eboekhouden-mcp .
```

You can try it interactively with the [MCP Inspector](https://github.com/modelcontextprotocol/inspector):

```sh
npx @modelcontextprotocol/inspector -e EBOEKHOUDEN_API_TOKEN=your-token -e EBOEKHOUDEN_READ_ONLY=true ./bin/eboekhouden-mcp
```

Contributions are welcome; see [CONTRIBUTING.md](CONTRIBUTING.md).

## Disclaimer

This is an independent open-source project. It is not affiliated with, or endorsed by, e-Boekhouden.nl. Use it at your own risk; you remain responsible for your bookkeeping.

## License

[MIT](LICENSE)
