# Contributing

Contributions are welcome and appreciated! Whether it's fixing a bug, improving documentation, adding a feature, or enhancing tests

Before opening a pull request, please read the following guidelines to ensure smooth collaboration.

## Contribution Guidelines

- Keep the project backward compatible and follow existing code conventions.
- Add unit tests for any new features, bug fixes, or important logic changes.
- Make sure the project still passes all existing tests:
- Document any relevant changes
- Use descriptive commit messages and clean up the history before submitting your PR.

## Running the Dev Environment with Docker compose

Build CoreDNS with the plugin

~~~ bash
sudo docker compose --progress=plain build
~~~

Start the stack (CoreDNS + webapps)

~~~ bash
sudo docker compose up -d 
~~~

Wait some seconds and test the DNS resolution

~~~ bash
$ dig -p 8053 @127.0.0.1 webapp.gslb.example.com +short
172.16.0.10
~~~

### Simulate Failover

Stop the webapp 1

~~~ bash
sudo docker compose stop webapp10
~~~

Wait 30 seconds, then resolve again:

~~~ bash
$ dig -p 8053 @127.0.0.1 webapp.gslb.example.com +short
172.16.0.11
~~~

Restart Webapp 1:

~~~ bash
sudo docker compose start webapp10
~~~

Wait a few seconds, then resolve again to observe traffic switching back to Webapp 1:

~~~ bash
$ dig -p 8053 @127.0.0.1 webapp.gslb.example.com +short
172.16.0.10
~~~

## Compilation

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

## Running Unit Tests

Run a specific test

~~~ bash
go test -timeout 10s -cover -v . -run TestGSLB_PickFailoverBackend
~~~