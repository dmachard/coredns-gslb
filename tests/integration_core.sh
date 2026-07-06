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

# Use sudo if docker commands require root permissions (detectable if docker run fails without sudo)
SUDO=""
if ! docker info >/dev/null 2>&1; then
    SUDO="sudo"
fi

COMPOSE_CMD="$SUDO $DOCKER_COMPOSE -f docker-compose.dev.yml"

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
echo "=== Starting Dev Stack ==="
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

# Wait for Consul to elect leader and register service
echo "=== Waiting for Consul to be ready ==="
for i in {1..30}; do
    if curl -s http://127.0.0.1:8500/v1/status/leader | grep -q '172.16.0'; then
        break
    fi
    sleep 1
done

echo "=== Registering Service in Consul ==="
curl -s -X PUT -H "Content-Type: application/json" -d '{"Node": "node1", "Address": "172.16.0.10", "Service": {"ID": "web1", "Service": "web-service", "Address": "172.16.0.10", "Port": 80}}' http://127.0.0.1:8500/v1/catalog/register

# Wait for healthcheck to be fully ready
echo "=== Waiting 15s for healthchecks to stabilize ==="
sleep 15

# 6. GSLB Failover Integration Tests
echo "=== Running GSLB Failover Tests ==="

test_initial_dig() {
    local ip
    ip=$(dig -p "$COREDNS_PORT_TCP" @127.0.0.1 webapp.app-x.gslb.example.com +short)
    echo "Got IP: $ip"
    [ "$ip" = "172.16.0.10" ]
}
run_test "Check initial dig (should be 172.16.0.10)" test_initial_dig

echo "Stopping webapp10..."
$COMPOSE_CMD stop webapp10

echo "Waiting for healthcheck to update (35s)..."
sleep 35

test_dig_after_stop() {
    local ip
    ip=$(dig -p "$COREDNS_PORT_TCP" @127.0.0.1 webapp.app-x.gslb.example.com +short)
    echo "Got IP: $ip"
    [ "$ip" = "172.16.0.11" ]
}
run_test "Check dig after webapp10 stopped (should be 172.16.0.11)" test_dig_after_stop

test_api_while_stopped() {
    local resp
    resp=$(curl -s -X GET http://127.0.0.1:"$COREDNS_PORT_API"/api/overview | jq 'keys | length')
    echo "Got Nb zones: $resp"
    [ "$resp" = "2" ]
}
run_test "Check API Overview while webapp10 is stopped (should be 2 zones)" test_api_while_stopped

echo "Restarting webapp10..."
$COMPOSE_CMD start webapp10

echo "Waiting for healthcheck to update (35s)..."
sleep 35

test_dig_after_restart() {
    local ip
    ip=$(dig -p "$COREDNS_PORT_TCP" @127.0.0.1 webapp.app-x.gslb.example.com +short)
    echo "Got IP: $ip"
    [ "$ip" = "172.16.0.10" ]
}
run_test "Check dig after webapp10 restarted (should be 172.16.0.10)" test_dig_after_restart


# 7. GSLB GeoIP Integration Tests
echo "=== Running GSLB GeoIP Tests ==="

test_geoip_subnet_10_1() {
    local ip
    ip=$(dig -p "$COREDNS_PORT_TCP" @127.0.0.1 webapp-geoip-loc.app-y.gslb.example.com +short +subnet=10.1.0.42/24)
    echo "Got IP: $ip"
    [ "$ip" = "172.16.0.10" ]
}
run_test "Check dig with query coming from subnet 10.1.0.0/24 (should be 172.16.0.10)" test_geoip_subnet_10_1

test_geoip_subnet_10_2() {
    local ip
    ip=$(dig -p "$COREDNS_PORT_TCP" @127.0.0.1 webapp-geoip-loc.app-y.gslb.example.com +short +subnet=10.2.0.7/24)
    echo "Got IP: $ip"
    [ "$ip" = "172.16.0.11" ]
}
run_test "Check dig with query coming from subnet 10.2.0.0/24 (should be 172.16.0.11)" test_geoip_subnet_10_2

test_geoip_us_country() {
    local ip
    ip=$(dig -p "$COREDNS_PORT_TCP" @127.0.0.1 webapp-geoip-country.app-y.gslb.example.com +short +subnet=8.8.8.8/24)
    echo "Got IP: $ip"
    [ "$ip" = "172.16.0.11" ]
}
run_test "Check dig with query coming from US IP (should be 172.16.0.11)" test_geoip_us_country

test_geoip_fr_country() {
    local ip
    ip=$(dig -p "$COREDNS_PORT_TCP" @127.0.0.1 webapp-geoip-country.app-y.gslb.example.com +short +subnet=90.0.0.0/24)
    echo "Got IP: $ip"
    [ "$ip" = "172.16.0.10" ]
}
run_test "Check dig with query coming from FR IP (should be 172.16.0.10)" test_geoip_fr_country


# 8. Additional Health Check & GeoIP City Tests
echo "=== Running Additional GSLB Feature Tests ==="

test_grpc_healthcheck() {
    local ip
    ip=$(dig -p "$COREDNS_PORT_TCP" @127.0.0.1 webapp-grpc.app-x.gslb.example.com +short)
    echo "Got IP: $ip"
    [ "$ip" = "172.16.0.12" ]
}
run_test "Check gRPC Healthcheck resolution (should be webapp12: 172.16.0.12)" test_grpc_healthcheck

test_mtls_healthcheck() {
    local ip
    ip=$(dig -p "$COREDNS_PORT_TCP" @127.0.0.1 webapp-mtls.app-x.gslb.example.com +short)
    echo "Got IP: $ip"
    echo "$ip" | grep -q '172.16.0.10' && echo "$ip" | grep -q '172.16.0.11'
}
run_test "Check mTLS Healthcheck resolution (should return both webapp10 and webapp11: 172.16.0.10 and 172.16.0.11)" test_mtls_healthcheck

test_lua_healthcheck() {
    local ip
    ip=$(dig -p "$COREDNS_PORT_TCP" @127.0.0.1 webapp-lua.app-x.gslb.example.com +short)
    echo "Got IP: $ip"
    [ "$ip" = "172.16.0.10" ]
}
run_test "Check Lua Healthcheck resolution (should be webapp10: 172.16.0.10)" test_lua_healthcheck

test_svcb_https_queries() {
    local resp
    resp=$(dig -p "$COREDNS_PORT_TCP" @127.0.0.1 webapp.app-x.gslb.example.com HTTPS +noall +answer)
    echo "Got HTTPS Response: $resp"
    echo "$resp" | grep -q 'HTTPS'
    echo "$resp" | grep -q 'alpn="h3,h2"'
    echo "$resp" | grep -q 'ipv4hint=172.16.0.10'

    local resp_svcb
    resp_svcb=$(dig -p "$COREDNS_PORT_TCP" @127.0.0.1 webapp.app-x.gslb.example.com SVCB +noall +answer)
    echo "Got SVCB Response: $resp_svcb"
    echo "$resp_svcb" | grep -q 'SVCB'
    echo "$resp_svcb" | grep -q 'alpn="h3,h2"'
    echo "$resp_svcb" | grep -q 'ipv4hint=172.16.0.10'
}
run_test "Check dynamic SVCB and HTTPS resolutions" test_svcb_https_queries

test_geoip_city_us_ca() {
    local ip
    ip=$(dig -p "$COREDNS_PORT_TCP" @127.0.0.1 webapp-geoip-city.app-y.gslb.example.com +short +subnet=9.9.9.9/32)
    echo "Got IP: $ip"
    [ "$ip" = "172.16.0.10" ]
}
run_test "Check GeoIP City - US CA Berkeley Subdivision Fallback (should be 172.16.0.10)" test_geoip_city_us_ca

test_geoip_city_ca_bc() {
    local ip
    ip=$(dig -p "$COREDNS_PORT_TCP" @127.0.0.1 webapp-geoip-city.app-y.gslb.example.com +short +subnet=24.80.0.1/32)
    echo "Got IP: $ip"
    [ "$ip" = "172.16.0.12" ]
}
run_test "Check GeoIP City - CA BC Canada Country Fallback (should be 172.16.0.12)" test_geoip_city_ca_bc

test_geoip_city_eu() {
    local ip
    ip=$(dig -p "$COREDNS_PORT_TCP" @127.0.0.1 webapp-geoip-city.app-y.gslb.example.com +short +subnet=81.185.159.80/32)
    echo "Got IP: $ip"
    [ "$ip" = "172.16.0.13" ]
}
run_test "Check GeoIP City - Europe Continent Fallback (should be 172.16.0.13)" test_geoip_city_eu


# 9. API Integration Tests
echo "=== Running API Tests ==="

test_api_zones() {
    local resp
    resp=$(curl -s -X GET http://127.0.0.1:"$COREDNS_PORT_API"/api/overview | jq 'keys | length')
    echo "Got Nb zones: $resp"
    [ "$resp" = "2" ]
}
run_test "Check API Overview - count zones (should be 2)" test_api_zones

test_api_records() {
    local resp
    resp=$(curl -s -X GET http://127.0.0.1:"$COREDNS_PORT_API"/api/overview | jq 'map(length) | add')
    echo "Got Nb records: $resp"
    [ "$resp" = "10" ]
}
run_test "Check API Overview - count records (should be 10)" test_api_records


# 9b. Dynamic Service Discovery Integration Tests
echo "=== Running Dynamic Discovery Tests ==="

test_consul_discovery() {
    local ip
    ip=$(dig -p "$COREDNS_PORT_TCP" @127.0.0.1 webapp-consul.app-x.gslb.example.com +short)
    echo "Got IP from Consul discovery: $ip"
    [ "$ip" = "172.16.0.10" ]
}
run_test "Check Consul Discovery (should resolve to 172.16.0.10)" test_consul_discovery

test_dns_https_discovery() {
    local ip
    ip=$(dig -p "$COREDNS_PORT_TCP" @127.0.0.1 webapp-dns.app-x.gslb.example.com +short)
    echo "Got IP from DNS HTTPS discovery: $ip"
    [ "$ip" = "172.16.0.12" ]
}
run_test "Check DNS HTTPS Discovery (should resolve to 172.16.0.12)" test_dns_https_discovery


# 9c. CNAME Backend Routing Integration Tests
echo "=== Running CNAME Backend Routing Tests ==="

test_cname_backend_routing() {
    local target
    target=$(dig -p "$COREDNS_PORT_TCP" @127.0.0.1 webapp-cname.app-x.gslb.example.com CNAME +short)
    echo "Got CNAME target: $target"
    [ "$target" = "some-alb.aws.com." ]
}
run_test "Check CNAME backend routing on CNAME query (should resolve to CNAME some-alb.aws.com.)" test_cname_backend_routing

test_cname_backend_routing_a_query() {
    local target
    target=$(dig -p "$COREDNS_PORT_TCP" @127.0.0.1 webapp-cname.app-x.gslb.example.com A +short)
    echo "Got CNAME target on A query: $target"
    [ "$target" = "some-alb.aws.com." ]
}
run_test "Check CNAME backend routing on A query (should resolve to CNAME some-alb.aws.com.)" test_cname_backend_routing_a_query


# 9d. CLI Validation Integration Tests
echo "=== Running CLI Validation Tests ==="

test_cli_validate_success() {
	$COMPOSE_CMD exec -T coredns_gslb gslbctl validate /coredns/db.app-x.gslb.example.com.yml
}
run_test "Check gslbctl validate with valid config" test_cli_validate_success

test_cli_validate_failure() {
	$COMPOSE_CMD exec -T coredns_gslb bash -c "echo 'records:
  test.example.com.:
    backends:
      - address: 1.2.3.4
      - address: 1.2.3.4' > /tmp/invalid_config.yml"

	if $COMPOSE_CMD exec -T coredns_gslb gslbctl validate /tmp/invalid_config.yml; then
		echo "Error: expected gslbctl validate to fail but it succeeded"
		return 1
	else
		echo "gslbctl validate failed as expected"
		return 0
	fi
}
run_test "Check gslbctl validate with invalid config" test_cli_validate_failure

# 10. Output Statistics
echo "=== Integration Test Suite Completed ==="
echo "Total executed integration tests: $TESTS_RUN"
echo "Total passed integration tests: $TESTS_PASSED"
echo "All $TESTS_PASSED integration tests passed successfully!"

