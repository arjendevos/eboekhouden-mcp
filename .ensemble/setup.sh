#!/usr/bin/env bash
# Prepares a fresh ensemble checkout: Go toolchain and module dependencies.
set -euo pipefail
if ! command -v go >/dev/null 2>&1; then
  brew install go
fi
go mod download
