## CoreDNS-GSLB: Selection Modes

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

- **Description:** Returns all healthy backends in random order. 
- **Use case:** Distributes load randomly, useful for stateless services.
- **Example:**
  ```yaml
  mode: "random"
  backends:
    - address: "10.0.0.1"
    - address: "10.0.0.2"
  ```

### GeoIP

- **Description:** Selects the backend(s) closest to the client based on a location map (subnet-to-location mapping), by country, city, or ASN using MaxMind databases. Requires the `geoip_maxmind` or `geoip_custom` options.
- **Matching behavior (current logic):**
  - If `city_db` is loaded and backends define `latitude` and `longitude`, the plugin computes client coordinates from MaxMind and returns the closest healthy+enabled backend for the requested record type.
  - Evaluates sources in this order: `city_db` hierarchy (`city -> subdivision -> country -> continent`), then `country_db` (`country -> continent`), then `asn_db`, then `geoip_custom` location map, then failover fallback.
  - Returns all healthy+enabled matches for `city_db`, `country_db`, and `geoip_custom` steps.
  - Returns only the first healthy+enabled ASN match in `asn_db` step.
  - If the nearest backend is unhealthy/disabled, the next nearest healthy backend is selected.
- **Use case:** Directs users to the nearest datacenter, region, or country for lower latency.
- **Coordinates fields:** `longitude` and `latitude` are supported.
- **Example (distance-based with coordinates):**
  ```yaml
  mode: geoip
  description: GeoIP nearest-backend routing based on MaxMind city coordinates
  backends:
    - address: 172.16.0.10
      description: webapp10
      longitude: -121.8863
      latitude: 37.3382
      enable: true
      healthchecks:
        - https_default
    - address: 172.16.0.11
      description: webapp11
      longitude: -122.2711
      latitude: 37.8044
      enable: true
      healthchecks:
        - https_default
  ```
- **Example (GeoIP city behavior):**
  ```yaml
  mode: geoip
  description: GeoIP-based routing example for multi geo distributed backends
  backends:
    - address: 172.16.0.10
      description: webapp10
      country: US
      subdivision: CA
      city: San Jose
      enable: true
      healthchecks:
        - https_default
    - address: 172.16.0.11
      description: webapp11
      country: US
      subdivision: CA
      city: Oakland
      enable: true
      healthchecks:
        - https_default
    - address: 172.16.0.12
      description: webapp12
      country: CA
      subdivision: BC
      continent: NA
      enable: true
      healthchecks:
        - https_default
    - address: 172.16.0.13
      description: webapp13
      continent: EU
  ```
  Expected behavior for a client in San Jose:
  - First match: `172.16.0.10` (city-level).
  - If `172.16.0.10` is down, fallback can match `172.16.0.11` at subdivision level (`CA`).
  - If US country matches are unavailable, fallback can match `172.16.0.12` at continent level (`NA`).
- **Example (custom-location-based):**
  ```yaml
  mode: "geoip"
  backends:
    - address: "10.0.0.1"
      location: [ "eu-west-1" ]
    - address: "192.168.1.1"
      location: [ "eu-west-2" ]
  ```
  And in your Corefile:
  ```
  gslb {
      geoip_custom location_map.yml
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
      country: [ "FR" ]
    - address: "20.0.0.1"
      country: [ "US" ]
  ```
  And in your Corefile:
  ```
  gslb {
    geoip_maxmind country_db coredns/GeoLite2-Country.mmdb
  }
  ```
- **Example (city-based):**
  ```yaml
  mode: "geoip"
  backends:
    - address: "10.0.0.1"
      city: [ "Paris" ]
    - address: "20.0.0.1"
      city: [ "New York" ]
  ```
  And in your Corefile:
  ```
  gslb {
    geoip_maxmind city_db coredns/GeoLite2-City.mmdb
  }
  ```
- **Example (ASN-based):**
  ```yaml
  mode: "geoip"
  backends:
    - address: "10.0.0.1"
      asn: [ "AS12345" ]
    - address: "20.0.0.1"
      asn: [ "AS67890" ]
  ```
  And in your Corefile:
  ```
  gslb {
    geoip_maxmind asn_db coredns/GeoLite2-ASN.mmdb
  }
  ```

### Weighted

- **Description:** Selects a healthy backend randomly, but proportionally to its `weight` value. A backend with a higher weight will be chosen more often.
- **Use case:** Distribute requests unevenly across backends, e.g. send 80% of traffic to a main server and 20% to a backup, or balance according to server capacity.
- **Example:**
  ```yaml
  mode: "weighted"
  backends:
    - address: "10.0.0.1"
      weight: 8
    - address: "10.0.0.2"
      weight: 2
  ```
  In this example, backend 10.0.0.1 will receive ~80% of the queries, and 10.0.0.2 ~20%.
- **How it works:**
  - Only healthy and enabled backends are considered.
  - If a backend has no `weight` or a weight ≤ 0, it is treated as weight 1 by default.
  - The probability of selection is: `weight / sum(weights of all healthy backends)`.

If no healthy backend matches the client's country or location, the plugin falls back to failover mode.

### Fail-safe behavior (Fallback)

For all selection modes, if **no healthy backends** are available (including during initial startup before the first health checks complete), the plugin implements a **fail-safe mechanism**:

- It returns **all enabled backends** for the requested record type, regardless of their health status.
- This ensures that a DNS response is always provided if possible, rather than returning an error (SERVFAIL) or an empty response.
- Once at least one backend is detected as healthy, the plugin resumes its normal selection logic.
