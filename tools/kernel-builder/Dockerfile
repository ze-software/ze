# Build environment for Ze runtime and installer kernels.
FROM golang:1.27-bookworm AS builder

WORKDIR /src
COPY go.mod go.sum ./
COPY internal/appliance/kernelbuilder ./internal/appliance/kernelbuilder
COPY tools/kernel-builder/main.go ./tools/kernel-builder/main.go
RUN CGO_ENABLED=0 go build -o /ze-kernel-builder ./tools/kernel-builder

FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
        build-essential \
        bc \
        bison \
        flex \
        libelf-dev \
        libssl-dev \
        linux-headers-generic \
        wget \
        ca-certificates \
        xz-utils \
        cpio \
        kmod \
        zstd \
        patch \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder /ze-kernel-builder /usr/local/bin/ze-kernel-builder
WORKDIR /build
