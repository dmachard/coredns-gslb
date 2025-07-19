
## CoreDNS-GSLB: Health Checks


The GSLB plugin supports several types of health checks for backends. Each type can be configured per backend in the YAML configuration file.

Additionally, the GSLB plugin automatically adapts the healthcheck interval for each DNS record based on recent resolution activity.

- If a record is not resolved (queried) for a duration longer than `resolution_idle_timeout`, the healthcheck interval for its backends is multiplied by `healthcheck_idle_multiplier` (default: 10, configurable in the Corefile).
- As soon as a DNS query is received for the record, the interval returns to its normal value (`scrape_interval`).
- This mechanism reduces unnecessary healthcheck traffic for rarely used records, while keeping healthchecks frequent for active records.

**Example:**
- `scrape_interval: 10s`, `resolution_idle_timeout: 3600s`, `healthcheck_idle_multiplier: 10`
- If no DNS query is received for 1 hour, healthchecks run every 100s instead of every 10s.
- When a query is received, healthchecks resume every 10s.

This feature helps optimize resource usage and backend load in large or dynamic environments.

### HTTP(S)

Checks the health of an HTTP or HTTPS endpoint by making a request and validating the response code and/or body.

```yaml
healthchecks:
  - type: http
    params:
      port: 443                # Port to connect (443 for HTTPS, 80 for HTTP)
      uri: "/"                 # URI to request
      method: "GET"            # HTTP method
      host: "localhost"        # Host header for the request
      headers:                 # Additional HTTP headers (key-value pairs)
      timeout: 5s              # Timeout for the HTTP request
      expected_code: 200       # Expected HTTP status code
      expected_body: ""        # Expected response body (empty means no body validation)
      enable_tls: true         # Use TLS for the health check (HTTPS)
      skip_tls_verify: true    # Skip TLS certificate validation
```

### TCP

Checks if a TCP connection can be established to the backend on a given port.

```yaml
healthchecks:
  - type: tcp
    params:
      port: 80         # TCP port to connect to
      timeout: "3s"    # Connection timeout
```

### ICMP (Ping)

Checks if the backend responds to ICMP echo requests (ping).

```yaml
healthchecks:
  - type: icmp
    params:
      timeout: 2s   # Timeout for the ICMP request
      count:  3     # Number of ICMP requests to send
```

### MySQL

Checks MySQL server health by connecting and executing a query.

```yaml
healthchecks:
  - type: mysql
    params:
      host: "10.0.0.5"         # MySQL server address
      port: 3306               # MySQL port
      user: "gslbcheck"        # Username
      password: "secret"       # Password
      database: "test"         # Database to connect
      timeout: "3s"            # Connection/query timeout
      query: "SELECT 1"        # Query to execute (optional, default: SELECT 1)
```

### gRPC

Checks the health of a gRPC service using the standard gRPC health checking protocol (`grpc.health.v1.Health/Check`).

```yaml
healthchecks:
  - type: grpc
    params:
      port: 9090                # gRPC port to connect to
      service: "grpc.health.v1.Health" # Service name (default: "")
      timeout: 5s               # Timeout for the gRPC request
```

- `service` can be left empty to check the overall server health, or set to a specific service name.


### Lua Scripting

Executes an embedded Lua script to determine the backend health. The script can use the helper functions http_get(url) and json_decode(str) to perform HTTP requests and parse JSON. The global variable 'backend' provides the backend's address and priority.

**Available helpers:**
- `http_get(url, [timeout_sec], [user], [password], [tls_verify])`: Performs an HTTP(S) GET request. Optional timeout (seconds), HTTP Basic auth (user, password), and TLS verification (default true).
- `json_decode(str)`: Parses a JSON string and returns a Lua table (or nil on error).
- `metric_get(url, metric_name, [timeout_sec], [tls_verify], [user], [password])`: Fetches the value of a Prometheus metric from a /metrics endpoint (returns the first value found as a number or string, or nil if not found). Optional timeout (seconds), TLS verification (default true), and HTTP Basic auth (user, password).
- `ssh_exec(host, user, password, command, [timeout_sec])`: Executes a command via SSH and returns the output as a string. Optional timeout (seconds).
- `backend`: A Lua table with fields:
    - `address`: the backend's address (string)
    - `priority`: the backend's priority (number)


**Example: Use http_get and json_decode**
```yaml
healthchecks:
  - type: lua
    params:
      timeout: 5s
      script: |
        local health = json_decode(http_get("http://" .. backend.address .. ":9200/_cluster/health"))
        if health and health.status == "green" and health.number_of_nodes >= 3 then
          return true
        else
          return false
        end
```

**Example: Get a Prometheus metric value**
```yaml
healthchecks:
  - type: lua
    params:
      timeout: 5s
      script: |
        local value = metric_get("http://myapp:9100/metrics", "nginx_connections_active")
        if value and value < 100 then
          return true
        end
        return false
```

**Example: Check a process via SSH**
```yaml
healthchecks:
  - type: lua
    params:
      timeout: 5s
      script: |
        local output = ssh_exec("10.0.0.5", "monitor", "secret", "pgrep nginx")
        if output and output ~= "" then
          return true
        else
          return false
        end
```

**Example: metric_get with timeout and skip TLS verification**
```yaml
healthchecks:
  - type: lua
    params:
      timeout: 5s
      script: |
        local value = metric_get("https://myapp:9100/metrics", "nginx_connections_active", 2, false)
        if value and value < 100 then
          return true
        end
        return false
```

**Example: ssh_exec with timeout**
```yaml
healthchecks:
  - type: lua
    params:
      timeout: 5s
      script: |
        local out = ssh_exec("10.0.0.5", "user", "pass", "pgrep nginx", 3)
        if out ~= "" then
          return true
        end
        return false
```

**Example: metric_get with HTTP Basic authentication**
```yaml
healthchecks:
  - type: lua
    params:
      timeout: 5s
      script: |
        local value = metric_get("https://myapp:9100/metrics", "nginx_connections_active", 2, true, "user", "pass")
        if value and value < 100 then
          return true
        end
        return false
```