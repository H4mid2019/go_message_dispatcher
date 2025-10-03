# PowerShell API Test Script for Message Dispatcher Service
# This script tests all the main API endpoints

param(
    [string]$BaseUrl = "http://localhost:8080"
)

# Function to write colored output
function Write-ColorOutput {
    param(
        [string]$Message,
        [string]$Color = "White"
    )
    
    switch ($Color) {
        "Red" { Write-Host $Message -ForegroundColor Red }
        "Green" { Write-Host $Message -ForegroundColor Green }
        "Yellow" { Write-Host $Message -ForegroundColor Yellow }
        "Cyan" { Write-Host $Message -ForegroundColor Cyan }
        default { Write-Host $Message }
    }
}

# Function to test API endpoint
function Test-Endpoint {
    param(
        [string]$Method,
        [string]$Endpoint,
        [string]$Description,
        [int]$ExpectedStatus = 200
    )
    
    Write-ColorOutput "`nTesting: $Description" "Yellow"
    Write-Host "Endpoint: $Method $BaseUrl$Endpoint"
    
    try {
        $response = Invoke-RestMethod -Uri "$BaseUrl$Endpoint" -Method $Method -ContentType "application/json" -StatusCodeVariable httpCode
        
        if ($httpCode -eq $ExpectedStatus) {
            Write-ColorOutput "✓ Success ($httpCode)" "Green"
            $response | ConvertTo-Json -Depth 3 | Write-Host
        } else {
            Write-ColorOutput "✗ Failed (Expected: $ExpectedStatus, Got: $httpCode)" "Red"
            $response | ConvertTo-Json -Depth 3 | Write-Host
        }
    }
    catch {
        $statusCode = $_.Exception.Response.StatusCode.value__
        if ($statusCode -eq $ExpectedStatus) {
            Write-ColorOutput "✓ Success ($statusCode)" "Green"
        } else {
            Write-ColorOutput "✗ Failed (Expected: $ExpectedStatus, Got: $statusCode)" "Red"
        }
        
        if ($_.Exception.Response) {
            $reader = New-Object System.IO.StreamReader($_.Exception.Response.GetResponseStream())
            $responseBody = $reader.ReadToEnd()
            $reader.Close()
            try {
                $responseBody | ConvertFrom-Json | ConvertTo-Json -Depth 3 | Write-Host
            } catch {
                Write-Host $responseBody
            }
        }
    }
}

# Function to check if service is running
function Test-ServiceRunning {
    Write-ColorOutput "🔍 Checking if service is running..." "Cyan"
    
    try {
        $response = Invoke-RestMethod -Uri "$BaseUrl/health" -Method GET -TimeoutSec 5
        Write-ColorOutput "✓ Service is running" "Green"
        return $true
    }
    catch {
        Write-ColorOutput "✗ Service is not running" "Red"
        Write-Host "Please start the service first with: go run cmd/server/main.go"
        return $false
    }
}

# Function to add sample messages
function Add-SampleMessages {
    Write-ColorOutput "`nAdding sample messages to database..." "Yellow"
    
    try {
        $dockerComposeExists = Get-Command docker-compose -ErrorAction SilentlyContinue
        if ($dockerComposeExists) {
            $sql = @"
INSERT INTO messages (phone_number, content) VALUES 
('+1555000001', 'Test message from PowerShell API test script 1'),
('+1555000002', 'Test message from PowerShell API test script 2'),
('+1555000003', 'Test message from PowerShell API test script 3'),
('+1555000004', 'Test message from PowerShell API test script 4')
ON CONFLICT DO NOTHING;
"@
            
            docker-compose exec -T postgres psql -U postgres -d messages_db -c $sql 2>$null
            if ($LASTEXITCODE -eq 0) {
                Write-ColorOutput "✓ Sample messages added" "Green"
            } else {
                Write-ColorOutput "⚠ Could not add sample messages (database might not be available)" "Yellow"
            }
        } else {
            Write-ColorOutput "⚠ Docker Compose not available, skipping sample data" "Yellow"
        }
    }
    catch {
        Write-ColorOutput "⚠ Could not add sample messages: $($_.Exception.Message)" "Yellow"
    }
}

# Main test execution
function Main {
    Write-ColorOutput "🚀 Message Dispatcher API Test Suite" "Cyan"
    Write-ColorOutput "======================================" "Cyan"
    
    # Check if service is running
    if (-not (Test-ServiceRunning)) {
        exit 1
    }
    
    # Add sample messages for testing
    Add-SampleMessages
    
    # Test all endpoints
    Test-Endpoint "GET" "/health" "Health Check" 200
    Test-Endpoint "POST" "/api/messaging/start" "Start Message Processing" 200
    
    Write-ColorOutput "`nWaiting 5 seconds for processing to begin..." "Yellow"
    Start-Sleep -Seconds 5
    
    Test-Endpoint "GET" "/api/messages/sent" "List Sent Messages" 200
    Test-Endpoint "POST" "/api/messaging/stop" "Stop Message Processing" 200
    
    # Test error cases
    Write-ColorOutput "`nTesting error cases..." "Yellow"
    Test-Endpoint "POST" "/api/messaging/start" "Start Again (Should Fail)" 400
    Test-Endpoint "POST" "/api/messaging/stop" "Stop Again (Should Fail)" 400
    
    # Test non-existent endpoints
    Test-Endpoint "GET" "/api/nonexistent" "Non-existent Endpoint" 404
    
    Write-ColorOutput "`n🎉 API test suite completed!" "Green"
}

# Run the tests
Main