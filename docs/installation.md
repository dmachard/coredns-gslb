# Installation & Download

CoreDNS-GSLB can be deployed using official Docker images, precompiled binaries, or by compiling it directly from source.

---

## 1. Docker Images (Recommended)

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

## 2. Precompiled Binaries

For bare-metal or virtual machine deployments, precompiled binaries for CoreDNS integrated with the GSLB plugin are available for download.

### GitHub Releases

Go to the **[GitHub Releases Page](https://github.com/dmachard/coredns-gslb/releases)** to download the archive for your platform.

Each release includes assets for:

*   **Linux**: `coredns-gslb_linux_amd64.tar.gz`, `coredns-gslb_linux_arm64.tar.gz`
*   **macOS**: `coredns-gslb_darwin_amd64.tar.gz`, `coredns-gslb_darwin_arm64.tar.gz`
*   **Windows**: `coredns-gslb_windows_amd64.zip`

### Installation Steps (Linux)

1.  Download and extract the binary:
    ```bash
    curl -L -O https://github.com/dmachard/coredns-gslb/releases/download/v0.21.0/coredns-gslb_linux_amd64.tar.gz
    tar -zxvf coredns-gslb_linux_amd64.tar.gz
    ```
2.  Move the binary to your system path:
    ```bash
    sudo mv coredns /usr/local/bin/coredns-gslb
    ```
3.  Verify the installation:
    ```bash
    coredns-gslb -version
    ```

---

## 3. Compiling From Source

If you need to build CoreDNS with GSLB along with other third-party CoreDNS plugins, you can compile it yourself from source.

Refer to the **[Binary Compilation Guide](developer/compilation.md)** in the Developer resources for step-by-step instructions on setting up `plugin.cfg` and compiling the Go binary.
