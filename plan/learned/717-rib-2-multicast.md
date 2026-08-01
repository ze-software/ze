# Learned: Multicast RIB RPF Lookup

## What Was Built

`bgp rib rpf <family> <source-addr>` command performing longest-prefix-match
against the sharded Loc-RIB. Returns JSON with matched prefix, next-hop, admin
distance, and metric. Wired through CLI proxy (YANG, RPC registration) to the
bgp-rib plugin.

## Key Decisions

- **LPM is generic, not multicast-specific.** Exposed on `store.Store` and
  `locrib.RIB` for any CIDR family. Useful for FIB debugging and connected-route
  lookup beyond multicast.

- **Query all shards, pick longest.** The Loc-RIB is sharded by prefix hash, so
  an LPM query cannot know which shard holds the covering prefix. Solution: query
  every shard's BART trie and take the result with the most bits. Acceptable because
  multicast tables are small and shard count is low (4-16).

- **Use `bart.Table.LookupPrefixLPM(hostPrefix)`.** BART's native `Lookup(addr)`
  returns only (val, ok) without the matched prefix. Using `LookupPrefixLPM` with
  a /32 or /128 host prefix gives both the matched prefix and the value.

## What Was Already Working

The spec's original problem statement was wrong. Storage separation for multicast
already existed: `FamilyRIB` keys by `family.Family`, Loc-RIB keys by family,
best-path selection runs per-family. The only real gap was the LPM query interface.

## Mistakes / Friction

- **Wiring gap caught by review.** The initial implementation registered the command
  inside the RIB plugin but did not add the CLI proxy handler in `cmd/rib/rib.go`.
  Without that proxy, the command would only be reachable through the interactive
  CLI fallback or dispatch-command from another plugin. Review step 6 (wiring
  verification) caught this.

- **Pre-commit hook spec audit.** The hook requires filled audit tables and a
  learned summary before allowing a commit with a selected spec. This is the right
  gate but requires discipline to fill these sections before attempting the commit.

## Files

None recorded.
