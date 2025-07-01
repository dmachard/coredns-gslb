# GSLB - CoreDNS plugin

## Name

*gslb* - A plugin for managing Global Server Load Balancing (GSLB) functionality in CoreDNS. 

## Description

This plugin provides support for GSLB, enabling advanced load balancing and failover mechanisms based on backend health checks and policies. 
It is particularly useful for managing geographically distributed services or for ensuring high availability and resilience.

Unlike many existing solutions, this plugin is designed for non-Kubernetes infrastructures — including virtual machines, bare metal servers, and hybrid environments.

### Features:
- **IPv4 and IPv6 support**
- **EDNS Client Subnet support** to get the real client IP
- **Health Checks**:
  - HTTP(S): checks HTTP(S) endpoint health.
  - TCP: checks if a TCP connection can be established.
  - ICMP: checks if the backend responds to ICMP echo (ping).
  - Custom script: executes a custom shell script
- **Selection Modes**:
  - **Failover**: Routes traffic to the highest-priority available backend
  - **Random**: Distributes traffic randomly across backends
  - **Round Robin**: Cycles through backends in sequence
- **Prometheus/OpenMetrics**:
  - Counters and histograms for all healthchecks (success, failure, duration)

## Syntax

~~~
gslb DB_YAML_FILE [ZONES...] {
    max_stagger_start "120s"
    resolution_idle_timeout "3600s"
    batch_size_start 100
}
~~~

* **DB_YAML_FILE** The GSLB configuration file in YAML format. If the path is relative, the path from the *root*
  plugin will be prepended to it.
* **ZONES** Specifies the zones the plugin should be authoritative for. If not provided, the zones from the CoreDNS configuration block are used.

### Configuration Options

* `max_stagger_start`: The maximum staggered delay for starting health checks (default: "120s").
* `resolution_idle_timeout`: The duration to wait before idle resolution times out (default: "3600s").
* `batch_size_start`: The number of backends to process simultaneously during startup (default: 100).

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

## Custom Health Check

The script should return exit code 0 for healthy, non-zero for unhealthy.
Environment variables available:
  - BACKEND_ADDRESS
  - BACKEND_FQDN
  - BACKEND_PRIORITY
  - BACKEND_ENABLE
Timeout for the script is 5s.

## Metrics (Prometheus/OpenMetrics)

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

## Compilation

The `GSLB` plugin must be integrated into CoreDNS during compilation.

1. Add the following line to plugin.cfg before the file plugin. It is recommended to put this plugin right before **file:file**

~~~ text
gslb:github.com/dmachard/coredns-gslb
~~~

2. Recompile CoreDNS:

~~~ bash
go generate
make
~~~

## Compilation with Docker compose

Build CoreDNS with the plugin

~~~ bash
sudo docker compose --progress=plain build
~~~

Start the stack (CoreDNS + webapps)

~~~ bash
sudo docker compose up -d 
~~~

Wait some seconds and test the DNS resolution

~~~ bash
$ dig -p 8053 @127.0.0.1 webapp.gslb.example.com +short
172.16.0.10
~~~

### Simulate Failover

Stop the webapp 1

~~~ bash
sudo docker compose stop webapp10
~~~

Wait 30 seconds, then resolve again:

~~~ bash
$ dig -p 8053 @127.0.0.1 webapp.gslb.example.com +short
172.16.0.11
~~~

Restart Webapp 1:

~~~ bash
sudo docker compose start webapp10
~~~

Wait a few seconds, then resolve again to observe traffic switching back to Webapp 1:

~~~ bash
$ dig -p 8053 @127.0.0.1 webapp.gslb.example.com +short
172.16.0.10
~~~

## Testing

Run a specific test

~~~ bash
go test -timeout 10s -cover -v . -run TestGSLB_PickFailoverBackend
~~~
