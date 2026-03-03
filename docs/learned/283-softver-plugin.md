# 283 — BGP Software Version Plugin Extraction

## Objective

Extract capability code 75 (draft-ietf-idr-software-version) from the core BGP reactor into a standalone `bgp-softver` plugin, following the identical pattern established by `bgp-hostname` (FQDN, code 73).

## Decisions

- Copied the exact `bgp-hostname` structure: `RunPlugin` → `OnConfigure` callback → `SetCapabilities` → `RunDecodeMode`. No variation from the template.
- Version string changed from hardcoded `"ExaBGP/5.0.0-0+test"` to `"Ze/0.1.0"` — the leftover ExaBGP string was a migration artifact.
- Mode support (enable/require/disable/refuse) added to match `bgp-hostname` — not mentioned in the original spec but consistent with the pattern.

## Patterns

- `bgp-hostname` is the canonical template for informational capability plugins: same SDK hooks, same decode protocol, same YANG augmentation target (`bgp:capability`).
- The `CodeSoftwareVersion` constant in `capability.go` can be safely removed — the Unknown capability handler catches unregistered codes; registered plugins intercept via `CapabilityCodes`.
- YANG-driven autocomplete is automatic: no additional editor integration work when YANG schema is updated.

## Gotchas

- None.

## Files

- `internal/plugins/bgp-softver/softver.go` — encode, decode, config extraction
- `internal/plugins/bgp-softver/register.go` — `init()` registration with code 75
- `internal/plugins/bgp-softver/schema/ze-softver.yang` — augments `bgp:capability`
- Removed from: `capability/capability.go`, `reactor/config.go`, `format/decode.go`, `cmd/ze/bgp/decode.go`, `ze-bgp-conf.yang`
