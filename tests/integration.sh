#!/bin/bash
set -eo pipefail

# 1. Determine Docker Compose command and configure ports
export COREDNS_PORT_UDP="${COREDNS_PORT_UDP:-8053}"
export COREDNS_PORT_TCP="${COREDNS_PORT_TCP:-8053}"
export COREDNS_PORT_METRICS="${COREDNS_PORT_METRICS:-9153}"
export COREDNS_PORT_API="${COREDNS_PORT_API:-8080}"

DOCKER_COMPOSE="docker compose"
if ! docker compose version >/dev/null 2>&1; then
    if docker-compose version >/dev/null 2>&1; then
        DOCKER_COMPOSE="docker-compose"
    else
        echo "Error: Neither 'docker compose' nor 'docker-compose' was found." >&2
        exit 1
    fi
fi

# Use sudo if docker commands require root permissions (detectable if docker run fails without sudo)
SUDO=""
if ! docker info >/dev/null 2>&1; then
    SUDO="sudo"
fi

COMPOSE_CMD="$SUDO $DOCKER_COMPOSE -f docker-compose.dev.yml"

echo "=== Using Docker Compose command: $COMPOSE_CMD ==="
echo "=== CoreDNS TCP Port: $COREDNS_PORT_TCP ==="
echo "=== CoreDNS API Port: $COREDNS_PORT_API ==="

# 2. Cleanup function for teardown on exit/failure
cleanup() {
    echo "=== Cleaning up environment ==="
    $COMPOSE_CMD down -v || true
}
trap cleanup EXIT

# 3. Start stack
echo "=== Starting Dev Stack ==="
$COMPOSE_CMD up -d

# 4. Wait for coredns_gslb to be ready
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

# Wait for healthcheck to be fully ready
echo "=== Waiting 15s for healthchecks to stabilize ==="
sleep 15

# 5. GSLB Failover Integration Tests
echo "=== Running GSLB Failover Tests ==="

echo "Check initial dig (should be 172.16.0.10)..."
ip=$(dig -p "$COREDNS_PORT_TCP" @127.0.0.1 webapp.app-x.gslb.example.com +short)
echo "Got IP: $ip"
[ "$ip" = "172.16.0.10" ] || (echo "Expected 172.16.0.10, got $ip" && exit 1)

echo "Stopping webapp10..."
$COMPOSE_CMD stop webapp10

echo "Waiting for healthcheck to update (35s)..."
sleep 35

echo "Check dig after webapp10 stopped (should be 172.16.0.11)..."
ip=$(dig -p "$COREDNS_PORT_TCP" @127.0.0.1 webapp.app-x.gslb.example.com +short)
echo "Got IP: $ip"
[ "$ip" = "172.16.0.11" ] || (echo "Expected 172.16.0.11, got $ip" && exit 1)

# API check while webapp10 is down - API should reflect active changes or at least respond
echo "Check API Overview while webapp10 is stopped..."
resp=$(curl -s -X GET http://127.0.0.1:"$COREDNS_PORT_API"/api/overview | jq 'keys | length')
echo "Got Nb zones: $resp"
[ "$resp" = "2" ] || (echo "Expected 2 zones, got $resp" && exit 1)

echo "Restarting webapp10..."
$COMPOSE_CMD start webapp10

echo "Waiting for healthcheck to update (35s)..."
sleep 35

echo "Check dig after webapp10 restarted (should be 172.16.0.10)..."
ip=$(dig -p "$COREDNS_PORT_TCP" @127.0.0.1 webapp.app-x.gslb.example.com +short)
echo "Got IP: $ip"
[ "$ip" = "172.16.0.10" ] || (echo "Expected 172.16.0.10, got $ip" && exit 1)

# 6. GSLB GeoIP Integration Tests
echo "=== Running GSLB GeoIP Tests ==="

echo "Check dig with query coming from subnet 10.1.0.0/24 (should be 172.16.0.10)..."
ip=$(dig -p "$COREDNS_PORT_TCP" @127.0.0.1 webapp-geoip-loc.app-y.gslb.example.com +short +subnet=10.1.0.42/24)
echo "Got IP: $ip"
[ "$ip" = "172.16.0.10" ] || (echo "Expected 172.16.0.10, got $ip" && exit 1)

echo "Check dig with query coming from subnet 10.2.0.0/24 (should be 172.16.0.11)..."
ip=$(dig -p "$COREDNS_PORT_TCP" @127.0.0.1 webapp-geoip-loc.app-y.gslb.example.com +short +subnet=10.2.0.7/24)
echo "Got IP: $ip"
[ "$ip" = "172.16.0.11" ] || (echo "Expected 172.16.0.11, got $ip" && exit 1)

echo "Check dig with query coming from US IP (should be 172.16.0.11)..."
ip=$(dig -p "$COREDNS_PORT_TCP" @127.0.0.1 webapp-geoip-country.app-y.gslb.example.com +short +subnet=8.8.8.8/24)
echo "Got IP: $ip"
[ "$ip" = "172.16.0.11" ] || (echo "Expected 172.16.0.11, got $ip" && exit 1)

echo "Check dig with query coming from FR IP (should be 172.16.0.10)..."
ip=$(dig -p "$COREDNS_PORT_TCP" @127.0.0.1 webapp-geoip-country.app-y.gslb.example.com +short +subnet=90.0.0.0/24)
echo "Got IP: $ip"
[ "$ip" = "172.16.0.10" ] || (echo "Expected 172.16.0.10, got $ip" && exit 1)

# 7. API Integration Tests
echo "=== Running API Tests ==="

echo "Check API Overview - count zones (should be 2)..."
resp=$(curl -s -X GET http://127.0.0.1:"$COREDNS_PORT_API"/api/overview | jq 'keys | length')
echo "Got Nb zones: $resp"
[ "$resp" = "2" ] || (echo "Expected 2 zones, got $resp" && exit 1)

echo "Check API Overview - count records (should be 7)..."
resp=$(curl -s -X GET http://127.0.0.1:"$COREDNS_PORT_API"/api/overview | jq 'map(length) | add')
echo "Got Nb records: $resp"
[ "$resp" = "7" ] || (echo "Expected 7 records, got $resp" && exit 1)

echo "=== All integration tests passed successfully! ==="
