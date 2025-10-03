@echo off
REM Setup script for Message Dispatcher Service on Windows
REM This script helps set up the development environment

echo.
echo ========================================
echo Message Dispatcher Service Setup
echo ========================================

REM Check if Go is installed
echo Checking Go installation...
go version >nul 2>&1
if %errorlevel% neq 0 (
    echo [ERROR] Go is not installed or not in PATH
    echo Please install Go 1.21+ from https://golang.org/dl/
    pause
    exit /b 1
)
echo [OK] Go is installed

REM Check if Docker is installed
echo Checking Docker installation...
docker --version >nul 2>&1
if %errorlevel% neq 0 (
    echo [ERROR] Docker is not installed or not running
    echo Please install Docker Desktop from https://www.docker.com/products/docker-desktop
    pause
    exit /b 1
)
echo [OK] Docker is available

REM Check if Docker Compose is available
echo Checking Docker Compose...
docker-compose --version >nul 2>&1
if %errorlevel% neq 0 (
    echo [ERROR] Docker Compose is not available
    echo Please ensure Docker Desktop includes Docker Compose
    pause
    exit /b 1
)
echo [OK] Docker Compose is available

REM Download Go dependencies
echo.
echo Downloading Go dependencies...
go mod tidy
if %errorlevel% neq 0 (
    echo [ERROR] Failed to download Go dependencies
    pause
    exit /b 1
)
echo [OK] Dependencies downloaded

REM Create .env file if it doesn't exist
if not exist .env (
    echo Creating .env file from template...
    copy .env.example .env
    echo [OK] .env file created
) else (
    echo [OK] .env file already exists
)

REM Start dependencies
echo.
echo Starting PostgreSQL and Redis dependencies...
docker-compose up -d postgres redis mock-sms-api
if %errorlevel% neq 0 (
    echo [ERROR] Failed to start dependencies
    pause
    exit /b 1
)

REM Wait for services to be ready
echo Waiting for services to be ready...
timeout /t 10 /nobreak >nul

REM Run database migrations
echo.
echo Running database migrations...
go run cmd/migrate/main.go
if %errorlevel% neq 0 (
    echo [ERROR] Failed to run migrations
    pause
    exit /b 1
)
echo [OK] Database migrations completed

echo.
echo ========================================
echo Setup completed successfully!
echo ========================================
echo.
echo Next steps:
echo 1. Start the service: go run cmd/server/main.go
echo 2. Test the API: scripts\test-api.bat
echo 3. View logs: docker-compose logs -f
echo.
echo API Endpoints:
echo - Health: http://localhost:8080/health
echo - Start:  http://localhost:8080/api/messaging/start
echo - Stop:   http://localhost:8080/api/messaging/stop
echo - List:   http://localhost:8080/api/messages/sent
echo.
pause