# gslbctl CLI

The `gslbctl` command-line tool allows you to interact with the CoreDNS-GSLB.

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