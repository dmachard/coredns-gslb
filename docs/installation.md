# Installation

After downloading the CoreDNS-GSLB Docker image or binary, follow these steps to install and set up the service on your environment.

---

## Running with Docker

You can run the CoreDNS-GSLB container by mounting your `Corefile` and configuration files:

```bash
docker run -d \
  --name coredns-gslb \
  -p 53:53/udp \
  -p 53:53/tcp \
  -p 8080:8080 \
  -v /etc/coredns:/etc/coredns \
  dmachard/coredns_gslb:latest -conf /etc/coredns/Corefile
```

!!! note "Tip for compose-based installation"
    For a full, multi-file configuration setup using Docker Compose, check out the **[Quick Start Guide](getting_started.md)**.

---

## Installing the Binary (Linux & macOS)

If you downloaded the precompiled binary archive, extract and install it as follows:

### Step 1: Extract the Archive

Extract the downloaded archive (replace the name with your downloaded archive file):
```bash
tar -zxvf coredns-gslb_linux_amd64.tar.gz
```

### Step 2: Install the Binary

Move the extracted `coredns` binary to a directory in your system path (e.g., `/usr/local/bin`) and rename it to `coredns-gslb` for clarity:
```bash
sudo mv coredns /usr/local/bin/coredns-gslb
```

Make sure it has execution permissions:
```bash
sudo chmod +x /usr/local/bin/coredns-gslb
```

### Step 3: Verify the Installation

Check that the binary runs and displays the correct version:
```bash
coredns-gslb -version
```

---

## Running as a System Service (Systemd)

To run CoreDNS-GSLB as a background service on Linux, you can configure a Systemd service unit.

1.  Create a dedicated system user and group (optional but recommended):
    ```bash
    sudo groupadd --system coredns
    sudo useradd -s /sbin/nologin --system -g coredns coredns
    ```

2.  Create the configuration directory and place your `Corefile` and zone YAML files inside:
    ```bash
    sudo mkdir -p /etc/coredns
    sudo chown -R coredns:coredns /etc/coredns
    ```

3.  Create the service unit file at `/etc/systemd/system/coredns-gslb.service`:
    ```ini
    [Unit]
    Description=CoreDNS GSLB Service
    After=network.target

    [Service]
    PermissionsStartOnly=true
    LimitNOFILE=1048576
    LimitNPROC=512
    CapabilityBoundingSet=CAP_NET_BIND_SERVICE
    AmbientCapabilities=CAP_NET_BIND_SERVICE
    Type=simple
    User=coredns
    Group=coredns
    WorkingDirectory=/etc/coredns
    ExecStart=/usr/local/bin/coredns-gslb -conf /etc/coredns/Corefile
    Restart=on-failure

    [Install]
    WantedBy=multi-user.target
    ```

4.  Enable and start the service:
    ```bash
    sudo systemctl daemon-reload
    sudo systemctl enable --now coredns-gslb
    ```
