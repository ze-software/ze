# Build environment for Ze runtime and installer kernels.
#
# The build script downloads pinned kernel source, merges config fragments,
# optionally applies Ze patches, and emits either installer Image artifacts or
# runtime vmlinuz/modules/DTBs. Docker is faster than QEMU for cross-arch builds
# on macOS when the requested platform is available.
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

WORKDIR /build
