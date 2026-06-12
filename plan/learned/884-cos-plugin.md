# 884 -- cos-plugin

## Context

Interface VLAN QoS maps (ingress-qos-map / egress-qos-map) required repetitive inline configuration on every unit. BNG deployments with hundreds of VLAN subscribers sharing the same 802.1p mapping needed a named profile abstraction. The goal was adding a `class-of-service` plugin that owns profile definitions and binds them to interfaces via YANG container-merge, while the iface component retains the low-level mechanism.

## Decisions

- Chose a shared registry in `internal/core/cos/` over DirectBridge or plugin-internal store, because config resolution is synchronous during InProcessConfigVerifier and a shared registry is the simplest pattern (same as subscriber handler_registry.go)
- Chose YANG container-merge over augment for the interface binding leaf, because container-merge needs no import of ze-iface-conf and deleting the plugin cleanly removes both profile definitions and the interface binding
- Chose profile ref as string (Go-validated) over leafref, because leafref across container-merged modules creates YANG coupling; Go validation via cos.Lookup() is equivalent and survives plugin removal
- Chose two explicit ingress/egress containers (PCP-keyed ingress, priority-keyed egress) over bidirectional shorthand, matching the kernel model of two independent maps
- Chose interface-level inheritance with per-unit override and "none" opt-out over per-unit-only, because the BNG use case has hundreds of units sharing one profile

## Consequences

- The cos plugin's InProcessConfigVerifier MUST run before the interface plugin's verifier; this works because registry.All() is alphabetically sorted and "cos" < "interface"
- Inline qos maps and class-of-service references are mutually exclusive on the same unit; the conflict check is in parseUnits
- Removing the cos plugin (delete dir + blank import in all.go) removes the entire class-of-service surface; inline qos maps continue working unchanged
- Future work: cos-cmd plugin for `show class-of-service`, RADIUS-driven dynamic profile selection, runtime modification

## Gotchas

- None. The container-merge assumption (A-1) was validated by functional tests without issues. Verification ordering (A-2) was confirmed by reading registry.All() sort behavior.

## Files

- `internal/core/cos/cos.go` -- shared profile registry (Register, Lookup, Clear)
- `internal/core/cos/cos_test.go` -- registry unit tests
- `internal/plugins/cos/cos.go` -- plugin main (logger, Name)
- `internal/plugins/cos/register.go` -- init() registration, InProcessConfigVerifier, runPlugin
- `internal/plugins/cos/config.go` -- parseAndRegisterProfiles, profile parsing
- `internal/plugins/cos/config_test.go` -- config parsing tests
- `internal/plugins/cos/yang/ze-cos-conf.yang` -- profile definitions + interface container-merge
- `internal/plugins/cos/yang/embed.go` -- generated YANG embed
- `internal/plugins/cos/yang/register.go` -- generated YANG registration
- `internal/component/iface/config.go` -- added cos.Lookup() resolution in parseUnits
- `internal/component/iface/config_test.go` -- 6 CoS profile resolution tests
- `test/parse/cos-profile.ci` -- valid profile + interface ref
- `test/parse/cos-profile-invalid.ci` -- out-of-range PCP rejected
- `test/parse/cos-profile-conflict.ci` -- profile + inline conflict
- `test/parse/cos-profile-not-found.ci` -- missing profile name
- `test/plugin/cos-profile-inherit.ci` -- inheritance
- `test/plugin/cos-profile-override.ci` -- unit override
- `test/plugin/cos-profile-none.ci` -- opt-out
- `docs/features.md` -- added cos profiles mention
- `docs/features/interfaces.md` -- added cos profiles row
- `docs/guide/configuration.md` -- added class-of-service section
