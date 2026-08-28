---
kind: note
level:
stage:
---
Two construction shapes keep compile-out features out of always-on code. **Listener services** such as looking-glass, web, and MCP register factories in `cmd/ze/hub/service_registry.go`; dedicated seams such as `ssh_infra.go`, `gnmi_infra.go`, `api_infra.go`, and the core metrics hook carry inputs that do not fit that registry. Each gated service keeps its direct package and YANG imports behind the matching `ze_<feature>` build tag. `feature-gates.txt` is the source of truth, `./le feature-tags write` updates static consumers, and `./le feature-tags check` refuses drift.
