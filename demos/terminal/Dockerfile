# The image every demo is rendered in.
#
# It carries what the demos need and nothing more: Chromium with Playwright for
# the browser screenshot, the network tools the scenarios drive, and the three
# font families below.
#
# The vendored fonts are load-bearing, and they are the ONLY fonts the image
# holds: the copy below removes every font a Debian package installed, so what
# the browser demo paints is decided by bytes this repository carries. Three
# families are needed, and the third is the one that is easy to miss. The Ze
# web interface names Fira Code in `--font-mono` and Noto Sans in `--font-ui`
# (internal/component/web/assets/style.css), and its form controls name no
# family at all, so chromium draws them in its default face: Arial, which
# fontconfig's metric aliases resolve to Liberation Sans. Drop Liberation and
# every input and button changes size. Provenance and licenses:
# demos/terminal/fonts/README.md.
FROM debian:trixie-slim@sha256:3a39a0592364683e6bab97937b72cad5a8fa6dcbbee90edb3bb48c7f8e94f258 AS debian

# Build against the runtime's glibc, never the Alpine interop image's musl.
# Engine.BuildImage supplies the canonical interop Dockerfile's release and hash.
FROM debian AS keepalived-build
RUN apt-get update \
    && apt-get install --yes --no-install-recommends build-essential ca-certificates curl libnl-3-dev libnl-genl-3-dev libssl-dev pkg-config \
    && rm -rf /var/lib/apt/lists/*
ARG KEEPALIVED_VERSION
ARG KEEPALIVED_SHA256
WORKDIR /build
RUN test -n "${KEEPALIVED_VERSION}" && test -n "${KEEPALIVED_SHA256}" \
    && curl -fsSL -o keepalived.tar.gz \
        "https://www.keepalived.org/software/keepalived-${KEEPALIVED_VERSION}.tar.gz" \
    && echo "${KEEPALIVED_SHA256}  keepalived.tar.gz" | sha256sum -c - \
    && tar -xzf keepalived.tar.gz --strip-components=1 \
    && ./configure --prefix=/usr --sysconfdir=/etc --disable-lvs --enable-vrrp --enable-vmac \
    && make -j2 \
    && make install DESTDIR=/out \
    && rm -rf /out/usr/share

FROM debian

RUN apt-get update \
    && apt-get install --yes --no-install-recommends ca-certificates chromium curl fontconfig iproute2 iputils-ping libnl-3-200 libnl-genl-3-200 libssl3t64 netcat-openbsd nftables nodejs npm openssh-client procps sshpass \
    && npm install --global playwright-core@1.55.0 \
    && npm cache clean --force \
    && rm -rf /var/lib/apt/lists/*
ENV NODE_PATH=/usr/local/lib/node_modules

COPY fonts/fira-code /usr/share/fonts/fira-code
COPY fonts/liberation /usr/share/fonts/liberation
COPY fonts/noto /usr/share/fonts/noto
RUN find /usr/share/fonts -mindepth 1 -maxdepth 1 \
        ! -name fira-code ! -name liberation ! -name noto -exec rm -rf {} + \
    && fc-cache -f >/dev/null

# Only the upstream repository supplies FRR; the release comes from FRRImage.
ARG FRR_VERSION
RUN test -n "${FRR_VERSION}" \
    && curl -fsSL -o /usr/share/keyrings/frrouting.gpg https://deb.frrouting.org/frr/keys.gpg \
    && echo 'deb [signed-by=/usr/share/keyrings/frrouting.gpg] https://deb.frrouting.org/frr trixie frr-stable' > /etc/apt/sources.list.d/frr.list \
    && printf 'Package: frr\nPin: origin deb.frrouting.org\nPin-Priority: 1001\n' > /etc/apt/preferences.d/frr \
    && apt-get update \
    && apt-get install --yes --no-install-recommends "frr=${FRR_VERSION}-0~deb13u1" \
    && rm -rf /var/lib/apt/lists/*

COPY --from=keepalived-build /out/ /
ARG KEEPALIVED_VERSION
ARG KEEPALIVED_SHA256
LABEL ze.renderer.frr="${FRR_VERSION}" \
      ze.renderer.keepalived="${KEEPALIVED_VERSION}" \
      ze.renderer.keepalived.sha256="${KEEPALIVED_SHA256}"

WORKDIR /src
ENTRYPOINT ["/src/tmp/terminal-demos/bin/ze-demo", "entrypoint"]
