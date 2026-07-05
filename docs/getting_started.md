# Getting Started with CoreDNS-GSLB

This guide walks you through setting up CoreDNS-GSLB for the first time, configuring a basic zone, and verifying traffic routing.

---

## Quick Start with Docker Compose

The fastest way to test CoreDNS-GSLB is by running it inside a Docker container.

### Step A: Create `docker-compose.yml`

Create a workspace directory and the `coredns` subdirectory:

```bash
mkdir -p coredns-gslb/coredns
cd coredns-gslb
```

Expected folder structure:

```
coredns-gslb/
├── docker-compose.yml
└── coredns/
    ├── Corefile
    ├── db.gslb.example.com
    └── db.gslb.example.com.yml
```

Create `docker-compose.yml` in the root of the `coredns-gslb/` directory:

```yaml
services:
  coredns-gslb:
    image: dmachard/coredns_gslb:latest
    ports:
      - "53:53/udp"
      - "53:53/tcp"
      - "9153:9153"  # Metrics+
      - "8080:8080"  # API
    volumes:
      - ./coredns:/coredns
    command: ["-conf", "/coredns/Corefile"]
    restart: unless-stopped
```

### Step B: Create the `Corefile`
Inside the `coredns/` subdirectory, create a `Corefile` specifying your DNS zones and directing GSLB to load configurations:

```corefile
.:53 {
    file /coredns/db.gslb.example.com gslb.example.com
    gslb {
        zone  gslb.example.com. /coredns/db.gslb.example.com.yml
    }
    prometheus
}
```

### Step C: Create `coredns/db.gslb.example.com`

```
$ORIGIN gslb.example.com.
@       3600    IN      SOA     ns1.example.com. admin.example.com. (
                                2024010101 7200 3600 1209600 3600 )
        3600    IN      NS      ns1.gslb.example.com.
        3600    IN      NS      ns2.gslb.example.com.
```


### Step D: Create the YAML Zone File (`coredns/db.gslb.example.com.yml`)
Define your record and its backend endpoints in `coredns/db.gslb.example.com.yml`:

```yaml
healthcheck_profiles:
  https_default:
    type: http
    rise: 2
    fall: 3
    params:
      enable_tls: true
      port: 443
      uri: "/"
      expected_code: 200
      timeout: 5s

records:
  webapp.gslb.example.com.:
    mode: "failover"
    record_ttl: 30
    scrape_interval: 10s
    backends:
    - address: "172.16.0.10"
      priority: 1
      healthchecks: [ https_default ]
    - address: "172.16.0.11"
      priority: 2
      healthchecks: [ https_default ]
```

### Step E: Start and Test
1. Start the container:
   ```bash
   docker-compose up -d
   ```

2. Query your GSLB record:
   ```bash
   dig @localhost webapp.gslb.example.com
   ```

3. Query the TXT debug record (by default, GSLB exposes status info here):
   ```bash
   dig @localhost TXT webapp.gslb.example.com
   ```

## Next Steps

Now that you have a running environment:
- Explore the different [Selection Modes](modes.md) (GeoIP, IP-Hash, Weighted, etc.).
- Learn how to set up active [Health Checks](healthchecks.md) (HTTP, TCP, ICMP, gRPC) to automatically detect backend failures.
- Automate backend management using the [REST API](api.md) or external [Service Discovery](discovery.md).
