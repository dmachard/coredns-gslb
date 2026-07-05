# Resource & Performance Benchmarks

Below is a resource usage summary (CPU, memory, goroutines) of CoreDNS-GSLB under different scale levels.

All benchmarks were run with a default interval configuration (`scrape_interval=10s`, `scrape_timeout=2s`, `scrape_retries=1`) under a worst-case scenario where backends do not respond (creating maximum timeout wait and connection states).

## HTTP Health Checks

| Records | Backends/record | Total Healthchecks | CPU Usage | Memory Usage | Goroutines |
|---------|-----------------|--------------------|-----------|--------------|------------|
| 100     | 2               | 200                | ~0.5%     | ~81.7 MB     | ~500       |
| 1000    | 2               | 2,000              | ~4.5%     | ~120 MB      | ~1,500     |
| 5000    | 2               | 10,000             | ~18%      | ~286 MB      | ~5,500     |
| 5000    | 3               | 15,000             | ~28%      | ~375 MB      | ~5,500     |
| 10000   | 2               | 20,000             | ~48%      | ~433 MB      | ~10,500    |
| 10000   | 3               | 30,000             | ~60%      | ~667 MB      | ~10,700    |

## TCP Health Checks

| Records | Backends/record | Total Healthchecks | CPU Usage | Memory Usage | Goroutines |
|---------|-----------------|--------------------|-----------|--------------|------------|
| 100     | 2               | 200                | ~0.5%     | ~84 MB       | ~400       |
| 1000    | 2               | 2,000              | ~2%       | ~118 MB      | ~1,600     |
| 5000    | 2               | 10,000             | ~13%      | ~286 MB      | ~5,600     |
| 5000    | 3               | 15,000             | ~25%      | ~344 MB      | ~5,800     |
| 10000   | 2               | 20,000             | ~40%      | ~460 MB      | ~10,600    |
| 10000   | 3               | 30,000             | ~51%      | ~600 MB      | ~10,800    |

## Lua Health Checks

*Note: Lua health checks execute custom scripting per backend using an embedded Lua VM, which incurs higher resource overhead.*

| Records | Backends/record | Total Healthchecks | CPU Usage | Memory Usage | Goroutines |
|---------|-----------------|--------------------|-----------|--------------|------------|
| 100     | 2               | 200                | ~2.5%     | ~128 MB      | ~1,300     |
| 1000    | 2               | 2,000              | ~26%      | ~218 MB      | ~4,200     |
| 5000    | 2               | 10,000             | ~147%     | ~373 MB      | ~8,200     |
