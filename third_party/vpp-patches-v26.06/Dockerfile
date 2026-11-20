# The VPP image `./le test deployment vpp` boots: FD.io VPP v26.06 built from
# source with every plugin the installed libraries allow, plus Ze's IPsec
# patch. Built only by `./le build vpp deployment`; the deployment run loads it
# from the image cache and never builds it. Not Ze's appliance or DPDK package.
FROM ubuntu:24.04 AS build

ARG VPP_COMMIT=c3200b88dc46bd380f00a49ca3392a102cc1980b
ARG VPP_SHA256=ee226ca1336c69ebe8b1e14535bd09bcab3ab46fbf341a69fa57e16d340af305

ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates clang cmake curl gcc git make ninja-build patch \
    libelf-dev libmnl-dev libnl-3-dev libnl-route-3-dev libnuma-dev \
    libpcap-dev libssl-dev libunwind-dev python3-dev python3-jsonschema \
    python3-pip python3-ply python3-setuptools python3-wheel python3-yaml \
    && rm -rf /var/lib/apt/lists/*

# One compile job per core of the Docker host, capped at one per 1.5 GiB of
# available memory and never below one, as in ../vpp-patches/Dockerfile.
RUN printf '%s\n' '#!/bin/sh' \
    'awk -v cpus="$(nproc)" '"'"'/^MemAvailable:/ { j = int($2 / 1572864); if (j > cpus) j = cpus; if (j < 1) j = 1; print j }'"'"' /proc/meminfo' \
    > /usr/local/bin/vpp-build-jobs \
    && chmod 0755 /usr/local/bin/vpp-build-jobs \
    && vpp-build-jobs

WORKDIR /src/vpp
RUN curl -fSL "https://codeload.github.com/FDio/vpp/tar.gz/${VPP_COMMIT}" -o /src/vpp.tar.gz \
    && echo "${VPP_SHA256}  /src/vpp.tar.gz" | sha256sum -c - \
    && tar -xzf /src/vpp.tar.gz --strip-components=1 \
    && rm /src/vpp.tar.gz \
    && printf 'v26.06-0-g%s\n' "${VPP_COMMIT}" > src/scripts/.version

COPY *.patch /patches/
# Patch filenames carry their application order. The build fails on any
# rejected hunk; fuzz is forbidden so changed upstream context is visible.
RUN cat /patches/*.patch | patch --batch --forward --fuzz=0 -p1

RUN cmake -S src -B build -G Ninja \
    -DCMAKE_BUILD_TYPE=release -DCMAKE_INSTALL_PREFIX=/usr \
    -DCMAKE_INSTALL_LIBDIR=lib -DVPP_BUILD_NATIVE_ONLY=ON \
    && cmake --build build --parallel "$(vpp-build-jobs)"

RUN DESTDIR=/out cmake --install build

FROM ubuntu:24.04
ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update && apt-get install -y --no-install-recommends \
    iproute2 iputils-ping libelf1t64 libmnl0 libnl-3-200 libnl-route-3-200 \
    libnuma1 libpcap0.8t64 libssl3t64 libunwind8 tcpdump \
    && rm -rf /var/lib/apt/lists/*
COPY --from=build /out/ /
COPY LICENSE /usr/share/doc/vpp/LICENSE
RUN ldconfig
CMD ["vpp", "-c", "/run/vpp/startup.conf"]
