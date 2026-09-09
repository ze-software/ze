# Spec: pppoe-discovery-omits-the-mandatory-service-name-tag

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Depends | - |
| Phase | 5/5 |
| Handoff | - |
| Updated | 2026-09-09 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**With no `service-name` configured, Ze's access concentrator accepts a PADR that
carries no Service-Name tag, and answers both a PADI and a PADR with a PADO and a
PADS that carry none either. RFC 2516 requires exactly one in all four packets.**

`MatchServiceName` returns true before it looks at the packet when the allow-list
is empty, which is the default. The tag is then never required on the way in.
On the way out, `BuildPADO` emits a Service-Name tag only for each configured
name plus the one the PADI carried, and `BuildPADS` emits only the tag copied from
the PADR, through `AddTagCopy`, which writes nothing when handed a nil tag. With
no configured names and a PADI or PADR that carried no tag, both replies go out
with no Service-Name tag at all.

RFC 2516 Section 5.1: "The PADI packet MUST contain exactly one TAG of TAG_TYPE
Service-Name". Section 5.2: "The PADO packet MUST contain one AC-Name TAG
containing the Access Concentrator's name, a Service-Name TAG identical to the one
in the PADI". Section 5.3: "The PADR packet MUST contain exactly one TAG of
TAG_TYPE Service-Name". Section 5.4: "The PADS packet contains exactly one TAG of
TAG_TYPE Service-Name".

A zero-length Service-Name value is legitimate and means any service, so the fix
is not to demand a name.

**The two MUSTs are addressed to different parties, and that decides how far this
spec goes.** Sections 5.1 and 5.3 bind the HOST that sends the PADI and the PADR.
Sections 5.2 and 5.4 bind the ACCESS CONCENTRATOR that composes the PADO and the
PADS, which is Ze. Ze cannot become non-conformant because a peer sent a packet
wrongly, so the emission half is an obligation Ze owes and the receive half is a
choice about what to tolerate. Both reference implementations were read at the
source before that choice was made, and neither is strict on receive (see Other
Implementations below).

The decision, taken by the owner on 2026-09-08 with that evidence in front of him:
**tolerate a PADI that omits the tag, refuse a PADR that omits it, and always emit
exactly one tag in PADO and PADS.** That is accel-ppp's position exactly, and it
leaves Ze more conformant than both references on transmit while refusing nothing
either of them accepts on the PADI.

Ze's own PPPoE client is not affected and is the model: `BuildPADI` and
`BuildPADR` call `AddTagString` unconditionally, which writes a zero-length tag
for an empty service name. The defect is in the AC's PADR admission and in its two
reply builders only.

The goal: every PADO and PADS Ze emits carries exactly one Service-Name tag, a
PADR that omits the tag is refused the way RFC 2516 Section 5.4 says to refuse it,
and a PADI that omits it is served as a request for any service.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/l2tp/bng-5-pppoe.md` - the AC's discovery design, its Service-Name filtering and its PADS-after-kernel-setup rule
  → Decision: the per-interface `service-name` list overrides the global list, and an empty list at both levels accepts every Service-Name, so "accepts any name" must stay possible while "omits the tag" stops being possible
  → Constraint: a refusal is answered with a PADS carrying an error tag, never with silence, so a PADR missing the tag gets a Service-Name-Error reply rather than a drop
  → Constraint: the page is silent on the tag being mandatory in each of the four packets, so it gains that statement in this work
- [ ] `docs/architecture/l2tp/cpe-1-pppoe-client.md` - the client side, read to confirm the sibling path does not carry the same defect
  → Decision: the client writes the Service-Name tag unconditionally, empty value included, so no client change is in scope and the reviewer does not have to re-derive that

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc2516.md` - PPPoE discovery packet composition and the error tags
  → Constraint: RFC 2516 Section 5.1: "The PADI packet MUST contain exactly one TAG of TAG_TYPE Service-Name, indicating the service being requested, and any number of other TAG types."
  → Constraint: RFC 2516 Section 5.2: "The PADO packet MUST contain one AC-Name TAG containing the Access Concentrator's name, a Service-Name TAG identical to the one in the PADI, and any number of other Service-Name TAGs indicating other services that the Access Concentrator offers."
  → Constraint: RFC 2516 Section 5.3: "The PADR packet MUST contain exactly one TAG of TAG_TYPE Service-Name, indicating the service being requested, and any number of other TAG types."
  → Constraint: RFC 2516 Section 5.4: "The PADS packet contains exactly one TAG of TAG_TYPE Service-Name, indicating the service under which the Access Concentrator has accepted the PPPoE session, and any number of other TAG types."
  → Constraint: RFC 2516 Section 5.4: "If the Access Concentrator does not like the Service-Name in the PADR, then it MUST reply with a PADS containing a TAG of TAG_TYPE Service-Name-Error (and any number of other TAG types). In this case the SESSION_ID MUST be set to 0x0000."
  → Constraint: RFC 2516 Section 5.2: "If the Access Concentrator can not serve the PADI it MUST NOT respond with a PADO". That is the AC's only refusal obligation on the PADI, and it is conditioned on being unable to SERVE the request, not on the packet's composition. A PADI Ze can serve is therefore answered, tag or no tag
  → Constraint: the Section 5.1 and 5.3 MUSTs bind the sender of those packets. Neither obliges the AC to refuse a peer that breaks them, which is why the receive-side strictness is a choice and the emission-side rules are not

### Other Implementations (read at the source 2026-09-08, quoted)

Both were read because the receive-side question is a choice rather than a
mechanism, and because a strictness no shipping AC has would refuse packets real
clients send. The table is the justification for the owner's decision.

| Behavior | accel-ppp `accel-pppd/ctrl/pppoe/pppoe.c` | FreeBSD `sys/netgraph/ng_pppoe.c` | Ze after this spec |
|---|---|---|---|
| PADI with no Service-Name tag | Tolerated when nothing is configured: `int len, n, service_match = conf_service_name[0] == NULL;` in `pppoe_recv_PADI`, so the match starts true and a tagless PADI passes. Refused when a name is configured | Tolerated always: `pppoe_recv` substitutes an empty tag, `tag = get_tag(ph, PTT_SRV_NAME); if (tag == NULL) tag = &sntag;` with `const struct pppoe_tag sntag = { PTT_SRV_NAME, 0 }`, then matches it against the listener, which accepts everything when empty or `*` | Tolerated when nothing is configured, refused when a name is configured. Identical to accel-ppp, and Ze's `MatchServiceName` already does this, so no code change |
| PADR with no Service-Name tag | Refused: `if (!service_name_tag) { ... "discard PADR packet (no Service-Name tag present)" ... return; }` in `pppoe_recv_PADR` | Tolerated: the `PADR_CODE` arm keys only on the cookie, `utag = get_tag(ph, PTT_AC_COOKIE); if ((utag == NULL) \|\| (ntohs(utag->tag_len) != sizeof(sp))) { LEAVE(ENETUNREACH); }`, and allocates the session without ever reading the Service-Name | Refused, with a PADS carrying Service-Name-Error and session ID `0x0000`. This is the one place Ze is stricter than FreeBSD, and it is accel-ppp's shipped behavior |
| Two Service-Name tags | Never refused; both match loops run and the LAST tag is echoed | Never refused; `get_tag` returns at the first match, `if (pt->tag_type == idx) { ... return (pt); }`, so the FIRST wins | Never refused, first wins. Ze's `FindTag` already returns the first, so no code change and one test pins it |
| PADO composition | Emits none when nothing is configured and the PADI carried none: `if (conf_service_name[0]) { ... add_tag(..., TAG_SERVICE_NAME, ...) }` then `if (service_name) add_tag2(...)` | Emits zero or two: `if ((tag = get_tag(ph, PTT_SRV_NAME))) insert_tag(sp, tag); if (((tag == NULL) \|\| (tag->tag_len == 0)) && (neg->service.hdr.tag_len != 0)) { insert_tag(sp, &neg->service.hdr); }` | Always exactly one. More conformant than both |
| PADS composition | Always exactly one, with an empty fallback: `add_tag2(pack, sizeof(pack), conn->service_name);`, where `allocate_channel` substitutes `struct pppoe_tag empty_service_name = { .tag_type = htons(TAG_SERVICE_NAME) };` | Emits none when the PADR carried none: only `if ((tag = get_tag(ph, PTT_SRV_NAME))) insert_tag(sp, tag);` | Always exactly one. Matches accel-ppp, more conformant than FreeBSD |
| Zero-length value means any service | Yes | Yes on the listener side; an empty listener or `*` accepts every name. `ng_pppoe.h`: "If no service is given this is assumed to accept ALL PADI requests." | Yes, unchanged: `MatchServiceName` returns true for an empty allow-list and for a zero-length value |

Two things follow, and both are recorded so a later reader does not reopen them.
`ng_pppoe` is a complete access concentrator rather than a name-answering stub:
it runs the full PADI, PADO, PADR, PADS and PADT exchange and allocates session
identifiers, so its leniency is a considered position and not an omission.
And accel-ppp's empty-PADS fallback is unreachable in its own code, because a
PADR lacking the tag is dropped before it; Ze reaches the same wire result with
one fewer path.

**Key insights:**
- The receive-side MUSTs bind the peer, not Ze. Refusing a peer's malformed packet is a robustness choice, and no implementation in the field makes the strict one on PADI.
- The two failures have one cause with two faces: a filter that short-circuits before reading the packet, and a builder that treats an absent tag as nothing to write.
- An empty Service-Name value is a valid value meaning any service. Requiring the tag does not require a name, so the "accepts every service" default is preserved exactly.
- PADI and PADR are refused differently. The RFC says the AC that cannot serve a PADI sends no PADO at all, while a PADR it will not serve takes a PADS error with session ID zero.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/l2tp/pppoe/discovery.go` - `MatchServiceName` returns true immediately when the allow-list is empty and returns true for a zero-length tag value otherwise; `BuildPADO` writes one tag per configured name plus the PADI's tag when it is non-empty and not already listed; `BuildPADS` writes only `AddTagCopy(padr.FindTag(TagServiceName))`; `AddTagCopy` returns true without writing when handed nil; `addTag` writes a zero-length value correctly; `BuildPADI` and `BuildPADR` (the client builders) write the tag unconditionally through `AddTagString`; `BuildPADSError` writes the error tag, the AC-Name, Host-Uniq and Relay-Session-Id, and no Service-Name
- [ ] `internal/component/l2tp/pppoe/server.go` - `handlePADI` drops the packet silently on a Service-Name mismatch; `handlePADR` answers a mismatch with `BuildPADSError(..., TagSvcNameError)`; `Session.ServiceName` is filled from `ServiceNameString`, which yields an empty string for an absent tag exactly as it does for an empty tag
- [ ] `internal/component/l2tp/pppoe/config.go` - `ExtractParameters` resolves the global and per-interface `service-name` lists, both empty by default
- [ ] `internal/component/l2tp/pppoe/yang/ze-pppoe-conf.yang` - the global `service-name` leaf-list and the per-interface override, whose help text states an empty list accepts any Service-Name
- [ ] `internal/component/l2tp/pppoeclient/dialer.go` - the client calls `BuildPADI` and `BuildPADR` with the configured service name, empty string included
- [ ] `rfc/full/rfc2516.txt` - Sections 5.1 through 5.4, read in full for the four composition rules and the two refusal forms

**Behavior to preserve:**
- An empty `service-name` list still accepts every Service-Name value.
- A per-interface list still overrides the global list.
- A PADI naming a service the AC does not offer still gets no PADO.
- A PADR naming a service the AC does not offer still gets a PADS carrying a Service-Name-Error tag with session ID `0x0000`.
- The client builders are unchanged: they already write the tag unconditionally.
- `Session.ServiceName` keeps holding the empty string for the any-service case.

**Behavior to change:**
- A PADR carrying no Service-Name tag gets a PADS carrying a Service-Name-Error tag and session ID `0x0000`, under any configuration.
- Nothing else on the receive side changes. A PADI with no tag is still served when nothing is configured and still refused when a name is, and a duplicate tag is still resolved first-wins by `FindTag`. Both are now pinned by tests rather than left as accidents of the code.
- `BuildPADO` always emits a Service-Name tag identical to the PADI's, including the zero-length case.
- `BuildPADS` always emits exactly one Service-Name tag, including the zero-length case.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- An Ethernet frame of EtherType `0x8863` on a configured access interface, read by `discoveryReader` from the subsystem's `AF_PACKET` socket.
- Format at entry: a PPPoE discovery frame, code `0x09` (PADI) or `0x19` (PADR), whose payload is a list of TYPE(2) LENGTH(2) VALUE tags.

### Transformation Path
1. `ParseDiscovery` (`discovery.go`) validates the header and walks the tags into the packet's tag slice.
2. `HandleDiscovery` (`server.go`) dispatches on the code.
3. `MatchServiceName` (`discovery.go`) decides whether the AC serves this request.
4. `BuildPADO` or `BuildPADS` (`discovery.go`) composes the reply into a caller-owned frame buffer.
5. `sendFrame` (`server.go`) writes the frame back to the access interface.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Wire → AC | `AF_PACKET` read parsed by `ParseDiscovery` | No |
| Config → AC | The `service-name` leaf-lists resolved by `ExtractParameters` into the per-interface server | No |
| AC → wire | The composed PADO or PADS frame written back to the interface | No |
| AC → operator | The session's recorded service name in the PPPoE show output | No |

### Integration Points
- `MatchServiceName` (`discovery.go`) - the single admission decision for both PADI and PADR, so the tag-presence rule lands here once.
- `BuildPADO` and `BuildPADS` (`discovery.go`) - the two reply builders that must always emit the tag.
- `handlePADR` (`server.go`) - the one admission point this spec changes; `handlePADI` is read and pinned by tests but not changed.
- `internal/component/l2tp/pppoeclient/` - read only, to confirm the sibling path is already conformant.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | No client Ze must interoperate with omits the Service-Name tag from a PADI or PADR | RFC 2516 Sections 5.1 and 5.3 make it mandatory | Refusing the packet breaks a real client, and the answer becomes leniency on receive with strictness on transmit | The interop scenarios against accel-ppp and pppd 2.5.1, plus a capture of each client's PADI | unvalidated |
| A-2 | A zero-length Service-Name tag in a PADO or PADS is accepted by the clients Ze faces | RFC 2516 permits the value; `MatchServiceName` already treats it as any-service on receive | Emitting the empty tag breaks a client that expects a named service | The interop scenarios, with no `service-name` configured | unvalidated |
| A-3 | `AddTagCopy(nil)` returning true without writing is used deliberately elsewhere and must not change meaning | `BuildPADO`, `BuildPADS` and `BuildPADSError` all rely on it for the optional Host-Uniq and Relay-Session-Id tags | Changing the helper breaks the genuinely optional tags | `gopls references` on `AddTagCopy`, each call site read and classified as mandatory or optional | unvalidated |
| A-4 | Ze's PPPoE client needs no change | `BuildPADI` and `BuildPADR` call `AddTagString` unconditionally, and `addTag` writes a zero-length value | The client carries the same defect and the spec's scope is too narrow | Read both builders and assert the emitted frame carries the tag with an empty configured name | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The PADR refusal stops serving a client that used to connect against Ze, since FreeBSD's `ng_pppoe` allocates a session for a tagless PADR and a client written against it may omit the tag | An interop scenario fails at PADR, or subscribers stop appearing after this lands | The refusal is logged and counted (AC-10) so it is visible rather than silent, and the interop scenarios run against both accel-ppp and a pppd client before it ships. accel-ppp has shipped this refusal for years, which bounds how many real clients can depend on the leniency |
| R-2 | Tolerating a duplicate tag first-wins serves a peer differently from accel-ppp, which uses the last | A client sending two different service names is matched on the wrong one | Recorded as a Known Limitation rather than fixed. Refusing would be stricter than both references, and picking the last would differ from FreeBSD; first-wins is what Ze's `FindTag` already does and what the wire convention is |
| R-3 | The always-emit change makes the reply exceed the frame buffer in a case that previously fit | `Builder.Finish` returns nil and the reply is dropped | The added tag is 4 octets plus the value, and the builder's truncation path already refuses rather than overflowing; a boundary test drives the maximum tag set |
| R-4 | The fix is applied at the builder and not at the admission point, so Ze emits a conformant reply to a non-conformant request | A test asserts the reply but nothing asserts the refusal | Both sides carry their own AC and their own test, and the Wiring Test table names one for each |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A subscriber that used to connect is refused, or a reply a client used to accept is rejected by it |
| How is it reverted? | Single commit revert. No config migration: no leaf changes and the default behavior for a conformant client is identical |
| Who else touches this path? | `plan/immediate/spec-pppoe-padr-replay-allocates-unbounded-sessions.md` changes the same PADR handler, so the two land in an agreed order |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| A PADI with no Service-Name tag and no configured names | → | `MatchServiceName` → `handlePADI` → `BuildPADO` (`server.go`, `discovery.go`) | `TestPADIWithoutServiceNameIsServedAsAnyService` |
| A PADR with no Service-Name tag on the AC's socket | → | `requireServiceNameTag` → `handlePADR` error reply (`server.go`) | `TestPADRWithoutServiceNameGetsServiceNameError` |
| A PADI with an empty Service-Name value and no configured names | → | `BuildPADO` (`discovery.go`) | `TestPADOAlwaysCarriesServiceName` |
| A PADR accepted with an empty Service-Name value | → | `BuildPADS` (`discovery.go`) | `TestPADSAlwaysCarriesServiceName` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A PADI carrying no Service-Name tag, with no `service-name` configured | The PADI is served as a request for any service: a PADO is sent, carrying exactly one Service-Name tag whose value is zero-length. This is accel-ppp's and FreeBSD's behavior, and it is what Ze already does apart from the missing tag on the reply |
| AC-2 | A PADI carrying no Service-Name tag, with one or more `service-name` values configured | No PADO is sent, unchanged from today. `MatchServiceName` already refuses a nil tag against a non-empty allow-list, which is accel-ppp's condition exactly; this row pins that behavior against regression |
| AC-3 | A PADR carrying no Service-Name tag, under any configuration | A PADS is sent carrying a Service-Name-Error tag with session ID `0x0000`, no session is allocated, and no socket, channel or unit is opened |
| AC-4 | A discovery packet carrying two Service-Name tags | The packet is not refused, and the FIRST tag is the one matched and echoed. Ze's `FindTag` already returns the first, matching FreeBSD's `get_tag`; this row pins it |
| AC-5 | A PADI carrying a zero-length Service-Name tag, with no `service-name` configured | A PADO is sent carrying exactly one Service-Name tag whose value is zero-length, plus the AC-Name and the AC-Cookie |
| AC-6 | A PADR carrying a zero-length Service-Name tag, accepted | A PADS is sent carrying exactly one Service-Name tag whose value is zero-length, and the allocated session ID |
| AC-7 | A PADI naming a service the AC offers, with names configured | A PADO carrying the offered names and a Service-Name tag identical to the PADI's, unchanged from today |
| AC-8 | A PADR naming a service the AC does not offer | The Service-Name-Error reply, unchanged from today |
| AC-9 | Ze's own PPPoE client with an empty configured service name | Its PADI and PADR each carry exactly one Service-Name tag with a zero-length value |
| AC-10 | A PADR refused for a missing Service-Name tag | The refusal is logged with the RFC condition and counted, so an operator sees it without a packet capture |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Connects a CPE with no service name configured on either end | PADI → PADO → PADR → PADS, each with one empty Service-Name tag | `01-pppoe-chap-ipv4` interop scenario |
| 2 | Configures a service name and sees only that service served | PADI naming it → PADO → PADR → PADS; a PADI naming another gets nothing | `test/plugin/pppoe-service-name.ci` |
| 3 | Sees why a subscriber was refused | The refusal counter and the debug log line | `TestPADIWithoutServiceNameGetsNoPADO` and the counter assertion |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPADIWithoutServiceNameIsServedAsAnyService` | `internal/component/l2tp/pppoe/server_test.go` | AC-1: a PADO IS sent, and it carries exactly one zero-length Service-Name tag | |
| `TestPADIWithoutServiceNameRefusedWhenNamesConfigured` | `internal/component/l2tp/pppoe/server_test.go` | AC-2: pins the existing `MatchServiceName` behavior against regression | |
| `TestPADRWithoutServiceNameGetsServiceNameError` | `internal/component/l2tp/pppoe/server_test.go` | AC-3 | |
| `TestDuplicateServiceNameTagTakesTheFirst` | `internal/component/l2tp/pppoe/discovery_test.go` | AC-4: neither refused nor last-wins; pins `FindTag` | |
| `TestPADOAlwaysCarriesServiceName` | `internal/component/l2tp/pppoe/discovery_test.go` | AC-5 and AC-7 | |
| `TestPADSAlwaysCarriesServiceName` | `internal/component/l2tp/pppoe/discovery_test.go` | AC-6 | |
| `TestPADSErrorCarriesSessionIDZero` | `internal/component/l2tp/pppoe/discovery_test.go` | AC-3 and AC-8, the session ID rule | |
| `TestClientPADICarriesEmptyServiceName` | `internal/component/l2tp/pppoe/discovery_test.go` | AC-9, and records that the client path needed no change | |
| `TestPADRRefusalIsCounted` | `internal/component/l2tp/pppoe/server_test.go` | AC-10 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Service-Name tags required in a PADR | exactly 1 required | 1 | 0 refused | 2 accepted, first wins |
| Service-Name tags required in a PADI | 0 or more | 0 accepted as any-service | N/A | 2 accepted, first wins |
| Service-Name value length | 0 to the frame's remaining capacity | the largest value that leaves room for AC-Name and AC-Cookie | N/A | one octet more, which makes `Finish` return nil |
| PADO tag count with configured names | 1 to `maxTags` (32) | 32 | N/A | 33 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `pppoe-service-name` | `test/plugin/pppoe-service-name.ci` | An operator configures a service name, connects a matching client, and sees a non-matching one refused | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `pppoe-chap-ipv4` | `test/interop-pppoe/scenarios/01-pppoe-chap-ipv4` | accel-ppp | The always-emit change did not break an ordinary session against a real client |  |
| `pppoe-empty-service-name` | `test/interop-pppoe/scenarios/` | accel-ppp client, and pppd with rp-pppoe | A client that requests any service completes discovery and accepts the empty Service-Name tag in PADO and PADS | |

## Files to Modify
- `internal/component/l2tp/pppoe/discovery.go` - `BuildPADO` and `BuildPADS` always emit exactly one Service-Name tag. `MatchServiceName` and `FindTag` are NOT changed: the narrowed shape leaves both correct, and they gain pinning tests instead
- `internal/component/l2tp/pppoe/server.go` - `requireServiceNameTag` and its PADR branch, the refusal log line and the counter
- `docs/architecture/l2tp/bng-5-pppoe.md` - the design document `discovery.go`, `server.go` and `config.go` declare: the tag is mandatory in all four packets, and the two refusal forms differ
- `docs/architecture/l2tp/cpe-1-pppoe-client.md` - the design document the client files declare, stating that the client already emits the tag unconditionally
- `docs/guide/pppoe.md` - what an operator sees when a client omits the tag
- `rfc/short/rfc2516.md` - the Section 5.1 through 5.4 requirement rows this work implements and proves

## Files to Create
- `test/plugin/pppoe-service-name.ci` - the operator path for service-name filtering and its refusal

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | The RFC decides the behavior; the existing `service-name` leaf-lists are unchanged |
| YANG validation constraints | N-A | No new leaf |
| YANG custom validators | N-A | No new leaf |
| CLI commands/flags | N-A | No new verb |
| CLI grammar (keyword before value) | N-A | No new command |
| Editor autocomplete | N-A | No new leaf |
| Functional test for new RPC/API | Yes | `test/plugin/pppoe-service-name.ci` |
| Pipe completeness | N-A | No new command output |
| Env var registration | N-A | No env var |
| Doctor check for runtime dependencies | N-A | No new file path, socket, port, module, binary or sysctl |
| Prometheus counters/metrics | Yes | A discovery-refusal counter labelled by reason, so a silent PADI drop is visible to an operator |
| BGP family surface (new SAFI / capability / attribute) | N-A | Not BGP |

### Documentation Update Checklist
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | Conformance, not a feature an operator selects |
| 2 | Config syntax changed? | No | The `service-name` leaves are unchanged |
| 3 | CLI command added/changed? | No | No verb changes |
| 4 | API/RPC added/changed? | No | No RPC change |
| 5 | Plugin added/changed? | No | The AC is a component subsystem |
| 6 | Has a user guide page? | Yes | `docs/guide/pppoe.md` |
| 7 | Wire format changed? | Yes | The discovery packet composition is described where the PPPoE wire format is documented |
| 8 | Plugin SDK/protocol changed? | No | No SDK surface |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc2516.md` Sections 5.1 to 5.4, and the generated `docs/features/rfc-status.md` row, with source anchors |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` for the new interop scenario |
| 11 | Affects daemon comparison? | No | No comparison row changes |
| 12 | Internal architecture changed? | Yes | `docs/architecture/l2tp/bng-5-pppoe.md` and `docs/architecture/l2tp/cpe-1-pppoe-client.md` |
| 13 | Route metadata keys added/changed? | No | No route metadata |
| 14 | Prometheus counters added/changed? | Yes | The discovery-refusal counter, in the PPPoE telemetry section |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | Nothing registers |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED: run `./le spec citation anchors spec plan/immediate/spec-pppoe-discovery-omits-the-mandatory-service-name-tag.md`. `discovery.go`, `server.go` and `config.go` declare `docs/architecture/l2tp/bng-5-pppoe.md`, and `pppoeclient/dialer.go` declares `docs/architecture/l2tp/cpe-1-pppoe-client.md`; this spec names and edits both |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | Verify the discovery examples in `docs/guide/pppoe.md` against the builders after the change |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the admission rule exists and both refusal paths reach it
   - Tests: `TestMatchServiceNameRequiresExactlyOneTag`, `TestPADIWithoutServiceNameGetsNoPADO`
   - Files: `discovery.go`, `server.go`
   - Verify: the rule is called from both handlers, and the wiring test fails because the rule still short-circuits on an empty allow-list
2. **Phase: admission** -- the PADR requires the tag, the PADI does not, and duplicates are tolerated first-wins
   - Tests: `TestPADRWithoutServiceNameGetsServiceNameError`, `TestPADIWithoutServiceNameIsServedAsAnyService`, `TestPADIWithoutServiceNameRefusedWhenNamesConfigured`, `TestDuplicateServiceNameTagTakesTheFirst`, `TestPADSErrorCarriesSessionIDZero`
   - Files: `server.go`, `discovery.go`
   - Verify: the PADR branch carries the RFC 2516 Section 5.4 sentence and refuses with session ID `0x0000`; `MatchServiceName` is unchanged and its two pinning tests pass against it; the phase 1 tests written for the stricter shape are RETARGETED onto these assertions rather than deleted
3. **Phase: emission** -- PADO and PADS always carry exactly one Service-Name tag
   - Tests: `TestPADOAlwaysCarriesServiceName`, `TestPADSAlwaysCarriesServiceName`, `TestClientPADICarriesEmptyServiceName`
   - Files: `discovery.go`
   - Verify: the zero-length case emits a tag, and the configured-names case is unchanged
4. **Phase: visibility** -- the refusal counter and the debug log line
   - Tests: the counter assertion in `server_test.go`
   - Files: `server.go`, the PPPoE metrics registration
   - Verify: a silent wire refusal is still visible to an operator
5. **Phase: proof against a real peer** -- the interop scenarios and the operator path
   - Tests: `pppoe-empty-service-name`, `01-pppoe-chap-ipv4`, `pppoe-service-name.ci`
   - Files: `test/interop-pppoe/scenarios/`, `test/plugin/pppoe-service-name.ci`
   - Verify: revert each enforcing branch in turn, record the red, restore it, confirm green. Where a unit carries an `RFC requirement:` tag, the walk runs through `./le rfc discriminate-record`

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Sections 5.2 and 5.4, the two that bind Ze, each have a branch, a quoted requirement and a test. Sections 5.1 and 5.3 bind the peer and are pinned by tests recording what Ze tolerates |
| Feature completeness | Emission is fixed unconditionally and the PADR admission is fixed; the PADI and duplicate cases are pinned so a later change cannot silently tighten them |
| Correctness | The PADR refusal is a PADS carrying Service-Name-Error with session ID `0x0000`, and the PADI path still answers a servable request |
| Reference agreement | Every receive-side behavior matches the Other Implementations table, and any divergence found during implementation is raised rather than absorbed |
| Naming | The counter's reason label names the RFC condition, not the function that detected it |
| Data flow | The tag-presence rule lives in `MatchServiceName` alone; neither handler re-implements it |
| Rule: `ai/rules/rfc-compliance.md` | Every claim in `rfc/short/rfc2516.md` states what the test body checks and no more |
| Rule: `ai/rules/principles.md` | An absent tag and an empty tag stop being the same value: `ServiceNameString` returning the empty string for both is exactly the confusion being removed |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| The admission rule requires exactly one tag | `TestMatchServiceNameRequiresExactlyOneTag` passes |
| Both builders always emit the tag | `TestPADOAlwaysCarriesServiceName` and `TestPADSAlwaysCarriesServiceName` pass |
| The RFC sentences are in the code | `grep -n "RFC 2516 Section 5" internal/component/l2tp/pppoe/discovery.go internal/component/l2tp/pppoe/server.go` |
| The interop scenario discriminates | The recorded red from each reverted branch |
| The design page states the rule | `grep -n "Service-Name" docs/architecture/l2tp/bng-5-pppoe.md` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | The tag walk is already bounds-checked; the new work is a count and a presence rule over the parsed tags |
| Fail-closed guard | The admission rule is a guard: a packet it cannot classify is refused, never served |
| Amplification | The PADS error reply is not larger than the PADR that provoked it |
| Resource exhaustion | The refusal path allocates nothing beyond the response frame, and it sits behind the existing PADI limiter |
| Error leakage | The refusal names the RFC condition, not peer-supplied bytes verbatim |

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

- A helper that treats nil as "nothing to do" is correct for an optional tag and wrong for a mandatory one. The same call spelling carried both meanings, which is why the omission was invisible at the call site.
- The default configuration is the vulnerable one here. An operator who sets a service name gets conformant behavior by accident, and the operator who sets nothing gets the defect.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Require the tag, not a name | Require a non-empty Service-Name value | A zero-length value is legitimate and means any service. Requiring a name would break the documented "empty list accepts any Service-Name" behavior |
| Tolerate a tagless PADI, refuse a tagless PADR | Strict on both, which is what this spec proposed before the two references were read | No shipping AC is strict on the PADI: accel-ppp starts its match true when nothing is configured, and FreeBSD substitutes an empty tag outright. The Section 5.1 MUST binds the host, so refusing buys no conformance and costs interop with clients both references serve. The PADR refusal is accel-ppp's shipped behavior and Section 5.4 gives it a defined reply form (owner decision, 2026-09-08) |
| Tolerate a duplicate tag, first wins | Refuse a packet carrying two | Neither reference refuses one, and FreeBSD's `get_tag` already fixes first-wins as the field convention. Refusing would be a strictness with no precedent and no obligation behind it |
| Fix emission unconditionally | Fix only what a strict receive side would have made unreachable | The emission MUSTs in Sections 5.2 and 5.4 bind Ze whatever it tolerates on receive, and both references are non-conformant here. This is where Ze gains conformance neither has |
| Leave `AddTagCopy` alone | Make `AddTagCopy` write an empty tag for nil | It is used for genuinely optional tags in three builders; changing it would emit Host-Uniq and Relay-Session-Id tags that were never sent |

## Known Limitations
- A PADI that omits the tag is served rather than refused, so Ze does not enforce the Section 5.1 MUST against a non-conformant host. That is deliberate: the MUST binds the host, and neither accel-ppp nor FreeBSD enforces it either.
- A packet carrying two Service-Name tags is served on the first tag rather than refused. accel-ppp uses the last in the same situation, so a peer sending two different names could be served differently by the two access concentrators. Refusing it would be stricter than both references, and this spec does not.
- A PADI refused because it names an unoffered service stays silent on the wire, because RFC 2516 Section 5.2 requires that. The operator's only visibility is the counter and the debug log.
- Ze's PPPoE client is read but not changed. If a later capture shows a real AC refusing Ze's empty Service-Name tag, that is its own defect and its own spec.

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: validation rules, error conditions, state transitions, timer
constraints, message ordering, and every MUST/MUST NOT. Each admission branch
and each builder carries the sentence from the RFC 2516 section that governs it,
and the wire format of the discovery tag list is documented with field offsets
where the composition rules are enforced.

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
- [ ] AC-1..AC-10 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)

---

## Implementation Summary

### What Was Implemented
- `BuildPADO` writes the Service-Name tag identical to the PADI's FIRST, through `AddTagString`, then the offered names, skipping one equal to the PADI's value. `BuildPADS` writes `padr.ServiceNameString()` through `AddTagString` in place of `AddTagCopy`. Both now emit exactly one tag in the zero-length case, which is where the MUST was violated (`internal/component/l2tp/pppoe/discovery.go`).
- `requireServiceNameTag` and its `handlePADR` branch, which refuses a tagless PADR with a PADS carrying `TagSvcNameError` and SESSION_ID `0x0000`, before any session is allocated and after the AC-Cookie check (`internal/component/l2tp/pppoe/server.go`).
- `ze_pppoe_discovery_refusals_total{reason}` on a closed five-value reason set, wired to the seven admission refusals in `handlePADI` and `handlePADR`, with the same reason string on each paired log line (`internal/component/l2tp/pppoe/metrics.go`, `server.go`).
- `MatchServiceName` and `FindTag` are UNCHANGED. The PADI tolerance and the first-wins duplicate resolution are pinned by tests instead.
- `rfc/short/rfc2516.md`: `RFC2516-5.2-2` closed, `RFC2516-5.4-1` closed, `RFC2516-5.4-2` allocated (below), notes on `5.1-1` and `5.3-2` corrected.
- Interop scenario `pppoe-empty-service-name` and its checker, which reads a real tcpdump capture rather than trusting the session coming up (`internal/le/interoplab/pppoe/check_service_name.go`). Operator path `test/pppoe/pppoe-service-name.ci` with its fixture personality and its `netnsSelections` entry.

### Bugs Found/Fixed
- **The refusal counter never reached an operator on a PPPoE-only daemon.** `Subsystem.Start` read `registry.GetMetricsRegistry()`, which is nil at that moment: `runYANGConfig` (`cmd/ze/hub/main.go`) runs `engine.Start`, which runs `Subsystem.Start`, and only later in the same function runs `startStandaloneTelemetry`, which creates the registry. `ze_pppoe_discovery_refusals_total` was therefore absent for the process lifetime, with no log line, in exactly the configuration `test/pppoe/pppoe-service-name.ci` drives. AC-10 was unreachable at the real entry point. Fixed by `registerDiscoveryMetrics`, which registers the hook through `registry.InjectPluginMetrics` so whichever of the two events happens second does the binding. Covered by `TestDiscoveryCounterBindsWhenTheRegistryArrivesLast`, observed RED against the old shape.
- **RFC 2516 Section 5.4's second sentence carried no requirement id**, so no gate asked for a test of the MUST that this spec's own new branch enforces. Fixed here: `RFC2516-5.4-2`, with both polarities tagged and both discrimination records observed.

### Documentation Updates
- `docs/architecture/l2tp/bng-5-pppoe.md`: four Decisions entries (the receive-side asymmetry, the always-emit rule, the refusal counter, the registration order), anchors `internal/component/l2tp/pppoe/server.go -- requireServiceNameTag, handlePADR`, `discovery.go -- BuildPADO, BuildPADS`, `metrics.go -- registerDiscoveryMetrics, bindPPPoEMetrics, countRefusal`.
- `docs/guide/pppoe.md`: a `## Metrics` section with the reason table, anchor `internal/component/l2tp/pppoe/metrics.go`.
- `docs/labs/pppoe-interop.md`: the new scenario in Layout, Running, its own section, and the `.ci` cross-reference.
- `rfc/short/rfc2516.md` and its regenerated `rfc/requirements/rfc2516.md`.
- `./le docs-to-code index-check`: 4 anchor failures, all pre-existing, none in a file this spec touched (`commands.md`, `exabgp-bridge.md`, `firewall-irr.md` twice).

### Deviations from Plan
- The spec's Files to Create named `test/plugin/pppoe-service-name.ci`. It landed at `test/pppoe/pppoe-service-name.ci`: `test/plugin` maps to the plugin suite, which `vmSuites` (`internal/le/qemu/alltests.go`) does not route into per-test netns mode, so an `option=netns-link` test there SKIPS (`applyNetnsLinkGate`, `internal/test/runner/caps.go`). `test/pppoe/` is reached by `./le qemu pppoe-test`.
- The spec proposed strictness on both PADI and PADR. Narrowed to PADR only by the owner's decision of 2026-09-08, with accel-ppp's and FreeBSD's sources read. `MatchServiceName` needed no change as a result.
- `TestPADSErrorCarriesSessionIDZero` was not added: `TestBuildPADSError` asserts the SID at the builder and `TestPADRWithoutServiceNameGetsServiceNameError` at the handler, so the assertion has two owners already.
- `TestPADRRefusalIsLoggedAndCounted` was renamed `TestPADRRefusalIsCounted` at closure: the body asserts the counter and never the log line, so the old name claimed more than it checked.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | The counter was bound by reading the metrics registry at `Subsystem.Start`, following the sibling `Start` in `internal/component/l2tp/subsystem.go` | On a PPPoE-only daemon no registry exists at that moment, so the counter was never created and AC-10 could not be reached through `/metrics` | Closure review traced `countRefusal` back to its producer and read the startup order in `runYANGConfig` | `registerDiscoveryMetrics` uses `registry.InjectPluginMetrics`, the deferral the registry already carries for the same failure; `TestDiscoveryCounterBindsWhenTheRegistryArrivesLast` pins it |
| escalation | A unit test handed the metric set in directly, so it passed against the broken wiring: `TestPADRRefusalIsCounted` was GREEN with the defect present | Only a test that drives the REGISTRATION, in the arrival order the deployment produces, can see it | The forced red: the old shape reddened only the new test | Recorded in Core Insight below and carried into the learned summary |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Every PADO carries exactly one Service-Name tag identical to the PADI's | Done | `discovery.go::BuildPADO` | `TestPADOAlwaysCarriesServiceName`, both cases |
| Every PADS carries exactly one Service-Name tag | Done | `discovery.go::BuildPADS` | `TestPADSAlwaysCarriesServiceName` |
| A PADR omitting the tag is refused the way Section 5.4 says | Done | `server.go::requireServiceNameTag`, `handlePADR` | `TestPADRWithoutServiceNameGetsServiceNameError` |
| A PADI omitting the tag is served as a request for any service | Done | `server.go::handlePADI` via unchanged `MatchServiceName` | `TestPADIWithoutServiceNameIsServedAsAnyService` |
| The refusal is visible to an operator | Done | `metrics.go::registerDiscoveryMetrics`, `countRefusal` | `TestPADRRefusalIsCounted` and `TestDiscoveryCounterBindsWhenTheRegistryArrivesLast` |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestPADIWithoutServiceNameIsServedAsAnyService` (PADO sent) plus `TestPADOAlwaysCarriesServiceName` case 1 (one zero-length tag) | The two halves are asserted by two tests |
| AC-2 | Done | `TestPADIWithoutServiceNameRefusedWhenNamesConfigured` | Pins unchanged `MatchServiceName` |
| AC-3 | Done | `TestPADRWithoutServiceNameGetsServiceNameError` | PADS, error tag, SID 0, `sessions.Count()` zero |
| AC-4 | Done | `TestDuplicateServiceNameTagTakesTheFirst` | Asserts the second name does NOT match |
| AC-5 | Done | `TestPADOAlwaysCarriesServiceName` case 1 | Exact tag sequence, one zero-length value |
| AC-6 | Done | `TestPADSAlwaysCarriesServiceName` | Exactly one tag from a tagless PADR |
| AC-7 | Done | `TestPADOAlwaysCarriesServiceName` case 2 | Exact sequence `internet`, `voip`, no duplicate |
| AC-8 | Done | `TestBuildPADSError` plus the `MatchServiceName` branch in `handlePADR` | Unchanged path, now also counted |
| AC-9 | Done | `TestClientPADICarriesEmptyServiceName` | No client change was needed |
| AC-10 | Done | `TestPADRRefusalIsCounted`, `TestDiscoveryCounterBindsWhenTheRegistryArrivesLast` | The second is what proves the counter reaches `/metrics` on a real daemon |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestPADIWithoutServiceNameIsServedAsAnyService` | Done | `server_test.go` | |
| `TestPADIWithoutServiceNameRefusedWhenNamesConfigured` | Done | `server_test.go` | |
| `TestPADRWithoutServiceNameGetsServiceNameError` | Done | `server_test.go` | Carries `RFC2516-5.4-2 positive` |
| `TestDuplicateServiceNameTagTakesTheFirst` | Done | `discovery_test.go` | |
| `TestPADOAlwaysCarriesServiceName` | Done | `discovery_test.go` | Carries `RFC2516-5.2-2` both polarities |
| `TestPADSAlwaysCarriesServiceName` | Done | `discovery_test.go` | Carries `RFC2516-5.4-1 negative` |
| `TestPADSErrorCarriesSessionIDZero` | Changed | `discovery_test.go::TestBuildPADSError`, `server_test.go` | Not added; the assertion has two existing owners (Deviations) |
| `TestClientPADICarriesEmptyServiceName` | Done | `discovery_test.go` | |
| `TestPADRRefusalIsCounted` | Done | `server_test.go` | Renamed at closure (Deviations) |
| `TestDiscoveryCounterBindsWhenTheRegistryArrivesLast` | Done | `metrics_test.go` | Added at closure for the wiring defect |
| `pppoe-service-name` functional | Written, NOT RUN | `test/pppoe/pppoe-service-name.ci` | Needs a Linux host with PPPoE kernel support; see Goal Validation |
| `pppoe-empty-service-name` interop | Written, NOT RUN | `test/interop-pppoe/scenarios/pppoe-empty-service-name/` | The Docker kernel carries no `pppoe` module; see Goal Validation |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/component/l2tp/pppoe/discovery.go` | Done | `BuildPADO`, `BuildPADS` |
| `internal/component/l2tp/pppoe/server.go` | Done | `requireServiceNameTag`, the refusal counts, the `sendFrameFn` test seam |
| `internal/component/l2tp/pppoe/metrics.go` | Done | New file; the plan named "the PPPoE metrics registration" without a path |
| `docs/architecture/l2tp/bng-5-pppoe.md` | Done | |
| `docs/architecture/l2tp/cpe-1-pppoe-client.md` | Changed | Not edited: the client needed no change, and `TestClientPADICarriesEmptyServiceName` records that in the test rather than restating it on the page |
| `docs/guide/pppoe.md` | Done | |
| `rfc/short/rfc2516.md` | Done | |
| `test/plugin/pppoe-service-name.ci` | Changed | Landed at `test/pppoe/pppoe-service-name.ci` (Deviations) |

### Audit Summary
- **Total items:** 35
- **Done:** 30
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 5 (each recorded in Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Every PADO and PADS Ze emits carries exactly one Service-Name tag | unit plus RFC discrimination | `TestPADOAlwaysCarriesServiceName` and `TestPADSAlwaysCarriesServiceName` PASS; `rfc/discrimination/rfc2516.json` holds three observed-red records where `BuildPADO` and `BuildPADS` were replaced by a panic and the red was seen |
| A PADR omitting the tag is refused the way Section 5.4 requires | unit plus RFC discrimination | `TestPADRWithoutServiceNameGetsServiceNameError` PASS; the `RFC2516-5.4-2 positive` record, break "body of handlePADR replaced by panic", observed red in 1s |
| A PADI omitting the tag is served as a request for any service | unit | `TestPADIWithoutServiceNameIsServedAsAnyService` PASS, and `TestPADIWithoutServiceNameRefusedWhenNamesConfigured` pins the other side |
| An operator sees the refusal without a packet capture | unit over the registration order the deployment produces | `TestDiscoveryCounterBindsWhenTheRegistryArrivesLast` PASS, and observed RED ("the discovery counters stayed unbound after the telemetry exporter created the registry") against the pre-closure shape that read the registry at `Subsystem.Start` |
| A real client completes discovery against Ze's empty Service-Name tags | interop | **NOT RUN, and not weakened.** `test/interop-pppoe/scenarios/pppoe-empty-service-name/` is written and registered, and `checkZeAccessConcentratorEmptyServiceName` asserts one Service-Name tag on the PADO and one on the PADS, read off a tcpdump capture, fail-closed on a missing frame. It has never executed: this host is darwin, and `./le deployment docker-pppoe-accel-test` fails its own preflight with "host kernel missing PPPoE requirements: pppoe (PPPoE pppox kernel module)". Confirmed against the Docker VM kernel directly: `docker run --rm --privileged alpine:3.21 sh -c "uname -r; modprobe pppoe"` gives `6.8.0-117-generic` and "modprobe: FATAL: Module pppoe not found". The preflight runs before scenario selection, so it blocks the two pre-existing scenarios identically and the block predates this work. It needs a Linux host whose kernel carries the `pppoe` module |
| An operator configuring a service name sees a matching client accepted and a mismatched one refused | functional | **NOT RUN, and not weakened.** `test/pppoe/pppoe-service-name.ci` is written and selected by `netnsSelections[netnsPPPoE]`. `./le qemu pppoe-test` exits 1 with "qemu guest evidence requires Linux" on this host. It needs a Linux host that can run the QEMU guest |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| Executing the `pppoe-empty-service-name` interop scenario, the `pppoe-service-name.ci` operator path, and the discrimination walk over the scenario | Environmental blocks on this darwin host that predate this work and stop the two pre-existing PPPoE scenarios identically. This is not a scope reduction: both artifacts are written, registered and committed, and no assertion was weakened to reach a pass | No spec is owed, because no CODE is outstanding. What is owed is an EXECUTION on a Linux host with PPPoE kernel support, named in Goal Validation with the exact blocking command and its output |

## Review Gate

Round 1 scope: the whole uncommitted diff of this spec, every changed Go file, doc and RFC artifact, reading each producer. Round 2 scope: the fixes round 1 made (`metrics.go`, `subsystem.go`, `server.go`, `server_test.go`, `metrics_test.go`, `discovery_test.go`, `rfc/short/rfc2516.md`) and the call sites they touched.

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/pppoe-discovery-omits-the-mandatory-service-name-tag-8a072de8-ce8e-47de-bdd2-015f5b44ca51.md`, 14 files, verdict=clean |
| `./le spec session review check` | `review_gate: OK (3 code files, clean, hashes match ...)` |
| Rounds | 2 |
| Reviewer lenses used | (1) correctness and wiring, reading each producer: `ServiceNameString`, `MatchServiceName`, `FindTag`, `Builder.addTag`, `ParseDiscovery`, `GetMetricsRegistry`, `InjectPluginMetrics`, `engine.Start`, `runYANGConfig`. (2) security and fail-closed guards, plus Ze Go style over every changed Go file. (3) test vacuity: does each assertion discriminate the behavior its name claims |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | BLOCKER | The discovery-refusal counter is never created on a PPPoE-only daemon, so AC-10 is unreachable through `/metrics` and the new `.ci` test could not pass. `Subsystem.Start` read `registry.GetMetricsRegistry()` at `engine.Start` time, and `startStandaloneTelemetry` creates the registry later in the same function | `internal/component/l2tp/pppoe/subsystem.go`, `metrics.go` | `registerDiscoveryMetrics` registers through `registry.InjectPluginMetrics`; the counter moved to a package-level `atomic.Pointer` because the store now happens on the startup goroutine while `countRefusal` runs on the discovery reader; `TestDiscoveryCounterBindsWhenTheRegistryArrivesLast` added and observed RED against the old shape |
| 2 | ISSUE | RFC 2516 Section 5.4's Service-Name-Error sentence, which the new `handlePADR` branch enforces, carried no requirement id, so no gate asked for a test of it | `rfc/short/rfc2516.md` | `RFC2516-5.4-2` allocated quoting the sentence; `TestPADRWithoutServiceNameGetsServiceNameError` tagged positive and `TestBuildPADS` tagged negative; both discrimination records observed red and written to `rfc/discrimination/rfc2516.json` |
| 3 | NOTE | `TestPADRRefusalIsLoggedAndCounted` asserts the counter and never the log line | `internal/component/l2tp/pppoe/server_test.go` | Renamed `TestPADRRefusalIsCounted` |
| 4 | NOTE | `BuildPADO` and `BuildPADS` each gained one heap allocation, the `string(t.Value)` inside `ServiceNameString`, where `AddTagCopy` allocated none | `internal/component/l2tp/pppoe/discovery.go` | Not changed. The Hot Path Rule in `ai/rules/performance.md` names the BGP paths and not this package; discovery composes one frame per subscriber session behind the PADI rate limiter, and both builders already write into the caller's buffer |
| 5 | NOTE | `requireServiceNameTag` names an imperative and returns a fact, so `if !requireServiceNameTag(pkt)` reads as a double negative | `internal/component/l2tp/pppoe/server.go` | Not changed. `hasServiceNameTag` reads better; the rename would move the doc anchor and the Wiring Test table for no behavior change |
| 6 | NOTE | `netnsSelections[netnsPPPoE]` is a hand-written name list, so a new `.ci` under `test/pppoe/` runs nowhere until an unrelated file is edited | `internal/le/qemu/netns_linux.go` | This spec's entry was added, so its own test is wired. The class is one row in `plan/journal/plugin-list-hardcoded.md` |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `test/pppoe/pppoe-service-name.ci` | Yes | `-rw-r--r-- 2.1K Sep 9 04:18` |
| `test/interop-pppoe/scenarios/pppoe-empty-service-name/role` | Yes | `-rw-r--r-- 6 Sep 9 03:56` |
| `test/interop-pppoe/scenarios/pppoe-empty-service-name/ze.conf` | Yes | `-rw-r--r-- 1.5K Sep 9 03:56` |
| `internal/component/l2tp/pppoe/metrics.go` | Yes | `-rw-r--r-- 3.8K Sep 9 04:42` |
| `internal/component/l2tp/pppoe/metrics_test.go` | Yes | `-rw-r--r-- 2.9K Sep 9 04:40` |
| `internal/le/interoplab/pppoe/check_service_name.go` | Yes | `-rw-r--r-- 7.3K Sep 9 03:58` |
| `internal/test/fixture/register_pppoe_service_name_linux.go` | Yes | `-rw-r--r-- 373 Sep 9 04:12` |
| `rfc/discrimination/rfc2516.json` | Yes | 5 records; `./le rfc discriminate stem rfc2516` shows `stale` empty |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | A tagless PADI with no names configured gets a PADO | `--- PASS: TestPADIWithoutServiceNameIsServedAsAnyService` |
| AC-2 | A tagless PADI with names configured gets none | `--- PASS: TestPADIWithoutServiceNameRefusedWhenNamesConfigured` |
| AC-3 | A tagless PADR gets Service-Name-Error, SID 0, no session | `--- PASS: TestPADRWithoutServiceNameGetsServiceNameError` |
| AC-4 | Two tags: first wins, not refused | `--- PASS: TestDuplicateServiceNameTagTakesTheFirst` |
| AC-5, AC-7 | The PADO tag sequence is exact in both configurations | `--- PASS: TestPADOAlwaysCarriesServiceName` |
| AC-6 | The PADS carries exactly one tag from a tagless PADR | `--- PASS: TestPADSAlwaysCarriesServiceName` |
| AC-8 | An unoffered name still gets the error reply | `--- PASS: TestBuildPADSError` |
| AC-9 | The client emits a zero-length tag | `--- PASS: TestClientPADICarriesEmptyServiceName` |
| AC-10 | The refusal is counted, and the counter exists on a real daemon | `--- PASS: TestPADRRefusalIsCounted`, `--- PASS: TestDiscoveryCounterBindsWhenTheRegistryArrivesLast` |
| all | The whole package under `-race` | `ok github.com/ze-software/ze/internal/component/l2tp/pppoe 2.161s`, 77 PASS, 0 FAIL |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| A PADI with no Service-Name tag and no configured names | `TestPADIWithoutServiceNameIsServedAsAnyService` through `handlePADI` | Yes: the test drives the handler. The `.ci` covers the configured-name half instead |
| A PADR with no Service-Name tag on the AC's socket | `TestPADRWithoutServiceNameGetsServiceNameError` through `handlePADR` | Yes: the test drives the handler, never the helper alone |
| A PADI with an empty Service-Name value | `TestPADOAlwaysCarriesServiceName` | Yes: the exact tag sequence is asserted |
| A PADR accepted with an empty Service-Name value | `TestPADSAlwaysCarriesServiceName` | Yes |
| An operator reading `/metrics` | `test/pppoe/pppoe-service-name.ci`, `http=wait ... contains=ze_pppoe_discovery_refusals_total{reason="service-name-mismatch"} 1` | Read the file: it configures `service-name internet`, dials `internet` (accepted) then `voice` (refused), then scrapes. NOT executed (Goal Validation). The registration order it depends on is proven by `TestDiscoveryCounterBindsWhenTheRegistryArrivesLast` |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed, by reference source rather than by a live client | The PADI half was dropped by the owner's decision, so the assumption now covers the PADR only. accel-ppp ships the identical refusal (`pppoe_recv_PADR`, quoted in Other Implementations), which bounds how many real clients can depend on the leniency. The interop run that would confirm it against a live pppd did NOT execute (Goal Validation) |
| A-2 | confirmed, by RFC and reference source rather than by a live client | RFC 2516 Section 5.1 makes a zero-length value legitimate; FreeBSD's `ng_pppoe` substitutes an empty tag and accel-ppp's `allocate_channel` supplies `empty_service_name`. The interop run did NOT execute |
| A-3 | confirmed | `grep -rn AddTagCopy --include='*.go' internal` outside tests: nine remaining call sites, every one an optional tag (the AC-Cookie echo in PADR, Host-Uniq three times, Relay-Session-Id four times, PPP-Max-Payload). No mandatory tag reaches it, and the helper is unchanged |
| A-4 | confirmed | `TestClientPADICarriesEmptyServiceName` PASS: `BuildPADI` and `BuildPADR` each emit exactly one zero-length tag with an empty configured name, with no production change |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| `bng-5-pppoe.md`: Ze refuses a PADR with no Service-Name tag and serves a PADI with none | `server.go::requireServiceNameTag` and `handlePADI` calling unchanged `MatchServiceName` | Yes, read both |
| `bng-5-pppoe.md`: always emits exactly one Service-Name tag in the PADO and the PADS | `discovery.go::BuildPADO`, `BuildPADS` | Yes, read both |
| `bng-5-pppoe.md`: the registration-order entry | `metrics.go::registerDiscoveryMetrics` and the two call sites in `runYANGConfig` | Yes, read the producer and both call sites |
| `docs/guide/pppoe.md` reason table | The five constants in `metrics.go` and the seven `countRefusal` call sites in `server.go` | Yes, every reason has at least one call site |
| Every source anchor resolves | `./le docs-to-code index-check` | 4 failures, all pre-existing, none in a file this spec touched |
| RFC status row | `rfc/short/rfc2516.md` Meta, regenerated by `./le rfc index-update` | Yes. `Support status` stays `Partial`: `./le rfc check` refuses `Supported` while `rfc/extraction/rfc2516.json` holds no sign-off |
| 9. RFC behavior newly proven | `./le rfc check`: 115 violations, none naming rfc2516, unchanged from the pre-work baseline | Yes |

## Core Insight

A unit test handed a dependency directly cannot see that the daemon never
supplies it. `TestPADRRefusalIsCounted` builds its own registry, hands it to the
server, and passes whether or not any real daemon ever creates one; it stayed
GREEN through the whole defect. The test that found it drives the REGISTRATION,
in the arrival order the deployment produces, and asserts the counter exists
afterwards. Where a feature depends on something arriving later, the test has to
own the arrival, never the arrived value.
