set -e

GRPC_ADDR="localhost:8081"
HTTP_ADDR="http://localhost:8080"
SERVICE="ManagementService"

log_http() {
  local method=$1
  local path=$2
  local body=$3

  if [ -z "$body" ]; then
    echo ">> HTTP REQUEST: $method $path"
  else
    echo ">> HTTP REQUEST: $method $path body=$body"
  fi
}

log_grpc() {
  echo ">> GRPC CALL: $1"
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
# Example 1: No config → 501
########################################
echo "=============================="
echo "Example 1: No config → 501"
echo "=============================="

clear_db

echo "-- HTTP call (expect 501) --"
log_http "GET" "/users"
curl -i $HTTP_ADDR/users
echo

########################################
# Example 2: GET /users
########################################
echo "=============================="
echo "Example 2: GET /users"
echo "=============================="

clear_db

BODY='["user-1","user-2"]'
B64_BODY=$(echo -n "$BODY" | base64)

log_grpc "SetReply GET /users"
grpcurl -insecure -d "{
  \"method\": \"GET\",
  \"path\": \"/users\",
  \"status_code\": 200,
  \"response_body\": \"$B64_BODY\"
}" $GRPC_ADDR $SERVICE/SetReply

dump_db

echo "-- HTTP call (match → 200) --"
log_http "GET" "/users"
curl -i $HTTP_ADDR/users
echo

echo "-- HTTP call (unknown path → 404) --"
log_http "GET" "/invalid"
curl -i $HTTP_ADDR/invalid
echo

########################################
# Example 3: POST with body match/mismatch
########################################
echo "=============================="
echo "Example 3: POST body match"
echo "=============================="

clear_db

REQ_BODY='{"name":"user-name"}'
B64_REQ=$(echo -n "$REQ_BODY" | base64)

log_grpc "SetReply POST /users/user-1 (with body=$REQ_BODY, in request base64 encoded)"
grpcurl -insecure -d "{
  \"method\": \"POST\",
  \"path\": \"/users/user-1\",
  \"request_body\": \"$B64_REQ\",
  \"status_code\": 200
}" $GRPC_ADDR $SERVICE/SetReply

dump_db

echo "-- HTTP call (match → 200) --"
log_http "POST" "/users/user-1" "$REQ_BODY"
curl -i -X POST $HTTP_ADDR/users/user-1 -d "$REQ_BODY"
echo

echo "-- HTTP call (body mismatch → 404) --"
BAD_BODY='{"name":"wrong"}'
log_http "POST" "/users/user-1" "$BAD_BODY"
curl -i -X POST $HTTP_ADDR/users/user-1 -d "$BAD_BODY"
echo

########################################
# Example 4: Query order ignored
########################################
echo "=============================="
echo "Example 4: Query order"
echo "=============================="

clear_db

RESP_BODY='{"ok": true}'
B64_RESP=$(echo -n "$RESP_BODY" | base64)

log_grpc "SetReply GET /test?name=Name&name=Surname&name=Nickname"
grpcurl -insecure -d "{
  \"method\": \"GET\",
  \"path\": \"/test?name=Name&name=Surname&name=Nickname\",
  \"status_code\": 200,
  \"response_body\": \"$B64_RESP\"
}" $GRPC_ADDR $SERVICE/SetReply

dump_db

echo "-- HTTP call (reordered → 200) --"
QUERY="/test?name=Surname&name=Nickname&name=Name"
log_http "GET" "$QUERY"
curl -i "$HTTP_ADDR$QUERY"
echo

echo "-- HTTP call (value mismatch → 404) --"
QUERY="/test?name=NotName&name=Nickname&name=Name"
log_http "GET" "$QUERY"
curl -i "$HTTP_ADDR$QUERY"
echo

########################################
# Example 5: Body mismatch
########################################
echo "=============================="
echo "Example 5: Body mismatch"
echo "=============================="

clear_db

REQ_BODY='aaa'
B64_REQ=$(echo -n "$REQ_BODY" | base64)

log_grpc "SetReply POST /test (body=$REQ_BODY)"
grpcurl -insecure -d "{
  \"method\": \"POST\",
  \"path\": \"/test\",
  \"request_body\": \"$B64_REQ\",
  \"status_code\": 200
}" $GRPC_ADDR $SERVICE/SetReply

dump_db

echo "-- HTTP call (bbb → 404) --"
log_http "POST" "/test" "bbb"
curl -i -X POST $HTTP_ADDR/test -d "bbb"
echo

########################################
# Example 6: Duplicate query params
########################################
echo "=============================="
echo "Example 6: Duplicate query params"
echo "=============================="

clear_db

RESP_BODY='ok'
B64_RESP=$(echo -n "$RESP_BODY" | base64)

log_grpc "SetReply GET /test?name=A&name=A"
grpcurl -insecure -d "{
  \"method\": \"GET\",
  \"path\": \"/test?name=A&name=A\",
  \"status_code\": 200,
  \"response_body\": \"$B64_RESP\"
}" $GRPC_ADDR $SERVICE/SetReply

dump_db

echo "-- HTTP call (exact match → 200) --"
QUERY="/test?name=A&name=A"
log_http "GET" "$QUERY"
curl -i "$HTTP_ADDR$QUERY"
echo

echo "-- HTTP call (missing one value → 404) --"
QUERY="/test?name=A"
log_http "GET" "$QUERY"
curl -i "$HTTP_ADDR$QUERY"
echo

########################################
# Example 7: Unsupported method → 405
########################################
echo "=============================="
echo "Example 7: Method not allowed"
echo "=============================="

clear_db

BODY='["user-1"]'
B64_BODY=$(echo -n "$BODY" | base64)

log_grpc "SetReply GET /users"
grpcurl -insecure -d "{
  \"method\": \"GET\",
  \"path\": \"/users\",
  \"status_code\": 200,
  \"response_body\": \"$B64_BODY\"
}" $GRPC_ADDR $SERVICE/SetReply

dump_db

echo "-- HTTP call DELETE /users (→ 405) --"
log_http "DELETE" "/users"
curl -i -X DELETE $HTTP_ADDR/users
echo

########################################
# List requests, dump db
########################################
echo "=============================="
echo "List requests, dump db"
echo "=============================="

log_grpc "DumpDatabase"
grpcurl -insecure -d '{}' $GRPC_ADDR $SERVICE/DumpDatabase

echo
log_grpc "ListRequests"
grpcurl -insecure -d '{}' $GRPC_ADDR $SERVICE/ListRequests
