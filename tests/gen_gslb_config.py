#!/usr/bin/env python3
import argparse
import yaml

parser = argparse.ArgumentParser(description="Generate GSLB config YAML for benchmarking.")
parser.add_argument('--records', type=int, default=100, help='Number of records')
parser.add_argument('--backends', type=int, default=2, help='Number of backends per record')
parser.add_argument('--healthchecks', type=int, default=1, help='Number of healthchecks per backend')
parser.add_argument('--output', type=str, default='gslb_bench.yml', help='Output YAML file')
args = parser.parse_args()

# Example healthcheck profiles
healthcheck_profiles = {}
for i in range(args.healthchecks):
    healthcheck_profiles[f"http_profile_{i}"] = {
        'type': 'http',
        'params': {
            'port': 8000 + i,
            'uri': f'/health{i}',
            'expected_code': 200,
            'timeout': '2s',
        }
    }

records = {}
for r in range(args.records):
    record_name = f"test{r}.bench.gslb.example.com."
    backends = []
    for b in range(args.backends):
        backend = {
            'address': f"10.0.{r}.{b+1}",
            'priority': b+1,
            'enable': True,
            'location': 'eu-west-1',
            'healthchecks': [f"http_profile_{i}" for i in range(args.healthchecks)]
        }
        backends.append(backend)
    records[record_name] = {
        'mode': 'failover',
        'record_ttl': 30,
        'scrape_interval': '10s',
        'backends': backends
    }

data = {
    'healthcheck_profiles': healthcheck_profiles,
    'records': records
}

with open(args.output, 'w') as f:
    yaml.dump(data, f, default_flow_style=False)

print(f"Generated {args.output} with {args.records} records, {args.backends} backends/record, {args.healthchecks} healthchecks/backend.") 