<p align="center">
  <img src="https://goreportcard.com/badge/github.com/dmachard/coredns-gslb" alt="Go Report"/>
  <img src="https://img.shields.io/badge/go%20lint%20rules-8-green" alt="Go lint"/>
  <img src="https://img.shields.io/badge/go%20tests-346-green" alt="Go tests"/>
  <img src="https://img.shields.io/badge/go%20coverage-85%25-green" alt="Go coverage"/>
  <img src="https://img.shields.io/badge/lines%20of%20code-6616-blue" alt="Lines of code"/>
  <img src="https://img.shields.io/badge/integration%20tests-24-blue" alt="Integration tests"/>
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
- **Supported record types**: Serving dynamic responses for standard `A`, `AAAA`, `CNAME`, `SRV`, `TXT`, `SVCB`, and `HTTPS` queries based on backend status
- **CNAME Redirection for FQDN Backends**: Support hostname/FQDN targets in backend pools, running healthchecks normally and automatically responding with CNAME records if selected
- **Wildcard record support**: Support for standard DNS wildcard records (`*.domain.tld`)
- **Configurable failover policies**: Choose how GSLB answers when all backends are unhealthy (fail-open, fail-closed with custom rcode, or fail-specific with fallback IPs)
- **Distributed HA Synchronization**: Optional Redis-backed shared health checking with distributed locks (`SETNX`), offloaded probing support, and real-time Pub/Sub propagation to synchronize status across multiple CoreDNS instances.
- **No external database**: Records are defined using a YAML file.
- **Dynamic backend discovery**: Automatic backend pool construction via Consul, HTTP, or DNS (SVCB/HTTPS)

Unlike many existing solutions, this plugin is designed for non-Kubernetes infrastructures — including virtual machines, bare metal servers, and hybrid environments.

- **Non-Kubernetes focused**: Designed for VMs, bare metal, and hybrid environments
- **Multiple health check types**: From simple TCP to complex Lua scripting
- **Real client IP detection**: EDNS Client Subnet support for accurate GeoIP routing  
- **Resource efficient**: Adaptive healthchecks reduce load on unused backends
- **Production ready**: Prometheus metrics and comprehensive observability

## 🚀 Getting Started

To get up and running quickly using Docker Compose or to compile the plugin locally, check out the **[Getting Started Guide](docs/getting_started.md)**.


## 📚 Documentation

| Topic | Description |
|-------|-------------|
| [Getting Started](docs/getting_started.md) | Quickstart guide and Docker setup |
| [Corefile Reference](docs/configuration.md) | Complete Corefile parameter reference |
| [Supported Records](docs/records.md) | Supported record types (A, AAAA, SRV, SVCB/HTTPS, TXT, wildcards) |
| [Selection & Failover](docs/modes.md) | Failover, round-robin, random, IP-hash, GeoIP, weighted, and failover policies |
| [GeoIP Setup](docs/geoip.md) | MaxMind databases and custom location mapping |
| [Health Checks](docs/healthchecks.md) | Active health checking (HTTP/S, TCP, ICMP, MySQL, gRPC, Lua) and profiles |
| [Backend Discovery](docs/discovery.md) | Static configs, defaults, tags, and dynamic discovery (Consul, HTTP, SVCB) |
| [ECS Compliance](docs/ecs.md) | EDNS Client Subnet compliance and caching scope |
| [Architecture & Deployments](docs/deployments/standalone.md) | Standalone and Cluster deployments, Anycast/BGP (RHI), and Consul integration |
| [API Reference](docs/api.md) | REST API endpoints and OpenAPI schema |
| [CLI Guide](docs/cli.md) | Command-line client tool |
| [Observability](docs/observability.md) | Prometheus metrics |
| [Performance Benchmarks](docs/performance/benchmarks.md) | Resource usage and local benchmarking guides |
| [Troubleshooting](docs/troubleshooting.md) | Troubleshooting and debugging |
| [Developer Guide](docs/developer/contributing.md) | Development rules, docker environment, compilation, and testing |

## 👥 Contributions

Contributions are welcome! Please read the [Contributing Guide](CONTRIBUTING.md) for local setup and testing instructions.

## 🧰 Related Projects

> [!TIP]
> Check out these other open-source DNS utilities to enhance your DNS infrastructure monitoring and testing:
>
> - **[DNS-collector](https://github.com/dmachard/DNS-collector)**: A high-performance DNS traffic analyzer and collector. Grab your DNS logs, detect anomalies, and finally understand what's happening on your network in real-time.
> - **[DNS-tester](https://github.com/dmachard/DNS-tester)**: A comprehensive DNS testing and verification toolkit designed to validate DNS response behavior under various network conditions.
