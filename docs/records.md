# CoreDNS-GSLB: Supported Record Types

CoreDNS-GSLB dynamically resolves and serves several DNS resource record types to route traffic, optimize client connections, and assist with debugging.

---

## Address Records (A & AAAA)

* **A** (Type 1): Resolves IPv4 addresses for healthy backends.
* **AAAA** (Type 28): Resolves IPv6 addresses for healthy backends.
* **CNAME**: Backend addresses can be configured as a hostname/FQDN (e.g. `some-alb.aws.com`) instead of a raw IP address:
    * **Health Checks**: Standard health checks (such as HTTP, HTTPS, TCP, gRPC, or custom Lua checks) will resolve the FQDN dynamically and run normally against the resolved target.
    * **DNS Resolution**: When a query (A, AAAA, or CNAME) is received and a hostname-based backend is selected by the routing policy, the plugin automatically returns a CNAME record pointing to that FQDN target.

When a query is received, the GSLB plugin uses the configured load-balancing or routing policy (e.g., `geoip`, `round-robin`, `failover`, `weighted`, `random`) to select the best active backend(s) matching the requested address family.

If no healthy backends of the requested type are available, the plugin applies the record's configured `fallback_policy` (such as returning fallback IPs or a custom response code).

---

## Service Binding Records (SVCB & HTTPS - RFC 9460)

Modern web browsers (like Safari and Chrome) and public resolvers (like Cloudflare `1.1.1.1` or Google `8.8.8.8`) frequently send `HTTPS` (Type 65) and `SVCB` (Type 64) queries to negotiate connection parameters (e.g., HTTP/3 ALPN, ports) before initiating a connection.

CoreDNS-GSLB natively resolves these queries dynamically:

* **Target**: Always set to `.` (the queried domain name itself).
* **IP Hints**: `ipv4hint` and `ipv6hint` are dynamically populated with the IP addresses of the selected healthy backends.
* **Port**: Automatically mapped to the backend's port (if specified).
* **ALPN**: Defaults to `h3,h2` or can be customized per record.

### Configuration Example
You can configure custom ALPN parameters on your record:
```yaml
records:
  webapp.gslb.example.com.:
    mode: geoip
    alpn:
      - h3
      - h2
    backends:
      - address: 192.168.1.10
        port: 443
        enable: true
```

---

## Service Records (SRV)

For non-HTTP services (like SIP, LDAP, or directory services), CoreDNS-GSLB supports dynamic `SRV` (Type 33) records (RFC 2782) to route clients to healthy backends.

* **Priority**: Mapped directly from the backend's `priority` property.
* **Weight**: Mapped directly from the backend's `weight` property (defaulting to `1` if weight is 0 or less).
* **Port**: Mapped directly from the backend's `port` property.
* **Target**: Set to the backend's address (FQDN or IP).

If multiple backends are selected by the active routing policy, CoreDNS-GSLB returns an SRV record for each selected backend with its corresponding port, priority, and weight.

---

## TXT Records (Debugging & Monitoring)

By default, CoreDNS-GSLB serves TXT records summarizing the state of all configured backends. This is useful for real-time monitoring and debugging using standard tools.

### Example Query

```bash
dig TXT webapp.gslb.example.com @127.0.0.1
```

### Sample Response

```text
webapp.gslb.example.com. 30 IN TXT "Backend: 172.16.0.10 | Priority: 1 | Status: healthy | Enabled: true"
webapp.gslb.example.com. 30 IN TXT "Backend: 172.16.0.11 | Priority: 2 | Status: unhealthy | Enabled: true"
```

### Disabling TXT Record Support

If you wish to hide backend details for security or privacy reasons, you can disable TXT queries by adding the `disable_txt` directive to your `gslb` block in the Corefile:

```corefile
gslb {
    ...
    disable_txt
}
```

With `disable_txt` enabled, TXT queries for GSLB-managed zones will be passed to the next plugin or return empty if none.

---

## Wildcard Records Support

CoreDNS-GSLB supports standard DNS wildcard records (`*.domain.tld`) as described in RFC 1034 §4.3.3.
If a query does not match any exact record configured in a zone, GSLB will look for a wildcard record by replacing the leftmost label of the query name with `*` and walking up towards the zone apex.

### Example Configuration

```yaml
records:
  "*.example.org.":
    mode: "geoip"
    backends:
      - address: "172.16.0.10"
        priority: 1
        location: "eu-west-1"
      - address: "172.16.0.20"
        priority: 1
        location: "us-east-1"
```

With this configuration:

- A query for `anything.example.org.` will match `*.example.org.` (if there is no exact record `anything.example.org.`).
- A query for `sub.anything.example.org.` will also match `*.example.org.`.
- An exact match always takes precedence over a wildcard match.
- In case of multiple overlapping authoritative zones configured in GSLB (e.g., `sub.example.org.` and `example.org.`), wildcard lookup is strictly bounded by the most specific zone (the zone with the longest matching suffix).

