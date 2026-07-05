# CoreDNS-GSLB: Troubleshooting

## Logging Health Checks

Example Corefile block:

~~~
. {
    # To log healthcheck results
    debug
}
~~~

---

## Why are all backend IPs returned in the DNS response?

If you observe that a DNS query returns all configured backend IPs instead of a single one (or based on your selection mode), it is likely due to the **fail-safe mechanism**.

This happens when the GSLB plugin detects that **no backends are healthy**. This is common in two scenarios:

1. **Initial Startup**: Immediately after CoreDNS starts, backends are marked as unhealthy until their first health check completes successfully.
2. **Total Outage**: If all health checks for a specific record are failing simultaneously.

In these cases, the plugin returns all enabled backends to ensure service continuity, assuming that trying an "unhealthy" backend is better than returning no result at all. Once at least one backend passes its health check, the plugin will resume its normal selection logic (Failover, Round-Robin, etc.).

Please refer to the **[Fallback Behavior](modes.md#fallback-behavior)** section for more information.