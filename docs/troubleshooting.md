## CoreDNS-GSLB: Troubleshooting

### To log Health Checks

Example Corefile block:

~~~
. {
    # To log healthcheck results
    debug
}
~~~

### TXT Record Support for Debugging

By default, the GSLB plugin supports DNS TXT queries for any managed domain. When you query a domain with type TXT, the plugin returns a TXT record for each backend, summarizing:
- Backend address (IP)
- Priority
- Health status (healthy/unhealthy)
- Enabled status (true/false)

This feature is useful for debugging and monitoring: you can instantly see the state of all backends for a domain with a single DNS TXT query.

**Example:**

```
dig TXT webapp.gslb.example.com.
```

**Sample response:**

```
webapp.gslb.example.com. 30 IN TXT "Backend: 172.16.0.10 | Priority: 1 | Status: healthy | Enabled: true"
webapp.gslb.example.com. 30 IN TXT "Backend: 172.16.0.11 | Priority: 2 | Status: unhealthy | Enabled: true"
```

This makes it easy to monitor backend health and configuration in real time using standard DNS tools.

**Note:**
If you want to disable TXT record support (for security or privacy reasons), add the `disable_txt` option in your `gslb` block in the Corefile:

~~~corefile
gslb {
    ...
    disable_txt
}
~~~

With `disable_txt` enabled, TXT queries for GSLB-managed zones will be passed to the next plugin (or return empty if none). No backend information will be exposed via TXT records.
