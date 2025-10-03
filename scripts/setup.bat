@echo off

go version >nul 2>&1
if %errorlevel% neq 0 (
    echo Go is not installed
    exit /b 1
)

docker --version >nul 2>&1
if %errorlevel% neq 0 (
    echo Docker is not available
    exit /b 1
)

docker-compose --version >nul 2>&1
if %errorlevel% neq 0 (
    echo Docker Compose is not available
    exit /b 1
)

go mod tidy
if %errorlevel% neq 0 (
    echo Failed to download dependencies
    exit /b 1
)

if not exist .env (
    copy .env.example .env
)

docker-compose up -d postgres redis mock-sms-api
if %errorlevel% neq 0 (
    echo Failed to start dependencies
    exit /b 1
)

timeout /t 10 /nobreak >nul

go run cmd/migrate/main.go
if %errorlevel% neq 0 (
    echo Failed to run migrations
    exit /b 1
)

echo Setup complete. Start with: go run cmd/server/main.go