# Docker

Run Ze in a Docker container for evaluation, lab testing, or lightweight deployments where you don't need interface configuration or kernel-level features (VPP, L2TP, nftables).

For production on bare metal or a dedicated VM, see [VM Appliance](appliance.md).

## Build the image

```bash
make ze-docker
```

This produces `ze:<YY.MM.DD>` and `ze:latest`. Override the image name or tag:

```bash
make ze-docker ZE_DOCKER_IMAGE=myregistry/ze ZE_DOCKER_TAG=v1
```

To include optional build tags (e.g. `maprib`):

```bash
make ze-docker ZE_TAGS=maprib
```

The image is ~89 MB: a static binary on a scratch base with no shell, no libc, no package manager.

## Run

Ze needs a config file. Mount one from the host:

```bash
docker run --rm -v ./example.conf:/etc/ze/ze.conf ze:latest /etc/ze/ze.conf
```

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
  ze:latest /etc/ze/ze.conf
```

## Initialize credentials

Ze requires SSH credentials for CLI access. Initialize them before starting the daemon:

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
  ze:latest /etc/ze/ze.conf
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

The container runs on a scratch base. Features that require kernel access do not work without extra privileges:

| Feature | Requirement |
|---------|-------------|
| Interface configuration | `--cap-add NET_ADMIN` or `--privileged` |
| VPP data plane | Not supported in containers (use [VM Appliance](appliance.md)) |
| L2TP tunnels | `--cap-add NET_ADMIN` + host networking |
| nftables / firewall | `--cap-add NET_ADMIN` + `--cap-add NET_RAW` |
| Binding port 179 | Works by default (container runs as root) |

For BGP peering without interface management (route server, looking glass, policy testing), no extra capabilities are needed.

## Troubleshooting

**Container exits immediately:** Ze needs a config file argument. Check `docker logs ze`.

**Cannot connect to CLI:** Make sure you ran `ze init` first (see above) and that port 1790 is published.

**Peer won't connect:** If peering with the host, use `--network host` or the Docker bridge gateway IP. Container-to-container peering works on a shared Docker network.
