# Spec: ipsec-remote-access

| Field | Value |
|-------|-------|
| Status | design |
| Scope | protocol |
| Priority | P0 |
| Release reason | Implement the owner-selected repair for ignored remote-access authentication: road-warrior admission and per-user credentials. |
| Depends | `plan/release/fixes/spec-eap-tls-identity-authorization.md` (skeleton; owns `RFC5216-5.3-2`, the authorization of the identity an EAP-TLS certificate carries. The `eap-tls` mode of this spec needs its export of the verified client identity. The `eap-mschapv2` and `eap-md5` modes do not depend on it). Closed prerequisites: `spec-ike-virtual-ip-assignment` (2026-10-26, `plan/learned/035-ike-virtual-ip-assignment.md`), `spec-fixit-ipsec-verify-siblings` (2026-08-03) |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-24 |

Recovery after compaction: `.claude/rules/post-compaction.md`. Then read
`internal/component/ike/engine/register.go` `tryResponderSAInit`,
`internal/component/ike/engine/cp.go` `assignVirtualIP`,
`internal/core/eap/pool.go` `Registry.Acquire` and
`internal/core/eap/eap_mschapv2.go` `handleResponse`.

## Owner decisions (2026-11-24)

| ID | Decision |
|----|----------|
| OD-1 (multiple devices per `eap-user`) | Support multiple simultaneous devices per user (with a device/session discriminator and a configurable per-user tunnel limit leaf), so one user can connect laptop + phone concurrently. |
| OD-2 (half-open handshake DoS caps) | Default `half-open-limit` to `64` and `half-open-limit-per-address` to `8` (for clients behind shared NAT), and refuse `0` (cannot disable the limit). |
| OD-3 (client-requested VPN IP & static per-user IP) | Support an optional static IP leaf per `eap-user` in config; otherwise honor a client's requested IP only if it was that user's own previous IP (never hand user B's recently released IP to user A on request). |
| OD-4 (initial EAP Identity Request, RFC 7296 §3.16) | Follow RFC 7296 Section 3.16 (`SHOULD NOT`): drop the redundant initial EAP Identity Request for `eap-mschapv2` and `eap-tls` (keeping it only where required for `eap-md5`), and update affected EAP tests cleanly. |

## Task

`vpn ipsec remote-access` declares a road-warrior service. Two of its three
parts still have no behavior. Re-read against the code on 2026-11-14:

| Config | Consumer today | Effect |
|--------|----------------|--------|
| `remote-access pool` | `virtualIPState.reload` (`engine/virtual_ip.go`), called from the apply path (`engine/apply.go`) | **live since 2026-10-26.** A pool serves every `site-to-site` respond peer whose `virtual-ip-pool` leaf names it |
| `remote-access ike-group`, `esp-group` | none | none. The YANG descriptions say so |
| `remote-access authentication` | `warnKeylessEAPModes` (`engine/eap_auth.go`) writes one warning for `eap-md5` | no handshake reads the mode or the `x509` leaves |
| `remote-access eap-user` | `parseEAPUser`, `ValidateRemoteAccess`, `remoteAccessEqual` only | authenticates nobody. `EAPUser.Password` and `EAPUser.Certificate` have no reader outside parse, validation and config equality (`gopls references`, 2026-11-14) |

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
   proved it receives packets at its address, and with a stated bound.
2. **Per-user credentials.** The EAP method authenticates against one
   `eap-user` entry. An unknown user fails closed.
3. **Identity binding.** Every policy decision uses the user the EAP method
   authenticated: the lease owner, the session index, the log, the show output.
4. **Policy for an admitted user.** A road warrior is always pool-bound, so
   its inner address is a lease and never its own choice.
5. **Lifecycle.** A session ends when its SA dies, and when a reload removes
   or changes its user.
6. **Config correctness.** Commit refuses a remote-access block that cannot
   authenticate the gateway.

### Recorded owner rulings

Each ruling stays in force. None is reopened here.

| Date | Ruling |
|------|--------|
| 2026-07-23 | Implement the feature. Do not reject or warn about the inert config |
| 2026-07-31 | The RFC 7296 Configuration-payload rows of work package WP-9 that need the address-assignment feature moved to this spec; the rows already conformant stayed in the rfcgate-1b RFC 7296 pilot |
| 2026-08-02 | The Section 3.15.4 `INTERNAL_ADDRESS_FAILURE` obligation is spec work, not the extraction sign-off's (owner item OI-4) |
| 2026-08-31 | Ze is an IRAS and not an IRAC: it answers CFG_REQUEST and never sends one (owner item OI-1) |
| 2026-10-26 | Ownership, below |

**Ownership decided 2026-10-26 (design pass of the sibling).** The sibling
`spec-ike-virtual-ip-assignment` (closed 2026-10-26, `plan/learned/035-ike-virtual-ip-assignment.md`) became the SINGLE owner of
the Configuration-payload work: codec hardening, the lease registry and pool
selection, CP receive sites, reply ordering, both refusal notifications, selector
substitution, lease lifetime, optional attributes, visibility and the tagged and
strongSwan evidence. Basis: the fix/feature split rule in `ai/META-DECISIONS.md`
(the RFC 3948 Section 5.1 MUST is a fix reachable without admission) and the
transfer outcome this section names. The sibling's "Ownership" section maps every
shared criterion: AC-7..AC-10, AC-15 and AC-16..AC-28 of this spec, the security
list items, the Section 3.15.4 obligation and `RFC7296-2.20-1`. None is dropped.
The CP phases A to G and phase 3 of this spec are therefore NOT implemented here;
they are prerequisites delivered by the sibling. This spec keeps AC-1..AC-6,
AC-11, AC-12, AC-13 and AC-14 (admission, per-user credentials, session reaping,
`ra.Auth` checks, the road-warrior strongSwan scenario), and its CP owner items
are answered there: OI-1 by the owner ruling of 2026-08-31 (IRAS yes, IRAC no),
OI-2 by RFC 7296 Section 3.15.1 ("An implementation of this specification MAY
ignore CFG_SET payloads."), OI-3 by answering SUPPORTED_ATTRIBUTES. Admission
must reuse the sibling's lease registry, passing the EAP identity as the owner key.

**What the ruling means for this file.** The AC numbers in the paragraph
above are those of the spec as it stood on 2026-10-26. The criteria the
sibling took are closed work and are not listed below, and the criteria this
spec kept are restated, with new numbers, in "Acceptance Criteria". The older
pause that waited on OI-1, the CP phases A to G, the id-allocation hazard and
the verbatim CP obligations were removed on 2026-11-14: the ruling above
ended them and the rows landed. The ten RFC 7296 rows live in
`rfc/short/rfc7296.md` under these ids: `RFC7296-2.19-5`, `RFC7296-2.19-6`,
`RFC7296-2.19-7`, `RFC7296-2.19-8`, `RFC7296-3.15.1-8`, `RFC7296-3.15.1-9`,
`RFC7296-3.15.1-10`, `RFC7296-4-6`, `RFC7296-4-7`, `RFC7296-1.7-3`, and the
Section 3.15.4 obligation is `RFC7296-3.15.4-1`.

"Passing the EAP identity as the owner key" is made exact in "Design",
row D-2: the key is the user the method authenticated, not the string the
client typed.

## Owner decisions open

Only what no RFC text, agreed design or correctness argument decides. The
spec stays `design` until each row has an answer.

| Id | Decision | Evidence | Options | What each changes |
|----|----------|----------|---------|-------------------|
| OD-1 | May one `eap-user` hold several sessions at once (laptop and phone)? | `Registry.Acquire` (`eap/pool.go`) gives one address per owner, pool and family, and a second holder of the slot gets the SAME lease. Two live sessions of one user would both name one inner address. RFC 7296 is silent. For a configured peer a newly authenticated SA supersedes the old one (`finishResponderEstablish`, `engine/responder.go`) | (a) one session per user, the new one supersedes. (b) one session per user, the new one is refused. (c) several per user | (a) is the design written below, because it extends the existing supersede. (b) changes AC-9 to a refusal. (c) needs an owner key of user plus a device discriminator, a per-user maximum leaf, and a new AC for the drain it reopens |
| OD-2 | The two admission bounds: half-open road-warrior handshakes in total, and per source address | `responderBusy` (`engine/reconcile.go`) bounds a configured peer to ONE half-open handshake. A road warrior has no configured peer, so nothing bounds it. A cookie proves only that the source receives packets. Clients behind one NAT share a source address | Leaf names are fixed in D-4. Open: the two default values, and whether zero may mean unlimited | The defaults decide how many clients may dial at one instant and how many of them one NAT may hold. AC-4 and the boundary table take the numbers |
| OD-3 | Does a road warrior get the specific address it asks for? | `Registry.requestedLocked` (`eap/pool.go`) grants any requested address that is in the pool and free. `grantLocked` then forgets that it was another owner's last address. An authenticated user can therefore pick any free address, including one another user just released | (a) keep. (b) honor a request only when it equals that user's own last address. (c) add a per-user static address leaf and ignore requests otherwise | (a) lets a reconnecting client keep its address, and lets any user take an address a firewall rule may trust. (b) keeps reconnect and removes the choice. (c) adds a leaf and a commit check that the address is in the bound pool |
| OD-4 | Does Ze stop sending the EAP Identity request? | RFC 7296 Section 3.16: "Note that since IKE passes an indication of initiator identity in the first message in the IKE_AUTH exchange, the responder SHOULD NOT send EAP Identity requests (type 1)." `Session.Begin` (`eap/eap.go`) sends one on every exchange, site-to-site included. It is a SHOULD NOT, and `ai/META-DECISIONS.md` schedules optional requirements after mandatory coverage. D-1 selects the `eap-md5` user by the Identity response, so that mode keeps the request under either option | (a) leave it, record the gap. (b) drop it for `eap-mschapv2` and `eap-tls` in this spec | (a) changes nothing on the wire. (b) changes the first EAP round of every existing EAP scenario and needs each re-proved |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/ike/ipsec-14-responder.md` - the responder role, and the pool-bound peer paragraph
  -> Constraint: a lease is granted after authentication and before the selectors are narrowed; every Child SA of that IKE SA names it.
- [ ] `docs/architecture/ike/ipsec-9-ikev2-eap-nat.md` - EAP and NAT traversal
  -> Constraint: the page names the road warrior as the EAP use case; its `parseEAPUser` row is the only eap-user reader it lists.
- [ ] `docs/architecture/ike/ipsec-7-ikev2-engine.md` - declared by `remote_id.go`; remote identity policy
- [ ] `docs/architecture/ike/ipsec-3-data-model.md` - the typed model between YANG and the engine
- [ ] `ai/doors/access-security.md`, `ai/doors/forwarding.md`, `ai/doors/core.md`, `ai/doors/system.md`
  -> Constraint: readers of IKE state use `ipsecinventory.Tunnels()` and `ikeprobe.Probe()`, never `ike/engine`.
  -> Constraint: a datagram that fails to decrypt changes no SA state; every handler opens a protected message through `openProtected`.
  -> Constraint: EAP-TLS builds only through `./le` (patched `crypto/tls`).
  -> Constraint: `internal/core/eap` is core and imports no component, so the credential lookup is a value the engine hands in.
- [ ] `ai/rules/goroutine-lifecycle.md`
  -> Constraint: a per-client session is one goroutine per lifecycle, reaped when its SA dies.
- [ ] `ai/rules/protocol.md`
  -> Constraint: a config the backend cannot apply exactly is refused at verify.

### RFC Summaries (Scope: protocol)
The authority is `rfc/full/`. Every sentence below was read there on 2026-11-14.
- [ ] `rfc/short/rfc7296.md`
  -> Constraint (Section 2.16): "For this reason, these protocols are typically used to authenticate the initiator to the responder and MUST be used in conjunction with a public-key-signature-based authentication of the responder to the initiator."
  -> Constraint (Section 2.16): "It is important that policy lookups and access control decisions use the actual authenticated identity."
  -> Constraint (Section 2.6, descriptive, no keyword): "These attacks can be made less effective if a responder uses minimal CPU and commits no state to an SA until it knows the initiator can receive packets at the address from which it claims to be sending them."
  -> Constraint (Section 2.6): "When a responder detects a large number of half-open IKE SAs, it SHOULD reply to IKE_SA_INIT requests with a response containing the COOKIE notification."
  -> Constraint (Section 2.19): "In the case where the IRAS's configuration requires that CP be used for a given identity IDi, but IRAC has failed to send a CP(CFG_REQUEST), IRAS MUST fail the request, and terminate the Child SA creation with a FAILED_CP_REQUIRED error."
- [ ] `rfc/short/rfc5216.md`
  -> Constraint (Section 5.3, MUST): "Once a TLS session is established, EAP-TLS peer and server implementations MUST validate that the identities represented in the certificate are appropriate and authorized for use with EAP-TLS."
  -> Constraint (Section 5.3, SHOULD, not MUST): "Since the EAP-TLS server is typically connected to the Internet, it SHOULD support validating the peer certificate using RFC 3280 [RFC3280] compliant path validation, including the ability to retrieve intermediate certificates that may be necessary to validate the peer certificate." Ze makes the validation mandatory by its own choice (`eapTLSServerConfig`), because an authenticator with no trust anchor authenticates nobody.
- [ ] `rfc/short/rfc3748.md`
  -> Constraint (Section 5.1): "It is RECOMMENDED that the Identity Response be used primarily for routing purposes and selecting which EAP method to use."
  -> Constraint (Section 5.1): "EAP Methods SHOULD include a method-specific mechanism for obtaining the identity, so that they do not have to rely on the Identity Response."
- [ ] `rfc/short/rfc4301.md`
  -> Constraint (Section 4.4.3.4): "It is necessary to impose these constraints on creation of child SAs to prevent an authenticated peer from spoofing IDs associated with other, legitimate peers."

**Key insights:**
- The cookie gate needs a session today: `cookieRequired(ps)` and
  `halfOpenResponderCount` read `activePeersMap`. Admission that creates the
  session first puts state ahead of the gate.
- MSCHAPv2 never compares the Identity response with anything.
  `handleResponse` hashes the Name field of the Response, after `stripDomain`,
  with the one configured password. `Session.Identity()` has no non-test caller.
- `reconcilePeersWith` stops every running session whose name is not in
  `newCfg.Peers`. A dynamic session kept in `activePeersMap` is stopped at the
  next reload.
- A `.ci` under `test/ipsec/` drives two `ze` daemons. Ze sends no CFG_REQUEST
  (ruling of 2026-08-31), so a `.ci` road warrior reaches authentication and is
  then refused its Child SA with FAILED_CP_REQUIRED. The proofs that carry an
  address are the strongSwan scenarios and engine tests that feed wire messages
  to `tryResponderSAInit`.

## Current Behavior (MANDATORY)

**Source files read:** (2026-11-14, each by its producing function)
- [ ] `internal/component/ike/engine/register.go` - `tryResponderSAInit`: peer match, then `cookieRequired(ps)`, then the `responderBusy` compare-and-swap, then `newResponderSA`. `matchResponderPeer` compares the source address only, never the port
- [ ] `internal/component/ike/engine/cookie.go` - `cookieRequired` reads true for a nil session and otherwise compares `halfOpenResponderCount` with `cookieThreshold`, default zero, which challenges every initiation
- [ ] `internal/component/ike/engine/reauth_receive.go` - `matchReauthenticatingPeer` serves initiate peers only and refuses an EAP peer
- [ ] `internal/component/ike/engine/cp.go` - `decideCPRequest` (one CP, type request, an address attribute), `assignVirtualIP` (returns nil for a peer with no pool; `errCPRequired` for a bound peer with no request; leases with `sa.PeerName` as owner), `substituteVirtualIP`, `releaseVirtualIP`, `moveVirtualIP`
- [ ] `internal/component/ike/engine/responder.go` - `selectAuthChildSA` calls `assignVirtualIP` after authentication and before the selectors are narrowed; `finishResponderEstablish` supersedes an established SA only on the authenticated message
- [ ] `internal/core/eap/pool.go` - `Registry.Acquire`, `requestedLocked`, `grantLocked`, `Release`
- [ ] `internal/component/ike/engine/virtual_ip.go` - `virtualIPState.reload` rebuilds pools from `RemoteAccessConfig.Pools`; the registry is never replaced
- [ ] `internal/component/ike/engine/sa.go` - `forgetKeys` calls `releaseVirtualIP`, the one release point
- [ ] `internal/component/ike/engine/responder_eap.go` - `eapMethodConfig`, `eapTLSServerConfig` (certificate, OCSP staple, CA, CRLs and intermediates all read from `sa.PeerCfg.Auth`), `computeServerAuth` (no PSK fallback), `startResponderEAP`, `handleResponderEAP`
- [ ] `internal/core/eap/eap.go` - `Session.Begin` sends the Identity request; `handleIdentity` stores the Identity response and starts the method; `MethodConfig.Password` is one value
- [ ] `internal/core/eap/eap_mschapv2.go` - `handleResponse`; `internal/core/eap/mschapv2.go` - `stripDomain`
- [ ] `internal/core/eap/eap_md5challenge.go` - `md5ChallengeMethod.Process` checks the Value against the one secret and reads no Name
- [ ] `internal/core/eap/eap_tls.go` - `eapTLSPeerName` returns the certificate subject and is called only to build error text
- [ ] `internal/component/ike/engine/remote_id.go` - `checkRemoteIdentity` runs no comparison when `remote-id` is empty; `remoteIDMatches` already carries the class gate and sub-tree matching
- [ ] `internal/component/ike/engine/reconcile.go` - `reconcilePeersWith`, `PeerSession.responderBusy`, `PeerSession.resumption`
- [ ] `internal/component/ike/engine/resumption.go` - `resumptionFor` keeps one ticket-key store per peer name, replaced when the authentication config changes
- [ ] `internal/component/ike/engine/child.go` - `appendChildPolicies` builds one XFRM policy per pair of `ChildSA.Selectors`, the negotiated set
- [ ] `internal/component/ike/ipsec/types.go` - `RemoteAccessConfig` (IKEGroup, ESPGroup, Auth, Pools, Users); no selectors, no pool choice
- [ ] `internal/component/ike/ipsec/config.go` - `parseRemoteAccess` reads the authentication block with `parseAuthConfig`, the peer's parser
- [ ] `internal/component/ike/ipsec/validate.go` - `ValidateRemoteAccess` (pools, a password or a certificate name per user), `ValidatePKIRefs` (iterates `c.Peers` only), `ValidateVirtualIPPoolBindings`
- [ ] `internal/component/ike/ipsec/yang/ze-ipsec-conf.yang` - `container remote-access`. No `configuration-payload`, `lease-lifetime` or `maximum-leases` leaf exists

**What is true today, by claim:**

| Fact | Producer |
|------|----------|
| An unconfigured source is dropped | `tryResponderSAInit`, after both matchers return nil |
| The cookie check runs after a session is found and before any SA exists | `tryResponderSAInit`, the block above the `responderBusy` compare-and-swap |
| A pool-bound respond peer is leased an address, owner key the peer name | `assignVirtualIP` |
| A peer with no pool proposes its own selectors and gets them, limited by its configured selectors and the engine-wide inner-selector registry | `assignVirtualIP` returns nil; `narrowChildSelectorsIn` (`engine/ts_narrow.go`) |
| A second holder of one owner slot gets the same lease | `Registry.Acquire` |
| The lease is released when the IKE SA closes, and an IKE rekey moves it | `forgetKeys`, `moveVirtualIP` |
| The kernel receives the narrowed selectors | `appendChildPolicies` reads `ChildSA.Selectors` |
| The EAP credential is the peer's one password, or its certificate pair | `eapMethodConfig` |
| The user name MSCHAPv2 hashes comes from the client and selects nothing | `handleResponse` |
| The EAP-TLS client certificate is validated for chain and revocation; its name is used in error text only | `eapTLSServerConfig`, `eapTLSPeerName` |
| A commit accepts a remote-access block with no gateway certificate | `ValidatePKIRefs` and `ValidateRemoteAccess` contain no such check |
| A commit refuses a remote-access certificate name the candidate PKI lacks | the `ze:reference` statements on the two `x509` leaves and on `eap-user certificate`, enforced by `config.ValidateCustomSections` (comment on `ValidatePKIRefs`) |
| A reload reads the remote-access block only to rebuild pools | `virtualIPState.reload`. `IPsecConfig.Changed` has no non-test caller |

**Behavior to preserve:**
- A source that matches a configured peer is served by that peer and is never diverted.
- One half-open handshake per configured peer, accept-in-parallel for an established SA (RFC 7296 Section 2.4).
- The pool-bound respond peer: its owner key stays the peer name, its leases, refusals and selectors unchanged.
- The site-to-site EAP path: one password in `MethodConfig.Password`.
- A config with no `remote-access` block behaves as today.
- Every existing interop scenario and `.ci`.

**Behavior to change:**
- An unsolicited IKE_SA_INIT from an unconfigured source is served when `remote-access` is configured.
- `eap-user` entries become the credentials of that service.
- A commit refuses a remote-access block that cannot authenticate the gateway or names no pool.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| The lease registry | `internal/core/eap/pool.go` `Registry`, `Acquire`, `Release` | One engine-wide authority: one address per owner, pool and family; the owner's last address preferred; never re-issues a live lease after a pool edit | Extends: a road-warrior SA passes the authenticated user as owner in place of `sa.PeerName`. No second allocator |
| The Configuration-payload decision | `engine/cp.go` `decideCPRequest`, `assignVirtualIP`, `substituteVirtualIP` | Answers a pool-bound peer's CFG_REQUEST, refuses a bound peer that sent none with FAILED_CP_REQUIRED, narrows TSi to the lease | Extends: a road warrior is always pool-bound, so "CP required" is the existing bound-peer rule and needs no new leaf |
| The cookie gate | `engine/cookie.go` `cookieRequired`, `verifyCookie`, `mintCookie`; `engine/register.go` `tryResponderSAInit` | Challenges an initiation before the half-open slot is taken, for a matched session | Extends: the same gate runs for a source with no session, before any session exists, and is unconditional there |
| The half-open slot | `engine/reconcile.go` `PeerSession.responderBusy`; `engine/cookie.go` `halfOpenResponderCount` | One half-open handshake per configured peer; the count is derived from the sessions | Extends: each road-warrior session holds the same slot, so a derived count covers them. Adds the two bounds of D-4, which a per-peer slot cannot express |
| The peer session and its supersede | `engine/reconcile.go` `PeerSession`, `startPeerSession`, `stopPeerSession`; `engine/responder.go` `finishResponderEstablish` | Owns one peer's SA, DPD, rekey and teardown; a newly authenticated SA supersedes the established one | Extends: one session per road-warrior handshake, built from the remote-access block. A parallel road-warrior FSM is refused |
| The EAP session and its methods | `internal/core/eap/eap.go` `Session`, `MethodConfig`; `eap_mschapv2.go`, `eap_md5challenge.go`, `eap_tls.go` | Authenticates against one credential | Extends: `MethodConfig` gains a credential lookup the method calls with the name it authenticates, and the session reports the authenticated user. `Password` keeps working for a site-to-site peer |
| PAD matching | `engine/remote_id.go` `checkRemoteIdentity`, `remoteIDMatches` | Compares IDi with `remote-id`; no check when `remote-id` is empty; sub-tree and range matching landed | Does not apply as the authorization: RFC 7296 Section 2.16 puts access control on the EAP-authenticated identity, not on IDi. The terminator refusal in `checkRemoteIdentity` still runs |
| The reconcile path | `engine/reconcile.go` `reconcilePeersWith`, `peerConfigChanged` | Stops a session whose peer is gone or changed, keeps the rest | Extends: road-warrior sessions are reconciled against the remote-access block and their user entry (D-6). The removal loop over `newCfg.Peers` must not see them as removed peers |
| Certificate, CRL and OCSP wiring | `engine/responder_eap.go` `eapTLSServerConfig` | Reads the certificate, its OCSP staple, the CA, its CRLs and the intermediates by the names in `sa.PeerCfg.Auth` | Extends by reuse: a session built from the remote-access `authentication` block reads the same function, so it inherits all of it. `parseRemoteAccess` reads the block with the peer's parser |
| EAP-TLS resumption store | `engine/resumption.go` `resumptionFor`, `forgetResumption`; `PeerSession.resumption` | One ticket-key store per peering, by peer name | Extends: one store for the remote-access block, shared by its sessions, because a store per client dies with the client and its tickets could never be redeemed |
| PKI reference check | `ze:reference` in `ze-ipsec-conf.yang`; `config.ValidateCustomSections` | Refuses a name the candidate PKI does not hold | Does not change: the former "unknown reference" criterion is already met. This spec adds the presence checks only |
| Pool doctor check | `engine/doctor_virtual_ip.go` `checkIPsecVirtualIPPool`, code `doctor-ipsec-virtual-ip-pool` | Warns on pool utilization | Does not change. This spec adds its own checks beside it (Integration Checklist) |

## Data Flow (MANDATORY)

### Entry Point
Inbound UDP IKE_SA_INIT from an arbitrary source, on the IKE or the NAT-T socket.

### Transformation Path
1. `deliverDatagram` finds no SATable entry; `tryResponderSAInit` checks the header.
2. `matchResponderPeer`, then `matchReauthenticatingPeer`. A match is served as today and the steps below do not run.
3. **New.** No match and `remote-access` is configured: the cookie is verified from the datagram alone (initiator SPI, nonce, source address). A missing or wrong cookie is answered with a fresh one. Nothing is stored.
4. **New.** With a verified cookie, the two admission bounds are checked. Over a bound, the initiation is dropped and counted.
5. **New.** One session is created for this initiator SPI from the remote-access block, with its half-open slot taken. `newResponderSA` and the IKE_SA_INIT exchange run unchanged.
6. IKE_AUTH carries IDi and no AUTH. `startResponderEAP` signs with the gateway certificate named in the remote-access block and starts EAP. A CFG_REQUEST is kept on the SA (`decideCPRequest`).
7. **New.** The method asks the credential lookup for the name it authenticates. An unknown name gets the same EAP-Failure as a wrong credential.
8. **New.** On success the session records the authenticated user. A live session of the same user is superseded (OD-1).
9. The final IKE_AUTH runs `assignVirtualIP` with that user as owner, then `substituteVirtualIP`, then the Child SA install, all unchanged.
10. SA end (`forgetKeys`) releases the lease. **New:** the session is removed from the road-warrior set and its goroutine ends.
11. **New.** A config apply ends the sessions whose user or block changed (D-6).

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| UDP transport to engine dispatch | `transport.Packet` | No |
| engine to EAP session | `eap.MethodConfig` with the credential lookup; the session's authenticated user | No |
| engine to lease registry | `Registry.Acquire` owner argument | No |
| config tree to engine | `parseRemoteAccess` (`ipsec/config.go`) | No |
| engine to readers of IKE state | `PeerSession.Info`, `ipsecinventory.Tunnels()` | No |

### Changed-Wire Census

**Search evidence:** scope `internal/component/ike`, `internal/core/eap`,
`cmd/ze`; terms `RemoteAccess`, `EAPUser`, `.Users`, `NewPool(`,
`eapTLSPeerName`; positive control: `virtualIPState.reload` was found as the
one engine reader of `RemoteAccessConfig.Pools`. `gopls references` on
`IPsecConfig.RemoteAccess`, `EAPUser.Password`, `EAPUser.Certificate` and
`Session.Identity`. No IKE payload format changes: the exchange uses the
payloads `wire` already encodes. The wires that change are the config tree
and the session inventory.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| `vpn ipsec remote-access` config tree | writer | `ipsec/yang/ze-ipsec-conf.yang` `container remote-access`; the config editor and every config file in the repository | grep of the YANG for `remote-access`; `test/parse/ipsec-remote-access.ci` is a repository config | Change: new leaves (D-3, D-4). Every repository config with the block is converted in the same work (`ai/META-DECISIONS.md`, config syntax) |
| `vpn ipsec remote-access` config tree | reader | `ipsec/config.go` `parseRemoteAccess`, `parseEAPUser`; `ipsec/validate.go` `ValidateRemoteAccess`; `engine/virtual_ip.go` `reload` | grep `RemoteAccess` over the scope, nine non-test sites | Change: parse and validate the new leaves; `reload` unchanged |
| Peer and SA inventory | writer | `engine/reconcile.go` `PeerSession.Info`; `engine/register.go` `PeerInfoMap`, `ActivePeers` | `gopls symbols` on both files | Change: road-warrior sessions appear, named by user |
| Peer and SA inventory | reader | `show vpn ipsec` handlers; `internal/core/ipsecinventory` `Tunnels()`; an operator's scripts over the JSON | `ai/doors/access-security.md`, "Two core leaves" | Preserve the existing fields; the user is an added field |
| IKEv2 and EAP messages | writer | Ze's responder: `startResponderEAP`, `handleResponderEAP`, `sendResponderEAP` | read of `engine/responder_eap.go` | Preserve: no format change. OD-4 would change the first EAP round |
| IKEv2 and EAP messages | reader | the road-warrior client outside the repository (strongSwan, an operating system's client) | scenario `virtual-ip-assignment` runs strongSwan as that client | Preserve |

### Integration Points
- `tryResponderSAInit` - gains the road-warrior arm after both matchers.
- `cookieRequired` and `halfOpenResponderCount` - must answer for a source with no session, and count road-warrior sessions.
- `lookupPeerSession`, `TerminatePeerSA`, `TerminateAllSAs`, `PeerInfoMap` - must find a road-warrior session.
- `assignVirtualIP` - the owner argument.
- `eapMethodConfig` - builds the lookup for a remote-access session.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding, outbound | No | |
| Registration over hardcoding, inbound | No | |

## Design

One row per hole. "Decided by" names the text or the argument, so no row is a
question.

| Id | Hole | Design | Decided by |
|----|------|--------|------------|
| D-1 | Which name is the user | The user is the `eap-user` entry whose credential the method verified. `eap-mschapv2`: the entry named by the Response's Name field after `stripDomain`. `eap-md5`: the entry named by the Identity response, which is the name `md5ChallengeMethod.Process` says the exchange authenticates. `eap-tls`: the entry whose `certificate` leaf names the PKI entry equal, byte for byte, to the client's validated leaf certificate. IDi, and the Identity response under `eap-mschapv2` and `eap-tls`, select nothing and authorize nothing | RFC 7296 Section 2.16 ("actual authenticated identity"); RFC 3748 Section 5.1 (both sentences quoted above); RFC 5216 Section 5.3 MUST for `eap-tls` |
| D-2 | The owner key | The lease owner, the session index and every log and show field carry the `eap-user` list key of D-1, never a string the client sent. `CORP\alice` and `alice` are one owner because both resolve to the entry `alice`. A name that resolves to no entry has no owner and no lease | Correctness: one credential must not yield several owners, or one user drains the pool that `Registry.Acquire` bounds per owner |
| D-3 | Pool binding, and CP mandatory | The remote-access block gains a `virtual-ip-pool` leaf naming one of its pools, read exactly as a peer's leaf is. Commit refuses a block with `eap-user` entries and no pool. Every road-warrior SA is therefore pool-bound: no CFG_REQUEST is FAILED_CP_REQUIRED, TSi is the lease, and the kernel gets the lease (`appendChildPolicies`). The block also gains the peer's `traffic-selector` list with the same grammar and the same meaning, checked by `selectorsCoverPool`; an empty list accepts any TSr the client proposes, as it does for a peer | RFC 4301 Section 4.4.3.4 (quoted above): a user who picks its own inner address can take another user's. RFC 7296 Section 2.19 FAILED_CP_REQUIRED |
| D-4 | No state before the cookie check, with a bound | For a source with no session the cookie is always demanded, whatever `cookie-threshold` says, and it is verified from the datagram alone. Only then are the bounds checked and a session created. Two leaves under `remote-access` state the bounds: `half-open-limit` (all road-warrior handshakes not yet authenticated) and `half-open-limit-per-address` (those from one source address). A handshake that does not authenticate within `responderHandshakeTimeout` is reaped and its session removed. Established sessions are bounded by the pool, because each needs a lease | Correctness: without the cookie a forged-source flood fills the bound and denies every real client. The RFC 7296 Section 2.6 sentence is advice, and its SHOULD is conditional on "a large number"; neither is cited as a MUST. Default values: OD-2 |
| D-5 | The session key behind a NAT | A session is created per verified initiator SPI and is found afterward only through the SATable, as a retransmit is today. It is never found by source address, so two clients behind one NAT are two sessions. After authentication the session is indexed by user | Correctness: `matchResponderPeer` keys on the source address, which two clients behind one NAT share |
| D-6 | A user removed or changed | On every apply, each road-warrior session is compared with the new block. A session whose user entry is gone, or whose password or certificate changed, is ended with an IKE Delete and its lease released. A change to the block's groups, authentication, pool binding or selectors ends every session, the same answer `peerConfigChanged` gives a peer. A pool range edit follows the registry's existing rule. An IKE SA rekey does not re-run EAP and needs no check, because the removal already ended the session. A new IKE_AUTH, reauthentication included, runs EAP against the table in force. An EAP-TLS resumption re-checks that the user entry still exists and still names the certificate | Correctness: a removed credential must stop working. `ai/doors/access-security.md` states the same rule for logins |
| D-7 | Commit refusal without a gateway certificate | `validateIPsecSections` refuses a remote-access block with `eap-user` entries whose `authentication x509 certificate` is empty, for every mode, since every remote-access mode is EAP. It refuses `mode eap-tls` with no `ca-certificate`, as Ze's own rule. Each refusal names the leaf to set | RFC 7296 Section 2.16 MUST (quoted above). The runtime backstop exists: `computeServerAuth` |
| D-8 | Unknown user | The method answers an unknown name exactly as it answers a wrong credential: same packets, same EAP-Failure. The operator's log names the attempted name once | Correctness: a different answer tells an attacker which names exist |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | A session built from the remote-access block drives the responder FSM unchanged | `newResponderSA`, `startResponderEAP` and `eapTLSServerConfig` read only `sa.PeerCfg` and the two groups; scenario `responder-eap-mschapv2` runs the same FSM | a parallel responder path, much larger | scenario `remote-access-eap-mschapv2` | unvalidated |
| A-2 | Road-warrior sessions can live outside the removal loop of `reconcilePeersWith` and still be found by `lookupPeerSession`, `halfOpenResponderCount`, `PeerInfoMap` and the terminate functions | each reads `activePeersMap` under `peersMu` today | either reload stops every road warrior, or show, clear and the cookie count miss them | `TestRemoteAccessSessionSurvivesUnrelatedReload`, `TestRemoteAccessSessionsVisible` | unvalidated. The earlier form of this assumption ("reconcile iterates config peers") was wrong: the removal loop iterates the running sessions |
| A-3 | A PKI certificate entry can hold a client certificate with no private key, so `eap-user certificate` can name one | `eapTLSServerConfig` checks `entry.PrivateKey == nil` itself, which implies the store allows it | the `eap-tls` pin of D-1 needs another carrier | a parse test of such an entry | unvalidated |
| A-4 | strongSwan as a client sends the MSCHAPv2 Name it is configured with, and a CFG_REQUEST in IKE_AUTH | scenario `virtual-ip-assignment` passes with `vips = 0.0.0.0` | the interop scenarios need another client setting | scenario `remote-access-eap-mschapv2` | unvalidated |
| A-5 | strongSwan reports a refused road warrior in a form a check can read | not read: no strongSwan source is in this checkout | the negative scenarios assert on Ze's side only | read the strongSwan function before each negative scenario is written (`ai/rules/interop-and-goal-validation.md`) | unvalidated |
| A-6 | A resumed EAP-TLS session exposes the client certificate the full handshake validated | the comment on `resumptionFor`: crypto/tls carries the chain in the ticket | D-6 cannot re-check a resumed user and must refuse resumption for remote access | a resumption test over a removed user | unvalidated |
| A-7 | An `eap-md5` client sends the user name in its Identity response | the comment in `md5ChallengeMethod.Process` | D-1 for `eap-md5` must read the Response's Name field | scenario or `.ci` with an `eap-md5` user | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Admission is a denial-of-service surface | goroutine or session count grows under a flood | D-4: cookie first, two bounds, reap at the handshake timeout; AC-3, AC-4 |
| R-2 | A session leaks when its SA dies in an unusual state | session count grows across connect and disconnect cycles | one removal point beside `forgetKeys`; AC-10 |
| R-3 | The credential lookup breaks the site-to-site EAP path | scenario `responder-eap-mschapv2` goes red | `MethodConfig.Password` is untouched when no lookup is set |
| R-4 | The lookup resolves from the Identity response under MSCHAPv2 | none on the happy path: an honest client sends one name twice | AC-6 sends two different names and asserts which one owns the lease |
| R-5 | Reload stops every road warrior | every client reconnects after an unrelated commit | A-2; AC-12 |
| R-6 | The reaper races the dispatch goroutine | race reports in the engine tests | reuse the `ownedSA` discipline; the engine tests run under the race detector |
| R-7 | A road-warrior source later becomes a configured peer's address | the client is served by the wrong block | matching order is fixed: configured peers first (AC-2) |
| R-8 | Two specs build two EAP-TLS allow-lists | `spec-eap-tls-identity-authorization` designs a list of its own | the Depends row; its design pass must take `eap-user` as the server-role list |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | An unauthorized client gets a tunnel, or one user takes another's inner address, or a flood denies the IKE service to configured peers. Nothing changes for a config with no `remote-access` block |
| How is it reverted? | A commit revert. The new leaves are additive; a config that uses them stops parsing after a revert |
| Who else touches this path? | `spec-eap-tls-identity-authorization` (EAP-TLS identity), `spec-rfc4301-architecture-gaps` (`remote_id.go`; its PAD matching phase has landed and this spec does not change that file, so there is no dependency), `spec-yang-single-source-of-truth` (owns the journal row on the inert subtree) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| IKE_SA_INIT from an unconfigured source, `remote-access` configured | → | the road-warrior arm of `tryResponderSAInit` | `test/ipsec/ipsec-remote-access-admits-unconfigured-source.ci` |
| MSCHAPv2 Response naming an `eap-user` | → | the credential lookup | `TestRemoteAccessAuthenticatesTheNamedUser` |
| final IKE_AUTH of an authenticated user | → | `assignVirtualIP` with the user as owner | `TestRemoteAccessLeaseOwnerIsTheAuthenticatedUser` |
| commit that removes an `eap-user` | → | the road-warrior reconcile | `test/ipsec/ipsec-remote-access-user-removed-ends-session.ci` |
| strongSwan road-warrior client | → | the whole path | interop scenario `remote-access-eap-mschapv2` |

## Acceptance Criteria

Numbering restarts here. The criteria the sibling took on 2026-10-26 are not
renumbered into this table.

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | IKE_SA_INIT with a valid cookie from an unconfigured source; `remote-access` configured | Ze answers IKE_SA_INIT and the handshake reaches IKE_AUTH; `show vpn ipsec` lists one road-warrior session | The same datagram with no `remote-access` block is still dropped and logged as unconfigured, because no service was configured |
| AC-2 | A source that matches a configured respond peer; `remote-access` also configured | The configured peer's session serves it, with that peer's credentials and selectors | None: a configured address is never served by the remote-access block |
| AC-3 | IKE_SA_INIT from an unconfigured source with no cookie, or a wrong one, at any `cookie-threshold` | Ze answers with a COOKIE notification, and the session count, SATable size and goroutine count are what they were | A wrong cookie is answered with a fresh one and is not rejected (`RFC7296-2.6-5`) |
| AC-4 | Verified initiations past `half-open-limit`, and past `half-open-limit-per-address` from one address | Each one past a bound is dropped and counted, and the doctor check of AC-15 reports the bound; those within the bounds proceed | A handshake already admitted finishes; a configured peer's handshake is outside both bounds |
| AC-5 | `eap-mschapv2`; the Response names a configured `eap-user` and proves its password | EAP-Success, and the session reports that user | A site-to-site EAP peer keeps authenticating with its one password |
| AC-6 | `eap-mschapv2`; Identity response `alice`, Response Name `bob`, proof of bob's password | The session's user and the lease owner are `bob` | The Identity response may be any string: it selects nothing |
| AC-7 | The Response names no `eap-user`, or names one and proves a wrong password | Both get the same EAP-Failure sequence and no session survives; the operator's log names the attempt | None: no fallback credential is tried |
| AC-8 | Response Name `CORP\alice`, then a second session with Name `alice` | Both resolve to the entry `alice` and one owner slot; the pool holds one lease for them | A name with a realm suffix resolves to no entry and is refused, because the `eap-user name` leaf admits no `@` |
| AC-9 | A user authenticates while holding a live session | The new session is established and the old one is ended with an IKE Delete; the user holds one lease | Subject to OD-1 |
| AC-10 | N connect and disconnect cycles, and N handshakes abandoned before authentication | The session count, the goroutine count and the pool's held count return to their base values | Sessions of configured peers are untouched |
| AC-11 | An authenticated road warrior sends no CFG_REQUEST | The final IKE_AUTH carries FAILED_CP_REQUIRED and no SA or TS payload; the IKE SA stays; no XFRM state or policy exists for it | A client that then sends a CFG_REQUEST in a new exchange is served |
| AC-12 | A commit removes one `eap-user`, or changes its password | That user's session ends with an IKE Delete and its lease is free; every other session stays up with its Child SAs | A commit that changes the block's groups, authentication, pool binding or selectors ends every road-warrior session, as it does for a changed peer |
| AC-13 | A commit of a remote-access block with `eap-user` entries and no `authentication x509 certificate`, or with `mode eap-tls` and no `ca-certificate`, or with no `virtual-ip-pool` | Each is refused with the leaf to set and, for the first, RFC 7296 Section 2.16 | A block with pools only and no `eap-user` still commits: it serves pool-bound peers |
| AC-14 | Two clients behind one NAT, each with its own user | Two sessions, two leases, two Child SAs, traffic on both | None |
| AC-15 | The gateway certificate or CA of the block is missing or has no key in the live store; an admission bound is reached | `ze doctor` reports each under its own code with the command that fixes it | A healthy block reports nothing |
| AC-16 | strongSwan road warrior, `eap-mschapv2`, from an address Ze has no peer for | It establishes, holds an address of the bound pool, and traffic passes; Ze's XFRM policy names the lease as the remote selector | None |
| AC-17 | `eap-tls`; a client whose chain validates and whose leaf certificate no `eap-user` names | EAP-Failure and no session; a client whose leaf is named is served as that user | Waits on the Depends spec for the exported identity |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | A road warrior dials in with EAP-MSCHAPv2 and gets its address | UDP, cookie, admission, EAP, user lookup, lease, CFG_REPLY, Child SA | interop scenario `remote-access-eap-mschapv2` |
| 2 | A road warrior dials in with EAP-TLS | the same, with the certificate pin | interop scenario `remote-access-eap-tls` |
| 3 | An operator removes a user and that user's tunnel drops | commit, apply, road-warrior reconcile, IKE Delete | `test/ipsec/ipsec-remote-access-user-removed-ends-session.ci` |
| 4 | An operator commits a block that cannot authenticate the gateway | commit, `validateIPsecSections`, refusal | `test/reload/tx-ipsec-remote-access-requires-certificate.ci` |
| 5 | An operator asks who is connected | `show vpn ipsec` lists road-warrior sessions by user | `test/ipsec/ipsec-remote-access-admits-unconfigured-source.ci` |
| 6 | An operator asks why clients cannot connect | `ze doctor` | `test/ipsec/ipsec-remote-access-doctor.ci` |

## 🧪 TDD Test Plan

### Unit Tests
Engine tests feed wire messages to `tryResponderSAInit` and the handlers it
reaches, so each one runs the real exchange.

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRemoteAccessAdmitsAfterCookie` | `engine/remote_access_test.go` | AC-1 | |
| `TestRemoteAccessNeverDivertsConfiguredPeer` | `engine/remote_access_test.go` | AC-2 | |
| `TestRemoteAccessCommitsNoStateBeforeCookie` | `engine/remote_access_test.go` | AC-3 | |
| `TestRemoteAccessHalfOpenBounds` | `engine/remote_access_test.go` | AC-4 | |
| `TestRemoteAccessAuthenticatesTheNamedUser` | `eap/eap_user_test.go` | AC-5 | |
| `TestRemoteAccessIdentityResponseSelectsNothing` | `eap/eap_user_test.go` | AC-6 | |
| `TestRemoteAccessUnknownUserAnswersAsWrongPassword` | `eap/eap_user_test.go` | AC-7 | |
| `TestRemoteAccessDomainPrefixIsOneOwner` | `engine/remote_access_test.go` | AC-8 | |
| `TestRemoteAccessLeaseOwnerIsTheAuthenticatedUser` | `engine/remote_access_test.go` | AC-6, AC-8 | |
| `TestRemoteAccessNewSessionSupersedes` | `engine/remote_access_test.go` | AC-9 | |
| `TestRemoteAccessNoSessionLeak` | `engine/remote_access_test.go` | AC-10 | |
| `TestRemoteAccessRequiresConfigurationPayload` | `engine/remote_access_test.go` | AC-11 | |
| `TestRemoteAccessUserRemovedEndsSession` | `engine/remote_access_test.go` | AC-12 | |
| `TestRemoteAccessSessionSurvivesUnrelatedReload` | `engine/remote_access_test.go` | AC-12, A-2 | |
| `TestValidateRemoteAccessGateway` | `ipsec/validate_test.go` | AC-13 | |
| `TestRemoteAccessTwoClientsOneAddress` | `engine/remote_access_test.go` | AC-14 | |
| `TestRemoteAccessSessionsVisible` | `engine/remote_access_test.go` | story 5, A-2 | |
| `TestRemoteAccessDoctor` | `engine/doctor_remote_access_test.go` | AC-15 | |
| `TestRemoteAccessEAPTLSUnlistedCertificateRefused` | `engine/remote_access_test.go` | AC-17 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `half-open-limit` | OD-2 | OD-2 | OD-2 | OD-2 |
| `half-open-limit-per-address` | OD-2 | OD-2 | OD-2 | OD-2 |

### Functional Tests
Each drives a second `ze` as the client. That client sends no CFG_REQUEST, so
these prove admission, authentication, refusal and teardown, and the address
proofs are the interop scenarios.

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ipsec-remote-access-admits-unconfigured-source` | `test/ipsec/` | a client Ze has no peer for authenticates as a user and is listed by `show vpn ipsec`; with the block removed the same client is dropped | |
| `ipsec-remote-access-unknown-user-refused` | `test/ipsec/` | a client naming no `eap-user`, and one with a wrong password, get no IKE SA | |
| `ipsec-remote-access-cp-required` | `test/ipsec/` | an authenticated client with no CFG_REQUEST keeps its IKE SA and gets no Child SA (AC-11) | |
| `ipsec-remote-access-user-removed-ends-session` | `test/ipsec/` | a commit that removes the user ends its IKE SA; a second user's stays | |
| `ipsec-remote-access-doctor` | `test/ipsec/` | `ze doctor` names the missing gateway certificate and its fix | |
| `tx-ipsec-remote-access-requires-certificate` | `test/reload/` | a commit of a block with no gateway certificate is refused | |
| `tx-ipsec-remote-access-requires-pool` | `test/reload/` | a commit of a block with users and no `virtual-ip-pool` is refused | |

### Interop Tests (Scope: protocol)
Named scenarios, no numeric prefix. Each negative is proved red first by
removing the refusal it tests. Before a check expects strongSwan to print or
send something, the strongSwan function is read and named (A-5).

| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `remote-access-eap-mschapv2` | `test/interop-ipsec/scenarios/` | strongSwan client | AC-16: an unconfigured source establishes, holds a pool address, passes traffic, and Ze's XFRM policy names the lease | |
| `remote-access-eap-tls` | `test/interop-ipsec/scenarios/` | strongSwan client | story 2 | |
| `remote-access-wrong-password-refused` | `test/interop-ipsec/scenarios/` | strongSwan client | AC-7: no IKE SA, no lease, no XFRM state on Ze | |
| `remote-access-unknown-user-refused` | `test/interop-ipsec/scenarios/` | strongSwan client | AC-7: the same outcome for a name no `eap-user` carries | |
| `remote-access-eap-tls-unlisted-certificate-refused` | `test/interop-ipsec/scenarios/` | strongSwan client | AC-17: a certificate the CA issued and no user names is refused | |
| `remote-access-eap-tls-revoked-client-refused` | `test/interop-ipsec/scenarios/` | strongSwan client | a listed and revoked client is refused: the block inherits the CRL wiring of `eapTLSServerConfig` | |
| `remote-access-two-clients-one-nat` | `test/interop-ipsec/scenarios/` | two strongSwan clients behind one NAT | AC-14 | |

## Files to Modify
- `internal/component/ike/engine/register.go` - `tryResponderSAInit` road-warrior arm; the session lookups
- `internal/component/ike/engine/cookie.go` - the gate for a source with no session; the count
- `internal/component/ike/engine/reconcile.go` - road-warrior sessions outside the peer removal loop; their reconcile
- `internal/component/ike/engine/apply.go` - the road-warrior reconcile on apply
- `internal/component/ike/engine/cp.go` - `assignVirtualIP` owner argument
- `internal/component/ike/engine/responder_eap.go` - `eapMethodConfig` builds the lookup
- `internal/component/ike/engine/resumption.go` - the block's shared store
- `internal/component/ike/engine/config.go` - `validateIPsecSections`
- `internal/core/eap/eap.go` - `MethodConfig` lookup; the session's authenticated user
- `internal/core/eap/eap_mschapv2.go`, `eap_md5challenge.go`, `eap_tls.go` - each method resolves its credential by D-1
- `internal/component/ike/ipsec/types.go`, `config.go`, `validate.go` - the new leaves, `remoteAccessEqual`, the D-7 refusals
- `internal/component/ike/ipsec/yang/ze-ipsec-conf.yang` - the new leaves; the descriptions of `container remote-access`, its `ike-group`, `esp-group`, `authentication`, `x509` and `eap-user` nodes, which today say that nothing reads them
- `internal/core/diagnostic/codes.go` - the doctor codes
- `docs/architecture/ike/ipsec-14-responder.md`, `ipsec-9-ikev2-eap-nat.md` - declared by the `// Design:` headers of `cp.go`, `register.go`, `pool.go`
- `docs/guide/ipsec.md` - the sentence that says an `eap-user` authenticates nobody and cites this spec
- `docs/features/ai-first.md` - declared by the `// Design:` header of `internal/core/diagnostic/codes.go`. Unaffected unless it lists the doctor codes one by one; the implementing agent checks that when the two codes are added

## Files to Create
- `internal/component/ike/engine/remote_access.go` and `remote_access_test.go`
- `internal/component/ike/engine/doctor_remote_access.go` and `doctor_remote_access_test.go`
- `internal/core/eap/eap_user_test.go` <!-- doc-links: ignore (a file this spec plans and has not written) -->
- the `.ci` files and scenario directories named in the test plan <!-- doc-links: ignore (tests this spec will create; the spec is `design`) -->

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `ze-ipsec-conf.yang`: `remote-access virtual-ip-pool`, `traffic-selector`, `half-open-limit`, `half-open-limit-per-address` |
| YANG validation constraints | Yes | ranges on both limits (OD-2); `virtual-ip-pool` as the peer leaf is typed |
| YANG custom validators | No | the pool and selector checks are plugin verify, as for a peer |
| CLI commands/flags | No | no new command; `show vpn ipsec` gains rows and a user field |
| CLI grammar (keyword before value) | N-A | no command added |
| Editor autocomplete | Yes | automatic for the typed leaves |
| Functional test for new RPC/API | N-A | no RPC |
| Pipe completeness | N-A | no new output path |
| Env var registration | No | |
| Doctor check for runtime dependencies | Yes | the gateway certificate, its key and the CA in the live store |
| Doctor code per problem class | Yes | `doctor-ipsec-remote-access-credential` (gateway certificate, key or CA unusable; runtime phase), `doctor-ipsec-remote-access-admission` (a bound reached, initiations dropped; runtime phase). Pool exhaustion stays `doctor-ipsec-virtual-ip-pool` |
| Prometheus counters/metrics | Yes | road-warrior sessions (gauge), half-open road-warrior handshakes (gauge), admissions dropped by bound (counter, label: bound), EAP refusals (counter, label: unknown-user or wrong-credential) |
| BGP family surface | N-A | |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `features/ipsec-eap-authentication.md` |
| 2 | Config syntax changed? | Yes | `docs/guide/ipsec.md`, the remote-access section |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md`, the `show vpn ipsec` fields |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | Yes | `docs/guide/ipsec.md` |
| 7 | Wire format changed? | No | no payload format changes |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc7296.md` (Section 2.16 rows), `rfc/short/rfc5216.md` (`RFC5216-5.3-2`, with the Depends spec), `docs/features/rfc-status.md` |
| 10 | Test infrastructure changed? | Yes | `docs/architecture/testing/interop.md`, the new scenarios |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md`, the remote-access VPN row |
| 12 | Internal architecture changed? | Yes | `docs/architecture/ike/ipsec-14-responder.md`, `ipsec-9-ikev2-eap-nat.md` |
| 13 | Route metadata keys? | No | |
| 14 | Prometheus counters? | Yes | the IKE telemetry page |
| 15 | Registered inventory changed? | Yes | the doctor check inventory |
| 16 | Doc source anchors on changed files? | Yes | derived at implementation with `./le spec citation anchors spec plan/release/fixes/spec-ipsec-remote-access.md` |
| 17 | Existing docs show examples? | Yes | every remote-access example in `docs/guide/ipsec.md` |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** - the road-warrior arm, cookie first, one session per verified initiator SPI
   - Tests: `ipsec-remote-access-admits-unconfigured-source.ci`, `TestRemoteAccessAdmitsAfterCookie`, `TestRemoteAccessCommitsNoStateBeforeCookie`, `TestRemoteAccessNeverDivertsConfiguredPeer`
   - Files: `engine/remote_access.go`, `engine/register.go`, `engine/cookie.go`
   - Verify: the wiring `.ci` is red because no user authenticates yet
2. **Phase: Config** - the leaves, their parse, the D-7 and D-3 refusals, the conversion of repository configs
   - Tests: `TestValidateRemoteAccessGateway`, the two `tx-` files
3. **Phase: Credentials and identity** - D-1, D-2, D-8 for `eap-mschapv2` and `eap-md5`
   - Tests: the `eap/eap_user_test.go` tests, `TestRemoteAccessLeaseOwnerIsTheAuthenticatedUser`, `TestRemoteAccessDomainPrefixIsOneOwner`
   - Verify: scenario `responder-eap-mschapv2` unchanged
4. **Phase: Bounds and lifecycle** - D-4 bounds, reaping, supersede, D-6
   - Tests: `TestRemoteAccessHalfOpenBounds`, `TestRemoteAccessNoSessionLeak`, `TestRemoteAccessNewSessionSupersedes`, `TestRemoteAccessUserRemovedEndsSession`, `TestRemoteAccessSessionSurvivesUnrelatedReload`
5. **Phase: EAP-TLS** - the certificate pin, after the Depends spec
   - Tests: `TestRemoteAccessEAPTLSUnlistedCertificateRefused`
6. **Phase: Visibility** - show fields, the two doctor checks, metrics
7. **Phase: Interop** - the seven scenarios, each negative proved red first
8. **Phase: Documentation** - the checklist rows, in the phase that makes each page wrong

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC has an implementation at a named function |
| Fail-closed | An unknown user, a lookup error, an empty user table, a nil lookup and a session with no user each deny. A session with no user gets no lease |
| No state before the cookie | Nothing is allocated, stored or logged at Warn for an unverified source |
| Identity | No policy decision reads IDi, or the Identity response under `eap-mschapv2` or `eap-tls` |
| Site-to-site untouched | Every existing scenario and `.ci` green; AC-2 |
| Concurrency | The engine and eap tests run under the race detector |
| Mutation | Each new guard removed in turn turns its test red |
| Operator errors and doctor codes | Every refusal names the leaf and the fix; every Warn names its code |
| Rule: no-layering | The YANG descriptions that say "no code reads this" are rewritten, not kept beside the new text |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| `eap-user` has a handshake reader | `gopls references` on `EAPUser.Password` and `EAPUser.Certificate` lists a site outside parse, validation and equality |
| The lease owner is the user | `TestRemoteAccessLeaseOwnerIsTheAuthenticatedUser` |
| No state before the cookie | `TestRemoteAccessCommitsNoStateBeforeCookie` |
| A removed user is disconnected | `ipsec-remote-access-user-removed-ends-session.ci` |
| Road warrior against strongSwan | `./le test integration interop-ipsec` on the seven scenarios, run by the main thread |
| The YANG says what the code does | no `remote-access` description contains "no code reads", "not served yet" or "has no effect" |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Unauthenticated resource use | D-4. No lease before EAP success (`assignVirtualIP` runs after authentication) |
| Credential handling | `eap-user` passwords never logged and never serialized |
| User enumeration | D-8: identical answers for an unknown user and a wrong password |
| Identity spoofing | D-1, D-2; AC-6, AC-8 |
| Inner address choice | D-3; OD-3 |
| Client chain validation | the block's `ca-certificate`, CRLs and intermediates are those `eapTLSServerConfig` reads |
| Stale authorization | D-6, including EAP-TLS resumption |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Scenario `responder-eap-mschapv2` regresses | the lookup was not additive: phase 3 |
| Reload stops road warriors | A-2: phase 4 |
| A negative interop scenario cannot go red | the check reads the client and not Ze: rewrite the check on Ze's state |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- A component with no consumer has no evidence of correctness. The codec and
  the pool both looked complete until the sibling spec gave them a caller and
  found the reserved-bit fold and the IPv6 out-of-range lease.
- An owner key is an authorization input. `sa.PeerName` is safe for a
  configured peer because the operator wrote it. The same argument position
  filled with a client's string is not.
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
| One session per road-warrior handshake, built from the remote-access block | one shared session; a parallel road-warrior FSM | reuses the established, DPD, rekey and teardown machinery, and each client has its own half-open slot |
| The method resolves the credential by the name it authenticates | resolve from the Identity response before the method starts (the 2026-07-31 design) | RFC 3748 Section 5.1 and RFC 7296 Section 2.16: the Identity response is a routing hint, sent in clear, and MSCHAPv2 hashes another field |
| Pool binding makes CP mandatory | a `configuration-payload required` leaf | `assignVirtualIP` already refuses a bound peer with no request; a second switch could disagree with it |
| Cookie always, for a source with no session | follow `cookie-threshold` | a tolerated spoofed initiation holds a slot of a shared bound |
| Lease only after EAP success | lease at admission | an unauthenticated source must not hold an address |

## Known Limitations
- An `eap-user` name cannot carry a realm, because the leaf's character set admits no `@`. A client that sends `alice@example.com` is refused.
- A `.ci` cannot prove address assignment, because Ze is not an IRAC (ruling of 2026-08-31).

## RFC Documentation

`// RFC 7296 Section 2.16` with the "actual authenticated identity" sentence
above the owner-key assignment, and with the public-key sentence above the
commit refusal. `// RFC 3748 Section 5.1` with both sentences above the
credential lookup. `// RFC 4301 Section 4.4.3.4` above the pool-binding
refusal. `// RFC 5216 Section 5.3` with the MUST above the certificate pin.
The RFC 7296 Section 2.6 sentence is quoted above the unconditional cookie as
the reason, and is not called a requirement.

## Implementation Summary

### What Was Implemented
- (fill during implementation)

### Bugs Found/Fixed
- (fill during implementation)

### Documentation Updates
- (fill during implementation)

### Deviations from Plan
- (fill during implementation)

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1..AC-16 | owed | phases 1 to 4, 6 and 7 | |
| AC-17 | owed | phase 5 | waits on the Depends spec |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|-----------|-------|

### Files from Plan
| File | Status | Notes |
|------|--------|-------|

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:**
- **Skipped:**
- **Changed:**

## Goal Validation (BLOCKING)

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| a road warrior can establish against ze | interop test | scenario `remote-access-eap-mschapv2` |
| per-user credentials are live, and an unknown user fails closed | interop negative + functional | scenarios `remote-access-unknown-user-refused`, `remote-access-wrong-password-refused`; `ipsec-remote-access-unknown-user-refused.ci` |
| policy uses the authenticated user | unit over the real exchange | `TestRemoteAccessIdentityResponseSelectsNothing`, `TestRemoteAccessLeaseOwnerIsTheAuthenticatedUser` |
| a road warrior cannot choose its inner address | interop (Ze's XFRM policy) + functional | scenario `remote-access-eap-mschapv2`; `ipsec-remote-access-cp-required.ci` |
| no resource leak, no state for an unverified source | unit lifecycle | `TestRemoteAccessNoSessionLeak`, `TestRemoteAccessCommitsNoStateBeforeCookie` |
| a removed user is disconnected | functional | `ipsec-remote-access-user-removed-ends-session.ci` |
| the config is validated | functional | the two `tx-` files |

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

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-17 all demonstrated, and OD-1..OD-4 answered and recorded
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] The targeted tests for what changed pass, run in the main checkout by the main thread: the `ike/engine`, `ike/ipsec` and `core/eap` package tests, `./le test functional ipsec`, the two reload `.ci` files, and `./le test integration interop-ipsec` on the scenarios named here. No agent runs any `./le verify` command: GitHub CI verifies the pushed commit
- [ ] Feature code integrated
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Risks & Assumptions: every A-N confirmed or broken

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING - before ANY commit)
- [ ] Critical Review passes
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`
- [ ] **Commit A:** code + tests + docs + spec
- [ ] **Commit B:** `remove plan/release/fixes/spec-ipsec-remote-access.md` only
