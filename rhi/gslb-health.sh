#!/bin/bash
# BGP Route Health Injection (RHI) script via FRRouting.
# It checks GSLB health and dynamically adds/removes the Anycast IP on a dummy interface.

METRICS_URL="${METRICS_URL:-http://localhost:9153/metrics}"
ZONE="${ZONE:-webapp.gslb.example.com.}"
ANYCAST_IP="${ANYCAST_IP:-192.168.100.10/32}"
INTERFACE="${INTERFACE:-dummy0}"

# Scrape the metric coredns_gslb_record_health_status for the zone
STATUS=$(curl -sf "$METRICS_URL" | grep "^coredns_gslb_record_health_status{.*fqdn=\"$ZONE\".*}" | awk '{print $2}')

if [ "$STATUS" = "1" ]; then
    # Healthy: Ensure Anycast IP is present on the interface so FRR advertises it
    if ! ip addr show dev "$INTERFACE" | grep -q "${ANYCAST_IP%/*}"; then
        ip addr add "$ANYCAST_IP" dev "$INTERFACE"
    fi
    exit 0
else
    # Unhealthy: Ensure Anycast IP is removed so FRR withdraws it
    if ip addr show dev "$INTERFACE" | grep -q "${ANYCAST_IP%/*}"; then
        ip addr del "$ANYCAST_IP" dev "$INTERFACE"
    fi
    exit 1
fi
