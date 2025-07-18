## CoreDNS-GSLB: Configuration 

### Syntax

~~~
gslb DB_YAML_FILE [ZONES...] {
    max_stagger_start "120s"
    resolution_idle_timeout "3600s"   # Duration before slow healthcheck (default: 3600s)
    healthcheck_idle_multiplier 10      # Multiplier for slow healthcheck interval (default: 10)
    batch_size_start 100
    geoip_country_maxmind_db /coredns/GeoLite2-Country.mmdb   # Enable GeoIP by country (MaxMind)
    geoip_city_maxmind_db /coredns/GeoLite2-City.mmdb         # Enable GeoIP by city (MaxMind)
    geoip_asn_maxmind_db /coredns/GeoLite2-ASN.mmdb           # Enable GeoIP by ASN (MaxMind)
    geoip_custom_db /coredns/location_map.yml                 # Enable GeoIP by region/subnet (YAML map)
    use_edns_csubnet
}
~~~

* **DB_YAML_FILE** The GSLB configuration file in YAML format. If the path is relative, the path from the *root*
  plugin will be prepended to it.
* **ZONES** Specifies the zones the plugin should be authoritative for. If not provided, the zones from the CoreDNS configuration block are used.

### Configuration Options

* `max_stagger_start`: The maximum staggered delay for starting health checks (default: "120s").
* `resolution_idle_timeout`: The duration to wait before idle resolution times out (default: "3600s").
* `healthcheck_idle_multiplier`: The multiplier for the healthcheck interval when a record is idle (default: 10).
* `batch_size_start`: The number of backends to process simultaneously during startup (default: 100).
* `geoip_country_maxmind_db`: Path to a MaxMind GeoLite2-Country.mmdb file for country-based GeoIP backend selection. Used for `geoip` mode (country-based routing).
* `geoip_city_maxmind_db`: Path to a MaxMind GeoLite2-City.mmdb file for city-based GeoIP backend selection. Used for `geoip` mode (city-based routing).
* `geoip_asn_maxmind_db`: Path to a MaxMind GeoLite2-ASN.mmdb file for ASN-based GeoIP backend selection. Used for `geoip` mode (ASN-based routing).
* `geoip_custom_db`: Path to a YAML file mapping subnets to locations for GeoIP-based backend selection. Used for `geoip` mode (location-based routing).
* `use_edns_csubnet`: If set, the plugin will use the EDNS Client Subnet (ECS) option to determine the real client IP for GeoIP and logging. Recommended for deployments behind DNS forwarders or public resolvers.


### Full example

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

### GeoIP

#### MaxMind Databases

Download from MaxMind and configure paths:
```
gslb config.yml {
    geoip_country_maxmind_db /coredns/GeoLite2-Country.mmdb
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
  location_country: [ "FR", "US" ]
  location_city: [ "Paris", "London" ]
  location_asn: [ "12345", "67890" ]
  location_custom: [ "eu-west-1" ]
  enable: true
  priority: 1
  healthchecks:
    - type: grpc
      params:
        port: 9090
        service: ""
        timeout: 5s
~~~

- All `location_*` fields must be YAML lists (even if empty or with one value).
- You can leave a list empty (`[ ]`) if you do not want to filter on that dimension.
- This allows flexible matching by country, city, ASN, or custom tags.




