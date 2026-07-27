# Role Plugin -- Meta Keys

<!-- source: internal/component/bgp/plugins/role/otc.go -- OTCIngressFilter, OTCEgressFilter -->

## Keys Set

| Key | Type | Stage | When Set | Description |
|-----|------|-------|----------|-------------|
| `src-role` | `string` | Ingress filter | Source peer has role config | Set to our configured role for the source peer (e.g., `"provider"`, `"customer"`, `"peer"`, `"rs"`, `"rs-client"`). Derived from `peerRoleConfig.role` (our config), not from the peer's OPEN capability. |

## Keys Read

| Key | Type | Stage | How Used | Description |
|-----|------|-------|----------|-------------|
| `src-role` | `string` | Egress filter | `resolveSrcRole(meta, srcCfg)` | Suppression frame: the value is OUR role toward the source, so `customer`/`peer`/`rs-client` mean the source peer IS a Provider/Peer/RS. A route from one of those is suppressed toward a destination that IS a Provider/Peer/RS. `provider` is the accepted case (the source is our Customer, whose routes may transit). If we don't configure a role, we don't filter. |

## Absence

`src-role` being absent from `meta` does **not** disable OTC suppression. The egress filter reads it through `resolveSrcRole`, which falls back to the source peer's configured role when the key is missing, and takes that same fallback when the key is present but not a usable string (a malformed input must never be more permissive than a missing one). Only a source peer with **no role config at all** yields the empty role, and that is what disables filtering: if we don't configure a role for a peer, we choose not to filter its routes.

The fallback exists because not every egress caller has been through the ingress filter. `RelayStoredRoute` replays out of the Adj-RIB-In with no ingress metadata, and treating the missing key as "no restriction" silently skipped an RFC 9234 Section 5 leak guard on that path.

The destination side has the same shape. The destination role comes from the peer's OPEN Role capability, which a peer may legitimately not send (accepted when `strict` is unset), so `resolvePeerRole` recovers what the peer IS from the complement of our configured role toward it. That recovery feeds the Section 5 gates only: export-set matching keeps using the capability value, because `unknown` there is an operator-selected target for peers that announced no role, not an unanswered question.

Export role filtering (separate from OTC) still applies based on the source peer's export policy.

<!-- source: internal/component/bgp/plugins/role/otc.go -- OTCEgressFilter -->
<!-- source: rfc/short/rfc9234.md -- Section 5, OTC leak prevention -->

## Ordering

Ordering is enforced by pipeline structure, not convention. On the receive path the reactor processes ingress filters to completion before starting egress forwarding, so `OTCIngressFilter` runs before `OTCEgressFilter` for that UPDATE.

Do **not** read that as "egress always sees ingress metadata". Egress is reachable on paths that never ran ingress for the advertisement being built: `RelayStoredRoute` replays out of the Adj-RIB-In and passes `meta` as nil. That is why the egress filter recovers the source role from config rather than trusting the key to be present (see Absence above).

<!-- source: internal/component/bgp/reactor/reactor_notify.go -- ingress filter step loop -->
<!-- source: internal/component/bgp/reactor/reactor_api_forward.go -- egress filter step loop -->

## Coupling

Self-contained within the role plugin. No other plugin reads or sets `src-role`, the only key in the tables above.

## Performance

Ingress: one `findOTC()` wire scan per UPDATE.

Egress is **not** a single map lookup. `OTCEgressFilter` scans the payload up to three times per destination peer: `isPayloadUnicast`, then `findOTC` inside `checkOTCEgress`, then `findOTC` again in the stamping block when the first two did not return. The `src-role` lookup replaces a role *derivation*, not the wire scans.

<!-- source: internal/component/bgp/plugins/role/otc.go -- OTCEgressFilter, checkOTCEgress, isPayloadUnicast -->

## Role resolution

Both directions resolve "what is this peer to us" through `resolvePeerRole`: the OPEN Role capability when the peer sent one, otherwise the complement of our configured role toward it (`peerRoleComplement`, the value form of RFC 9234 Table 2). RFC 9234 Section 4.2 makes the configured role the prescribed input for the Section 5 procedures, so a peer that sends no capability is still subject to those procedures rather than skipping them.

Two asymmetries are worth stating plainly, because they are policy choices rather than oversights. A peer with **no role config at all** is not filtered as a *source* (`OTCIngressFilter` returns early, and the egress source role resolves to empty), but a config-less *destination* is still gated from whatever capability it announced. And an ingress rejection now follows from **local config alone** rather than from a bilaterally negotiated capability, so a one-sided role typo can blackhole prefixes rather than merely misroute policy. The egress stamping rule is the exception to the first asymmetry: it depends on the destination only, exactly as RFC 9234 Section 5 states, and applies even when the source has no role config.

One consequence operators should know: that recovery feeds the RFC gates only. Export-set matching still sees such a peer as `unknown`, so a peer can hold two roles at once -- its real role for conformance, `unknown` for policy. Reaching a capability-less customer with `export default` therefore also needs `unknown` in the export set.

<!-- source: internal/component/bgp/plugins/role/otc.go -- resolvePeerRole, peerRoleComplement -->
<!-- source: internal/component/bgp/plugins/role/config.go -- resolveExport -->

## Known limits

`extractPeerRoleConfigs` falls back to keying by peer **name** when the peer has no resolvable remote IP, while all three readers look up by address, so such a peer gets no config and the RFC gates revert to permissive.

Remote roles learned from a capability are never cleared on session down, so a peer that once advertised a role and reconnects without one keeps the stale value, which `resolvePeerRole` prefers over the config complement.

<!-- source: internal/component/bgp/plugins/role/config.go -- extractPeerRoleConfigs -->
<!-- source: internal/component/bgp/plugins/role/role.go -- setFilterState, setFilterRemoteRole -->

