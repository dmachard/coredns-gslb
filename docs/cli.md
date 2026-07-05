# gslbctl CLI

The `gslbctl` command-line tool allows you to interact with the CoreDNS-GSLB.

The CLI can be used:
- as a **standalone binary**
- or **embedded in a Docker image** alongside CoreDNS

## Installation

### Standalone binary

A prebuilt binary is available in the project **GitHub releases**.
Download the appropriate `gslbctl_<version>_<os>_<arch>.tar.gz` archive for your platform, extract it and run it.

### From the Docker image

The CLI is also included in the Docker image that embeds CoreDNS with the GSLB plugin.

You can execute gslbctl directly from the container:

```bash
docker run --rm <image> gslbctl <command> [options]
```

## Usage

```
gslbctl <command> [options]
```

## Commands

- `backends enable [--tags tag1,tag2] [--address addr] [--location loc]`  
  Enable backends by tags, address prefix, or location.
- `backends disable [--tags tag1,tag2] [--address addr] [--location loc]`  
  Disable backends by tags, address prefix, or location.
- `status`  
  Show the current GSLB status (all records and backends).

## Examples

Enable all backends with tag `prod`:
```
gslbctl backends enable --tags prod
```
Enable all backends in location `eu-west-1`:
```
gslbctl backends enable --location eu-west-1
```
Example output (if backends are affected):
```
ZONE                    RECORD                        BACKEND
app-x.gslb.example.com. webapp.app-x.gslb.example.com. 172.16.0.10
app-x.gslb.example.com. webapp.app-x.gslb.example.com. 172.16.0.11
```
Or, if no backends are affected:
```
Backends updated successfully.
```

Disable all backends with tag `test` or `hdd`:
```
gslbctl backends disable --tags test,hdd
```
Disable all backends in location `eu-west-1`:
```
gslbctl backends disable --location eu-west-1
```
Example output (if backends are affected):
```
ZONE                    RECORD                        BACKEND
app-x.gslb.example.com. webapp.app-x.gslb.example.com. 172.16.0.10
```