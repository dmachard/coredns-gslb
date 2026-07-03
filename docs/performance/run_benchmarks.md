# Running Benchmarks Locally

You can evaluate the performance of CoreDNS-GSLB under workloads matching your production scale (number of zones, records, backends, and health check types).

A helper Python script is provided in the repository to automatically generate large-scale test configuration YAML files.

## Generating Configuration Profiles

Run the Python generator script located in the `tests/` directory:

```bash
./tests/gen_gslb_config.py --records 1000 --backends 3 --healthchecks 2 --healthcheck-type tcp --output gslb_bench.yml
```

### Argument Reference

*   `--records`: The number of unique DNS records to generate.
*   `--backends`: The number of backend endpoints per record.
*   `--healthchecks`: The number of healthcheck profiles per backend.
*   `--healthcheck-type`: The protocol/type of healthcheck to configure (`tcp`, `http`, `lua`, etc.).
*   `--output`: Filename of the generated YAML configuration.

## Testing Procedure

1. Generate the benchmark YAML configuration using the script above.
2. Configure CoreDNS-GSLB to load the generated `gslb_bench.yml` file.
3. Start CoreDNS and monitor its resource usage (CPU, RSS memory, goroutine count) via the `/metrics` endpoint or system monitoring tools (`top`, `ps`).
4. Generate queries to measure resolution latency under load.
