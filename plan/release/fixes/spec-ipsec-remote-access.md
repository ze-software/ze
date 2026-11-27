# Spec: ipsec-remote-access

| Field | Value |
|-------|-------|
| Status | design |
| Scope | protocol |
| Priority | P0 |
| Release reason | Implement the owner-selected repair for ignored remote-access authentication: road-warrior admission and per-user credentials. |
| Depends | Closed prerequisites: `spec-eap-tls-identity-authorization` (2026-11-27, owns `RFC5216-5.2-1`, `RFC5216-5.3-2`, and `RFC5216-5.3-3`; exported `Session.PeerID()` / `Session.PeerIDs()` in `internal/core/eap/eap.go` and documented in `docs/architecture/ike/ipsec-9-ikev2-eap-nat.md`), `spec-ike-virtual-ip-assignment` (2026-10-26, `plan/learned/035-ike-virtual-ip-assignment.md`), `spec-fixit-ipsec-verify-siblings` (2026-08-03) |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-26 |

Recovery after compaction: `.claude/rules/post-compaction.md`. Then read
`internal/component/ike/engine/register.go` `tryResponderSAInit`,
`internal/component/ike/engine/cp.go` `assignVirtualIP`,
`internal/core/eap/pool.go` `Registry.Acquire`,
`internal/core/eap/eap.go` `Session.Begin`, and
`internal/core/eap/eap_mschapv2.go` `handleResponse`.

## Owner decisions (2026-11-24)

All four design choices are settled and folded into this spec.

| ID | Decision |
|----|----------|
| OD-1 (multiple devices per `eap-user`) | Support multiple simultaneous devices per user (with a per-session discriminator and a configurable `max-sessions-per-user` leaf, range `1..64`, default `4`), so one user can connect laptop + phone concurrently. |
| OD-2 (half-open handshake DoS caps) | Default `half-open-limit` to `64` (range `1..4096`) and `half-open-limit-per-address` to `8` (range `1..256`, for clients behind shared NAT), and refuse `0` (cannot disable either limit). |
| OD-3 (client-requested VPN IP & static per-user IP) | Support optional `static-ip` and `static-ip6` leaves per `eap-user` in config (validated in-pool and unique across users, reserved against dynamic allocation); otherwise honor a road-warrior client's requested IP only if it was that user's own previous IP (never hand user B's recently released IP or an arbitrary free IP to user A on request). |
| OD-4 (initial EAP Identity Request, RFC 7296 §3.16) | Follow RFC 7296 Section 3.16 (`SHOULD NOT`): drop the redundant initial EAP Identity Request in `Session.Begin` for `eap-mschapv2` and `eap-tls` (keeping it only where required for `eap-md5`), and update affected EAP tests cleanly. |

## Task

`vpn ipsec remote-access` declares a road-warrior service. Two of its three
parts still have no behavior. Re-read against the code on 2026-11-26:

| Config | Consumer today | Effect |
|--------|----------------|--------|
| `remote-access pool` | `virtualIPState.reload` (`engine/virtual_ip.go`), called from the apply path (`engine/apply.go`) | **live since 2026-10-26.** A pool serves every `site-to-site` respond peer whose `virtual-ip-pool` leaf names it |
| `remote-access ike-group`, `esp-group` | none | none. The YANG descriptions say so |
| `remote-access authentication` | `warnKeylessEAPModes` (`engine/eap_auth.go`) writes one warning for `eap-md5` | no handshake reads the mode or the `x509` leaves |
| `remote-access eap-user` | `parseEAPUser`, `ValidateRemoteAccess`, `remoteAccessEqual` only | authenticates nobody. `EAPUser.Password` and `EAPUser.Certificate` have no reader outside parse, validation and config equality |

The responder admits an unsolicited IKE_SA_INIT only from a source that
`matchResponderPeer` (`engine/register.go`) resolves to a configured respond
peer by source address, or that `matchReauthenticatingPeer`
(`engine/reauth_receive.go`) resolves to an established initiate peer.
Everything else is logged "unsolicited IKE_SA_INIT from unconfigured source"
and dropped in `tryResponderSAInit`. A road warrior, whose address is not
configured, cannot establish. An operator who wants one today writes it as a
respond peer with a fixed `remote-address` and one shared password
(`eapMethodConfig`, `engine/responder_eap.go`, reads `sa.PeerCfg.Auth.PSK`).

What this spec delivers:

1. **Admission.** An IKE_SA_INIT from an unconfigured source is served when
   `remote-access` is configured, with no state committed before the source
   proves it receives packets at its address (unconditional stateless COOKIE),
   and bounded by `half-open-limit` (default 64) and
   `half-open-limit-per-address` (default 8), neither disableable with 0 (OD-2).
2. **Per-user credentials and direct EAP start.** The EAP method authenticates
   against one `eap-user` entry. An unknown user fails closed identically to a
   wrong credential. Following RFC 7296 Section 3.16 (OD-4), `eap-mschapv2` and
   `eap-tls` skip the redundant initial EAP Identity Request and begin directly
   with the method's first Request; `eap-md5` keeps the Identity Request so it
   can select the user.
3. **Identity binding and multi-device sessions.** Every policy decision uses
   the `eap-user` list key the EAP method authenticated: the lease owner's user
   component, the per-user session count, the log, and the show output. One user
   may hold up to `max-sessions-per-user` (default 4) simultaneous device
   sessions, each distinguished by a per-session discriminator and leased its
   own distinct inner address (OD-1).
4. **Policy and address assignment for an admitted user.** Every road warrior is
   pool-bound, so its inner address comes from the bound `virtual-ip-pool` and
   never from an unverified client selector. An `eap-user` may configure an
   optional `static-ip` and/or `static-ip6` reserved inside that pool; otherwise
   a client's requested IP is honored only when it was that user's own previous
   lease (OD-3).
5. **Lifecycle.** Each session ends when its SA dies, and when a reload removes
   or changes its user (including credential or static-IP edits) or changes the
   remote-access block.
6. **Config correctness.** Commit refuses a remote-access block that cannot
   authenticate the gateway, names no pool when users exist, sets any limit to
   `0` or `half-open-limit-per-address > half-open-limit`, or configures a
   per-user static IP outside the bound pool or duplicated across users.

### Recorded owner rulings

Each ruling stays in force. None is reopened here.

| Date | Ruling |
|------|--------|
| 2026-07-23 | Implement the feature. Do not reject or warn about the inert config |
| 2026-07-31 | The RFC 7296 Configuration-payload rows of work package WP-9 that need the address-assignment feature moved to this spec; the rows already conformant stayed in the rfcgate-1b RFC 7296 pilot |
| 2026-08-02 | The Section 3.15.4 `INTERNAL_ADDRESS_FAILURE` obligation is spec work, not the extraction sign-off's (owner item OI-4) |
| 2026-08-31 | Ze is an IRAS and not an IRAC: it answers CFG_REQUEST and never sends one (owner item OI-1) |
| 2026-10-26 | Ownership, below |
| 2026-11-24 | OD-1 through OD-4, recorded above |

**Ownership decided 2026-10-26 (design pass of the sibling).** The sibling
`spec-ike-virtual-ip-assignment` (closed 2026-10-26, `plan/learned/035-ike-virtual-ip-assignment.md`)
became the SINGLE owner of the Configuration-payload work: codec hardening, the
lease registry and pool selection, CP receive sites, reply ordering, both
refusal notifications, selector substitution, lease lifetime, optional
attributes, visibility and the tagged and strongSwan evidence. Basis: the
fix/feature split rule in `ai/META-DECISIONS.md` (the RFC 3948 Section 5.1 MUST
is a fix reachable without admission) and the transfer outcome this section
names. The sibling's "Ownership" section maps every shared criterion:
AC-7..AC-10, AC-15 and AC-16..AC-28 of the older draft, the security list items,
the Section 3.15.4 obligation and `RFC7296-2.20-1`. None is dropped. The CP
phases A to G and phase 3 are therefore NOT implemented here; they are
prerequisites delivered by the sibling. This spec keeps admission, per-user
credentials, multi-device sessions, per-user static IPs and own-prior-only
requested-IP reuse, dropping the redundant initial EAP Identity Request, session
reaping, `ra.Auth` checks, and the road-warrior strongSwan scenarios. Admission
reuses the sibling's lease registry, passing the authenticated user and a
per-session discriminator (OD-1, OD-3).

**What the ruling means for this file.** The criteria the sibling took are
closed work and are not listed below. The ten RFC 7296 rows live in
`rfc/short/rfc7296.md` under these ids: `RFC7296-2.19-5`, `RFC7296-2.19-6`,
`RFC7296-2.19-7`, `RFC7296-2.19-8`, `RFC7296-3.15.1-8`, `RFC7296-3.15.1-9`,
`RFC7296-3.15.1-10`, `RFC7296-4-6`, `RFC7296-4-7`, `RFC7296-1.7-3`, and the
Section 3.15.4 obligation is `RFC7296-3.15.4-1`.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/ike/ipsec-14-responder.md` - the responder role, and the pool-bound peer paragraph
  → Constraint: a lease is granted after authentication and before the selectors are narrowed; every Child SA of that IKE SA names it.
- [ ] `docs/architecture/ike/ipsec-9-ikev2-eap-nat.md` - EAP, NAT traversal, and the virtual-IP lease registry
  → Decision: the page documents single-slot per-owner leases and free requested-IP acceptance; OD-1 and OD-3 extend the registry with composite `(user, session)` active slots, static-IP reservations, and own-prior-only requested-IP reuse for road warriors while preserving site-to-site respond peers.
- [ ] `docs/architecture/ike/ipsec-7-ikev2-engine.md` - engine dispatch, cookie gate, reconcile, and `tryResponderSAInit`
  → Decision: the page states `tryResponderSAInit` admits only configured peers; this spec adds the remote-access arm after configured-peer matching.
- [ ] `docs/architecture/ike/ipsec-3-data-model.md` - the typed model between YANG and the engine
  → Constraint: `SiteToSitePeer.Equal` and `AuthConfig.Equal` use total structural comparison; `remoteAccessEqual` must cover every new `RemoteAccessConfig` and `EAPUser` field.
- [ ] `docs/architecture/ike/ipsec-10-cli-diag.md` - `show vpn ipsec` snapshot boundary and `IPsecMetrics`
  → Constraint: `PeerInfo` is a value snapshot carrying no key material; Prometheus label names use underscores, never hyphens.
- [ ] `docs/architecture/doctor-and-health-checks.md` - runtime doctor checks and code ownership
  → Constraint: plugin runtime checks register in `engineDoctorChecks` (`doctor_runtime.go`), never `registry.DoctorCheckDef` (which refuses the `runtime` phase), and their codes register in `register_doctor_codes.go`, never `internal/core/diagnostic/codes.go`.
- [ ] `ai/doors/access-security.md`, `ai/doors/config-yang.md`, `ai/doors/core.md`, `ai/doors/le-tooling.md`, `ai/doors/system.md`
  → Constraint: readers of IKE state use `ipsecinventory.Tunnels()` and `ikeprobe.Probe()`, never `ike/engine`.
  → Constraint: a datagram that fails to decrypt changes no SA state; every handler opens a protected message through `openProtected`.
  → Constraint: EAP-TLS builds only through `./le` (patched `crypto/tls`).
  → Constraint: `internal/core/eap` is core and imports no component, so the credential lookup is a callback/value the engine hands in.
- [ ] `ai/rules/goroutine-lifecycle.md`
  → Constraint: a per-client session is one goroutine per lifecycle, reaped when its SA dies or its handshake times out.
- [ ] `ai/rules/protocol.md`
  → Constraint: a config the backend cannot apply exactly is refused at verify.

### RFC Summaries (Scope: protocol)
The authority is `rfc/full/`. Every sentence below was read there.
- [ ] `rfc/short/rfc7296.md`
  → Constraint (Section 2.16): "For this reason, these protocols are typically used to authenticate the initiator to the responder and MUST be used in conjunction with a public-key-signature-based authentication of the responder to the initiator."
  → Constraint (Section 2.16): "It is important that policy lookups and access control decisions use the actual authenticated identity."
  → Constraint (Section 2.6, descriptive, no keyword): "These attacks can be made less effective if a responder uses minimal CPU and commits no state to an SA until it knows the initiator can receive packets at the address from which it claims to be sending them."
  → Constraint (Section 2.6): "When a responder detects a large number of half-open IKE SAs, it SHOULD reply to IKE_SA_INIT requests with a response containing the COOKIE notification."
  → Constraint (Section 2.19): "In the case where the IRAS's configuration requires that CP be used for a given identity IDi, but IRAC has failed to send a CP(CFG_REQUEST), IRAS MUST fail the request, and terminate the Child SA creation with a FAILED_CP_REQUIRED error."
  → Constraint (Section 3.16): "Note that since IKE passes an indication of initiator identity in the first message in the IKE_AUTH exchange, the responder SHOULD NOT send EAP Identity requests (type 1)."
- [ ] `rfc/short/rfc3948.md`
  → Constraint (Section 5.1): two concurrent clients behind one NAT (or two devices of one user) MUST NOT hold the same inner address.
- [ ] `rfc/short/rfc5216.md`
  → Constraint (Section 5.3, MUST): "Once a TLS session is established, EAP-TLS peer and server implementations MUST validate that the identities represented in the certificate are appropriate and authorized for use with EAP-TLS."
  → Constraint (Section 5.3, SHOULD, not MUST): "Since the EAP-TLS server is typically connected to the Internet, it SHOULD support validating the peer certificate using RFC 3280 [RFC3280] compliant path validation, including the ability to retrieve intermediate certificates that may be necessary to validate the peer certificate." Ze makes the validation mandatory by its own choice (`eapTLSServerConfig`), because an authenticator with no trust anchor authenticates nobody.
- [ ] `rfc/short/rfc3748.md`
  → Constraint (Section 5.1): "It is RECOMMENDED that the Identity Response be used primarily for routing purposes and selecting which EAP method to use."
  → Constraint (Section 5.1): "EAP Methods SHOULD include a method-specific mechanism for obtaining the identity, so that they do not have to rely on the Identity Response."
- [ ] `rfc/short/rfc4301.md`
  → Constraint (Section 4.4.3.4): "It is necessary to impose these constraints on creation of child SAs to prevent an authenticated peer from spoofing IDs associated with other, legitimate peers."

**Key insights:**
- The cookie gate needs a session today: `cookieRequired(ps)` and
  `halfOpenResponderCount` read `activePeersMap`. Admission that creates the
  session first puts state ahead of the gate.
- MSCHAPv2 never compares the Identity response with anything.
  `handleResponse` hashes the Name field of the Response, after `stripDomain`,
  with the one configured password. `Session.Identity()` has no non-test caller.
- `PeerSession.handleRequest` (`internal/core/eap/peer.go`) already accepts an
  immediate method Request in `peerStateIdentity` without an Identity Request,
  so OD-4 requires no peer-side protocol change.
- Multi-device sessions (OD-1) and own-prior-only requested-IP reuse (OD-3)
  separate the lease registry's two keys: active lease slots key on
  `(user, session, pool, family)` so laptop + phone get two distinct addresses,
  while released-address memory (`lastBy`) keys on `(user, pool, family)` so
  either device reconnecting with a fresh SPI can reclaim its own prior IP while
  another user cannot.
- `reconcilePeersWith` stops every running session whose name is not in
  `newCfg.Peers`. A dynamic session kept in `activePeersMap` is stopped at the
  next reload unless road-warrior sessions live in their own map or are
  excluded from the configured-peer removal loop.
- A `.ci` under `test/ipsec/` drives two `ze` daemons. Ze sends no CFG_REQUEST
  (ruling of 2026-08-31), so a `.ci` road warrior reaches authentication and is
  then refused its Child SA with FAILED_CP_REQUIRED. The proofs that carry an
  address are the strongSwan scenarios and engine tests that feed wire messages
  to `tryResponderSAInit`.

## Current Behavior (MANDATORY)

**Source files read:** (2026-11-26, each by its producing function)
- [ ] `internal/component/ike/engine/register.go` - `tryResponderSAInit`: peer match, then `cookieRequired(ps)`, then the `responderBusy` compare-and-swap, then `newResponderSA`. `matchResponderPeer` compares the source address only, never the port
- [ ] `internal/component/ike/engine/cookie.go` - `cookieRequired` reads true for a nil session and otherwise compares `halfOpenResponderCount` with `cookieThreshold`, default zero, which challenges every initiation
- [ ] `internal/component/ike/engine/reauth_receive.go` - `matchReauthenticatingPeer` serves initiate peers only and refuses an EAP peer
- [ ] `internal/component/ike/engine/cp.go` - `decideCPRequest` (one CP, type request, an address attribute), `assignVirtualIP` (returns nil for a peer with no pool; `errCPRequired` for a bound peer with no request; leases with `sa.PeerName` as owner), `substituteVirtualIP`, `releaseVirtualIP`, `moveVirtualIP`
- [ ] `internal/component/ike/engine/responder.go` - `selectAuthChildSA` calls `assignVirtualIP` after authentication and before the selectors are narrowed; `finishResponderEstablish` supersedes an established SA only on the authenticated message
- [ ] `internal/core/eap/pool.go` - `Registry.Acquire`, `requestedLocked` (grants any free in-pool address regardless of prior owner), `grantLocked`, `Release`; `ownerSlot` has `{owner, pool, family}` only
- [ ] `internal/component/ike/engine/virtual_ip.go` - `virtualIPState.reload` rebuilds pools from `RemoteAccessConfig.Pools`; the registry is never replaced; no static-IP reservations exist
- [ ] `internal/component/ike/engine/sa.go` - `forgetKeys` calls `releaseVirtualIP`, the one release point
- [ ] `internal/component/ike/engine/responder_eap.go` - `eapMethodConfig`, `eapTLSServerConfig` (certificate, OCSP staple, CA, CRLs and intermediates all read from `sa.PeerCfg.Auth`), `computeServerAuth` (no PSK fallback), `startResponderEAP`, `handleResponderEAP`
- [ ] `internal/core/eap/eap.go` - `NewSession` initializes every method in `stateIdentity`; `Session.Begin` sends an EAP Identity Request on every exchange; `handleIdentity` stores `s.identity` and starts the method; `MethodConfig.Password` is one value
- [ ] `internal/core/eap/peer.go` - `PeerSession.handleRequest` already accepts a method Request directly in `peerStateIdentity` and transitions to `peerStateMethod`
- [ ] `internal/core/eap/eap_mschapv2.go` - `handleResponse`; `internal/core/eap/mschapv2.go` - `stripDomain`
- [ ] `internal/core/eap/eap_md5challenge.go` - `md5ChallengeMethod.Process` checks the Value against the one secret and reads no Name
- [ ] `internal/core/eap/eap_tls.go` - `eapTLSPeerName` returns the certificate subject and is called only to build error text
- [ ] `internal/component/ike/engine/remote_id.go` - `checkRemoteIdentity` runs no comparison when `remote-id` is empty; `remoteIDMatches` already carries the class gate and sub-tree matching
- [ ] `internal/component/ike/engine/reconcile.go` - `reconcilePeersWith`, `PeerSession.responderBusy`, `PeerSession.resumption`, `PeerInfo`
- [ ] `internal/component/ike/engine/apply.go` - `ikeEngineState.applyConfig` rebuilds pools and calls `reconcilePeersWith`
- [ ] `internal/component/ike/engine/config.go` - `validateIPsecSections` runs all commit-time IPsec validators
- [ ] `internal/component/ike/engine/resumption.go` - `resumptionFor` keeps one ticket-key store per peer name, replaced when the authentication config changes
- [ ] `internal/component/ike/engine/doctor_runtime.go`, `register_doctor_codes.go`, `doctor_virtual_ip.go` - `engineDoctorChecks` owns runtime checks answered via `OnDoctorCheck`; `register_doctor_codes.go` owns all `doctor-ipsec-*` codes
- [ ] `internal/component/ike/engine/metrics.go`, `internal/component/ike/cmd/show_ipsec.go` - Prometheus gauges and `show vpn ipsec sa`/`status` handlers
- [ ] `internal/component/ike/ipsec/types.go` - `RemoteAccessConfig` (IKEGroup, ESPGroup, Auth, Pools, Users); `EAPUser` (Name, Password, Certificate); no selectors, no pool choice, no limits, no static IP
- [ ] `internal/component/ike/ipsec/config.go` - `parseRemoteAccess` reads the authentication block with `parseAuthConfig`, the peer's parser
- [ ] `internal/component/ike/ipsec/validate.go` - `ValidateRemoteAccess` (pools, a password or a certificate name per user), `ValidatePKIRefs` (iterates `c.Peers` only), `ValidateVirtualIPPoolBindings`
- [ ] `internal/component/ike/ipsec/yang/ze-ipsec-conf.yang` - `container remote-access` and `list eap-user`. No `virtual-ip-pool`, `traffic-selector`, `half-open-limit`, `half-open-limit-per-address`, `max-sessions-per-user`, `static-ip`, or `static-ip6` leaf exists

**What is true today, by claim:**

| Fact | Producer |
|------|----------|
| An unconfigured source is dropped | `tryResponderSAInit`, after both matchers return nil |
| The cookie check runs after a session is found and before any SA exists | `tryResponderSAInit`, the block above the `responderBusy` compare-and-swap |
| A pool-bound respond peer is leased an address, owner key the peer name | `assignVirtualIP` |
| `Registry.requestedLocked` grants any free in-pool address regardless of prior owner | `internal/core/eap/pool.go` `requestedLocked` |
| A second holder of one `ownerSlot` gets the same lease | `Registry.Acquire` |
| The lease is released when the IKE SA closes, and an IKE rekey moves it | `forgetKeys`, `moveVirtualIP` |
| The kernel receives the narrowed selectors | `appendChildPolicies` reads `ChildSA.Selectors` |
| `Session.Begin` sends an EAP Identity Request for every EAP method | `internal/core/eap/eap.go` `Session.Begin` |
| The EAP credential is the peer's one password, or its certificate pair | `eapMethodConfig` |
| The user name MSCHAPv2 hashes comes from the client and selects nothing | `handleResponse` |
| The EAP-TLS client certificate is validated for chain and revocation; its name is used in error text only | `eapTLSServerConfig`, `eapTLSPeerName` |
| A commit accepts a remote-access block with no gateway certificate | `ValidatePKIRefs` and `ValidateRemoteAccess` contain no such check |
| A commit refuses a remote-access certificate name the candidate PKI lacks | the `ze:reference` statements on the two `x509` leaves and on `eap-user certificate`, enforced by `config.ValidateCustomSections` |
| A reload reads the remote-access block only to rebuild pools | `virtualIPState.reload` |

**Behavior to preserve:**
- A source that matches a configured peer is served by that peer and is never diverted.
- One half-open handshake per configured peer, accept-in-parallel for an established SA (RFC 7296 Section 2.4).
- The pool-bound site-to-site respond peer: its owner key stays the peer name, single-slot semantics and free requested-IP acceptance (`Acquire`, except respecting static-IP reservations), refusals and selectors unchanged.
- The site-to-site EAP path: one password in `MethodConfig.Password` (now starting directly with the method Request for `eap-mschapv2` and `eap-tls` per OD-4, and keeping Identity Request for `eap-md5`).
- A config with no `remote-access` block behaves as today.
- Every existing interop scenario and `.ci` (with existing EAP tests updated cleanly for OD-4).

**Behavior to change:**
- An unsolicited IKE_SA_INIT from an unconfigured source is served when `remote-access` is configured, behind an unconditional cookie challenge and bounded by `half-open-limit` (default 64) and `half-open-limit-per-address` (default 8) (OD-2).
- `eap-user` entries become the credentials of that service, supporting up to `max-sessions-per-user` (default 4) concurrent device sessions per user (OD-1) and optional per-user `static-ip`/`static-ip6` leaves with own-prior-only requested-IP reuse (OD-3).
- `Session.Begin` drops the redundant initial EAP Identity Request for `eap-mschapv2` and `eap-tls` while keeping it for `eap-md5` (OD-4).
- A commit refuses a remote-access block that cannot authenticate the gateway, names no pool when users exist, sets any limit to 0 or `half-open-limit-per-address > half-open-limit`, or configures an out-of-pool or duplicate static IP.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| The lease registry and virtual-IP state | `internal/core/eap/pool.go` `Registry`, `Acquire`, `requestedLocked`, `Release`; `internal/component/ike/engine/virtual_ip.go` `virtualIPState.reload` | One engine-wide authority: one address per `(owner, pool, family)`; grants any free requested address; remembers last released address; reloads pool descriptors without re-issuing live leases | Extends: adds a session discriminator to `ownerSlot` so one road-warrior user holds up to `max-sessions-per-user` distinct leases (OD-1), adds static-IP reservations loaded by `virtualIPState.reload` and skipped by `scanLocked`, and restricts `requestedLocked` for road warriors to that user's own previous lease (OD-3). No second allocator |
| The Configuration-payload decision | `engine/cp.go` `decideCPRequest`, `assignVirtualIP`, `substituteVirtualIP` | Answers a pool-bound peer's CFG_REQUEST, refuses a bound peer that sent none with FAILED_CP_REQUIRED, narrows TSi to the lease | Extends: a road warrior is always pool-bound, so "CP required" is the existing bound-peer rule and needs no new leaf; `assignVirtualIP` passes the road warrior's `(user, session)` and configured static IP |
| The cookie gate | `engine/cookie.go` `cookieRequired`, `verifyCookie`, `mintCookie`; `engine/register.go` `tryResponderSAInit` | Challenges an initiation before the half-open slot is taken, for a matched session | Extends: the same stateless gate runs for a source with no session, before any session exists, and is unconditional there |
| The half-open slot | `engine/reconcile.go` `PeerSession.responderBusy`; `engine/cookie.go` `halfOpenResponderCount` | One half-open handshake per configured peer; the count is derived from the sessions | Extends: each road-warrior session holds the same slot, so a derived count covers them. Adds `half-open-limit` (default 64) and `half-open-limit-per-address` (default 8) (D-4, OD-2), which a per-peer slot cannot express |
| The peer session and its owner loop | `engine/reconcile.go` `PeerSession`, `startPeerSession`, `stopPeerSession`; `engine/established.go` `maintainSA` | Owns one peer's SA, DPD, rekey and teardown | Extends: one `PeerSession` per road-warrior handshake, built from the remote-access block, with a per-session discriminator so one user can run multiple simultaneous sessions (OD-1). A parallel road-warrior FSM is refused |
| The EAP session and its methods | `internal/core/eap/eap.go` `Session`, `Begin`, `MethodConfig`; `eap_mschapv2.go`, `eap_md5challenge.go`, `eap_tls.go` | Sends Identity Request for every method and authenticates against one credential | Extends: `Session.Begin` skips the Identity Request for `eap-mschapv2` and `eap-tls` while keeping it for `eap-md5` (OD-4); `MethodConfig` gains a credential lookup the method calls with the name it authenticates, and the session reports the authenticated user. `Password` keeps working for a site-to-site peer |
| PAD matching | `engine/remote_id.go` `checkRemoteIdentity`, `remoteIDMatches` | Compares IDi with `remote-id`; no check when `remote-id` is empty; sub-tree and range matching landed | Does not apply as the authorization: RFC 7296 Section 2.16 puts access control on the EAP-authenticated identity, not on IDi. The terminator refusal in `checkRemoteIdentity` still runs |
| The reconcile path | `engine/reconcile.go` `reconcilePeersWith`, `peerConfigChanged`; `engine/apply.go` `applyConfig` | Stops a session whose peer is gone or changed, keeps the rest | Extends: road-warrior sessions are reconciled against the remote-access block and their user entry (D-6, including static-IP and limit changes). The configured-peer removal loop over `newCfg.Peers` does not stop them |
| Certificate, CRL and OCSP wiring | `engine/responder_eap.go` `eapTLSServerConfig` | Reads the certificate, its OCSP staple, the CA, its CRLs and the intermediates by the names in `sa.PeerCfg.Auth` | Extends by reuse: a session built from the remote-access `authentication` block reads the same function, so it inherits all of it |
| EAP-TLS resumption store | `engine/resumption.go` `resumptionFor`, `forgetResumption`; `PeerSession.resumption` | One ticket-key store per peering, by peer name | Extends: one store for the remote-access block, shared by its sessions, because a store per client dies with the client and its tickets could never be redeemed |
| PKI reference check | `ze:reference` in `ze-ipsec-conf.yang`; `config.ValidateCustomSections` | Refuses a name the candidate PKI does not hold | Does not change: unknown PKI references are already refused. This spec adds the presence checks (D-7) only |
| Runtime doctor checks and codes | `engine/doctor_runtime.go` `engineDoctorChecks`; `engine/register_doctor_codes.go` `doctorDiagnosticCodes`; `engine/doctor_virtual_ip.go` `checkIPsecVirtualIPPool` | Declares IKE runtime checks at Stage 1 and answers them via `OnDoctorCheck`; registers all `doctor-ipsec-*` codes in the owning `engine` package | Extends: adds the two runtime remote-access checks to `engineDoctorChecks` and registers `doctor-ipsec-remote-access-credential` and `doctor-ipsec-remote-access-admission` in `register_doctor_codes.go`. Pool exhaustion stays `doctor-ipsec-virtual-ip-pool` |
| IPsec metrics | `internal/component/ike/engine/metrics.go` `IPsecMetrics`, `RegisterMetrics`, `Update` | Publishes push-model Prometheus gauges and counters for site-to-site SAs, COOKIE checks, error notifications, EAP-TLS resumption, dataplane drift, and per-pool virtual IP leases | Extends: adds the four remote-access gauges and counters (`ze_ipsec_remote_access_sessions_active`, `ze_ipsec_remote_access_half_open`, `ze_ipsec_remote_access_admission_drops_total`, `ze_ipsec_remote_access_auth_failures_total`) to `IPsecMetrics` on the existing push-model registry |
| Show surface and peer snapshot | `internal/component/ike/engine/reconcile.go` `PeerInfo`, `PeerInfoMap`; `internal/component/ike/cmd/show_ipsec.go` `saToMap`, `handleShowVPNIPsecSA`, `handleShowVPNIPsecStatus` | Copies per-peer runtime state under lock without exporting key material and renders `show vpn ipsec sa` and `show vpn ipsec status` | Extends: surfaces live road-warrior sessions in `PeerInfoMap()` and `show vpn ipsec sa`, adds `eap-user` to `PeerInfo` and `saToMap` (empty for site-to-site peers), and keeps `configured-peers` in `handleShowVPNIPsecStatus` scoped to configured site-to-site peers |
| Remote-access config validation and equality | `internal/component/ike/ipsec/validate.go` `ValidateRemoteAccess`, `ValidateVirtualIPPoolBindings`; `internal/component/ike/ipsec/types.go` `RemoteAccessConfig`, `EAPUser`, `remoteAccessEqual` | Validates `remote-access` pool ranges/overlaps and user credential shapes, and compares `RemoteAccessConfig` for reload diffing | Extends: `ValidateRemoteAccess` enforces gateway `authentication x509 certificate` (and `ca-certificate` under `eap-tls`), `virtual-ip-pool` binding and selector coverage, non-zero half-open/per-user limits with `half-open-limit-per-address <= half-open-limit`, and in-pool unique per-user `static-ip`/`static-ip6`; `remoteAccessEqual` compares every new field structurally |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | One schema, one command registry, every interface | the door every change reads | "A backend that cannot apply a config exactly refuses it at verify. It never approximates" | agrees | Every remote-access misconfiguration (missing gateway certificate, missing pool, zero or inverted limits, out-of-pool or duplicate static IP) is refused at verify/commit with the exact leaf and fix. |
| `ai/doors/access-security.md` | Traps | the door covering `internal/component/ike/` and `internal/core/eap/` | "RFC 7296 Section 2.4 forbids concluding anything from an unprotected message." | agrees | Road-warrior handshakes reuse the existing `openProtected` path for every protected IKE_AUTH/EAP round and run a stateless COOKIE gate before allocating any session state; EAP-TLS builds only through `./le` and state readers keep using `ipsecinventory.Tunnels()`. |
| `ai/doors/le-tooling.md` | Boundaries | the door covering `test/` | "A `.ci` that needs a Linux kernel feature carries `option=needs-linux`" | agrees | New functional `.ci` tests under `test/ipsec/` and `test/reload/` follow the `.ci` harness rules, and address-assignment proofs run as named strongSwan interop scenarios under `test/interop-ipsec/scenarios/`. |
| `docs/architecture/ike/ipsec-7-ikev2-engine.md` | Decisions | `// Design:` page of `register.go`, `cookie.go`, `reconcile.go`, `apply.go`, `config.go`, `virtual_ip.go`; modified by this spec | "A responder inserts one only when `tryResponderSAInit` admits a received request from a configured peer." | departs | Under the owner ruling of 2026-07-23 and OD-2 (2026-11-24), `tryResponderSAInit` also admits an unsolicited `IKE_SA_INIT` from an unconfigured source when `vpn ipsec remote-access` is configured, gated behind an unconditional stateless COOKIE challenge and bounded by `half-open-limit` (default 64) and `half-open-limit-per-address` (default 8). Configured peers still match first and are never diverted. |
| `docs/architecture/ike/ipsec-9-ikev2-eap-nat.md` | Decisions | `// Design:` page of `eap.go`, `eap_mschapv2.go`, `eap_md5challenge.go`, `eap_tls.go`, `pool.go`; modified by this spec | "An owner (the peer) holds at most one address per pool and family. A second `Acquire` for the same slot returns the same lease with one more holder, which is what reauthentication and a parallel re-initiation need. Otherwise the order is: the requested address when it is a free host address of the range (for IPv6 outside the prefix, its interface identifier placed inside the prefix); the owner's last released address while it is still free" | departs | Under OD-1 and OD-3 (2026-11-24), a road-warrior `eap-user` may hold up to `max-sessions-per-user` (default 4) concurrent addresses per pool and family via a composite `(user, session)` active slot, a configured per-user `static-ip`/`static-ip6` reserves and pins that address, and a road-warrior client's requested IP is honored only when it was that user's own previous lease. Under OD-4 (2026-11-24), `Session.Begin` also omits the initial EAP Identity Request for `eap-mschapv2` and `eap-tls` per RFC 7296 Section 3.16 while keeping it for `eap-md5`. Site-to-site respond peers keep single-slot leases and existing requested-IP semantics unchanged. |
| `docs/architecture/ike/ipsec-14-responder.md` | Decisions | `// Design:` page of `cp.go`, `responder_eap.go`, `resumption.go`; modified by this spec | "The engine holds one `eap.Resumption` per configured peer, keyed by peer name, and `eapTLSServerConfig` REFUSES an SA that reached it without one rather than falling back to that dead key." | departs | Road-warrior sessions share one `eap.Resumption` store keyed to the `remote-access` block (rebuilt on `remote-access` auth edits, removed when `remote-access` is deleted) rather than one store per configured peer name, because a per-initiation client session dies when its SA ends and could never redeem its own tickets. Configured site-to-site peers keep one store per peer name. |
| `docs/architecture/ike/ipsec-14-responder.md` | Decisions | `// Design:` page of `cp.go`, `responder_eap.go`, `resumption.go`; modified by this spec | "no CLI command prints `DidResume`, and `show vpn ipsec sa` carries no EAP field." | departs | `show vpn ipsec sa` gains an `eap-user` field (carrying the authenticated `eap-user` list key for road-warrior sessions, empty for site-to-site peers) so operators can identify remote-access tunnels by authenticated user. `DidResume` still stays out of CLI output. |
| `docs/architecture/ike/ipsec-14-responder.md` | Decisions | `// Design:` page of `cp.go`, `responder_eap.go`, `resumption.go`; modified by this spec | "The responder SA is created by the dispatch goroutine on an unsolicited IKE_SA_INIT request from a peer configured to respond, advanced inline on that goroutine, and adopted into the owner loop by a polling `runResponder` at establishment." | departs | When `vpn ipsec remote-access` is configured, an unsolicited `IKE_SA_INIT` from an unconfigured source also creates an ephemeral road-warrior `PeerSession` (after unconditional stateless COOKIE verification and within `half-open-limit` and `half-open-limit-per-address`) and runs the same responder handshake and `CP` flow (`decideCPRequest`, `assignVirtualIP`, `substituteVirtualIP`). Configured peers still match first. |
| `docs/architecture/ike/ipsec-14-responder.md` | Decisions | `// Design:` page of `cp.go`, `responder_eap.go`, `resumption.go`; modified by this spec | "The session outlives the SA -- `ps.run` loops `runOnce` on one `PeerSession` per peer --" | departs | Each road-warrior `PeerSession` is ephemeral to one client initiation (`rw-<id>`); when its SA closes, fails authentication, or times out half-open, its lease is released, the session is removed from the road-warrior map, and its owner goroutine exits instead of looping `runOnce`. Configured site-to-site peers keep one long-lived looping `PeerSession` per peer. |
| `docs/architecture/ike/ipsec-3-data-model.md` | Decisions | `// Design:` page of `types.go`, `config.go`, `validate.go`; anchored to `ze-ipsec-conf.yang`; modified by this spec | "`RemoteAccessConfig.Pools` maps each `remote-access pool` name to a `VirtualIPPool` (its `range`, `range6` and `dns`)." | agrees | Extends `RemoteAccessConfig` and `EAPUser` with the new typed fields (`virtual-ip-pool`, `traffic-selector`, `half-open-limit`, `half-open-limit-per-address`, `max-sessions-per-user`, `static-ip`, `static-ip6`), structural equality in `remoteAccessEqual`, and commit-time validation in `validate.go`. |
| `docs/architecture/ike/ipsec-10-cli-diag.md` | Decisions | `// Design:` page of `metrics.go`, `show_ipsec.go`; anchored to `register.go`, `reconcile.go`, `cp.go`; modified by this spec | "The peer session holds crypto key material on the child SA. Exporting the fields would leak it into the show layer. `Info()` copies under the mutex; `PeerInfoMap()` returns value copies." | agrees | Road-warrior sessions surface through the same `PeerInfo` value snapshot with an added `eap-user` field and no exported key material, and the new road-warrior Prometheus gauges/counters register in `IPsecMetrics` using underscore label names. |
| `docs/architecture/doctor-and-health-checks.md` | The runtime phase: checks only the daemon runs | `// Design:` page of `doctor_runtime.go`, `register_doctor_codes.go`; anchored to `apply.go`; modified by this spec | "A plugin's runtime check reads state that lives in the plugin's own process, so its one route is the doctor callback, which reaches a forked plugin as well as an internal one." | agrees | Both new remote-access doctor checks (`ipsec-remote-access-credential` and `ipsec-remote-access-admission`) are declared in `engineDoctorChecks` (`doctor_runtime.go`), answered via `OnDoctorCheck`, and register their codes in `register_doctor_codes.go`. |
| `docs/architecture/ike/ipsec-11-interop-eap.md` | Decisions | anchored to `responder_eap.go`, `resumption.go`, `eap.go`, `eap_tls.go` | "`eap/peer.go` handles initiator-side dispatch. `eap/eap.go` stays the authenticator side. Keeping them apart is what let the responder role reuse the server session unchanged." | agrees | Keeps `eap/peer.go` and `eap/eap.go` separate; `peer.go` already accepts a method Request in `peerStateIdentity` when an authenticator skips the Identity Request (OD-4), and the road-warrior responder reuses `eap.Session` with a credential lookup. |
| `docs/architecture/ike/ipsec-13-rekey-wire.md` | Decisions | anchored to `register.go` | "Packets that arrive after establishment are routed off the shared dispatch goroutine to the owning peer session's inbound channel. The send is non-blocking, so one slow owner cannot wedge the shared loop." | agrees | Road-warrior sessions run as `PeerSession` instances and use the same `ownedSA` / `inbound` hand-off after establishment, preserving single-owner rekey and DPD handling. |
| `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` | Decisions | anchored to `register.go`, `reconcile.go`, `apply.go`, `config.go`, `types.go`, `validate.go` | "RFC 4301 Sections 4.4.2 and 5.2 require a received packet to match the selectors of the SA that carried it." | agrees | Road-warrior Child SAs install through `createFirstChildSA` and `installChildDataplane` using the narrowed leased host selector, inheriting complete directional state-scope enforcement unchanged. |
| `docs/architecture/ike/ipsec-dataplane-inspection.md` | Decision: drift is compared in one direction only | anchored to `reconcile.go`, `metrics.go`, `show_ipsec.go` | "`ObserveDataplane` records a generation before reading peer belief and checks it after the SAD dump. Child publication and peer-roster changes advance that generation." | agrees | Creating, establishing, or removing a road-warrior session advances the same dataplane generation under `peersMu`, so `ObserveDataplane` and drift counters remain consistent. |
| `docs/architecture/ike/rfcgate-1b-rfc7296-pilot.md` | Decisions | anchored to `cp.go` | "For a responder SA that holds a lease from a remote-access pool, `substituteVirtualIP` narrows each proposed initiator selector that contains a leased host address to that host, inside `narrowChildSelectorsIn`, so every caller of that entry point gets it" | agrees | Every road-warrior SA is bound to `remote-access virtual-ip-pool`, so all its Child SAs pass through `substituteVirtualIP` and `dropForeignInnerOverlaps` unchanged. |
| `docs/architecture/linux-limitations.md` | What belongs here | anchored to `config.go` | | silent | This spec adds no Linux kernel or netlink workaround and changes no XFRM state-scope, fragment, or selector programming contract; road-warrior Child SAs reuse the existing tunnel-mode XFRM install path. |
| `docs/architecture/decisions/001-pull-model-metrics.md` | Consequences | anchored to `metrics.go` | "New Ze Prometheus metrics should follow the registry-backed push model unless this decision is explicitly reopened." | agrees | All new remote-access gauges and counters (`ze_ipsec_remote_access_*`) register on the existing push-model `metrics.Registry` inside `IPsecMetrics` using the `ze_<subsystem>_<thing>_<unit>` naming convention. |
| `docs/architecture/api/process-protocol.md` | Design decisions | anchored to `register.go`, `doctor_runtime.go` | "Every engine operation names the message it carries or `none` with a reason, and a plugin identity is authorized by a profile compiled from its filter" | agrees | This spec adds no engine RPC operation and changes no plugin authorization profile; the `ike` plugin keeps its existing Stage 1 registration and declared `Sends` list unchanged. |
| `docs/architecture/api/process-protocol.md` | Design decisions | anchored to `register.go`, `doctor_runtime.go` | "Engine-to-plugin event delivery never drops events on queue overflow; endpoints expect every subscribed network event losslessly" | agrees | This spec does not alter engine-to-plugin event delivery or overflow behavior; road-warrior SA lifecycle events emit through the existing lossless EventBus path. |

## Data Flow (MANDATORY)

### Entry Point
Inbound UDP `IKE_SA_INIT` from an arbitrary source address, on the IKE (UDP 500) or NAT-T (UDP 4500) socket.

### Transformation Path
1. `deliverDatagram` finds no `SATable` entry; `tryResponderSAInit` checks the IKEv2 header (exchange `IKE_SA_INIT`, request flag, zero responder SPI).
2. `matchResponderPeer`, then `matchReauthenticatingPeer`. A match is served by the configured peer as today and the steps below do not run.
3. **New.** No configured peer matched and `remote-access` is configured (with `eap-user` entries): the COOKIE is verified unconditionally from the raw datagram alone (initiator SPI, nonce, source address). A missing or wrong cookie is answered with a fresh COOKIE notification. Nothing is allocated or stored.
4. **New.** With a verified cookie, the two half-open admission bounds (`half-open-limit`, default 64; `half-open-limit-per-address`, default 8) are checked against the live road-warrior half-open set. Over either bound, the initiation is dropped, counted, and exposed to `doctor-ipsec-remote-access-admission`.
5. **New.** One road-warrior `PeerSession` is created with a unique monotonic session discriminator (`rw-<id>`), built from the remote-access block (with `VirtualIP` bound to `remote-access virtual-ip-pool`, shared `resumption` store, and `responderBusy` set). `newResponderSA` and the `IKE_SA_INIT` exchange run unchanged.
6. `IKE_AUTH` arrives carrying `IDi` and no `AUTH`. `startResponderEAP` signs the responder `AUTH` with the gateway certificate named in `remote-access authentication x509 certificate` and calls `sess.Begin()`. For `eap-mschapv2` and `eap-tls`, `Begin()` omits the initial EAP Identity Request and emits the method's first Request directly (OD-4); for `eap-md5`, `Begin()` emits the EAP Identity Request. Any `CFG_REQUEST` is stashed on the SA (`decideCPRequest`).
7. **New.** The EAP method calls the `MethodConfig` credential lookup with the name it authenticates (D-1). An unknown user receives the exact same failure packet sequence and EAP-Failure as a wrong credential (D-8).
8. **New.** On EAP success, the session records the authenticated `eap-user` list key (`user`, D-1/D-2) and checks the user's active authenticated session count against `max-sessions-per-user` (OD-1, default 4). If already at the limit, the excess session is refused with `AUTHENTICATION_FAILED`, logged with the fix, counted, and torn down while existing sessions of that user stay up. Otherwise the session joins the user's active session set alongside any other device sessions of that user.
9. The final `IKE_AUTH` verifies `AUTH`-from-MSK (or `SK_pi` for `eap-md5`), then runs `assignVirtualIP` passing `(user, session)` and any configured `static-ip`/`static-ip6` for `user` to `Registry` (OD-1, OD-3). With a static IP configured for the requested family, that reserved address is granted (or `INTERNAL_ADDRESS_FAILURE` if another live session already holds it). Without a static IP, `requestedLocked` honors a client-requested IP only if `lastBy` records it as that exact `user`'s own prior lease in that pool and family; otherwise the user's own free last address or the rotating cursor is leased (skipping all static-IP reservations). `substituteVirtualIP` and Child SA install run unchanged.
10. SA end (`forgetKeys`) releases the lease (recording the released address in `lastBy` under `user`) and removes the road-warrior session from the road-warrior set, ending its goroutine cleanly. An unauthenticated handshake that stalls past `responderHandshakeTimeout` is reaped the same way.
11. **New.** On every config apply, `virtualIPState.reload` updates pool descriptors and static-IP reservations, and the road-warrior reconciler ends sessions whose user was removed or changed (credential or static IP), sessions exceeding a lowered `max-sessions-per-user` (newest-first or all excess), or all road-warrior sessions when the block's groups, authentication, pool binding, or traffic selectors changed (D-6).

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| UDP transport to engine dispatch | `transport.Packet` | No |
| engine to EAP session | `eap.MethodConfig` with credential lookup callback; `Session.AuthenticatedUser()` | No |
| engine to lease registry | `Registry.AcquireForUser` with `(user, session)` owner and optional static address | No |
| config tree to engine | `parseRemoteAccess` (`ipsec/config.go`) | No |
| engine to readers of IKE state | `PeerSession.Info`, `PeerInfoMap`, `ipsecinventory.Tunnels()` | No |
| engine to daemon doctor | `engineDoctorChecks` declared at Stage 1, answered via `OnDoctorCheck` | No |

### Changed-Wire Census

**Search evidence:** scope `internal/component/ike`, `internal/core/eap`,
`test/`, `docs/guide/ipsec.md`; terms `RemoteAccess`, `EAPUser`, `.Users`,
`NewPool(`, `eapTLSPeerName`, `\.Begin\(`, `remote-access`; positive controls:
`virtualIPState.reload` found as the one engine reader of
`RemoteAccessConfig.Pools`; `responder_eap.go:258` found as the one production
caller of `Session.Begin()`; 68 test call sites of `.Begin(` found across
`internal/core/eap/*_test.go` and `internal/component/ike/engine/*_test.go`;
`test/parse/ipsec-remote-access.ci` found carrying `remote-access` with
`eap-user` and no `virtual-ip-pool`. No IKE payload binary layout changes.
Three wires change: the `vpn ipsec remote-access` config tree, the peer/SA
inventory (`show vpn ipsec`), and the first EAP round on the wire for
`eap-mschapv2` and `eap-tls` (OD-4).

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| `vpn ipsec remote-access` config tree | writer | `ipsec/yang/ze-ipsec-conf.yang` `container remote-access`, `list eap-user`; the config editor; repository configs (`test/parse/ipsec-remote-access.ci`, `test/parse/ipsec-eap-auth.ci`, `docs/guide/ipsec.md`) | grep `remote-access` over `test/` and `docs/` | Change: add `virtual-ip-pool`, `traffic-selector`, `half-open-limit`, `half-open-limit-per-address`, `max-sessions-per-user`, and `eap-user` `static-ip`/`static-ip6`. Update repository configs that carry `eap-user` without `virtual-ip-pool` (`test/parse/ipsec-remote-access.ci`, etc.) in the same work |
| `vpn ipsec remote-access` config tree | reader | `ipsec/config.go` `parseRemoteAccess`, `parseEAPUser`; `ipsec/validate.go` `ValidateRemoteAccess`; `engine/virtual_ip.go` `reload`; `engine/apply.go` `applyConfig` | grep `RemoteAccess` across `internal/component/ike/` | Change: parse and validate all new leaves; sync static-IP reservations into `virtualIPState.reload` and reconcile road-warrior sessions on apply |
| Peer and SA inventory | writer | `engine/reconcile.go` `PeerSession.Info`; `engine/register.go` `PeerInfoMap`, `ActivePeers`; `engine/inventory.go` `inventorySnapshot` | read of `reconcile.go`, `register.go`, `cmd/show_ipsec.go` | Change: include road-warrior sessions alongside configured peers, carrying the authenticated `eap-user` field |
| Peer and SA inventory | reader | `internal/component/ike/cmd/show_ipsec.go` `saToMap`, `handleShowVPNIPsecSA`, `handleShowVPNIPsecStatus`; `internal/core/ipsecinventory` `Tunnels()`; operator JSON scripts | read of `show_ipsec.go` and `ai/doors/access-security.md` | Preserve all existing JSON keys; add `eap-user` (present for road-warrior sessions, omitted or null for site-to-site peers) |
| IKEv2 / EAP first round (OD-4) | writer | `internal/core/eap/eap.go` `Session.Begin`; `internal/component/ike/engine/responder_eap.go` `startResponderEAP` | grep `\.Begin\(` found `responder_eap.go:258` as the sole production caller | Change: for `eap-mschapv2` and `eap-tls`, `Session.Begin` sends `method.Start(1)` directly instead of `TypeIdentity` (RFC 7296 Section 3.16); `eap-md5` keeps `TypeIdentity` |
| IKEv2 / EAP first round (OD-4) | reader | `internal/core/eap/peer.go` `PeerSession.handleRequest`; external road-warrior and site-to-site clients (strongSwan, OS IKEv2 clients); unit tests in `internal/core/eap/*_test.go` and `internal/component/ike/engine/*_test.go` | read of `peer.go:557-570` (already handles direct method Request in `peerStateIdentity`); grep `\.Begin\(` found 68 test call sites | Preserve `peer.go` and external clients (already conformant to RFC 7296 Section 3.16); update affected unit tests cleanly and re-prove all strongSwan responder EAP interop scenarios |

### Integration Points
- `tryResponderSAInit` (`register.go`) - gains the road-warrior arm after `matchResponderPeer` and `matchReauthenticatingPeer`.
- `cookieRequired` and `halfOpenResponderCount` (`cookie.go`) - stateless verification for unconfigured sources; count includes road-warrior sessions.
- `lookupPeerSession`, `TerminatePeerSA`, `TerminateAllSAs`, `PeerInfoMap`, `ActivePeers` (`register.go`) - include road-warrior sessions alongside configured peers.
- `assignVirtualIP` (`cp.go`) and `virtualIPState.reload` (`virtual_ip.go`) - pass `(user, session)` and static-IP reservations to `eap.Registry`.
- `eapMethodConfig` and `handleResponderEAP` (`responder_eap.go`) - build the per-user lookup, record the authenticated user, and enforce `max-sessions-per-user`.
- `engineDoctorChecks` (`doctor_runtime.go`) and `doctorDiagnosticCodes` (`register_doctor_codes.go`) - register the two runtime remote-access doctor checks and their codes.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding, outbound: new doctor checks and codes register in `engineDoctorChecks` and `register_doctor_codes.go` | No | |
| Registration over hardcoding, inbound: no switch or central list learns a new subsystem or command name | No | |

## Design

One row per design decision. Every owner decision (`OD-1`..`OD-4`) is unfolded here.

| Id | Hole | Design | Decided by |
|----|------|--------|------------|
| D-1 | Which name is the user | The user is the `eap-user` entry whose credential the method verified. `eap-mschapv2`: the entry named by the MS-CHAPv2 Response's Name field after `stripDomain`. `eap-md5`: the entry named by the Identity Response (the one mode that keeps the initial Identity Request per OD-4). `eap-tls`: the entry whose `certificate` leaf names the PKI entry equal, byte for byte, to the client's validated leaf certificate (consuming the verified identity exported by the Depends spec). `IDi`, and any Identity Response under `eap-mschapv2` or `eap-tls`, select nothing and authorize nothing | RFC 7296 Section 2.16 ("actual authenticated identity"); RFC 3748 Section 5.1; RFC 5216 Section 5.3 MUST for `eap-tls` |
| D-2 | Owner key, multi-device sessions, and per-user limit | Every log, show field, and per-user session count carries the authenticated `eap-user` list key (`user`) of D-1, never a raw client string (`CORP\alice` and `alice` both resolve to `alice`). Under OD-1, one user may hold up to `max-sessions-per-user` (YANG leaf under `remote-access`, `uint32`, range `1..64`, default `4`) simultaneous device sessions. Each road-warrior session receives a unique monotonic session discriminator (`rw-<id>`), and `Registry.AcquireForUser` keys the active lease slot on `(user, session, pool, family)` so laptop + phone get two distinct inner addresses (RFC 3948 Section 5.1). An authenticated session that would exceed `max-sessions-per-user` is refused with `AUTHENTICATION_FAILED` (leaving existing live sessions untouched, logging the user, count, limit and fix, and incrementing the admission-drop counter). `INITIAL_CONTACT` does not supersede sibling device sessions of a multi-device road-warrior user. Site-to-site respond peers keep `(peerName, "")` single-slot semantics unchanged | OD-1 (2026-11-24); RFC 3948 Section 5.1; RFC 7296 Section 2.4 (`INITIAL_CONTACT` is `MAY`) |
| D-3 | Pool binding, CP mandatory, static IPs, and own-prior-only requested IP | The `remote-access` block gains `virtual-ip-pool` (naming one of its pools, mandatory at commit whenever `eap-user` is non-empty) and `traffic-selector` (same grammar and `selectorsCoverPool` check as a peer; empty accepts any TSr). Every road-warrior SA is therefore pool-bound: no `CFG_REQUEST` draws `FAILED_CP_REQUIRED`, `TSi` narrows to the lease, and the kernel installs only the lease. Under OD-3, `list eap-user` gains optional `static-ip` (`zt:ipv4-address`) and `static-ip6` (`zt:ipv6-address`). Commit refuses a static IP outside the bound pool's leasable host range (or when that family has no range) and refuses duplicate static IPs across users. `virtualIPState.reload` registers static IPs as per-user reservations on the bound pool so `scanLocked` never leases them dynamically to another user or peer. When a user has a static IP for a requested family, `AcquireForUser` grants only that address (ignoring any other requested IP; if a second live session of that static-IP user already holds it, `AcquireForUser` returns `ErrAddressInUse` -> `INTERNAL_ADDRESS_FAILURE`, preserving RFC 3948 Section 5.1 uniqueness). Without a static IP, `requestedLocked` (for road warriors, `ownPriorOnly = true`) honors a client's requested IP ONLY when `lastBy[candidate]` records it as that exact `user`'s own previously released address in that pool and family (and it is free, in-pool, and unreserved); otherwise it falls back to the user's own free last address or `scanLocked`. Site-to-site respond peers keep `ownPriorOnly = false` (respecting static-IP reservations) | OD-3 (2026-11-24); RFC 4301 Section 4.4.3.4; RFC 7296 Section 2.19 `FAILED_CP_REQUIRED`; RFC 3948 Section 5.1 |
| D-4 | No state before the cookie check, with non-disableable bounds | For an unconfigured source, the stateless COOKIE is always demanded regardless of `cookie-threshold` and verified from the raw datagram alone (`scanSAInitPreState`, `verifyCookie`). Only after verification are the two admission bounds checked and a session created. Two leaves under `remote-access` state the bounds (OD-2): `half-open-limit` (`uint32`, range `1..4096`, default `64`, all unauthenticated road-warrior handshakes) and `half-open-limit-per-address` (`uint32`, range `1..256`, default `8`, unauthenticated road-warrior handshakes from one source IP). Zero is refused on both leaves at YANG range and in `ValidateRemoteAccess`, and `half-open-limit-per-address > half-open-limit` is refused at commit. A handshake that does not authenticate within `responderHandshakeTimeout` is reaped and its session removed. Established sessions are bounded by `max-sessions-per-user` and pool capacity | OD-2 (2026-11-24); RFC 7296 Section 2.6 |
| D-5 | The session key behind a NAT | Each road-warrior session is created per verified initiation with its unique monotonic discriminator (`rw-<id>`) and is found afterward on the wire only through the `SATable` by SPI pair, never by source address. Two clients behind one NAT (same or different users) therefore run as independent sessions. After authentication, the session is also indexed under its authenticated `user` for per-user limits, reconcile, and visibility | Correctness: `matchResponderPeer` keys on source address, which clients behind one NAT share |
| D-6 | A user or block removed or changed on reload | On every apply, `virtualIPState.reload` updates pools and static-IP reservations, and the road-warrior reconciler compares every live road-warrior session with the new block. A session whose user entry is gone, or whose password, certificate, `static-ip`, or `static-ip6` changed, is ended with an IKE Delete (`StopGraceful`) and its lease released. If `max-sessions-per-user` was lowered below a user's active count, the newest excess sessions of that user are ended with an IKE Delete until the count meets the new limit. A change to the block's groups, authentication, pool binding, or traffic selectors ends every road-warrior session, matching `peerConfigChanged`. A pool range edit follows the registry's existing rule. An IKE SA rekey does not re-run EAP because user removal already ended the session. An EAP-TLS resumption re-checks that the user entry still exists and still names the certificate | Correctness: a removed or edited credential/static IP must take effect immediately; `ai/doors/access-security.md` states the same rule for logins |
| D-7 | Commit refusal without a gateway certificate | `validateIPsecSections` refuses a remote-access block with `eap-user` entries whose `authentication x509 certificate` is empty, for every mode (all remote-access modes are EAP). It refuses `mode eap-tls` with no `ca-certificate` (Ze's own trust-anchor rule). Each refusal names the exact leaf to set and the RFC reference | RFC 7296 Section 2.16 MUST; runtime backstop already exists in `computeServerAuth` |
| D-8 | Unknown user | The method answers an unknown name identically to a wrong credential: same wire packets, same EAP-Failure. The operator's log names the attempted name once and increments the `unknown-user` EAP refusal counter | Correctness: differing responses leak valid usernames |
| D-9 | Drop initial EAP Identity Request for `eap-mschapv2` and `eap-tls` | In `internal/core/eap/eap.go`, `NewSession` initializes `TypeMSCHAPv2` and `TypeTLS` in `stateMethod` and `Session.Begin` sets `identifier = 1` and returns `method.Start(1)` directly (MS-CHAPv2 Challenge or EAP-TLS Start), omitting the redundant EAP Identity Request (type 1). `TypeMD5Challenge` stays in `stateIdentity` and sends `TypeIdentity` first because `eap-md5` obtains the user name from the Identity Response (D-1). Ze's peer (`peer.go`) already accepts a method Request in `peerStateIdentity`; affected unit tests are updated cleanly, and RFC 3748 Type 1 tests exercise `TypeMD5Challenge` | OD-4 (2026-11-24); RFC 7296 Section 3.16 (`SHOULD NOT`) |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | A `PeerSession` built from the remote-access block drives the responder FSM unchanged | `newResponderSA`, `startResponderEAP` and `eapTLSServerConfig` read only `sa.PeerCfg` and the two groups; scenario `responder-eap-mschapv2` runs the same FSM | a parallel responder path, much larger | scenario `remote-access-eap-mschapv2` | unvalidated |
| A-2 | Road-warrior sessions can live outside the configured-peer removal loop of `reconcilePeersWith` and still be found by `lookupPeerSession`, `halfOpenResponderCount`, `PeerInfoMap`, `ActivePeers`, and the terminate functions | each reads under `peersMu` today | either reload stops every road warrior, or show, clear, and the cookie count miss them | `TestRemoteAccessSessionSurvivesUnrelatedReload`, `TestRemoteAccessSessionsVisible` | unvalidated |
| A-3 | A PKI certificate entry can hold a client certificate with no private key, so `eap-user certificate` can name one | `eapTLSServerConfig` checks `entry.PrivateKey == nil` itself, which implies the store allows it | the `eap-tls` pin of D-1 needs another carrier | a parse test of such an entry | unvalidated |
| A-4 | strongSwan as a client sends the MSCHAPv2 Name it is configured with, accepts an immediate MSCHAPv2 Challenge or EAP-TLS Start without an Identity Request (OD-4), and sends a `CFG_REQUEST` in `IKE_AUTH` | scenario `virtual-ip-assignment` passes with `vips = 0.0.0.0`; RFC 7296 Section 3.16 prescribes omitting the Identity Request | the interop scenarios need another client setting or expose a peer quirk | scenarios `remote-access-eap-mschapv2`, `responder-eap-mschapv2`, `responder-eap-tls13` | unvalidated |
| A-5 | strongSwan reports a refused road warrior in a form a check can read | no strongSwan source is in this checkout | the negative scenarios assert on Ze's side only | read the strongSwan behavior before each negative scenario is written (`ai/rules/interop-and-goal-validation.md`) | unvalidated |
| A-6 | A resumed EAP-TLS session exposes the client certificate the full handshake validated | comment on `resumptionFor`: `crypto/tls` carries the chain in the ticket | D-6 cannot re-check a resumed user and must refuse resumption for remote access | a resumption test over a removed user | unvalidated |
| A-7 | An `eap-md5` client sends the user name in its Identity Response | comment in `md5ChallengeMethod.Process` | D-1 for `eap-md5` must read the Response's Name field | unit/functional test with an `eap-md5` user | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Admission is a denial-of-service surface | goroutine or session count grows under a flood | D-4 / OD-2: stateless cookie first, non-disableable `half-open-limit` (64) and `half-open-limit-per-address` (8), reap at `responderHandshakeTimeout`; AC-3, AC-4 |
| R-2 | A session leaks when its SA dies in an unusual state | session count grows across connect and disconnect cycles | one removal point beside `forgetKeys`; AC-10 |
| R-3 | The credential lookup or OD-4 breaks the site-to-site EAP path | scenarios `responder-eap-mschapv2` or `responder-eap-tls13` go red | `MethodConfig.Password` is untouched when no lookup is set; re-prove all existing responder EAP scenarios against the shorter first round |
| R-4 | Multi-device user drains the pool or shares one inner IP across two devices | two devices of `alice` receive the same IP or exceed `max-sessions-per-user` | composite `(user, session)` active lease slots in `Registry.AcquireForUser` and hard refusal at `max-sessions-per-user`; AC-9 |
| R-5 | User A requests and steals user B's recently released IP or another user's static IP | `requestedLocked` hands out an IP not previously held by user A, or `scanLocked` hands out a static IP | `ownPriorOnly` check against `lastBy[addr].user == user` plus static-IP reservation table skipped by `scanLocked`; AC-18, AC-19 |
| R-6 | The lookup resolves from the Identity Response under MSCHAPv2 | none on the happy path: an honest client sends one name | AC-6 and OD-4 (no Identity Request is sent under MSCHAPv2 at all, and an unprompted Identity payload selects nothing) |
| R-7 | Reload stops every road warrior | every client reconnects after an unrelated commit | A-2; AC-12 |
| R-8 | The reaper races the dispatch goroutine | race reports in the engine tests | reuse the `ownedSA` discipline and `peersMu`; engine tests run under `-race` |
| R-9 | A road-warrior source later becomes a configured peer's address | client is served by the wrong block | matching order is fixed: configured peers first (AC-2) |
| R-10 | Two specs build two EAP-TLS allow-lists | `spec-eap-tls-identity-authorization` designs a conflicting server list | the `Depends` row; parallel design keeps `eap-user` as the remote-access user list while `spec-eap-tls-identity-authorization` exports the verified SAN/subject identity and handles site-to-site `allowed-identities` |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | An unauthorized client gets a tunnel, one user steals another's inner address, a multi-device user hits duplicate inner IPs or drains the pool, a flood denies IKE to configured peers, or site-to-site EAP handshakes break on the first EAP round. Nothing changes for a config with no `remote-access` block and no EAP peers |
| How is it reverted? | A commit revert. The new leaves are additive; a config that uses them stops parsing after a revert. Reverting OD-4 restores the initial Identity Request |
| Who else touches this path? | `spec-eap-tls-identity-authorization` (closed 2026-11-27; exported `Session.PeerID()`/`PeerIDs()` in `internal/core/eap/eap.go`), `spec-yang-single-source-of-truth` (owns the journal row on the inert subtree) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| IKE_SA_INIT from an unconfigured source, `remote-access` configured | → | the road-warrior arm of `tryResponderSAInit` | `test/ipsec/ipsec-remote-access-admits-unconfigured-source.ci` |
| MSCHAPv2 Response naming an `eap-user` (without initial Identity Request) | → | `Session.Begin` direct start + `MethodConfig` credential lookup | `TestRemoteAccessAuthenticatesTheNamedUser` |
| final IKE_AUTH of two concurrent devices of one user | → | `assignVirtualIP` -> `Registry.AcquireForUser` with `(user, session)` | `TestRemoteAccessMultiDeviceGetsDistinctLeases` |
| final IKE_AUTH of a user with `static-ip` / own prior IP | → | `assignVirtualIP` -> `Registry.AcquireForUser` + `requestedLocked` | `TestRemoteAccessStaticIPAndOwnPriorRequestOnly` |
| commit that removes an `eap-user` | → | the road-warrior reconcile | `test/ipsec/ipsec-remote-access-user-removed-ends-session.ci` |
| strongSwan road-warrior client | → | the whole path | interop scenario `remote-access-eap-mschapv2` |

## Acceptance Criteria

Numbering restarts here. The criteria the sibling took on 2026-10-26 are not
renumbered into this table.

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | `IKE_SA_INIT` with a valid cookie from an unconfigured source; `remote-access` configured | Ze answers `IKE_SA_INIT` and the handshake reaches `IKE_AUTH`; `show vpn ipsec sa` lists one road-warrior session | The same datagram with no `remote-access` block is still dropped and logged as unconfigured, because no service was configured |
| AC-2 | A source that matches a configured `respond` peer; `remote-access` also configured | The configured peer's session serves it, with that peer's credentials and selectors | None: a configured address is never served by the remote-access block |
| AC-3 | `IKE_SA_INIT` from an unconfigured source with no cookie, or a wrong one, at any `cookie-threshold` | Ze answers with a COOKIE notification, and the session count, `SATable` size, and goroutine count are unchanged | A wrong cookie is answered with a fresh one and is not rejected (`RFC7296-2.6-5`) |
| AC-4 | Verified initiations past `half-open-limit` (default 64), and past `half-open-limit-per-address` (default 8) from one address | Each initiation past a bound is dropped and counted, and `doctor-ipsec-remote-access-admission` reports the bound; initiations within both bounds proceed | A handshake already admitted finishes; a configured peer's handshake is outside both bounds |
| AC-5 | `eap-mschapv2`; the first responder `IKE_AUTH` response carries EAP-Request/MSCHAPv2 Challenge directly (no Identity Request, OD-4), and the Response names a configured `eap-user` and proves its password | EAP-Success, and the session reports that authenticated user | A site-to-site EAP peer keeps authenticating with its one `Password`, also without the initial Identity Request; `eap-md5` still sends the initial Identity Request and authenticates against the named `eap-user` |
| AC-6 | `eap-mschapv2`; client asserts `IDi = alice`, MS-CHAPv2 Response Name `bob`, proof of `bob`'s password | The session's authenticated user, show output, and lease owner user are `bob` | `IDi` may be any syntactically valid identity: it selects nothing and authorizes nothing |
| AC-7 | The Response names no `eap-user`, or names one and proves a wrong password | Both receive the identical failure packet sequence and EAP-Failure and no session survives; the operator's log names the attempt once and increments the matching refusal counter | None: no fallback credential is tried |
| AC-8 | Response Name `CORP\alice`, then a second session with Name `alice` | Both resolve to the `eap-user` entry `alice` for per-user session counting, static-IP lookup, and prior-IP history | A name with a realm suffix (`alice@example.com`) resolves to no entry and is refused, because the `eap-user name` leaf admits no `@` |
| AC-9 | One `eap-user` authenticates two concurrent devices (laptop + phone) under default `max-sessions-per-user` (4), then dials a 5th concurrent device past the limit | Devices 1..4 establish concurrently, each holding its own distinct inner IP from the pool and its own Child SA; device 5 is refused with `AUTHENTICATION_FAILED` while devices 1..4 stay up with their Child SAs intact | A site-to-site respond peer still supersedes its single session on authenticated re-initiation |
| AC-10 | N connect and disconnect cycles, and N handshakes abandoned before authentication | The road-warrior session count, goroutine count, and pool held count return to their base values | Sessions of configured peers are untouched |
| AC-11 | An authenticated road warrior sends no `CFG_REQUEST` | The final `IKE_AUTH` carries `FAILED_CP_REQUIRED` and no SA or TS payload; the IKE SA stays; no XFRM state or policy exists for it | A client that then sends a `CFG_REQUEST` in a new exchange is served |
| AC-12 | A commit removes one `eap-user`, or changes its password, certificate, `static-ip`, or `static-ip6`; or lowers `max-sessions-per-user` below a user's active count | That user's affected session(s) end with an IKE Delete and their leases are freed; every other user's sessions stay up with their Child SAs | A commit that changes the block's groups, authentication, pool binding, or selectors ends every road-warrior session, matching `peerConfigChanged` |
| AC-13 | A commit of a remote-access block with `eap-user` entries and no `authentication x509 certificate`, or `mode eap-tls` with no `ca-certificate`, or no `virtual-ip-pool`, or `half-open-limit 0`, or `half-open-limit-per-address 0`, or `max-sessions-per-user 0`, or `half-open-limit-per-address > half-open-limit`, or a `static-ip`/`static-ip6` outside the bound pool or duplicated across two users | Each invalid config is refused at `ze config validate` and commit, naming the offending leaf, why, and the exact fix (citing RFC 7296 Section 2.16 for the missing gateway certificate) | A block with pools only and no `eap-user` still commits: it serves pool-bound site-to-site peers |
| AC-14 | Two clients behind one NAT, each with its own user (or two devices of the same user) | Two sessions, two distinct leases, two Child SAs, traffic on both | None |
| AC-15 | The gateway certificate or CA of the block is missing or has no key in the live store; or an admission bound (`half-open-limit`, `half-open-limit-per-address`, or `max-sessions-per-user`) is reached | `ze show doctor` / `ze doctor` reports each under `doctor-ipsec-remote-access-credential` or `doctor-ipsec-remote-access-admission` with the command that fixes it | A healthy block reports nothing |
| AC-16 | strongSwan road warrior, `eap-mschapv2`, from an address Ze has no peer for | It establishes without an initial EAP Identity Request, holds an address of the bound pool, and traffic passes; Ze's XFRM policy names the lease as the remote selector | None |
| AC-17 | `eap-tls`; first responder EAP packet is EAP-TLS Start (no Identity Request, OD-4); a client whose chain validates and whose leaf certificate no `eap-user` names vs one whose leaf is named | Unlisted leaf gets EAP-Failure and no session; listed leaf is served as that `eap-user` | Uses `Session.PeerID()` / `Session.PeerIDs()` (`internal/core/eap/eap.go`, `docs/architecture/ike/ipsec-9-ikev2-eap-nat.md`, landed by `spec-eap-tls-identity-authorization`) |
| AC-18 | `eap-user alice` configures `static-ip 10.10.0.50` (and/or `static-ip6`) inside the bound pool | `alice` always receives `10.10.0.50` regardless of what address she requests; `bob` connecting while `alice` is offline never receives `10.10.0.50` from dynamic scan or explicit request; a second concurrent device of `alice` gets `INTERNAL_ADDRESS_FAILURE` while her first device keeps `10.10.0.50` | None: RFC 3948 Section 5.1 forbids two live SAs holding the same inner address |
| AC-19 | `eap-user bob` (no static IP) leases `10.10.0.2` and disconnects; `alice` connects and requests `10.10.0.2` (or requests an arbitrary unused pool IP `10.10.0.99`); then `bob` reconnects and requests `10.10.0.2` | `alice` does NOT get `10.10.0.2` or `10.10.0.99` on request — she gets a cursor-allocated IP; `bob` gets his own previous `10.10.0.2` back on reconnect | A site-to-site pool-bound respond peer keeps existing `Acquire` requested-IP behavior (except reserved static IPs) |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | A road warrior dials in with EAP-MSCHAPv2 (direct Challenge, no Identity Request) and gets its address | UDP, cookie, admission, direct EAP-MSCHAPv2, user lookup, lease, CFG_REPLY, Child SA | interop scenario `remote-access-eap-mschapv2` |
| 2 | A road warrior connects laptop and phone simultaneously under one `eap-user`, and a user with `static-ip` always gets their pinned IP | UDP, cookie, admission, multi-device `(user, session)` leases, static-IP reservation | interop scenario `remote-access-multi-device-and-static-ip` |
| 3 | A road warrior dials in with EAP-TLS (direct EAP-TLS Start, no Identity Request) | UDP, cookie, admission, direct EAP-TLS, certificate pin, lease, CFG_REPLY, Child SA | interop scenario `remote-access-eap-tls` |
| 4 | An operator removes a user and that user's tunnel(s) drop while other users stay up | commit, apply, road-warrior reconcile, IKE Delete | `test/ipsec/ipsec-remote-access-user-removed-ends-session.ci` |
| 5 | An operator commits a block with missing gateway cert, missing pool, zero limit, or invalid/duplicate static IP | commit, `validateIPsecSections`, refusal with fix | `test/reload/tx-ipsec-remote-access-requires-certificate.ci`, `test/reload/tx-ipsec-remote-access-requires-pool.ci`, `TestValidateRemoteAccessLimitsAndStaticIPs` |
| 6 | An operator asks who is connected | `show vpn ipsec sa` lists road-warrior sessions with `eap-user` and `virtual-ip` | `test/ipsec/ipsec-remote-access-admits-unconfigured-source.ci` |
| 7 | An operator asks why clients cannot connect | `ze show doctor` / `ze doctor` | `test/ipsec/ipsec-remote-access-doctor.ci` |

## 🧪 TDD Test Plan

### Unit Tests
Engine tests feed wire messages to `tryResponderSAInit` and the handlers it
reaches, so each one runs the real exchange.

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRemoteAccessAdmitsAfterCookie` | `internal/component/ike/engine/remote_access_test.go` | AC-1 | |
| `TestRemoteAccessNeverDivertsConfiguredPeer` | `internal/component/ike/engine/remote_access_test.go` | AC-2 | |
| `TestRemoteAccessCommitsNoStateBeforeCookie` | `internal/component/ike/engine/remote_access_test.go` | AC-3 | |
| `TestRemoteAccessHalfOpenBounds` | `internal/component/ike/engine/remote_access_test.go` | AC-4 (defaults 64/8 and configured limits) | |
| `TestSessionBeginSkipsIdentityForMSCHAPv2AndTLS` | `internal/core/eap/eap_user_test.go` | AC-5, AC-17, OD-4 (`eap-mschapv2` and `eap-tls` start with method Request; `eap-md5` keeps Identity Request) | |
| `TestRemoteAccessAuthenticatesTheNamedUser` | `internal/core/eap/eap_user_test.go` | AC-5 (`eap-mschapv2` and `eap-md5`) | |
| `TestRemoteAccessIdentityResponseSelectsNothing` | `internal/core/eap/eap_user_test.go` | AC-6 | |
| `TestRemoteAccessUnknownUserAnswersAsWrongPassword` | `internal/core/eap/eap_user_test.go` | AC-7 | |
| `TestRemoteAccessDomainPrefixIsOneOwner` | `internal/component/ike/engine/remote_access_test.go` | AC-8 | |
| `TestRemoteAccessLeaseOwnerIsTheAuthenticatedUser` | `internal/component/ike/engine/remote_access_test.go` | AC-6, AC-8 | |
| `TestRemoteAccessMultiDeviceGetsDistinctLeases` | `internal/component/ike/engine/remote_access_test.go` | AC-9, OD-1 (4 concurrent devices get 4 distinct IPs; 5th refused; existing 4 stay up) | |
| `TestRemoteAccessNoSessionLeak` | `internal/component/ike/engine/remote_access_test.go` | AC-10 | |
| `TestRemoteAccessRequiresConfigurationPayload` | `internal/component/ike/engine/remote_access_test.go` | AC-11 | |
| `TestRemoteAccessUserRemovedEndsSession` | `internal/component/ike/engine/remote_access_test.go` | AC-12 (removal, password change, static-IP change, lowered `max-sessions-per-user`) | |
| `TestRemoteAccessSessionSurvivesUnrelatedReload` | `internal/component/ike/engine/remote_access_test.go` | AC-12, A-2 | |
| `TestValidateRemoteAccessGateway` | `internal/component/ike/ipsec/validate_test.go` | AC-13 (missing cert, missing CA for eap-tls, missing pool) | |
| `TestValidateRemoteAccessLimitsAndStaticIPs` | `internal/component/ike/ipsec/validate_test.go` | AC-13, OD-1, OD-2, OD-3 (zero limits, per-address > total, out-of-pool / network / broadcast / duplicate static IP) | |
| `TestRemoteAccessTwoClientsOneAddress` | `internal/component/ike/engine/remote_access_test.go` | AC-14 | |
| `TestRemoteAccessSessionsVisible` | `internal/component/ike/engine/remote_access_test.go` | story 6, A-2 | |
| `TestRemoteAccessDoctor` | `internal/component/ike/engine/doctor_remote_access_test.go` | AC-15 | |
| `TestRemoteAccessEAPTLSUnlistedCertificateRefused` | `internal/component/ike/engine/remote_access_test.go` | AC-17 | |
| `TestRegistryRoadWarriorStaticIPReservedAndPinned` | `internal/core/eap/pool_test.go` | AC-18, OD-3 | |
| `TestRegistryRoadWarriorRequestedIPRestrictedToOwnPrior` | `internal/core/eap/pool_test.go` | AC-19, OD-3 (user A cannot steal user B's released IP or arbitrary free IP; user B reclaims own prior IP) | |
| `TestRemoteAccessStaticIPAndOwnPriorRequestOnly` | `internal/component/ike/engine/remote_access_test.go` | AC-18, AC-19 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `half-open-limit` | `1..4096` (default `64`) | `4096` | `0` | `4097` |
| `half-open-limit-per-address` | `1..256` (default `8`, also `<= half-open-limit`) | `256` (when `half-open-limit >= 256`) | `0` | `257` (and `half-open-limit + 1`) |
| `max-sessions-per-user` | `1..64` (default `4`) | `64` | `0` | `65` |

### Functional Tests
Each drives a second `ze` as the client. That client sends no `CFG_REQUEST`, so
these prove admission, authentication, refusal and teardown, and the address
proofs are the interop scenarios.

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ipsec-remote-access-admits-unconfigured-source` | `test/ipsec/ipsec-remote-access-admits-unconfigured-source.ci` | a client Ze has no peer for authenticates as a user and is listed by `show vpn ipsec sa` with `eap-user`; with the block removed the same client is dropped | |
| `ipsec-remote-access-unknown-user-refused` | `test/ipsec/ipsec-remote-access-unknown-user-refused.ci` | a client naming no `eap-user`, and one with a wrong password, get no IKE SA | |
| `ipsec-remote-access-cp-required` | `test/ipsec/ipsec-remote-access-cp-required.ci` | an authenticated client with no `CFG_REQUEST` keeps its IKE SA and gets no Child SA (AC-11) | |
| `ipsec-remote-access-user-removed-ends-session` | `test/ipsec/ipsec-remote-access-user-removed-ends-session.ci` | a commit that removes the user ends its IKE SA(s); a second user's stays | |
| `ipsec-remote-access-doctor` | `test/ipsec/ipsec-remote-access-doctor.ci` | `ze show doctor` names the missing gateway certificate and its fix under `doctor-ipsec-remote-access-credential` | |
| `tx-ipsec-remote-access-requires-certificate` | `test/reload/tx-ipsec-remote-access-requires-certificate.ci` | a commit of a block with no gateway certificate is refused | |
| `tx-ipsec-remote-access-requires-pool` | `test/reload/tx-ipsec-remote-access-requires-pool.ci` | a commit of a block with users and no `virtual-ip-pool` (or zero limit / duplicate static IP) is refused | |

### Interop Tests (Scope: protocol)
Named scenarios, no numeric prefix. Each negative is proved red first by
removing the refusal it tests. Before a check expects strongSwan to print or
send something, the strongSwan behavior is verified (A-5). Existing responder
EAP scenarios (`responder-eap-mschapv2`, `responder-eap-tls13`,
`responder-eap-tls13-leaf-only-client`, `responder-eap-tls13-revoked-client`,
`responder-eap-survives-unprotected-auth`, `virtual-ip-assignment`,
`virtual-ip-uniqueness`) are also re-proved under OD-4.

| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `remote-access-eap-mschapv2` | `test/interop-ipsec/scenarios/remote-access-eap-mschapv2/` | strongSwan client | AC-16, OD-4: an unconfigured source establishes without an initial Identity Request, holds a pool address, passes traffic, and Ze's XFRM policy names the lease | |
| `remote-access-multi-device-and-static-ip` | `test/interop-ipsec/scenarios/remote-access-multi-device-and-static-ip/` | strongSwan clients | AC-9, AC-18, AC-19: one user connects two devices concurrently with two distinct leases and traffic on both; a user with `static-ip` receives its pinned IP | |
| `remote-access-eap-tls` | `test/interop-ipsec/scenarios/remote-access-eap-tls/` | strongSwan client | story 3, OD-4: direct EAP-TLS Start, certificate pin, pool lease, traffic | |
| `remote-access-wrong-password-refused` | `test/interop-ipsec/scenarios/remote-access-wrong-password-refused/` | strongSwan client | AC-7: no IKE SA, no lease, no XFRM state on Ze | |
| `remote-access-unknown-user-refused` | `test/interop-ipsec/scenarios/remote-access-unknown-user-refused/` | strongSwan client | AC-7: the same outcome for a name no `eap-user` carries | |
| `remote-access-eap-tls-unlisted-certificate-refused` | `test/interop-ipsec/scenarios/remote-access-eap-tls-unlisted-certificate-refused/` | strongSwan client | AC-17: a certificate the CA issued and no user names is refused | |
| `remote-access-eap-tls-revoked-client-refused` | `test/interop-ipsec/scenarios/remote-access-eap-tls-revoked-client-refused/` | strongSwan client | a listed and revoked client is refused: the block inherits the CRL wiring of `eapTLSServerConfig` | |
| `remote-access-two-clients-one-nat` | `test/interop-ipsec/scenarios/remote-access-two-clients-one-nat/` | two strongSwan clients behind one NAT | AC-14 | |

## Files to Modify
- `internal/component/ike/engine/register.go` - `tryResponderSAInit` road-warrior arm; session lookups (`PeerInfoMap`, `ActivePeers`, `lookupPeerSession`, `TerminatePeerSA`, `TerminateAllSAs`)
- `internal/component/ike/engine/cookie.go` - stateless gate for a source with no session; `halfOpenResponderCount` includes road-warrior sessions
- `internal/component/ike/engine/reconcile.go` - road-warrior sessions outside the configured-peer removal loop; `PeerInfo` `EAPUser` field; road-warrior reconcile
- `internal/component/ike/engine/apply.go` - road-warrior reconcile and static-IP reservation sync on apply
- `internal/component/ike/engine/cp.go` - `assignVirtualIP` passes road-warrior `(user, session)` and optional static IP to `Registry.AcquireForUser`
- `internal/component/ike/engine/virtual_ip.go` - `virtualIPState.reload` carries configured `eap-user` static-IP reservations onto the bound pool
- `internal/component/ike/engine/responder_eap.go` - `eapMethodConfig` builds the per-user lookup; `handleResponderEAP` records the authenticated user and enforces `max-sessions-per-user`
- `internal/component/ike/engine/resumption.go` - the remote-access block's shared EAP-TLS resumption store
- `internal/component/ike/engine/config.go` - `validateIPsecSections` invokes remote-access gateway, pool, limit, and static-IP validations
- `internal/component/ike/engine/doctor_runtime.go` - register the two runtime remote-access doctor checks in `engineDoctorChecks`
- `internal/component/ike/engine/register_doctor_codes.go` - register `doctor-ipsec-remote-access-credential` and `doctor-ipsec-remote-access-admission`
- `internal/component/ike/engine/metrics.go` - remote-access session, half-open, admission-drop, and EAP-refusal metrics
- `internal/component/ike/cmd/show_ipsec.go` - report road-warrior sessions and `eap-user` in `show vpn ipsec sa` and `show vpn ipsec status`
- `internal/core/eap/eap.go` - `MethodConfig` credential lookup; authenticated user accessor; `Session.Begin` skips Identity Request for `eap-mschapv2` and `eap-tls` while keeping it for `eap-md5` (OD-4)
- `internal/core/eap/eap_mschapv2.go`, `internal/core/eap/eap_md5challenge.go`, `internal/core/eap/eap_tls.go` - each method resolves its credential by D-1 and reports its authenticated user
- `internal/core/eap/pool.go` - composite `(user, session)` active lease slots (OD-1), static-IP reservations, and `requestedLocked` restricted to the user's own previous lease for road warriors (OD-3)
- `internal/component/ike/ipsec/types.go`, `internal/component/ike/ipsec/config.go`, `internal/component/ike/ipsec/validate.go` - new leaves (`virtual-ip-pool`, `traffic-selector`, `half-open-limit`, `half-open-limit-per-address`, `max-sessions-per-user`, `eap-user static-ip`, `eap-user static-ip6`), `remoteAccessEqual`, and commit refusals (D-3, D-7, OD-1, OD-2, OD-3)
- `internal/component/ike/ipsec/yang/ze-ipsec-conf.yang` - new leaves and rewritten descriptions under `container remote-access` and `list eap-user`
- `internal/core/eap/eap_test.go` (and other `internal/core/eap/*_test.go` / `internal/component/ike/engine/*_test.go` files calling `Session.Begin()` for `eap-mschapv2` or `eap-tls`) - update cleanly for OD-4 while keeping RFC 3748 Type 1 tests on `TypeMD5Challenge`
- `test/parse/ipsec-remote-access.ci` - add `virtual-ip-pool roadwarrior` required by D-3
- `docs/architecture/ike/ipsec-14-responder.md` - road-warrior admission, multi-device sessions, shared resumption store
- `docs/architecture/ike/ipsec-9-ikev2-eap-nat.md` - per-user lookup, composite `(user, session)` lease slots, static IP reservations, own-prior-only requested IP for road warriors, and OD-4 direct EAP start
- `docs/architecture/ike/ipsec-7-ikev2-engine.md` - road-warrior admission arm, unconditional cookie gate, half-open bounds, reconcile
- `docs/architecture/ike/ipsec-3-data-model.md` - new `remote-access` and `eap-user` leaves and commit validations
- `docs/architecture/ike/ipsec-10-cli-diag.md` - `show vpn ipsec` road-warrior rows, `eap-user` field, and new Prometheus metrics
- `docs/architecture/doctor-and-health-checks.md` - add the two new runtime `ike` doctor checks to the plugin table
- `docs/guide/ipsec.md` - replace the sentence stating `eap-user` authenticates nobody and document road-warrior remote access, limits, multi-device, and static IPs

## Files to Create
- `internal/component/ike/engine/remote_access.go` and `internal/component/ike/engine/remote_access_test.go`
- `internal/component/ike/engine/doctor_remote_access.go` and `internal/component/ike/engine/doctor_remote_access_test.go`
- `internal/core/eap/eap_user_test.go` <!-- doc-links: ignore (a file this spec plans and has not written) -->
- `test/ipsec/ipsec-remote-access-admits-unconfigured-source.ci`, `test/ipsec/ipsec-remote-access-unknown-user-refused.ci`, `test/ipsec/ipsec-remote-access-cp-required.ci`, `test/ipsec/ipsec-remote-access-user-removed-ends-session.ci`, `test/ipsec/ipsec-remote-access-doctor.ci`, `test/reload/tx-ipsec-remote-access-requires-certificate.ci`, `test/reload/tx-ipsec-remote-access-requires-pool.ci` <!-- doc-links: ignore (planned tests; spec is in design) -->
- `test/interop-ipsec/scenarios/remote-access-eap-mschapv2/`, `test/interop-ipsec/scenarios/remote-access-multi-device-and-static-ip/`, `test/interop-ipsec/scenarios/remote-access-eap-tls/`, `test/interop-ipsec/scenarios/remote-access-wrong-password-refused/`, `test/interop-ipsec/scenarios/remote-access-unknown-user-refused/`, `test/interop-ipsec/scenarios/remote-access-eap-tls-unlisted-certificate-refused/`, `test/interop-ipsec/scenarios/remote-access-eap-tls-revoked-client-refused/`, `test/interop-ipsec/scenarios/remote-access-two-clients-one-nat/` <!-- doc-links: ignore (planned interop scenarios; spec is in design) -->

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `ze-ipsec-conf.yang`: `remote-access virtual-ip-pool`, `traffic-selector`, `half-open-limit`, `half-open-limit-per-address`, `max-sessions-per-user`; `eap-user static-ip`, `static-ip6` |
| YANG validation constraints | Yes | `range "1..4096"` (default `64`) on `half-open-limit`, `range "1..256"` (default `8`) on `half-open-limit-per-address`, `range "1..64"` (default `4`) on `max-sessions-per-user`; `zt:ipv4-address` on `static-ip`, `zt:ipv6-address` on `static-ip6`; `virtual-ip-pool` typed as peer leaf |
| YANG custom validators | No | pool coverage, limit cross-check (`per-address <= total`), static-IP in-pool/uniqueness, and gateway-certificate presence checks run in plugin verify (`validateIPsecSections`), matching peer validation |
| CLI commands/flags | No | no new command; `show vpn ipsec sa` gains road-warrior rows and `eap-user` field |
| CLI grammar (keyword before value) | N-A | no command added |
| Editor autocomplete | Yes | automatic for the typed leaves |
| Functional test for new RPC/API | N-A | no new RPC |
| Pipe completeness | N-A | no new output path (`show vpn ipsec sa` already routes through pipes) |
| Env var registration | No | all tunables are YANG leaves |
| Doctor check for runtime dependencies | Yes | gateway certificate, its key, and CA in the live PKI store (`doctor_remote_access.go`, declared in `engineDoctorChecks`) |
| Doctor code per problem class | Yes | `doctor-ipsec-remote-access-credential` (gateway certificate, key, or CA unusable; runtime phase), `doctor-ipsec-remote-access-admission` (a half-open or per-user session bound reached and initiations/sessions dropped; runtime phase), both registered in `engine/register_doctor_codes.go`. Pool exhaustion stays `doctor-ipsec-virtual-ip-pool` |
| Prometheus counters/metrics | Yes | `ze_ipsec_remote_access_sessions` (gauge), `ze_ipsec_remote_access_half_open` (gauge), `ze_ipsec_remote_access_drops_total{bound}` (gauge-backed cumulative counter matching `IPsecMetrics` style; labels `half_open_limit`, `half_open_per_address`, `max_sessions_per_user`), `ze_ipsec_remote_access_eap_refusals_total{reason}` (labels `unknown_user`, `wrong_credential`) |
| BGP family surface | N-A | IPsec/IKE subsystem only |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `features/ipsec-eap-authentication.md` |
| 2 | Config syntax changed? | Yes | `docs/guide/ipsec.md`, the remote-access section |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md`, the `show vpn ipsec sa` fields |
| 4 | API/RPC added/changed? | No | no RPC added or changed |
| 5 | Plugin added/changed? | No | `ike` plugin registration roots unchanged |
| 6 | Has a user guide page? | Yes | `docs/guide/ipsec.md` |
| 7 | Wire format changed? | No | no IKE payload binary format changes; OD-4 drops an optional EAP round |
| 8 | Plugin SDK/protocol changed? | No | uses existing SDK callbacks and `OnDoctorCheck` |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc7296.md` (Sections 2.16 and 3.16), `rfc/short/rfc5216.md` (`RFC5216-5.3-2`, with the Depends spec), `docs/features/rfc-status.md` |
| 10 | Test infrastructure changed? | Yes | `docs/architecture/testing/interop.md`, the new scenarios |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md`, the remote-access VPN row |
| 12 | Internal architecture changed? | Yes | `docs/architecture/ike/ipsec-14-responder.md`, `ipsec-9-ikev2-eap-nat.md`, `ipsec-7-ikev2-engine.md`, `ipsec-3-data-model.md`, `ipsec-10-cli-diag.md`, `docs/architecture/doctor-and-health-checks.md` |
| 13 | Route metadata keys? | No | no BGP/route metadata changes |
| 14 | Prometheus counters? | Yes | `docs/architecture/ike/ipsec-10-cli-diag.md` |
| 15 | Registered inventory changed? | Yes | `docs/architecture/doctor-and-health-checks.md` (the two runtime `ike` doctor checks) |
| 16 | Doc source anchors on changed files? | Yes | verified at implementation with `./le spec citation anchors spec plan/release/fixes/spec-ipsec-remote-access.md` |
| 17 | Existing docs show examples? | Yes | every remote-access example in `docs/guide/ipsec.md` |

## Design presentation

### What changes

`vpn ipsec remote-access` becomes a live road-warrior VPN gateway. Clients from
unconfigured source addresses can dial in over IKEv2 behind an unconditional
stateless COOKIE challenge and non-disableable half-open handshake bounds
(`half-open-limit` default 64, `half-open-limit-per-address` default 8). Each
client authenticates against an `eap-user` entry (with `eap-mschapv2` and
`eap-tls` skipping the redundant initial EAP Identity Request per RFC 7296
Section 3.16, and `eap-md5` keeping it to identify the user). One user can hold
up to `max-sessions-per-user` (default 4) concurrent device tunnels (e.g. laptop
+ phone), each leased its own unique inner address from `remote-access
virtual-ip-pool`. An `eap-user` may optionally pin a `static-ip` and/or
`static-ip6` reserved inside that pool; otherwise a client's requested IP is
honored only when it was that user's own previous lease. Removing or editing a
user disconnects their active tunnel(s) on commit, and `show vpn ipsec sa` and
`ze show doctor` surface active road-warrior sessions, credential faults, and
admission-bound hits.

### Before and after

1. **Road-warrior dial-in from an unconfigured address:**
   - Before: `tryResponderSAInit` logs `unsolicited IKE_SA_INIT from unconfigured source` and drops the packet; `eap-user` entries are never read outside parse/validate.
   - After: Ze challenges the unconfigured source with a stateless COOKIE, enforces `half-open-limit` (64) and `half-open-limit-per-address` (8), authenticates the client against `eap-user`, leases a unique virtual IP from `virtual-ip-pool`, installs the narrowed Child SA, and lists the session in `show vpn ipsec sa` with its `eap-user`.

2. **Multi-device user connecting laptop + phone concurrently (OD-1):**
   - Before: a second connection for one owner shares the same lease slot or supersedes the first tunnel.
   - After: `alice` connects laptop and phone concurrently under `max-sessions-per-user` (default 4); each gets its own distinct inner IP and Child SA; a 5th concurrent connection is refused with `AUTHENTICATION_FAILED` while all 4 live sessions stay up.

3. **Client-requested IP and per-user static IP (OD-3):**
   - Before: `Registry.requestedLocked` granted any free in-pool IP a client asked for, allowing user A to claim user B's recently released IP; no per-user static IP leaf existed.
   - After: an `eap-user` with `static-ip`/`static-ip6` always receives that reserved in-pool address (and no other user can lease it); without a static IP, a road warrior's requested IP is honored only if it was that exact user's own previous lease.

4. **First EAP round on the wire (OD-4):**
   - Before: `Session.Begin()` sent an EAP Identity Request (type 1) on every `eap-mschapv2`, `eap-tls`, and `eap-md5` exchange, adding one round trip against RFC 7296 Section 3.16 (`SHOULD NOT`).
   - After: `eap-mschapv2` starts directly with MS-CHAPv2 Challenge (type 26) and `eap-tls` starts directly with EAP-TLS Start (type 13); only `eap-md5` still sends the initial Identity Request.

5. **Commit validation of incomplete remote-access blocks (D-3, D-7, OD-2, OD-3):**
   - Before: `ze config validate` accepted `remote-access` with `eap-user` entries, no gateway certificate, and no `virtual-ip-pool`.
   - After: commit refuses missing `authentication x509 certificate` (citing RFC 7296 Section 2.16), missing `ca-certificate` under `eap-tls`, missing `virtual-ip-pool` when users exist, zero limits, `half-open-limit-per-address > half-open-limit`, and out-of-pool or duplicate per-user static IPs, naming the leaf and fix.

### Alternatives set aside

| Alternative | Why set aside |
|-------------|---------------|
| Single session per `eap-user` with supersede on reconnect | Set aside by OD-1 (2026-11-24): prevents a user from running laptop and phone concurrently. |
| Allowing `0` on `half-open-limit` or `half-open-limit-per-address` to mean unlimited | Set aside by OD-2 (2026-11-24): an unconfigured-source responder must never be left with an unbounded half-open table. |
| Keeping unrestricted requested-IP acceptance in `Registry.requestedLocked` for road warriors | Set aside by OD-3 (2026-11-24): lets user A request user B's recently released IP (breaking IP-based firewall trust) and offers no per-user static IP pinning. |
| Leaving the initial EAP Identity Request in place for `eap-mschapv2` and `eap-tls` | Set aside by OD-4 (2026-11-24): violates RFC 7296 Section 3.16 (`SHOULD NOT`) and wastes a round trip on every handshake. |
| A separate road-warrior FSM or second IP allocator beside `PeerSession` and `eap.Registry` | Duplicates DPD, rekey, teardown, and uniqueness enforcement; rejected in favor of extending `PeerSession` and `eap.Registry`. |
| Resolving `eap-user` from `IDi` or the EAP Identity Response under `eap-mschapv2`/`eap-tls` | Violates RFC 7296 Section 2.16 and RFC 3748 Section 5.1: `IDi` and Identity Response are unauthenticated routing hints, whereas MS-CHAPv2 hashes its own Name field and EAP-TLS authenticates the client certificate. |

### Documents departed from

1. **`docs/architecture/ike/ipsec-7-ikev2-engine.md` (`Decisions`)**
   - Current statement: `"A responder inserts one only when tryResponderSAInit admits a received request from a configured peer."`
   - After this work: `tryResponderSAInit` admits a received `IKE_SA_INIT` from a configured peer first (unchanged), and when no configured peer matches and `vpn ipsec remote-access` is configured, admits an unconfigured source after an unconditional stateless COOKIE verification and within `half-open-limit` (default 64) and `half-open-limit-per-address` (default 8) (owner ruling 2026-07-23, OD-2 2026-11-24).

2. **`docs/architecture/ike/ipsec-9-ikev2-eap-nat.md` (`Decisions`)**
   - Current statement: `"An owner (the peer) holds at most one address per pool and family. A second Acquire for the same slot returns the same lease with one more holder, which is what reauthentication and a parallel re-initiation need. Otherwise the order is: the requested address when it is a free host address of the range (for IPv6 outside the prefix, its interface identifier placed inside the prefix); the owner's last released address while it is still free"`
   - After this work: site-to-site respond peers keep single-slot leases and existing requested-IP semantics (skipping reserved static IPs); road-warrior `eap-user` leases key active slots on `(user, session, pool, family)` up to `max-sessions-per-user` (default 4, OD-1), pin configured per-user `static-ip`/`static-ip6` addresses reserved against dynamic scan, and honor a client-requested IP only when `lastBy` records it as that exact `user`'s own previously released lease (OD-3). Additionally, `Session.Begin` omits the initial EAP Identity Request for `eap-mschapv2` and `eap-tls` per RFC 7296 Section 3.16 while keeping it for `eap-md5` (OD-4).

3. **`docs/architecture/ike/ipsec-14-responder.md` (`Decisions`)**
   - Current statements:
     - `"The engine holds one eap.Resumption per configured peer, keyed by peer name, and eapTLSServerConfig REFUSES an SA that reached it without one rather than falling back to that dead key."`
     - `"no CLI command prints DidResume, and show vpn ipsec sa carries no EAP field."`
     - `"The responder SA is created by the dispatch goroutine on an unsolicited IKE_SA_INIT request from a peer configured to respond, advanced inline on that goroutine, and adopted into the owner loop by a polling runResponder at establishment."`
     - `"The session outlives the SA -- ps.run loops runOnce on one PeerSession per peer --"`
   - After this work: site-to-site respond peers keep one long-lived looping `PeerSession` and one `eap.Resumption` store per configured peer name, while `vpn ipsec remote-access` admits unsolicited `IKE_SA_INIT` from unconfigured sources into ephemeral per-initiation road-warrior `PeerSession` instances (`rw-<id>`) whose owner goroutine exits when the SA dies, shares one `eap.Resumption` store across the `remote-access` block (reset on auth edit), and adds `eap-user` to `show vpn ipsec sa` (`DidResume` still stays out of CLI output).

### Choices for Thomas

none

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** - the road-warrior arm, unconditional stateless cookie gate, one `PeerSession` per verified initiation with unique session discriminator
   - Tests: `ipsec-remote-access-admits-unconfigured-source.ci`, `TestRemoteAccessAdmitsAfterCookie`, `TestRemoteAccessCommitsNoStateBeforeCookie`, `TestRemoteAccessNeverDivertsConfiguredPeer`
   - Files: `internal/component/ike/engine/remote_access.go`, `register.go`, `cookie.go`
   - Verify: the wiring `.ci` is red because no user authenticates yet
2. **Phase: Config** - add `virtual-ip-pool`, `traffic-selector`, `half-open-limit`, `half-open-limit-per-address`, `max-sessions-per-user`, `eap-user static-ip`, `eap-user static-ip6`, rewrite inert YANG descriptions, implement `remoteAccessEqual` and D-3/D-7/OD-1/OD-2/OD-3 commit refusals, update `test/parse/ipsec-remote-access.ci`
   - Tests: `TestValidateRemoteAccessGateway`, `TestValidateRemoteAccessLimitsAndStaticIPs`, `tx-ipsec-remote-access-requires-certificate.ci`, `tx-ipsec-remote-access-requires-pool.ci`
3. **Phase: Direct EAP start & per-user credentials** - OD-4 (`Session.Begin` omits initial Identity Request for `eap-mschapv2` and `eap-tls`, keeps it for `eap-md5`), update affected existing EAP unit tests cleanly, implement D-1, D-2, D-8 credential lookup for `eap-mschapv2` and `eap-md5`
   - Tests: `TestSessionBeginSkipsIdentityForMSCHAPv2AndTLS`, `eap/eap_user_test.go`, `TestRemoteAccessLeaseOwnerIsTheAuthenticatedUser`, `TestRemoteAccessDomainPrefixIsOneOwner`
   - Verify: existing `internal/core/eap` and `internal/component/ike/engine` unit tests pass under `-race`
4. **Phase: Multi-device leases, static IPs, own-prior-only requests, bounds & lifecycle** - OD-1 composite `(user, session)` slots and `max-sessions-per-user`, OD-2 half-open bounds (64 / 8), OD-3 static-IP reservations and `requestedLocked` own-prior-only check, reaping, and D-6 reload reconcile
   - Tests: `TestRegistryRoadWarriorStaticIPReservedAndPinned`, `TestRegistryRoadWarriorRequestedIPRestrictedToOwnPrior`, `TestRemoteAccessHalfOpenBounds`, `TestRemoteAccessMultiDeviceGetsDistinctLeases`, `TestRemoteAccessStaticIPAndOwnPriorRequestOnly`, `TestRemoteAccessNoSessionLeak`, `TestRemoteAccessUserRemovedEndsSession`, `TestRemoteAccessSessionSurvivesUnrelatedReload`, `TestRemoteAccessTwoClientsOneAddress`
5. **Phase: EAP-TLS** - certificate pin and resumption user re-check, integrating the verified identity exported via `Session.PeerID()` / `Session.PeerIDs()` (`internal/core/eap/eap.go`, `spec-eap-tls-identity-authorization`)
   - Tests: `TestRemoteAccessEAPTLSUnlistedCertificateRefused`
6. **Phase: Visibility** - `show vpn ipsec sa` `eap-user` field, runtime doctor checks in `doctor_runtime.go` / `doctor_remote_access.go` and codes in `register_doctor_codes.go`, Prometheus metrics in `metrics.go`
   - Tests: `TestRemoteAccessSessionsVisible`, `TestRemoteAccessDoctor`, `ipsec-remote-access-doctor.ci`
7. **Phase: Interop** - the eight road-warrior scenarios (each negative proved red first) plus re-proving existing responder EAP scenarios under OD-4
8. **Phase: Documentation** - every Documentation Update Checklist row, updating each architecture page and guide in the phase that changes its behavior

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1..AC-19 has an implementation at a named function |
| Fail-closed | An unknown user, a lookup error, an empty user table, a nil lookup, a session with no authenticated user, a zero limit, and an excess session past `max-sessions-per-user` each deny |
| No state before the cookie | Nothing is allocated, stored, or logged at Warn for an unverified source |
| Identity | No policy decision reads `IDi`, or the Identity Response under `eap-mschapv2` or `eap-tls` |
| Lease isolation (OD-1, OD-3) | Two concurrent devices of one user never share an inner IP; no user can claim another user's recently released IP or static IP |
| Site-to-site untouched | Every existing site-to-site scenario and `.ci` green; AC-2 |
| Concurrency | Engine and EAP package tests pass under `-race` |
| Mutation | Each new guard removed in turn turns its test red |
| Operator errors and doctor codes | Every refusal names the leaf, why, and the fix; both runtime doctor checks live in `engineDoctorChecks` and both codes in `register_doctor_codes.go` |
| Rule: no-layering | The YANG descriptions that say "no code reads this" are rewritten, not kept beside the new text |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| `eap-user` has a handshake reader | references on `EAPUser.Password`, `EAPUser.Certificate`, `EAPUser.StaticIP`, `EAPUser.StaticIP6` list sites outside parse, validation, and equality |
| Multi-device and static-IP / own-prior-only leases work | `TestRemoteAccessMultiDeviceGetsDistinctLeases`, `TestRemoteAccessStaticIPAndOwnPriorRequestOnly` |
| Initial Identity Request omitted for `eap-mschapv2`/`eap-tls`, kept for `eap-md5` | `TestSessionBeginSkipsIdentityForMSCHAPv2AndTLS` |
| No state before the cookie | `TestRemoteAccessCommitsNoStateBeforeCookie` |
| A removed or edited user is disconnected | `ipsec-remote-access-user-removed-ends-session.ci` |
| Road warrior against strongSwan | `./le test integration interop-ipsec` on the eight road-warrior scenarios and re-proved responder EAP scenarios, run by the main thread |
| The YANG says what the code does | no `remote-access` description contains "no code reads", "not served yet", or "has no effect" |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Unauthenticated resource use | D-4 / OD-2: stateless cookie always, non-disableable 64 / 8 bounds; no lease before EAP success (`assignVirtualIP` runs after authentication) |
| Credential handling | `eap-user` passwords never logged and never serialized |
| User enumeration | D-8: identical wire packets and EAP-Failure for an unknown user and a wrong password |
| Identity spoofing | D-1, D-2; AC-6, AC-8 |
| Inner address choice & pool drain | D-2 (`max-sessions-per-user` cap), D-3 / OD-3 (static IP reservation + own-prior-only requested IP, never another user's released IP) |
| Client chain validation | the block's `ca-certificate`, CRLs, and intermediates are those `eapTLSServerConfig` reads |
| Stale authorization | D-6, including EAP-TLS resumption and lowered `max-sessions-per-user` |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Scenario `responder-eap-mschapv2` or `responder-eap-tls13` regresses | check OD-4 direct start or lookup additivity: phase 3 |
| Reload stops road warriors | A-2: phase 4 |
| A negative interop scenario cannot go red | the check reads the client and not Ze: rewrite the check on Ze's state |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- A component with no consumer has no evidence of correctness. The codec and
  the pool both looked complete until the sibling spec gave them a caller and
  found the reserved-bit fold and the IPv6 out-of-range lease.
- An owner key is an authorization input. `sa.PeerName` is safe for a
  configured peer because the operator wrote it. For multi-device road warriors,
  separating the authorization identity (`user`) from the active lease slot
  discriminator (`session`) lets one user hold concurrent device leases (OD-1)
  while keeping released-address reuse strictly scoped to that user (OD-3).
- A gate that needs a session cannot protect the creation of sessions.

## Owned journal rows

None. The open row on the inert `remote-access` subtree
(`plan/journal/unwired-feature.md`, 2026-09-04) is owned by
`plan/release/fixes/spec-yang-single-source-of-truth.md`. This spec's work
ends its remote-access half.

| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| One `PeerSession` per road-warrior handshake with unique session discriminator, up to `max-sessions-per-user` (default 4) per user | single session per user with supersede; one shared session; parallel road-warrior FSM | OD-1: allows concurrent laptop + phone sessions, reuses DPD/rekey/teardown machinery, and keeps each client's half-open slot and inner IP distinct |
| Composite `(user, session)` active lease slot with per-user `lastBy` history and static-IP reservations | single slot per user; honoring any free requested IP | OD-1 + OD-3: gives concurrent devices distinct IPs, pins static IPs safely, lets a user reclaim their own previous IP on reconnect, and blocks cross-user IP theft |
| Non-disableable half-open limits (`64` total, `8` per source address, `0` refused) behind an unconditional stateless cookie | following `cookie-threshold`; allowing `0` = unlimited | OD-2: prevents spoofed floods and NAT-burst exhaustion on an unconfigured-source responder |
| Omit initial EAP Identity Request in `Session.Begin` for `eap-mschapv2` and `eap-tls`; keep it for `eap-md5` | keep Identity Request for all methods; drop for `eap-md5` too | OD-4 / RFC 7296 Section 3.16 (`SHOULD NOT`): saves a round trip on `eap-mschapv2` and `eap-tls` while keeping `eap-md5` user selection working |
| The method resolves the credential by the name it authenticates | resolve from `IDi` or Identity Response before the method starts | RFC 3748 Section 5.1 and RFC 7296 Section 2.16: `IDi` and Identity Response are unauthenticated routing hints |
| Pool binding makes CP mandatory | a `configuration-payload required` leaf | `assignVirtualIP` already refuses a bound peer with no request; a second switch could disagree with it |
| Lease only after EAP success | lease at admission | an unauthenticated source must not hold an address |

## Known Limitations
- An `eap-user` name cannot carry a realm, because the leaf's character set admits no `@`. A client that sends `alice@example.com` is refused.
- A `.ci` cannot prove address assignment, because Ze is not an IRAC (ruling of 2026-08-31); strongSwan interop scenarios and engine tests provide the address proofs.
- An `eap-user` with a configured `static-ip` (or `static-ip6`) can hold that static address on at most one live session at a time (RFC 3948 Section 5.1 forbids duplicate inner addresses across concurrent Child SAs); a second concurrent device of that static-IP user receives `INTERNAL_ADDRESS_FAILURE`.

## RFC Documentation (Scope: protocol)

Add verbatim `// RFC NNNN Section X.Y` quotes above enforcing code: RFC 7296
Section 2.16 ("actual authenticated identity" above owner-key assignment, and
public-key sentence above commit refusal), RFC 7296 Section 3.16 (`SHOULD NOT`
send EAP Identity requests above `Session.Begin`), RFC 3748 Section 5.1 (both
sentences above the credential lookup), RFC 4301 Section 4.4.3.4 (above the
pool-binding refusal), RFC 3948 Section 5.1 (above composite multi-device lease
allocation and static-IP uniqueness), RFC 5216 Section 5.3 (MUST above the
certificate pin), and RFC 7296 Section 2.6 (as rationale above the unconditional
stateless cookie).

| RFC / section | Default configuration and required behavior | Explicit options / extensions to preserve | Tests of default behavior | Tests preserving options / extensions |
|---------------|---------------------------------------------|-------------------------------------------|---------------------------|---------------------------------------|
| RFC 7296 Section 2.6 | Unconfigured-source `IKE_SA_INIT` is unconditionally challenged with a stateless COOKIE before state allocation, then bounded by `half-open-limit` (64) and `half-open-limit-per-address` (8) | Configured site-to-site peers keep `cookie-threshold` behavior unchanged | `TestRemoteAccessCommitsNoStateBeforeCookie`, `TestRemoteAccessHalfOpenBounds` | `TestRemoteAccessNeverDivertsConfiguredPeer` |
| RFC 7296 Section 2.16 | Remote-access gateway must authenticate with an X.509 certificate; policy and lease owner use the actual EAP-authenticated `eap-user` | Site-to-site EAP peers keep `MethodConfig.Password` and configured peer owner key | `TestValidateRemoteAccessGateway`, `TestRemoteAccessLeaseOwnerIsTheAuthenticatedUser` | `responder-eap-mschapv2` |
| RFC 7296 Section 3.16 | `eap-mschapv2` and `eap-tls` omit the initial EAP Identity Request (`Session.Begin` starts the method directly) | `eap-md5` retains the initial EAP Identity Request so it can select the `eap-user` | `TestSessionBeginSkipsIdentityForMSCHAPv2AndTLS`, `remote-access-eap-mschapv2`, `remote-access-eap-tls` | `TestRemoteAccessAuthenticatesTheNamedUser` (`eap-md5`), `rfc3748_md5challenge_test.go` |
| RFC 7296 Section 2.19 | Pool-bound road warrior without `CFG_REQUEST` is refused Child SA creation with `FAILED_CP_REQUIRED` while keeping the IKE SA | Unbound site-to-site peers keep negotiating without CP | `TestRemoteAccessRequiresConfigurationPayload`, `ipsec-remote-access-cp-required.ci` | existing site-to-site `.ci` and interop scenarios |
| RFC 3948 Section 5.1 | Concurrent devices of one user and concurrent users behind one NAT receive distinct inner addresses; static IPs are reserved and unique | Pool-bound site-to-site respond peers keep single-slot `Acquire` semantics | `TestRemoteAccessMultiDeviceGetsDistinctLeases`, `TestRemoteAccessTwoClientsOneAddress`, `TestRegistryRoadWarriorStaticIPReservedAndPinned` | `TestOwnerGetsLastAddress`, `TestRequestedAddressHonored` |
| RFC 3748 Section 5.1 | `eap-mschapv2` uses its method-specific Response Name (after `stripDomain`) rather than `IDi` or an Identity Response | `eap-md5` selects by Identity Response | `TestRemoteAccessIdentityResponseSelectsNothing`, `TestRemoteAccessDomainPrefixIsOneOwner` | `TestRemoteAccessAuthenticatesTheNamedUser` (`eap-md5`) |
| RFC 5216 Section 5.3 | `eap-tls` verifies the client certificate chain, revocation, and exact `eap-user certificate` pin | Site-to-site `eap-tls` peers follow `docs/architecture/ike/ipsec-9-ikev2-eap-nat.md` (`allowed-identities`, `spec-eap-tls-identity-authorization`) | `TestRemoteAccessEAPTLSUnlistedCertificateRefused`, `remote-access-eap-tls-unlisted-certificate-refused` | `responder-eap-tls13` |
| RFC 4301 Section 4.4.3.4 | Road-warrior block with users requires `virtual-ip-pool`, preventing self-chosen inner addresses | Remote-access block with pools only (no `eap-user`) remains valid for pool-bound site-to-site peers | `tx-ipsec-remote-access-requires-pool.ci`, `remote-access-eap-mschapv2` | `test/parse/ipsec-virtual-ip-pool-config.ci` |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior, Existing Mechanisms, Design documents, Design presentation, and Data Flow sections completed
- [ ] AC-N rows carry testable assertions and Permitted Survivors
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks "Doctor check" and "Doctor code per problem class"

### Goal Gates (MUST pass)
- [ ] AC-1..AC-19 all demonstrated, with OD-1..OD-4 folded in and satisfied
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] The targeted tests for what changed pass, run in the main checkout by the main thread: the `ike/engine`, `ike/ipsec`, `ike/cmd`, and `core/eap` package tests (`-race`), `./le test functional ipsec`, `./le test functional parse`, the two reload `.ci` files, and `./le test integration interop-ipsec` on the scenarios named here. No agent runs whole-tree verify commands: GitHub CI verifies the pushed commit
- [ ] Feature code integrated
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Risks & Assumptions: every A-N confirmed or broken

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (`half-open-limit`, `half-open-limit-per-address`, `max-sessions-per-user`)
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/release/fixes/spec-ipsec-remote-access.md` only
