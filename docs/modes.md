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

- **Description:** Selects the backend(s) closest to the client based on a location map (subnet-to-location mapping), by country, city, or ASN using MaxMind databases. Requires the `geoip_maxmind` or `geoip_custom` options.
- **Use case:** Directs users to the nearest datacenter, region, or country for lower latency.
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

If no healthy backend matches the client's country or location, the plugin falls back to failover mode.

