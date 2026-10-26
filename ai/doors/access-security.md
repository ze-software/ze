# Door: access and security

<!-- door-base: internal/component/aaa internal/component/authz internal/component/tacacs internal/component/radius internal/component/pki internal/component/ike internal/core/ipsecinventory internal/core/ikeprobe internal/core/eap internal/component/l2tp internal/core/audit -->

Read `ai/doors/system.md` first. This page names only what is particular to
three areas:

- who can log in
- who can run what
- the protocols that admit a remote party: IKEv2/IPsec, L2TP, PPPoE and PPP

## Where it sits

- **Components, each removable.** `tacacs`, `radius`, `ike` and `l2tp` (with
  PPPoE, the PPPoE client and the subscriber plugins) are feature gates
  `ze_tacacs`, `ze_radius`, `ze_ike` and `ze_l2tp` (`feature-gates.txt`).
  `aaa`, `authz` and `pki` are always on. The allowed imports of each are the
  table in `docs/architecture/core-design.md`, section 19.
- **AAA has its own composition root.** `internal/component/aaa/all` is
  hand-written, not generated: `all.go` blank-imports `authz`, and the gated
  siblings `all_ze_radius.go` and `all_ze_tacacs.go` add the other two.
- **The hub assembles the bundle.** `aaa.Default.Build()` orders backends by
  `Priority()`: RADIUS 50 (`radius/aaa.go`), TACACS+ 100 (`tacacs/register.go`),
  local bcrypt 200 (`authz/register.go`). The bundle is swapped atomically on
  reload (`cmd/ze/hub/aaa_lifecycle.go`). The flow is the digest's job.
- **Two IKE packages stay ungated on purpose.** `ike/dataplane` is the XFRM
  seam OSPF also uses for RFC 4552 (`internal/plugins/ospf/ipsec_install.go`),
  and `ike/crypto` is the transform registry `show mtu` reads
  (`feature-gates.txt`, the `ze_ike` comment).
- **Two core leaves publish IKE state without an import of the engine.**
  `ipsecinventory.Tunnels()` and `ikeprobe.Probe()` answer `ErrNotRegistered`
  when no engine registered. The engine registers both in
  `ike/engine/register.go` (core-design section 19).
- **`internal/core/audit` owns the audit log.** Transports accept its
  `Recorder` interface. The hub creates the log (core-design section 19).

## Who depends on it from outside

| Outside party | Contract | What breaks if it changes |
|---------------|----------|---------------------------|
| TACACS+ server (RFC 8907) | Authentication and authorization, plus START/STOP accounting from one hook in `Dispatcher.Dispatch` (`docs/architecture/aaa-tacacs.md`) | Operator logins and the server's command log |
| RADIUS server (RFC 2865, 2866, 3579) | Admin login by PAP, CHAP and EAP-MSCHAPv2. Subscriber auth and accounting through `l2tp/plugins/authradius` (`docs/guide/radius.md`) | Logins and billing records the operator's RADIUS keeps |
| RADIUS dynamic-authorization client (RFC 5176) | CoA and Disconnect requests Ze answers (`authradius/coa.go`) | Session control from the operator's policy system |
| IKEv2 peer, such as strongSwan (RFC 7296) | Initiator and responder, EAP, NAT-T, MOBIKE (`docs/architecture/ike/`) | Site-to-site tunnels |
| LAC, PPPoE host, PPP client (RFC 2661, 2516, 1661) | Tunnel, session and PPP negotiation (`docs/architecture/l2tp/`, `docs/architecture/wire/l2tp.md`) | Every subscriber session |
| PPPoE access concentrator, such as accel-ppp | Ze as PPPoE client (`internal/component/l2tp/pppoeclient`) | The uplink of a CPE build |
| An external plugin process | It verifies the hub's TLS leaf against the root `*pki.Root` issues (`internal/component/plugin/acceptor.go` `Authority`) | Every external plugin connection |
| TLS clients of web, API, looking glass, DoT and DoH | The PKI certificate and full chain (`docs/architecture/pki/tls-listeners.md`) | HTTPS for every operator surface |
| Operator | `system authentication`, `system authorization` profiles, `show audit` (`docs/guide/authentication.md`, `docs/guide/authorization.md`, `docs/guide/audit.md`) | Who can log in and run what, on every interface |

Login is not an SSH feature. SSH, web, REST and gRPC read one accepted
generation of users, and command authorization reads its policy from the same
generation (core-design section 19, `cmd/ze/hub/aaa_lifecycle.go`). Removing
a user takes effect at once on every credential surface
(`docs/guide/operator-access-rbac.md`, section 8).

## Boundaries

- **A new backend registers with `aaa.Default` and adds one blank import to
  `aaa/all`.** It never edits the hub or the chain.
- **Explicit rejection stops the chain. A connection error moves to the next
  backend.** (`aaa/types.go` `ChainAuthenticator`). A wrong TACACS+ password must
  not fall through to a stale local hash.
- **A backend that cannot build degrades, it does not abort.** A `Build` error
  fails the whole bundle and locks the operator out of SSH (`radius/aaa.go`
  `radiusBackend.Build`, its comments).
- **Readers of IKE state use the two core leaves**, never `ike/engine`
  (core-design section 19, the `mtu` row).
- **Subscriber auth, pool and shaper reach a session through
  `l2tp/subscriber` handler registries**, shared by L2TP and PPPoE
  (`docs/architecture/l2tp/subscriber-session-model.md`).
- **`ppp` is a leaf**: PPP, LCP and NCP state machines, no component import
  (core-design section 19).
- **EAP-TLS compiles only against a patched `crypto/tls`.**
  `internal/core/eap/eap_tls.go` references `tls.ZeServerChoosesSessionID`, so
  a plain `go build` or `go test` of any package that links `internal/core/eap`
  stops with `undefined`. Build through `./le`, or select the tree as
  `docs/contributing/patching-the-standard-library.md` shows.

## Read before code

| Read | Question it answers |
|------|---------------------|
| `ai/digests/aaa-auth.md` | How a login flows from the listener through the chain to RBAC |
| `ai/digests/ipsec-ike.md` | How an IKE SA is negotiated, rekeyed and installed into XFRM |
| `ai/digests/subscriber.md` | How a PPPoE or L2TP session reaches PPP, auth and the pool |
| `docs/architecture/core-design.md`, section 19 | What each component is allowed to import, and the two IKE leaves |
| `docs/architecture/aaa-tacacs.md` | The TACACS+ backend and its accounting hook |
| `docs/architecture/pki/pki-store.md` | The configured store versus the local CA that issues leaves |
| `docs/contributing/patching-the-standard-library.md` | Why the build needs a patched GOROOT, and how to select it |
| `docs/architecture/testing/interop.md` | Which peer and suite prove IPsec, L2TP, PPPoE and RADIUS |
| `ai/patterns/plugin.md`, `ai/patterns/config-option.md` | The shape of a new plugin or config leaf here |
| `ai/rules/rfc-compliance.md`, `ai/rules/protocol.md` | The quote owed above RFC code, and refusing what a backend cannot apply |
| `ai/rules/interop-and-goal-validation.md` | When an interop scenario is owed |

## Traps

- **One subscriber auth handler, last write wins.** `RegisterAuthHandler`
  (`l2tp/subscriber/handler_registry.go`) holds one handler. `authradius`
  claims it at configure time over `authlocal`, and in a test the local handler
  still answered. Open (`plan/journal/design-claim-never-measured.md`,
  2026-09-03).
- **A field list over a config type misses the next field.**
  `peerConfigChanged` named eight of twelve peer fields, so a narrowed traffic
  selector committed and never applied. Fixed by `SiteToSitePeer.Equal`
  (`plan/journal/guard-enumerates-instead-of-subtracting.md`, 2026-08-24).
- **A datagram that fails to decrypt kills a live SA.** Both the responder EAP
  round and the initiator IKE_AUTH set `StateDead`, against RFC 7296
  Section 2.4. Open (`plan/journal/unprotected-message-changes-sa-state.md`).
- **A refusal added for reload also ran at startup**, so the IPsec config
  apply (then `applyIPsecConfig`, now `applyConfig` taking an `applyPhase`) skipped every IKE transport when the WAN came up late
  (`plan/learned/RECURRING-PATTERNS.md`, from
  `plan/journal/guard-addition-drops-what-it-refuses.md`).
- **The scenario name does not say which role Ze plays.** In `eap-tls` Ze is the
  EAP peer, not the authenticator. Read both config files
  (`plan/learned/DESIGN-HISTORY.md`, the invariants table).
- **Configured is not negotiated.** `show vpn ipsec sa` read the ESP transform
  from the configured group (`plan/journal/constant-reported-as-measured-state.md`,
  2026-09-11).
- **A security `.ci` passed, but it proved nothing.**
  `test/parse/tacacs-key-required.ci` never reached the config
  (`plan/journal/green-that-could-not-have-been-red.md`, 2026-09-06). Prove red
  first.
- **A TACACS+ failure that names no fix.** `sendToServers`
  (`tacacs/client.go`) (`plan/journal/error-names-no-fix-command.md`,
  2026-10-19).

## Gates

| Command | Judges |
|---------|--------|
| `./le test functional ipsec`, `./le test functional l2tp`, `./le test pppoe` | The `.ci` suites for IPsec, L2TP and PPPoE |
| `./le test functional plugin`, `./le test functional parse` | AAA, TACACS+ and RADIUS config and behavior `.ci` files |
| `./le test integration interop-ipsec` | Ze against strongSwan |
| `./le test integration interop-radius`, `./le test integration interop-radius-subscriber` | Ze against FreeRADIUS, admin and subscriber |
| `./le test deployment l2tp-ppp`, `./le test deployment docker-pppoe-accel` | L2TP PPP/NCP, and the PPPoE client against accel-ppp |
| `./le rfc check` | Every tagged RFC requirement still has its tests and records |
| `./le plugin boundary check`, `./le arch tier check` | No same-process call across a plugin, and import direction |
