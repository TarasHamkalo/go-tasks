#!/usr/bin/env bash
set -euo pipefail

export APP_LOG_FILE="logs/profile-server.log"
export PROFILES_DB="data/profiles.db"

export JWT_PUBLIC_KEY="resources/jwt-keys/public.key"
export JWT_PRIVATE_KEY="resources/jwt-keys/private.key"

export TLS_CERT="resources/certs/localhost-cert.pem"
export TLS_KEY="resources/certs/localhost-privkey.pem"

export JWT_ISSUER="hamkatar-gomessenger"

export PORT="8081"

mkdir -p logs data

go run ./cmd/profile-server
