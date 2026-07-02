# CoreDNS-GSLB: EDNS Client Subnet (ECS) & Caching

The GSLB plugin supports the EDNS Client Subnet (ECS) option defined in **RFC 7871**. This is critical for deployments behind public resolvers (like Google Public DNS or Cloudflare) or forwarders, as it allows GSLB to route clients based on their real IP address rather than the resolver's IP address.

## 1. Enabling ECS

To enable ECS support, add the `use_edns_csubnet` option to the `gslb` block in your Corefile:

```corefile
gslb {
    zone example.org. db.example.org.yml
    use_edns_csubnet
}
```

## 2. Client IP Determination

When `use_edns_csubnet` is enabled, CoreDNS-GSLB determines the client's IP in the following order:
1. **ECS Option**: If the incoming DNS query contains an EDNS0 Client Subnet option, GSLB parses the subnet address and uses it for GeoIP routing, hashing, and logs.
2. **Connection Remote Address**: If the query does not contain an ECS option, GSLB falls back to the socket's remote IP address (transport layer IP).

If `use_edns_csubnet` is disabled, GSLB will always use the connection's remote IP address.

---

## 3. RFC 7871 Scope Compliance & Caching

To ensure optimal caching efficiency on recursive resolvers and to prevent cache hazards or cache poisoning, CoreDNS-GSLB carefully manages the **Source Scope** returned in the ECS response.

### Geo-Routed Responses (Scoped Cache)
For record types that are processed by GSLB using geo-routing or IP-hashing modes (`geoip`, `geoip_affinity`, `hash`, `ip-hash`, `client-ip-hash`):
* **Behavior**: GSLB echoes back the client's subnet mask as the **Source Scope** (e.g. `/24` for IPv4 or `/48` for IPv6).
* **Result**: The recursive resolver caches this specific response only for clients belonging to that subnet. Clients in other subnets will trigger new queries to GSLB, ensuring they receive the best localized IP.

### Static, Fallback, or NODATA Responses (Global Cache)
For responses where the answer is identical for all clients regardless of location, GSLB returns a **Source Scope of `/0` (global)**. This allows resolvers to cache the response globally, drastically reducing GSLB query load and preventing cache fragmentation:
* **Fallthrough Queries**: Queries for records GSLB is not responsible for (e.g. `MX`, `SOA`, `NS`, `CAA` or subdomains not configured in GSLB) that fall through to downstream plugins (like `file` or `hosts`).
* **Unconfigured Record Types (NODATA)**: Queries for record types that exist in GSLB but do not have the requested type configured (e.g., querying `AAAA` on a domain that only has IPv4/`A` backends). GSLB directly returns a `NOERROR` with empty answers (NODATA) and a `/0` scope.
* **Fail-Closed Responses**: When all backends for a record are unhealthy and the failover policy is set to `fail-closed` (returning `NXDOMAIN`, `REFUSED`, or `SERVFAIL`).
* **TXT Status Queries**: Queries for TXT records (which return a static summary of backend health and priority for administrative inspection).
