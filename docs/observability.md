
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
- `gslb_record_resolution_total{name, result}`: Total number of GSLB record resolutions, labeled by record name and result.
- `gslb_config_reload_total{result}`: Total number of config reloads, labeled by result (success/failure).


You can then scrape metrics at http://localhost:9153/metrics
