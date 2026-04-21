set -e

GRPC_ADDR="localhost:8081"
HTTP_ADDR="http://localhost:8080"
SERVICE="ManagementService"

log_http() {
  local method=$1
  local path=$2
  local body=$3

  if [ -z "$body" ]; then
    echo "HTTP Request: method: $method, path: $path"
  else
    echo "HTTP Request: method: $method, path: $path, body: $body"
  fi
}

log_grpc() {
  echo "gRPC Call: method: $1"
}

clear_db() {
  log_grpc "ClearDatabase"
  grpcurl -insecure -d '{}' $GRPC_ADDR $SERVICE/ClearDatabase
  echo
}

dump_db() {
  log_grpc "DumpDatabase"
  grpcurl -insecure $GRPC_ADDR $SERVICE/DumpDatabase
  echo
}

########################################
# Example 1: Initial state
########################################
echo "=== Example 1: Empty state test ==="

clear_db

echo "Test: Expect 501 when no configuration exists"
log_http "GET" "/users"
curl -i $HTTP_ADDR/users
echo

########################################
# Example 2: GET with Body
########################################
echo "=== Example 2: Basic GET configuration ==="

clear_db

BODY='["user-1","user-2"]'
B64_BODY=$(echo -n "$BODY" | base64)

log_grpc "SetReply, method: GET, path: /users"
grpcurl -insecure -d "{
  \"method\": \"GET\",
  \"path\": \"/users\",
  \"status_code\": 200,
  \"response_body\": \"$B64_BODY\"
}" $GRPC_ADDR $SERVICE/SetReply

dump_db

echo "Test: Expect 200 on path match"
log_http "GET" "/users"
curl -i $HTTP_ADDR/users
echo

echo "Test: Expect 404 on path mismatch"
log_http "GET" "/invalid"
curl -i $HTTP_ADDR/invalid
echo

########################################
# Example 3: POST body matching
########################################
echo "=== Example 3: POST body exact match ==="

clear_db

REQ_BODY='{"name":"user-name"}'
B64_REQ=$(echo -n "$REQ_BODY" | base64)

log_grpc "SetReply, method: POST, path: /users/user-1, req_body: $REQ_BODY"
grpcurl -insecure -d "{
  \"method\": \"POST\",
  \"path\": \"/users/user-1\",
  \"request_body\": \"$B64_REQ\",
  \"status_code\": 200
}" $GRPC_ADDR $SERVICE/SetReply

dump_db

echo "Test: Expect 200 on matching body"
log_http "POST" "/users/user-1" "$REQ_BODY"
curl -i -X POST $HTTP_ADDR/users/user-1 -d "$REQ_BODY"
echo

echo "Test: Expect 404 on body content mismatch"
BAD_BODY='{"name":"wrong"}'
log_http "POST" "/users/user-1" "$BAD_BODY"
curl -i -X POST $HTTP_ADDR/users/user-1 -d "$BAD_BODY"
echo

########################################
# Example 4: Query parameter ordering
########################################
echo "=== Example 4: Query parameter permutation ==="

clear_db

RESP_BODY='{"ok": true}'
B64_RESP=$(echo -n "$RESP_BODY" | base64)

log_grpc "SetReply, path: /test?name=Name&name=Surname&name=Nickname"
grpcurl -insecure -d "{
  \"method\": \"GET\",
  \"path\": \"/test?name=Name&name=Surname&name=Nickname\",
  \"status_code\": 200,
  \"response_body\": \"$B64_RESP\"
}" $GRPC_ADDR $SERVICE/SetReply

dump_db

echo "Test: Expect 200 even if query order is different"
QUERY="/test?name=Surname&name=Nickname&name=Name"
log_http "GET" "$QUERY"
curl -i "$HTTP_ADDR$QUERY"
echo

########################################
# Example 5: Simple string body mismatch
########################################
echo "=== Example 5: Plain text body mismatch ==="

clear_db

REQ_BODY='aaa'
B64_REQ=$(echo -n "$REQ_BODY" | base64)

log_grpc "SetReply, method: POST, req_body: $REQ_BODY"
grpcurl -insecure -d "{
  \"method\": \"POST\",
  \"path\": \"/test\",
  \"request_body\": \"$B64_REQ\",
  \"status_code\": 200
}" $GRPC_ADDR $SERVICE/SetReply

dump_db

echo "Test: Expect 404 for provided body 'bbb'"
log_http "POST" "/test" "bbb"
curl -i -X POST $HTTP_ADDR/test -d "bbb"
echo

########################################
# Example 6: Multi-value query params
########################################
echo "=== Example 6: Duplicate query keys ==="

clear_db

RESP_BODY='ok'
B64_RESP=$(echo -n "$RESP_BODY" | base64)

log_grpc "SetReply, path: /test?name=A&name=A"
grpcurl -insecure -d "{
  \"method\": \"GET\",
  \"path\": \"/test?name=A&name=A\",
  \"status_code\": 200,
  \"response_body\": \"$B64_RESP\"
}" $GRPC_ADDR $SERVICE/SetReply

dump_db

echo "Test: Expect 200 on exact count match"
QUERY="/test?name=A&name=A"
curl -i "$HTTP_ADDR$QUERY"
echo

echo "Test: Expect 404 when query parameter count differs"
QUERY="/test?name=A"
curl -i "$HTTP_ADDR$QUERY"
echo

########################################
# Example 7: Method enforcement
########################################
echo "=== Example 7: Method Enforcement (HTTP 405 and gRPC Validation) ==="

clear_db

log_grpc "SetReply, method: GET, path: /users"
grpcurl -insecure -d "{
  \"method\": \"GET\",
  \"path\": \"/users\",
  \"status_code\": 200
}" $GRPC_ADDR $SERVICE/SetReply

echo "Test: Expect 405, reason: HEAD is not supported by mocker"
log_http "HEAD" "/users"
curl --head $HTTP_ADDR/users
echo

echo "Test: Expect gRPC Error, reason: Cannot configure unsupported method"
log_grpc "SetReply, method: HEAD, path: /users"
# expect this to fail with InvalidArgument
grpcurl -insecure -d "{
  \"method\": \"HEAD\",
  \"path\": \"/users\",
  \"status_code\": 200
}" $GRPC_ADDR $SERVICE/SetReply || echo "gRPC call failed as expected"
echo

########################################
# Example 8: Multi-value Response Headers
########################################
echo "=== Example 8: Custom Headers/Cookies ==="

clear_db

REQ_BODY='{"name":"user-name"}'
B64_REQ=$(echo -n "$REQ_BODY" | base64)
RESP_BODY='{"ok": true}'
B64_RESP=$(echo -n "$RESP_BODY" | base64)

log_grpc "SetReply, adding Content-Type and multiple Cookie values"
grpcurl -insecure -d "{
  \"method\": \"POST\",
  \"path\": \"/users/user-1\",
  \"request_body\": \"$B64_REQ\",
  \"response_body\": \"$B64_RESP\",
  \"status_code\": 200,
  \"response_headers\": {
    \"Content-Type\": { \"values\": [\"application/json\"] },
    \"Cookie\": { \"values\": [\"a\", \"b\"] }
  }
}" $GRPC_ADDR $SERVICE/SetReply

dump_db

echo "Test: Verify response headers in output"
curl -i -X POST $HTTP_ADDR/users/user-1 -d "$REQ_BODY"
echo

########################################
# Example 9: Configuration Update
########################################
echo "=== Example 9: Overwriting existing configuration (matching despite different request body) ==="

clear_db

REQ_BODY_OLD='{"name":"old-name"}'
B64_REQ_OLD=$(echo -n "$REQ_BODY_OLD" | base64)

log_grpc "SetReply, initial config: $REQ_BODY_OLD"
grpcurl -insecure -d "{
  \"method\": \"POST\",
  \"path\": \"/users/user-1\",
  \"request_body\": \"$B64_REQ_OLD\",
  \"status_code\": 200
}" $GRPC_ADDR $SERVICE/SetReply

dump_db

REQ_BODY_NEW='{"name":"new-name"}'
B64_REQ_NEW=$(echo -n "$REQ_BODY_NEW" | base64)

log_grpc "SetReply, updated config: $REQ_BODY_NEW"
grpcurl -insecure -d "{
  \"method\": \"POST\",
  \"path\": \"/users/user-1\",
  \"request_body\": \"$B64_REQ_NEW\",
  \"status_code\": 200
}" $GRPC_ADDR $SERVICE/SetReply

dump_db

########################################
# Example 10: Extended HTTP Methods (PUT, DELETE, PATCH)
########################################
echo "=== Example 10: Extended HTTP Methods (PUT, DELETE, PATCH) ==="

clear_db

# 1. Register PUT
log_grpc "SetReply, method: PUT, path: /users/user-1"
grpcurl -insecure -d "{
  \"method\": \"PUT\",
  \"path\": \"/users/user-1\",
  \"request_body\": \"$(echo -n '{"status":"active"}' | base64)\",
  \"status_code\": 204
}" $GRPC_ADDR $SERVICE/SetReply

# 2. Register DELETE
log_grpc "SetReply, method: DELETE, path: /users/user-1"
grpcurl -insecure -d "{
  \"method\": \"DELETE\",
  \"path\": \"/users/user-1\",
  \"status_code\": 200,
  \"response_body\": \"$(echo -n '{"deleted":true}' | base64)\"
}" $GRPC_ADDR $SERVICE/SetReply

# 3. Register PATCH
log_grpc "SetReply, method: PATCH, path: /users/user-1"
grpcurl -insecure -d "{
  \"method\": \"PATCH\",
  \"path\": \"/users/user-1\",
  \"status_code\": 200
}" $GRPC_ADDR $SERVICE/SetReply

dump_db

echo "Test: Expect 204 No Content for PUT"
curl -i -X PUT $HTTP_ADDR/users/user-1 -d '{"status":"active"}'
echo

echo "Test: Expect 200 OK for DELETE"
curl -i -X DELETE $HTTP_ADDR/users/user-1
echo

echo "Test: Expect 200 OK for PATCH"
curl -i -X PATCH $HTTP_ADDR/users/user-1
echo

########################################
# Final configuration and received requests
########################################
echo "=== Final Database State ==="
dump_db

log_grpc "ListRequests"
grpcurl -insecure -d '{}' $GRPC_ADDR $SERVICE/ListRequests
