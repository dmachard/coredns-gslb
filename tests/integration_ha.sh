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

export COREDNS_2_PORT_UDP="${COREDNS_2_PORT_UDP:-8153}"
export COREDNS_2_PORT_TCP="${COREDNS_2_PORT_TCP:-8153}"
export COREDNS_2_PORT_API="${COREDNS_2_PORT_API:-8081}"

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

    # 2. Verify that both CoreDNS instances have the same number of records in their API overview
    local count1 count2
    count1=$(curl -s "http://127.0.0.1:$COREDNS_PORT_API/api/overview" | jq '.[][] | .record' | wc -l)
    count2=$(curl -s "http://127.0.0.1:$COREDNS_2_PORT_API/api/overview" | jq '.[][] | .record' | wc -l)
    echo "Instance 1 records count: $count1"
    echo "Instance 2 records count: $count2"
    if [ "$count1" -eq 0 ] || [ "$count2" -eq 0 ]; then
        echo "Error: records count should be greater than 0"
        return 1
    fi
    if [ "$count1" -ne "$count2" ]; then
        echo "Error: record counts do not match between instances ($count1 vs $count2)"
        return 1
    fi

    # 3. Verify initial DNS resolution on BOTH instances (should resolve to 172.16.0.10)
    local ip1 ip2
    ip1=$(dig -p "$COREDNS_PORT_TCP" @127.0.0.1 webapp.app-x.gslb.example.com +short)
    ip2=$(dig -p "$COREDNS_2_PORT_TCP" @127.0.0.1 webapp.app-x.gslb.example.com +short)
    echo "Instance 1 initial IP: $ip1"
    echo "Instance 2 initial IP: $ip2"
    if [ "$ip1" != "172.16.0.10" ] || [ "$ip2" != "172.16.0.10" ]; then
        echo "Error: initial resolution on both instances should be 172.16.0.10"
        return 1
    fi

    # 4. Verify initial API status on BOTH instances (should be healthy)
    local api_status1 api_status2
    api_status1=$(curl -s "http://127.0.0.1:$COREDNS_PORT_API/api/overview" | jq -r '."app-x.gslb.example.com."[] | select(.record == "webapp.app-x.gslb.example.com.") | .backends[] | select(.address == "172.16.0.10") | .alive')
    api_status2=$(curl -s "http://127.0.0.1:$COREDNS_2_PORT_API/api/overview" | jq -r '."app-x.gslb.example.com."[] | select(.record == "webapp.app-x.gslb.example.com.") | .backends[] | select(.address == "172.16.0.10") | .alive')
    echo "Instance 1 initial API status: $api_status1"
    echo "Instance 2 initial API status: $api_status2"
    if [ "$api_status1" != "healthy" ] || [ "$api_status2" != "healthy" ]; then
        echo "Error: initial status of 172.16.0.10 in API should be healthy"
        return 1
    fi

    # 5. Stop webapp10 container to trigger failover
    echo "Stopping webapp10 container..."
    $COMPOSE_CMD stop webapp10 > /dev/null

    # 6. Wait for the failover to propagate to both CoreDNS instances
    echo "Waiting for both instances to detect webapp10 is offline..."
    local ok=false
    for i in {1..30}; do
        ip1=$(dig -p "$COREDNS_PORT_TCP" @127.0.0.1 webapp.app-x.gslb.example.com +short)
        ip2=$(dig -p "$COREDNS_2_PORT_TCP" @127.0.0.1 webapp.app-x.gslb.example.com +short)
        if [ "$ip1" = "172.16.0.11" ] && [ "$ip2" = "172.16.0.11" ]; then
            ok=true
            break
        fi
        sleep 1
    done

    if [ "$ok" = false ]; then
        echo "Error: failover did not propagate to both instances in time. IP1: $ip1, IP2: $ip2"
        return 1
    fi
    echo "Both CoreDNS instances successfully failed over to 172.16.0.11"

    # Verify updated API status on BOTH instances (should be unhealthy)
    api_status1=$(curl -s "http://127.0.0.1:$COREDNS_PORT_API/api/overview" | jq -r '."app-x.gslb.example.com."[] | select(.record == "webapp.app-x.gslb.example.com.") | .backends[] | select(.address == "172.16.0.10") | .alive')
    api_status2=$(curl -s "http://127.0.0.1:$COREDNS_2_PORT_API/api/overview" | jq -r '."app-x.gslb.example.com."[] | select(.record == "webapp.app-x.gslb.example.com.") | .backends[] | select(.address == "172.16.0.10") | .alive')
    echo "Instance 1 offline API status: $api_status1"
    echo "Instance 2 offline API status: $api_status2"
    if [ "$api_status1" != "unhealthy" ] || [ "$api_status2" != "unhealthy" ]; then
        echo "Error: status of 172.16.0.10 in API should be unhealthy on both instances"
        return 1
    fi

    # 7. Start webapp10 container again
    echo "Starting webapp10 container back up..."
    $COMPOSE_CMD start webapp10 > /dev/null

    # 8. Wait for both instances to detect webapp10 is back online
    echo "Waiting for both instances to detect webapp10 is online..."
    ok=false
    for i in {1..30}; do
        ip1=$(dig -p "$COREDNS_PORT_TCP" @127.0.0.1 webapp.app-x.gslb.example.com +short)
        ip2=$(dig -p "$COREDNS_2_PORT_TCP" @127.0.0.1 webapp.app-x.gslb.example.com +short)
        if [ "$ip1" = "172.16.0.10" ] && [ "$ip2" = "172.16.0.10" ]; then
            ok=true
            break
        fi
        sleep 1
    done

    if [ "$ok" = false ]; then
        echo "Error: recovery did not propagate to both instances in time. IP1: $ip1, IP2: $ip2"
        return 1
    fi
    echo "Both CoreDNS instances successfully recovered and resolved back to 172.16.0.10"

    # 9. Verify that the Redis lock was active and prevented duplicate checks.
    # At least one instance must have logged skipping execution due to lock not being acquired.
    echo "Checking logs to verify Redis lock execution bypass..."
    local logs1 logs2
    logs1=$($COMPOSE_CMD logs coredns_gslb | grep "Redis lock not acquired" || true)
    logs2=$($COMPOSE_CMD logs coredns_gslb_2 | grep "Redis lock not acquired" || true)
    if [ -z "$logs1" ] && [ -z "$logs2" ]; then
        echo "Error: no instance logged skipping health check execution via Redis lock"
        return 1
    fi
    echo "Successfully verified Redis lock is active and preventing duplicate checks:"
    if [ -n "$logs1" ]; then
        echo "Instance 1 skipped some checks: $(echo "$logs1" | tail -n 1)"
    fi
    if [ -n "$logs2" ]; then
        echo "Instance 2 skipped some checks: $(echo "$logs2" | tail -n 1)"
    fi

    return 0
}
run_test "Check Redis synchronization and Pub/Sub propagation on both CoreDNS instances" test_redis_sync

# 7. Output Statistics
echo "=== Redis HA Integration Test Suite Completed ==="
echo "Total executed integration tests: $TESTS_RUN"
echo "Total passed integration tests: $TESTS_PASSED"
echo "All $TESTS_PASSED integration tests passed successfully!"
