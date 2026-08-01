# 753: Docker Support and Go 1.26 Upgrade

## Context
Ze had no containerized deployment option. The build targeted Go 1.25. Docker support needed to target the `scratch` base image (no OS, just the binary) since ze is a statically linked Go binary with no system dependencies.

## Decision
Added `docker/Dockerfile` with a two-stage build: `golang:1.26-alpine` builder, `scratch` runtime. `compose.yaml` for local development with port mapping (179, 1790, 8080). `make ze-docker` target wraps `docker build` with version and build-date ldflags.

Key choices:
- **`scratch` over `alpine`**: ze has no shell commands, no libc deps, no filesystem expectations beyond its own blob storage. `scratch` eliminates the attack surface of an OS image.
- **`CGO_ENABLED=0`**: forces pure-Go net and crypto, avoiding glibc/musl divergence.
- **Build args for tags**: `ZE_TAGS` allows `linux` or `vpp` build tags without Dockerfile changes.
- **Go 1.26 upgrade**: bumped `go.mod`, CI `.woodpecker/verify.yml`, interop Dockerfiles, and docs. Single commit to avoid intermediate broken states.

## Consequences
- `docker build` produces a ~30MB image with just the ze binary.
- Docs guide covers bind-mount for config and blob storage.
- Interop test Dockerfiles also updated to 1.26 for consistency.

## Gotchas
- The `docs/guide/quickstart.md` Go version reference is easy to miss during upgrades; search for `go1.` across the whole tree.
- `scratch` has no shell, so `docker exec` debugging requires a multi-stage override or a separate debug image.

## Files

None recorded.
