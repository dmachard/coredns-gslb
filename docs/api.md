# GSLB REST API

## Authentication

If HTTP Basic Auth is configured (see Corefile options `api_basic_user` and `api_basic_pass`), all endpoints require authentication.

## TLS/HTTPS Support

You can enable HTTPS for the REST API by specifying the following options in your Corefile:

- `api_tls_cert`: Path to the TLS certificate file
- `api_tls_key`: Path to the TLS private key file

Example in Corefile:
```
gslb {
    api_tls_cert /etc/ssl/certs/mycert.pem
    api_tls_key /etc/ssl/private/mykey.pem
}
```

When enabled, the API will be served over HTTPS (default port 8080 unless changed with `api_listen_port`).

**Example curl call with HTTPS:**
```bash
curl -k https://localhost:8080/api/overview
```
- The `-k` flag allows curl to connect to self-signed certificates (remove it if using a trusted CA).

## Endpoints

For all request/response schemas and detailed documentation, see [swagger.yaml](https://petstore.swagger.io/?url=https://raw.githubusercontent.com/dmachard/coredns-gslb/refs/heads/main/docs/swagger.yaml).

### Example: GET /api/overview
```bash
curl http://localhost:8080/api/overview
```

Example response:
```json
{
  "zone1.example.com.": [
    {
      "record": "webapp1.zone1.example.com.",
      "status": "healthy",
      "backends": [
        {
          "address": "172.16.0.10",
          "alive": "healthy",
          "last_healthcheck": "2025-07-21T13:03:29Z"
        }
      ]
    }
  ],
  "zone2.example.com.": [
    {
      "record": "webapp2.zone2.example.com.",
      "status": "unhealthy",
      "backends": [
        {
          "address": "172.16.0.20",
          "alive": "unhealthy",
          "last_healthcheck": "2025-07-21T13:03:29Z"
        }
      ]
    }
  ]
}
```

### Example: GET /api/overview/{zone}
```bash
curl http://localhost:8080/api/overview/zone1.example.com.
```

Example response:
```json
[
  {
    "record": "webapp1.zone1.example.com.",
    "status": "healthy",
    "backends": [
      {
        "address": "172.16.0.10",
        "alive": "healthy",
        "last_healthcheck": "2025-07-21T13:03:29Z"
      }
    ]
  }
]
```

If the zone does not exist:
```json
{"error": "Zone not found"}
```

### Example: Bulk disable backends
```bash
curl -X POST http://localhost:8080/api/backends/disable \
  -H "Content-Type: application/json" \
  -d '{"location":"eu-west-1"}'
```

#### Disable by tags
```bash
curl -X POST http://localhost:8080/api/backends/disable \
  -H "Content-Type: application/json" \
  -d '{"tags":["prod","ssd"]}'
```
This will disable all backends that have at least one of the specified tags.

### Example: Bulk to enable all backends
```bash
curl -X POST http://localhost:8080/api/backends/enable \
  -H "Content-Type: application/json" \
  -d '{"location":"eu-west-1"}'
```

#### Enable by tags
```bash
curl -X POST http://localhost:8080/api/backends/enable \
  -H "Content-Type: application/json" \
  -d '{"tags":["prod","ssd"]}'
```
This will enable all backends that have at least one of the specified tags.