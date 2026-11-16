# Containers

## Meta

| Field | Value |
|-------|-------|
| Name | Containers |
| Page | docs/architecture/container/runtime.md |
| Kind | daemon |
| Scope | future |
| Docs | docs/architecture/container/runtime.md |

## Description

Ze runs OCI container images itself, confined and supervised, through the upstream runtime library behind a patch series. What exists, in a build with `ze_container`: the `container` plugin with its config root (`container instance <name>` and its `enabled` leaf), the command `request container restart name <name>`, the container init role inside the `ze` binary, a start path (description, root overlay, create, release, watch) that no test has yet proven on a real kernel, the plugin's doctor codes and runtime doctor checks, and the kernel requirement for a hard CPU limit (`CONFIG_CFS_BANDWIDTH`). No shipped build holds an image store, so no shipped build starts a container. <!-- source: internal/plugins/container/register.go -- registerContainer --> <!-- source: internal/plugins/container/supervise.go -- supervisor.startLocked, noImageStore --> <!-- source: internal/plugins/container/kernelcap_linux.go -- containerKernelCapabilities -->
