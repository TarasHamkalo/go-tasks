#!/usr/bin/env bash
set -e

mkdir -p bin

go build -o bin/profile-server ./cmd/profile-server
go build -o bin/messaging-server ./cmd/messaging-server
go build -o bin/client ./cmd/client

echo "Build complete. See ./bin"
