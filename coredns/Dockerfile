FROM golang:1.23-bookworm AS builder

# Set the GOPATH and create directories for CoreDNS and the GSLB plugin
WORKDIR /go/src

# Copy the local GSLB plugin to the builder environment
COPY . /go/src/gslb/

# Build CoreDNS with the GSLB plugin
ARG COREDNS_VERSION=v1.12.2
ARG GSLB_VERSION=dev
RUN git clone https://github.com/coredns/coredns.git /coredns && \
    cd /coredns && \
    git checkout $COREDNS_VERSION && \
    sed -i '/file:file/i gslb:github.com/dmachard/coredns-gslb' plugin.cfg && \
    go mod edit -replace github.com/dmachard/coredns-gslb=/go/src/gslb && \
    go get github.com/dmachard/coredns-gslb && \
    go generate && \
    CGO_ENABLED=0 go build -ldflags "-X github.com/dmachard/coredns-gslb.Version=$GSLB_VERSION" -o /coredns/coredns .

# Create the final image with CoreDNS binary and necessary files
FROM debian:bookworm

COPY --from=builder /coredns/coredns /usr/bin/coredns

RUN apt-get update && apt-get upgrade -y
WORKDIR /

ENTRYPOINT ["/usr/bin/coredns"]
