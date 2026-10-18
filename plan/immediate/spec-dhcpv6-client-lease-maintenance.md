# Spec: dhcpv6-client-lease-maintenance

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Handoff | verify |
| Updated | 2026-10-18 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The DHCPv6 client (`internal/plugins/iface/dhcp/dhcp_v6_linux.go`) does not
maintain a lease the way RFC 8415 requires. An operator with a DHCPv6-addressed
uplink loses the address on a cycle tied to the first lease, sees a fresh
binding from the server each time (a new DUID per Solicit), and reads a Warn
line every 45 seconds for as long as no server answers.

**Owner decision, 2026-10-18 (Thomas, verbatim):** "Include RFC 8415 in the
scope." This lifts the 2026-10-08 RFC enrolment freeze (owner decision
2026-10-08, "no new RFC enrolment/summaries") for RFC 8415 only. Enrolment of
the document is part of this work (Implementation Steps, phase 1). Research
found that RFC 8415 is obsoleted by RFC 9915 (STD 102, January 2026); which
document is enrolled is open question Q1.

Found 2026-10-18 by reading during the VyOS T9108 comparison, journal row
`plan/journal/silent-fall-through.md` (2026-10-18). Every defect below was
CONFIRMED at the producer by this spec's research; none was reproduced at
runtime.

| # | Defect | Producer | RFC 8415 text the behavior breaks |
|---|--------|----------|-----------------------------------|
| D-1 | A renewal is a fresh Solicit: `renewV6` calls `client.RapidSolicit`, never a Renew or Rebind carrying the leased IA | `renewV6` | §18.2.4: "At time T1, the client initiates a Renew/Reply message exchange to extend the lifetimes on any leases in the IA." |
| D-2 | `renewV6` opens its socket with `nclient6.New(c.ifaceName)`, the logical name, while acquisition uses `binding.OsName`; a selector-bound interface fails every renewal | `renewV6` | §18.2.4 (same) |
| D-3 | `runV6` renews at most twice per acquisition (once at T1, once at T2 if the first failed), then sleeps `validLft - t2` and removes the address even when the server answered every time | `runV6` | §18.2.5: "The message exchange is terminated when the valid lifetimes of all leases across all IAs have expired" |
| D-4 | No backoff: a failed exchange is the library's 3 tries (5 s, 10 s, 20 s) then a fixed 10 s sleep and a Warn, forever, one Warn per 45 s | `runV6`, `retryFn` (`vendor/github.com/insomniacslk/dhcp/dhcpv6/nclient6/client.go`) | §15: "RT = 2*RTprev + RAND*RTprev"; §18.2.1 Solicit uses SOL_TIMEOUT 1 s capped at SOL_MAX_RT 3600 s |
| D-5 | The library retransmits the identical bytes: the Elapsed Time option stays 0 | `SendAndRead`, `retryFn` | §15: "The client MUST update an "elapsed-time" value within an Elapsed Time option (see Section 21.9) in the retransmitted message." |
| D-6 | The DUID changes: `dhcpv6.NewSolicit` builds a DUID-LLT stamped with the current time on every call, so each Solicit (every retry, every re-acquisition, every restart) presents a new client identity. The YANG `duid` leaf promises "When the leaf is omitted, ze generates a DUID-LL from the interface MAC", and the configured value reaches `dHCPConfig.DUID` and is read by nothing | `NewSolicit` (`vendor/github.com/insomniacslk/dhcp/dhcpv6/dhcpv6message.go`), `dHCPConfig.DUID` (`dhcp_linux.go`) | §11.2: "Clients and servers using this type of DUID MUST store the DUID-LLT in stable storage and MUST continue to use this DUID-LLT" |
| D-7 | A Reply lease with valid lifetime 0 is installed PERMANENT: `ReplaceAddressWithLifetime` with 0 and 0 reaches the vendored netlink `addr_linux.go`, which sends no IFA_CACHEINFO when both lifetimes are 0, so the kernel holds the address forever | `handleV6Reply`, `netlinkBackend.ReplaceAddressWithLifetime` (`internal/plugins/iface/netlink/manage_linux.go`) | §18.2.10.1: "Discard any leases from the IA, as recorded by the client, that have a valid lifetime of 0 in the IA Address or IA Prefix option." |
| D-8 | No validation of lifetimes or timers: preferred greater than valid is installed, T1 greater than T2 is used | `handleV6Reply`, `v6Timers` | §21.6: "The client MUST discard any addresses for which the preferred lifetime is greater than the valid lifetime." §21.4: "If a client receives an IA_NA with T1 greater than T2 and both T1 and T2 are greater than 0, the client discards the IA_NA option" |
| D-9 | T1/T2 come from the first IA_NA only, IA_PD timers are ignored, and T1 = 0 becomes a fixed 30 min whatever the lifetime (a 10 min lease expires in the kernel 20 min before the first renewal) | `v6Timers` | §18.2.4: "the client MUST select T1 and T2 times from all IA options that will guarantee that the client initiates transmissions of Renew/Rebind messages not later than at the T1/T2 times associated with any of the client's bindings (earliest T1/T2)." |
| D-10 | Reply validation is the library's transaction-id match alone: `Request` passes a nil matcher, so any message type with the right xid is taken, and no Server Identifier or Client Identifier check exists | `Client.Request` (vendored) | §16.10: "Clients MUST discard any received Reply message that meets any of the following conditions: - the message does not include a Server Identifier option" |
| D-11 | The Solicit Option Request option omits SOL_MAX_RT (82), and a SOL_MAX_RT option in a reply is never read | `NewSolicit` (vendored) | §21.24: "A DHCP client MUST include the SOL_MAX_RT option code in any Option Request option (see Section 21.7) it sends in a Solicit message." |
| D-13 | No message carries IA_PD. `parseDHCPv6Config` (`internal/component/iface/config.go`) reads `pd { length }` into `dhcpv6UnitConfig.PDLength`, `internal/component/iface/register.go` carries it into `dHCPConfig.PDLength`, and no code reads it after that. `RapidSolicit` adds only `WithRapidCommit` and `NewSolicit` only `WithIAID` (an IA_NA); the vendored `WithIAPD` (`vendor/github.com/insomniacslk/dhcp/dhcpv6/modifiers.go`) is named by no file under `internal/`. The receive half is complete: `handleV6Reply` installs and publishes each `msg.Options.IAPD()` prefix (`dhcp_v6_pd_linux_test.go`), so Ze installs a prefix a server volunteers and asks no server for one. Refusing the leaf at commit (as `unimplementedVRFValidator` refuses `vrf`) would remove the only prefix delegation Ze has, so the request is built. Folded in from `spec-dhcpv6-client-never-requests-a-delegated-prefix` (owner Q3, 2026-10-18) | `runV6`, `renewV6`, `dHCPConfig.PDLength` | §18.2.1: "The client uses IA_NA options (see Section 21.4) to request the assignment of non-temporary addresses, IA_TA options (see Section 21.5) to request the assignment of temporary addresses, and IA_PD options (see Section 21.21) to request prefix delegation." §21.22: "A client MAY send a non-zero value in the "prefix-length" field and the unspecified value (::) in the "IPv6-prefix" field to indicate a preference for the size of the prefix to be delegated." (quoted from RFC 8415 by the folded skeleton; re-quote from RFC 9915 at enrolment, Q1) |
| D-12 | Expiry publishes `dhcp-expired` for IA_NA addresses only; a delegated prefix that expires is never reported expired | `publishV6Expired` | §18.2.5 (expiry of all leases) |

The goal: a client that acquires a lease with Solicit/Advertise/Request/Reply
(or Rapid Commit), keeps it with Renew at T1 and Rebind at T2 for as long as a
server answers, installs on the address exactly the lifetimes the lease carries
(infinite stays infinite), retransmits per RFC 8415 Section 15, presents one
stable DUID, and reports one Warn per outage rather than one per retry.

**Not this spec:** making an INFINITE lease survive the iface reconcile. The
reconcile (`learnedAddrSet`, `internal/component/iface/config_apply.go`,
commit 0fbb8b2065) spares addresses with a finite lifetime; another session
is making infinite leases safe through owned-address registration
(`RegisterOwnedAddresses`, `internal/component/iface/address_owner.go`). This
spec only guarantees that the lifetime Ze installs equals the lease's, and
depends on that work for the infinite case (A-6).

## Related Work

| Spec / row | Relation | Coordination |
|------------|----------|--------------|
| `plan/immediate/spec-dhcpv6-client-never-requests-a-delegated-prefix.md` (skeleton) | Same two call sites (`runV6`, `renewV6`); this spec replaces both with message builders, so the PD request becomes one IA row in the builder this spec writes | Folded into this spec (owner Q3, 2026-10-18): D-13, AC-25; the skeleton file is removed |
| `plan/immediate/spec-dhcpv4-client-drops-lease-every-period.md` (skeleton) | Same loop shape (renew at most once, then expire), same logical-name socket defect, same untestable `sleepOrStop` | Shares only the controlled clock this spec adds to `DHCPClient`; RFC 2131 timers and retransmission differ, so no shared state machine (Key Design Decisions, Q7) |
| `plan/spec-dhcpv6-server.md` (skeleton) | Server side of DHCPv6 | Enrolment excludes server and relay sections as a role Ze does not fill today; that spec re-walks them when a server exists (Q2) |
| `plan/journal/unwired-feature.md` (2026-09-03, `dhcpv6/pd`) | The unwired PD leaf | Closed by this spec (AC-25), since Q3 folded the PD spec in |
| `plan/journal/reconciler-claims-state-another-owner-installed.md` (2026-10-18) | Reconcile spares finite-lifetime leases | A-6 |

## Required Reading

### Architecture Docs
- [ ] `docs/features/interfaces.md` - DHCP feature rows and the reconcile paragraph
  → Constraint: the row "DHCPv6 proper Renew (not re-solicit) | missing | medium" becomes `have` only when AC-1..AC-4 pass; the row "DHCPv6 (SARR, IA_NA, IA_PD) | have" overstates IA_PD until AC-25 passes; this spec owns that correction now that Q3 folded the PD spec in.
  → Constraint: the reconcile paragraph states "An address with a finite valid lifetime stays: a DHCPv4 or DHCPv6 lease ... Its client renews it and the kernel expires it." This spec makes "its client renews it" true for DHCPv6; the sentence must stay true after the change (AC-10).
- [ ] `docs/architecture/testing/interop.md` - interop scenarios
  → Constraint: a protocol change owes a named scenario against another implementation; no DHCP scenario exists under `test/interop/scenarios/` (BGP, BFD, VRRP with `Dockerfile.keepalived` only), so this spec creates one and its Dockerfile.
  → Constraint: the discrimination walk (revert, rebuild the ze image, red, restore, green) is owed for the new scenario.
- [ ] `docs/contributing/rfc-conformance-gates.md` - enrolment, extraction sign-off, superseded marker
  → Constraint: a new enrolment needs `rfc/extraction/<stem>.json` (`checkEnrolment`); the forward walk maps or excludes EVERY normative site of the document, so enrolment covers the whole RFC, never only the client sections.
  → Constraint: a summary with an `Obsoleted-by` Meta row must carry a `{superseded: ...}` marker on every requirement line (`checkSuperseded`). Enrolling RFC 8415 therefore means every line names its RFC 9915 disposition (Q1).
  → Constraint: each added test tag owes a discrimination record (`./le rfc discriminate-record`) in the same change.

### RFC Text (read in `rfc/full/`, fetched 2026-10-18)
- [ ] `rfc/full/rfc8415.txt` - DHCPv6 (owner-named)
  → Constraint: §7.6 Table 1 defaults: SOL_MAX_DELAY 1 s, SOL_TIMEOUT 1 s, SOL_MAX_RT 3600 s, REQ_TIMEOUT 1 s, REQ_MAX_RT 30 s, REQ_MAX_RC 10, REN_TIMEOUT 10 s, REN_MAX_RT 600 s, REB_TIMEOUT 10 s, REB_MAX_RT 600 s, REL_TIMEOUT 1 s, REL_MAX_RC 4, MAX_WAIT_TIME 60 s.
  → Constraint: §7.7: 0xffffffff is infinity for lifetimes, T1 and T2; "A client will never attempt to extend the lifetimes of any addresses in an IA with T1 set to 0xffffffff."
  → Constraint: §14.1: "a DHCP client MUST limit the rate of DHCP messages it transmits or retransmits"; "For a device that has multiple interfaces, the limit MUST be enforced on a per-interface basis."; possible default 20 packets in 20 s; "A transmission rate limit SHOULD be configurable."
  → Constraint: §14.2: with T1 or T2 0 "the client MUST choose a time to avoid packet storms. In particular, it MUST NOT transmit immediately."
  → Constraint: §15: RT = IRT + RAND*IRT, then RT = 2*RTprev + RAND*RTprev, cap MRT + RAND*MRT, RAND uniform in [-0.1, +0.1]; MRC and MRD end the exchange; MRC and MRD both 0 means transmit until answered; elapsed time updated on every retransmission.
  → Constraint: §16.1: "A client MUST leave the transaction ID unchanged in retransmissions of a message."; new random xid per exchange.
  → Constraint: §16.10: discard a Reply without Server Identifier, or whose Client Identifier does not match the DUID sent.
  → Constraint: §16: "Clients, relay agents, and servers MUST NOT discard messages that contain unknown options"; "A client or server MUST discard any received DHCP messages with an unknown message type."
  → Constraint: §18.2.1: Solicit carries Client Identifier and Elapsed Time; first RT strictly greater than IRT; collect Advertise for the first RT unless preference 255; MRC 0 and MRD 0 (SHOULD); a Reply to a Rapid Commit Solicit is used only when it carries the Rapid Commit option.
  → Constraint: §18.2.4: Renew to the leasing server with its Server Identifier, Client Identifier, Elapsed Time, each IA with every leased address/prefix; MUST NOT include a lease the client did not obtain or whose valid lifetime is 0; ORO requests SOL_MAX_RT; IRT REN_TIMEOUT, MRT REN_MAX_RT, MRC 0, MRD = time to earliest T2.
  → Constraint: §18.2.5: Rebind = Renew without Server Identifier; IRT REB_TIMEOUT, MRT REB_MAX_RT, MRC 0, MRD = time until valid lifetimes of all leases expire; then Solicit.
  → Constraint: §18.2.10: process SOL_MAX_RT even in a failure Reply; UnspecFail retry rate-limited; UseMulticast resend via multicast.
  → Constraint: §18.2.10.1: compute T1/T2 from packet reception time; add new leases, update recorded lifetimes, discard valid 0, keep leases the Reply omits; NoBinding in a Renew/Rebind Reply → Request with all IAs; missing IA → rate-limited retry; MUST perform DAD per RFC 4862 §5.4 before use (the kernel performs it on a non-NODAD address, A-4).
  → Constraint: §21.4 / §21.21: client MUST use server T1/T2 unless 0; recommended 0.5 and 0.8 of the shortest preferred lifetime; T1 > T2 (both > 0) discards the IA.
  → Constraint: §21.24: SOL_MAX_RT valid range 60..86400, out-of-range ignored, a valid value replaces the internal parameter, per interface.
  → Constraint: §11, §11.2, §11.4: the DUID SHOULD NOT change; DUID-LLT needs stable storage; "A DUID-LL SHOULD NOT be used by DHCP clients or servers that cannot tell whether or not a network interface is permanently attached to the device" (Q4).
- [ ] `rfc/full/rfc9915.txt` - DHCPv6, obsoletes RFC 8415
  → Constraint: the client sections this spec cites keep their numbers (14, 14.1, 14.2, 15, 16.x, 18.2.1 to 18.2.10.1, 21.4, 21.24) and Table 1 keeps its values; §14.2 drops the IA_TA case; §18.2.13 "Restarting Server Discovery Process" is new and states the stop-using-all-bindings SHOULD that RFC 8415 carried inside §18.2.10.1.
  → Decision: code comments quote the enrolled document's text (Q1); the design is identical under either.

### Source
- [ ] `internal/plugins/iface/dhcp/dhcp_v6_linux.go` (329L) - `runV6`, `renewV6`, `handleV6Reply`, `removeV6Addrs`, `publishV6Expired`, `v6Timers`
  → Constraint: `handleV6Reply` caps at 16 addresses and 16 prefixes; keep both caps (resource bound on untrusted input).
  → Constraint: DNS from the reply goes to resolv.conf unless static name-servers exist; preserve, and apply §18.2.10 "SHOULD stop using" when a Renew reply omits DNS (A-7).
  → Constraint: events `dhcp-acquired` on first lease, `dhcp-renewed` on each extending Reply, `dhcp-expired` on expiry; payload `iface.DHCPPayload` (Name, Unit, Address, PrefixLength, DNS, DNSAll, LeaseTime) is preserved.
- [ ] `internal/plugins/iface/dhcp/dhcp_linux.go` (213L) - `DHCPClient`, `sleepOrStop` (direct `time.NewTimer`), `stoppableContext`, `publishDHCP`
  → Decision: `DHCPClient` takes a `clock.Clock` (`internal/core/clock`), `RealClock{}` in production; `sleepOrStop` uses it, so lease timing is test-driven. The v4 spec reuses the same field.
- [ ] `vendor/github.com/insomniacslk/dhcp/dhcpv6/nclient6/client.go` - `New`, `SendAndRead`, `retryFn`, `RapidSolicit`, `Request`
  → Constraint: `New(iface)` binds UDP 546 on the interface's link-local address and reads the MAC from `net.InterfaceByName`, so it MUST be given the OS device name.
  → Decision: keep the library for socket, encoding and xid demultiplexing; never call `RapidSolicit`, `Solicit` or `Request`. Each transmission is one `SendAndRead` on a client built with `WithRetry(1)` and `WithTimeout(RT)`, so Ze owns RT, RAND, MRC, MRD and the Elapsed Time update. The `Matcher` argument is the validation hook (D-10) and the Advertise collector (a matcher that records each valid Advertise and returns false until preference 255).
  → Constraint: `retryFn` with retry 1 returns `errDeadlineExceeded`, mapped to `ErrNoResponse`, after one RT; the receive loop drops packets for an xid with no pending entry, so a reply arriving between two transmissions of one exchange is lost; with RT of at least 1 s the window is the call overhead only (R-3).
- [ ] `vendor/github.com/insomniacslk/dhcp/dhcpv6/dhcpv6message.go`, `modifiers.go` - `NewSolicit`, `NewRequestFromAdvertise`, `WithIAID`, `WithIAPD`
  → Constraint: `NewSolicit` hard-codes a time-stamped DUID-LLT (D-6) and an ORO without SOL_MAX_RT (D-11); Ze builds its own Solicit from `NewMessage` plus options.
  → Constraint: IAID for IA_NA is the last four bytes of the MAC (library convention); Ze keeps that value so an existing server binding survives the change.
- [ ] `internal/component/iface/dispatch.go` - `ReplaceAddressWithLifetime`, `RemoveAddress`, `ResolveDevice`
  → Constraint: both address helpers resolve the logical name themselves; only the DHCPv6 SOCKET needs the OS name (D-2).
- [ ] `internal/plugins/iface/netlink/manage_linux.go` and `vendor/github.com/vishvananda/netlink/addr_linux.go`
  → Constraint: lifetimes are sent as `uint32`, so 4294967295 reaches the kernel as INFINITY_LIFE_TIME; both 0 sends no IFA_CACHEINFO and makes the address permanent (D-7). Ze never calls the helper with a valid lifetime of 0; it removes instead.
- [ ] `internal/component/iface/register.go` - `dhcpClientFactory`, `dhcpParams.duid`, `reconcileDHCP`
  → Constraint: the `duid` string already flows config → `dhcpParams` → factory → `dHCPConfig.DUID`; only the client side is missing.
- [ ] `internal/component/iface/yang/ze-iface-conf.yang` - leaf `duid` under `dhcpv6`, type string, no pattern
  → Decision: a malformed `duid` must be refused at commit (AC-20), through a YANG pattern on the leaf.
- [ ] `internal/plugins/iface/dhcp/dhcp_v6_pd_linux_test.go`, `dhcp_shortframe_integration_linux_test.go`
  → Constraint: unit tests drive `handleV6Reply` with a synthetic Reply and a recording bus; the integration test (`integration && linux` build tags) runs the real client over a veth lab with a Go-written server. Both patterns are reused.

**Key insights:**
- The vendored library's exchange layer (`RapidSolicit`, `Request`, `retryFn`) is the source of D-4, D-5, D-6, D-10, D-11; the fix is Ze-owned exchanges over the library's socket and codec, not library patches.
- The lease is a client-side record (DUID, server DUID, IAs with leases and reception times) that must outlive one Reply; today nothing records it, which is why Renew cannot be built.
- RFC 8415 is obsoleted by RFC 9915 with the same client section numbers (Q1).
- A valid lifetime of 0 installs a PERMANENT address today (D-7).

## Current Behavior (MANDATORY)

**Source files read:** (annotations under Required Reading, Source)
- [ ] `internal/plugins/iface/dhcp/dhcp_v6_linux.go` - DHCPv6 worker: acquisition by RapidSolicit, renewal by RapidSolicit on the logical name, linear sleep chain to expiry
- [ ] `internal/plugins/iface/dhcp/dhcp_linux.go` - client lifecycle, `sleepOrStop` on a real timer, event publishing
- [ ] `internal/plugins/iface/dhcp/register.go` - factory carrying `duid` and `pdLength` into `dHCPConfig`
- [ ] `vendor/github.com/insomniacslk/dhcp/dhcpv6/nclient6/client.go` - socket, xid demultiplexing, fixed 5 s doubling retry
- [ ] `vendor/github.com/insomniacslk/dhcp/dhcpv6/dhcpv6message.go` - `NewSolicit` with a time-stamped DUID-LLT, `NewRequestFromAdvertise`
- [ ] `internal/component/iface/dispatch.go` - address helpers resolving the logical name
- [ ] `internal/plugins/iface/netlink/manage_linux.go` - lifetime install; 0 and 0 become permanent through the vendored netlink
- [ ] `internal/component/iface/register.go` - `reconcileDHCP`, `dhcpClientFactory`
- [ ] `internal/component/iface/yang/ze-iface-conf.yang` - `dhcpv6` container, `duid` leaf

**Behavior to preserve:**
- Event names and payload shape (`dhcp-acquired`, `dhcp-renewed`, `dhcp-expired`, `iface.DHCPPayload`).
- `/128` install for IA_NA addresses (§18.2.10.1: "Addresses obtained from an IA_NA or IA_TA MUST NOT be used to form an implicit prefix with a length other than 128.").
- The 16-address and 16-prefix caps.
- resolv.conf write rules (static name-servers win).
- Stop semantics: `Stop` closes the stop channel, every wait returns, leased addresses are removed (`removeV6Addrs`), the worker exits; panics recovered by `safeRunV6`.
- IA_NA IAID derived from the last four bytes of the MAC.
- The v4 worker is untouched except for reading the injected clock through `sleepOrStop`.

**Behavior to change:** D-1 to D-12.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config: `interface ... unit ... ipv6 dhcpv6 { enabled true; duid ...; pd { length ... } }` (`ze-iface-conf.yang`).
- Wire: DHCPv6 Advertise and Reply on UDP 546 of the OS device's link-local address.

### Transformation Path
1. `parseDHCPv6Config` (`internal/component/iface/config.go`) → `dhcpv6UnitConfig`.
2. `reconcileDHCP` (`internal/component/iface/register.go`) → `dhcpParams` → `dhcpClientFactory` → `newDHCPClient` (`internal/plugins/iface/dhcp/register.go`).
3. `runV6`: resolve OS name → lease state machine (Solicit/Request → Bound → Renew → Rebind → Expired) → each exchange runs the Section 15 engine over one `nclient6.Client` bound to the OS name.
4. Reply → validation (§16.10, §21.4, §21.6) → lease record update (§18.2.10.1) → address install with the lease's lifetimes (`iface.ReplaceAddressWithLifetime`) or removal → event on the bus → resolv.conf.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| iface component ↔ dhcp plugin | `dhcpClientFactory` function variable, `iface.DHCPStopper` | Yes (`dhcpClientFactory`, `reconcileDHCP` in register.go) |
| dhcp plugin ↔ kernel | `iface.ReplaceAddressWithLifetime` / `RemoveAddress` → netlink backend | Yes (`ReplaceAddressWithLifetime`, `RemoveAddress` in dispatch.go) |
| dhcp plugin ↔ event bus | `publishDHCP` → `ze.EventBus.Emit` (iface namespace) | Yes (`publishDHCP` in dhcp_linux.go) |
| Ze ↔ DHCPv6 server | UDP 546/547 via vendored `nclient6` | Yes (`New`, `SendAndRead` in client.go) |

### Integration Points
- `iface.Resolve` / `binding.OsName` - socket device for every exchange, re-resolved per exchange.
- `internal/core/clock.Clock` - lease and retransmission timing.
- `iface.ReplaceAddressWithLifetime` - lifetime install.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | addresses still go through `iface` dispatch helpers; sockets through `nclient6` |
| No unintended coupling (components stay isolated) | Yes | all new code inside `internal/plugins/iface/dhcp/` |
| No duplicated functionality (extends existing, does not recreate) | Yes | reuses library codec and socket; replaces only the exchange layer Ze must own |
| Zero-copy preserved where applicable (refs, not copies) | N-A | control plane, a few messages per lease period |
| Registration over hardcoding, outbound | Yes | no new command, family or handler; the plugin's existing factory registration is the only entry |
| Registration over hardcoding, inbound | Yes | searched iface `register.go`, `config.go`, the YANG: no list learns a new name; message types and option codes are the library's constants |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The cited client sections and Table 1 values are the same in RFC 9915 as in RFC 8415 | heading and constant comparison of both texts, 2026-10-18 | quotes and timers differ by document | enrolment forward walk of the chosen document | unvalidated |
| A-2 | `nclient6.Client` with `WithRetry(1)` sends exactly one datagram per `SendAndRead` and reuses `msg.TransactionID` | `retryFn`, `send` (vendored client.go) | retransmissions double or change xid | unit test with a recording `PacketConn` via `NewWithConn` | unvalidated |
| A-3 | A matcher that returns false keeps `SendAndRead` waiting until RT, so it can collect Advertise messages | `SendAndRead` loop (vendored) | Advertise collection needs its own receive path | unit test over `NewWithConn` | unvalidated |
| A-4 | The kernel performs DAD on addresses `ReplaceAddressWithLifetime` installs (no IFA_F_NODAD), satisfying the §18.2.10.1 DAD MUST | netlink backend sets no NODAD flag | §18.2.10.1 DAD unmet | read `manage_linux.go` flags; integration test sees `tentative` then clear | unvalidated |
| A-5 | Re-creating the `nclient6.Client` per exchange is harmless (port 546 rebind succeeds immediately) | today's code already does it per solicit | EADDRINUSE between exchanges | integration test runs at least 3 exchanges back to back | unvalidated |
| A-6 | An infinite-lifetime lease is protected from the reconcile by the owned-address work in another session | owner brief 2026-10-18; `learnedAddrSet` spares finite only | an infinite lease is stripped on commit | that session's test; this spec's AC-10 asserts only the installed lifetime | unvalidated |
| A-7 | No consumer outside resolv.conf uses DHCPv6-provided DNS, so "stop using" means rewriting resolv.conf without them | `handleV6Reply` is the only writer | other consumers keep stale servers | grep `DNSAll` consumers | unvalidated |
| A-8 | `dhcp-expired` consumers accept a payload with PrefixLength other than 128 (D-12) | `DHCPPayload` already carries prefixes on acquired/renewed | a consumer mis-handles PD expiry | grep `EventDHCPExpired` subscribers | unvalidated |
| A-9 | Kea (`kea-dhcp6`) runs in the interop container with valid lifetimes of tens of seconds and explicit T1/T2 | Kea config schema (`renew-timer`, `rebind-timer`, `valid-lifetime`) | the scenario needs minutes per cycle | build the Dockerfile, run the scenario | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Changing the DUID from per-Solicit DUID-LLT to a stable DUID makes every server hand out a new binding once at upgrade | the address changes on first boot after upgrade | expected and one-time; stated in the commit body and the interfaces page |
| R-2 | Rapid Commit Reply and a later Reply to Request both arrive | duplicate install | §18.2.1 lets the client keep either; keep the first processed, discard the other |
| R-3 | A Reply arriving between two transmissions of one exchange is dropped by the library (no pending xid) | rare missed reply, exchange retransmits | acceptable: the next transmission draws the same answer (same xid); covered by a scripted-server unit test |
| R-4 | `Stop` during a Renew/Rebind wait must stay prompt | reconcile blocks | every wait goes through `sleepOrStop` or a stop-aware context; AC-24 |
| R-5 | Lifetimes counted from Reply reception: a slow processing path shortens real lifetimes | kernel lifetime smaller than the server's by the processing delay | install on reception; the delay is milliseconds |
| R-6 | RAND and xid randomness in tests | flaky timings | inject the random source; tests assert ranges, the engine test pins RAND |
| R-7 | Advertise collection holds the first RT (about 1.1 s) even with one server | slower first acquisition | RFC-mandated; a Rapid Commit Reply still short-circuits |
| R-8 | Rate limit (20 per 20 s) starves a legitimate burst (NoBinding → Request right after Renew) | Request delayed | the bucket admits it: a Renew plus a Request is 2 of 20 |
| R-9 | v4 and v6 workers share `DHCPClient` fields | data race on new lease state | the v6 lease record lives in the v6 worker goroutine only; no shared mutable state added |
| R-10 | Server sends T1/T2 infinite with finite lifetimes | lease expires with no renewal | RFC behavior (§7.7); the client follows it and logs once at Info that the server forbids renewal |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | DHCPv6-addressed uplinks lose or never obtain their address; IPv4 unaffected |
| How is it reverted? | single commit revert; no config migration (the `duid` pattern only refuses values the client could not use) |
| Who else touches this path? | PD spec (same file), v4 lease spec (`dhcp_linux.go` clock), owned-address session (reconcile), DHCPv6 server spec (enrolment rows) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `dhcpv6 { enabled true }` on a selector-bound unit, Go server on a veth answers Renew | → | `runV6` → renew exchange on `binding.OsName` | `TestDHCPv6RenewsOnSelectorBoundInterface` (`internal/plugins/iface/dhcp/dhcp_v6_lease_integration_linux_test.go`) |
| ze.conf `dhcpv6 { enabled true }` against kea-dhcp6 | → | full lease cycle: Solicit, Request, Renew three times, Rebind | interop scenario `dhcpv6-client-renew-kea` |
| config `dhcpv6 { pd { length 56 } }` | → | message builder adds the IA_PD and its hint (AC-25) | `TestDHCPv6PDLengthRequestsDelegatedPrefix` (`dhcp_v6_lease_linux_test.go`) |
| config `duid` leaf | → | Client Identifier in every message | `TestDHCPv6ConfiguredDUIDReachesEveryMessage` (`dhcp_v6_lease_linux_test.go`) |

## Acceptance Criteria

Timing ACs run on the injected clock with a scripted transport; wire ACs decode the bytes the client sent.

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Lease with T1 10 s, T2 16 s, valid 30 s; clock reaches T1 | Client sends RENEW (type 5) with Client ID equal to the lease's, Server ID equal to the leasing server's DUID, Elapsed Time 0, the lease's IA_NA IAID and every leased IA Address; no SOLICIT is sent |
| AC-2 | Server answers every Renew; clock runs 5 lease periods | No address removal, no `dhcp-expired`, one `dhcp-renewed` per Reply, installed lifetimes refreshed each time |
| AC-3 | Server ignores Renew | At the earliest T2 the Renew exchange ends and REBIND (type 6) is sent to ff02::1:2 with no Server ID and the same IAs |
| AC-4 | No server answers Renew or Rebind | When all valid lifetimes have expired: addresses removed, one `dhcp-expired` per lease (IA_NA address and IA_PD prefix), then a SOLICIT exchange starts |
| AC-5 | Any exchange retransmits | RT sequence obeys §15 (each RT within ±10% of twice the previous, first RT within IRT ±10%, capped at MRT ±10%); xid unchanged within an exchange, new per exchange; Elapsed Time equals hundredths of a second since the first transmission, 0xffff past its range |
| AC-6 | No server on the link | Solicit first transmission delayed uniformly in [0, 1 s]; first RT strictly greater than 1 s; RT grows to SOL_MAX_RT (3600 s) and stays; never stops; ORO carries option 82 |
| AC-7 | Advertise or Reply carries SOL_MAX_RT 60 / 86400 / 59 / 86401 | 60 and 86400 replace SOL_MAX_RT for that interface; 59 and 86401 ignored; processed even when the Reply carries a failure status |
| AC-8 | 2 hours with no server, then a server appears | Exactly one Warn naming the interface, the failed exchange and the next attempt; retransmissions at Debug; one Info when the lease is obtained |
| AC-9 | Logical name differs from the OS device | Every exchange, Renew and Rebind included, binds its socket to the OS device |
| AC-10 | Reply with valid V, preferred P (finite), and a Reply with 0xffffffff | Kernel address lifetimes equal V and P at install and after each Renew; 0xffffffff installs as forever |
| AC-11 | Reply (initial or renew) carries a lease with valid lifetime 0 | Lease discarded, address removed if present, never installed |
| AC-12 | Address with preferred > valid; IA with T1 > T2 (both > 0) | Address not installed; IA discarded and the rest of the message processed |
| AC-13 | IA with T1 = T2 = 0, shortest preferred lifetime P | T1 = 0.5 P and T2 = 0.8 P; never transmitted immediately; infinite P means no renewal |
| AC-14 | IA_NA T1 100 s, IA_PD T1 40 s | Renew at 40 s carrying both IAs |
| AC-15 | T1 0xffffffff; T2 0xffffffff | No Renew ever sent; no Rebind ever sent |
| AC-16 | Renew Reply with NoBinding in one IA | Request sent to the responding server with all IAs; other bindings kept |
| AC-17 | Renew Reply omits an IA the client holds | Recorded leases of that IA kept unchanged; retry obeys the rate limit |
| AC-18 | Reply without Server ID; Reply with a foreign Client ID; Reply with an unknown option; unknown message type | First two discarded, third processed with the option ignored, fourth discarded |
| AC-19 | Any sequence of exchanges on one interface | No more than 20 transmissions in any 20 s window per interface |
| AC-20 | Default and configured DUID | Same Client ID in every message across exchanges, worker restarts and daemon restarts; default form per Q4; configured `duid` used verbatim; a malformed `duid` refused at commit with a message naming the expected form |
| AC-21 | Release on stop (per Q5) | On `Stop` with a lease: addresses removed first, then RELEASE (type 8) with Server ID, Client ID and the leases, REL_TIMEOUT and REL_MAX_RC, bounded so `Stop` returns within the bound Q5 fixes |
| AC-22 | Two Advertise, preferences 10 and 200; one with 255 | Request goes to preference 200 after the first RT; a 255 Advertise triggers Request at once; a Rapid Commit Reply without the Rapid Commit option is discarded |
| AC-23 | Request unanswered | REQ_TIMEOUT and REQ_MAX_RT, at most 10 transmissions, then a new Solicit exchange |
| AC-25 | `dhcpv6 { pd { length 56 } }` configured (D-13) | Every Solicit, Request, Renew and Rebind carries an IA_PD with its own IAID, distinct from the IA_NA IAID, and an IA Prefix hint of prefix-length 56 with `::`; `pd {}` with no `length` sends an IA_PD with no hint; no `pd` sends no IA_PD |
| AC-24 | `Stop` during any wait (Solicit, Bound, Renew, Rebind) | Worker exits promptly on the stop signal, leased addresses removed |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Configures `dhcpv6 { enabled true }` on an uplink served by Kea | config → reconcileDHCP → runV6 → Solicit/Request → install → Renew at T1 repeatedly | `dhcpv6-client-renew-kea` |
| 2 | Server changes identity (Renew goes unanswered) | Renew to T2 → Rebind → new server's Reply → lease kept | `dhcpv6-client-renew-kea` (rebind phase) |
| 3 | Uplink has no DHCPv6 server for hours | Solicit backoff to 3600 s, one Warn | `TestDHCPv6SolicitBacksOffToSolMaxRT` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRFC8415Section15RetransmissionTimeout` | `internal/plugins/iface/dhcp/dhcp_v6_exchange_linux_test.go` | AC-5 RT sequence, MRT cap, MRC/MRD end | |
| `TestRFC8415Section15ElapsedTimeUpdated` | same | AC-5 elapsed time and 0xffff saturation, xid unchanged | |
| `TestRFC8415Section1821FirstSolicitRTAboveIRT` | same | AC-6 first RT > IRT, delay in [0, 1 s], ORO 82 | |
| `TestDHCPv6SolicitBacksOffToSolMaxRT` | same | AC-6, AC-8 one Warn over 2 h | |
| `TestRFC8415Section2124SolMaxRTRange` | `internal/plugins/iface/dhcp/dhcp_v6_lease_linux_test.go` | AC-7 | |
| `TestRFC8415Section1824RenewAtT1CarriesLease` | same | AC-1 | |
| `TestDHCPv6LeaseHeldAcrossFiveRenewals` | same | AC-2 | |
| `TestRFC8415Section1825RebindAtT2WithoutServerID` | same | AC-3 | |
| `TestRFC8415Section1825ExpiryRemovesAndSolicits` | same | AC-4, D-12 | |
| `TestDHCPv6InstalledLifetimeEqualsLease` | same | AC-10 incl. infinity | |
| `TestRFC8415Section182101ValidZeroDiscarded` | same | AC-11 | |
| `TestRFC8415Section216PreferredAboveValidDiscarded` | same | AC-12 address | |
| `TestRFC8415Section214T1AboveT2DiscardsIA` | same | AC-12 IA | |
| `TestRFC8415Section142ZeroTimersChosenByClient` | same | AC-13 | |
| `TestRFC8415Section1824EarliestT1AcrossIAs` | same | AC-14 | |
| `TestRFC8415Section77InfiniteT1NeverRenews` | same | AC-15 | |
| `TestRFC8415Section182101NoBindingSendsRequest` | same | AC-16 | |
| `TestRFC8415Section182101OmittedIAKept` | same | AC-17 | |
| `TestRFC8415Section1610ReplyValidation` | `dhcp_v6_exchange_linux_test.go` | AC-18 | |
| `TestRFC8415Section141PerInterfaceRateLimit` | same | AC-19 | |
| `TestDHCPv6ConfiguredDUIDReachesEveryMessage` | `dhcp_v6_lease_linux_test.go` | AC-20 configured | |
| `TestDHCPv6DefaultDUIDStable` | same | AC-20 default | |
| `TestDHCPv6ReleaseOnStop` | same | AC-21 | |
| `TestRFC8415Section1821AdvertiseSelection` | `dhcp_v6_exchange_linux_test.go` | AC-22 | |
| `TestRFC8415Section1822RequestGivesUpAfterMRC` | same | AC-23 | |
| `TestDHCPv6StopDuringEveryWait` | `dhcp_v6_lease_linux_test.go` | AC-24 | |
| `TestDHCPv6DUIDPatternRefusesMalformed` | `internal/component/iface/config_test.go` | AC-20 commit refusal | |

Test names carry `RFC8415`; if Q1 enrols RFC 9915 they carry `RFC9915` instead.

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| SOL_MAX_RT option | 60-86400 | 60, 86400 | 59 | 86401 |
| Elapsed Time | 0-0xffff (hundredths) | 0xfffe | N/A | saturates at 0xffff |
| T1 / T2 | 0-0xffffffff | 0xfffffffe finite, 0xffffffff infinity | N/A | N/A |
| valid / preferred lifetime | 0-0xffffffff | 1 | 0 (discard) | N/A (0xffffffff infinity) |
| RAND | [-0.1, +0.1] | ±0.1 | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `TestDHCPv6RenewsOnSelectorBoundInterface` | `internal/plugins/iface/dhcp/dhcp_v6_lease_integration_linux_test.go` (integration, veth lab, Go server) | selector-bound uplink keeps its lease across three Renews and one Rebind; kernel lifetimes match | |
| `.ci` | N-A | the `.ci` harness has no DHCPv6 server; the user path from config is proven by the interop scenario below (A-9) | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `dhcpv6-client-renew-kea` | `test/interop/scenarios/dhcpv6-client-renew-kea/` and `test/interop/Dockerfile.kea` | Kea (`kea-dhcp6`), valid 40 s, renew-timer 10 s, rebind-timer 20 s | Ze holds one address for at least 3 lease periods with Kea logging RENEW from one DUID; after Kea restarts with a new server-id, Ze rebinds and keeps the address; kernel lifetimes at most 40 s. Discrimination: with renewal restored to RapidSolicit the scenario goes red (Kea logs SOLICIT, the address drops) | |

## Files to Modify
- `internal/plugins/iface/dhcp/dhcp_v6_linux.go` - `runV6` becomes the lease state machine; `renewV6` and `v6Timers` deleted (no-layering); `handleV6Reply` validates and applies to the lease record; `removeV6Addrs` and `publishV6Expired` iterate the lease record and include IA_PD
- `internal/plugins/iface/dhcp/dhcp_linux.go` - `clock.Clock` and random-source fields on `DHCPClient`; `sleepOrStop` reads the clock; DUID read from `dHCPConfig.DUID`
- `internal/plugins/iface/dhcp/register.go` - pass `clock.RealClock{}`
- `internal/component/iface/yang/ze-iface-conf.yang` - `duid` pattern and a description that says what Ze sends by default (per Q4)
- `docs/features/interfaces.md` - DHCPv6 Renew row to `have`, DUID behavior, retransmission and backoff sentence, source anchors
- `rfc/short/<stem>.md`, `rfc/extraction/<stem>.json`, `rfc/enrolled.txt`, `rfc/discrimination/<stem>.json` - enrolment (Q1, Q2)
- `plan/journal/silent-fall-through.md` - the 2026-10-18 row status to fixed at closure

## Files to Create
- `internal/plugins/iface/dhcp/dhcp_v6_lease_linux.go` - lease record (DUID, server DUID, IAs, leases, reception time), Reply application (§18.2.10.1), timer selection (§14.2, §18.2.4, §21.4)
- `internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go` - message builders (Solicit, Request, Renew, Rebind, Release), §15 engine, §16.10 matcher, Advertise collector, per-interface token bucket
- `internal/plugins/iface/dhcp/dhcp_v6_lease_linux_test.go`, `dhcp_v6_exchange_linux_test.go`, `dhcp_v6_lease_integration_linux_test.go`
- `test/interop/Dockerfile.kea`, `test/interop/scenarios/dhcpv6-client-renew-kea/` (Kea config, ze.conf, assertions)

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | no new node; the existing `duid` leaf gains a pattern |
| YANG validation constraints | Yes | `ze-iface-conf.yang` leaf `duid`: pattern for colon-separated hex octets, at most 130 octets (2-octet type plus at most 128 octets of identifier) |
| YANG custom validators | No | the pattern suffices |
| CLI commands/flags | No | no command |
| CLI grammar (keyword before value) | N-A | no command |
| Editor autocomplete | N-A | string leaf |
| Functional test for new RPC/API | N-A | no RPC |
| Pipe completeness | N-A | no output |
| Env var registration | No | rate limit and constants fixed (Q6) |
| Doctor check for runtime dependencies | No | no new path, socket, port or module: UDP 546 and the netlink address calls already exist |
| Prometheus counters/metrics | No | none added |
| BGP family surface (new SAFI / capability / attribute) | N-A | not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features/interfaces.md` (DHCPv6 Renew row) |
| 2 | Config syntax changed? | Yes | `duid` accepted form, in `docs/features/interfaces.md` and the YANG description |
| 3 | CLI command added/changed? | No | none |
| 4 | API/RPC added/changed? | No | none |
| 5 | Plugin added/changed? | No | plugin surface unchanged |
| 6 | Has a user guide page? | No | grep `docs/guide` for dhcpv6 at implementation; update any page that names renewal |
| 7 | Wire format changed? | Yes | messages Ze sends change; documented in the RFC summary, not `docs/architecture/wire/` (BGP wire docs) |
| 8 | Plugin SDK/protocol changed? | No | none |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/<stem>.md` (Q1) and its `docs/features/rfc-status.md` row via the Meta `Support` row |
| 10 | Test infrastructure changed? | Yes | `docs/architecture/testing/interop.md` gains the Kea daemon and the scenario |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` if it lists DHCPv6 client renewal; check at implementation |
| 12 | Internal architecture changed? | No | inside one plugin |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | No | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | events unchanged (PD expiry uses the existing event) |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | run `./le spec citation anchors spec plan/immediate/spec-dhcpv6-client-lease-maintenance.md`; `docs/features/interfaces.md` is the `// Design:` doc of both dhcp files |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | verify `dhcpv6` examples in `docs/features/interfaces.md` against the YANG |

## Implementation Steps

1. **Phase: Enrolment and wiring** -- enrol the chosen RFC per `/ze-rfc` (whole document, Q1 and Q2), extraction sign-off; add the clock and random-source fields; write `TestDHCPv6RenewsOnSelectorBoundInterface` and the timing tests against a stub state machine
   - Tests: wiring table rows
   - Files: `rfc/`, `dhcp_linux.go`, `register.go`
   - Verify: tests fail on the stub
2. **Phase: Exchange engine** -- builders, §15 engine, matcher, Advertise collector, rate limit
   - Tests: every `dhcp_v6_exchange_linux_test.go` row
   - Files: `dhcp_v6_exchange_linux.go`
3. **Phase: Lease record and state machine** -- `runV6` rewrite, Reply application, timer selection, expiry, DUID, PD expiry; delete `renewV6` and `v6Timers`
   - Tests: every `dhcp_v6_lease_linux_test.go` row
   - Files: `dhcp_v6_lease_linux.go`, `dhcp_v6_linux.go`
4. **Phase: Release on stop** (per Q5)
5. **Phase: YANG `duid` pattern and docs** -- the page edits land with the code they describe
6. **Phase: Interop** -- `Dockerfile.kea`, `dhcpv6-client-renew-kea`, discrimination walk recorded

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | No call to `RapidSolicit`, `Solicit`, `Request` or `NewSolicit` remains under `internal/`; no `ReplaceAddressWithLifetime` with a valid lifetime of 0 |
| Correctness | Every socket built from `binding.OsName`, never `c.ifaceName` |
| Data flow | Lease state owned by the v6 goroutine only |
| Rule: rfc-compliance | Every implementing function quotes its section; every caller names it |
| Rule: no-layering | `renewV6` and `v6Timers` deleted, no fallback to the old loop |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| No library exchange calls left | grep for `RapidSolicit`, `.Request(ctx` and `NewSolicit` under `internal/plugins/iface/dhcp/` returns nothing outside tests |
| Enrolment | `./le rfc check` green for the stem |
| Interop | scenario `dhcpv6-client-renew-kea` green, red recorded on revert |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | Reply from an on-link attacker: Server ID required, Client ID match, xid random per exchange, caps of 16 addresses and 16 prefixes kept |
| Resource exhaustion | Advertise collector bounded (cap on stored Advertise messages); rate limit bounds Ze's own transmissions |
| Reconfigure | Not supported: Reconfigure messages are discarded (no Reconfigure Accept is sent) |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Open Questions for the Owner

Answered 2026-10-18. Thomas, verbatim: "For the rest it looks good to me", accepting
every recommendation below.

| Q | Which way | Recommendation | Owner answer (2026-10-18) |
|---|-----------|----------------|---------------------------|
| Q1 | RFC 8415 is obsoleted by RFC 9915 (January 2026). Enrol RFC 9915 as the authority (8415 not enrolled, or enrolled with an `Obsoleted-by` row and a `{superseded}` marker on every line), or enrol RFC 8415 as named? | Enrol RFC 9915 and quote it in code; do not enrol 8415 (`ai/rules/rfc-compliance.md`: an RFC the current one obsoletes is not evidence of what Ze owes). The decision "Include RFC 8415" then reads as "the DHCPv6 standard" | Recommendation accepted: enrol RFC 9915 and quote it in code; "Include RFC 8415" reads as "the DHCPv6 standard" |
| Q2 | Enrolment covers the whole document (forced by the forward walk). Server and relay sections: exclude as `binds-another-role` (Ze has no DHCPv6 server or relay today; `internal/plugins/dhcpserver` is DHCPv4 only), or record them as gaps until `plan/spec-dhcpv6-server.md` lands? | Exclude as `binds-another-role` naming the absent role; the server spec re-walks them. Client features this spec does not build (Confirm, Information-request, Reconfigure, IA_TA, authentication) are recorded as gaps or `feature-out-of-scope`, never as met | Recommendation accepted: server and relay sections `binds-another-role` naming the absent role; unbuilt client features recorded as gaps or `feature-out-of-scope` |
| Q3 | Fold `spec-dhcpv6-client-never-requests-a-delegated-prefix` into this spec (one IA_PD row in the builder), or keep it separate with `Depends` on this spec? | Fold it in: the builder this spec writes is the only place the IA_PD request can go, and §18.2 asks the client to keep requesting every IA it is programmed to obtain | Recommendation accepted: folded in as D-13, AC-25 and its wiring row; the skeleton spec file is removed in the commit that records these answers |
| Q4 | Default DUID: DUID-LL from the OS device's MAC (what the YANG description already promises; §11.4 says it SHOULD NOT be used when the client cannot tell the NIC is permanently attached), DUID-LLT generated once and kept in Ze storage (§11.2 needs stable storage), or DUID-UUID? | DUID-LL from the MAC: keeps the published promise, needs no storage, stable across restarts; the SHOULD NOT is recorded on the summary as a known deviation and the `duid` leaf is the operator's override | Recommendation accepted: DUID-LL from the MAC, the SHOULD NOT recorded as a deviation, the `duid` leaf is the override |
| Q5 | Release on stop (§18.2.7): send it with a bounded wait (REL_MAX_RC 4 is up to about 15 s; cap `Stop` at about 3 s), or leave it out and record a gap (v4 sends no DHCPRELEASE either)? | Send it with a bounded wait; Decline (§18.2.8, needs DAD-failure observation) goes to its own spec | Recommendation accepted: Release on stop with a bounded wait of about 3 s |
| Q6 | §14.1 "A transmission rate limit SHOULD be configurable": add a YANG leaf now, or fixed 20 per 20 s and record the SHOULD as a gap? | Fixed default now, gap recorded | Recommendation accepted: fixed 20 per 20 s, gap recorded |
| Q7 | Should `spec-dhcpv4-client-drops-lease-every-period` depend on this spec for the injected clock, or land its own clock change first? | Whichever spec lands first adds the `clock.Clock` field; the other reuses it. No shared state machine | `plan/immediate/spec-dhcpv4-client-drops-lease-every-period.md` (in design on 2026-10-18) adds the injected `clock.Clock`; this spec reuses it. No shared state machine |

## Design Insights

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Ze owns the exchange (§15 engine, builders, matcher) over the vendored socket and codec | (a) patch the vendored `nclient6`; (b) write a Ze socket and codec | (a) vendor patches are lost on update and the library has no Renew or Rebind; (b) duplicates a working codec and receive loop. `SendAndRead` with retry 1 and a custom matcher gives every hook the RFC needs |
| Lease record owned by the v6 goroutine, explicit states (Solicit, Request, Bound, Renew, Rebind) | keep the linear sleep chain | the chain cannot express "renew again" or "keep leases the Reply omitted"; the record is what Renew is built from |
| No shared v4/v6 state machine; shared clock only | one generic lease machine for both | RFC 2131 retransmission (half the remaining time, 60 s floor) and RFC 8415 §15 differ; a generic machine would carry per-protocol switches |
| Remove an address whose lease reaches valid 0, never install 0 and 0 | rely on the kernel | the netlink library makes 0 and 0 permanent (D-7) |
| Always multicast; ignore Server Unicast | honour Server Unicast (§18.2 SHOULD) | multicast is always valid (§14); unicast needs a source address of sufficient scope and is an optimisation; recorded as a SHOULD gap |

## Known Limitations
- Confirm on link change (§18.2.3), Information-request (§18.2.6), Reconfigure (§18.2.11), IA_TA and authentication: not built; recorded as gaps on the summary (Q2).
- Decline on DAD failure (§18.2.8): separate spec (Q5).
- Server Unicast (§18.2 SHOULD): not used.
- Lease persistence across a daemon restart: not built; a restart re-solicits with the same DUID, so the server returns the same binding.

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code, with
the enrolled document's number (Q1). Every implementing function quotes the
sentence it performs; every caller names the section.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/immediate/spec-dhcpv6-client-lease-maintenance.md` only, in the same `./le commit create` script
