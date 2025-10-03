#!/bin/bash

# API Test Script for Message Dispatcher Service
# This script tests all the main API endpoints

BASE_URL="http://localhost:8080"

echo "🚀 Message Dispatcher API Test Suite"
echo "======================================"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to test API endpoint
test_endpoint() {
    local method=$1
    local endpoint=$2
    local description=$3
    local expected_status=$4
    
    echo -e "\n${YELLOW}Testing: $description${NC}"
    echo "Endpoint: $method $endpoint"
    
    response=$(curl -s -w "\n%{http_code}" -X $method "$BASE_URL$endpoint" -H "Content-Type: application/json")
    http_code=$(echo "$response" | tail -n1)
    body=$(echo "$response" | head -n -1)
    
    if [ "$http_code" = "$expected_status" ]; then
        echo -e "${GREEN}✓ Success ($http_code)${NC}"
        echo "$body" | jq . 2>/dev/null || echo "$body"
    else
        echo -e "${RED}✗ Failed (Expected: $expected_status, Got: $http_code)${NC}"
        echo "$body"
    fi
}

# Function to check if service is running
check_service() {
    echo "🔍 Checking if service is running..."
    
    if curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        echo -e "${GREEN}✓ Service is running${NC}"
        return 0
    else
        echo -e "${RED}✗ Service is not running${NC}"
        echo "Please start the service first with: make run"
        return 1
    fi
}

# Function to add sample messages
add_sample_messages() {
    echo -e "\n${YELLOW}Adding sample messages to database...${NC}"
    
    # Check if we can connect to database
    if command -v docker-compose &> /dev/null; then
        docker-compose exec -T postgres psql -U postgres -d messages_db -c "
            INSERT INTO messages (phone_number, content) VALUES 
            ('+1555000001', 'Test message from API test script 1'),
            ('+1555000002', 'Test message from API test script 2'),
            ('+1555000003', 'Test message from API test script 3'),
            ('+1555000004', 'Test message from API test script 4')
            ON CONFLICT DO NOTHING;
        " 2>/dev/null
        
        if [ $? -eq 0 ]; then
            echo -e "${GREEN}✓ Sample messages added${NC}"
        else
            echo -e "${YELLOW}⚠ Could not add sample messages (database might not be available)${NC}"
        fi
    else
        echo -e "${YELLOW}⚠ Docker Compose not available, skipping sample data${NC}"
    fi
}

# Main test execution
main() {
    echo "Starting API tests..."
    
    # Check if service is running
    if ! check_service; then
        exit 1
    fi
    
    # Add sample messages for testing
    add_sample_messages
    
    # Test all endpoints
    test_endpoint "GET" "/health" "Health Check" "200"
    test_endpoint "POST" "/api/messaging/start" "Start Message Processing" "200"
    
    echo -e "\n${YELLOW}Waiting 5 seconds for processing to begin...${NC}"
    sleep 5
    
    test_endpoint "GET" "/api/messages/sent" "List Sent Messages" "200"
    test_endpoint "POST" "/api/messaging/stop" "Stop Message Processing" "200"
    
    # Test error cases
    echo -e "\n${YELLOW}Testing error cases...${NC}"
    test_endpoint "POST" "/api/messaging/start" "Start Again (Should Fail)" "400"
    test_endpoint "POST" "/api/messaging/stop" "Stop Again (Should Fail)" "400"
    
    # Test non-existent endpoints
    test_endpoint "GET" "/api/nonexistent" "Non-existent Endpoint" "404"
    
    echo -e "\n🎉 ${GREEN}API test suite completed!${NC}"
}

# Run the tests
main