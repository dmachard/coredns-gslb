## CoreDNS-GSLB: Troubleshooting

### To log Health Checks

Example Corefile block:

~~~
. {
    # To log healthcheck results
    debug
}
~~~

### TXT Record Support for Debugging

By default, the GSLB plugin supports DNS TXT queries for any managed domain. When you query a domain with type TXT, the plugin returns a TXT record for each backend, summarizing:
- Backend address (IP)
- Priority
- Health status (healthy/unhealthy)
- Enabled status (true/false)

This feature is useful for debugging and monitoring: you can instantly see the state of all backends for a domain with a single DNS TXT query.

**Example:**

```
dig TXT webapp.gslb.example.com.
```

**Sample response:**

```
webapp.gslb.example.com. 30 IN TXT "Backend: 172.16.0.10 | Priority: 1 | Status: healthy | Enabled: true"
webapp.gslb.example.com. 30 IN TXT "Backend: 172.16.0.11 | Priority: 2 | Status: unhealthy | Enabled: true"
```

This makes it easy to monitor backend health and configuration in real time using standard DNS tools.

**Note:**
If you want to disable TXT record support (for security or privacy reasons), add the `disable_txt` option in your `gslb` block in the Corefile:

~~~corefile
gslb {
    ...
    disable_txt
}
~~~

With `disable_txt` enabled, TXT queries for GSLB-managed zones will be passed to the next plugin (or return empty if none). No backend information will be exposed via TXT records.

### Unexpected SOA / NXDOMAIN responses

When running CoreDNS-GSLB behind any resolver that performs modern DNS probing, you may see intermittent responses returning only the SOA record or NXDOMAIN, even though the GSLB record is healthy.

This is usually caused by resolvers are sending HTTPS (type 65) or SVCB DNS queries.
CoreDNS-GSLB does not serve these record types for GSLB-managed zones, so it returns NXDOMAIN + SOA.
Resolvers then cache this negative response, causing subsequent A/CNAME lookups to temporarily return the SOA instead of the correct GSLB result.

Solution to prevent negative caching:

Ignore HTTPS queries explicitly in your Corefile:

```
template IN HTTPS {
    rcode NOERROR
}
```

Optionally, you may also ignore SVCB queries:

```
template IN SVCB {
    rcode NOERROR
}
```
Issue report with full details: https://github.com/dmachard/CoreDNS-GSLB/issues/83

### Why are all backend IPs returned in the DNS response?

If you observe that a DNS query returns all configured backend IPs instead of a single one (or based on your selection mode), it is likely due to the **fail-safe mechanism**.

This happens when the GSLB plugin detects that **no backends are healthy**. This is common in two scenarios:
1. **Initial Startup**: Immediately after CoreDNS starts, backends are marked as unhealthy until their first health check completes successfully.
2. **Total Outage**: If all health checks for a specific record are failing simultaneously.

In these cases, the plugin returns all enabled backends to ensure service continuity, assuming that trying an "unhealthy" backend is better than returning no result at all. Once at least one backend passes its health check, the plugin will resume its normal selection logic (Failover, Round-Robin, etc.).
