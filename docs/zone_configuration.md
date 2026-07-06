# Zone YAML Reference

While the Corefile configures CoreDNS startup flags, global API settings, and databases, the actual GSLB DNS records and their corresponding backend endpoints are defined inside YAML configuration files configured per-zone.

CoreDNS-GSLB automatically monitors these zone files and reloads them at runtime when changes are detected.

---

## File Structure Overview

A GSLB zone YAML file is composed of three main root sections:

1. **`defaults`** (Optional): A block containing default parameter values inherited by all records in this zone.
2. **`healthcheck_profiles`** (Optional): A dictionary of named health check templates that can be referenced by backends.
3. **`records`** (Required): A dictionary containing the actual GSLB domain names (must end with a trailing dot) and their routing policies.

```yaml
# 1. Defaults
defaults:
  owner: admin
  record_ttl: 30
  scrape_interval: 10s
  scrape_timeout: 5s

# 2. Healthcheck Profiles
healthcheck_profiles:
  http_check:
    type: http
    params:
      port: 80
      uri: "/healthz"

# 3. Records
records:
  api.example.org.:
    mode: failover
    backends:
      - address: "192.168.1.10"
        priority: 1
        healthchecks: [ http_check ]
      - address: "192.168.1.11"
        priority: 2
        healthchecks: [ http_check ]
```

---

## Reusable Record Defaults

The `defaults` section allows you to define parameters once and have them automatically inherited by all records under the `records` block. Any record can explicitly override these values.

### Defaults Parameters

| Parameter | Type | Default | Description |
| :--- | :--- | :--- | :--- |
| `owner` | string | `""` | Optional metadata to document the owner/maintainer of the records. |
| `description` | string | `""` | Optional description of the records. |
| `record_ttl` | integer | `30` | The DNS Time-to-Live (TTL) in seconds returned to DNS clients. |
| `scrape_interval` | duration | `"10s"` | How often health check scrapers run (e.g. `"10s"`, `"30s"`, `"1m"`). |
| `scrape_retries` | integer | `1` | Number of consecutive failures before marking a backend down. |
| `scrape_timeout` | duration | `"5s"` | Maximum duration to wait for a single health check probe. |
| `alpn` | list of strings | `[]` | List of ALPN protocols advertised in HTTPS/SVCB queries. |

### Defaults Override Example

```yaml
defaults:
  owner: infra-team
  record_ttl: 60

records:
  # Inherits owner: infra-team, record_ttl: 60
  service-a.example.org.:
    mode: round-robin
    backends:
      - address: "192.168.10.5"

  # Overrides record_ttl: 300, inherits owner: infra-team
  service-b.example.org.:
    mode: failover
    record_ttl: 300
    backends:
      - address: "192.168.20.5"
```

---

## Records Reference

The `records` block contains the mapping of fully-qualified domain names (FQDNs) to their load-balancing settings and backends.

!!! important "Important"
    All record keys in the YAML file MUST end with a trailing dot (e.g., `api.example.org.`). If you omit the dot, the record will fail to match incoming DNS queries.

### Record Parameters

| Parameter | Type | Default | Description |
| :--- | :--- | :--- | :--- |
| `mode` | string | `"failover"` | The selection mode/algorithm: `failover`, `round-robin`, `weighted`, `random`, `geoip`, `ip-hash`. |
| `backends` | list of backends | `[]` | List of backend endpoints participating in this record (see [Backend Parameters](#backend-parameters)). |
| `fallback_policy` | object | *None* | Configuration determining behavior when all backends are offline (see [Fallback Policy](#fallback-policy)). |
| `discovery` | object | *None* | External service discovery configuration (see [Backend Discovery](discovery.md)). |
| `alpn` | list of strings | `[]` | Overrides the default ALPN list for SVCB/HTTPS resource records. |
| `owner` | string | *Inherited* | Overrides the default owner value. |
| `description` | string | *Inherited* | Overrides the default description value. |
| `record_ttl` | integer | *Inherited* | Overrides the default record TTL. |
| `scrape_interval` | duration | *Inherited* | Overrides the default scrape interval. |
| `scrape_retries` | integer | *Inherited* | Overrides the default scrape retries. |
| `scrape_timeout` | duration | *Inherited* | Overrides the default scrape timeout. |

---

### Fallback Policy

When all backends for a record are determined to be unhealthy or disabled, the GSLB plugin applies the `fallback_policy` to decide how to respond.

```yaml
records:
  app.example.org.:
    mode: round-robin
    fallback_policy:
      mode: fallback_ips # can be: fallback_ips, fallback_cname, rcode
      fallback_ips:
        - "192.0.2.1"
      fallback_cname: "status.example.org."
      rcode: "SERVFAIL"
```

| Parameter | Type | Default | Description |
| :--- | :--- | :--- | :--- |
| `mode` | string | `"rcode"` | Determines the fallback mechanism. Must be one of:<br>• `fallback_ips`: Returns a static list of IP addresses.<br>• `fallback_cname`: Redirects to a fallback canonical name (CNAME).<br>• `rcode`: Returns a specific DNS response code. |
| `fallback_ips` | list of strings | `[]` | The list of IPv4/IPv6 addresses returned when `mode` is `fallback_ips`. |
| `fallback_cname` | string | `""` | The target FQDN returned when `mode` is `fallback_cname`. |
| `rcode` | string | `"SERVFAIL"` | The DNS response code returned when `mode` is `rcode` (e.g. `SERVFAIL`, `NXDOMAIN`, `REFUSED`). |

---

## Backend Parameters

Backends represent the actual destination endpoints (IP addresses or CNAME targets) that CoreDNS-GSLB serves to clients.

```yaml
backends:
  - address: "192.168.1.10"
    port: 443
    priority: 1
    weight: 50
    enable: true
    assume_healthy: false
    passive: false
    rise: 2
    fall: 3
    healthchecks:
      - type: http
        params:
          port: 443
          uri: "/healthz"
          enable_tls: true
```

### Backend Fields

| Parameter | Type | Default | Description |
| :--- | :--- | :--- | :--- |
| `address` | string | `"127.0.0.1"` | The IP address or CNAME target of the backend. |
| `port` | integer | `0` | Port number of the service (used for healthchecks and SVCB/SRV hints). |
| `priority` | integer | `0` | Lower values represent higher priority in `failover` routing. |
| `weight` | integer | `1` | Weight representation for `weighted` load balancing mode. |
| `enable` | boolean | `true` | Set to `false` to immediately disable the backend and stop traffic. |
| `description` | string | `""` | Optional documentation field. |
| `tags` | list of strings | `[]` | List of tags used to group or filter backends. |
| `timeout` | duration | `"5s"` | Health check timeout override specific to this backend. |
| `assume_healthy` | boolean | `false` | If `true`, health checks are bypassed and this backend is always considered healthy. |
| `passive` | boolean | `false` | If `true`, the instance will not probe this backend locally. It will instead read its status from a shared Redis instance. |
| `rise` | integer | `2` | Number of consecutive successful probes required to transition the backend from unhealthy to healthy. |
| `fall` | integer | `3` | Number of consecutive failed probes required to transition the backend from healthy to unhealthy. |
| `healthchecks` | list of objects | `[]` | Local health check configurations (see [Health Checks Reference](healthchecks.md)). |

!!! note "Tip"
    Use `assume_healthy: true` for backends that you know are highly reliable and do not need to be probed continuously.

### Geographical Routing

For records using `mode: geoip`, backends can be matched based on geographic client location using these parameters:

| Parameter | Type | Default | Description |
| :--- | :--- | :--- | :--- |
| `location` | string | `""` | Custom location identifier mapping to `geoip_custom` rules. |
| `continent` | string | `""` | Two-character continent code (e.g. `EU`, `NA`, `AS`). |
| `country` | string | `""` | Two-character ISO country code (e.g. `FR`, `US`, `CN`). |
| `subdivision` | string | `""` | State/region/subdivision code (e.g. `CA`, `NY`). |
| `city` | string | `""` | City name (e.g. `Paris`, `New York`). |
| `asn` | string | `""` | Autonomous System Number (e.g. `AS15169`). |
| `longitude` | float | `0.0` | Longitude for coordinates-based proximity routing. |
| `latitude` | float | `0.0` | Latitude for coordinates-based proximity routing. |

---

## Full Example

Below is a complete GSLB zone YAML file demonstrating defaults inheritance, custom health checks, standard failover routing, fallback policies, and GeoIP configuration.

```yaml
defaults:
  owner: core-infrastructure
  record_ttl: 60
  scrape_interval: 15s
  scrape_timeout: 5s
  scrape_retries: 2

healthcheck_profiles:
  http_check:
    type: http
    rise: 2
    fall: 3
    params:
      port: 8080
      uri: "/health"
      expected_code: 200

records:
  # 1. Round-Robin Load Balanced Web Service
  www.example.org.:
    mode: round-robin
    backends:
      - address: "192.0.2.10"
        port: 8080
        healthchecks: [ http_check ]
      - address: "192.0.2.11"
        port: 8080
        healthchecks: [ http_check ]

  # 2. Geographically Routed Database API
  api.example.org.:
    mode: geoip
    fallback_policy:
      mode: fallback_ips
      fallback_ips:
        - "198.51.100.1"
    backends:
      # European Clients
      - address: "203.0.113.10"
        continent: "EU"
        priority: 1
        healthchecks: [ http_check ]
      # North American Clients
      - address: "203.0.113.20"
        continent: "NA"
        priority: 1
        healthchecks: [ http_check ]
```
