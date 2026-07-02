---
hide:
  - navigation
  - toc
  - path
---

<h1 style="display: none;">Home</h1>

<div class="hero-card">
  <div class="hero-title">CoreDNS-GSLB</div>
  <p style="font-size: 1.15rem; max-width: 650px; margin: 0.5rem auto 1.5rem auto; line-height: 1.6;">
    An open-source Global Server Load Balancing (GSLB) plugin for CoreDNS. Efficient traffic routing designed for VMs, bare-metal servers, and hybrid environments.
  </p>
  <div style="display: flex; gap: 12px; justify-content: center; flex-wrap: wrap;">
    <a href="getting_started/" class="btn-primary">Get Started</a>
  </div>
</div>

## Why CoreDNS-GSLB?

<div class="grid-2-cols">
  <div class="feature-box">
    <h3>No Vendor Lock-In</h3>
    <p>A lightweight, open-source alternative to expensive, proprietary GSLB hardware boxes. Run it on standard VMs or bare-metal without vendor lock-in.</p>
  </div>
  <div class="feature-box">
    <h3>Not Everything Runs in Kubernetes</h3>
    <p>CoreDNS-GSLB brings advanced global server load balancing to VMs, bare-metal servers, and hybrid cloud environments.</p>
  </div>
  <a href="healthchecks/" class="feature-box" style="text-decoration: none; color: inherit; display: block;">
    <h3>Modern Health Checks</h3>
    <p>Supports HTTP/S, TCP, ICMP, MySQL, gRPC, and custom Lua scripting to instantly reroute traffic when a backend fails.</p>
  </a>
  <a href="geoip/" class="feature-box" style="text-decoration: none; color: inherit; display: block;">
    <h3>GeoIP Routing</h3>
    <p>Uses MaxMind databases and EDNS Client Subnet (ECS) to route users to the closest geographical backend.</p>
  </a>
  <a href="records/" class="feature-box" style="text-decoration: none; color: inherit; display: block;">
    <h3>Supported Records</h3>
    <p>Natively resolves and dynamically routes not only standard A/AAAA but also SRV, SVCB, HTTPS, TXT, and wildcard records for modern application delivery.</p>
  </a>
  <a href="discovery/" class="feature-box" style="text-decoration: none; color: inherit; display: block;">
    <h3>Backend Discovery</h3>
    <p>Automatically discovers backend endpoints from external registries like Consul, custom HTTP JSON APIs, or upstream DNS records with zero-touch configuration.</p>
  </a>
</div>

## Where to Start?

<div class="grid-container" style="grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));">
  <a href="getting_started/" class="feature-box" style="text-decoration: none; color: inherit; display: block;">
    <h3>1. Getting Started</h3>
    <p>Run your first failover setup with Docker Compose.</p>
  </a>
  <a href="configuration/" class="feature-box" style="text-decoration: none; color: inherit; display: block;">
    <h3>2. Configuration</h3>
    <p>Configure DNS zones, load balancing algorithms, and metrics.</p>
  </a>
  <a href="healthchecks/" class="feature-box" style="text-decoration: none; color: inherit; display: block;">
    <h3>3. Health Checks</h3>
    <p>Build robust healthcheck profiles for your endpoints.</p>
  </a>
  <a href="api/" class="feature-box" style="text-decoration: none; color: inherit; display: block;">
    <h3>4. REST API</h3>
    <p>Dynamically enable/disable backends via REST endpoints.</p>
  </a>
</div>

<h2 style="margin-top: 4rem; margin-bottom: 1.5rem;">More DNS tools?</h2>

<div class="grid-2-cols" style="margin-top: 1rem;">
  <a href="https://github.com/dmachard/DNS-collector" target="_blank" class="feature-box" style="text-decoration: none; color: inherit; display: block;">
    <h3 style="margin-top: 0;">DNS-collector</h3>
    <p style="margin-bottom: 0; font-size: 0.95rem; line-height: 1.5;">Grab your DNS logs, detect anomalies, analyze traffic patterns, and finally understand exactly what's happening on your network in real-time.</p>
  </a>
  <a href="https://github.com/dmachard/DNS-tester" target="_blank" class="feature-box" style="text-decoration: none; color: inherit; display: block;">
    <h3 style="margin-top: 0;">DNS-tester</h3>
    <p style="margin-bottom: 0; font-size: 0.95rem; line-height: 1.5;">A comprehensive DNS testing and verification toolkit designed to validate DNS response behavior and performance under various network conditions.</p>
  </a>
</div>

<div style="text-align: center; margin-top: 3rem; opacity: 0.7; font-size: 0.9rem;">
  Released under the MIT License. Made by <a href="https://github.com/dmachard" target="_blank" style="color: inherit; text-decoration: underline;">@dmachard</a>
</div>
