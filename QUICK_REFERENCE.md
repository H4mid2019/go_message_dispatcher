# Quick Reference - What Changed and How to Use

## What's Different Now

### 1. Auto-Start (No Manual Trigger Needed)

**Before**: You had to call `POST /api/messaging/start` to begin processing  
**Now**: Processing starts automatically when the app launches

**Evidence**: Check logs after startup:

```json
{"msg":"Message scheduler started","interval":120}
{"msg":"Background message processing started"}
```

### 2. Retry Behavior (Webhook Down Scenarios)

**Before**: Not fully specified  
**Now**:

- If webhook is down, messages remain in queue
- System retries same batch every 2 minutes until success
- No new messages loaded until current batch succeeds
- Individual message success: Message 1 sent + Message 2 fails = Message 1 stays sent

### 3. SSL/TLS Support

**Before**: Not specified  
**Now**:

- Accepts both http:// and https:// webhook URLs
- Self-signed certificates accepted
- 6-second timeout per webhook call

### 4. Data Validation

**Now**: Invalid messages automatically filtered:

- Phone numbers: 10-20 characters, non-null, non-empty
- Content: non-null, non-empty
- Validation happens at SQL level

### 5. Race Conditions

**Now**: Protected against:

- Multiple processes accessing same records (`FOR UPDATE SKIP LOCKED`)
- New messages during processing (locked batch)
- Status update failures (per-message handling)
- Duplicate prevention (idempotent updates)

### 6. Redis Failures

**Now**: Redis failures don't stop message sending

- Messages still sent if Redis is down
- Caching is best-effort only

### 7. Logging

**Now**: Structured JSON logging is used:

- No `fmt.Print` statements are used for logging.
- Log levels are used to distinguish between informational messages, debug details, and errors.
- Example levels:
  - INFO: For events like startup, connections, etc.
  - DEBUG: For routine events like batch completion.
  - ERROR: For failures that require attention.

## How to Run

### Start Everything (Development)

```powershell
# Terminal 1: Mock API
go run .\cmd\mock-api\main.go

# Terminal 2: Main Server
$env:DB_PASSWORD="your_password"
go run .\cmd\server\main.go
```

### Start Everything (Docker)

```bash
docker-compose up
```

## Testing Commands

### Add Test Messages

```powershell
$env:DB_PASSWORD="postgres"
go run .\cmd\add-test-messages\main.go
```

### Clear All Messages

```powershell
$env:DB_PASSWORD="postgres"
go run .\cmd\clear-messages\main.go
```

### Check Sent Messages

```powershell
Invoke-WebRequest -Uri "http://localhost:8080/api/messages/sent" | ConvertFrom-Json
```

### Check Health

```powershell
Invoke-WebRequest -Uri "http://localhost:8080/health" | ConvertFrom-Json
```

### Manual Stop (Optional)

```powershell
Invoke-WebRequest -Uri "http://localhost:8080/api/messaging/stop" -Method POST
```

### Manual Start (Optional)

```powershell
Invoke-WebRequest -Uri "http://localhost:8080/api/messaging/start" -Method POST
```

## Expected Behavior

### Normal Operation

1. App starts → Scheduler auto-starts
2. Every 2 minutes: Sends 2 messages (or 1 if only 1 available)
3. Messages sent in FIFO order (oldest first)
4. Successful sends cached in Redis with messageId

### Webhook Down

1. Messages remain in queue (sent=false)
2. Errors logged every 2 minutes
3. Same batch retried indefinitely
4. When webhook returns: messages sent in next cycle

### Only 1 Message Available

- Sends that 1 message (not waiting for 2)
- Waits 2 minutes for next check

### Database/Redis Issues

- Database down: App fails to start (critical dependency)
- Redis down: Messages still sent, caching skipped

## Monitoring

### Check if Processing is Running

```bash
curl http://localhost:8080/health
```

Look for: `"processing_status": "running"`

### Check Logs

Look for these patterns:

```json
{"level":"info","msg":"Database connection established"}
{"level":"info","msg":"Redis connection established"}
{"level":"info","msg":"Message scheduler started"}
{"level":"debug","msg":"Message batch processed"}
{"level":"error","msg":"Failed to process message batch"}
```

## Configuration

All configuration via environment variables (no changes needed):

```bash
DB_HOST=localhost
DB_PORT=5432
DB_NAME=messages_db
DB_USER=postgres
DB_PASSWORD=your_password
DB_SSLMODE=disable

REDIS_HOST=localhost
REDIS_PORT=6379

SMS_API_URL=http://localhost:3001/send   # or https://...
SMS_API_TOKEN=mock-token

PROCESSING_INTERVAL=2m   # 2 minutes
LOG_LEVEL=info           # or debug
```

## Troubleshooting

### "Processing not starting"

- Check logs for `"Message scheduler started"`
- Should auto-start, no manual trigger needed

### "Messages not being sent"

- Check webhook is running: `curl http://localhost:3001/health`
- Check logs for errors
- Verify messages exist: Query database `SELECT * FROM messages WHERE sent=false`

### "Too fast/slow processing"

- Change `PROCESSING_INTERVAL` environment variable
- Default: 2 minutes
- Example: `PROCESSING_INTERVAL=30s`

### "Want to see detailed logs"

- Set `LOG_LEVEL=debug`
- Will show batch completion messages

## Performance Notes

- Processes 2 messages per batch
- ~100ms per batch (fast)
- 6-second webhook timeout
- Graceful shutdown: waits for current batch
- Database connection pooling: 25 connections
