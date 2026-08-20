# Pattern: BGP Family (NLRI / Capability / Attribute)

Structural template for adding a new BGP address family or protocol extension.
Derived from post-mortem analysis of SR-Policy (3 commits for 1 feature),
PATHS-LIMIT (missed interop + functional tests), and SRv6 Prefix-SID.

Related: `ai/patterns/plugin.md` (general plugin), `ai/patterns/registration.md`
(registration architecture), `docs/architecture/wire/nlri.md` (NLRI type hierarchy).

## Why This Exists

Adding a BGP family touches 10+ integration points across the codebase. The generic
rules (`completion.md`, `repo-maintenance.md`) say "wire everything" but
don't enumerate the family-specific touchpoints. Result: features ship in 2-3 commits
instead of 1, with lint failures, missing JSON paths, unregistered decoders, and
broken snapshot tests discovered after the fact.

This pattern is the exhaustive checklist. Every item was missed at least once.

## Scope

Use this pattern when adding:
- A new SAFI (address family: SR-Policy, MUP, MVPN, etc.)
- A new capability with negotiation impact (PATHS-LIMIT, ADD-PATH, etc.)
- A new attribute that needs decode/JSON/CLI support (Tunnel Encap, PrefixSID, etc.)

Not every section applies to every feature. A new attribute without a new SAFI skips
the NLRI sections. A new capability without NLRI skips the splitter. Use judgment,
but read every section before deciding to skip it.

## Checklist

### 1. Core Type + Wire (NLRI families only)

```
[ ] SAFI constant in internal/core/family/family.go
[ ] Family registration via MustRegister() in register.go
      (both IPv4 and IPv6 if applicable)
[ ] NLRI struct implementing the NLRI interface:
      Family(), Bytes(), Len(), String(), PathID(), WriteTo(), SupportsAddPath()
[ ] Parse function: (AFI, []byte) -> (NLRI, error)
[ ] Splitter in nlrisplit/ + registration for each family
[ ] ValidNextHopLens() case in attribute/mpnlri.go
      (and check ALL exhaustive switches on SAFI -- see section 6)
```

### 2. JSON + CLI Decode

These are the most commonly missed items. They are separate integration points
from wire encode/decode.

```
[ ] AppendJSON(buf []byte) []byte method on the NLRI struct
      (satisfies nlri.JSONAppender -- hot-path JSON, no map[string]any)
[ ] DecodeNLRIHex(family, hex string) (any, error) function
      (CLI decode path: ze bgp decode --nlri)
[ ] parseNLRIByFamily() case in cli/decode_mp.go
      (dispatch for the new SAFI in UPDATE decode)
```

### 3. Plugin Registry Registration

The NLRI type, the splitter, and the plugin registry entry are three different
registrations. Missing the plugin registry entry means `--nlri decode` and
`ze bgp decode` cannot find the family.

```
[ ] registry.Register(Registration{...}) in init() with:
      Name:                 "bgp-nlri-<name>"
      Description:          "..."
      SupportsNLRI:         true
      Features:             "nlri"
      Families:             []string{"ipv4/<name>", "ipv6/<name>"}
      InProcessNLRIDecoder: DecodeNLRIHex
      RunEngine:            func(net.Conn) int { return 0 }  // codec-only plugin
      CLIHandler:           func([]string) int { return 0 }   // codec-only plugin
[ ] Run make generate (updates internal/component/plugin/all/all.go)
```

### 4. Capability (if the feature introduces one)

```
[ ] Capability struct in capability/capability.go with Parse/Write/Len
[ ] Negotiation logic in reactor/session_negotiate.go
[ ] EncodingCaps / EncodingContext fields if it affects per-peer encoding
[ ] capabilityToZeJSON() case in cli/decode_open.go
[ ] JSON encoder case in format/json.go
[ ] DecodedNegotiated event fields in format/decode.go
[ ] ExaBGP bridge event encoding in bridge/bridge_event.go
```

### 5. ExaBGP Bridge (if the family is ExaBGP-visible)

```
[ ] parseFamilyToAFISAFI case in bridge/bridge.go
[ ] convertAnnounceFamily / convertWithdrawFamily in bridge/bridge_command.go
[ ] Family-specific command parser (e.g., parseSRPolicySection)
[ ] Bridge event forwarding for the family in bridge/bridge_event.go
[ ] Bridge test coverage
```

### 5b. ExaBGP Migration (if ExaBGP supports the family)

ExaBGP config migration is a separate path from the bridge. When ExaBGP
supports a SAFI, ze's migration must handle it or `ze exabgp migrate` rejects
configs that use it. Three files need updating; missing any one causes silent
config rejection.

```
[ ] exabgp.yang schema container for the new SAFI
      (internal/exabgp/migration/exabgp.yang — add container under ipv4/ipv6)
[ ] flexSafis list in convertAnnounceToUpdate OR a dedicated convert*ToUpdate
      function if the SAFI's attribute encoding doesn't fit the generic
      convertFlexToUpdate (FlowSpec and SR-Policy both needed dedicated converters)
      (internal/exabgp/migration/migrate_routes.go)
[ ] ExaBGP compat encoding test files
      (test/exabgp-compat/encoding/conf-<name>.ci + test/exabgp-compat/etc/conf-<name>.conf)
```

Verification: diff ExaBGP's `qa/encoding/*.ci` filenames against
`test/exabgp-compat/encoding/*.ci`. Any ExaBGP test without a ze counterpart
is an untested migration path.

### 6. Exhaustive Switch Audit (BLOCKING)

New SAFI/capability constants break exhaustive switches elsewhere in the codebase.
The `exhaustive` linter catches these, but only if you run `make ze-lint-changed`.
Proactively find them:

```
grep -rn 'case.*SAFI' internal/ --include="*.go" | grep -v _test.go
grep -rn 'switch.*safi' internal/ --include="*.go" | grep -v _test.go
```

Known locations that need a case for every SAFI:
- `attribute/mpnlri.go` ValidNextHopLens (L2VPN AFI switch)
- Any switch on SAFI in reactor, rib, or forward path

For capabilities: grep for `switch.*cap` or `case.*Capability`.

### 7. Snapshot Tests

These golden-list tests break silently (test failure, not compile error).

```
[ ] internal/component/plugin/all/all_test.go TestRegisteredPluginNames
      (add "bgp-nlri-<name>" in alphabetical order)
[ ] internal/component/plugin/all/all_test.go TestRegisteredWireMethods
      (add any new wire methods in alphabetical order)
```

### 8. Config Surface Guards

New CLI verbs or identifiers can collide with existing config grammar.

```
[ ] reservedPeerNames in bgp/config/resolve.go
      (if the feature introduces words that could be mistaken for peer names)
[ ] command_ownership.go no-owner allowlist
      (if the feature adds commands without a runtime component owner)
[ ] YANG schema entries if the family has configurable behavior
```

### 9. Functional Tests (BLOCKING)

These must exist before claiming done. They are not "nice to have."

```
[ ] test/decode/ .ci file per AFI (e.g., bgp-srpolicy-1-decode.ci, bgp-srpolicy-2-decode.ci)
[ ] test/encode/ .ci round-trip test if the family supports encoding
[ ] test/interop/ scenario if FRR/Bird supports the family
      (also check existing interop configs for stale syntax)
```

### 10. Attribute Support (if a new attribute code)

```
[ ] Attribute constant in attribute/attribute.go
[ ] Parse/encode functions (or document that OpaqueAttribute passthrough is intentional)
[ ] RFC 7606 validator if the attribute has structural constraints
[ ] Wire round-trip test
```

### 11. Documentation + Feature Tables

```
[ ] docs/features/ entry (or update existing feature page)
[ ] Feature comparison tables (docs/comparison/)
[ ] Config reference (docs/reference/)
[ ] RFC summary in rfc/short/
```

### 12. Cross-Plugin Impact Assessment

New code can expose latent bugs in existing code paths. The SR-Policy plugin
exposed a bug where `decodeNLRIOnly` used an init-time snapshot, breaking all
InProcessNLRIDecoder plugins including MUP.

```
[ ] Test existing similar plugins still work after your changes
      (run their decode/encode tests, not just yours)
[ ] If you changed a shared code path (decode_mp.go, registry lookup, etc.),
      run the full test suite for that package
```

## Reference Implementations

| Feature | Commits | What to study |
|---------|---------|---------------|
| SR-Policy (SAFI 73) | d100f7fb, 70882f29, 179700d5 | Full NLRI plugin + what was missed |
| PATHS-LIMIT (cap 76) | 90d860a4, 6f2e9c7e, 56f48c85 | Capability + config restructure |
| SRv6 Prefix-SID (attr 40) | d7ee4b19 | Attribute-only (no new SAFI) |
| MUP (SAFI 85) | see nlri/mup/ | Standalone NLRI type pattern |

## Anti-Patterns (from real incidents)

| What happened | Root cause | Checklist item that catches it |
|---------------|-----------|-------------------------------|
| `ze bgp decode --nlri` returned `parsed:false` | No `InProcessNLRIDecoder` in plugin Registration | Section 3 |
| All other NLRI plugins broke on decode | `decodeNLRIOnly` used init-time snapshot, not live registry | Section 12 |
| Lint failure: exhaustive switch | New SAFI not added to L2VPN switch in mpnlri.go | Section 6 |
| Snapshot test failure | Plugin name missing from all_test.go golden list | Section 7 |
| No JSON output for the NLRI in show/monitor | Missing AppendJSON method | Section 2 |
| Interop test used stale config syntax | Config restructure didn't update existing interop scenarios | Section 9 |
| Config rejected valid peer name | New CLI verb not in reservedPeerNames | Section 8 |
| `ze exabgp migrate` rejected config with SR-Policy routes | No `sr-policy` container in `exabgp.yang`, no entry in `flexSafis`, no dedicated converter | Section 5b |
| ExaBGP added feature silently missing from ze compat tests | No diff check between ExaBGP `qa/encoding/` and ze `test/exabgp-compat/encoding/` | Section 5b |
