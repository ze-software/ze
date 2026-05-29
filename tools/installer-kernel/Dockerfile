# Build environment for the ze installer kernel.
#
# Native build: run on a host whose docker arch matches ZE_INSTALL_ARCH
# (arm64 on Apple Silicon / colima). The build script downloads pinned kernel
# source, merges kernel.config onto defconfig, and emits the kernel Image.
FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
        build-essential \
        bc \
        bison \
        flex \
        libelf-dev \
        libssl-dev \
        wget \
        ca-certificates \
        xz-utils \
        cpio \
        kmod \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /build
