# Running Tests

CoreDNS-GSLB features extensive unit and integration tests to ensure stability, proper protocol responses, and configuration parsing.

## Running Unit Tests

To run the entire unit test suite:
```bash
make test-unit
```

To run a specific test case (e.g., `TestGSLB_PickFailoverBackend`):
```bash
go test -timeout 10s -cover -v . -run TestGSLB_PickFailoverBackend
```

---

## Running Integration Tests

Integration tests validate end-to-end failover behavior, GeoIP features, and API configuration changes using real container environments. The integration test suite is split into two parts:

*   **Basic Integration Tests** (standard features like GeoIP, Failover, API, and DNS discovery):
    ```bash
    make test-integ-basic
    ```
*   **HA/Redis Integration Tests** (shared health checks, locking, and Pub/Sub propagation):
    ```bash
    make test-integ-ha
    ```

To run both integration suites sequentially:
```bash
make test-integration
```

## Troubleshooting Port Conflicts

If some ports (like `8080` for the REST API or `8053` for DNS) are already in use on your host, the Docker integration stack will throw binding errors.

You can override these default ports using environment variables:
```bash
COREDNS_PORT_API=8082 COREDNS_PORT_TCP=8055 make test-integ-basic
```

Supported port environment variables:

- `COREDNS_PORT_API` (default: `8080`)
- `COREDNS_PORT_TCP` (default: `8053`)
- `COREDNS_PORT_UDP` (default: `8053`)
- `COREDNS_PORT_METRICS` (default: `9153`)
