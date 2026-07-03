# CoreDNS-GSLB: Troubleshooting

## 1. Logging Health Checks

Example Corefile block:

~~~
. {
    # To log healthcheck results
    debug
}
~~~

## 2. Unexpected SOA / NXDOMAIN Responses (Legacy Workaround)

In older versions of CoreDNS-GSLB, when running behind resolvers that perform modern DNS probing (sending HTTPS/type 65 or SVCB queries), you might have seen intermittent negative caching (returning NXDOMAIN or SOA).

**This is now fully supported natively** in CoreDNS-GSLB. The plugin dynamically responds to HTTPS and SVCB queries with RFC 9460-compliant resource records, echoing ALPN, ports, and selected backend IP hints.

Therefore, the legacy workaround using the `template` plugin (e.g., `template IN HTTPS { rcode NOERROR }`) is **no longer required**.

## 3. Why are all backend IPs returned in the DNS response?

If you observe that a DNS query returns all configured backend IPs instead of a single one (or based on your selection mode), it is likely due to the **fail-safe mechanism**.

This happens when the GSLB plugin detects that **no backends are healthy**. This is common in two scenarios:

1. **Initial Startup**: Immediately after CoreDNS starts, backends are marked as unhealthy until their first health check completes successfully.
2. **Total Outage**: If all health checks for a specific record are failing simultaneously.

In these cases, the plugin returns all enabled backends to ensure service continuity, assuming that trying an "unhealthy" backend is better than returning no result at all. Once at least one backend passes its health check, the plugin will resume its normal selection logic (Failover, Round-Robin, etc.).
