# Build environment for Ze runtime and installer kernels.
#
# python3 is a KERNEL build dependency, not a Ze one. Several in-tree header
# generators are Python scripts, and the arm64 runtime build reaches one of them
# through drivers/gpu/drm/msm, whose Makefile runs gen_header.py. Without it the
# build dies at GENHDR with "/bin/sh: 1: python3: not found", after twenty
# minutes of compiling. The amd64 build never selects that driver, which is why
# the nightly stayed green while an arm64 build could not finish.
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
        python3 \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder /ze-kernel-builder /usr/local/bin/ze-kernel-builder
WORKDIR /build
