# HA Synchronization

CoreDNS-GSLB is designed to run in clustered, high-availability deployments. To ensure consistent traffic routing decisions across multiple independent GSLB instances and to reduce probing overhead on backends, it integrates a distributed state synchronization mechanism powered by Redis.

---

## Core Capabilities

### 1. Distributed Scraper Locking (Deduplication)
When multiple GSLB nodes monitor the same backends, duplicate health checks can overwhelm backend services. 
Using Redis `SETNX` distributed locking (`redis_sync_mode: "lock"`), only **one GSLB instance** in the cluster runs a physical health check per cycle for a given backend. The winner writes the result to Redis, and all other instances consume it without probing the backend.

### 2. Real-Time Pub/Sub Propagation
Whenever a GSLB instance changes the health status of a backend, it broadcasts the result over a Redis Pub/Sub channel. Other GSLB instances instantly receive the event and adjust their local state and rise/fall threshold metrics within milliseconds.

### 3. Passive Backend Monitoring
For security or network topology reasons, public-facing GSLB instances might not have access to backends located inside private, isolated networks. 
By marking a backend as `passive: true` in the zone YAML, the GSLB instance skips local health check probing entirely and relies solely on the status published to Redis by an internal probing agent.

> [!NOTE]
> **Key Use Case (Restricted Networks)**:
> Enables deploying a private "active check agent" in a restricted subnet to probe isolated servers and report status to Redis, while public-facing GSLB nodes (which cannot route to those private networks) consume the status passively.

See the [Split-Horizon Passive Monitoring Guide](deployments/ha_passive.md) for a detailed architecture.

### 4. Cold-Start Prevention
Upon startup, a GSLB instance fetches the current health status of all backends from Redis. This prevents "cold-start" routing inconsistency while the local checks execute their first cycle.

### 5. Fail-Open Resiliency
If the shared Redis server becomes unreachable, GSLB instances automatically fall back to independent local active health checking. Once Redis is restored, they seamlessly rejoin the shared cluster.

---

## Configuration Summary

To enable state synchronization, add the following options to your Corefile `gslb` block:

```nginx
gslb {
    redis_enable true
    redis_addr "127.0.0.1:6379"
    redis_password ""
    redis_db 0
    redis_key_prefix "gslb:"
    redis_sync_mode lock # 'lock' (deduplicated) or 'none' (independent report)
}
```

*For step-by-step setup guides, refer to:*
* [Redis HA Deployment Guide](deployments/ha_redis.md)
* [Split-Horizon Passive Deployment Guide](deployments/ha_passive.md)
