# gslbctl CLI

The `gslbctl` command-line tool allows you to interact with the CoreDNS-GSLB.

## Usage

```
gslbctl <command> [options]
```

## Commands

- `backends enable [--tags tag1,tag2] [--address addr]`  
  Enable backends by tags or address prefix.
- `backends disable [--tags tag1,tag2] [--address addr]`  
  Disable backends by tags or address prefix.
- `status`  
  Show the current GSLB status (all records and backends).

## Examples

Enable all backends with tag `prod`:
```
gslbctl backends enable --tags prod
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
Example output (if backends are affected):
```
ZONE                    RECORD                        BACKEND
app-x.gslb.example.com. webapp.app-x.gslb.example.com. 172.16.0.10
```

Show the current status:
```
gslbctl status
```

Example output (table):
```
ZONE                    RECORD                        STATUS    BACKEND        ALIVE     LAST_HEALTHCHECK
app-x.gslb.example.com. webapp.app-x.gslb.example.com. healthy  172.16.0.10    healthy   2025-07-28T13:54:18Z
```

## Notes
- The CLI communicates with the API at `http://127.0.0.1:8080` (default).
- It is intended to be used **inside the container** (not exposed externally).
- You can use it with `docker exec`:
  ```
  docker exec -it <container> gslbctl status
  ```