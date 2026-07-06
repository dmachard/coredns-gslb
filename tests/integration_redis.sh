#!/bin/bash
set -eo pipefail

# 1. Determine Docker Compose command and configure ports
if [ -n "${COREDNS_PORT_TCP:-}" ] && [ -z "${COREDNS_PORT_UDP:-}" ]; then
    export COREDNS_PORT_UDP="${COREDNS_PORT_TCP}"
elif [ -n "${COREDNS_PORT_UDP:-}" ] && [ -z "${COREDNS_PORT_TCP:-}" ]; then
    export COREDNS_PORT_TCP="${COREDNS_PORT_UDP}"
fi

export COREDNS_PORT_UDP="${COREDNS_PORT_UDP:-8053}"
export COREDNS_PORT_TCP="${COREDNS_PORT_TCP:-8053}"
export COREDNS_PORT_METRICS="${COREDNS_PORT_METRICS:-9153}"
export COREDNS_PORT_API="${COREDNS_PORT_API:-8082}"

DOCKER_COMPOSE="docker compose"
if ! docker compose version >/dev/null 2>&1; then
    if docker-compose version >/dev/null 2>&1; then
        DOCKER_COMPOSE="docker-compose"
    else
        echo "Error: Neither 'docker compose' nor 'docker-compose' was found." >&2
        exit 1
    fi
fi

# Use sudo if docker commands require root permissions
SUDO=""
if ! docker info >/dev/null 2>&1; then
    SUDO="sudo"
fi

COMPOSE_CMD="$SUDO $DOCKER_COMPOSE -f docker-compose.redis.yml"

echo "=== Using Docker Compose command: $COMPOSE_CMD ==="
echo "=== CoreDNS TCP Port: $COREDNS_PORT_TCP ==="
echo "=== CoreDNS API Port: $COREDNS_PORT_API ==="

# 2. Test counter variables and helper function
TESTS_RUN=0
TESTS_PASSED=0

run_test() {
    local name="$1"
    shift
    TESTS_RUN=$((TESTS_RUN + 1))
    echo "Running test $((TESTS_RUN)): $name..."
    if "$@"; then
        TESTS_PASSED=$((TESTS_PASSED + 1))
        echo "✓ Pass"
    else
        echo "✗ Fail"
        exit 1
    fi
    echo ""
}

# 3. Cleanup function for teardown on exit/failure
cleanup() {
    EXIT_CODE=$?
    if [ $EXIT_CODE -ne 0 ]; then
        echo "=== Test failed! Showing coredns_gslb logs: ==="
        $COMPOSE_CMD logs coredns_gslb || true
    fi
    echo "=== Cleaning up environment ==="
    $COMPOSE_CMD down -v || true
}
trap cleanup EXIT

# 4. Start stack
echo "=== Starting Redis HA Stack ==="
$COMPOSE_CMD up -d --build

# 5. Wait for coredns_gslb to be ready
echo "=== Waiting for CoreDNS GSLB to be ready ==="
READY=false
for i in {1..30}; do
    if dig -p "$COREDNS_PORT_TCP" @127.0.0.1 webapp.app-x.gslb.example.com +short | grep -q '172.16.0.10'; then
        READY=true
        break
    fi
    sleep 2
done

if [ "$READY" = false ]; then
    echo "Error: coredns_gslb did not become ready in time" >&2
    $COMPOSE_CMD logs coredns_gslb
    exit 1
fi

# Wait a short duration for healthchecks to stabilize
echo "=== Waiting 10s for healthchecks to stabilize ==="
sleep 10

# 6. Redis Synchronization Integration Tests
echo "=== Running Redis Synchronization Tests ==="

test_redis_sync() {
    # 1. Verify we can ping Redis
    $COMPOSE_CMD exec -T redis redis-cli ping | grep -q "PONG"
    if [ $? -ne 0 ]; then
        echo "Error: redis is not reachable"
        return 1
    fi

    # 2. Verify that coredns has registered records in Redis.
    local keys
    keys=$($COMPOSE_CMD exec -T redis redis-cli keys "*")
    echo "Redis keys: $keys"
    
    # We should have health check keys for the webapps.
    echo "$keys" | grep -q "gslb:health:app-x.gslb.example.com.:webapp.app-x.gslb.example.com.:172.16.0.10"
    if [ $? -ne 0 ]; then
        echo "Error: expected health status key not found in Redis"
        return 1
    fi

    # 3. Test Pub/Sub:
    # Set the key for 172.16.0.10 to unhealthy ("0") and publish the update using redis-cli.
    local key="gslb:health:app-x.gslb.example.com.:webapp.app-x.gslb.example.com.:172.16.0.10"
    local channel="gslb:health:updates"
    local payload='{"zone":"app-x.gslb.example.com.","fqdn":"webapp.app-x.gslb.example.com.","address":"172.16.0.10","alive":false}'
    
    $COMPOSE_CMD exec -T redis redis-cli set "$key" "0" > /dev/null
    $COMPOSE_CMD exec -T redis redis-cli publish "$channel" "$payload" > /dev/null

    # Wait for the background pub/sub to propagate and local status to update
    sleep 2

    # Query dig for webapp.app-x.gslb.example.com (since 172.16.0.10 is now offline, 172.16.0.11 should resolve)
    local ip
    ip=$(dig -p "$COREDNS_PORT_TCP" @127.0.0.1 webapp.app-x.gslb.example.com +short)
    echo "Got IP: $ip"
    
    # Verify that the IP is 172.16.0.11 (since 10 was marked unhealthy via Redis)
    if [ "$ip" != "172.16.0.11" ]; then
        echo "Error: expected DNS resolution to avoid 172.16.0.10 after Redis pubsub update, got $ip"
        return 1
    fi

    # 4. Now restore it to healthy
    payload='{"zone":"app-x.gslb.example.com.","fqdn":"webapp.app-x.gslb.example.com.","address":"172.16.0.10","alive":true}'
    $COMPOSE_CMD exec -T redis redis-cli set "$key" "1" > /dev/null
    $COMPOSE_CMD exec -T redis redis-cli publish "$channel" "$payload" > /dev/null
    sleep 2

    return 0
}
run_test "Check Redis synchronization and Pub/Sub propagation" test_redis_sync

# 7. Output Statistics
echo "=== Redis HA Integration Test Suite Completed ==="
echo "Total executed integration tests: $TESTS_RUN"
echo "Total passed integration tests: $TESTS_PASSED"
echo "All $TESTS_PASSED integration tests passed successfully!"
