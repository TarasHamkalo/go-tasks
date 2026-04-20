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

echo "== Clear database =="
grpcurl -insecure -d '{}' $GRPC_ADDR $SERVICE/ClearDatabase

########################################
# Example 1: GET /users
########################################
echo "== Example 1: GET /users =="

BODY='["user-1","user-2"]'
B64_BODY=$(echo -n "$BODY" | base64)

grpcurl -insecure \
  -d "{
    \"method\": \"GET\",
    \"path\": \"/users\",
    \"status_code\": 200,
    \"response_body\": \"$B64_BODY\"
  }" \
  $GRPC_ADDR \
  $SERVICE/SetReply

echo "-- Dump database --"
grpcurl -insecure $GRPC_ADDR $SERVICE/DumpDatabase
echo -e "\n"

echo "-- HTTP call /users (should match, 200) --"
log_http "GET" "/users"
curl -i $HTTP_ADDR/users
echo -e "\n"

echo "-- HTTP call /invalid (path not configured, 404) --"
log_http "GET" "/invalid"
curl -i $HTTP_ADDR/invalid

echo -e "\n"

########################################
# Example 2: POST /users/user-1 (body match)
########################################
echo "== Example 2: POST /users/user-1 =="

REQ_BODY='{"name":"user-name"}'
B64_REQ=$(echo -n "$REQ_BODY" | base64)

grpcurl -insecure \
  -d "{
    \"method\": \"POST\",
    \"path\": \"/users/user-1\",
    \"request_body\": \"$B64_REQ\",
    \"status_code\": 200
  }" \
  $GRPC_ADDR \
  $SERVICE/SetReply

echo "-- Dump database --"
grpcurl -insecure $GRPC_ADDR $SERVICE/DumpDatabase
echo -e "\n"
echo "-- HTTP call (req body match, 200) --"
log_http "POST" "/users/user-1" "$REQ_BODY"
curl -i -X POST $HTTP_ADDR/users/user-1 -d "$REQ_BODY"
echo -e "\n"

echo "-- HTTP call (req body mismatch, 404) --"
BAD_BODY='{"name":"wrong"}'
log_http "POST" "/users/user-1" "$BAD_BODY"
curl -i -X POST $HTTP_ADDR/users/user-1 -d "$BAD_BODY"
echo -e "\n"

########################################
# Example 3: Query order ignored
########################################
echo "== Example 3: Query order =="

RESP_BODY='{"ok": true}'
B64_RESP=$(echo -n "$RESP_BODY" | base64)

grpcurl -insecure \
  -d "{
    \"method\": \"GET\",
    \"path\": \"/test?name=Name&name=Surname&name=Nickname\",
    \"status_code\": 200,
    \"response_body\": \"$B64_RESP\"
  }" \
  $GRPC_ADDR \
  $SERVICE/SetReply

echo "-- Dump database --"
grpcurl -insecure $GRPC_ADDR $SERVICE/DumpDatabase
echo -e "\n"

QUERY="/test?name=Surname&name=Nickname&name=Name"
echo "-- HTTP call (reordered query, match, 200) --"

echo -e "Registered: /test?name=Name&name=Surname&name=Nickname",
echo -e "\n"

log_http "GET" "$QUERY"
curl -i "$HTTP_ADDR$QUERY"
echo -e "\n"

QUERY="/test?name=NotName&name=Nickname&name=Name"
echo "-- HTTP call (name=NotName, mismatch, 404) --"
log_http "GET" "$QUERY"
curl -i "$HTTP_ADDR$QUERY"
echo -e "\n"

########################################
# Example 4: Body mismatch
########################################
echo "== Example 4: POST mismatch =="

REQ_BODY='aaa'
B64_REQ=$(echo -n "$REQ_BODY" | base64)

grpcurl -insecure \
  -d "{
    \"method\": \"POST\",
    \"path\": \"/test\",
    \"request_body\": \"$B64_REQ\",
    \"status_code\": 200
  }" \
  $GRPC_ADDR \
  $SERVICE/SetReply

echo "-- Dump database --"
grpcurl -insecure $GRPC_ADDR $SERVICE/DumpDatabase
echo -e "\n"

echo "-- HTTP call (body mismatch, 404) --"
log_http "POST" "/test" "bbb"
curl -i -X POST $HTTP_ADDR/test -d "bbb"
echo -e "\n"


########################################
# GRPC endpoints
########################################
echo "== DumpDatabase =="
grpcurl -insecure -d '{}' $GRPC_ADDR $SERVICE/DumpDatabase

echo -e "\n== ListRequests =="
grpcurl -insecure -d '{}' $GRPC_ADDR $SERVICE/ListRequests
