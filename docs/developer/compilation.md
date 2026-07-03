# Binary Compilation

To run CoreDNS-GSLB, you must build a custom CoreDNS binary that compiles the `gslb` plugin.

## Step 1: Update plugin configuration

1. Clone or download the CoreDNS source code.
2. Edit the file `plugin.cfg` inside the CoreDNS source directory.
3. Register the `gslb` plugin by adding the following line. It is recommended to place it right before `file:file` to ensure proper routing priority:

```text
gslb:github.com/dmachard/coredns-gslb
```

## Step 2: Compile the Binary

Run code generation and compilation commands:

```bash
go generate
make
```

This compiles a `coredns` executable with the GSLB plugin compiled in. You can run and configure it using standard Corefiles.
