# Docker Support

## Meta

| Field | Value |
|-------|-------|
| Name | Docker Support |
| Page | docs/guide/docker.md |
| Kind | packaging |
| Scope | complete |
| Level | experimental |
| Components | docker |
| Docs | docs/guide/docker.md |
| Doc review | 2026-10-07: Dockerfile is FROM scratch with ENTRYPOINT /ze, Dockerfile.lab is FROM alpine:3.21 with tini and iproute2; the 119 MB and 137 MB sizes were not measured |
| Defect review | 2026-10-07: no open spec or journal row found naming the Docker images |
| Extra criteria | supported: a gate that builds both images and runs ze = none yet; supported: measured image sizes = none yet |

## Description

Two images, one binary. `docker build -t ze:latest -f docker/Dockerfile .` builds the deployment image: a static binary on a scratch base, 119 MB, `ENTRYPOINT /ze`. `docker build -t netlab/ze:latest -f docker/Dockerfile.lab .` builds the lab image: `alpine:3.21` with `tini` and `iproute2`, 137 MB, because lab tools run `sh` and `ip` inside a node. Both Dockerfiles derive their build tags from `feature-gates.txt`. `ZE_TAGS` adds tags to that set. Compose support is in `docker/compose.yaml`. <!-- source: docker/Dockerfile -- FROM scratch, feature tags from feature-gates.txt --> <!-- source: docker/Dockerfile.lab -- FROM alpine:3.21, tini, iproute2 -->
