# Getting Started with CoreDNS-GSLB

This guide walk you through setting up CoreDNS-GSLB for the first time, configuring a basic zone, and verifying traffic routing.

---

## 1. Quick Start with Docker Compose

The fastest way to test CoreDNS-GSLB is by running it inside a Docker container.

### Step A: Create `docker-compose.yml`
Create a directory and write the following `docker-compose.yml`:

```yaml
version: '3.8'

services:
  coredns:
    image: dmachard/coredns-gslb:latest
    ports:
      - "53:53/udp"
      - "53:53/tcp"
      - "8080:8080" # REST API
    volumes:
      - ./Corefile:/etc/coredns/Corefile
      - ./gslb_zone.yml:/etc/coredns/gslb_zone.yml
```

### Step B: Create the `Corefile`
In the same directory, create a `Corefile` specifying your DNS zones and directing GSLB to load configurations:

```corefile
. {
    log
    errors
    gslb {
        zone example.org. /etc/coredns/gslb_zone.yml
    }
}
```

### Step C: Create the YAML Zone File (`gslb_zone.yml`)
Define your record and its backend endpoints in `gslb_zone.yml`:

```yaml
records:
  webapp.example.org.:
    mode: "failover"
    record_ttl: 30
    backends:
      - address: "192.168.1.10"
        priority: 1
      - address: "192.168.1.11"
        priority: 2
```

### Step D: Start and Test
1. Start the container:
   ```bash
   docker-compose up -d
   ```

2. Query your GSLB record:
   ```bash
   dig @localhost webapp.example.org
   ```

3. Query the TXT debug record (by default, GSLB exposes status info here):
   ```bash
   dig @localhost TXT webapp.example.org
   ```

---

## 2. Compiling and Running Locally

If you prefer to run CoreDNS-GSLB directly on your host machine:

1. Clone and compile the binary:
   ```bash
   git clone https://github.com/dmachard/CoreDNS-GSLB.git
   cd CoreDNS-GSLB
   make build
   ```

2. Run CoreDNS with your Corefile:
   ```bash
   ./coredns -conf /path/to/Corefile
   ```

---

## 3. Next Steps

Now that you have a running environment:
- Explore the different [Selection Modes](modes.md) (GeoIP, IP-Hash, Weighted, etc.).
- Learn how to set up active [Health Checks](healthchecks.md) (HTTP, TCP, ICMP, gRPC) to automatically detect backend failures.
- Automate backend management using the [REST API](api.md) or external [Service Discovery](discovery.md).
