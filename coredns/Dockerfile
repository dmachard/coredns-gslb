FROM golang:1.23 AS builder

# Set the GOPATH and create directories for CoreDNS and the GSLB plugin
WORKDIR /go/src

# Copy the local GSLB plugin to the builder environment
COPY . /go/src/gslb/

# Build CoreDNS with the GSLB plugin
ARG COREDNS_VERSION=v1.12.2
RUN git clone https://github.com/coredns/coredns.git /coredns && \
    cd /coredns && \
    git checkout $COREDNS_VERSION && \
    sed -i '/file:file/i gslb:github.com/dmachard/coredns-gslb' plugin.cfg && \
    go mod edit -replace github.com/dmachard/coredns-gslb=/go/src/gslb && \
    go get github.com/dmachard/coredns-gslb && \
    go generate && \
    make

# Create the final image with CoreDNS binary and necessary files
FROM debian:bullseye

COPY --from=builder /coredns/coredns /usr/bin/coredns

RUN apt-get update && apt-get upgrade -y
WORKDIR /

ENTRYPOINT ["/usr/bin/coredns"]
