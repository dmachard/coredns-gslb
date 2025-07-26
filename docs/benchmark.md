# Benchmarking CoreDNS-GSLB

Below is a summary of resource usage (CPU, memory, goroutines) for different scales (scrape_interval/timeout/retry = 10s/2s/1) and in worse case (no backend response)

##  HTTP healthchecks

| Records | Backends/record | Healthchecks/backend | CPU    | Memory    | Goroutines |
|---------|-----------------|----------------------|--------|-----------|------------|
| 100     | 2 (200)         | 1 (200)              | ~0.5%  | ~81.7 MB  | ~500       |
| 1000    | 2 (2000)        | 1 (2000)             | ~4.5%  | ~120 MB   | ~1500      |
| 5000    | 2 (10000)       | 1 (10000)            | ~18%   | ~286 MB   | ~5500      |
| 5000    | 3 (15000)       | 1 (15000)            | ~28%   | ~375 MB   | ~5500      |
| 10000   | 2 (20000)       | 1 (20000)            | ~48%   | ~433 MB   | ~10500     |
| 10000   | 3 (30000)       | 1 (30000)            | ~60%   | ~667 MB   | ~10700     |

##  TCP healthchecks

| Records | Backends/record | Healthchecks/backend | CPU    | Memory    | Goroutines |
|---------|-----------------|----------------------|--------|-----------|------------|
| 100     | 2 (200)         | 1 (200)              | ~0.5%  | ~84 MB    | ~400       |
| 1000    | 2 (2000)        | 1 (2000)             | ~2%    | ~118 MB   | ~1600      |
| 5000    | 2 (10000)       | 1 (10000)            | ~13%   | ~286 MB   | ~5600      |
| 5000    | 3 (15000)       | 1 (15000)            | ~25%   | ~344 MB   | ~5800      |
| 10000   | 2 (20000)       | 1 (20000)            | ~40%   | ~460 MB   | ~10600     |
| 10000   | 3 (30000)       | 1 (30000)            | ~51%   | ~600 MB   | ~10800     |

##  LUA healthchecks

| Records | Backends/record | Healthchecks/backend | CPU    | Memory    | Goroutines |
|---------|-----------------|----------------------|--------|-----------|------------|
| 100     | 2 (200)         | 1 (200)              | ~2.5%  | ~128 MB   | ~1300      |
| 1000    | 2 (2000)        | 1 (2000)             | ~26%   | ~218 MB   | ~4200      |
| 5000    | 2 (10000)       | 1 (10000)            | ~147%  | ~373 MB   | ~8200      |

## Benchmarking CoreDNS-GSLB

This guide explains how to evaluate the performance of CoreDNS-GSLB based on the number of records, backends, and healthchecks.

A Python script is provided to automatically generate test YAML files:

```bash
./tests/gen_gslb_config.py --records 1000 --backends 3 --healthchecks 2 --healthcheck-type tcp  --output gslb_bench.yml
```

- `--records`: number of DNS records to generate
- `--backends`: number of backends per record
- `--healthchecks`: number of healthcheck profiles per backend
- `--output`: name of the generated YAML file

The generated file can be used as a configuration for a zone in CoreDNS-GSLB.
