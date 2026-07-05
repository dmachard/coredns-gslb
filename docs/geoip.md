# CoreDNS-GSLB: GeoIP Setup & Configuration

This guide explains how to set up geographic routing in **CoreDNS-GSLB** using either **MaxMind GeoIP2 databases** or **custom subnet location maps**. 

To use GeoIP-based routing, you must configure either `geoip_maxmind` or `geoip_custom` in your Corefile. GSLB will then route queries to the nearest backends using the matching logic defined in the selection modes (see [Selection & Failover](modes.md)).

---

## MaxMind Databases

CoreDNS-GSLB supports MaxMind GeoIP2 Country, City, and ASN databases to automatically detect the client's location and match it against the backend pool attributes.

To download the `.mmdb` files from MaxMind and configure their paths in the `gslb` block of your Corefile, please refer to the **[GeoIP Settings in the Corefile Reference](configuration.md#geoip-settings)**.

---

## Custom Location Mapping

If you run an internal network or want to override public GeoIP locations for specific IP subnets, you can use a custom location mapping file. 

To enable this by pointing to a YAML mapping file in your Corefile, please refer to the **[GeoIP Settings in the Corefile Reference](configuration.md#geoip-settings)**.

### Custom Mapping File (`location_map.yml`)

Create a YAML file mapping subnets to custom location tags:

```yaml
subnets:
  - subnet: "10.0.0.0/24"
    location: ["eu-west-1"]
  - subnet: "192.168.1.0/24" 
    location: ["us-east-1"]
```

GSLB will map incoming client IPs belonging to these subnets to the defined location tag and direct traffic to backends tagged with the same `location` value.

---

## Complete Backend Configuration Example

Below is an example of a backend definition demonstrating all available GeoIP location attributes, active healthcheck configuration, and bypass options:

```yaml
- address: "172.16.0.12"
  continent: "EU"
  country: "FR"
  subdivision: "IDF"
  city: "Paris"
  asn: "12345"
  location: "eu-west-1"
  enable: true
  assume_healthy: true  # Optional: bypasses healthchecks and treats this backend as permanently UP
  priority: 1
  healthchecks:
    - type: grpc
      params:
        port: 9090
        service: ""
        timeout: 5s
```

### Location Matching Fields Reference

* **`continent`**: The exact MaxMind continent code (e.g. `AF`, `AN`, `AS`, `EU`, `NA`, `OC`, `SA`).
* **`country`**: Two-letter ISO country code (e.g., `FR`, `US`, `CN`).
* **`subdivision`**: Subdivision ISO code (e.g., `IDF` for Île-de-France, `CA` for California).
* **`city`**: City name (matched case-insensitively, English name returned by MaxMind database, e.g., `Paris`, `Oakland`).
* **`asn`**: Autonomous System Number as a string (e.g., `12345`).
* **`location`**: The custom location tag (matching subnets in `location_map.yml`).
