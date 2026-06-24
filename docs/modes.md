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
  - If `city_db` is loaded and backends define `latitude` and `longitude`, the plugin computes client coordinates from MaxMind and returns the closest healthy+enabled backend(s) for the requested record type. If multiple backends share the same nearest coordinates (same datacenter) and have the same priority, all of them are returned.
  - Evaluates sources in this order: `city_db` hierarchy (`city -> subdivision -> country -> continent`), then `country_db` (`country -> continent`), then `asn_db`, then `geoip_custom` location map, then failover fallback.
  - Returns all healthy+enabled matches for `city_db`, `country_db`, and `geoip_custom` steps.
  - Returns only the first healthy+enabled ASN match in `asn_db` step.
  - If the nearest backend is unhealthy/disabled, the next nearest healthy backend(s) are selected.
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

### GeoIP Affinity

- **Description:** Combines geographical narrowing (subnet pinning and GeoIP hierarchy) with coordinate-based distance picking. Unlike `geoip` mode, which immediately routes by coordinate distance globally if coordinates are present, `geoip_affinity` first restricts the candidate backend pool based on client-to-backend geographical affinity (Subnet -> City -> Subdivision -> Country -> Continent). It then picks the nearest backend by coordinates *within* that narrowed candidate pool.
- **Matching behavior:**
  1. **Subnet Pinning**: Checks if the client IP matches any subnet in the `geoip_custom` `location_map`. If yes, restricts candidate backends to those with the matching `location`.
  2. **Geo Hierarchy Narrowing**: If no subnet pin matches, queries MaxMind DBs to determine the client's location. Restricts candidates by searching in order of specificity: City, then Subdivision, then Country, then Continent.
  3. **Distance Pick**: Within the filtered candidate subset, selects the closest healthy backend(s) using coordinate distance (if coordinates are defined on backends and client location is found). If multiple backends share the same nearest coordinates and the same priority, all of them are returned. If no coordinates are defined, it falls back to failover priority within the subset.
  4. **Global Fallback**: If no candidates match the affinity group, or all of them are unhealthy/disabled, falls back to selecting the closest backend(s) globally, and finally to global failover priority.
- **Use case:** Gives country/region/subnet preference first (e.g. keep European users in European datacenters, or pin specific subnets to specific sites), and then uses latency/distance optimization *within* that preferred region.
- **Example:**
  ```yaml
  mode: geoip_affinity
  backends:
    - address: 10.0.0.1
      country: US
      latitude: 47.6062
      longitude: -122.3321 # Seattle
    - address: 10.0.0.2
      country: US
      latitude: 40.7128
      longitude: -74.0060 # New York
    - address: 10.0.0.3
      country: FR
      latitude: 48.8566
      longitude: 2.3522 # Paris
  ```
  Expected behavior for a US user:
  - Candidates are narrowed to the `US` country subset (Seattle and New York).
  - Paris is completely excluded even if it is closer to the user than Seattle.
  - Coordinate distance selects between Seattle and New York based on which one is closer to the client.

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
