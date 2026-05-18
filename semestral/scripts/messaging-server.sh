#!/usr/bin/env bash
set -euo pipefail

export APP_LOG_FILE="logs/messaging-server.log"
export MESSAGING_DB="data/messaging.db"

# Profile service gRPC endpoint
export PROFILES_API_ADDR="localhost:8081"

# JWT verification
export JWT_PUBLIC_KEY="resources/jwt-keys/public.key"
export JWT_ISSUER="hamkatar-gommessenger"

# TLS certificate used by the messaging server
export TLS_CERT="resources/certs/localhost-cert.pem"
export TLS_KEY="resources/certs/localhost-privkey.pem"

# Listening port
export PORT="8082"

./bin/messaging-server
