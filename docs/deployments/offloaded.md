# Offloaded Probing Deployment

This guide describes how to configure and deploy CoreDNS-GSLB in an **Offloaded Probing** configuration, where health check execution is decoupled from the DNS resolution path.

---

## Configuration Example

### Passive GSLB Resolver Node Configuration (`Corefile`)

Configure the GSLB instances with Redis enabled:

```nginx
gslb {
    zone app.example.org. /etc/coredns/db.app.yml
    redis_enable true
    redis_addr "redis.shared.local:6379"
    redis_password "securepass"
    redis_sync_mode lock
}
```

In the zone YAML file (`/etc/coredns/db.app.yml`), mark the backends as **passive**:

```yaml
records:
  api.example.org.:
    mode: failover
    backends:
      - address: "10.0.1.10"
        priority: 1
        passive: true   # Will NOT run local probes, reads from Redis
      - address: "10.0.1.11"
        priority: 2
        passive: true   # Will NOT run local probes, reads from Redis
```

---

### Active Probing Agent Configuration

Configure the probing agent to connect to the same Redis instance:

```nginx
gslb {
    zone app.example.org. /etc/coredns/db.app.yml
    redis_enable true
    redis_addr "redis.shared.local:6379"
    redis_password "securepass"
    redis_sync_mode lock
}
```

In its local zone YAML file, configure the backends with **active health checks**:

```yaml
healthcheck_profiles:
  http_check:
    type: http
    params:
      port: 80
      uri: "/healthz"

records:
  api.example.org.:
    mode: failover
    backends:
      - address: "10.0.1.10"
        priority: 1
        healthchecks: [ http_check ] # Actively supervised locally
      - address: "10.0.1.11"
        priority: 2
        healthchecks: [ http_check ] # Actively supervised locally
```
