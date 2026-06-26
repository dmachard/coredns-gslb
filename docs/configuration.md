## CoreDNS-GSLB: Configuration 

CoreDNS-GSLB automatically watches configuration files and reloads them at runtime when changes are detected. This allows most configuration updates to be applied without restarting CoreDNS,

### Syntax

~~~
gslb {
    # Zones definition
    zone example.org.   db.example.org.yml
    zone test.org.      db.test.org.yml

    # GeoIP MaxMind databases
    geoip_maxmind country_db /coredns/GeoLite2-Country.mmdb
    geoip_maxmind city_db /coredns/GeoLite2-City.mmdb
    geoip_maxmind asn_db /coredns/GeoLite2-ASN.mmdb
    geoip_custom /coredns/location_map.yml
    
    # Miscs
    use_edns_csubnet
    disable_txt

    # Maximum delay for staggered start
    max_stagger_start "120s"
    batch_size_start 100

    # Idle timeout for resolution
    resolution_idle_timeout "3600s"
    healthcheck_idle_multiplier 10
    
    # API
    api_enable true
    api_tls_cert ""
    api_tls_key ""
    api_listen_addr 0.0.0.0
    api_listen_port 8080
    api_basic_user admin
}
~~~

* **zone**: Declare each DNS zone (with trailing dot) and its YAML record file. All records for a zone are loaded from the specified file. This directive can be repeated for multiple zones.
* **geoip_maxmind <type> <path>**: Load a MaxMind GeoIP database. `<type>` can be `country_db`, `city_db`, or `asn_db`.
* **geoip_custom**: Path to a YAML file mapping subnets to locations for GeoIP-based backend selection.

### Configuration Options

* `max_stagger_start`: The maximum staggered delay for starting health checks (default: "120s").
* `resolution_idle_timeout`: The duration to wait before idle resolution times out (default: "3600s").
* `healthcheck_idle_multiplier`: The multiplier for the healthcheck interval when a record is idle (default: 10).
* `batch_size_start`: The number of backends to process simultaneously during startup (default: 100).
* `geoip_maxmind <type> <path>`: Path to a MaxMind GeoLite2 database for GeoIP backend selection. `<type>` can be `country`, `city`, or `asn`.
* `geoip_maxmind { ... }`: Block syntax for MaxMind DBs. Use `country_db`, `city_db`, and/or `asn_db` as keys inside the block to specify the database paths. Both syntaxes are supported and can be used interchangeably.
* `geoip_custom`: Path to a YAML file mapping subnets to locations for GeoIP-based backend selection. Used for `geoip` mode (location-based routing). Subnets are pre-parsed on load and file reload to eliminate IP parsing overhead during DNS query resolution.
* `use_edns_csubnet`: If set, the plugin will use the EDNS Client Subnet (ECS) option to determine the real client IP for GeoIP and logging, and will echo back the ECS option in DNS responses with the appropriate scope prefix-length per RFC 7871. Recommended for deployments behind DNS forwarders or public resolvers.
* `api_enable`: Enable or disable the HTTP API server (default: true). Set to `false` to disable the API endpoint.
* `api_tls_cert`: Path to the TLS certificate file for the API server (optional, enables HTTPS if set with `api_tls_key`).
* `api_tls_key`: Path to the TLS private key file for the API server (optional, enables HTTPS if set with `api_tls_cert`).
* `api_listen_addr`: IP address to bind the API server to (default: `0.0.0.0`).
* `api_listen_port`: Port to bind the API server to (default: `8080`).
* `api_basic_user`: HTTP Basic Auth username for the API (optional, if set, authentication is required).
* `api_basic_pass`: HTTP Basic Auth password for the API (optional, if set, authentication is required).
* `disable_txt`: If set, disables TXT record resolution for GSLB-managed zones. TXT queries will be passed to the next plugin or return empty if none.

### Full example

Load the `example.org.` and `test.org.` zones from their respective YAML files and enable GSLB records on them:

~~~ corefile
. {
    file db.example.org
    file db.test.org
    gslb {
        zone example.org.   gslb_config.example.org.yml
        zone test.org.      gslb_config.test.org.yml
        geoip_maxmind country_db /coredns/GeoLite2-Country.mmdb
        geoip_maxmind city_db /coredns/GeoLite2-City.mmdb
        geoip_maxmind asn_db /coredns/GeoLite2-ASN.mmdb
        disable_txt
    }
}
~~~

Where `db.example.org` would contain:

~~~ text
$ORIGIN example.org.
@       3600    IN      SOA     ns1.example.org. admin.example.org. (
                                2024010101 ; Serial
                                7200       ; Refresh
                                3600       ; Retry
                                1209600    ; Expire
                                3600       ; Minimum TTL
                                )
        3600    IN      NS      ns1.example.org.
        3600    IN      NS      ns2.example.org.
~~~

And `gslb_config.example.org.yml` would contain:

~~~ yaml
healthcheck_profiles:
  https_default:
    type: http
    params:
      enable_tls: true
      port: 443
      uri: /
      expected_code: 200
      timeout: 5s

records:
  webapp.example.org.:
    mode: "failover"
    record_ttl: 30
    scrape_interval: 10s
    alpn:
      - "h3"
      - "h2"
    backends:
    - address: "172.16.0.10"
      priority: 1
      healthchecks: [ https_default ]  # Reference the profile by name
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

### Wildcard Records Support

CoreDNS-GSLB supports standard DNS wildcard records (`*.domain.tld`) as described in RFC 1034 §4.3.3.
If a query does not match any exact record configured in a zone, GSLB will look for a wildcard record by replacing the leftmost label of the query name with `*` and walking up towards the zone apex.

**Example Configuration:**

~~~yaml
records:
  "*.example.org.":
    mode: "geoip"
    backends:
      - address: "172.16.0.10"
        priority: 1
        location: "eu-west-1"
      - address: "172.16.0.20"
        priority: 1
        location: "us-east-1"
~~~

With this configuration:
- A query for `anything.example.org.` will match `*.example.org.` (if there is no exact record `anything.example.org.`).
- A query for `sub.anything.example.org.` will also match `*.example.org.`.
- An exact match always takes precedence over a wildcard match.
- In case of multiple overlapping authoritative zones configured in GSLB (e.g., `sub.example.org.` and `example.org.`), wildcard lookup is strictly bounded by the most specific zone (the zone with the longest matching suffix).

### Failover Policies (Fail-safe Behavior)

By default, when all backends for a record are detected as unhealthy or disabled, CoreDNS-GSLB applies a **fail-open** behavior, returning all configured enabled backends to maintain connectivity.

You can customize this behavior using the `failover_policy` block in your record configuration.

#### Supported Policies:
- **`fail-open`** (default): Returns all enabled backends if all are unhealthy.
- **`fail-closed`**: Returns a custom DNS response code with an empty answer section.
- **`fail-specific`**: Returns a specific, stable list of fallback IP addresses (regardless of their health state).

#### Configuration Parameters:
- **`mode`**: The policy mode (`fail-open`, `fail-closed`, or `fail-specific`).
- **`rcode`**: The DNS response code to return when `mode` is `fail-closed` (Options: `NXDOMAIN`, `SERVFAIL`, `REFUSED`, `NOERROR`). Defaults to `SERVFAIL`.
- **`fallback_ips`**: A list of fallback IP addresses (IPv4/IPv6) to return when `mode` is `fail-specific`.

#### Example Configurations:

**1. Fail-closed with NXDOMAIN**
~~~yaml
records:
  webapp.example.org.:
    mode: "roundrobin"
    failover_policy:
      mode: "fail-closed"
      rcode: "NXDOMAIN"
    backends:
      - address: "172.16.0.10"
      - address: "172.16.0.11"
~~~

**2. Fail-specific with Fallback IPs**
~~~yaml
records:
  api.example.org.:
    mode: "failover"
    failover_policy:
      mode: "fail-specific"
      fallback_ips: ["1.2.3.4", "2001:db8::1"]
    backends:
      - address: "172.16.0.20"
      - address: "172.16.0.21"
~~~

### Using the `defaults` block in YAML zone files

You can define a `defaults` block at the top of your zone YAML file to avoid repeating common fields in every record. Any field defined in `defaults` will be automatically applied to all records, unless a record explicitly overrides that field.

**Example:**

~~~yaml
defaults:
  owner: admin
  record_ttl: 30
  scrape_interval: 10s
  scrape_retries: 1
  scrape_timeout: 5s
  alpn:
    - "h3"
    - "h2"

records:
  web1.example.org.:
    mode: failover
    # Inherits all defaults above
  web2.example.org.:
    mode: failover
    owner: alice  # Overrides the default owner
    record_ttl: 60  # Overrides the default TTL
~~~

In this example:
- `web1.example.org.` will have `owner=admin`, `record_ttl=30`, etc. (from defaults)
- `web2.example.org.` will have `owner=alice` and `record_ttl=60`, but will inherit the other defaults.

This makes your YAML files much more concise and easier to maintain.

### Backend tags

You can add a `tags` field to any backend in your YAML configuration. This field is a list of keywords (strings) that you can use to group, filter, or target backends for API operations (such as enable/disable by tag).

**Example:**

~~~yaml
records:
  webapp.example.org.:
    backends:
      - address: "172.16.0.10"
        tags: ["prod", "ssd", "eu"]
      - address: "172.16.0.11"
        tags: ["test", "hdd", "us"]
~~~

- You can assign any number of tags to a backend.
- Tags are used by the API to enable/disable backends in bulk (see API documentation).
- Tags can be used for your own grouping or inventory purposes as well.

### Dynamic Service Discovery & Backend Discovery

For details on configuring backend pools, static configs, API bulk updates, or dynamic service discovery (via Consul, HTTP APIs, or DNS SVCB/HTTPS records), please refer to the dedicated [Backends Discovery](discovery.md) guide.

### GeoIP

#### MaxMind Databases

Download from MaxMind and configure paths:
```
gslb {
    geoip_maxmind country_db /coredns/GeoLite2-Country.mmdb
    geoip_maxmind city_db /coredns/GeoLite2-City.mmdb
    geoip_maxmind asn_db /coredns/GeoLite2-ASN.mmdb
}
```

#### Custom Location Mapping

Create `location_map.yml`:
```yaml
subnets:
  - subnet: "10.0.0.0/24"
    location: ["eu-west-1"]
  - subnet: "192.168.1.0/24" 
    location: ["us-east-1"]
```

Example backend with all GeoIP location fields and health options

~~~yaml
- address: "172.16.0.12"
  continent: "EU"
  country: "FR"
  subdivision: "IDF"
  city: "Paris"
  asn: "12345"
  location: "eu-west-1"
  enable: true
  assume_healthy: true  # Optional, bypasses healthchecks and treats this backend as permanently UP
  priority: 1
  healthchecks:
    - type: grpc
      params:
        port: 9090
        service: ""
        timeout: 5s
~~~

For `continent`, use the exact MaxMind continent code from `Continent.Code`: `AF`, `AN`, `AS`, `EU`, `NA`, `OC`, `SA`.

### API Server Options

You can control the HTTP API server with the following options in your Corefile GSLB block:

```
gslb {
    api_enable true
    api_listen_addr 127.0.0.1
    api_listen_port 9090
    api_tls_cert /path/to/cert.pem
    api_tls_key /path/to/key.pem
    api_basic_user admin
    api_basic_pass secret
}
```

- If `api_enable` is set to `false`, the API server will not be started.
- If both `api_tls_cert` and `api_tls_key` are set, the API will be served over HTTPS on the configured address/port.
- If neither is set, the API will be served over HTTP on the configured address/port.
- Use `api_listen_addr` and `api_listen_port` to change the default bind address and port (default: `0.0.0.0:8080`).
- If `api_basic_user` and `api_basic_pass` are set, HTTP Basic Authentication is required for all API requests.

### Global Healthcheck Profiles

You can define reusable healthcheck profiles globally for all zones using the Corefile directive:

```
gslb {
    ...
    healthcheck_profiles healthcheck_profiles.yml
}
```

The referenced file should contain:

```yaml
healthcheck_profiles:
  https_default:
    type: http
    params:
      port: 443
      uri: /
      expected_code: 200

# In db.app-x.gslb.example.com.yml (zone file)
healthcheck_profiles:
  https_default:
    type: http
    params:
      port: 443
      uri: /custom
      expected_code: 200

records:
  webapp.app-x.gslb.example.com.:
    backends:
      - address: 10.0.0.1
        healthchecks: [ https_default ]  # Uses the local version
```


