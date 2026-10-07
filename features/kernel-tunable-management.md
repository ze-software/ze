# Kernel Tunable Management

## Meta

| Field | Value |
|-------|-------|
| Name | Kernel Tunable Management |
| Kind | daemon |
| Scope | complete |
| Level | experimental |
| Components | internal/component/sysctl, internal/core/sysctl |
| Real-path tests | test/parse/sysctl-config.ci, test/parse/sysctl-config-override.ci, test/plugin/sysctl-list.ci, test/plugin/sysctl-describe-show.ci |
| Docs | docs/guide/configuration.md |
| Doc review | 2026-10-07: anchors sysctl.go and profiles.go exist; the row's last source anchor is truncated at HEAD (unclosed comment), reported to the main thread |
| Defect review | 2026-10-07: no open spec or journal row found naming sysctl |
| Extra criteria | supported: privileged functional test of the three precedence layers and restore on clean stop = none yet |

## Description

Sysctl plugin centralizes kernel parameter management with three-layer precedence (config > transient > default). Plugins declare required defaults (e.g., fib-kernel enables forwarding), users override via config or CLI. Original values restored on clean stop. Named profiles group co-dependent tunables (dsr, router, hardened, multihomed, proxy) applied per interface unit. User-defined profiles supported. CLI: `show sysctl`, `ze sysctl list`, `ze sysctl describe`, `set sysctl`, `ze sysctl list-profiles`, `ze sysctl describe-profile`. <!-- source: internal/component/sysctl/sysctl.go -- store, setDefault, setTransient, applyConfig --> <!-- source: internal/core/sysctl/profiles.go -- ProfileDef, MustRegisterProfile, builtinProfiles…
