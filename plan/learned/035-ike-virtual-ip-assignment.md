# Learned: a virtual IP is runtime state, lands on the remote selector, and is answered only inside the proposal

`spec-ike-virtual-ip-assignment` met `RFC3948-5.1-1` for pool-bound peers
only. Two peers behind separate NATs that both chose the inner address
10.1.2.3 could each get a tunnel Child SA naming it, which is the hazard RFC
3948 Section 5.1 says implementors MUST prevent. Ze now leases each pool-bound
responder peer a locally unique address and answers its CFG_REQUEST (the IRAS
role of RFC 7296 Section 2.19). It still never sends CFG_REQUEST (no IRAC
role).

A later independent audit (2026-10-26) found the hazard still open for a
tunnel-mode peer bound to no pool: with no configured remote selector,
`narrowSelectors` accepts any inner source the peer proposes, and the policy
registry refuses only an identical tunnel selector, so two such peers both
proposing 10.1.2.3/32 each install a policy. The row is `{partial}`, and
`plan/release/fixes/spec-ike-unbound-peer-inner-address.md` owns the rest.

## What changed

- The Configuration payload decoder is bounded: a 1 to 3 octet remnant and
  more than 64 attributes are refused (`PayloadCP.ReadFrom`,
  `internal/component/ike/wire/payload_cp.go`).
- Named pools, a per-peer `virtual-ip-pool` binding and commit refusals for
  an unknown, empty, overlapping or uncovered pool, an `initiate` peer and the
  `domain` leaf (`ValidateVirtualIPPoolBindings`,
  `internal/component/ike/ipsec/validate.go`).
- One engine-wide lease registry that outlives the pools
  (`eap.Registry`, `internal/core/eap/pool.go`): one address per owner per
  family, the owner's last address preferred on reconnect, a rotating IPv4
  cursor, and a pool edit that never re-issues a live lease.
- The responder consumer (`internal/component/ike/engine/cp.go`):
  `assignVirtualIP` leases and builds the CFG_REPLY ahead of the SA payload, on
  the direct and the EAP path; FAILED_CP_REQUIRED and INTERNAL_ADDRESS_FAILURE
  refuse the Child SA and keep the IKE SA; `substituteVirtualIP` narrows the
  remote (TSi) selector to the lease host. `forgetKeys` releases the lease, an
  IKE rekey moves it, and a Child SA Ze starts on a leased IKE SA proposes only
  the lease (`proposeLeasedChildTSPayloads`, `answerNamesOnlyLeases`).
- Visibility: the `virtual-ip` field of `show vpn ipsec sa`, a pool
  utilization doctor check and metrics.
- Interop: `virtual-ip-assignment` (EAP-MSCHAPv2, `vips = 0.0.0.0`) and
  `virtual-ip-uniqueness` (two NAT-translated PSK clients both asking
  10.1.2.3), green against strongSwan with their reds recorded.

## What outlives the spec

1. **The client's address lands on the REMOTE selector.** The lease is the
   initiator's inner address, so it narrows TSi. The skeleton said TSr, which
   would have narrowed Ze's own protected side to the client.
2. **Uniqueness belongs to an object that outlives configuration.** A pool is
   rebuilt on every edit; leases are runtime state. Keeping them in the pool
   would have re-issued a live address the moment an operator touched the
   range.
3. **Answer inside the proposal, never instead of it.** The first version
   replaced any TSi with the lease, which answered traffic the peer never
   proposed. RFC 7296 Section 2.9 allows only a subset of the proposal, and
   strongSwan's gateway intersects (`traffic_selector_list.c`, `select_`). A
   proposal that does not contain the lease is TS_UNACCEPTABLE.
4. **A client-side green can hide a gateway-side red.** With the substitution
   disabled strongSwan still established and narrowed its own side to the
   lease. Only the checks reading Ze's installed selectors and XFRM policies
   went red, so those are the discriminating assertions of both scenarios.
5. **A discrimination record does not see helpers.** `behaviorSHA`
   (`internal/le/rfc/discriminate.go`) fingerprints the tagged unit's body
   without comments, so editing a fixture it calls (`leasedRekeySelector`)
   leaves the record verified. The row is in
   `plan/journal/check-cannot-see-the-change-it-looks-for.md`.
