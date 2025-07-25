#!/usr/bin/env python3
import argparse
import yaml

parser = argparse.ArgumentParser(description="Generate GSLB config YAML for benchmarking.")
parser.add_argument('--records', type=int, default=100, help='Number of records')
parser.add_argument('--backends', type=int, default=2, help='Number of backends per record')
parser.add_argument('--healthchecks', type=int, default=1, help='Number of healthchecks per backend')
parser.add_argument('--healthcheck-type', type=str, default='http', help='Type of healthcheck (http, tcp, icmp, etc.)')
parser.add_argument('--output', type=str, default='gslb_bench.yml', help='Output YAML file')
args = parser.parse_args()

# Example healthcheck profiles
healthcheck_profiles = {}
for i in range(args.healthchecks):
    profile = {
        'type': args.healthcheck_type,
        'params': {}
    }
    if args.healthcheck_type == 'http':
        profile['params'] = {
            'port': 8000,
            'uri': f'/health',
            'expected_code': 200,
            'timeout': '2s',
        }
    elif args.healthcheck_type == 'tcp':
        profile['params'] = {
            'port': 8000,
            'timeout': '2s',
        }
    elif args.healthcheck_type == 'icmp':
        profile['params'] = {
            'timeout': '2s',
            'count': 3,
        }
    elif args.healthcheck_type == 'lua':
        profile['params'] = {
            'timeout': '2s',
            'script': (
                'local body = http_get("http://" .. backend.address .. ":8000/health", 2)\n'
                'if not body or body == "" then\n'
                '  return false\n'
                'end\n'
                'local health = json_decode(body)\n'
                'if not health or health.status ~= "ok" then\n'
                '  return false\n'
                'end\n'
                'return true'
            ),
        }
    # Add more types as needed
    healthcheck_profiles[f"{args.healthcheck_type}_profile_{i}"] = profile

records = {}
for r in range(args.records):
    record_name = f"test{r}.bench.example.com."
    backends = []
    for b in range(args.backends):
        backend = {
            'address': f"10.0.{r}.{b+1}",
            'priority': b+1,
            'enable': True,
            'healthchecks': [f"{args.healthcheck_type}_profile_{i}" for i in range(args.healthchecks)]
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

print(f"Generated {args.output} with {args.records} records, {args.backends} backends/record, {args.healthchecks} healthchecks/backend, type={args.healthcheck_type}.") 