---
kind: directive
level:
stage:
---
**Seam (ssh, gNMI).** Use a seam when the listener registry genuinely cannot express the construction shape. ssh is built inside shared daemon startup, interleaved with always-on AAA/authz/accounting, and owns an interactive session, so it uses `ssh_infra.go` (`sshBuild` / `sshWirePostStart` / `sshBuildStandalone`). gNMI has richer constructor dependencies, a reload notification hook, and no listener live-migration contract, so it uses `gnmi_infra.go` (`gnmiBuild` / `gnmiReloadNotify`). Always-on code calls the seam if non-nil; with the tag off the vars stay nil and the feature is skipped. Use a seam ONLY when the registry genuinely does not fit; prefer the registry.
