FROM ghcr.io/charmbracelet/vhs:v0.11.0@sha256:9d5fc3dc0c160b0fb1d2212baff07e6bdf3fa9438c504a3237484567302fcf93

USER root

RUN apt-get update \
    && apt-get install --yes --no-install-recommends chromium curl frr iproute2 iputils-ping keepalived netcat-openbsd nftables nodejs npm openssh-client procps python3 sshpass \
    && npm install --global playwright-core@1.55.0 \
    && npm cache clean --force \
    && mkdir -p /opt/ms-playwright/ffmpeg-1011 \
    && ln -s /usr/bin/ffmpeg /opt/ms-playwright/ffmpeg-1011/ffmpeg-linux \
    && rm -rf /var/lib/apt/lists/*
ENV NODE_PATH=/usr/local/lib/node_modules
ENV PLAYWRIGHT_BROWSERS_PATH=/opt/ms-playwright

COPY container-entrypoint.sh /usr/local/bin/terminal-demo-entrypoint

WORKDIR /src
ENTRYPOINT ["/usr/local/bin/terminal-demo-entrypoint"]
