# GSLB - CoreDNS plugin

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
  - HTTP(S): checks HTTP(S) endpoint health.
  - TCP: checks if a TCP connection can be established.
  - ICMP: checks if the backend responds to ICMP echo (ping).
  - Custom script: executes a custom shell script
- **Selection Modes**:
  - **Failover**: Routes traffic to the highest-priority available backend
  - **Random**: Distributes traffic randomly across backends
  - **Round Robin**: Cycles through backends in sequence
  - **GeoIP**: Routes clients to the closest backend by location (datacenter, region, etc.)
- **Prometheus/OpenMetrics**:
  - Counters and histograms for all healthchecks (success, failure, duration)

## Syntax

~~~
gslb DB_YAML_FILE [ZONES...] {
    max_stagger_start "120s"
    resolution_idle_timeout "3600s"
    batch_size_start 100
    location_db location_map.yml
    use_edns_csubnet
}
~~~

* **DB_YAML_FILE** The GSLB configuration file in YAML format. If the path is relative, the path from the *root*
  plugin will be prepended to it.
* **ZONES** Specifies the zones the plugin should be authoritative for. If not provided, the zones from the CoreDNS configuration block are used.

### Configuration Options

* `max_stagger_start`: The maximum staggered delay for starting health checks (default: "120s").
* `resolution_idle_timeout`: The duration to wait before idle resolution times out (default: "3600s").
* `batch_size_start`: The number of backends to process simultaneously during startup (default: 100).
* `location_db`: Path to a YAML file mapping subnets to locations for GeoIP-based backend selection. Required for `geoip` mode.
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
- **Description:** Selects the backend(s) closest to the client based on a location map (subnet-to-location mapping). Requires the `location_db` option and a YAML map file.
- **Use case:** Directs users to the nearest datacenter or region for lower latency.
- **Example:**
  ```yaml
  mode: "geoip"
  backends:
    - address: "10.0.0.1" # e.g. EU
    - address: "192.168.1.1" # e.g. US
  ```
  And in your Corefile:
  ```
  gslb gslb_config.example.com.yml gslb.example.com {
      location_db location_map.yml
  }
  ```
  And in `location_map.yml`:
  ```yaml
  subnets:
    - subnet: "10.0.0.0/24"
      location: "eu-west"
    - subnet: "192.168.1.0/24"
      location: "us-east"
  ```

If no healthy backend matches the client's location, the plugin falls back to failover mode.

## Monitoring & Health Checks

### Custom Health Check

The script should return exit code 0 for healthy, non-zero for unhealthy.
Environment variables available:
  - BACKEND_ADDRESS
  - BACKEND_PRIORITY

Timeout for the script is 5s.

### Adaptive Healthcheck Intervals

The GSLB plugin automatically adapts the healthcheck interval for each DNS record based on recent resolution activity.

- If a record is not resolved (queried) for a duration longer than `resolution_idle_timeout`, the healthcheck interval for its backends is multiplied by 10 (slowed down).
- As soon as a DNS query is received for the record, the interval returns to its normal value (`scrape_interval`).
- This mechanism reduces unnecessary healthcheck traffic for rarely used records, while keeping healthchecks frequent for active records.

**Example:**
- `scrape_interval: 10s`, `resolution_idle_timeout: 3600s`
- If no DNS query is received for 1 hour, healthchecks run every 100s instead of every 10s.
- When a query is received, healthchecks resume every 10s.

This feature helps optimize resource usage and backend load in large or dynamic environments.

### Metrics

If you enable the `prometheus` block in your Corefile, the plugin exposes the following metrics on `/metrics` (default port 9153):

- `gslb_healthcheck_total{type, address, result}`: Total number of healthchecks performed, labeled by type (http, tcp, icmp, custom), backend address, and result (success/fail).
- `gslb_healthcheck_duration_seconds{type, address}`: Duration of healthchecks in seconds, labeled by type and backend address.

Example Corefile block:

~~~
. {
    prometheus
    ...
}
~~~

You can then scrape metrics at http://localhost:9153/metrics


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
