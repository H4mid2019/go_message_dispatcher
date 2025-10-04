# Automated Message Sending System

A production-ready Golang service that automatically sends SMS messages from a PostgreSQL queue with Redis caching and REST API controls.

## 🏗️ Architecture Overview

This system implements a background message processor that:

- Polls the database every 2 minutes for unsent messages
- Sends exactly 2 messages per batch in FIFO order
- Tracks delivery status to prevent duplicates
- Provides REST API for control and monitoring
- Caches successful deliveries in Redis for performance

## 📁 Project Structure

```
go_message_dispatcher/
├── cmd/                    # Application entry points
│   ├── migrate/           # Database migration tool
│   └── server/            # Main HTTP server
├── internal/              # Private application code
│   ├── config/           # Configuration management
│   ├── domain/           # Business entities and interfaces
│   ├── handler/          # HTTP request handlers
│   ├── repository/       # Data access implementations
│   ├── scheduler/        # Background processing
│   └── service/          # Business logic services
├── migrations/           # Database schema migrations
├── docker-compose.yml   # Local development environment
├── Dockerfile          # Production container build
├── Makefile           # Development workflow commands
└── README.md          # Project documentation
```

### System Architecture Diagram

```text
┌─────────────────────┐    ┌──────────────────────┐    ┌─────────────────────┐
│    PostgreSQL       │    │   Message Dispatcher │    │   SMS API Provider  │
│   Database          │◄───│     Service          │───►│   (Real/Mock)       │
│   (Port 5432)       │    │   (Port 8080)        │    │   (Port 3001)       │
│                     │    │                      │    │                     │
│ • Stores messages   │    │ • Background polling │    │ • Sends SMS messages│
│ • FIFO queue order  │    │ • 2 messages/batch   │    │ • Returns message ID│
│ • Tracks sent status│    │ • Every 2 minutes    │    │ • Mock for testing  │
│ • Auto-increment ID │    │ • REST API control   │    │ • Real for production│
└─────────────────────┘    │ • Graceful shutdown  │    └─────────────────────┘
         ▲                 │ • Error handling     │
         │                 └──────────┬───────────┘
         │                            │
         │                            ▼
         │                 ┌──────────────────────┐
         │                 │       Redis          │
         └─────────────────│     Cache            │
                           │   (Port 6379)        │
                           │                      │
                           │ • Caches sent data   │
                           │ • Enhanced responses │
                           │ • Performance boost  │
                           │ • Delivery metadata  │
                           └──────────────────────┘

┌─────────────────────────────────────────────────────────────────────────────┐
│                              REST API Endpoints                             │
├─────────────────────────────────────────────────────────────────────────────┤
│ GET  /health                    │ System health check                       │
│ POST /api/messaging/start       │ Start message processing                  │
│ POST /api/messaging/stop        │ Stop message processing                   │
│ GET  /api/messages/sent         │ List sent messages (with Redis cache)    │
└─────────────────────────────────────────────────────────────────────────────┘

Data Flow:
1. 📝 Messages inserted into PostgreSQL database
2. 🔄 Background processor polls database every 2 minutes
3. 📤 Sends 2 messages per batch to SMS API (FIFO order)
4. ✅ Updates database with sent status
5. 💾 Caches delivery metadata in Redis
6. 🔍 API queries return enhanced data from cache + database
```

## ✨ Features

- **Automated Processing**: Background goroutine processes messages every 2 minutes
- **FIFO Queue**: Messages processed in creation order
- **Graceful Shutdown**: Proper cleanup of resources and in-flight operations
- **REST API**: Start/stop controls and sent message listing
- **Redis Integration**: Caches delivery metadata for enhanced responses
- **Comprehensive Logging**: Structured logging with contextual information
- **Error Resilience**: Robust error handling with exponential backoff
- **Docker Support**: Complete containerization with docker-compose
- **🆕 Distributed Locking**: Multi-instance deployment support with Redis-based locking (Tier 2)
- **🆕 Container Resilience**: Enhanced health checks, connection retry, Docker health monitoring (Tier 1)
- **🆕 Production Ready**: Auto-recovery from transient failures, graceful shutdown, comprehensive error handling

## 🎯 Production-Ready Enhancements

This system includes enterprise-grade features for production deployment:

### Tier 1: Container Resilience ✅
- **Enhanced Health Checks**: Verifies DB and Redis connectivity with detailed status reporting
- **Connection Retry Logic**: Exponential backoff (5 attempts) for database and Redis connections
- **Docker Health Monitoring**: Automatic health checks and container restart on failure
- **Auto-Recovery**: Handles transient network issues and temporary service outages

### Tier 2: Multi-Instance Support ✅
- **Distributed Locking**: Redis-based coordination for running multiple instances
- **High Availability**: Automatic failover if one instance crashes
- **Zero Downtime Deployments**: Rolling updates without message processing interruption
- **Lock Auto-Extension**: Prevents lock expiration during long processing operations

### Core Features
- **Exactly 2 messages per batch**: Processes 2 messages every 2 minutes (or 1 if only 1 available)
- **Indefinite Retry**: Failed messages retry in next cycle until successful
- **SSL/TLS Support**: Works with self-signed certificates, 6-second webhook timeout
- **Data Validation**: Phone (10-20 chars) and content validation at SQL level
- **Race Condition Protection**: `FOR UPDATE SKIP LOCKED` prevents concurrent processing
- **Redis Fault Tolerance**: Continues sending even if Redis cache is down
- **Graceful Shutdown**: Waits for current batch to complete before shutdown
- **Individual Message Handling**: M1 success + M2 failure = M1 stays sent, M2 retries

See [TIER2_IMPLEMENTATION.md](TIER2_IMPLEMENTATION.md) for technical details.

## 📚 Documentation

### Getting Started
- **[README.md](README.md)** - This file, overview and quick start
- **[QUICK_REFERENCE.md](QUICK_REFERENCE.md)** - Common commands and workflows
- **[QUICK_START_MULTI_INSTANCE.md](QUICK_START_MULTI_INSTANCE.md)** - Multi-instance deployment guide

### Implementation Details
- **[TIER2_IMPLEMENTATION.md](TIER2_IMPLEMENTATION.md)** - Multi-instance technical guide (10 pages)
- **[IMPLEMENTATION_SUMMARY.md](IMPLEMENTATION_SUMMARY.md)** - Technical implementation details
- **[FINAL_SUMMARY.md](FINAL_SUMMARY.md)** - Production readiness checklist

### Testing & Completion
- **[TEST_RESULTS.md](TEST_RESULTS.md)** - Test coverage and results
- **[COMPLETION_CHECKLIST.md](COMPLETION_CHECKLIST.md)** - Feature implementation checklist
- **[TASKS_COMPLETE.md](TASKS_COMPLETE.md)** - Final completion summary

## 🚀 Deployment Options

Choose the deployment method that best fits your needs:

### Option 1: 📦 Pre-built Binaries (Recommended)

**No setup required - just download and run!**

#### Option 1.1: Standalone Binaries

1. **Download the latest release** from [GitHub Releases](../../releases)

   - `message-dispatcher-server-windows-amd64.exe` (Windows)
   - `message-dispatcher-server-linux-amd64` (Linux)
   - `message-dispatcher-server-darwin-amd64` (macOS Intel)
   - `message-dispatcher-server-darwin-arm64` (macOS Apple Silicon)

2. **Set up your environment**:

   ```bash
   # Create .env file with your configuration
   DB_HOST=your-postgres-host
   DB_USER=your-db-user
   DB_PASSWORD=your-db-password
   REDIS_HOST=your-redis-host
   ```

3. **Run database migrations**:

   ```bash
   # Download and run the migrate tool
   ./message-dispatcher-migrate --version
   ./message-dispatcher-migrate
   ```

4. **Start the service**:

   ```bash
   # Check version
   ./message-dispatcher-server --version

   # Start the service
   ./message-dispatcher-server
   ```

#### Option 1.2: Docker Images

Pre-built Docker images available on Docker Hub:

```bash
# Pull the latest image
docker pull h4mid2019/message-dispatcher

# Run with your existing PostgreSQL and Redis
docker run -d \
  --name message-dispatcher \
  -p 8080:8080 \
  -e DB_HOST=your-postgres-host \
  -e DB_USER=your-db-user \
  -e DB_PASSWORD=your-db-password \
  -e REDIS_HOST=your-redis-host \
  h4mid2019/message-dispatcher

# Check logs
docker logs message-dispatcher

# Stop the container
docker stop message-dispatcher
```

**Available at:** [https://hub.docker.com/r/h4mid2019/message-dispatcher](https://hub.docker.com/r/h4mid2019/message-dispatcher)

### Option 2: 🐳 Docker (Production Ready)

Complete containerized solution with all dependencies

```bash
# Clone the repository
git clone <repository>
cd go_message_dispatcher

# Start everything with Docker Compose
docker-compose up -d

# Check logs
docker-compose logs -f message-dispatcher

# Stop the service
docker-compose down
```

**What's included:**

- PostgreSQL database with automatic schema setup
- Redis for caching
- Mock SMS API for testing (Go-based, fast startup)
- Message dispatcher service
- Automatic health checks and restarts

### Option 3: 🔧 Build from Source

For development and customization

**Prerequisites:**

- Go 1.25+
- PostgreSQL 15+ (running)
- Redis 7+ (running)

```bash
# Clone and setup
git clone <repository>
cd go_message_dispatcher
cp .env.example .env

# Install dependencies
go mod tidy

# Run database migrations
go run cmd/migrate/main.go

# Start the service
go run cmd/server/main.go

# Or build and run
make build
./bin/server
```

## 🔧 Quick Start Guide

### Automated Setup (Windows)

```powershell
# Run the setup script
.\scripts\setup.bat

# Start the service
go run cmd/server/main.go

# Test the API
.\scripts\test-api.ps1
```

### Testing Your Deployment

Regardless of your deployment method, test the API:

```bash
# Health check
curl http://localhost:8080/health

# Start processing
curl -X POST http://localhost:8080/api/messaging/start

# List sent messages
curl http://localhost:8080/api/messages/sent

# Stop processing
curl -X POST http://localhost:8080/api/messaging/stop
```

## API Documentation

### Control Endpoints

#### Start Message Processing

```http
POST /api/messaging/start
Content-Type: application/json

Response: 200 OK
{
  "status": "started",
  "message": "Message processing started successfully"
}
```

#### Stop Message Processing

```http
POST /api/messaging/stop
Content-Type: application/json

Response: 200 OK
{
  "status": "stopped",
  "message": "Message processing stopped successfully"
}
```

### Monitoring Endpoints

#### List Sent Messages

```http
GET /api/messages/sent

Response: 200 OK
{
  "messages": [
    {
      "id": 1,
      "phone_number": "+1234567890",
      "content": "Hello, this is a test message",
      "sent": true,
      "created_at": "2025-10-02T10:00:00Z",
      "message_id": "uuid-from-provider",
      "cached_at": "2025-10-02T10:01:00Z"
    }
  ],
  "total": 1
}
```

## Database Schema

```sql
CREATE TABLE messages (
    id SERIAL PRIMARY KEY,
    phone_number VARCHAR(20) NOT NULL,
    content TEXT NOT NULL,
    sent BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT NOW()
);

-- Performance indexes
CREATE INDEX idx_messages_sent_created ON messages(sent, created_at);
CREATE INDEX idx_messages_phone ON messages(phone_number);
```

## Configuration

Set these environment variables:

| Variable                    | Description                         | Default                            |
| --------------------------- | ----------------------------------- | ---------------------------------- |
| `DB_HOST`                   | PostgreSQL host                     | localhost                          |
| `DB_PORT`                   | PostgreSQL port                     | 5432                               |
| `DB_NAME`                   | Database name                       | messages_db                        |
| `DB_USER`                   | Database user                       | postgres                           |
| `DB_PASSWORD`               | Database password                   | password                           |
| `REDIS_HOST`                | Redis host                          | localhost                          |
| `REDIS_PORT`                | Redis port                          | 6379                               |
| `REDIS_PASSWORD`            | Redis password                      | ""                                 |
| `SERVER_PORT`               | HTTP server port                    | 8080                               |
| `LOG_LEVEL`                 | Log level (debug/info/warn/error)   | info                               |
| `SMS_API_URL`               | SMS provider API URL                | `http://localhost:3001/send`       |
| `SMS_API_TOKEN`             | SMS provider auth token             | mock-token                         |
| `DISTRIBUTED_LOCK_ENABLED`  | Enable distributed locking          | false                              |
| `DISTRIBUTED_LOCK_TTL`      | Lock TTL for distributed mode       | 3m                                 |
| `DISTRIBUTED_LOCK_KEY`      | Redis key for distributed lock      | message-dispatcher:lock            |

### 🚀 Multi-Instance Deployment (Tier 2)

To run multiple instances of the message dispatcher (for high availability and load distribution):

1. **Enable distributed locking**:
   ```bash
   DISTRIBUTED_LOCK_ENABLED=true
   ```

2. **Configure lock TTL** (should be longer than processing interval):
   ```bash
   DISTRIBUTED_LOCK_TTL=3m  # Default is 3 minutes (processing interval is 2 minutes)
   ```

3. **Deploy multiple instances**:
   ```bash
   # Instance 1
   docker run -d --name dispatcher-1 \
     -e DISTRIBUTED_LOCK_ENABLED=true \
     -e DB_HOST=postgres \
     -e REDIS_HOST=redis \
     h4mid2019/message-dispatcher

   # Instance 2
   docker run -d --name dispatcher-2 \
     -e DISTRIBUTED_LOCK_ENABLED=true \
     -e DB_HOST=postgres \
     -e REDIS_HOST=redis \
     h4mid2019/message-dispatcher

   # Instance 3
   docker run -d --name dispatcher-3 \
     -e DISTRIBUTED_LOCK_ENABLED=true \
     -e DB_HOST=postgres \
     -e REDIS_HOST=redis \
     h4mid2019/message-dispatcher
   ```

**How it works:**
- Only one instance processes messages at a time
- Lock is automatically acquired before processing
- Lock extends during processing to prevent expiration
- If an instance crashes, lock auto-expires and another takes over
- Each instance logs whether it acquired the lock or skipped the cycle

**Benefits:**
- ✅ **High Availability**: If one instance fails, others continue
- ✅ **Zero Downtime Deployments**: Rolling updates without message loss
- ✅ **Load Distribution**: Instances share the processing workload
- ✅ **Fault Tolerance**: Automatic failover between instances

## Testing

Run all tests:

```bash
go test ./...
```

Run tests with coverage:

```bash
go test -race -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

Run integration tests:

```bash
go test -tags=integration ./...
```

## Monitoring & Health Checks

- Health endpoint: `GET /health`
- Metrics endpoint: `GET /metrics`
- Service logs via structured JSON logging

## Development Setup

### Required Tools

```powershell
# Install Go formatting and linting tools
go install golang.org/x/tools/cmd/goimports@latest
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
```

### VS Code Configuration

The project includes VS Code settings that will:

- ✅ **Format on save** with `goimports`
- ✅ **Lint on save** with `golangci-lint`
- ✅ **Organize imports** automatically
- ✅ **Run vet checks** on save
- ✅ **Build checks** on save

### Pre-commit Hooks

Git hooks automatically run before each commit:

- 🔧 Code formatting
- 🔍 Linting checks
- 🧪 All tests
- 🏗️ Build verification

## Code Quality

### Manual Commands

```powershell
# Format code
goimports -w .

# Run linter
golangci-lint run

# Run linter with auto-fix
golangci-lint run --fix

# Run specific VS Code tasks
# Ctrl+Shift+P -> "Tasks: Run Task"
# - go: build
# - go: test
# - go: lint
# - go: format
```

### Quick Quality Checks

**Check everything at once:**

```bash
# Bash/Linux/macOS
gofmt -s -l . && goimports -l . && go vet ./... && golangci-lint run
```

```powershell
# PowerShell/Windows
gofmt -s -l .; if ($LASTEXITCODE -eq 0) { goimports -l . }; if ($LASTEXITCODE -eq 0) { go vet ./... }; if ($LASTEXITCODE -eq 0) { golangci-lint run }
```

**Fix formatting issues:**

```bash
# Bash/Linux/macOS
gofmt -s -w . && goimports -w .
```

```powershell
# PowerShell/Windows
gofmt -s -w .; goimports -w .
```

## Development Notes

### Code Organization

- `cmd/`: Application entry points
- `internal/`: Private application code
- `pkg/`: Public library code
- `migrations/`: Database migration files

### Design Decisions

- **Goroutine-based scheduler**: No external cron dependencies for better control
- **Graceful shutdown**: Ensures in-flight messages complete before shutdown
- **Interface-driven design**: Enables easy testing and mocking
- **Structured logging**: JSON logs for better observability
- **Error wrapping**: Maintains error context throughout the call stack

### Production Considerations

- Connection pooling configured for high throughput
- Circuit breaker pattern for external API calls
- Retry logic with exponential backoff
- Resource cleanup and connection management
- Comprehensive error handling and logging

## 🏗️ Building and Releases

### Local Development Build

```bash
# Build with version information
make build

# Build for all platforms
make build-all

# Check version
./bin/server --version
./bin/migrate --version
```

### Cross-Platform Builds

```bash
# Build for Windows
make build-windows

# Build for Linux
make build-linux

# Build for macOS (Intel and Apple Silicon)
make build-darwin
```

### Creating Releases

The project uses GitHub Actions for automated building and releasing:

1. **Create a release** (triggers automated build):

   ```bash
   # Using the release script
   ./scripts/release.ps1 v1.0.0

   # Or using git directly
   git tag -a v1.0.0 -m "Release v1.0.0"
   git push origin v1.0.0
   ```

2. **GitHub Actions will automatically**:

   - Run all tests
   - Build binaries for Windows, Linux, and macOS (Intel + Apple Silicon)
   - Create a GitHub release with downloadable artifacts
   - Build and push Docker images

3. **Download pre-built binaries** from the [Releases page](../../releases)

### Version Information

All binaries include version information accessible via:

- `--version` flag: `./message-dispatcher-server --version`
- API endpoint: `GET /version`
- Health endpoint: `GET /health` (includes version in response)

## License

MIT License - see LICENSE file for details.
