# 340 — ADD-PATH RIB Threading

## Objective

Thread RFC 7911 ADD-PATH negotiation state from the format pipeline into the bgp-rib plugin, replacing hardcoded `addPath=false` that silently corrupted stored routes when ADD-PATH was negotiated.

## Decisions

- ADD-PATH per-family flags travel in the `"raw"` JSON block alongside `"attributes"`, `"nlri"`, `"withdrawn"` — no new envelope fields needed
- `Event.AddPath map[string]bool` parsed from `raw.add-path` — Go map lookup returns `false` for missing keys, safe default for non-ADD-PATH peers
- Forward path: added `ExtractMPFamily` to message package and `addPathForUpdate` helper to reactor — queries per-family instead of hardcoding IPv4Unicast
- No changes to `SplitUpdateWithAddPath` signature — per-family determination happens at the call site

## Patterns

- The format pipeline already had full `EncodingContext` at the point where JSON is built — the information existed, just wasn't emitted
- `PeerRIB.Lookup(family, wireBytes)` is the right way to verify stored content — count-only assertions (`Len()==N`) can pass by coincidence when wrong parsing produces entries that dedup to the same count
- Small helper functions at package boundaries (`ExtractMPFamily` in message, `addPathForUpdate` in reactor) compose cleanly through `nlri.Family`

## Gotchas

- Count-only test assertions on map-backed stores are dangerous: with `addPath=false`, splitNLRIs misparses ADD-PATH wire bytes into multiple zero-prefix entries that collide in the map, yielding the same count (2) as correct parsing. Must assert on actual stored wire bytes.
- RIB plugin still uses JSON-RPC mode (`sdk.NewWithConn`), not text protocol (`sdk.NewTextPlugin`) — text conversion is a separate future spec

## Files

- `internal/component/bgp/format/text.go` — emit `"add-path"` per-family flags in `formatFullFromResult`
- `internal/component/bgp/event.go` — `AddPath map[string]bool` field + `parseRawFields` extraction
- `internal/component/bgp/plugins/bgp-rib/rib.go` — `event.AddPath[familyStr]` replaces hardcoded `false`
- `internal/component/bgp/message/update_split.go` — `ExtractMPFamily` for raw path attribute family extraction
- `internal/component/bgp/reactor/reactor_api_forward.go` — `addPathForUpdate` for per-family forward path
