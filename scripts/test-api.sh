#!/bin/bash

BASE_URL="http://localhost:8080"

test_endpoint() {
    local method=$1
    local endpoint=$2
    local expected=$3
    
    response=$(curl -s -w "%{http_code}" -X $method "$BASE_URL$endpoint")
    http_code="${response: -3}"
    body="${response%???}"
    
    if [ "$http_code" = "$expected" ]; then
        echo "OK: $method $endpoint ($http_code)"
        echo "$body" | jq -c . 2>/dev/null || echo "$body"
    else
        echo "FAIL: $method $endpoint (expected $expected, got $http_code)"
    fi
}

if ! curl -s "$BASE_URL/health" > /dev/null; then
    echo "Service not running. Start with: go run cmd/server/main.go"
    exit 1
fi

docker-compose exec -T postgres psql -U postgres -d messages_db -c "INSERT INTO messages (phone_number, content) VALUES ('+1555000001', 'Test message 1'), ('+1555000002', 'Test message 2') ON CONFLICT DO NOTHING;" 2>/dev/null

test_endpoint "GET" "/health" "200"
test_endpoint "POST" "/api/messaging/start" "200"
sleep 3
test_endpoint "GET" "/api/messages/sent" "200"
test_endpoint "POST" "/api/messaging/stop" "200"
test_endpoint "POST" "/api/messaging/start" "400"
test_endpoint "GET" "/api/nonexistent" "404"

echo "Tests completed"