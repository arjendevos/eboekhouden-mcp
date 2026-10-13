# Contributing

Thanks for helping out! Bug reports, ideas and pull requests are all welcome.

## Reporting a bug

Open an [issue](https://github.com/arjendevos/eboekhouden-mcp/issues) with what you asked, which tool was called with which arguments, and the error you got. Leave out your API token and any personal or financial data.

## Making a change

1. Fork the repository and create a branch.
2. Make your change, and add or update tests in `internal/server/server_test.go`. The tests use a fake API, so you don't need an e-Boekhouden account.
3. Run `gofmt -w .`, `go vet ./...` and `go test ./...`.
4. Open a pull request that explains what changed and why.

Commit messages follow [Conventional Commits](https://www.conventionalcommits.org): `feat: ...`, `fix: ...`, `docs: ...`.

## Updating the API spec

The tools are generated from `internal/spec/openapi.json`. To update it:

```sh
curl -sSL https://api.e-boekhouden.nl/openapi/v1.json -o internal/spec/openapi.json
go test ./...
```

## Releasing

Maintainers tag a version (`git tag v0.2.0 && git push --tags`). GitHub Actions then builds the binaries and the Docker image.
