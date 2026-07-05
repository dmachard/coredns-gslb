# Corefile Reference

This document provides a comprehensive reference of all Corefile configuration parameters available inside the `gslb` block.

CoreDNS-GSLB automatically watches its configuration files and reloads them at runtime when changes are detected. This allows most configuration updates to be applied without restarting CoreDNS.

---

## Syntax Overview

```corefile
gslb {
    # Zone-to-file mappings
    zone example.org.   /etc/coredns/db.example.org.yml

    # GeoIP MaxMind databases
    geoip_maxmind country_db /coredns/GeoLite2-Country.mmdb
    geoip_maxmind city_db    /coredns/GeoLite2-City.mmdb
    geoip_maxmind asn_db     /coredns/GeoLite2-ASN.mmdb
    geoip_custom             /coredns/location_map.yml
    
    # DNS Features
    use_edns_csubnet
    disable_txt

    # Scrapers and Healthchecks Startup Settings
    max_stagger_start "120s"
    batch_size_start 100

    # Scraper Idle optimization
    resolution_idle_timeout "3600s"
    healthcheck_idle_multiplier 10
    
    # REST API Configuration
    api_enable true
    api_tls_cert ""
    api_tls_key ""
    api_listen_addr 0.0.0.0
    api_listen_port 8080
    api_basic_user admin
    api_basic_pass secret

    # Global Healthcheck Profiles Reference File
    healthcheck_profiles /etc/coredns/healthcheck_profiles.yml
}
```

---

## Configuration Parameters

### Zone Definitions
* **`zone <fqdn> <path>`** (Required, repeatable): Maps an authoritative DNS zone (must end with a trailing dot) to its corresponding YAML configuration file containing records.

### GeoIP Settings
* **`geoip_maxmind <type> <path>`**: Loads a MaxMind GeoIP2 database. 
  * `<type>` can be:
    * `country_db`: Used for matching by `country` (two-letter ISO code) and `continent`.
    * `city_db`: Used for matching by coordinates (`latitude` and `longitude`), `city`, `subdivision`, `country`, and `continent`.
    * `asn_db`: Used for matching by Autonomous System Number (`asn`).
  * **Block Syntax Support**: You can also use a block format:
    ```corefile
    geoip_maxmind {
        country_db /coredns/GeoLite2-Country.mmdb
        city_db    /coredns/GeoLite2-City.mmdb
        asn_db     /coredns/GeoLite2-ASN.mmdb
    }
    ```
* **`geoip_custom <path>`**: Path to a YAML file mapping subnets to custom locations for custom-scoped geographical routing. Subnets are pre-parsed on load and configuration reloads to avoid runtime overhead.

### DNS & Protocol Flags
* **`use_edns_csubnet`**: Enables EDNS Client Subnet (ECS, RFC 7871) support. When active, GSLB uses the client's subnet instead of the DNS resolver IP to perform geo-routing. See the [ECS and Caching Guide](ecs.md) for details.
* **`disable_txt`**: Disables automatic TXT record generation for GSLB-managed domains. If set, TXT queries are passed to the next plugin or return empty.

### Startup & Timing Tuning
* **`max_stagger_start <duration>`**: The maximum duration to randomize/stagger the start of background healthchecks (default: `"120s"`).
* **`batch_size_start <number>`**: The number of backend healthcheck scrapers to boot up concurrently during startup (default: `100`).
* **`resolution_idle_timeout <duration>`**: The duration after which a record is considered idle if it receives no DNS queries (default: `"3600s"`).
* **`healthcheck_idle_multiplier <multiplier>`**: The factor by which a backend's health check scrape interval is multiplied when the record becomes idle (default: `10`).

### REST API Options
* **`api_enable <bool>`**: Enables or disables the REST API server (default: `true`).
* **`api_listen_addr <ip>`**: The bind IP address for the REST API server (default: `0.0.0.0`).
* **`api_listen_port <port>`**: The bind port for the REST API server (default: `8080`).
* **`api_tls_cert <path>`**: Path to the TLS certificate file (enables HTTPS if set with `api_tls_key`).
* **`api_tls_key <path>`**: Path to the TLS private key file (enables HTTPS if set with `api_tls_cert`).
* **`api_basic_user <username>`**: Username to enforce HTTP Basic Auth on API requests (optional).
* **`api_basic_pass <password>`**: Password to enforce HTTP Basic Auth on API requests (optional).

### Reusable Profiles File
* **`healthcheck_profiles <path>`**: Path to a YAML file containing global health check profiles shared across all zone files. See the [Health Checks Guide](healthchecks.md) for structure.

---

## Zone YAML Configuration

While the Corefile configures CoreDNS startup flags, api settings, and databases, the actual GSLB DNS records and their corresponding backends are defined inside YAML files configured per-zone.

### File Structure Overview

A GSLB zone YAML file is composed of three main root sections:

1. **`defaults`** (Optional): A block containing default parameter values inherited by all records in this zone.
2. **`healthcheck_profiles`** (Optional): A dictionary of named health check templates that can be referenced in record backends.
3. **`records`** (Required): A dictionary containing the actual GSLB domain names (must end with a trailing dot) and their routing policies.

```yaml
# 1. Defaults
defaults:
  owner: admin
  record_ttl: 30
  scrape_interval: 10s

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

### Reusable Record Defaults

Any parameter defined in the `defaults` block is automatically applied to all records under the `records` block, unless a record explicitly overrides it.

Supported default fields:

* **`owner`** (string): Metadata to document the owner/maintainer.
* **`record_ttl`** (int): The DNS TTL (in seconds) returned to clients (default: `30`).
* **`scrape_interval`** (duration): How often health check scrapers run (default: `"10s"`).
* **`scrape_retries`** (int): Number of failures required to mark a backend down (default: `1`).
* **`scrape_timeout`** (duration): Max time to wait for a health check probe (default: `"5s"`).
* **`alpn`** (list of strings): List of ALPN protocols advertised in SVCB/HTTPS queries.

Overriding Example:

```yaml
defaults:
  owner: admin
  record_ttl: 30

records:
  web1.example.org.:
    mode: failover
    # Inherits: owner="admin", record_ttl=30
  web2.example.org.:
    mode: failover
    owner: alice    # Overrides default owner
    record_ttl: 60  # Overrides default TTL
```

