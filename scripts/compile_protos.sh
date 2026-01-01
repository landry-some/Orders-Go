#!/usr/bin/env bash
set -euo pipefail

# Ensure protoc plugins are installed.
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# Make sure the plugins are on PATH for this script execution.
export PATH="$PATH:$(go env GOPATH)/bin"

# Generate Go sources from all proto definitions under api/proto, keeping outputs next to inputs.
protos=$(find api/proto -name '*.proto')
if [ -z "$protos" ]; then
  echo "No .proto files found under api/proto"
  exit 0
fi

# Use source_relative so generated files stay in their respective proto folders.
protoc --go_out=. --go-grpc_out=. \
  --go_opt=paths=source_relative \
  --go-grpc_opt=paths=source_relative \
  $protos
