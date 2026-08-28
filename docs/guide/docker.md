# Docker

Run Ze in a Docker container for evaluation, lab testing, or lightweight deployments where you do not need interface configuration or kernel-level features (VPP, L2TP, nftables).

For production on bare metal or a dedicated VM, see [VM Appliance](appliance.md).

## Two images

Ze builds two container images. They carry the same binary and differ in the base.

| Image | Recipe | Target | Base | Size |
|-------|--------|--------|------|------|
| `ze:latest` | `docker/Dockerfile` | Deployment | `scratch`: no shell, no libc, no package manager | 119 MB |
| `netlab/ze:latest` | `docker/Dockerfile.lab` | Labs under netlab and containerlab | `alpine:3.21` with `tini` and `iproute2` | 137 MB |

The sizes are `docker image ls` values measured on 2026-08-14. Use the deployment image everywhere except a lab. The lab image carries a shell and `ip` because containerlab and netlab drive a node from the outside. They run `sh` and `ip` inside the container to assign the addresses the topology declares, and a scratch base has neither. For the lab image see [netlab](netlab.md).
<!-- source: docker/Dockerfile -- FROM scratch, ENTRYPOINT ["/ze"] -->
<!-- source: docker/Dockerfile.lab -- FROM alpine:3.21, apk add tini iproute2, ENTRYPOINT ["tini", "--", "ze"] -->
<!-- source: docker/Dockerfile -- default feature-tag derivation and deployment image -->

## Build the deployment image

```bash
docker build -t ze:latest -f docker/Dockerfile .
```

Choose another repository or tag with the normal Docker tag argument:

```bash
docker build -t myregistry/ze:v1 -f docker/Dockerfile .
```

The Dockerfile derives the default tag set from `feature-gates.txt`.
`ZE_TAGS` adds tags on top of that set:

```bash
docker build --build-arg ZE_TAGS=maprib -t ze:maprib -f docker/Dockerfile .
```

That command builds the default feature set plus `maprib`.
<!-- source: docker/Dockerfile -- ZE_FEATURES awk over feature-gates.txt, -tags "ze_core $ZE_FEATURES $ZE_TAGS" -->

## Build the lab image

```bash
docker build -t netlab/ze:latest -f docker/Dockerfile.lab .
```

This produces `netlab/ze:latest`. The image runs `ze` under `tini`, which
forwards SIGHUP and SIGTERM. See [netlab](netlab.md).
<!-- source: docker/Dockerfile.lab -- image contents and entrypoint -->

## Run

Ze needs a config file. Mount one from the host:

```bash
docker run --rm -v ./example.conf:/etc/ze/ze.conf ze:latest start /etc/ze/ze.conf
```

The deployment image `ENTRYPOINT` is `/ze`, so everything after the image name is passed to `ze`. The config path goes behind the `start` keyword: a bare `ze /etc/ze/ze.conf` is rejected with `unknown command`. The lab image keeps the same grammar behind `tini`.

<!-- source: docker/Dockerfile — ENTRYPOINT; cmd/ze/ze_core_dispatch.go — zeDispatch, "start" root handler -->


Expose the ports you need:

| Port | Service |
|------|---------|
| 179 | BGP |
| 1790 | SSH CLI |
| 8080 | Web UI / API |

```bash
docker run -d \
  --name ze \
  -p 179:179 \
  -p 1790:1790 \
  -p 8080:8080 \
  -v ./myconfig.conf:/etc/ze/ze.conf \
  -v ./ze-data:/etc/ze \
  ze:latest start /etc/ze/ze.conf
```

## Initialize credentials

Ze requires SSH credentials for CLI access. Initialize them before you start the daemon:

```bash
docker run --rm -v ./ze-data:/etc/ze ze:latest init
```

This prompts for username and password. For scripting:

```bash
echo -e "admin\nsecret" | docker run --rm -i -v ./ze-data:/etc/ze ze:latest init
```

Then start the daemon with the same volume:

```bash
docker run -d \
  --name ze \
  -p 179:179 \
  -p 1790:1790 \
  -p 8080:8080 \
  -v ./ze-data:/etc/ze \
  -v ./myconfig.conf:/etc/ze/ze.conf \
  ze:latest start /etc/ze/ze.conf
```

## Compose

A ready-made `docker/compose.yaml` is included in the repo. Copy it and edit to taste:

```bash
cp docker/compose.yaml .
# edit volumes to point at your config and data directory
docker compose up -d
```

The compose file builds the image from source using `docker/Dockerfile`.

## Limitations

The deployment container runs on a scratch base. Features that require kernel access do not work without extra privileges. The same capability table applies to the lab image:

| Feature | Requirement |
|---------|-------------|
| Interface configuration | `--cap-add NET_ADMIN` or `--privileged` |
| VPP data plane | Not supported in containers (use [VM Appliance](appliance.md)) |
| L2TP tunnels | `--cap-add NET_ADMIN` + host networking |
| nftables / firewall | `--cap-add NET_ADMIN` + `--cap-add NET_RAW` |
| Binding port 179 | Works by default (container runs as root) |

For BGP peering without interface management (route server, looking glass, policy testing), no extra capabilities are needed.

## Troubleshooting

**Container exits immediately:** Ze needs `start` plus a config path. A bare path (`ze:latest /etc/ze/ze.conf`) exits 1 with `unknown command: /etc/ze/ze.conf`. Check `docker logs ze`.

**Cannot connect to CLI:** Make sure you ran `ze init` first (see above) and that port 1790 is published.

**Peer won't connect:** If peering with the host, use `--network host` or the Docker bridge gateway IP. Container-to-container peering works on a shared Docker network.
