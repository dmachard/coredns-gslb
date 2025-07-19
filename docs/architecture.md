
## CoreDNS-GSLB: Architecture

### High Availability and Scalability

For production environments requiring high availability and scalability, 
the CoreDNS-GSLB can be deployed as below to ensure resilience and performance

In this model:
  - Each CoreDNS-GSLB instance is deployed with the same configuration across datacenters.
  - All GSLB nodes monitor the same backend pool, ensuring consistent health-based decisions regardless of location.
  - GeoDNS logic (via EDNS Client Subnet and GeoIP) allows each instance to respond optimally from its point of view.

```
              DNS Query for gslb.example.com
                             │
                             ▼
                    ┌──────────────────┐
                    │ Authoritative NS │
                    │   ns1 / ns2      │
                    └────────┬─────────┘
                             │ Delegation to:
         ┌───────────────────┴──────────────────┐
         ▼                                      ▼
┌───────────────────┐              ┌───────────────────┐
│   Datacenter 1    │              │   Datacenter 2    │
│                   │              │                   │
│  ┌─────────────┐  │              │  ┌─────────────┐  │
│  │  dnsdist    │  │              │  │  dnsdist    │  │
│  │ with cache  │  │              │  │ with cache  │  │
│  └─────┬───────┘  │              │  └─────┬───────┘  │
│        │          │              │        │          │
│    ┌───┴───┐      │              │    ┌───┴───┐      │
│    │CoreDNS│      │              │    │CoreDNS│      │
│    │GSLB   │      │              │    │GSLB   │      │
│    └───┬───┘      │              │    └───┬───┘      │
│        │          │              │        │          │
└───────────────────┘              └───────────────────┘
         │                                  │           
         ▼                                  ▼           
 ┌────────────────────────────────────────────────────┐
 │                 Backends to check                  │
 │           web1.dc1.com   web1.dc2.com              │
 │           web2.dc1.com    web2.dc2.com             │
 │           api1.dc1.com    api1.dc2.com             │
 └────────────────────────────────────────────────────┘
```

Per-Datacenter Scalability Model

```
                    ┌─────────────────┐
                    │   dnsdist       │
                    │ (Load Balancer) │
                    └─────────┬───────┘
                             │
        ┌────────────────────┼────────────────────┐
        │                    │                    │
        ▼                    ▼                    ▼
┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐
│ CoreDNS-GSLB/1  │  │ CoreDNS-GSLB/2  │  │ CoreDNS-GSLB/3  │
│ Zones: A, B     │  │ Zones: C, D     │  │ Zones: E, F     │
└─────────────────┘  └─────────────────┘  └─────────────────┘
        │                    │                    │
        ▼                    ▼                    ▼
┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐
│ Backend Pool 1  │  │ Backend Pool 2  │  │ Backend Pool 3  │
│ web1.dc1.com    │  │ api1.dc1.com    │  │ db1.dc1.com     │
│ web2.dc1.com    │  │ api2.dc1.com    │  │ db2.dc1.com     │
└─────────────────┘  └─────────────────┘  └─────────────────┘
```

**Benefits:**
- **Horizontal scalability**: Add more CoreDNS instances as needed
- **Zone isolation**: Each CoreDNS instance handles specific zones
- **Load balancing**: dnsdist distributes queries intelligently
- **Fault tolerance**: If one CoreDNS fails, others continue serving their zones
- **Resource optimization**: Each instance optimized for its zone workload
