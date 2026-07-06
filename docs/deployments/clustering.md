# Cluster Mode Deployment

In highly available (HA) deployments with multiple CoreDNS-GSLB instances, executing health checks independently from every instance can put redundant load on backends and lead to temporary DNS answer inconsistencies.

CoreDNS-GSLB features a **Cluster Mode** backed by a shared Redis database to solve this.

---

## Startup Sequence

When a CoreDNS-GSLB instance starts up, it aligns its in-memory status with the shared Redis state immediately to avoid cold-start issues or temporary routing discrepancies.

```mermaid
sequenceDiagram
    autonumber
    participant CoreDNS as CoreDNS-GSLB Instance
    participant YAML as Local YAML Zone Files
    participant Redis as Redis Cache
    participant Backend as Backend Pool

    Note over CoreDNS: Startup / Initialization
    CoreDNS->>YAML: 1. Load zone records and backend definitions
    YAML-->>CoreDNS: Return records in-memory
    CoreDNS->>Redis: 2. Connect & Subscribe to "gslb:health:updates"
    
    loop For each Backend in loaded records
        CoreDNS->>Redis: 3. GetRedisHealth(zone, fqdn, IP)
        alt Health status exists in Redis
            Redis-->>CoreDNS: Return status (alive/dead)
            CoreDNS->>CoreDNS: Set initial backend state (SetAlive)
        else Key does not exist / Redis unreachable
            CoreDNS->>CoreDNS: Default to offline/local initial state
        end
    end
    
    CoreDNS->>CoreDNS: 4. Initialize zone-level health status (updateRecordHealthStatus)
    CoreDNS->>CoreDNS: 5. Start background scraper loop
```

---

## Runtime Check Execution (Distributed Lock Mode)

At each scraping interval, CoreDNS-GSLB instances coordinate check execution via distributed locks (`SETNX`) to ensure only one instance executes a health check per cycle.

```mermaid
flowchart TD
    Start([Scraper Loop Triggered]) --> CheckLock{1. Acquire Lock via SETNX?}
    
    CheckLock -- "Yes (Winner)" --> RunCheck[2. Run raw health check locally]
    RunCheck --> ApplyLocal[3. Apply raw status locally]
    ApplyLocal --> WriteRedis[4. Write raw status to Redis\nwith 2x ScrapeInterval TTL]
    WriteRedis --> PubUpdate[5. Publish raw status update\nto Pub/Sub channel]
    PubUpdate --> End([End Scrape Cycle])
    
    CheckLock -- "No (Runner-up)" --> ReadRedis[2. Read raw status from Redis]
    ReadRedis -- "Success" --> ApplyRemote[3. Apply raw status locally]
    ReadRedis -- "Redis Down / Key Missing" --> RunCheckLocal[Fallback: Run raw check locally]
    RunCheckLocal --> ApplyFallback[Apply raw status locally]
    ApplyFallback --> End
    ApplyRemote --> End
```

---

## State & Threshold Synchronization (Pub/Sub)

To guarantee that all instances apply rise/fall state machine transitions consistently, instances publish and consume **raw health check results** (before applying threshold logic). Each instance then runs the threshold state machine locally on every update.

```mermaid
flowchart TD
    subgraph Publisher ["Active Checker (Node A)"]
        A_Check[1. Perform Check] --> A_Apply[2. Apply local threshold]
        A_Apply --> A_Pub[3. Publish raw status to updates channel]
    end

    subgraph SubscriberB ["Standby Node (Node B)"]
        B_Sub[1. Receive update via Pub/Sub] --> B_Apply[2. Apply raw status locally]
        B_Apply --> B_Thresh[3. Re-evaluate rise/fall thresholds]
    end

    subgraph SubscriberC ["Standby Node (Node C)"]
        C_Sub[1. Receive update via Pub/Sub] --> C_Apply[2. Apply raw status locally]
        C_Apply --> C_Thresh[3. Re-evaluate rise/fall thresholds]
    end

    A_Pub -.->|"Pub/Sub Channel: gslb:health:updates"| B_Sub
    A_Pub -.->|"Pub/Sub Channel: gslb:health:updates"| C_Sub
```

---

## Configuration

To enable Cluster Mode, configure the Redis options inside the `gslb` block of your `Corefile`. 

For a comprehensive description of all configuration options, refer to the [Corefile Reference](../configuration.md#redis-cluster-mode-options) guide.

### Corefile Example

```nginx
gslb {
    # ... other options ...

    # Enable Redis sync (default: false)
    redis_enable true

    # Redis address (default: "127.0.0.1:6379")
    redis_addr "redis-server.local:6379"

    # Redis password (default: "")
    redis_password "securepass"

    # Redis database number (default: 0)
    redis_db 0

    # Key prefix to namespace Redis keys (default: "gslb:")
    redis_key_prefix "gslb:"

    # Sync mode (default: "lock"). Use "none" to disable distributed locking
    redis_sync_mode "lock"
}
```

---

## Resilience & Failover Modes

- **Redis Unreachable**: If the connection to Redis fails during startup or execution, the GSLB plugin logs the warning and automatically switches to **independent local check mode**.
- **Lock Owner Death**: Locks are acquired with a short Time-To-Live (TTL) equal to `scrapeInterval / 2` (minimum 2s). If an instance dies mid-check, the lock naturally expires and another instance will automatically assume check execution during the next cycle.
