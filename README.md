<p align="center">
  <img src="https://goreportcard.com/badge/github.com/dmachard/coredns-gslb" alt="Go Report"/>
  <img src="https://img.shields.io/badge/go%20lint%20rules-8-green" alt="Go lint"/>
  <img src="https://img.shields.io/badge/go%20tests-281-green" alt="Go tests"/>
  <img src="https://img.shields.io/badge/go%20coverage-85.8%25-green" alt="Go coverage"/>
  <img src="https://img.shields.io/badge/lines%20of%20code-5230-blue" alt="Lines of code"/>
  <img src="https://img.shields.io/badge/integration%20tests-21-blue" alt="Integration tests"/>
</p>

<p align="center">
  <img src="https://img.shields.io/github/v/release/dmachard/coredns-gslb?logo=github&sort=semver" alt="release"/>
</p>

<p align="center">
  <img src="docs/coredns_gslb_logo.svg" alt="CoreDNS-GSLB"/>
</p>

## What is CoreDNS-GSLB?

**CoreDNS-GSLB** is a plugin that provides Global Server Load Balancing functionality in **[CoreDNS](https://coredns.io/)**. It intelligently routes your traffic to healthy backends based on geographic location, priority, or load balancing algorithms.

What it does:
- **Health monitoring** of your backends with HTTP(S), TCP, ICMP, MySQL, gRPC, or custom Lua checks
- **Reusable healthcheck profiles**: Define health check templates globally (in the Corefile) or per zone, and reference them by name in your backends
- **Geographic routing** using MaxMind GeoIP databases or custom location mapping
- **Load balancing** with failover, round-robin, random, weighted, IP-hash, GeoIP or GeoIP-affinity selection
- **Adaptive monitoring** that reduces healthcheck frequency for idle records
- **Live configuration reload** without restarting CoreDNS
- **Bulk backends management via API**: Instantly enable or disable multiple backends by location or IP prefix
- **Supported record types**: Serving dynamic responses for standard `A`, `AAAA`, `CNAME`, `TXT`, `SVCB`, and `HTTPS` queries based on backend status
- **CNAME Redirection for FQDN Backends**: Support hostname/FQDN targets in backend pools, running healthchecks normally and automatically responding with CNAME records if selected
- **Wildcard record support**: Support for standard DNS wildcard records (`*.domain.tld`)
- **Configurable failover policies**: Choose how GSLB answers when all backends are unhealthy (fail-open, fail-closed with custom rcode, or fail-specific with fallback IPs)
- **No external database**: Records are defined using a YAML file.
- **Dynamic backend discovery**: Automatic backend pool construction via Consul, HTTP, or DNS (SVCB/HTTPS)

Unlike many existing solutions, this plugin is designed for non-Kubernetes infrastructures — including virtual machines, bare metal servers, and hybrid environments.

- **Non-Kubernetes focused**: Designed for VMs, bare metal, and hybrid environments
- **Multiple health check types**: From simple TCP to complex Lua scripting
- **Real client IP detection**: EDNS Client Subnet support for accurate GeoIP routing  
- **Resource efficient**: Adaptive healthchecks reduce load on unused backends
- **Production ready**: Prometheus metrics and comprehensive observability

## 🚀 Quick Start

1. **Create docker-compose.yml:**

Prepare folder

```
mkdir coredns
```

Expected folder structure

```
coredns-gslb/
├── docker-compose.yml
└── coredns/
    ├── Corefile
    ├── db.gslb.example.com
    └── db.gslb.example.com.yml
```

Create the `docker-compose.yml`, update binding ports according to your system

```yaml
services:
  coredns-gslb:
    image: dmachard/coredns_gslb:latest
    ports:
      - "53:53/udp"
      - "53:53/tcp"
      - "9153:9153"  # Metrics
    volumes:
      - ./coredns:/coredns
    command: ["-conf", "/coredns/Corefile"]
    restart: unless-stopped
```
    
2. **Create coredns/Corefile:**

Create the `Corefile`

```
.:53 {
    file /coredns/db.gslb.example.com gslb.example.com
    gslb {
        zone  gslb.example.com. /coredns/db.gslb.example.com.yml
    }
    prometheus
}
```

3. **Create coredns/db.gslb.example.com:**

```
$ORIGIN gslb.example.com.
@       3600    IN      SOA     ns1.example.com. admin.example.com. (
                                2024010101 7200 3600 1209600 3600 )
        3600    IN      NS      ns1.gslb.example.com.
        3600    IN      NS      ns2.gslb.example.com.
```

4. **Create coredns/gslb_config.yml:**

```yaml
healthcheck_profiles:
  https_default:
    type: http
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

5. **Run and test:**

```bash
docker-compose up -d
dig @localhost webapp.gslb.example.com
dig @localhost TXT webapp.gslb.example.com  # Debug info
```

## 📚 Documentations

| Topic | Description |
|-------|-------------|
| [Supported Records](docs/records.md) | Supported DNS record types |
| [Selection Modes](docs/modes.md) | Failover, round-robin, random, IP-hash, GeoIP, GeoIP affinity, weighted |
| [Health Checks](docs/healthchecks.md) | HTTP(S), TCP, ICMP, MySQL, gRPC, Lua scripting |
| [Backend Discovery](docs/discovery.md) | Static, API-driven, and dynamic backend pool management |
| [GeoIP Setup](docs/configuration.md#geoip) | MaxMind databases and custom location mapping |
| [Configuration](docs/configuration.md) | Complete parameter reference |
| [High Availability](docs/architecture.md) | Production deployment patterns |
| [API Reference](docs/api.md) | REST API endpoints and OpenAPI schema |
| [CLI Guide](docs/cli.md) | Command-line tool for operations |
| [Observability](docs/observability.md) | Prometheus metrics |
| [Benchmarking](docs/benchmark.md) | Performance |
| [Troubleshooting](docs/troubleshooting.md) | Troubleshooting and debugging |

## 👥 Contributions

Contributions are welcome! Please read the [Developer Guide](CONTRIBUTING.md) for local setup and testing instructions.

## 🧰 Related Projects:

- [DNS-tester](https://github.com/dmachard/DNS-tester) - DNS testing toolkit
- [DNS-collector](https://github.com/dmachard/DNS-collector) - Grab your DNS logs, detect anomalies, and finally understand what's happening on your network.
