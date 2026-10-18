# Route Metadata Keys

Route metadata (`map[string]any`) travels with UPDATEs through the forwarding
pipeline. Ingress filters set metadata; egress filters read it.

Each plugin documents the meta keys it sets and reads below.

## Key Registry

| Key | Set By | Read By | Type | Description |
|-----|--------|---------|------|-------------|
| `src-role` | role (ingress) | role (egress) | `string` | Source peer's role from our config (e.g., "provider", "customer", "peer", "rs", "rs-client") |
| `src-peer-role` | role (ingress) | filter_community (ingress) | `string` | What the source peer IS to us: the Role capability it announced, or the complement of our configured role, from RFC 9234 Table 2. Absent when the role cannot be resolved |
| `stale` | gr (ingress, `LLGRIngressFilter`); rib (replay and `rib_commands.go` re-advertise, from the stored stale level) | rib (received store, `receivedLLGRStaleFromMeta`); gr (egress, `LLGREgressFilter`) | `uint8` | The route's stale level: 2 marks an LLGR-stale route. The ingress step writes 2 for a received route carrying LLGR_STALE on a session whose own OPEN carried the LLGR Capability (RFC 9494 Section 4.3), and writes nothing otherwise |

**Where the map is visible.** The reactor puts the ingress steps' map on the
received event (`RawMessage.Meta`), so an in-process consumer reads the decision
with no second scan, and a JSON event for a received UPDATE carries the same
keys in its `route-meta` object. A sent event carries `ReceivedUpdate.Meta`
there, beside the forward provenance keys (`source-peer`, `source-message-id`,
`source-id`, `source-owner`, `source-local`, `sent-path-sources`). A JSON reader
gets `stale` as a number.

**`src-role` and `src-peer-role` carry OPPOSITE values for the same peer.**
The first is OUR role toward that peer, read from the `import` keyword. The
second is that peer's role toward us. A peer we configure `role import
customer` sets `src-role` to `customer` and `src-peer-role` to `provider`. A
reader that wants "is this a provider" wants the second key. Reading the first
gives the exact inverse, silently.

## Convention

Keys are short, lowercase, no prefix needed (the map is per-UPDATE, not
global). Use the attribute or concept name directly: `otc`, `stale`, `weight`.

**Collision prevention:** check the Key Registry above before adding a key.
Two plugins using the same key silently overwrite each other.

**Type contract:** the type in the Key Registry is the contract. Readers MUST
type-assert and treat wrong-type as absent (not panic). The map is
`map[string]any` -- a plugin setting `"true"` (string) instead of `true`
(bool) would silently bypass readers.

## Per-Plugin Documentation

Each plugin that sets or reads metadata keys should have a file
`docs/architecture/meta/<plugin>.md` with these sections:

| Section | Required | Content |
|---------|----------|---------|
| Keys Set | Yes | Key, type, stage, when set, description |
| Keys Read | Yes | Key, type, stage, how used, description |
| Absence | Yes | What happens when the key is missing (default behavior) |
| Ordering | If applicable | Which filter stage sets/reads, how ordering is enforced (pipeline structure vs convention) |
| Coupling | If applicable | Other plugins that read keys this plugin sets, or vice versa |
| Performance | If applicable | Cost of setting/reading (wire scan, map lookup, etc.) |
