# 574 -- BMP Receiver + Sender (RFC 7854)

## Context

Ze had no BMP support. Operators need BMP to ingest live peer state from their
fleet (receiver) and stream ze's own BGP state to external collectors (sender).
The comparison matrix showed "No" for BMP. Both GoBGP (sender) and bio-routing
(receiver) were studied as reference implementations.

## Decisions

- **Single plugin** (`bgp-bmp`) owns both directions, over a spec-set approach
  with separate receiver/sender specs. The wire format is shared and the plugin
  is small enough to stay cohesive.
- **Receiver listener under `environment`**, over `bgp`, matching SSH/web/LG
  pattern. Sender stays under `bgp { bmp { sender { ... } } }` since it streams
  BGP-specific state.
- **Synthetic OPEN messages** in Peer Up, over skipping them. The plugin event
  system does not carry raw OPEN PDUs. `BuildSyntheticOpen` creates minimal
  29-byte OPENs from AS metadata. Capabilities not reflected.
- **No per-NLRI ribout dedup**, over a broken implementation. Initial ribout
  was keyed by messageID (unique per UPDATE), so dedup never fired and the map
  grew without bound. Removed. Per-NLRI dedup requires parsing NLRIs from raw
  UPDATE bytes.
- **All config values as strings**, over typed Go fields. YANG config tree
  delivers all values as strings ("true", "11019"). YANG list with key is
  delivered as `map[string]T`, not `[]T`. ExtractConfigSubtree wraps results
  with the root key: `{"environment": {...}}`.

## Consequences

- BMP comparison row changed from "No" to "Yes"
- Plugins wanting `environment` config can now use `WantsConfig: ["bgp", "environment"]`
  with the correct JSON wrapping pattern
- Follow-up needed: Adj-RIB-Out (RFC 8671), Loc-RIB (RFC 9069), Route Mirroring
  sender, per-NLRI ribout dedup, raw OPEN PDU access from event system

## Gotchas

- **ExtractConfigSubtree wraps results** with the root key. `ExtractConfigSubtree(tree, "environment")`
  returns `{"environment": {"bmp": {...}}}`, not `{"bmp": {...}}`. Config structs
  need two levels of nesting. This cost significant debugging time.
- **YANG values are strings.** `"enabled": "true"` not `"enabled": true`. Numeric
  fields need `strconv.ParseUint`. Boolean fields need string comparison.
  First attempt used Go typed fields and got `json: cannot unmarshal string` errors.
- **YANG list = JSON map.** `list server { key "name"; }` becomes
  `"server": {"default": {"ip": "0.0.0.0"}}`, not `[{"name": "default", "ip": "0.0.0.0"}]`.
  The `Name` field is the map key, not a struct field.
- **Plugin logger defaults to WARN.** `slogutil.Logger("bgp.bmp")` without an
  explicit env var creates a WARN-level logger. Info/Debug messages are invisible
  unless `ze.log.bgp.bmp=info` is set. This blocked debugging for an hour.
- **Per-event goroutines blocked by hooks.** The `block-silent-ignore.sh` hook
  matches `default:\s*$` in Go switch statements, even when the next line is a
  `return`. Required restructuring switch/select to avoid bare `default:` lines.

## Files

- `internal/component/bgp/plugins/bmp/` -- 15 Go files (header, tlv, msg, bmp, sender, state, + tests)
- `internal/component/bgp/plugins/bmp/schema/` -- YANG, embed, register
- `rfc/short/rfc7854.md` -- RFC summary
- `docs/guide/bmp.md` -- user guide
- `docs/guide/plugins.md`, `docs/guide/command-reference.md` -- updated
- `docs/features.md`, `docs/comparison.md` -- BMP listed
- `test/parse/bmp-config.ci` -- config parse test
- `test/plugin/bmp-receiver-session.ci` -- listener functional test
- `test/plugin/bmp-receiver-messages.ci` -- Initiation + Peer Up + Peer Down functional test
- `internal/component/plugin/server/config_delivery_test.go` -- ExtractConfigSubtree test
