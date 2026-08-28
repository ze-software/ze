---
kind: directive
level: MUST
stage:
---
**Rule:** a compile-out-able feature (gated by `//go:build ze_<feature>`) MUST NOT be directly imported by always-on code. Reach it through the construction registry or a seam (`ssh_infra.go` / `gnmi_infra.go` style) in another gated file. Gates are declared in ONE place as `<tag> <pkg>` rows in `feature-gates.txt`. A feature MAY reuse one tag for sidecar packages that MUST vanish with it. `./le tier check` derives the disable-able package set and refuses every always-on, non-test importer. `./le feature-tags check` refuses drift in the generated static consumers. Full procedure and the two registration shapes: `ai/rules/plugins.md`.
