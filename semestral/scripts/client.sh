#!/usr/bin/env bash
set -euo pipefail

# Optional first argument specifies instance
INSTANCE=${1:-c1}

mkdir -p "data/$INSTANCE" "logs/$INSTANCE"

# JWT verification
export JWT_ISSUER="hamkatar-gommessenger"
export JWT_PUBLIC_KEY="resources/jwt-keys/public.key"

# TLS certificate used by both servers
export SERVER_CERT="resources/certs/localhost-cert.pem"

# gRPC endpoints
export PROFILES_API_ADDR="localhost:8081"
export MESSAGING_API_ADDR="localhost:8082"

# Local client storage
export LOCAL_DATA_DIR="data/$INSTANCE"

# Log file path
export APP_LOG_FILE="logs/$INSTANCE/client.log"

./bin/client
