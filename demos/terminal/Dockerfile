# The image every demo is rendered in.
#
# It carries what the demos need and nothing more: the Python the tape recorder
# runs on (demos/terminal/pty-session.py), chromium with Playwright for the one
# `kind: browser` demo, ffmpeg for the video that demo records, the network
# tools the demo scenarios drive, and the three font families below.
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
FROM debian:trixie-slim@sha256:3a39a0592364683e6bab97937b72cad5a8fa6dcbbee90edb3bb48c7f8e94f258

RUN apt-get update \
    && apt-get install --yes --no-install-recommends chromium curl ffmpeg fontconfig frr iproute2 iputils-ping keepalived netcat-openbsd nftables nodejs npm openssh-client procps python3 sshpass \
    && npm install --global playwright-core@1.55.0 \
    && npm cache clean --force \
    && mkdir -p /opt/ms-playwright/ffmpeg-1011 \
    && ln -s /usr/bin/ffmpeg /opt/ms-playwright/ffmpeg-1011/ffmpeg-linux \
    && rm -rf /var/lib/apt/lists/*
ENV NODE_PATH=/usr/local/lib/node_modules
ENV PLAYWRIGHT_BROWSERS_PATH=/opt/ms-playwright

COPY fonts/fira-code /usr/share/fonts/fira-code
COPY fonts/liberation /usr/share/fonts/liberation
COPY fonts/noto /usr/share/fonts/noto
RUN find /usr/share/fonts -mindepth 1 -maxdepth 1 \
        ! -name fira-code ! -name liberation ! -name noto -exec rm -rf {} + \
    && fc-cache -f >/dev/null

COPY container-entrypoint.sh /usr/local/bin/terminal-demo-entrypoint

WORKDIR /src
ENTRYPOINT ["/usr/local/bin/terminal-demo-entrypoint"]
