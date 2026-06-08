# Build environment for the ze installer kernel.
#
# The build script downloads pinned kernel source, merges config fragments,
# and emits the kernel Image. Docker is faster than QEMU for cross-arch
# builds on macOS (Rosetta 2 vs TCG emulation).
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
    && rm -rf /var/lib/apt/lists/*

WORKDIR /build
