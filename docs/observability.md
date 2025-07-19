
## CoreDNS-GSLB: Observability

### Metrics

If you enable the `prometheus` block in your Corefile, the plugin exposes the following metrics on `/metrics` (default port 9153):

Enable in Corefile:
```
. {
    prometheus
    ...
}
```

Available metrics:
- `gslb_healthcheck_total{type, address, result}`: Total number of healthchecks performed, labeled by type, backend address, and result (success/fail).
- `gslb_healthcheck_duration_seconds{type, address}`: Duration of healthchecks in seconds, labeled by type and backend address.
- `gslb_healthcheck_failures_total{type, address, reason}`: Total number of healthcheck failures, labeled by type, address and reason. The `reason` label can be:
    - `timeout`: Timeout or duration parsing error
    - `connection`: Network or protocol connection failure (TCP, ICMP, MySQL, gRPC, etc.)
    - `protocol`: Protocol-level error (unexpected HTTP code, MySQL query error, gRPC status, Lua script error, etc.)
    - `other`: Any other or unknown failure
- `gslb_record_resolution_total{name, result}`: Total number of GSLB record resolutions, labeled by record name and result.
- `gslb_config_reload_total{result}`: Total number of config reloads, labeled by result (success/failure).
- `gslb_backend_active{name}`: Number of active (healthy) backends per record. The `name` label is the FQDN of the GSLB record.


You can then scrape metrics at http://localhost:9153/metrics
