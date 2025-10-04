# Quick Start: Multi-Instance Deployment

## Overview

This guide shows you how to run multiple instances of the message dispatcher for high availability.

## Prerequisites

- Docker and Docker Compose installed
- Redis running (provided in docker-compose)
- PostgreSQL running (provided in docker-compose)

## Option 1: Quick Test (3 Instances)

Use the provided multi-instance docker-compose file:

```bash
# Start 3 instances
docker-compose -f docker-compose.multi-instance.yml up

# Watch the logs - you'll see only one instance processing at a time
# Look for these log messages:
# - "Lock acquired" - Instance that will process
# - "Another instance is processing, skipping" - Standby instances
```

### What You'll See

```json
// Instance 1 (Active)
{"level":"info","msg":"Lock acquired","key":"message-dispatcher:lock"}
{"level":"debug","msg":"Message batch processed","duration":"100ms"}
{"level":"info","msg":"Lock released","key":"message-dispatcher:lock"}

// Instance 2 (Standby)
{"level":"debug","msg":"Another instance is processing, skipping this cycle"}

// Instance 3 (Standby)
{"level":"debug","msg":"Another instance is processing, skipping this cycle"}
```

### Test Failover

1. **Kill the active instance**:

   ```bash
   docker stop message_dispatcher_1
   ```

2. **Watch another instance take over**:

   - Within 3 minutes, Instance 2 or 3 will acquire the lock
   - Processing continues automatically

3. **Restart the stopped instance**:
   ```bash
   docker start message_dispatcher_1
   ```

## Option 2: Custom Configuration

### Step 1: Enable Distributed Locking

Add to your `.env` file or environment:

```env
DISTRIBUTED_LOCK_ENABLED=true
DISTRIBUTED_LOCK_TTL=3m
DISTRIBUTED_LOCK_KEY=message-dispatcher:lock
```

### Step 2: Run Multiple Instances

```bash
# Terminal 1 - Instance 1
export DISTRIBUTED_LOCK_ENABLED=true
go run cmd/server/main.go

# Terminal 2 - Instance 2
export DISTRIBUTED_LOCK_ENABLED=true
export SERVER_PORT=8081
go run cmd/server/main.go

# Terminal 3 - Instance 3
export DISTRIBUTED_LOCK_ENABLED=true
export SERVER_PORT=8082
go run cmd/server/main.go
```

## Option 3: Kubernetes Deployment

### Step 1: Create Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: message-dispatcher
spec:
  replicas: 3
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
          ports:
            - containerPort: 8080
          env:
            - name: DISTRIBUTED_LOCK_ENABLED
              value: "true"
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

### Step 2: Deploy

```bash
kubectl apply -f deployment.yaml
kubectl get pods
# You should see 3 pods running
```

### Step 3: Monitor

```bash
# Watch logs from all instances
kubectl logs -l app=message-dispatcher --follow
```

## Verification

### Check All Instances Are Healthy

```bash
# Instance 1
curl http://localhost:8080/health

# Instance 2
curl http://localhost:8081/health

# Instance 3
curl http://localhost:8082/health
```

Expected response:

```json
{
  "status": "healthy",
  "processing_status": "running",
  "dependencies": {
    "database": "healthy",
    "redis": "healthy"
  }
}
```

### Check Redis Lock

```bash
# Connect to Redis
docker exec -it messages_redis_ha redis-cli

# Check if lock exists
GET message-dispatcher:lock

# Check lock TTL
TTL message-dispatcher:lock
# Should show remaining seconds (0-180)
```

### Monitor Active Instance

```bash
# Watch logs for lock acquisitions
docker-compose -f docker-compose.multi-instance.yml logs -f | grep "Lock acquired"
```

## Troubleshooting

### Problem: Multiple instances processing simultaneously

**Solution**: Verify distributed locking is enabled

```bash
docker exec message_dispatcher_1 env | grep DISTRIBUTED_LOCK_ENABLED
# Should output: DISTRIBUTED_LOCK_ENABLED=true
```

### Problem: No instance processing messages

**Solution**: Check Redis connectivity

```bash
# Test Redis connection
docker exec message_dispatcher_1 wget -q -O- http://localhost:8080/health
# Check "redis" status in dependencies
```

### Problem: Lock stuck (no processing for > 3 minutes)

**Solution**: Manually release lock

```bash
docker exec -it messages_redis_ha redis-cli DEL message-dispatcher:lock
```

## Performance Testing

### Load Test with Multiple Instances

1. **Add 100 test messages**:

   ```bash
   docker exec message_dispatcher_1 /app/add-test-messages
   ```

2. **Watch processing across instances**:

   ```bash
   docker-compose -f docker-compose.multi-instance.yml logs -f | grep "Message batch processed"
   ```

3. **Verify only one processes at a time**:
   ```bash
   # Count concurrent processing (should be 1)
   docker-compose -f docker-compose.multi-instance.yml logs --since 5m | grep "Message batch processed" | cut -d' ' -f1-2 | uniq -c
   ```

## Best Practices

### 1. Set Appropriate Lock TTL

Rule: `DISTRIBUTED_LOCK_TTL > PROCESSING_INTERVAL + max_processing_time`

Example:

```env
PROCESSING_INTERVAL=2m
MAX_PROCESSING_TIME=30s
DISTRIBUTED_LOCK_TTL=3m  # 2m + 30s + 30s buffer
```

### 2. Run Odd Number of Instances

Recommended:

- **Development**: 1 instance (no locking)
- **Staging**: 2-3 instances (test failover)
- **Production**: 3-5 instances (high availability)

### 3. Monitor Lock Metrics

Watch for:

- Lock acquisition success rate
- Lock hold duration
- Failed lock acquisitions
- Lock extensions

### 4. Health Check Configuration

```yaml
healthcheck:
  test:
    [
      "CMD",
      "wget",
      "--quiet",
      "--tries=1",
      "--spider",
      "http://localhost:8080/health",
    ]
  interval: 30s
  timeout: 10s
  retries: 3
  start_period: 40s
```

## Scaling Commands

### Docker Compose

```bash
# Scale to 5 instances
docker-compose -f docker-compose.multi-instance.yml up --scale dispatcher-1=5

# Scale down to 2 instances
docker-compose -f docker-compose.multi-instance.yml up --scale dispatcher-1=2
```

### Kubernetes

```bash
# Scale to 5 instances
kubectl scale deployment message-dispatcher --replicas=5

# Scale down to 2 instances
kubectl scale deployment message-dispatcher --replicas=2

# Auto-scale based on CPU
kubectl autoscale deployment message-dispatcher --min=3 --max=10 --cpu-percent=80
```

## Rolling Updates

### Docker Compose

```bash
# Update to new version with zero downtime
docker-compose -f docker-compose.multi-instance.yml pull
docker-compose -f docker-compose.multi-instance.yml up -d --no-deps --build dispatcher-1
docker-compose -f docker-compose.multi-instance.yml up -d --no-deps --build dispatcher-2
docker-compose -f docker-compose.multi-instance.yml up -d --no-deps --build dispatcher-3
```

### Kubernetes

```bash
# Update image
kubectl set image deployment/message-dispatcher dispatcher=h4mid2019/message-dispatcher:v2.0.0

# Monitor rollout
kubectl rollout status deployment/message-dispatcher

# Rollback if needed
kubectl rollout undo deployment/message-dispatcher
```

## Migration from Single to Multi-Instance

### Step 1: Test Locally

```bash
# Start with 2 instances
docker-compose -f docker-compose.multi-instance.yml up dispatcher-1 dispatcher-2
```

### Step 2: Enable in Staging

```bash
# Update staging environment
kubectl set env deployment/message-dispatcher DISTRIBUTED_LOCK_ENABLED=true
kubectl scale deployment/message-dispatcher --replicas=2
```

### Step 3: Monitor

```bash
# Watch for 24 hours
kubectl logs -l app=message-dispatcher --follow | grep -E "(Lock acquired|Lock released)"
```

### Step 4: Roll Out to Production

```bash
# Enable locking
kubectl set env deployment/message-dispatcher DISTRIBUTED_LOCK_ENABLED=true

# Scale gradually
kubectl scale deployment/message-dispatcher --replicas=2
# Wait 1 hour
kubectl scale deployment/message-dispatcher --replicas=3
# Wait 1 hour
kubectl scale deployment/message-dispatcher --replicas=5
```

## Summary

**Easy Setup**: Can be enabled with a single environment variable.  
**Zero Downtime**: Supports rolling updates.  
**Auto Failover**: If an instance crashes, another takes over.  
**Simple Testing**: A Docker Compose file is provided for testing.  
**Good for Production**: Suitable for production use with monitoring.

For more details, see:

- [TIER2_IMPLEMENTATION.md](TIER2_IMPLEMENTATION.md) - Technical details
- [README.md](README.md) - General documentation
- [COMPLETION_CHECKLIST.md](COMPLETION_CHECKLIST.md) - Feature checklist
