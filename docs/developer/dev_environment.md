# Setup Development Environment

CoreDNS-GSLB provides a comprehensive local development environment using Docker Compose. This environment spins up CoreDNS configured with the GSLB plugin along with dummy web applications to simulate routing, health checks, failovers, and GeoIP/ECS features.

## Prerequisites

*   Docker and Docker Compose installed.
*   `dig` command-line tool (usually part of `dnsutils` or `bind-utils`) for DNS queries.

*Note: TLS certificates are generated on-demand for TLS and mTLS validation. See the `cert_gen` folder and the `webapp` `init.sh` for details.*

## Starting the Dev Stack

1. Build CoreDNS containing the GSLB plugin:
   ```bash
   sudo docker compose -f docker-compose.dev.yml --progress=plain build
   ```

2. Start the stack (CoreDNS + webapp nodes):
   ```bash
   sudo docker compose -f docker-compose.dev.yml up -d
   ```

## Verifying and Simulating Workloads

### 1. Basic DNS Resolution

Verify that CoreDNS is resolving records properly:
```bash
dig -p 8053 @127.0.0.1 webapp.app-x.gslb.example.com +short
```
*Expected output:* `172.16.0.10`

### 2. Simulating a Failover

Stop the primary webapp container (`webapp10`) to trigger an unhealthy backend status:
```bash
sudo docker compose -f docker-compose.dev.yml stop webapp10
```

Wait for the health check scrape interval (approx. 30 seconds), then query again:
```bash
dig -p 8053 @127.0.0.1 webapp.app-x.gslb.example.com +short
```
*Expected output:* `172.16.0.11` (automatically failed over to the secondary backend).

Now start the primary container again:
```bash
sudo docker compose -f docker-compose.dev.yml start webapp10
```
Wait a few seconds, then resolve again to observe the traffic routing switch back:
```bash
dig -p 8053 @127.0.0.1 webapp.app-x.gslb.example.com +short
```
*Expected output:* `172.16.0.10`

---

## Testing EDNS Client Subnet (ECS) & GeoIP

You can simulate DNS queries originating from different regions to test the GeoIP routing engine.

### 1. Subnet Location-based Routing

Simulate a query coming from subnet `10.1.0.0/24`:
```bash
dig -p 8053 @127.0.0.1 webapp-geoip-loc.app-y.gslb.example.com +short +subnet=10.1.0.42/24
```
*Expected output:* `172.16.0.10`

Simulate a query coming from subnet `10.2.0.0/24`:
```bash
dig -p 8053 @127.0.0.1 webapp-geoip-loc.app-y.gslb.example.com +short +subnet=10.2.0.7/24
```
*Expected output:* `172.16.0.11`

### 2. Country-based Routing

Simulate a query coming from a US IP address (`8.8.8.8`):
```bash
dig -p 8053 @127.0.0.1 webapp-geoip-country.app-y.gslb.example.com +short +subnet=8.8.8.8/24
```
*Expected output:* `172.16.0.11`

Simulate a query coming from a French IP address (`90.0.0.0`):
```bash
dig -p 8053 @127.0.0.1 webapp-geoip-country.app-y.gslb.example.com +short +subnet=90.0.0.0/24
```
*Expected output:* `172.16.0.10`

---

## Starting the HA Sync (Redis) Dev Stack

If you are developing or testing the high-availability synchronization features (shared health states, locks, pub/sub propagation), you should use the dedicated HA Docker Compose configuration:

1. Build the HA dev stack:
   ```bash
   sudo docker compose -f docker-compose.redis.yml build
   ```

2. Start the HA dev stack (Redis + 2 CoreDNS instances + webapps):
   ```bash
   sudo docker compose -f docker-compose.redis.yml up -d
   ```

---

## Stopping the Dev Stack

To stop and remove all container resources (including volume cleanup):

*   **For the Basic Dev Stack**:
    ```bash
    sudo docker compose -f docker-compose.dev.yml down -v
    ```

*   **For the HA Dev Stack**:
    ```bash
    sudo docker compose -f docker-compose.redis.yml down -v
    ```
