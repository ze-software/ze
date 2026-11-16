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

Ze runs OCI container images itself, confined and supervised, through the upstream runtime library behind a patch series. No build starts a container today: a configuration that enables one is refused, with the statement that disables it. What exists is the wiring, in a build with `ze_container`: the `container` plugin with its config root (`container instance <name>` and its `enabled` leaf), the command `request container restart name <name>`, the container init role inside the `ze` binary, the plugin's doctor codes and runtime doctor checks, and the kernel requirement for a hard CPU limit (`CONFIG_CFS_BANDWIDTH`). <!-- source: internal/plugins/container/register.go -- registerContainer --> <!-- source: internal/plugins/container/config.go -- refuseUnbuiltRuntime --> <!-- source: internal/plugins/container/kernelcap_linux.go -- containerKernelCapabilities -->
