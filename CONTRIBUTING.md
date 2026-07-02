# Contributing

Contributions are welcome and appreciated! Whether it's fixing a bug, improving documentation, adding a feature, or enhancing tests

Before opening a pull request, please read the following guidelines to ensure smooth collaboration.

## 1. Contribution Guidelines

- Keep the project backward compatible and follow existing code conventions.
- Add unit tests for any new features, bug fixes, or important logic changes.
- Make sure the project still passes all existing tests:
- Document any relevant changes
- Use descriptive commit messages and clean up the history before submitting your PR.

## 2. Running the Dev Environment with Docker Compose

Certificates are generated on-demand for TLS and mTLS validation
see the cert_gen folder and the webapp init.sh for details

Build CoreDNS with the plugin

~~~ bash
sudo docker compose -f docker-compose.dev.yml --progress=plain build
~~~

Start the stack (CoreDNS + webapps)

~~~ bash
sudo docker compose -f docker-compose.dev.yml up -d
~~~

Wait some seconds and test the DNS resolution

~~~ bash
$ dig -p 8053 @127.0.0.1 webapp.app-x.gslb.example.com +short
172.16.0.10
~~~

Stop the webapp 1 to simulate a failover

~~~ bash
sudo docker compose -f docker-compose.dev.yml stop webapp10
~~~

Wait 30 seconds, then resolve again:

~~~ bash
$ dig -p 8053 @127.0.0.1 webapp.app-x.gslb.example.com +short
172.16.0.11
~~~

Restart Webapp 1:

~~~ bash
sudo docker compose -f docker-compose.dev.yml start webapp10
~~~

Wait a few seconds, then resolve again to observe traffic switching back to Webapp 10:

~~~ bash
$ dig -p 8053 @127.0.0.1 webapp.app-x.gslb.example.com +short
172.16.0.10
~~~

Testing GeoIP from specific region selection with EDNS Client Subnet
Simulate a query coming from subnet 10.1.0.0/24

~~~ bash
$ dig -p 8053 @127.0.0.1 webapp-geoip-loc.app-y.gslb.example.com +short +subnet=10.1.0.42/24
172.16.0.10
~~~

Simulate a query coming from subnet 10.2.0.0/24

~~~ bash
$ dig -p 8053 @127.0.0.1 webapp-geoip-loc.app-y.gslb.example.com +short +subnet=10.2.0.7/24
172.16.0.11
~~~


Testing GeoIP with country selection, based EDNS Client Subnet
Simulate a query coming from an US IP

~~~ bash
$ dig -p 8053 @127.0.0.1 webapp-geoip-country.app-y.gslb.example.com +short +subnet=8.8.8.8/24
172.16.0.11
~~~

Simulate a query coming from an FR IP

~~~ bash
$ dig -p 8053 @127.0.0.1 webapp-geoip-country.app-y.gslb.example.com +short +subnet=90.0.0.0/24
172.16.0.10
~~~

Cleanup docker compose artifacts, the `-v` flag ensures the devcert volume is cleaned up

~~~ bash
sudo docker compose -f docker-compose.dev.yml down -v
~~~

## 3. Binary Compilation with the Plugin

The `GSLB` plugin must be integrated into CoreDNS during compilation.

1. Add the following line to plugin.cfg before the file plugin. It is recommended to put this plugin right before **file:file**

~~~ text
gslb:github.com/dmachard/coredns-gslb
~~~

2. Recompile CoreDNS:

~~~ bash
go generate
make
~~~

## 4. Running Unit Tests

Run all unit tests

~~~ bash
make test-unit
~~~

Run a specific test

~~~ bash
go test -timeout 10s -cover -v . -run TestGSLB_PickFailoverBackend
~~~

## 5. Running Integration Tests

You can run the entire integration test suite (failover, GeoIP, and API checks) locally using:

~~~ bash
make test-integration
~~~

### Troubleshooting Port Conflicts

If some ports (like `8080` or `8053`) are already in use on your host machine, you will see a Docker bind error. You can override these ports using environment variables:

~~~ bash
COREDNS_PORT_API=8082 COREDNS_PORT_TCP=8055 make test-integration
~~~

Supported variables:
- `COREDNS_PORT_API` (default: `8080`)
- `COREDNS_PORT_TCP` (default: `8053`)
- `COREDNS_PORT_UDP` (default: `8053`)
- `COREDNS_PORT_METRICS` (default: `9153`)

## 6. Running Linters

**Install make:**

**Debian based:**
```bash
sudo apt install build-essential
```
**RHEL based:**
```bash
sudo dnf group install c-development
```

**Install linter:**

```bash
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
```

**Execute linter before commit:**

```bash
make lint
```

## 7. Updating CoreDNS

```bash
go mod edit -go=1.25
go get github.com/coredns/coredns@v1.14.4
go get github.com/miekg/dns@v1.1.72
go mod tidy
```
