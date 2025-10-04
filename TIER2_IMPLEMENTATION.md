# Tier 2 Implementation: Multi-Instance Deployment

## Overview

Tier 2 adds distributed locking capabilities to enable running multiple instances of the message dispatcher simultaneously. This provides high availability, fault tolerance, and zero-downtime deployments.

## Architecture

### Distributed Lock Mechanism

The system uses **Redis-based distributed locking** (Redlock algorithm) to ensure only one instance processes messages at any given time.

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│ Instance 1  │     │ Instance 2  │     │ Instance 3  │
│ (Active)    │     │ (Standby)   │     │ (Standby)   │
└──────┬──────┘     └──────┬──────┘     └──────┬──────┘
       │                   │                   │
       │    Acquire Lock   │                   │
       ├──────────────────►│◄──────────────────┤
       │                   │                   │
       │          ┌────────▼────────┐          │
       └─────────►│  Redis Lock     │◄─────────┘
                  │  Key: "message- │
                  │   dispatcher:   │
                  │   lock"         │
                  │  TTL: 3 minutes │
                  └─────────────────┘

Timeline:
T+0s:   Instance 1 acquires lock, starts processing
T+30s:  Instance 1 extends lock (auto-extends every interval/2)
T+60s:  Instance 1 extends lock
T+90s:  Instance 1 extends lock
T+120s: Instance 1 completes batch, releases lock
T+120s: Instance 2 acquires lock, starts processing
```

## Components

### 1. Distributed Lock Interface (`internal/lock/distributed_lock.go`)

Defines the contract for distributed locking:

```go
type DistributedLock interface {
    Acquire(ctx context.Context) error  // Acquire the lock
    Release(ctx context.Context) error  // Release the lock
    Extend(ctx context.Context) error   // Extend lock TTL
    IsHeld() bool                        // Check if held
}
```

### 2. Redis Lock Implementation

Uses Redis SET NX (Set if Not Exists) with TTL:

**Acquire:**

```redis
SET lock:key unique-value NX EX ttl
```

**Release (Lua script for atomicity):**

```lua
if redis.call("get", KEYS[1]) == ARGV[1] then
    return redis.call("del", KEYS[1])
else
    return 0
end
```

**Extend (Lua script for atomicity):**

```lua
if redis.call("get", KEYS[1]) == ARGV[1] then
    return redis.call("expire", KEYS[1], ARGV[2])
else
    return 0
end
```

### 3. Enhanced Scheduler

The scheduler now supports optional distributed locking:

```go
// Without locking (single instance)
scheduler := NewMessageScheduler(messageService, logger, interval)

// With locking (multi-instance)
distributedLock := lock.NewRedisLock(redis, "lock:key", 3*time.Minute, logger)
scheduler := NewMessageSchedulerWithLock(messageService, logger, interval, distributedLock)
```

## Configuration

### Environment Variables

| Variable                   | Description                 | Default                 |
| -------------------------- | --------------------------- | ----------------------- |
| `DISTRIBUTED_LOCK_ENABLED` | Enable distributed locking  | false                   |
| `DISTRIBUTED_LOCK_TTL`     | Lock time-to-live           | 3m                      |
| `DISTRIBUTED_LOCK_KEY`     | Redis key for the lock      | message-dispatcher:lock |
| `PROCESSING_INTERVAL`      | Message processing interval | 2m                      |

**Important:** `DISTRIBUTED_LOCK_TTL` should be longer than `PROCESSING_INTERVAL` to prevent lock expiration during processing.

### Example Configurations

#### Single Instance (Default)

```env
DISTRIBUTED_LOCK_ENABLED=false
```

#### Multi-Instance (High Availability)

```env
DISTRIBUTED_LOCK_ENABLED=true
DISTRIBUTED_LOCK_TTL=3m
DISTRIBUTED_LOCK_KEY=message-dispatcher:lock
```

## Deployment Scenarios

### Scenario 1: Docker Compose (Multiple Instances)

```yaml
version: "3.8"

services:
  dispatcher-1:
    image: h4mid2019/message-dispatcher
    environment:
      - DISTRIBUTED_LOCK_ENABLED=true
      - DB_HOST=postgres
      - REDIS_HOST=redis
    depends_on:
      - postgres
      - redis

  dispatcher-2:
    image: h4mid2019/message-dispatcher
    environment:
      - DISTRIBUTED_LOCK_ENABLED=true
      - DB_HOST=postgres
      - REDIS_HOST=redis
    depends_on:
      - postgres
      - redis

  dispatcher-3:
    image: h4mid2019/message-dispatcher
    environment:
      - DISTRIBUTED_LOCK_ENABLED=true
      - DB_HOST=postgres
      - REDIS_HOST=redis
    depends_on:
      - postgres
      - redis

  postgres:
    image: postgres:15
    # ... config ...

  redis:
    image: redis:7
    # ... config ...
```

### Scenario 2: Kubernetes (StatefulSet)

> **Warning:** This config has been written by llm, not been tested

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: message-dispatcher
spec:
  replicas: 3
  serviceName: message-dispatcher
  selector:
    matchLabels:
      app: message-dispatcher
  template:
    metadata:
      labels:
        app: message-dispatcher
    spec:
      containers:
        - name: dispatcher
          image: h4mid2019/message-dispatcher:latest
          env:
            - name: DISTRIBUTED_LOCK_ENABLED
              value: "true"
            - name: DISTRIBUTED_LOCK_TTL
              value: "3m"
            - name: DB_HOST
              value: "postgres-service"
            - name: REDIS_HOST
              value: "redis-service"
          livenessProbe:
            httpGet:
              path: /health
              port: 8080
            periodSeconds: 30
          readinessProbe:
            httpGet:
              path: /health
              port: 8080
            periodSeconds: 10
```

## How It Works

### Normal Operation

1. **Instance 1** acquires lock at `T+0s`
2. **Instance 1** processes messages (2 messages)
3. **Instance 1** extends lock every 60 seconds (interval/2)
4. **Instance 2** tries to acquire lock, fails (already held)
5. **Instance 3** tries to acquire lock, fails (already held)
6. **Instance 1** completes processing at `T+120s`, releases lock
7. **Instance 2** acquires lock at `T+120s`
8. Repeat...

### Failure Scenarios

#### Instance Crash During Processing

```
T+0s:   Instance 1 acquires lock (TTL=3m)
T+30s:  Instance 1 extends lock
T+60s:  Instance 1 crashes (no extension)
T+180s: Lock expires (3 minutes from last extension)
T+180s: Instance 2 acquires lock, continues processing
```

**Result:** Maximum delay of 3 minutes before another instance takes over

#### Redis Temporarily Down

```
T+0s:   Redis connection lost
T+0s:   All instances fail to acquire lock
T+0s:   Processing skipped, logs warning
T+30s:  Redis connection restored
T+30s:  Instance 1 acquires lock
T+30s:  Processing resumes
```

**Result:** Processing paused until Redis is available

#### Network Partition

```
T+0s:   Instance 1 acquires lock
T+30s:  Network partition: Instance 1 isolated from Redis
T+60s:  Instance 1 cannot extend lock
T+90s:  Instance 1 cannot extend lock
T+180s: Lock expires
T+180s: Instance 2 acquires lock
```

**Result:** Automatic failover to healthy instance

## Lock Lifecycle

### State Diagram

```
┌─────────────┐
│   Created   │
└──────┬──────┘
       │
       │ Acquire()
       ▼
┌─────────────┐     Extend()    ┌─────────────┐
│  Acquiring  ├────────────────►│   Held      │
└──────┬──────┘                 └──────┬──────┘
       │                               │
       │ Success                       │ Release()
       ▼                               ▼
┌─────────────┐                 ┌─────────────┐
│   Held      │                 │  Released   │
└──────┬──────┘                 └─────────────┘
       │
       │ TTL Expired
       ▼
┌─────────────┐
│  Expired    │
└─────────────┘
```

### Code Flow

```go
// Before processing batch
if err := lock.Acquire(ctx); err != nil {
    if err == ErrLockNotAcquired {
        // Another instance is processing, skip
        return
    }
    // Redis error, log and skip
    return
}
defer lock.Release(ctx)

// During processing (every interval/2)
if err := lock.Extend(ctx); err != nil {
    // Lock lost, log warning
    // Continue processing (already started)
}

// Process messages...
messageService.ProcessMessages(ctx)
```

## Testing

### Unit Tests

```bash
# Test distributed lock
go test ./internal/lock/... -v

# Test scheduler with locking
go test ./internal/scheduler/... -v
```

### Integration Tests

1. **Single Instance Test:**

```bash
DISTRIBUTED_LOCK_ENABLED=false go run cmd/server/main.go
```

2. **Multi-Instance Test:**

```bash
# Terminal 1
DISTRIBUTED_LOCK_ENABLED=true go run cmd/server/main.go

# Terminal 2
DISTRIBUTED_LOCK_ENABLED=true SERVER_PORT=8081 go run cmd/server/main.go

# Terminal 3
DISTRIBUTED_LOCK_ENABLED=true SERVER_PORT=8082 go run cmd/server/main.go
```

3. **Observe logs:**

```json
{"level":"info","msg":"Lock acquired","key":"message-dispatcher:lock"}
{"level":"debug","msg":"Another instance is processing, skipping"}
{"level":"info","msg":"Lock released","key":"message-dispatcher:lock"}
```

## Monitoring

### Metrics to Track

1. **Lock Acquisition Success Rate**

   - Successful: Lock acquired and processing started
   - Skipped: Another instance has the lock
   - Failed: Redis error or timeout

2. **Lock Hold Duration**

   - How long each instance holds the lock
   - Should match processing time + overhead

3. **Lock Extension Success Rate**

   - Successful extensions during processing
   - Failed extensions (may indicate Redis issues)

4. **Active Instance**
   - Which instance currently holds the lock
   - Track failover events

### Example Prometheus Metrics (Future Enhancement)

```go
lockAcquisitions = prometheus.NewCounterVec(
    prometheus.CounterOpts{
        Name: "dispatcher_lock_acquisitions_total",
        Help: "Total lock acquisition attempts",
    },
    []string{"result"}, // success, skipped, failed
)

lockHoldDuration = prometheus.NewHistogram(
    prometheus.HistogramOpts{
        Name: "dispatcher_lock_hold_seconds",
        Help: "Lock hold duration",
    },
)
```

## Best Practices

### 1. Lock TTL Configuration

**Rule:** `DISTRIBUTED_LOCK_TTL > PROCESSING_INTERVAL + max_processing_time`

Example:

- Processing interval: 2 minutes
- Max processing time: 30 seconds
- Recommended TTL: 3 minutes (buffer for safety)

### 2. Number of Instances

**Recommendations:**

- **Development:** 1 instance (no locking needed)
- **Staging:** 2 instances (test failover)
- **Production:** 3+ instances (high availability)

### 3. Redis High Availability

For production, use Redis Cluster or Redis Sentinel for Redis high availability:

```yaml
services:
  redis-master:
    image: redis:7
    command: redis-server --appendonly yes

  redis-sentinel:
    image: redis:7
    command: redis-sentinel /etc/redis/sentinel.conf
```

### 4. Health Check Configuration

```yaml
healthcheck:
  test: ["CMD", "curl", "-f", "http://localhost:8080/health"]
  interval: 30s
  timeout: 10s
  retries: 3
  start_period: 40s
```

### 5. Graceful Shutdown

Always use graceful shutdown to release locks properly:

```bash
# Send SIGTERM (not SIGKILL)
docker stop message-dispatcher  # Default 10s grace period
docker stop -t 30 message-dispatcher  # 30s grace period
```

## Troubleshooting

### Problem: Multiple instances processing simultaneously

**Cause:** Distributed locking not enabled or Redis connection issue

**Solution:**

1. Verify `DISTRIBUTED_LOCK_ENABLED=true`
2. Check Redis connectivity: `redis-cli ping`
3. Check logs for lock acquisition errors

### Problem: No instance processing messages

**Cause:** All instances failing to acquire lock

**Solution:**

1. Check Redis health: `redis-cli get message-dispatcher:lock`
2. Manually release stuck lock: `redis-cli del message-dispatcher:lock`
3. Check lock TTL configuration

### Problem: Lock expires during processing

**Cause:** Processing time exceeds lock TTL

**Solution:**

1. Increase `DISTRIBUTED_LOCK_TTL`
2. Check lock extension logs
3. Verify processing interval is appropriate

### Problem: Instance not releasing lock

**Cause:** Crash or improper shutdown

**Solution:**

1. Lock will auto-expire after TTL
2. Improve graceful shutdown handling
3. Add monitoring for stuck locks

## Performance Impact

### Single Instance Mode (Locking Disabled)

- **Overhead:** ~0ms per cycle
- **Latency:** None

### Multi-Instance Mode (Locking Enabled)

- **Overhead:** ~5-10ms per cycle (Redis round trip)
- **Latency:** 2-minute max delay if one instance has lock
- **Throughput:** Same as single instance (still 2 messages/2 minutes)

**Note:** Distributed locking adds minimal overhead but provides significant reliability benefits.

## Migration Guide

### From Single Instance to Multi-Instance

1. **Update configuration:**

   ```bash
   DISTRIBUTED_LOCK_ENABLED=true
   ```

2. **Deploy second instance:**

   ```bash
   docker run -d --name dispatcher-2 \
     -e DISTRIBUTED_LOCK_ENABLED=true \
     -e DB_HOST=postgres \
     -e REDIS_HOST=redis \
     h4mid2019/message-dispatcher
   ```

3. **Monitor logs:**

   - Should see alternating lock acquisitions
   - One instance processes, others wait

4. **Add more instances:**

   ```bash
   docker run -d --name dispatcher-3 ...
   ```

### From Multi-Instance to Single Instance

1. **Stop extra instances:**

   ```bash
   docker stop dispatcher-2 dispatcher-3
   ```

2. **Update remaining instance:**

   ```bash
   DISTRIBUTED_LOCK_ENABLED=false
   ```

3. **Restart:**

   ```bash
   docker restart dispatcher-1
   ```

## Tier 2 implementation provides

**High Availability** - Multiple instances for fault tolerance.  
**Zero Downtime** - Supports rolling deployments without interruption.  
**Automatic Failover** - Crashed instances are replaced automatically by other running instances.  
**Redis-based locking** - A reliable locking mechanism.
**Minimal Overhead** - Adds about 10ms per cycle.  
**Easy Configuration** - Enabled with a single environment variable.  
**Good for Production** - Recommended for production deployments requiring high availability.
