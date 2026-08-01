# Learned: vpp-3-mpls

Spec: spec-vpp-3-mpls.md
Date: 2026-05-15

## What changed

Wired MPLS labels from BGP labeled unicast NLRI (RFC 8277) through the full
RIB/sysRIB chain into VPP's FIB via GoVPP. Three layers that were previously
separate became one indivisible chain:

1. `SplitLabeled` in nlrisplit strips 3-byte label entries from SAFI 4 NLRI,
   yielding plain CIDR bytes for BART storage. Registered for ipv4/ipv6
   mpls-label families.

2. `FamilyRIB` stores labels as side-data (parallel BART store), not on
   RouteEntry. `isCIDRFamily` extended with `SAFIMPLSLabel` so labeled unicast
   enters the trie after label stripping.

3. `BestChangeEntry.Labels []uint32` added to both bgp-rib and sysrib events
   (omitempty for backward compat). fibvpp dispatches to `govppMPLSBackend`
   when labels present: push via `IPRouteAddDel` with `LabelStack`, swap/pop
   via `MplsRouteAddDel`.

## Key decisions

1. **Labels as FamilyRIB side-data, not RouteEntry field:** RouteEntry is a
   pure attribute cache unit (pool handles for interned wire attributes). Labels
   are per-prefix metadata from NLRI, not path attributes. Storing them in a
   parallel BART keyed by the same prefix keeps RouteEntry's contract clean.
   Tradeoff: lookup requires a second BART query in `lookupLabelsForBest`.

2. **FRR/BIRD-aligned SAFI remap:** Both strip labels at parse time, store
   routes as plain IP prefixes, run best-path on unlabeled keys. Ze does the
   same: `SplitLabeled` strips, BART keys on `netip.Prefix`, labels are
   metadata. This means labeled and unlabeled unicast share the same trie,
   matching RFC 8277 Section 2 (label binding to prefix).

3. **Three MPLS operations, two VPP APIs:** Push uses `IPRouteAddDel` with
   `LabelStack` in `FibPath` (IP prefix lookup, encap with labels). Swap and
   pop use `MplsRouteAddDel` (MPLS label lookup). This matches how VPP
   internally separates IP FIB from MPLS FIB.

4. **omitempty backward compatibility:** The Labels field is `json:",omitempty"`
   on both event structs. External plugin processes consuming JSON see
   `"labels":[100,200]` only when labels are present. No schema version bump
   needed.

## Bugs caught by review

- **flushRoutes missed MPLS:** Only flushed IP installed routes, not MPLS.
  VPP restart would leave stale MPLS entries.

- **govppMPLSBackend never wired:** Created but not assigned in register.go.
  All MPLS calls hit nil pointer.

- **Label side-data leaked:** `FamilyRIB.Remove` and `PurgeStale` cleaned the
  route trie but not the parallel labels trie.

- **recomputeBest ignored label changes:** Same-best optimization skipped
  re-emission when only labels changed (e.g., label re-binding).

- **Label 0 (IPv4 Explicit NULL) rejected:** Test peer dispatch rejected
  label 0 as invalid. RFC 3032 reserves label 0 as IPv4 Explicit NULL,
  which is valid and commonly used.

## Patterns for future work

- The side-data parallel BART pattern (keyed identically to the main trie)
  can carry other per-prefix metadata that does not belong on RouteEntry.

- MPLS VPN (L3VPN) will extend this: labels come from VPN NLRI, dispatch
  goes to per-VRF FIB tables. The mplsBackend interface is ready for that
  (tableID parameter on govppMPLSBackend).

- Multi-label stack (stacked LSPs) deferred. The wire format and pool
  already support it; the fibvpp dispatch needs ECMP label handling.

## Mistakes to avoid

- When adding a parallel data store (side-data), ensure all removal paths
  (Remove, PurgeStale, Reset) clean both stores. Missing one creates leaks
  that are invisible until memory pressure.

- When extending same-best optimization, check whether the new field
  participates in equality. Labels changing on the same prefix is a
  meaningful event that must propagate.

## Files

None recorded.
