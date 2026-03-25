# Route Metadata Keys

Route metadata (`map[string]any`) travels with UPDATEs through the forwarding pipeline.
Ingress filters set metadata; egress filters read it.

Each plugin documents the meta keys it sets and reads below.

## Key Registry

| Key | Set By | Read By | Type | Description |
|-----|--------|---------|------|-------------|
| `src-role` | role (ingress) | role (egress) | `string` | Source peer's role from our config (e.g., "provider", "customer", "peer", "rs", "rs-client") |

## Convention

Keys are short, lowercase, no prefix needed (the map is per-UPDATE, not global).
Use the attribute or concept name directly: `otc`, `stale`, `weight`.

**Collision prevention:** check the Key Registry above before adding a key. Two plugins using the same key silently overwrite each other.

**Type contract:** the type in the Key Registry is the contract. Readers MUST type-assert and treat wrong-type as absent (not panic). The map is `map[string]any` -- a plugin setting `"true"` (string) instead of `true` (bool) would silently bypass readers.

## Per-Plugin Documentation

Each plugin that sets or reads metadata keys should have a file `docs/architecture/meta/<plugin>.md` with these sections:

| Section | Required | Content |
|---------|----------|---------|
| Keys Set | Yes | Key, type, stage, when set, description |
| Keys Read | Yes | Key, type, stage, how used, description |
| Absence | Yes | What happens when the key is missing (default behavior) |
| Ordering | If applicable | Which filter stage sets/reads, how ordering is enforced (pipeline structure vs convention) |
| Coupling | If applicable | Other plugins that read keys this plugin sets, or vice versa |
| Performance | If applicable | Cost of setting/reading (wire scan, map lookup, etc.) |
