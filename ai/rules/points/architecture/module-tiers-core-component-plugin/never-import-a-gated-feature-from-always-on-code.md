---
kind: directive
level: MUST NOT
stage:
---
**Rule:** a compile-out-able feature (gated by `//go:build ze_<feature>`) MUST NOT be directly imported by always-on code. Reach it through the construction registry or a seam (`ssh_infra.go` / `gnmi_infra.go` style) in another gated file. `dep_audit.py` enumerates these gated packages (`DISABLEABLE`); the gate flags any always-on, non-test importer. Gates are declared in ONE place: `<tag> <pkg>` lines in the repo-root `feature-gates.txt` manifest. A feature may reuse one tag for sidecar packages that must vanish with it. `ZE_FEATURES` (Makefile), `TestBuildTags()` (`internal/test/runner`), `featureTags` (`plugin_imports.go`), and `DISABLEABLE` (`dep_audit.py`) all DERIVE from it; only `.golangci.yml` build-tags is edited by hand (static YAML), and `dep_audit.py --check` fails on its drift. Full procedure and the two registration shapes: `ai/rules/plugins.md`.
