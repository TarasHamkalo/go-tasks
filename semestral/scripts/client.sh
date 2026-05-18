#!/usr/bin/env bash
set -euo pipefail

# Optional first argument specifies the log directory.
# Defaults to "logs" if not provided.
DATA_DIR="${1:-data}"
LOG_DIR="${2:-logs}"

mkdir -p "$DATA_DIR"

mkdir -p "$LOG_DIR"

# TUI client configuration
export APP_LOG_FILE="$LOG_DIR/tui.log"

# JWT verification
export JWT_ISSUER="hamkatar-gommessenger"
export JWT_PUBLIC_KEY="resources/jwt-keys/public.key"

# TLS certificate used by both servers
export SERVER_CERT="resources/certs/localhost-cert.pem"

# gRPC endpoints
export PROFILES_API_ADDR="localhost:8081"
export MESSAGING_API_ADDR="localhost:8082"

# Local client storage
export LOCAL_DATA_DIR=$DATA_DIR

go run ./cmd/client
