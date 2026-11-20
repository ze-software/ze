# Phase 0 mechanism image. This intentionally builds only the af_packet/LCP/
# PPPoE lab path; it is not Ze's appliance or DPDK distribution.
FROM ubuntu:24.04 AS build

ARG VPP_COMMIT=cbba0451bb0af02a3ab8e163f6f99062782258e6
ARG VPP_SHA256=752e783ea81cd91882479944adbaf437f6c1f38db8b37a08c177a45e32c39e83

ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates clang cmake curl gcc git make ninja-build patch \
    libelf-dev libmnl-dev libnl-3-dev libnl-route-3-dev libnuma-dev \
    libpcap-dev libssl-dev libunwind-dev python3-dev python3-jsonschema \
    python3-pip python3-ply python3-setuptools python3-wheel python3-yaml \
    && rm -rf /var/lib/apt/lists/*

# One compile job per core of the Docker host, capped at one per 1.5 GiB of
# available memory and never below one: a two-GiB host builds serially, a
# larger lab guest uses its cores. Every compile step below reads it.
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
    && printf 'v25.10-0-g%s\n' "${VPP_COMMIT}" > src/scripts/.version

COPY *.patch /patches/
# Patch filenames carry their application order. The build fails on any
# rejected hunk; fuzz is forbidden so changed upstream context is visible.
RUN cat /patches/*.patch | patch --batch --forward --fuzz=0 -p1

RUN cmake -S src -B build -G Ninja \
    -DCMAKE_BUILD_TYPE=release -DCMAKE_INSTALL_PREFIX=/usr \
    -DCMAKE_INSTALL_LIBDIR=lib -DVPP_PLUGINS=af_packet,linux-cp,pppoe \
    -DVPP_PPPOE_FRAME_TESTS=ON -DVPP_BUILD_NATIVE_ONLY=ON \
    && cmake --build build --parallel "$(vpp-build-jobs)"

# Keep the executable's failure status and require its complete success output.
RUN crc_output="$(build/bin/test_crc32c_scalar 2>&1)" \
    || { crc_status=$?; printf '%s\n' "$crc_output" >&2; exit "$crc_status"; }; \
    printf '%s\n' "$crc_output"; \
    test "$crc_output" = 'VPP CRC32C scalar helpers: 29 cases passed'

# Lab API regressions follow the cached seven-patch compile. Reconfiguration
# reuses its configured options; the test patch does not change the producer.
COPY ze/ /patches/ze/
RUN cat /patches/ze/*.patch | patch --batch --forward --fuzz=0 -p1 \
    && cmake -S src -B build \
    && cmake --build build --parallel "$(vpp-build-jobs)"

COPY proof/ /proof/
ARG VPP_PROOF_VARIANT=baseline
RUN case "${VPP_PROOF_VARIANT}" in \
      baseline) ;; \
      missing-frame-metadata) \
        patch --batch --forward --fuzz=0 -p1 < /proof/missing-frame-metadata.diff \
        && test -s src/plugins/pppoe/pppoe_frame.h \
        && grep -Fx '  (void) out; // MUTATION-APPLIED: proof-only missing-frame-metadata variant.' src/plugins/pppoe/pppoe_frame.h \
        && cmake --build build --parallel "$(vpp-build-jobs)" ;; \
      *) printf 'Unsupported VPP proof variant: %s\n' "${VPP_PROOF_VARIANT}" >&2; exit 2 ;; \
    esac

RUN DESTDIR=/out cmake --install build \
    && mkdir -p /out/usr/share/doc/vpp \
    && printf '%s\n' "${VPP_PROOF_VARIANT}" > /out/usr/share/doc/vpp/proof-variant

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
