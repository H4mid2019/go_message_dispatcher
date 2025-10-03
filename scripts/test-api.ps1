param([string]$BaseUrl = "http://localhost:8080")

function Test-Endpoint($Method, $Endpoint, $ExpectedStatus = 200) {
    try {
        $response = Invoke-RestMethod -Uri "$BaseUrl$Endpoint" -Method $Method -StatusCodeVariable httpCode
        if ($httpCode -eq $ExpectedStatus) {
            Write-Output "OK: $Method $Endpoint ($httpCode)"
            $response | ConvertTo-Json -Compress
        } else {
            Write-Output "FAIL: $Method $Endpoint (expected $ExpectedStatus, got $httpCode)"
        }
    }
    catch {
        $statusCode = $_.Exception.Response.StatusCode.value__
        if ($statusCode -eq $ExpectedStatus) {
            Write-Output "OK: $Method $Endpoint ($statusCode)"
        } else {
            Write-Output "FAIL: $Method $Endpoint (expected $ExpectedStatus, got $statusCode)"
        }
    }
}

try {
    Invoke-RestMethod -Uri "$BaseUrl/health" -TimeoutSec 5 | Out-Null
} catch {
    Write-Output "Service not running. Start with: go run cmd/server/main.go"
    exit 1
}

$sql = "INSERT INTO messages (phone_number, content) VALUES ('+1555000001', 'Test message 1'), ('+1555000002', 'Test message 2') ON CONFLICT DO NOTHING;"
docker-compose exec -T postgres psql -U postgres -d messages_db -c $sql 2>$null

Test-Endpoint "GET" "/health"
Test-Endpoint "POST" "/api/messaging/start"
Start-Sleep 3
Test-Endpoint "GET" "/api/messages/sent"
Test-Endpoint "POST" "/api/messaging/stop"
Test-Endpoint "POST" "/api/messaging/start" 400
Test-Endpoint "GET" "/api/nonexistent" 404

Write-Output "Tests completed"