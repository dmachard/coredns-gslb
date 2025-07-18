# GSLB - CoreDNS plugin

<p align="center">
  <img src="https://goreportcard.com/badge/github.com/dmachard/coredns-gslb" alt="Go Report"/>
  <img src="https://img.shields.io/badge/go%20tests-85-green" alt="Go tests"/>
  <img src="https://img.shields.io/badge/go%20coverage-60%25-green" alt="Go coverage"/>
  <img src="https://img.shields.io/badge/lines%20of%20code-2434-blue" alt="Lines of code"/>
</p>

<p align="center">
  <img src="https://img.shields.io/github/v/release/dmachard/coredns-gslb?logo=github&sort=semver" alt="release"/>
</p>

## Name

*gslb* - A plugin for managing Global Server Load Balancing (GSLB) functionality in CoreDNS. 

## Description

This plugin provides support for GSLB, enabling advanced load balancing and failover mechanisms based on backend health checks and policies. 
It is particularly useful for managing geographically distributed services or for ensuring high availability and resilience.

Unlike many existing solutions, this plugin is designed for non-Kubernetes infrastructures — including virtual machines, bare metal servers, and hybrid environments.

### Features:
- **IPv4 and IPv6 support**
- **EDNS Client Subnet support** to get the real client IP through DNS
- **Adaptive healthcheck intervals**: healthcheck frequency is automatically reduced for records that are not frequently resolved, minimizing unnecessary backend load
- **Automatic configuration reload**: changes to the YAML configuration file are detected and applied live, without restarting CoreDNS
- **Health Checks**:
  - **HTTP(S)**: checks HTTP(S) endpoint health.
  - **TCP**: checks if a TCP connection can be established.
  - **ICMP**: checks if the backend responds to ICMP echo (ping).
  - **MySQL**: checks database status
  - **gRPC**: checks gRPC health service
  - **LUA**: executes a user-defined Lua script for advanced, programmable healthchecks (HTTP, JSON, Prometheus metrics, SSH, and more)
- **Selection Modes**:
  - **Failover**: Routes traffic to the highest-priority available backend
  - **Random**: Distributes traffic randomly across backends
  - **Round Robin**: Cycles through backends in sequence
  - **GeoIP**: Routes clients to the closest backend by location (asn, country, city, custom location)
- **Prometheus/OpenMetrics**:
  - Counters and histograms for all healthchecks (success, failure, duration)

## Syntax

~~~
gslb DB_YAML_FILE [ZONES...] {
    max_stagger_start "120s"
    resolution_idle_timeout "3600s"   # Duration before slow healthcheck (default: 3600s)
    healthcheck_idle_multiplier 10      # Multiplier for slow healthcheck interval (default: 10)
    batch_size_start 100
    geoip_country_maxmind_db /coredns/GeoLite2-Country.mmdb   # Enable GeoIP by country (MaxMind)
    geoip_city_maxmind_db /coredns/GeoLite2-City.mmdb         # Enable GeoIP by city (MaxMind)
    geoip_asn_maxmind_db /coredns/GeoLite2-ASN.mmdb           # Enable GeoIP by ASN (MaxMind)
    geoip_custom_db /coredns/location_map.yml                 # Enable GeoIP by region/subnet (YAML map)
    use_edns_csubnet
}
~~~

* **DB_YAML_FILE** The GSLB configuration file in YAML format. If the path is relative, the path from the *root*
  plugin will be prepended to it.
* **ZONES** Specifies the zones the plugin should be authoritative for. If not provided, the zones from the CoreDNS configuration block are used.

### Configuration Options

* `max_stagger_start`: The maximum staggered delay for starting health checks (default: "120s").
* `resolution_idle_timeout`: The duration to wait before idle resolution times out (default: "3600s").
* `healthcheck_idle_multiplier`: The multiplier for the healthcheck interval when a record is idle (default: 10).
* `batch_size_start`: The number of backends to process simultaneously during startup (default: 100).
* `geoip_country_maxmind_db`: Path to a MaxMind GeoLite2-Country.mmdb file for country-based GeoIP backend selection. Used for `geoip` mode (country-based routing).
* `geoip_city_maxmind_db`: Path to a MaxMind GeoLite2-City.mmdb file for city-based GeoIP backend selection. Used for `geoip` mode (city-based routing).
* `geoip_asn_maxmind_db`: Path to a MaxMind GeoLite2-ASN.mmdb file for ASN-based GeoIP backend selection. Used for `geoip` mode (ASN-based routing).
* `geoip_custom_db`: Path to a YAML file mapping subnets to locations for GeoIP-based backend selection. Used for `geoip` mode (location-based routing).
* `use_edns_csubnet`: If set, the plugin will use the EDNS Client Subnet (ECS) option to determine the real client IP for GeoIP and logging. Recommended for deployments behind DNS forwarders or public resolvers.

## Examples

Load the `gslb.example.com` zone from `db.gslb.example.com` and enable GSLB records on it

~~~ corefile
. {
    file db.gslb.example.com
    gslb gslb_config.example.com.yml gslb.example.com {
        max_stagger_start "120s"
        resolution_idle_timeout "3600s"
        batch_size_start 100
    }
}
~~~

Where `db.gslb.example.com` would contain 

~~~ text
$ORIGIN gslb.example.com.
@       3600    IN      SOA     ns1.example.com. admin.example.com. (
                                2024010101 ; Serial
                                7200       ; Refresh
                                3600       ; Retry
                                1209600    ; Expire
                                3600       ; Minimum TTL
                                )
        3600    IN      NS      ns1.gslb.example.com.
        3600    IN      NS      ns2.gslb.example.com.
~~~

And `gslb_config.example.com.yml` would contain 

~~~ yaml
records:
  webapp.gslb.example.com.:
    mode: "failover"
    record_ttl: 30
    scrape_interval: 10s
    backends:
    - address: "172.16.0.10"
      priority: 1
      healthchecks:
      - type: http
        params:
          port: 443
          uri: "/"
          host: "localhost"
          expected_code: 200
          enable_tls: true
    - address: "172.16.0.11"
      priority: 2
      healthchecks:
      - type: http
        params:
          port: 443
          uri: "/"
          host: "localhost"
          expected_code: 200
          enable_tls: true
~~~

A complete example with all parameters is available in the folder coredns

#### Example backend with all GeoIP location fields

~~~yaml
- address: "172.16.0.12"
  location_country: [ "FR", "US" ]
  location_city: [ "Paris", "London" ]
  location_asn: [ "12345", "67890" ]
  location_custom: [ "eu-west-1" ]
  enable: true
  priority: 1
  healthchecks:
    - type: grpc
      params:
        port: 9090
        service: ""
        timeout: 5s
~~~

- All `location_*` fields must be YAML lists (even if empty or with one value).
- You can leave a list empty (`[ ]`) if you do not want to filter on that dimension.
- This allows flexible matching by country, city, ASN, or custom tags.


## Selection modes

The GSLB plugin supports several backend selection modes, configurable per record via the `mode` parameter in your YAML config. Each mode determines how the plugin chooses which backend(s) to return for a DNS query.

### Failover
- **Description:** Always returns the highest-priority healthy backend. If it becomes unhealthy, the next-highest is used.
- **Use case:** Classic active/passive or prioritized failover.
- **Example:**
  ```yaml
  mode: "failover"
  backends:
    - address: "10.0.0.1"
      priority: 1
    - address: "10.0.0.2"
      priority: 2
  ```

### Round Robin
- **Description:** Cycles through all healthy backends in order, returning a different one for each query.
- **Use case:** Simple load balancing across all available backends.
- **Example:**
  ```yaml
  mode: "roundrobin"
  backends:
    - address: "10.0.0.1"
    - address: "10.0.0.2"
  ```

### Random
- **Description:** Returns a random healthy backend for each query.
- **Use case:** Distributes load randomly, useful for stateless services.
- **Example:**
  ```yaml
  mode: "random"
  backends:
    - address: "10.0.0.1"
    - address: "10.0.0.2"
  ```

### GeoIP
- **Description:** Selects the backend(s) closest to the client based on a location map (subnet-to-location mapping), by country, city, or ASN using MaxMind databases. Requires the `geoip_custom_db`, `geoip_country_maxmind_db`, `geoip_city_maxmind_db`, and/or `geoip_asn_maxmind_db` options.
- **Use case:** Directs users to the nearest datacenter, region, or country for lower latency.
- **Example (custom-location-based):**
  ```yaml
  mode: "geoip"
  backends:
    - address: "10.0.0.1"
      location_custom: [ "eu-west-1" ]
    - address: "192.168.1.1"
      location_custom: [ "eu-west-2" ]
  ```
  And in your Corefile:
  ```
  gslb gslb_config.example.com.yml gslb.example.com {
      geoip_custom_db location_map.yml
  }
  ```
  And in `location_map.yml`:
  ```yaml
  subnets:
    - subnet: "10.0.0.0/24"
      location: [ "eu-west" ]
    - subnet: "192.168.1.0/24"
      location: [ "us-east" ]
  ```
- **Example (country-based):**
  ```yaml
  mode: "geoip"
  backends:
    - address: "10.0.0.1"
      location_country: [ "FR" ]
    - address: "20.0.0.1"
      location_country: [ "US" ]
  ```
  And in your Corefile:
  ```
  gslb gslb_config.example.com.yml gslb.example.com {
      geoip_maxmind_db coredns/GeoLite2-Country.mmdb
  }
  ```
- **Example (city-based):**
  ```yaml
  mode: "geoip"
  backends:
    - address: "10.0.0.1"
      location_city: [ "Paris" ]
    - address: "20.0.0.1"
      location_city: [ "New York" ]
  ```
  And in your Corefile:
  ```
  gslb gslb_config.example.com.yml gslb.example.com {
      geoip_maxmind_db coredns/GeoLite2-City.mmdb
  }
  ```
- **Example (ASN-based):**
  ```yaml
  mode: "geoip"
  backends:
    - address: "10.0.0.1"
      location_asn: [ "AS12345" ]
    - address: "20.0.0.1"
      location_asn: [ "AS67890" ]
  ```
  And in your Corefile:
  ```
  gslb gslb_config.example.com.yml gslb.example.com {
      geoip_maxmind_db coredns/GeoLite2-ASN.mmdb
  }
  ```

If no healthy backend matches the client's country or location, the plugin falls back to failover mode.

## Health Checks

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

### ICMP

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

### Lua

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
## Observability

### Metrics

If you enable the `prometheus` block in your Corefile, the plugin exposes the following metrics on `/metrics` (default port 9153):

- `gslb_healthcheck_total{type, address, result}`: Total number of healthchecks performed, labeled by type, backend address, and result (success/fail).
- `gslb_healthcheck_duration_seconds{type, address}`: Duration of healthchecks in seconds, labeled by type and backend address.

Example Corefile block:

~~~
. {
    prometheus
    ...
}
~~~

You can then scrape metrics at http://localhost:9153/metrics

## High Availability and Scalability

For production environments requiring high availability and scalability, 
the CoreDNS-GSLB can be deployed as below to ensure resilience and performance

For Multi-Datacenter Deployment
In this model:
  - Each CoreDNS-GSLB instance is deployed with the same configuration across datacenters.
  - All GSLB nodes monitor the same backend pool, ensuring consistent health-based decisions regardless of location.
  - GeoDNS logic (via EDNS Client Subnet and GeoIP) allows each instance to respond optimally from its point of view.

```
              DNS Query for gslb.example.com
                             │
                             ▼
                    ┌──────────────────┐
                    │ Authoritative NS │
                    │   ns1 / ns2      │
                    └────────┬─────────┘
                             │ Delegation to:
         ┌───────────────────┴──────────────────┐
         ▼                                      ▼
┌───────────────────┐              ┌───────────────────┐
│   Datacenter 1    │              │   Datacenter 2    │
│                   │              │                   │
│  ┌─────────────┐  │              │  ┌─────────────┐  │
│  │  dnsdist    │  │              │  │  dnsdist    │  │
│  │ with cache  │  │              │  │ with cache  │  │
│  └─────┬───────┘  │              │  └─────┬───────┘  │
│        │          │              │        │          │
│    ┌───┴───┐      │              │    ┌───┴───┐      │
│    │CoreDNS│      │              │    │CoreDNS│      │
│    │GSLB   │      │              │    │GSLB   │      │
│    └───┬───┘      │              │    └───┬───┘      │
│        │          │              │        │          │
└───────────────────┘              └───────────────────┘
         │                                  │           
         ▼                                  ▼           
 ┌────────────────────────────────────────────────────┐
 │                 Backends to check                  │
 │           web1.dc1.com   web1.dc2.com              │
 │           web2.dc1.com    web2.dc2.com             │
 │           api1.dc1.com    api1.dc2.com             │
 └────────────────────────────────────────────────────┘
```

Per-Datacenter Scalability Model

```
                    ┌─────────────────┐
                    │   dnsdist       │
                    │ (Load Balancer) │
                    └─────────┬───────┘
                             │
        ┌────────────────────┼────────────────────┐
        │                    │                    │
        ▼                    ▼                    ▼
┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐
│ CoreDNS-GSLB/1  │  │ CoreDNS-GSLB/2  │  │ CoreDNS-GSLB/3  │
│ Zones: A, B     │  │ Zones: C, D     │  │ Zones: E, F     │
└─────────────────┘  └─────────────────┘  └─────────────────┘
        │                    │                    │
        ▼                    ▼                    ▼
┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐
│ Backend Pool 1  │  │ Backend Pool 2  │  │ Backend Pool 3  │
│ web1.dc1.com    │  │ api1.dc1.com    │  │ db1.dc1.com     │
│ web2.dc1.com    │  │ api2.dc1.com    │  │ db2.dc1.com     │
└─────────────────┘  └─────────────────┘  └─────────────────┘
```

**Benefits:**
- **Horizontal scalability**: Add more CoreDNS instances as needed
- **Zone isolation**: Each CoreDNS instance handles specific zones
- **Load balancing**: dnsdist distributes queries intelligently
- **Fault tolerance**: If one CoreDNS fails, others continue serving their zones
- **Resource optimization**: Each instance optimized for its zone workload

## Troubleshooting

### To log Health Checks

Example Corefile block:

~~~
. {
    # To log healthcheck results
    debug
}
~~~

### TXT Record Support for Debugging

The GSLB plugin supports DNS TXT queries for any managed domain. When you query a domain with type TXT, the plugin returns a TXT record for each backend, summarizing:
- Backend address (IP)
- Priority
- Health status (healthy/unhealthy)
- Enabled status (true/false)

This feature is useful for debugging and monitoring: you can instantly see the state of all backends for a domain with a single DNS TXT query.

**Example:**

```
dig TXT webapp.gslb.example.com.
```

**Sample response:**

```
webapp.gslb.example.com. 30 IN TXT "Backend: 172.16.0.10 | Priority: 1 | Status: healthy | Enabled: true"
webapp.gslb.example.com. 30 IN TXT "Backend: 172.16.0.11 | Priority: 2 | Status: unhealthy | Enabled: true"
```

This makes it easy to monitor backend health and configuration in real time using standard DNS tools.


## Contributions

Contributions are welcome!
Please read the [Developer Guide](CONTRIBUTING.md) for local setup and testing instructions.
