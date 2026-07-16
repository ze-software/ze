FROM ghcr.io/charmbracelet/vhs:v0.11.0@sha256:9d5fc3dc0c160b0fb1d2212baff07e6bdf3fa9438c504a3237484567302fcf93

USER root

RUN apt-get update \
    && apt-get install --yes --no-install-recommends iproute2 iputils-ping openssh-client procps sshpass \
    && rm -rf /var/lib/apt/lists/*

COPY container-entrypoint.sh /usr/local/bin/terminal-demo-entrypoint

WORKDIR /src
ENTRYPOINT ["/usr/local/bin/terminal-demo-entrypoint"]
