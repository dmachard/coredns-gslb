# Standalone Deployments

For production environments, CoreDNS-GSLB can be deployed across multiple datacenters or nodes. 

Without a shared backend (like Redis), each GSLB instance runs as an **independent, standalone node**. Each node performs its own active health checks and maintains its own in-memory routing state independently.

## Datacenter Redundancy Model (Standalone Nodes)

In this model:

- Each CoreDNS-GSLB instance is deployed with the same configuration across datacenters.
- All GSLB nodes monitor the same backend pool, ensuring consistent health-based decisions regardless of location.
- GeoDNS logic (via EDNS Client Subnet and GeoIP) allows each instance to respond optimally from its point of view.

```mermaid
flowchart TD
    query[DNS Query for gslb.example.com] --> authns[Authoritative NS<br>ns1 / ns2]
    authns -->|Delegation to DC1| dc1
    authns -->|Delegation to DC2| dc2

    subgraph dc1[Datacenter 1]
        dnsdist1[dnsdist with cache] --> coredns1[CoreDNS GSLB]
    end

    subgraph dc2[Datacenter 2]
        dnsdist2[dnsdist with cache] --> coredns2[CoreDNS GSLB]
    end

    coredns1 --> backends[Backends to check<br>- web1.dc1.com / web1.dc2.com<br>- web2.dc1.com / web2.dc2.com<br>- api1.dc1.com / api1.dc2.com]
    coredns2 --> backends
```

## Per-Datacenter Scalability Model (Standalone Nodes)

To scale queries within a single datacenter, you can load-balance multiple CoreDNS-GSLB instances using a frontend DNS load balancer like `dnsdist`.

```mermaid
flowchart TD
    dnsdist[dnsdist<br>Load Balancer] --> coredns1[CoreDNS-GSLB/1<br>Zones: A, B]
    dnsdist --> coredns2[CoreDNS-GSLB/2<br>Zones: C, D]
    dnsdist --> coredns3[CoreDNS-GSLB/3<br>Zones: E, F]

    coredns1 --> pool1[Backend Pool 1<br>- web1.dc1.com<br>- web2.dc1.com]
    coredns2 --> pool2[Backend Pool 2<br>- api1.dc1.com<br>- api2.dc1.com]
    coredns3 --> pool3[Backend Pool 3<br>- db1.dc1.com<br>- db2.dc1.com]
```

### Key Benefits

*   **Horizontal Scalability**: Add more CoreDNS instances as query volume increases.
*   **Zone Isolation**: Segment your DNS architecture by configuring instances to handle specific zones.
*   **Load Balancing**: `dnsdist` distributes queries intelligently and caches frequent responses.
*   **Fault Tolerance**: If one CoreDNS instance fails, other instances or backend paths continue serving workloads.
*   **Resource Optimization**: Each instance is optimized for its specific zone workload and backend monitoring pool.
