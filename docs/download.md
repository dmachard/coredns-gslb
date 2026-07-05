# Download

CoreDNS-GSLB can be obtained as official Docker images, precompiled binaries for major operating systems, or compiled directly from source.

---

## Docker Images (Recommended)

The easiest and most common way to run CoreDNS-GSLB is via Docker. Official multi-architecture images are published to Docker Hub.

*   **Repository**: [dmachard/coredns_gslb](https://hub.docker.com/r/dmachard/coredns_gslb)
*   **Architectures**: `amd64`, `arm64`

### Pulling the Image

To get the latest release:
```bash
docker pull dmachard/coredns_gslb:latest
```

To pull a specific version (highly recommended for production):
```bash
docker pull dmachard/coredns_gslb:v0.21.0
```

---

## Precompiled Binaries

For bare-metal or virtual machine deployments, precompiled binaries for CoreDNS integrated with the GSLB plugin are available for download.

### GitHub Releases

Go to the **[GitHub Releases Page](https://github.com/dmachard/coredns-gslb/releases)** to download the archive for your platform.

Each release includes assets for:

*   **Linux**: `coredns-gslb_linux_amd64.tar.gz`, `coredns-gslb_linux_arm64.tar.gz`
*   **macOS**: `coredns-gslb_darwin_amd64.tar.gz`, `coredns-gslb_darwin_arm64.tar.gz`
*   **Windows**: `coredns-gslb_windows_amd64.zip`

---

## Source Code & Compilation

If you need to build CoreDNS with GSLB along with other third-party CoreDNS plugins, or want to build it from the latest source code:

*   **Repository**: [GitHub - dmachard/CoreDNS-GSLB](https://github.com/dmachard/CoreDNS-GSLB)
*   Refer to the **[Binary Compilation Guide](developer/compilation.md)** for step-by-step instructions on setting up `plugin.cfg` and compiling the Go binary.
