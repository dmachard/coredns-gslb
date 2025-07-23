# GSLB REST API

## Authentication

If HTTP Basic Auth is configured (see Corefile options `api_basic_user` and `api_basic_pass`), all endpoints require authentication.

## TLS/HTTPS Support

You can enable HTTPS for the REST API by specifying the following options in your Corefile:

- `api_tls_cert`: Path to the TLS certificate file
- `api_tls_key`: Path to the TLS private key file

Example in Corefile:
```
gslb gslb_config.yml example.com {
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

For all request/response schemas and detailed documentation, see [swagger.yaml](https://raw.githubusercontent.com/dmachard/coredns-gslb/refs/heads/main/doc/swagger.yaml).

### Example: GET /api/overview
```bash
curl -u admin:secret http://localhost:8080/api/overview
```

### Example: Bulk disable backends
```bash
curl -u admin:secret -X POST http://localhost:8080/api/backends/disable \
  -H "Content-Type: application/json" \
  -d '{"location":"eu-west-1"}'
```
