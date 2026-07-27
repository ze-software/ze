# Role Plugin -- Meta Keys

<!-- source: internal/component/bgp/plugins/role/otc.go -- OTCIngressFilter, OTCEgressFilter -->

## Keys Set

| Key | Type | Stage | When Set | Description |
|-----|------|-------|----------|-------------|
| `src-role` | `string` | Ingress filter | Source peer has role config | Set to our configured role for the source peer (e.g., `"provider"`, `"customer"`, `"peer"`, `"rs"`, `"rs-client"`). Derived from `peerRoleConfig.role` (our config), not from the peer's OPEN capability. |

## Keys Read

| Key | Type | Stage | How Used | Description |
|-----|------|-------|----------|-------------|
| `src-role` | `string` | Egress filter | `resolveSrcRole(meta, srcCfg)` | If source role is Provider/Peer/RS and destination role is Provider/Peer/RS, route is suppressed. This is our configured knowledge of the peer relationship. If we don't configure a role, we don't filter. |

## Absence

`src-role` being absent from `meta` does **not** disable OTC suppression. The egress filter reads it through `resolveSrcRole`, which falls back to the source peer's configured role when the key is missing, and takes that same fallback when the key is present but not a usable string (a malformed input must never be more permissive than a missing one). Only a source peer with **no role config at all** yields the empty role, and that is what disables filtering: if we don't configure a role for a peer, we choose not to filter its routes.

The fallback exists because not every egress caller has been through the ingress filter. `RelayStoredRoute` replays out of the Adj-RIB-In with no ingress metadata, and treating the missing key as "no restriction" silently skipped an RFC 9234 Section 5 leak guard on that path.

The destination side has the same shape. The destination role comes from the peer's OPEN Role capability, which a peer may legitimately not send (accepted when `strict` is unset), so `resolveDestRole` recovers what the peer IS from the complement of our configured role toward it. That recovery feeds the Section 5 gates only: export-set matching keeps using the capability value, because `unknown` there is an operator-selected target for peers that announced no role, not an unanswered question.

Export role filtering (separate from OTC) still applies based on the source peer's export policy.

<!-- source: internal/component/bgp/plugins/role/otc.go -- OTCEgressFilter -->
<!-- source: rfc/short/rfc9234.md -- Section 5, OTC leak prevention -->

## Ordering

Ordering is enforced by pipeline structure, not convention. On the receive path the reactor processes ingress filters to completion before starting egress forwarding, so `OTCIngressFilter` runs before `OTCEgressFilter` for that UPDATE.

Do **not** read that as "egress always sees ingress metadata". Egress is reachable on paths that never ran ingress for the advertisement being built: `RelayStoredRoute` replays out of the Adj-RIB-In and passes `meta` as nil. That is why the egress filter recovers the source role from config rather than trusting the key to be present (see Absence above).

<!-- source: internal/component/bgp/reactor/received_update.go -- ingress/egress pipeline -->

## Coupling

Self-contained within the role plugin. No other plugins read or set `otc`.

## Performance

Ingress: one `findOTC()` wire scan per UPDATE (unchanged from before -- was already done).
Egress: one map lookup per destination peer (replaces `extractAttrsFromPayload` + `findOTC` wire scan per peer). Net saving: N-1 wire scans for N destination peers.
