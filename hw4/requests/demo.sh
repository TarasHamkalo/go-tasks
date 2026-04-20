set -e

GRPC_ADDR="localhost:8081"
HTTP_ADDR="http://localhost:8080"
SERVICE="ManagementService"

echo "== Clear database =="
grpcurl -insecure -d '{}' $GRPC_ADDR $SERVICE/ClearDatabase

########################################
# Example 1: GET /users
########################################
echo "== Example 1: GET /users =="

BODY='["user-1","user-2"]'
B64_BODY=$(echo -n "$BODY" | base64)

grpcurl -insecure \
  -d '{
    "method": "GET",
    "path": "/users",
    "status_code": 200,
    "response_body": '\"$B64_BODY\"'
  }' \
  $GRPC_ADDR \
  $SERVICE/SetReply

echo "-- Dump database --"
grpcurl -insecure $GRPC_ADDR $SERVICE/DumpDatabase

echo "-- HTTP call /users --"
curl -i $HTTP_ADDR/users

echo "-- HTTP call to /invalid --"
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

echo "-- HTTP call (match) --"
curl -i -X POST $HTTP_ADDR/users/user-1 -d "$REQ_BODY"
echo -e "\n"

echo "-- HTTP call (mismatch → 404) --"
curl -i -X POST $HTTP_ADDR/users/user-1 -d '{"name":"wrong"}'
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

echo "-- HTTP call (reordered query) --"
curl -i "$HTTP_ADDR/test?name=Surname&name=Nickname&name=Name"
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

echo "-- HTTP call (bbb → should be 404) --"
curl -i -X POST $HTTP_ADDR/test -d "bbb"
echo -e "\n"

########################################
# Debug helpers
########################################
echo "== DumpDatabase =="
grpcurl -insecure -d '{}' $GRPC_ADDR $SERVICE/DumpDatabase

echo -e "\n== ListRequests =="
grpcurl -insecure -d '{}' $GRPC_ADDR $SERVICE/ListRequests
