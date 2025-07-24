## CoreDNS-GSLB: Configuration 

### Syntax

~~~
gslb {
    zone example.org.   db.example.org.yml
    zone test.org.      db.test.org.yml

    geoip_maxmind country_db /coredns/GeoLite2-Country.mmdb
    geoip_maxmind city_db /coredns/GeoLite2-City.mmdb
    geoip_maxmind asn_db /coredns/GeoLite2-ASN.mmdb
    geoip_custom_db /coredns/location_map.yml
    
    use_edns_csubnet
    max_stagger_start "120s"
    resolution_idle_timeout "3600s"
    healthcheck_idle_multiplier 10
    batch_size_start 100
    disable_txt
}
~~~

* **zone**: Declare each DNS zone (with trailing dot) and its YAML record file. All records for a zone are loaded from the specified file. This directive can be repeated for multiple zones.
* **geoip_maxmind <type> <path>**: Load a MaxMind GeoIP database. `<type>` can be `country_db`, `city_db`, or `asn_db`.
* **geoip_custom_db**: Path to a YAML file mapping subnets to locations for GeoIP-based backend selection.

### Configuration Options

* `max_stagger_start`: The maximum staggered delay for starting health checks (default: "120s").
* `resolution_idle_timeout`: The duration to wait before idle resolution times out (default: "3600s").
* `healthcheck_idle_multiplier`: The multiplier for the healthcheck interval when a record is idle (default: 10).
* `batch_size_start`: The number of backends to process simultaneously during startup (default: 100).
* `geoip_maxmind <type> <path>`: Path to a MaxMind GeoLite2 database for GeoIP backend selection. `<type>` can be `country`, `city`, or `asn`.
* `geoip_maxmind { ... }`: Block syntax for MaxMind DBs. Use `country_db`, `city_db`, and/or `asn_db` as keys inside the block to specify the database paths. Both syntaxes are supported and can be used interchangeably.
* `geoip_custom_db`: Path to a YAML file mapping subnets to locations for GeoIP-based backend selection. Used for `geoip` mode (location-based routing).
* `use_edns_csubnet`: If set, the plugin will use the EDNS Client Subnet (ECS) option to determine the real client IP for GeoIP and logging. Recommended for deployments behind DNS forwarders or public resolvers.
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

### GeoIP

#### MaxMind Databases

Download from MaxMind and configure paths:
```
gslb config.yml {
    # Either single-line or block syntax for geoip_maxmind:
    geoip_maxmind country /coredns/GeoLite2-Country.mmdb
    # or
    geoip_maxmind {
        country_db /coredns/GeoLite2-Country.mmdb
    }
    geoip_city_maxmind_db /coredns/GeoLite2-City.mmdb
    geoip_asn_maxmind_db /coredns/GeoLite2-ASN.mmdb
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

Example backend with all GeoIP location fields

~~~yaml
- address: "172.16.0.12"
  country: "FR"
  city: "Paris"
  asn: "12345"
  location: "eu-west-1"
  enable: true
  priority: 1
  healthchecks:
    - type: grpc
      params:
        port: 9090
        service: ""
        timeout: 5s
~~~

### API Server Options

You can control the HTTP API server with the following options in your Corefile GSLB block:

```
gslb gslb_config.yml example.com {
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




