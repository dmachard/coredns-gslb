# Service Discovery & Backend Discovery

CoreDNS-GSLB allows you to build and manage backend pools using three distinct approaches: static configuration, dynamic management via the REST API, or dynamic Service Discovery from external catalogs and DNS endpoints.

- **Static Configuration**: Configured directly in the zone YAML files. Best for stable, fixed backend pools.
- **REST API**: Managed via HTTP REST endpoints (bulk PUT/POST operations). Best for CI/CD integrations.
- **Backend Discovery**: Dynamically fetched from external service registries (Consul, HTTP endpoints, or DNS SVCB/HTTPS records).

---

## Static Configuration

Static configurations are defined under the `backends` block of a record in your zone files.

```yaml
records:
  web.example.org.:
    mode: "roundrobin"
    backends:
      - address: "192.168.1.10"
        port: 80
        weight: 10
        priority: 1
      - address: "192.168.1.11"
        port: 80
        weight: 10
        priority: 1
```

You can add a `tags` list to any backend in your YAML configuration. These tags are strings that you can use to group, filter, or target backends for bulk API operations.

```yaml
records:
  webapp.example.org.:
    backends:
      - address: "172.16.0.10"
        tags: ["prod", "ssd", "eu"]
      - address: "172.16.0.11"
        tags: ["test", "hdd", "us"]
```

> - You can assign any number of tags to a backend.
> - The API uses tags to enable/disable backends in bulk.

---

## Management via REST API

You can enable the REST API server to dynamically register, update, or remove backends. This is useful for automated pipelines or external monitoring tools.

Corefile Configuration

```
gslb {
    api_enable true
    api_listen_addr 0.0.0.0
    api_listen_port 8080
}
```

Endpoints
- `PUT /api/v1/zones/{zone}/records/{fqdn}/backends`: Overwrite the backend pool for a specific record.
- `GET /api/v1/status`: View all configured records, backends, and their health statuses.

---

## Backend Discovery

CoreDNS-GSLB can query external registries at a regular interval to dynamically refresh its backend pool, enabling zero-touch configuration.

> [!NOTE]
> CoreDNS-GSLB delegates health checking to the discovery source. Discovered endpoints are assumed to be healthy. The discovery source (e.g. Consul catalog, custom HTTP endpoint, or upstream DNS server) is responsible for performing health checks and/or only returning active nodes, which CoreDNS-GSLB picks up on the next scrape interval.

### Consul Catalog Discovery

Fetches services from the Consul Catalog API.

Configuration Example:

```yaml
records:
  api.example.org.:
    mode: "roundrobin"
    discovery:
      type: "consul"
      endpoint: "http://localhost:8500"
      service: "web-service"
      tag: "prod"  # Optional tag filter
      interval: "10s"
```

Consul JSON Response Mapping:

CoreDNS-GSLB maps the `"ServiceAddress"` (or `"Address"` if empty) and `"ServicePort"` parameters returned from `/v1/catalog/service/{service}`.
If `tag` is specified, it will append the tag as a query parameter (e.g. `/v1/catalog/service/{service}?tag=prod`) to filter endpoints at the source.

---

### HTTP Discovery

Queries a custom HTTP JSON endpoint returning either a list of IP address strings or a list of structured backend objects.

Configuration Example

```yaml
records:
  api.example.org.:
    mode: "roundrobin"
    discovery:
      type: "http"
      endpoint: "http://my-registry.internal/endpoints"
      interval: "15s"
```

Supported JSON Structures:

1. **Simple List of IP Strings:**
   ```json
   ["10.0.0.1", "10.0.0.2"]
   ```
2. **Detailed Backend Objects:**
   ```json
   [
     {"address": "10.0.0.1", "port": 8080},
     {"address": "10.0.0.2", "port": 8081}
   ]
   ```

---

### Upstream DNS Discovery

Queries upstream DNS servers for `SVCB` or `HTTPS` records (RFC 9460). It parses target hosts, port numbers, ALPN support lists, and IP address hints (`ipv4hint` / `ipv6hint`) to build the backend pool. If no hints are returned but a target is set, it issues fallback `A` / `AAAA` queries to resolve the target domain's IP addresses.

Configuration Example:

```yaml
records:
  secure.example.org.:
    mode: "roundrobin"
    discovery:
      type: "dns_svcb"  # Or "dns_https"
      endpoint: "10.0.0.10:53"
      service: "_https._tcp.secure.internal."
      interval: "30s"
```

