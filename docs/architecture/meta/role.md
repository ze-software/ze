# Role Plugin -- Meta Keys

<!-- source: internal/component/bgp/plugins/role/otc.go -- OTCIngressFilter, OTCEgressFilter -->

## Keys Set

| Key | Type | Stage | When Set | Description |
|-----|------|-------|----------|-------------|
| `src-role` | `string` | Ingress filter | Source peer has role config | Set to our configured role for the source peer (e.g., `"provider"`, `"customer"`, `"peer"`, `"rs"`, `"rs-client"`). Derived from `peerRoleConfig.role` (our config), not from the peer's OPEN capability. |

## Keys Read

| Key | Type | Stage | How Used | Description |
|-----|------|-------|----------|-------------|
| `src-role` | `string` | Egress filter | `meta["src-role"].(string)` | If source role is Provider/Peer/RS and destination role is Provider/Peer/RS, route is suppressed. This is our configured knowledge of the peer relationship. If we don't configure a role, we don't filter. |

## Absence

When `src-role` is absent (no role config for source peer), no OTC suppression applies. If we don't configure a role for a peer, we choose not to filter its routes. Export role filtering (separate from OTC) still applies based on the source peer's export policy.

<!-- source: internal/component/bgp/plugins/role/otc.go -- OTCEgressFilter -->
<!-- source: rfc/short/rfc9234.md -- Section 5, OTC leak prevention -->

## Ordering

Ordering is enforced by pipeline structure, not convention. The reactor processes ingress filters to completion before starting egress forwarding. `OTCIngressFilter` (ingress) always runs before `OTCEgressFilter` (egress) for the same UPDATE.

<!-- source: internal/component/bgp/reactor/received_update.go -- ingress/egress pipeline -->

## Coupling

Self-contained within the role plugin. No other plugins read or set `otc`.

## Performance

Ingress: one `findOTC()` wire scan per UPDATE (unchanged from before -- was already done).
Egress: one map lookup per destination peer (replaces `extractAttrsFromPayload` + `findOTC` wire scan per peer). Net saving: N-1 wire scans for N destination peers.
