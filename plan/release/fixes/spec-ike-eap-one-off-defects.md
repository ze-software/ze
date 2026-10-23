# Spec: IKE, EAP and IPsec one-off defects from the journal

| Field | Value |
|-------|-------|
| Status | design |
| Scope | protocol |
| Priority | P1 |
| Release reason | Each row is incorrect shipped IKEv2, EAP or IPsec behaviour (Child SA rekey, delete, endpoint, EAP-TLS) or a lab carrier that hides it |
| Depends | - |
| Phase | - |
| Handoff | verify |
| Updated | 2026-10-23 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The 2026-10-23 journal sweep left about 265 open rows that fit none of the
thirteen root-cause specs. This spec owns the 13 IKE, EAP and IPsec ones that no
existing IPsec spec already owns. Each row is a claim until a red test
reproduces it at HEAD.

Existing specs checked first: `spec-ipsec-strongswan-interop-reds` already owns
the `ike-padded-probe-strongswan` row (`conflicting-feature-contracts`,
2026-10-20), so it is not repeated here. The two IKE rows naming decrypt-failure
`StateDead` (`handleAuthResponse`, 2026-10-11) and the VPP govpp reply deadline
rows stay with root causes S3 and S4.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` and the IKE door named in `ai/doors/README.md`
- [ ] `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md`

### RFC Summaries (Scope: protocol)
- [ ] `rfc/full/rfc7296.txt` Sections 1.3.1, 1.3.3, 1.4.1, 2.7
- [ ] `rfc/full/rfc3748.txt` Section 4
- [ ] `rfc/full/rfc5216.txt` (EAP-TLS session id)
- [ ] `rfc/full/rfc4301.txt` (SPD catch-all)

## Current Behavior (MANDATORY)

Spot-checked on 2026-10-23: `closeDesignatedChildSAs` in
`internal/component/ike/engine/delete.go` returns `sessionChildDown`, and the
caller's reestablish exit is still the path the 2026-10-22 row names.

- [ ] `internal/core/eap/peer.go` - `Process`
- [ ] `internal/core/eap/eap_tls.go` - `newTLSMethod`, `tlsMethod.Process`
- [ ] `internal/component/ike/engine/rekey.go` - `applyChildRekeyResponse`, `respondChildRekey`
- [ ] `internal/component/ike/engine/delete.go` - `closeDesignatedChildSAs`
- [ ] `internal/component/ike/engine/register.go` - `runEngine`
- [ ] `internal/component/ike/engine/child.go` - `createFirstChildSA`, `initiatorFirstChildSA`
- [ ] `internal/component/ike/dataplane/register.go` - `noop` backend
- [ ] `internal/le/interoplab/ipsec/checkers.go` - narrowed-rekey checker
- [ ] `test/interop-ipsec/scenarios/real-nat-port-translated-transport-ze-responder/ze.conf`

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
An IKE datagram (IKE_AUTH with EAP, CREATE_CHILD_SA, INFORMATIONAL Delete);
the operator command `clear vpn ipsec sa`; daemon start with a dataplane key.

### Transformation Path
1. Decrypt and parse; dispatch on exchange type.
2. SA state change and Child SA install or delete through the dataplane.
3. XFRM states and policies written to the kernel.

### Boundaries Crossed
Wire to engine; engine to dataplane backend (XFRM or VPP); engine to EAP method.

### Integration Points
`dataplane.Dataplane`, the EAP method registry, the interop checkers.

### Architectural Verification
- [ ] Every SA transition goes through the one transition owner S3 builds

## Risks & Assumptions

### Assumptions
- The lab runs Ze's own kernel; a row that exists only on an Alpine guest is out of scope (`ai/META-DECISIONS.md`).

### Risks
- `spec-ipsec-strongswan-interop-reds` edits the same checkers; coordinate.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Peer Delete of the only Child SA | → | `closeDesignatedChildSAs` | interop `child-rekey-narrowing` asserting the IKE SA survives |
| Child SA rekey answered N(NO_PROPOSAL_CHOSEN) | → | `applyChildRekeyResponse` | unit asserting a hold, not a 1 s resend |

## Acceptance Criteria

| AC | Rows | Observable |
|----|------|------------|
| AC-1 | EAP undefined Codes | RFC 3748 Section 4: "Since EAP only defines Codes 1-4, EAP packets with other codes MUST be silently discarded by both authenticators and peers." A discarded packet does not consume the round budget; a unit sends 25 undefined Codes then a valid Request and asserts success |
| AC-2 | rekey answered NO_PROP, encap-port and default refusals | A refused rekey sets a rekey hold and is logged as a refusal; no once-per-second resend |
| AC-3 | `real-nat-port-translated-transport-ze-responder` fixture | The swanctl and ze proposals agree on PFS, and the scenario asserts the migration-refused rekey line |
| AC-4 | `runEngine` error exit | Every resource `runEngine` acquires is released on every exit; a unit fails start after each acquisition and asserts nothing remains |
| AC-5 | `noop` dataplane | The untagged `noop` backend is not selectable in a shipped build, or selecting it refuses start with a message naming the env key |
| AC-6 | EAP-TLS session id | An EAP-TLS 1.2 full handshake sends a non-empty ServerHello session_id when resumption is offered, or the method states it offers no resumption (RFC 5216 quote owed at design) |
| AC-7 | `clear vpn ipsec sa` | The responder's two XFRM states and two policies are gone when the command returns; `ipsec-teardown-leaves-nothing.ci` asserts it |
| AC-8 | `initiator-rekey-answer-narrows` | The checker reads the refusal before the Child SA's hard lifetime ends (lifetime set by the scenario, not luck) |
| AC-9 | `SA.peerEndpoint` | The Child SA uses the authenticated peer endpoint, not the configured address; a unit with a NAT-moved peer |
| AC-10 | EAP-TLS fragment ack | A bare fragment acknowledgement is not answered with another; `ipsec-eap-tls-clienthello.ci` goes red on the old behaviour |
| AC-11 | `closeDesignatedChildSAs` | A peer Delete of the only Child SA leaves the IKE SA up (RFC 7296 Section 1.4.1, quote owed at design); strongSwan's next CREATE_CHILD_SA succeeds |
| AC-12 | `respondChildRekey` transport mode | RFC 7296 Section 1.3.1 and 1.3.3: a rekey request without USE_TRANSPORT_MODE asks for tunnel mode, so ze installs tunnel or refuses; it never installs transport silently |
| AC-13 | `espProposalMatches` | An offered ESP proposal is matched on every transform type (ENCR, INTEG, DH, ESN), per RFC 7296 Sections 2.7 and 3.3.3 |
| AC-14 | EAP-TLS tests x4 (`tls12ClientState`, `TestRFC3748UndefinedCodesAreSilentlyDiscarded`, `TestRFC9190MitigationOffersExtendedMasterSecret` comment, RFC5216-2.1.1-4 tags) | Each test reaches its behavior through the path production takes, its comment cites a test that proves what it says, and tags that prove nothing are withdrawn |
| AC-15 | IPsec labs x7 (`ipsec-bgp-redistribute-frr` x4, PKI fixture resolution, `real-nat-port-translated-ze-responder`, `responder-eap-tls12`, `TestXFRMReadbackShowsInstalledSA`) | Each scenario runs green or is refused with the cause named (an FRR policy setting, a TLS 1.2 client the scenario cannot pass against); the PKI fixtures decode; the XFRM readback test passes in the QEMU guest |
| AC-16 | padded probe vs `unmatched` discard | The fixture sets the `unmatched` policy it needs, or the probe is exempt from the catch-all; the scenario passes on Ze's guest |
| AC-17 | NAT keepalive | RFC 3948 Section 2.3: a one-octet 0xFF keepalive reaches `dispatchNATTInbound` and is consumed; a test sends one |
| AC-18 | VPP `RemoveSA` order | Rekey removes the policy before the refcounted SA, or the comment states the order callers keep; a test rekeys on the VPP backend |
| AC-19 | EAP Types 1-4 | RFC 3748 Section 5: "All EAP implementations MUST support Types 1-4"; Identity, Notification, Nak and MD5-Challenge are answered, each with a test |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | AC |
|------|------|----|
| one red-first unit per AC | package of the producer | AC-1 to AC-12 |

### Interop Tests (Scope: protocol)

| Scenario | AC |
|----------|----|
| `child-rekey-narrowing`, `initiator-rekey-answer-narrows`, `real-nat-port-translated-transport-ze-responder` | AC-3, AC-8, AC-11 |

## Files to Modify
- The producer files under Current Behavior and the named scenario configs.

### Integration Checklist
- [ ] Each fix reached through an IKE exchange or `clear vpn ipsec sa`

### Documentation Update Checklist (BLOCKING)
- [ ] `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` where rekey or delete behaviour changes

## Implementation Steps
1. Read each producer at HEAD; move a fixed row to "Rows found already fixed".
2. Red test, fix, green, page edit, journal cell closed with the commit.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `counter-counts-the-wrong-packets` | 2026-10-05 | `eap/peer.go::Process` |
| `error-answered-with-another-errors-code` | 2026-10-09 | `applyChildRekeyResponse` |
| `fixture-encodes-an-impossible-state` | 2026-10-20 | `real-nat-port-translated-transport-ze-responder` |
| `guard-added-to-one-half-of-a-pair` | 2026-08-17 | `runEngine` error exit |
| `inert-registration-offered-as-a-choice` | 2026-10-20 | `ike/dataplane/register.go` noop |
| `invariant-enforced-by-an-absent-call-site` | 2026-09-26 | `newTLSMethod` |
| `invariant-enforced-by-an-absent-call-site` | 2026-08-17 | `clear vpn ipsec sa` |
| `lifetime-ends-before-its-last-reader` | 2026-10-21 | `initiator-rekey-answer-narrows` |
| `one-state-held-in-two-fields` | 2026-09-16 | `SA.peerEndpoint` |
| `shared-leniency-hides-the-defect` | 2026-08-10 | ike eap-tls fragment ack |
| `silent-fall-through` | 2026-10-22 | `closeDesignatedChildSAs` |
| `silent-fall-through` | 2026-10-21 | `respondChildRekey` |
| `silent-fall-through` | 2026-10-21 | `errChildEncapPort` rekey hold |
| `silent-fall-through` | 2026-08-29 | responder `espProposalMatches` compares only ENCR |
| `test-against-broken-path` | 2026-08-29 | `tls12ClientState` and `TestEAPTLSExportRefusalNamesTheCause` |
| `test-against-broken-path` | 2026-10-05 | `TestRFC3748UndefinedCodesAreSilentlyDiscarded` |
| `test-against-broken-path` | 2026-10-20 | `TestRFC9190MitigationOffersExtendedMasterSecret` comment |
| `reference-checked-claim-unchecked` | 2026-10-13 | RFC5216-2.1.1-4 `rfc5216_tls_config_test.go` |
| `test-against-broken-path` | 2026-08-23 | `ipsec-bgp-redistribute-frr` |
| `test-against-broken-path` | 2026-09-04 | `ipsec-bgp-redistribute-frr` HEAD confirmation |
| `test-against-broken-path` | 2026-10-17 | `ipsec-bgp-redistribute-frr` nightly, FRR policy |
| `unwired-feature` | 2026-08-22 | `ipsec-bgp-redistribute-frr` route never reaches FRR |
| `test-against-broken-path` | 2026-08-12 | ipsec-interop PKI fixtures `resolve_pki_placeholders` |
| `test-against-broken-path` | 2026-10-20 | `real-nat-port-translated-ze-responder` |
| `test-against-broken-path` | 2026-10-20 | `responder-eap-tls12` against a TLS 1.2 charon |
| `test-against-broken-path` | 2026-08-16 | qemu `TestXFRMReadbackShowsInstalledSA` |
| `conflicting-feature-contracts` | 2026-10-20 | `ike-padded-probe-strongswan` under the `unmatched` discard default |
| `earlier-guard-hides-the-better-error` | 2026-08-31 | `UDPTransport.Run` drops datagrams under 28 octets, NAT-keepalive guard unreachable |
| `false-synchronization-claim` | 2026-08-10 | VPP `RemoveSA` documents a policy-first order no rekey keeps |
| `gate-excludes-part-of-its-population` | 2026-08-30 | EAP Types 1-4 (RFC 3748 Section 5) |

## Rows found already fixed

| Class file | Date | Surface | Evidence |
|------------|------|---------|----------|
| `silent-fall-through` | 2026-08-29 | `espProposalMatches` | now reads ESN and DH transforms and refuses unknown types (`responder.go`, `espProposalMatches` with `esns`, `dhs`, `offeredTypes`) |
| `silent-fall-through` | 2026-09-14 | `xfrmAEADName` | `ike/dataplane/xfrm_linux.go`: an unknown name now returns an error, and `ICVLen: 128` is bound to the `xfrmAEADNames` table, every entry of which carries a 16-octet ICV |

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (red recorded per AC)
- [ ] Tests PASS
- [ ] Scoped runs: `./le job run go test ./internal/component/ike/... ./internal/core/eap/...`, `./le test integration ipsec`
