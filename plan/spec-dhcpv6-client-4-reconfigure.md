# Spec: dhcpv6-client-4-reconfigure

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | protocol |
| Depends | spec-dhcpv6-client-1-gap-annotations (closed 2026-10-19), `plan/spec-dhcpv6-client-3-stateless.md` |
| Phase | 4/4 of `plan/spec-dhcpv6-client-0-umbrella.md` |
| Handoff | - |
| Updated | 2026-10-19 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

A DHCPv6 server sends Reconfigure to make a client renew, rebind or refresh
its configuration at once, instead of waiting for T1 or the refresh time.
Because a forged Reconfigure could steer a client, RFC 9915 requires it to be
authenticated: the server hands the client a reconfigure key in a Reply
(RKAP, Section 20.4), and every Reconfigure carries an HMAC-MD5 under that key
and a replay detection counter (Sections 20.3 and 21.11). In RFC 9915 this is
the whole of DHCP authentication a client performs; delayed authentication
was removed by RFC 8415 (RFC 9915 Section 1.1).

Ze today sends no Reconfigure Accept, holds its socket open only during its
own exchanges, and discards every Reconfigure. The owner put Reconfigure and
authentication in scope on 2026-10-19. This child adds a `reconfigure-accept`
leaf; with it set, ze announces Reconfigure Accept, records the reconfigure
key and replay counter of the server, listens on UDP 546 for as long as it
holds a lease or stateless configuration, validates every Reconfigure against
the six conditions of Section 16.11 and RKAP, and answers a valid one with
the Renew, Rebind or Information-request it asks for. It closes the seven
`{gap}` rows RFC9915-15-2, RFC9915-18.2.4-2, RFC9915-18.2.6-3,
RFC9915-18.2.11-2, RFC9915-20.3-3, RFC9915-20.3-4, RFC9915-20.2-1, and adds
the missing polarity to RFC9915-16.11-2, RFC9915-18.2-1, RFC9915-18.2.11-1 and
RFC9915-22.3-1, whose `{single-polarity}` reason ("ze accepts no Reconfigure
at all") stops being true.

## Required Reading

### Architecture Docs
- [ ] `docs/features/interfaces.md` - DHCPv6 paragraphs
  → Constraint: the page describes one socket per exchange; the persistent listening link is a change of architecture the page states in the same change
- [ ] `ai/rules/goroutine-lifecycle.md` - before any new goroutine
  → Constraint: no goroutine per datagram; the receive path that hands Reconfigure to the worker is the existing per-link read loop of the vendored client, or one long-lived reader per link, ending when the link closes
- [ ] `ai/rules/performance.md` - buffer rules
  → Constraint: the HMAC check runs over the received datagram with the HMAC field zeroed in a copy taken from a bounded buffer, never a per-message `make` beyond the one datagram buffer the read already owns

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc9915.md` - the seven rows, the four single-polarity rows, RFC9915-21.7-1
  → Constraint: removing a `{single-polarity}` annotation is allowed only together with a test of the missing polarity; the ratchet blocks a lost polarity, not a gained one
- [ ] `rfc/full/rfc9915.txt` - Sections 15, 16.11, 18.2, 18.2.4, 18.2.5, 18.2.6, 18.2.11, 20, 21.11, 21.19, 21.20, 22.3, Appendix B
  → Constraint: Section 15 "a client MUST listen for a Reconfigure if it has negotiated for its use with the server"
  → Constraint: Section 16.11, a client MUST discard a Reconfigure that was not unicast to it, has no Server Identifier, has no Client Identifier holding its DUID, has no Reconfigure Message option, carries an invalid msg-type, or "does not include authentication (such as RKAP; see Section 20.4) or fails authentication validation"
  → Constraint: Section 21.19 msg-type "5 for Renew message, 6 for Rebind message, 11 for Information-request message"
  → Constraint: Section 21.20 "In the absence of this option, the default behavior is that the client is unwilling to accept Reconfigure messages"; Appendix B allows it in Solicit, Request, Renew, Rebind, Information-request and Reply
  → Constraint: Section 20.4.1 RKAP sets protocol 3, algorithm 1, RDM 0; authentication information is a 1-octet Type (1 reconfigure key value in a Reply, 2 HMAC-MD5 digest in a Reconfigure) and a 16-octet Value
  → Constraint: Section 20.4.3 "the client computes an HMAC-MD5 over the Reconfigure message, with zeroes substituted for the HMAC-MD5 field, using the reconfigure key received from the server. If this computed HMAC-MD5 matches the value in the Authentication option, the client accepts the Reconfigure message"
  → Constraint: Section 20.3 "A client that receives a message with the RDM field set to 0x00 MUST compare its replay detection field with the previous value sent by that same server ... and only accept the message if the received value is greater and record this as the new value. If this is the first time a client processes an Authentication option sent by a server, the client MUST record the replay detection value and skip the replay detection check"
  → Constraint: Section 20.2 "Any DHCP message MUST NOT include more than one Authentication option"
  → Constraint: Section 18.2.11 "the client MUST ignore any additional Reconfigure messages until the exchange is complete"; "The client ignores the "transaction-id" field in the received Reconfigure message"; "the client SHOULD log these events"
  → Constraint: Section 18.2 "it MUST NOT abandon any resources it might have already obtained. The client SHOULD treat the Reconfigure message as if the T1 timer had expired"
  → Constraint: Section 18.2.6 "When responding to a Reconfigure, the client MUST include a Server Identifier option ... with the identifier from the Reconfigure message"
  → Constraint: Section 18.2.4 "A client MUST also initiate a Renew/Reply message exchange before time T1 if the client's link-local address used in previous interactions with the server is no longer valid and it is willing to receive Reconfigure messages"
  → Constraint: Section 18.2.5 "If the terminated Rebind exchange was initiated as a result of receiving a Reconfigure message, the client terminates the reconfigure process and resumes as if the Reconfigure message had not been received"

**Key insights:**
- The vendored `nclient6.Client` drops every datagram whose transaction id has no pending exchange (`receiveLoop`), and a Reconfigure's transaction id is the server's, so the library never delivers one. It accepts a caller's `net.PacketConn` (`NewWithConn`), which is where ze can take Reconfigure datagrams, raw, before the library sees them.
- The HMAC is over the bytes as received; re-encoding a parsed message is not guaranteed to reproduce them, so validation needs the raw datagram.
- The socket binds the link-local unicast address (`NewIPv6UDPConn`); a datagram to a multicast group is not delivered to a socket bound to a unicast address, so "unicast to the client" is enforced by the bind and must be proven by a test that sends one to ff02::1.
- Kea 2.6.2 never sends Reconfigure (umbrella A-2). OpenWrt odhcpd does: `send_reconf` builds a Reconfigure with msg-type Renew, an Authentication option protocol 3 algorithm 1, reconf type 2 and an HMAC-MD5 under the per-lease key, and `handle_addrlist_change` triggers it when the server's interface addresses change, for a client that sent Reconfigure Accept (umbrella A-3).

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go` - `v6Link`, `dialV6Link`, `nclient6Link.exchange`, `replyMatcher`, `newV6Message`, `buildSolicit`, `buildRequest`, `buildFromLease`
- [ ] `internal/plugins/iface/dhcp/dhcp_v6_linux.go` - `open`, `openOnce`, `closeV6Link`, every exchange opening and closing its own link; `maintain` dispatch; `extend`
- [ ] `internal/plugins/iface/dhcp/dhcp_v6_attach_linux.go` - `waitLease` select (timer, link events, Stop)
- [ ] `internal/plugins/iface/dhcp/rfc9915_dhcp_v6_messages_linux_test.go` - `TestRFC9915Section16ClientDiscardsOtherMessageTypes`: the Reconfigure negative cases and the RFC9915-18.2-1 positive case
- [ ] `vendor/github.com/insomniacslk/dhcp/dhcpv6/nclient6/client.go` - `receiveLoop`, `NewWithConn`, `NewIPv6UDPConn`

**Behavior to preserve:**
- with `reconfigure-accept` absent or false: no Reconfigure Accept in any message, a link per exchange as today, every Reconfigure discarded, no Authentication option read
- ze never sends an Authentication option (RKAP is server-initiated)
- one datagram per transmission (`TestDHCPv6LinkOneDatagramPerExchange`)

**Behavior to change:** with `reconfigure-accept true`, everything in the AC table.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config `dhcpv6 { reconfigure-accept true; }`.
- Wire: a Reply carrying Reconfigure Accept and an RKAP Authentication option of Type 1; later a Reconfigure unicast to UDP 546 on the link-local address.

### Transformation Path
1. Config parse sets `ReconfigureAccept` in the client config.
2. Every Solicit, Request, Renew, Rebind and Information-request carries a Reconfigure Accept option.
3. A Reply from the leasing server is checked for its Authentication option: one only, RKAP protocol 3, algorithm 1, RDM 0, Type 1, 16 octets; replay detection per Section 20.3; the key and the replay value are recorded against that server's DUID. Reconfigure is accepted from that server only while its last Reply carried both Reconfigure Accept and a key.
4. While accepting, the worker holds one link open from lease acquisition (or the first stateless Reply) to lease loss, Stop or a device change; the link's packet connection passes every datagram whose message type is Reconfigure, with its raw bytes and source, into a bounded channel the worker selects on in `waitLease`.
5. The worker validates: Server ID present and equal to the keyed server, Client ID equal to its DUID, Reconfigure Message option present with msg-type 5, 6 or 11, exactly one Authentication option, RKAP fields, Type 2, HMAC-MD5 over the raw bytes with the 16 HMAC octets zeroed equal to the carried value, replay value greater than recorded. Any failure discards it with one Debug line naming the condition.
6. A valid Reconfigure logs one Info line, records the replay value, and starts the exchange it names: Renew to that server as if T1 had passed (MRD to T2, then Rebind as usual), Rebind, or Information-request carrying that Server Identifier. Reconfigures arriving before that exchange ends are read and discarded.
7. When the device's link-local address changes while accepting, the link is reopened on the new address and a Renew starts before T1.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config to plugin | `dHCPConfig.ReconfigureAccept` | No |
| Wire to worker | packet connection wrapper under `nclient6.NewWithConn`, bounded channel | No |
| Plugin to wire | existing exchanges with one more option | No |

### Integration Points
- `waitLease` - a third wake source, Reconfigure.
- `extend` - Renew and Rebind started by a Reconfigure use it unchanged.
- child 3's Information-request builder - with a Server Identifier when answering a Reconfigure.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | exchanges still go through `runExchange` and the vendored codec |
| No unintended coupling (components stay isolated) | Yes | plugin-internal plus one YANG leaf |
| No duplicated functionality (extends existing, does not recreate) | Yes | no second socket or second engine |
| Zero-copy preserved where applicable (refs, not copies) | Yes | the raw datagram is handed over, one zeroed copy for the HMAC |
| Registration over hardcoding, outbound | Yes | schema-discovered leaf |
| Registration over hardcoding, inbound | Yes | searched `replyMatcher`, `newV6Message`, the builders: all the DHCPv6 client's own |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Reconfigure acceptance is opt-in with default false | umbrella Q1; the key travels in clear in the Reply | the default flips and every lease negotiates Reconfigure | owner answer | confirmed: owner decision 2026-10-19, "ok" (umbrella Q1) |
| A-2 | odhcpd builds and runs in a container without the rest of OpenWrt | odhcpd's CMake build makes ubus optional | no interop peer | `test/interop/Dockerfile.odhcpd` builds and the scenario runs | unvalidated |
| A-3 | odhcpd's HMAC is computed exactly as Section 20.4.2 says, over the whole message with the HMAC field zero | `send_reconf` in `src/dhcpv6-ia.c` | interop red for the peer's reason, not ze's | the scenario, cross-checked by a unit test vector built from a captured odhcpd datagram | unvalidated |
| A-4 | the kernel does not deliver a datagram to ff02::1 port 546 to a socket bound to the link-local unicast address | Linux UDP socket lookup | a multicast Reconfigure reaches validation | `TestRFC9915Section1611ReconfigureMulticastNotDelivered` in a network namespace | unvalidated |
| A-5 | `crypto/md5` with `crypto/hmac` is acceptable to the lint set | Go standard library | a gosec finding needs a justified suppression naming RFC 9915 Section 20.4 | lint run by the main thread | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | the persistent link breaks the per-exchange tests | existing link tests red | the persistent link exists only while accepting; the default path is unchanged |
| R-2 | a Reconfigure flood fills memory | channel full | bounded channel; when full the datagram is dropped with a rate-limited Debug line |
| R-3 | a forged Reconfigure drops a lease | negative tests | a Reconfigure only starts an exchange; nothing is removed on its account (Section 18.2) |
| R-4 | the key leaks to logs | grep of log calls | the key and HMAC are never logged; the Debug lines name the failed condition only |
| R-5 | odhcpd sends msg-type Renew only, so Rebind and Information-request responses have no interop proof | scenario coverage | they are proven on the rig with a datagram built to the Section 20.4.2 rule; the gap is stated in the Goal Validation |
| R-6 | the link-local change is missed because only link down and up are watched | rig | read the device's link-local on each address event the worker already receives, compare with the bound one |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | with the leaf set, a forged or stale Reconfigure accepted, or a valid one ignored; with it unset, nothing |
| How is it reverted? | single commit revert; configs carrying the leaf must drop it |
| Who else touches this path? | children 2 and 3 in the same files |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| config `reconfigure-accept true` | → | Reconfigure Accept in Solicit | `test/plugin/iface-dhcpv6-reconfigure-accept-config.ci` |
| authenticated Reconfigure on the rig | → | validation and Renew | `TestRFC9915Section18211ReconfigureTriggersRenew` |
| odhcpd on the wire | → | key record, Reconfigure, Renew | `dhcpv6-client-reconfigure-odhcpd` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `reconfigure-accept true` | Solicit, Request, Renew, Rebind and Information-request each carry one Reconfigure Accept option; with the leaf false none does |
| AC-2 | Reply from the leasing server with Reconfigure Accept and an RKAP Type 1 option | key and replay value recorded for that server's DUID; a first Authentication option skips the replay check (RFC9915-20.3-4) |
| AC-3 | later message from that server whose RDM 0 replay value is not greater than recorded | discarded; one greater is accepted and recorded (RFC9915-20.3-3) |
| AC-4 | lease held and accepting | the link stays open between exchanges and a Reconfigure sent between them is delivered to the worker (RFC9915-15-2) |
| AC-5 | valid Reconfigure, msg-type 5 | one Info log line; a Renew carrying that Server Identifier starts at once with the Renew parameters; its transaction id is the client's own |
| AC-6 | valid Reconfigure, msg-type 6 | a Rebind starts at once |
| AC-7 | valid Reconfigure, msg-type 11 | an Information-request carrying the Reconfigure's Server Identifier starts (RFC9915-18.2.6-3) |
| AC-8 | second valid Reconfigure while the first one's exchange runs | discarded; no second exchange (RFC9915-18.2.11-2) |
| AC-9 | Reconfigure with each Section 16.11 failure: no Server ID; Server ID of an unkeyed server; no Client ID; another DUID; no Reconfigure Message option; msg-type 1; no Authentication option; two Authentication options (RFC9915-20.2-1); protocol, algorithm, RDM or Type wrong; HMAC one bit off; replay value equal to recorded | discarded with a Debug line naming the condition; lease, timers and addresses unchanged (RFC9915-16.11-2, RFC9915-18.2.11-1, RFC9915-22.3-1, RFC9915-18.2-1) |
| AC-10 | Reconfigure sent to ff02::1 port 546 | never reaches validation |
| AC-11 | Reply whose server sent no Reconfigure Accept | no key recorded; every later Reconfigure from it discarded |
| AC-12 | Renew started by a Reconfigure unanswered until T2 | Rebind at T2; Rebind unanswered to lease expiry: discovery as after any expiry (Section 18.2.5) |
| AC-13 | the device's link-local address changes while accepting | the link reopens on the new address and a Renew starts before T1 (RFC9915-18.2.4-2) |
| AC-14 | `reconfigure-accept` false or absent | today's behavior: a Reconfigure is discarded, the link closes after each exchange, `TestDHCPv6LinkOneDatagramPerExchange` unchanged |
| AC-15 | any log line of this feature | never carries the key or an HMAC value |
| AC-16 | the 11 rows | the seven `{gap}` rows and the four `{single-polarity}` rows carry a positive and a negative tagged test each, with discrimination records; `Support remaining` count lowered by seven, Reconfigure and the Authentication option removed from its list |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | renumbers the network at the server and expects clients to pick it up at once | odhcpd Reconfigure, RKAP check, Renew, new lease | `dhcpv6-client-reconfigure-odhcpd` |
| 2 | an attacker on the link sends a forged Reconfigure | validation refusal | AC-9 tests on the rig |

### Planned Goal Validation
<!-- The evidence each goal owes at closure (ai/rules/interop-and-goal-validation.md); /ze-close copies it into the closure table. -->
| Goal | Evidence owed |
|------|---------------|
| a server can make ze renew at once | `dhcpv6-client-reconfigure-odhcpd` passing: after the checker changes odhcpd's interface addresses, odhcpd logs a sent Reconfigure and ze sends a RENEW within REC_TIMEOUT 2 s, long before T1; recorded red with the Reconfigure branch reverted |
| a forged Reconfigure changes nothing | the AC-9 negative tests, each with a discrimination record for RFC9915-16.11-2, RFC9915-18.2.11-1, RFC9915-22.3-1 |
| replay of a captured Reconfigure is refused | AC-3 and the equal-replay case of AC-9 |
| Rebind and Information-request responses | rig tests only (R-5), stated as such |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRFC9915Section2120ReconfigureAcceptInMessages` | `internal/plugins/iface/dhcp/rfc9915_dhcp_v6_messages_linux_test.go` | AC-1, AC-14 | |
| `TestRFC9915Section2043RecordReconfigureKey` | `internal/plugins/iface/dhcp/rfc9915_dhcp_v6_reconfigure_linux_test.go` | AC-2, AC-11 | |
| `TestRFC9915Section203ReplayDetection` | same | AC-3 | |
| `TestRFC9915Section15ListensBetweenExchanges` | same | AC-4 | |
| `TestRFC9915Section18211ReconfigureTriggersRenew` | same | AC-5 | |
| `TestRFC9915Section18211ReconfigureTriggersRebind` | same | AC-6 | |
| `TestRFC9915Section1826ReconfigureInformationRequestServerID` | same | AC-7 | |
| `TestRFC9915Section18211IgnoresReconfigureDuringExchange` | same | AC-8 | |
| `TestRFC9915Section1611ReconfigureDiscardConditions` | same | AC-9, one subtest per condition | |
| `TestRFC9915Section1611ReconfigureMulticastNotDelivered` | `internal/plugins/iface/dhcp/rfc9915_dhcp_v6_link_linux_test.go` | AC-10 | |
| `TestRFC9915Section1825ReconfigureRenewFallsToRebind` | `rfc9915_dhcp_v6_reconfigure_linux_test.go` | AC-12 | |
| `TestRFC9915Section1824LinkLocalChangeRenews` | same | AC-13 | |
| `TestDHCPv6ReconfigureLogsNoKey` | same | AC-15 | |
| `TestRFC9915Section2043HMACVector` | same | HMAC over a captured odhcpd datagram (A-3) | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Reconfigure Message msg-type | 5, 6, 11 | 5, 6, 11 | 4 | 12 |
| replay detection | strictly greater than recorded, modulo 2^64 | recorded + 1 | recorded | N/A |
| RKAP authentication information | Type 1 octet + Value 16 octets | 17 octets | 16 | 18 |
| Authentication option count | exactly 1 | 1 | 0 | 2 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `iface-dhcpv6-reconfigure-accept-config` | `test/plugin/iface-dhcpv6-reconfigure-accept-config.ci` | the leaf commits and the client announces Reconfigure Accept | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `dhcpv6-client-reconfigure-odhcpd` | `test/interop/scenarios/` | OpenWrt odhcpd (`test/interop/Dockerfile.odhcpd`, built from source) | ze leases with Reconfigure Accept, odhcpd records accept and sends its key; the checker adds an address on odhcpd's interface, odhcpd sends an authenticated Reconfigure, ze answers with RENEW within 2 s and keeps its lease. Red phase recorded with the Reconfigure branch reverted | |

## Files to Modify
- `internal/component/iface/yang/ze-iface-conf.yang` - `leaf reconfigure-accept`
- `internal/component/iface/config.go` - parse
- `internal/plugins/iface/dhcp/dhcp_linux.go` - `dHCPConfig.ReconfigureAccept`
- `internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go` - Reconfigure Accept in builders, packet connection wrapper, Authentication option parse in Reply handling
- `internal/plugins/iface/dhcp/dhcp_v6_linux.go` - persistent link lifetime while accepting, key and replay state per server
- `internal/plugins/iface/dhcp/dhcp_v6_attach_linux.go` - `waitLease` Reconfigure wake, link-local change
- `internal/plugins/iface/dhcp/rfc9915_dhcp_v6_messages_linux_test.go` - the Reconfigure discard cases now run with acceptance on and off
- `docs/features/interfaces.md`, `features/interfaces.md`, `rfc/short/rfc9915.md`, `docs/features/rfc-status.md`, `docs/architecture/testing/interop.md`

## Files to Create
- `internal/plugins/iface/dhcp/dhcp_v6_reconfigure_linux.go` - Reconfigure validation, RKAP, replay detection
- `internal/plugins/iface/dhcp/rfc9915_dhcp_v6_reconfigure_linux_test.go`
- `test/plugin/iface-dhcpv6-reconfigure-accept-config.ci`
- `test/interop/Dockerfile.odhcpd`, `test/interop/scenarios/dhcpv6-client-reconfigure-odhcpd/` and its checker beside `internal/le/interoplab/bgp/check_dhcpv6.go`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `reconfigure-accept` |
| YANG validation constraints | Yes | boolean type, default false |
| YANG custom validators | No | none needed |
| CLI commands/flags | No | none |
| CLI grammar (keyword before value) | N-A | none |
| Editor autocomplete | Yes | automatic for a boolean |
| Functional test for new RPC/API | Yes | `iface-dhcpv6-reconfigure-accept-config.ci` |
| Pipe completeness | N-A | no output |
| Env var registration | N-A | not under `environment/` |
| Doctor check for runtime dependencies | No | the same UDP 546 socket, held longer |
| Prometheus counters/metrics | No | none in this client |
| BGP family surface (new SAFI / capability / attribute) | N-A | not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | `features/interfaces.md` |
| 2 | Config syntax changed? | Yes | YANG description; `docs/features/interfaces.md` |
| 3 | CLI command added/changed? | No | none |
| 4 | API/RPC added/changed? | No | none |
| 5 | Plugin added/changed? | No | same plugin |
| 6 | Has a user guide page? | No | none |
| 7 | Wire format changed? | Yes | `rfc/short/rfc9915.md` Wire Format gains the Authentication option and RKAP layout |
| 8 | Plugin SDK/protocol changed? | No | none |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc9915.md`, `docs/features/rfc-status.md` |
| 10 | Test infrastructure changed? | Yes | `docs/architecture/testing/interop.md` (odhcpd image and scenario) |
| 11 | Affects daemon comparison? | No | none |
| 12 | Internal architecture changed? | Yes | `docs/features/interfaces.md` socket lifetime while accepting |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | No | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | none |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `docs/features/interfaces.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/features/interfaces.md` example |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- YANG leaf, parse, config field, `.ci`, `TestRFC9915Section2120ReconfigureAcceptInMessages`; they fail.
2. **Phase: Reconfigure Accept and key record** -- AC-1, AC-2, AC-3, AC-11.
3. **Phase: Listening link** -- packet connection wrapper, persistent lifetime, `waitLease` wake; AC-4, AC-10, AC-14.
4. **Phase: Validation** -- Section 16.11 and RKAP; AC-9, AC-15, HMAC vector.
5. **Phase: Response exchanges** -- AC-5 to AC-8, AC-12, AC-13.
6. **Phase: Interop** -- odhcpd image, scenario, red phase.
7. **Phase: Ledger and docs** -- tags, discrimination records, Meta, pages.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | every AC-N has an implementation at file:line |
| Correctness | the HMAC is computed over the raw received bytes; the comparison is constant time (`hmac.Equal`) |
| Correctness | replay value recorded only after the message passed every other check |
| Data flow | Reconfigure reaches the worker only through the bounded channel; validation runs on the worker goroutine |
| Rule: goroutine-lifecycle | no goroutine per datagram |
| Rule: rfc-compliance | every validation step quotes its Section 16.11, 20.3 or 20.4.3 sentence |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Reconfigure handled | `TestRFC9915Section18211ReconfigureTriggersRenew` passes |
| 11 rows proven both ways | `./le rfc check` and grep of the ids for `{gap` and `{single-polarity` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | every option length checked before reading; an Authentication option shorter than 11 octets, or than the RKAP body, is a discard |
| Authentication | no path starts an exchange before the HMAC and replay checks pass; a missing key fails closed |
| Timing | `hmac.Equal` for the digest comparison |
| Resource exhaustion | bounded channel, no allocation per discarded datagram beyond the read buffer |
| Error leakage | key and digest never logged |

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

## Design Insights

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| take Reconfigure from a packet connection wrapper under `nclient6.NewWithConn` | a second socket on UDP 546 with SO_REUSEPORT; replace the vendored client | a second socket would share unicast datagrams with the first, losing Replies; replacing the client rewrites every exchange for one message type |
| persistent link only while accepting | one persistent link for every worker | the default path stays byte-for-byte as tested; Section 15 obliges listening only when negotiated |
| RKAP inside this child | a separate authentication spec | RKAP has no use but Reconfigure (Section 20.4) |
| opt-in leaf, default false | accept whenever the server offers | the reconfigure key crosses the link in clear (umbrella Q1) |
| key kept in memory only | persist it | ze keeps no lease across restarts; a restart solicits and gets a new key |

## Known Limitations
- Only odhcpd's msg-type Renew is proven against another implementation (R-5).
- RFC 6977 relay-triggered Reconfigure is out of scope.

## RFC Documentation (Scope: protocol)

Add `// RFC 9915 Section 16.11: "..."`, `// RFC 9915 Section 20.3: "..."`, `// RFC 9915 Section 20.4.3: "..."`, `// RFC 9915 Section 18.2.11: "..."` above the implementing statements; the Authentication option and RKAP layout as an ASCII diagram with offsets above the parser.

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
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/spec-dhcpv6-client-4-reconfigure.md` only, in the same `./le commit create` script
