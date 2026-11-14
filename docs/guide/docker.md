# Docker

Run Ze in Docker for BGP evaluation or a daemon deployment where the host owns
the network interfaces. This guide starts from the repository root and uses the
supplied Compose file. Docker and its Compose plugin must already be installed.
<!-- source: docker/compose.yaml -- build context, service and persistent volume -->

Ze is pre-release software. For an immutable VM or hardware image, see
[VM Appliance](appliance.md). For a native Linux service, see
[Linux installation](ubuntu-build-install.md). To compare every route, see
[Choose a deployment path](ze-install.md#choose-a-deployment-path).

## Two images

Ze builds two container images. They carry the same daemon features and differ in the base.

| Local image | Recipe | Target | Base |
|-------------|--------|--------|------|
| `ze:local` | `docker/Dockerfile` | Deployment | `scratch`: no shell, no libc, no package manager |
| `netlab/ze:latest` | `docker/Dockerfile.lab` | Labs under netlab and containerlab | `alpine:3.24.3` with `tini` and `iproute2` |

These are local build tags, not public registry downloads. Use the deployment
image for a daemon and the lab image with netlab/containerlab. Lab tools execute
`sh` and `ip` inside nodes to assign topology addresses; the scratch image has
neither. For the lab image, see [netlab](netlab.md).
<!-- source: docker/Dockerfile -- FROM scratch, ENTRYPOINT ["/ze"] -->
<!-- source: docker/Dockerfile.lab -- FROM alpine:3.24.3, apk add tini iproute2, ENTRYPOINT and CMD -->
<!-- source: docker/Dockerfile -- default feature-tag derivation and deployment image -->

The [preview artifact builder](../contributing/preview-artifacts.md) instead
packages its inspected native binary through `docker/Dockerfile.runtime`.
Those prepared images carry the binary's exact tags, including `ze_distro`;
the source-building commands below keep their own feature selection.
<!-- source: docker/Dockerfile.runtime -- deployment and lab -->
<!-- source: internal/le/build/preview/images.go -- prepareImages, exportImage -->

Both images include system CA certificates for outbound HTTPS requests,
including `ze init --from https://...`.
<!-- source: docker/Dockerfile -- runtime certificate bundle -->
<!-- source: docker/Dockerfile.lab -- ca-certificates package -->
<!-- source: internal/core/fetch/fetch.go -- metadataClient, doToFile -->

Both deployment recipes also provide an empty `/tmp` with mode `1777` for
temporary files during provisioning and configuration reload.
<!-- source: docker/Dockerfile -- runtime-root temporary directory -->
<!-- source: docker/Dockerfile.runtime -- deployment temporary directory -->

## Build the deployment image

```bash
docker build -t ze:local -f docker/Dockerfile .
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
<!-- source: docker/Dockerfile -- feature_tags and ZE_TAGS -->

## Build the lab image

```bash
docker build -t netlab/ze:latest -f docker/Dockerfile.lab .
```

This produces `netlab/ze:latest`. The image runs `ze` under `tini`, which
forwards SIGHUP and SIGTERM. See [netlab](netlab.md).
<!-- source: docker/Dockerfile.lab -- image contents and entrypoint -->

## Initialize persistent state

Use the supplied Compose file in place, from the repository root. Its build
context is the repository; copying the file elsewhere changes that context.
Initialization, offline configuration commands and the daemon all mount the same
named volume at `/etc/ze` and use the same `ZE_CONFIG_DIR`.
<!-- source: docker/compose.yaml -- build.context, environment and ze-data volume -->

```bash
docker compose -f docker/compose.yaml build
docker compose -f docker/compose.yaml run --rm ze init
```

Choose a username and strong password at the prompt. Keep the default host
`127.0.0.1`, port `2222` and instance name `ze`; the Compose hostname makes that
name stable. There is no password in the image or Compose file. Initialization
creates a private store in the volume and refuses to replace an existing store.
Run it once, before first start.
<!-- source: docker/compose.yaml -- hostname and persistent store -->
<!-- source: internal/plugins/init/main.go -- initialization prompts and replacement refusal -->

For this newly initialized volume, import the supplied management configuration
in place of the `ze.conf` seed that initialization generated. `--yes` confirms
replacement of that seed; the initialized credential store persists. The
configuration enables SSH without taking over host interfaces. The input is
streamed into the image; no shell is needed inside it.
<!-- source: docker/ze.conf -- management listener -->
<!-- source: internal/component/config/cli/cmd_import.go -- configuration import from stdin -->

```bash
docker compose -f docker/compose.yaml run --rm -T ze config import --yes --name ze.conf - < docker/ze.conf
docker compose -f docker/compose.yaml up -d
```

## Verify and configure

```bash
docker compose -f docker/compose.yaml logs ze
docker compose -f docker/compose.yaml exec ze /ze status
docker compose -f docker/compose.yaml exec ze /ze cli -c "show bgp peer list"
ssh -p 2222 YOUR_USERNAME@127.0.0.1
```

The initial configuration enables management and defines no BGP peer. Add a
peer through the SSH editor using [BGP peering](bgp-peering.md), commit the
change, and check the peer and received routes. For a complete two-peer example,
start with the [local quickstart](quickstart.md) or [netlab](netlab.md).
<!-- source: docker/ze.conf -- management-only initial config -->
<!-- source: internal/component/bgp/yang/ze-bgp-conf.yang -- peer configuration -->

Compose publishes SSH on the host's loopback port `2222`, and BGP on port `179`.
To allow remote operator access, publish SSH on your management address and
apply the host's access policy. Web/API listeners need their own explicit
configuration and port publication. Exposing a Docker port does not enable a
service in Ze.
<!-- source: docker/compose.yaml -- published ports -->
<!-- source: docker/ze.conf -- enabled SSH service -->

## Persistence, updates and removal

Configuration commits and credentials live in the named volume. Recreating the
container keeps them. Bare `start` reads the active config from that store, so
there is no bind-mounted file competing with the committed configuration.
<!-- source: docker/compose.yaml -- bare start and named volume -->
<!-- source: cmd/ze/ze_core_start.go -- stored active configuration -->

```bash
# Recreate without discarding the store, then verify your configuration.
docker compose -f docker/compose.yaml up -d --force-recreate
docker compose -f docker/compose.yaml exec ze /ze cli -c "show bgp peer list"

# Stop and remove the container; retain its volume.
docker compose -f docker/compose.yaml down
```

Before an update, export or back up the configuration using the
[lifecycle guide](lifecycle.md), record the running image ID, and retain the
previous image. Build a new image from the selected source revision, recreate
the container during a maintenance window, and verify login, peers and routes.
Roll back by selecting the retained image and restoring a compatible config if
the syntax changed. Do not use the daemon's binary self-update inside a container.
<!-- source: docker/compose.yaml -- image and build configuration -->
<!-- source: docker/Dockerfile -- immutable image executable -->

`docker compose -f docker/compose.yaml down --volumes` also deletes this Compose
project's named state volume. Use it only to discard the installation, after
exporting anything you need. Ordinary `down` keeps the volume.
<!-- source: docker/compose.yaml -- project-owned named volume -->

## Run without Compose

Use the same image entrypoint and explicit config-directory mount in plain
Docker. This first-start sequence uses a new volume and replaces only the seed
configuration generated by initialization, keeping its credential store:

```bash
docker volume create ze-data
docker run --rm -it --hostname ze -e ZE_CONFIG_DIR=/etc/ze -v ze-data:/etc/ze ze:local init
docker run --rm -i -e ZE_CONFIG_DIR=/etc/ze -v ze-data:/etc/ze ze:local config import --yes --name ze.conf - < docker/ze.conf
docker run -d --name ze --hostname ze --restart unless-stopped \
  --ulimit memlock=-1 -e ZE_CONFIG_DIR=/etc/ze -v ze-data:/etc/ze \
  -p 179:179 -p 127.0.0.1:2222:2222 ze:local start
```

Everything after the image name is a Ze command because the entrypoint is `/ze`.
An explicit configuration path belongs after `start`. When using that form,
mount a writable directory containing both the file and its store; a single
read-only file mount prevents the editor from saving commits back to the file.
<!-- source: docker/Dockerfile -- ENTRYPOINT -->
<!-- source: cmd/ze/ze_core_start.go -- explicit-file start -->
<!-- source: cmd/ze/hub/config_source.go -- explicit configuration writeback -->

## Limitations

The deployment container runs on a scratch base. Features that require kernel access do not work without extra privileges. The same capability table applies to the lab image:

| Feature | Requirement |
|---------|-------------|
| Interface configuration | `--cap-add NET_ADMIN` or `--privileged` |
| VPP data plane | Not supported in containers (use [VM Appliance](appliance.md)) |
| L2TP tunnels | `--cap-add NET_ADMIN` + host networking |
| nftables / firewall | `--cap-add NET_ADMIN` + `--cap-add NET_RAW` |
| Binding port 179 | Works by default (container runs as root) |
| Locking the executable in memory (`memlock` plugin) | `--ulimit memlock=-1` or `--cap-add IPC_LOCK`. Docker grants neither by default, so the lock fails, `ze doctor` warns under `doctor-memlock-rlimit-low`, and `show plugin list` reports the `memlock` row as `soft-failure` |

For BGP peering without interface management (route server, looking glass, policy testing), no extra capabilities are needed.

## Troubleshooting

**Container exits immediately:** inspect `docker compose -f docker/compose.yaml logs ze`.
Bare `start` needs the initialized store and imported active configuration. A bare
config path without the `start` keyword is not a daemon command.
<!-- source: cmd/ze/ze_core_start.go -- startConfigPath -->

**Cannot connect to CLI:** verify initialization, the imported SSH listener, and
the `127.0.0.1:2222:2222` publication. Use the username and password you chose.
The scratch image has no shell: use `/ze` directly with `docker exec`.
<!-- source: docker/compose.yaml -- management port publication -->
<!-- source: docker/Dockerfile -- scratch runtime -->

**Peer won't connect:** If peering with the host, use `--network host` or the Docker bridge gateway IP. Container-to-container peering works on a shared Docker network.
