FROM golang:1.26-bookworm AS builder

# Set the GOPATH and create directories for CoreDNS and the GSLB plugin
WORKDIR /go/src

# Copy the local GSLB plugin to the builder environment
COPY . /go/src/gslb/

# Build CoreDNS with the GSLB plugin
ARG COREDNS_VERSION=v1.14.4
ARG GSLB_VERSION=dev
RUN mkdir -p /coredns && \
    wget -qO- https://github.com/coredns/coredns/archive/refs/tags/${COREDNS_VERSION}.tar.gz | tar -xz -C /coredns --strip-components=1 && \
    cd /coredns && \
    sed -i '/file:file/i gslb:github.com/dmachard/coredns-gslb' plugin.cfg && \
    go mod edit -replace github.com/dmachard/coredns-gslb=/go/src/gslb && \
    go get github.com/dmachard/coredns-gslb && \
    go generate && \
    CGO_ENABLED=0 go build -ldflags "-X github.com/dmachard/coredns-gslb.Version=$GSLB_VERSION" -o /coredns/coredns .

# Build the CLI gslbctl
RUN cd /go/src/gslb && go build -o /gslbctl ./cli

# Create the final image with CoreDNS binary and necessary files
FROM debian:bookworm

COPY --from=builder /coredns/coredns /usr/bin/coredns

RUN apt-get update && apt-get upgrade -y
WORKDIR /

COPY --from=builder /gslbctl /usr/local/bin/gslbctl

ENTRYPOINT ["/usr/bin/coredns"]
