# Spec: code that cannot answer refuses instead of returning a plausible value

| Field | Value |
|-------|-------|
| Status | verification |
| Scope | protocol |
| Priority | P1 |
| Release reason | Shipped producers answer a default, a zero or a constant where they cannot answer, so operators and peers act on values Ze never measured; several rows are RFC defects in implemented capabilities (RADIUS, LDP, IS-IS, ADD-PATH, LLGR) |
| Depends | spec-plugin-optional-dependency-refusal (closed; its refusal is `config.refuseAbsentOptionalProviders` and `registry.MissingDependencyError`, documented in `docs/architecture/plugin/feature-gates.md`; it owned the "not built reads as not configured" rows of the 2026-10-20 dependency census; this spec reuses its refusal for the two build-gated rows it owns) |
| Phase | 7/7 |
| Handoff | verify |
| Updated | 2026-10-24 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Root cause S2 of the journal backlog (owner order, 2026-10-23: turn the open journal rows
into root-cause specs). 46 open rows across five problem classes share one mechanism: a
function reaches a state where it has no answer (a lookup missed, an input it does not
recognise, a resource it cannot read, a value the protocol gives no "absent" spelling) and
its signature or its branch structure gives it no way to say so. It then returns what the
type defaults to, or what the author guessed: `true` for an absent authenticator, `nil` maps
for a missing resolver, "absent" for any link-read error, path identifier 0 as "no
identifier", a literal `StatusHealthy`, ORIGIN IGP. The caller cannot tell that value from
a real one, so the defect surfaces far from its cause and reads as data
(`ai/rules/principles.md`, first directive).

Why it matters for shipping Ze: these values reach peers (a label bound to two FECs, a VPN
NLRI misframed, an LLGR_STALE route sent to a non-LLGR peer), reach operators (a healthy
health check that cannot fail, `show` answering empty for a path it does not render), and
reach security boundaries (a RADIUS EAP answer with no Message-Authenticator accepted).

The fix has two halves. First, every owned producer gets a refusal path and its callers
handle it. Second, a change-set check, `./le arch refuse-by-default`, refuses the shapes
that produced the family, so new code cannot reintroduce it. Where zero really is the
answer that another branch relies on, the check requires the guard to be named
(`ai/rules/principles.md`: "A zero that another branch relies on is a guard, and it MUST be
named as one").

## Required Reading

### Architecture Docs
- [ ] `ai/rules/principles.md` - first directive, silently wrong values
  → Constraint: code that cannot answer MUST say so; a zero another branch relies on is a guard and is named as one
- [ ] `docs/architecture/core-design.md` - registration, plugin boundary
  → Constraint: a cross-boundary call that no-ops when the plugin runs external is this defect; the refusal crosses the boundary as an error answer, never as an empty success
- [ ] `docs/architecture/testing/verify-freshness-scope.md` - the fixed verify stage population (`internal/le/verify/engine/stages.go`)
  → Decision: the new check is a change-set stage beside `arch enumeration` and `arch compound-guard`, so the tree's existing instances are reported, not a red on every commit
- [ ] `docs/contributing/ze-go-style.md` - error and naming conventions for the new error types
- [ ] `docs/architecture/plugin/feature-gates.md` - the not-built refusal spec-plugin-optional-dependency-refusal (closed) delivered
  → Constraint: rows 5 to 8 of the census are owned there; do not duplicate its refusal, call it

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc3579.md` - RADIUS EAP, Message-Authenticator
  → Constraint: RFC 3579 Section 3.1: "Access-Accept, Access-Reject or Access-Challenge packets including EAP-Message attribute(s) without a Message-Authenticator attribute SHOULD be silently discarded by the NAS."
- [ ] `rfc/short/rfc3031.md` - label uniqueness
  → Constraint: RFC 3031 Section 3.14: "In all other cases, Rd MUST NOT distribute to Ru bindings of the same label value to two different FECs."
- [ ] `rfc/short/rfc5036.md` - LDP unknown TLV handling
  → Constraint: RFC 5036 Section 3.3: "Upon receipt of an unknown TLV, if U is clear (=0), a notification MUST be returned to the message originator and the entire message MUST be ignored; if U is set (=1), the unknown TLV MUST be silently ignored and the rest of the message processed as if the unknown TLV did not exist."
- [ ] `rfc/short/rfc5303.md` - IS-IS three-way adjacency
  → Constraint: RFC 5303 Section 3.2: "If the option is present and contains invalid Adjacency Three-Way State, the PDU SHALL be discarded and no further action is taken."
- [ ] `rfc/short/rfc7911.md` - ADD-PATH identifier
  → Constraint: RFC 7911 Section 3: "In order to carry the Path Identifier in an UPDATE message, the NLRI encoding MUST be extended by prepending the Path Identifier field, which is of four octets." Section 2: "A BGP speaker that receives a route should not assume that the identifier carries any particular semantics." Presence follows the negotiated capability, never the value
- [ ] `rfc/short/rfc9494.md` - LLGR
  → Constraint: RFC 9494 Section 4.3: "The route SHOULD NOT be advertised to any neighbor from which the Long-Lived Graceful Restart Capability has not been received."
- [ ] `rfc/short/rfc2205.md` - RSVP Path message
  → Constraint: RFC 2205 Section 3.1.3 grammar: "<sender descriptor> ::= <SENDER_TEMPLATE> <SENDER_TSPEC>"; a sender descriptor without SENDER_TSPEC is malformed
- [ ] `rfc/short/rfc4090.md` - RRO protection flags
  → Constraint: RFC 4090 Section 4.4, "Local protection available: 0x01": "Indicates that the link downstream of this node is protected via a local repair mechanism"; the flag reports state, so it is read from the bypass LSP's state, not from config
- [ ] `rfc/short/rfc9069.md` - BMP Loc-RIB Route Monitoring
  → Constraint: RFC 9069 Section 5.4 quotes RFC 7854 Section 4.6: "Following the common BMP header and per-peer header is a BGP Update PDU."; the PDU carries the best path's own attributes

**Key insights:**
- The family is a signature problem as often as a branch problem: `interfaceExists` returns `bool`, `allocateLabelLocked` returns `uint32`, `DefaultConfig` returns `string`. No branch can refuse through those signatures
- ADD-PATH presence by value is one fact coded at seven sites; presence must come from the negotiated capability carried with the route

## Current Behavior (MANDATORY)

**Source files read (verified 2026-10-23):**
- [ ] `internal/component/radius/packet.go` - `verifyResponseMessageAuthenticator` returns `true` when the attribute is absent (`if !present { return true }`); `dispatchResponse` in `client.go` then delivers an EAP-bearing Accept with no Message-Authenticator. The send side already refuses the mirror case (`carriesEAPMessage` check in the encoder)
- [ ] `internal/component/iface/config_apply.go` - `interfaceExists` is `GetLink` error equals nil, so any read failure reads as "absent" and the apply recreates a present device
- [ ] `internal/plugins/ldp/lib.go` - `allocateLabelLocked` loops over the label space and, when every label is in use, returns `l.nextLabel`, a label already bound; `EnsureLocal` stores it for a second FEC
- [ ] `internal/core/cos/cos.go` - `Resolve` returns nil maps and nil error when no resolver is registered, so a profile reference in a build without the cos plugin is accepted and does nothing
- [ ] `internal/plugins/trafficusage/attach_linux.go` - `ebpfSupported` is a constant nil, so the readiness diagnostic cannot fail on Linux
- [ ] `internal/component/l2tp/health.go` - `checkHealth` answers Healthy whenever the service is registered; nothing it reads can be false about the kernel data plane
- [ ] `internal/component/bgp/rib/route.go` - the index key sets `hasPathID` from `pathID != 0`; the same test appears in `bgp/format.go`, `plugins/rib/rib_replay.go`, `plugins/rib/rib_pipeline.go`, `format/text_json.go`, `plugins/rib/rib.go`
- [ ] `internal/component/plugin/server/startup_autoload.go` - `appendDataPlaneWriter` logs a WARN and starts with no FIB writer when no compiled-in plugin programs the configured backend

**Behavior to preserve:**
- Every existing explicit refusal and its message; the send-side RADIUS Message-Authenticator refusal
- Wire output of every family where the path identifier is negotiated (the fix changes presence for identifier 0 only)
- RFC 5036 U=1 unknown TLVs stay silently ignored (`capability_rfc5561_test.go`)

**Behavior to change:** every owned row's producer answers a refusal (typed error, a discard with a counted reason, or an explicit "unknown" state) instead of the plausible value. The row table below names each.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Wire input (RADIUS response, LDP Init, IS-IS Hello, RSVP PATH, BGP UPDATE), config verify and apply, operator `show` and web pages, health and doctor checks, plugin RPC answers

### Transformation Path
1. The producer meets a state it cannot answer (lookup miss, unknown input, unreadable resource)
2. Today: it returns the type's zero, a constant or a fallback. After: it returns a typed refusal (`ErrNotFound`, `ErrNotBuilt` from the dependency spec, `ErrUnrecognised`, `ErrExhausted`) or the protocol's discard action
3. The caller branches on the refusal: config verify refuses the config, the wire handler discards and counts, the show handler answers an error naming the path, the health check reports Unknown or Degraded with the reason

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine and plugin | refusal travels as the RPC error answer, never as an empty success | No |
| Config verify and apply | verify refuses what apply cannot honour | No |
| Wire | protocol discard or notification per the quoted RFC sentence | No |

### Integration Points
- `internal/le/arch/` - the new `refusebydefault` package registers like `arch enumeration` (`internal/le/arch/enumeration/register.go`) and joins the stage list in `internal/le/verify/engine/stages.go`
- The dependency spec's not-built refusal - reused for `cos.Resolve` and `appendDataPlaneWriter`

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | refusals use each producer's existing error return or gain one; no side channel |
| No unintended coupling (components stay isolated) | Yes | typed errors live in the producing package; callers match with `errors.Is` |
| No duplicated functionality (extends existing, does not recreate) | Yes | not-built refusal is the dependency spec's; ADD-PATH presence becomes one accessor replacing seven value tests |
| Zero-copy preserved where applicable (refs, not copies) | Yes | no encoding path gains a copy; presence is a flag already carried |
| Registration over hardcoding, outbound | Yes | the check registers through `leroot.Register`; no switch learns its name |
| Registration over hardcoding, inbound | Yes | the check's rule set is read from AST shapes, not from a list of package names; searched `stages.go` (one stage line is the declared population, as for every arch stage) |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The four rule shapes below cover every owned row | row-by-row reading 2026-10-23 | an owned row has no rule; add one before closing | each owned row's regression test plus a fixture per rule | unvalidated |
| A-2 | A change-set check produces few false positives on new code | `arch compound-guard` precedent judges unpushed lines | authors meet a noisy check and annotate by reflex | run the check over the last 200 commits in report mode; count findings per true defect | confirmed 2026-10-23: `report` findings intersected with the lines `git diff -U0 a870c72164` (HEAD~200, to the working tree) changed: 10 findings over 200 commits, 6 true (a read failure or a wrong type read as "no" or absent: `filter_family/config.go` `exportChain`, `le/commit/owed_trailers.go` `answering.answered` x2 and `answering.reviewed`, `le/hookruntime/lifecycle.go` `linkedWorktree`, `le/verify/green.go` `provenParts`), 4 relied-on zeros that take the annotation (`reactor/policy_reload.go` `reconcileExportFinal` and `flushPolicyReplay`, `green.go` `recordGreen`, `exabgp/bridge/bridge_route_forms.go` `refuseMaskNotNumber`), 0 pure false positives: 1.7 findings per true defect. Reached after five precision cuts, each pinned by a fixture case: `false` beside a nil error (the flag hands the miss on), a branch that records the miss (field or element write, counter step, error handed to an outer variable), an answer that carries the error, a parse's own "no" (`Parse*`, `Atoi`), and a bool function literal (visitor, yield, poll predicate). The first cut measured 28 findings over the same range |
| A-3 | Every ADD-PATH consumer has the negotiated flag at hand | `AddPath` fields already present at the `format.go` and `rib_pipeline.go` sites | a consumer needs the flag threaded | gopls references on each value test | confirmed with exception: the `format.go` and `rib_pipeline.go` sites hold `AddPath`; four further sites found (`bgp.RouteKey`, the adj_rib_in show key, the `appendNLRIJSONValue` bare-prefix branch, `handleSent`), and the flag is threaded for `NewINET` callers (new `hasPathID` parameter) and `adj_rib_in.RawRoute.HasPathID`. Presence accessors for the AC-3 check: `nlri.HasPathID(n)` for NLRIs, `Route.AddPath` for `bgp.Route`, `RawRoute.HasPathID` |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Refusing where Ze accepted before breaks a working interop | interop red after a row fix | the RFC sentence decides; where the RFC is silent read FRR and BIRD and follow both |
| R-2 | Health checks reporting Unknown alarm operators | doctor output change | Unknown carries the reason and the command that measures it (`ai/rules/user-facing-errors.md`) |
| R-3 | Annotation becomes a reflex escape | many `refuse-by-default` annotations in one commit | the annotation requires a named guard and the check prints every annotation in the change set for the reviewer |

## Blast Radius
| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | a producer refuses a value that was valid: a config rejected, a packet discarded, a show error |
| How is it reverted? | one commit per family; the check is one stage line |
| Who else touches this path? | `spec-plugin-optional-dependency-refusal` (startup autoload), `spec-startup-deferred-reapply-idempotency` (`interfaceExists` guards), the S1 schema spec (`validateYangType`) |

## Wiring Test (MANDATORY -- NOT deferrable)
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le arch refuse-by-default check` over a change set holding a fixture of each rule | → | `refusebydefault.Check` | `TestRefuseByDefaultRefusesEachRuleFixture` |
| `./le verify current` stage list | → | `stages.go` stage `arch refuse-by-default` | `TestVerifyStagesIncludeRefuseByDefault` |
| RADIUS Access-Accept with EAP-Message and no Message-Authenticator on the socket | → | `dispatchResponse` | `TestRadiusDiscardsEAPAnswerWithoutMessageAuthenticator` |
| config naming a cos profile in a build without the cos plugin | → | iface verify through `cos.Resolve` | `test/parse/cos-profile-not-built-refused.ci` |
| LDP FEC allocation with the label space full | → | `EnsureLocal` through `allocateLabelLocked` | `TestLDPLabelExhaustionRefusesSecondBinding` |

## Acceptance Criteria
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A change adds a `default:` arm, or a fall-off after a switch, in a function with an `error` result that returns a nil error | `./le arch refuse-by-default check` refuses it, naming file, line and the rule |
| AC-2 | A change adds a branch on a lookup miss (`!ok` from a map index or type assertion, a nil registered function, a non-nil error from a read) that returns `true`, a zero value, or nil error | refused, naming the rule |
| AC-3 | A change compares an ADD-PATH path identifier with zero | refused, naming the negotiated-presence accessor to use |
| AC-4 | A change writes a state or health answer from a literal (`StatusHealthy` with no measured read in the function, a literal boolean into a state member) | refused |
| AC-5 | The same shapes carrying a `refuse-by-default:` annotation that names the guard the zero serves | accepted, and listed in the check's output |
| AC-6 | An annotation with no named guard | refused |
| AC-7 | `./le arch refuse-by-default report` over the tree | lists every instance in the tree; none of the owned rows' producers appears in it after this spec |
| AC-8 | Family F1 (lookup miss read as an answer): an unreadable link, a missing cos resolver, an unreadable instance name, a failed RPC publication, a destination the RIB never saw, a web interface read failure | each answers an error naming what could not be read and the command that fixes it; none answers "absent", empty or the default |
| AC-9 | Family F2 (unrecognised input accepted): a gNMI Get with PROTO encoding, `set debug module m level debug flag x`, an LDP Init with an unknown U=0 TLV, an IS-IS Hello with an invalid three-way state, an RSVP PATH with no SENDER_TSPEC, an IPv6 BFD session on a loop with no IPv6 socket, a RADIUS EAP answer with no Message-Authenticator, label space exhausted | each is refused or discarded as the quoted RFC sentence says (LDP: notification and message ignored; IS-IS and RADIUS: PDU discarded; LDP labels: no second binding), with a counter or an error naming the cause |
| AC-10 | Family F3 (zero as identity): ADD-PATH negotiated and a route with path identifier 0 | indexed, rendered and re-advertised as carrying identifier 0; without ADD-PATH no identifier is shown whatever the value |
| AC-11 | Family F3: a nil LLGR egress state, a 0.0 degC thermal reading, a DDoS clear for a resolved victim, a KEEPALIVE for a dynamic peer before its AS resolves | nil egress state refuses the stale route toward a non-LLGR peer; 0.0 is reported as a reading; the clear closes the incident; the event names the AS or says it is not yet known |
| AC-12 | Family F4 (constant or config reported as measured): l2tp health, traffic-usage eBPF readiness, LG `primary`, BMP Loc-RIB Route Monitoring, RSVP RRO local protection, VPP interface addresses | each reads the measured state; where it cannot, it answers Unknown with the reason |
| AC-13 | Every owned row in the table below | has a regression test that was red against the producer before its fix; the row's representative input no longer reproduces |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRefuseByDefaultRefusesEachRuleFixture` | `internal/le/arch/refusebydefault/refusebydefault_test.go` | AC-1 to AC-4, one testdata fixture per rule | |
| `TestRefuseByDefaultAnnotationNeedsGuard` | same | AC-5, AC-6 | |
| `TestRefuseByDefaultReportListsTree` | same | AC-7 | |
| `TestRadiusDiscardsEAPAnswerWithoutMessageAuthenticator` | `internal/component/radius/client_test.go` | AC-9, RFC 3579 Section 3.1 | |
| `TestLDPLabelExhaustionRefusesSecondBinding` | `internal/plugins/ldp/lib_test.go` | AC-9, RFC 3031 Section 3.14 | |
| `TestLDPInitUnknownTLVUClearNotifies` | `internal/plugins/ldp/wire_test.go` | AC-9, RFC 5036 Section 3.3 | |
| `TestISISHelloInvalidThreeWayStateDiscarded` | `internal/plugins/isis/rfc5303_invalid_state_wiring_test.go` | AC-9, RFC 5303 Section 3.2: `handleIIH` counts the discarded Hello | |
| `TestRFC5303InvalidStateLeavesAdjacencyUntouched` (existing, RFC5303-3.2-7) | `internal/plugins/isis/adjacency/rfc5303_invalid_state_test.go` | AC-9, RFC 5303 Section 3.2: the FSM discards before mutation | |
| `TestRFC2205PathWithoutSenderTSpecRefused` (existing, tagged), `TestRFC2205PathWithoutSenderTSpecDropped` (existing) | `internal/plugins/rsvpte/rfc2205_path_test.go`, `repairs_rfc2205_test.go` | AC-9, RFC 2205 Section 3.1.3 | |
| `TestRadiusUnsignedEAPDiscardCounted` | `internal/component/radius/metrics_test.go` | AC-9: the discard reaches the `ze_radius_unsigned_eap_discards_total` series | |
| `TestRFC4090FlagsClearUntilBypassEstablished`, `TestRFC4090RefreshTracksBypassState` (existing, RFC4090-6-3, 6-4) | `internal/plugins/rsvpte/rfc4090_frr_bypass_test.go` | AC-12: RRO local protection follows the bypass LSP's state | |
| `TestAddPathIdentifierZeroIsAnIdentifier` | `internal/component/bgp/rib/route_test.go` and each consumer package | AC-10, RFC 7911 Sections 2 and 3 | |
| `TestLLGREgressFilter_NilStateWithdrawsEBGP`, `_NilStateDepreferencesIBGP` (existing, RFC9494-4.3-3, 4.6-2, 4.6-3; planned as `TestLLGREgressNilStateRefusesStaleRoute`) | `internal/component/bgp/plugins/gr/rfc9494_gr_egress_test.go` | AC-11, RFC 9494 Section 4.3 | -> Decision: fixed at 6ec2a10bcf with these tests; a second test of the same branch would duplicate them |
| `TestDetectThermal_ZeroReadingIsAReading` | `internal/component/host/thermal_linux_test.go` | AC-11 | |
| `TestClearedCarriesTheDetectedVictim` | `internal/plugins/ddos/detect/characterize_test.go` | AC-11 | |
| `TestKeepaliveNamesTheNegotiatedASOfADynamicPeer` | `internal/component/bgp/reactor/reactor_notify_test.go` | AC-11 | |
| `TestInterfaceExistsReadErrorIsNotAbsence` | `internal/component/iface/config_apply_test.go` | AC-8 | |
| `TestL2TPHealthReadsDataPlane`, `TestTrafficUsageReadinessCanFail` | owning packages | AC-12 | |
| one regression test per remaining owned row, named in the row's fix commit | owning packages | AC-13 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| LDP local label | 16 to 1048575 | 1048575 (last free) | 15 | none free: refusal |
| ADD-PATH identifier | 0 to 4294967295 | 0 and 4294967295 both valid | N/A | N/A |
| BGP OPEN size is not owned here | N/A | N/A | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `cos-profile-not-built-refused` | `test/parse/` | operator loads a cos reference on a build without cos and is refused with the tag | |
| `show-bgp-peer-bare-list` | `test/plugin/` | web or SSH `show bgp peer` with no key lists entries or names the path | |
| `addpath-identifier-zero-forwarded` | `test/plugin/` | route with identifier 0 under ADD-PATH reaches the peer with four identifier octets | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `addpath-zero-id-frr` | `test/interop/scenarios/`, checkers in `internal/le/interoplab/bgp/` (`scenarioAddPathZeroIDFRR`) | FRR | identifier 0 round trip under ADD-PATH | written, NOT RUN: the Docker lab must run Ze's kernel (macOS `ze` Colima profile or the Ze-kernel QEMU guest). Run: `INTEROP_SCENARIO=addpath-zero-id-frr ./le test integration interop`. It proves acceptance only (vacuity trap 4): Ze's identifier octets come from the session flag in `nlri.WriteNLRI` (reached from `reactor_api_batch.go` `writeBatchNLRI`), which this spec did not change, so reverting the spec's diff cannot redden it. Red phase owed: in `internal/core/bgp/nlri/nlri.go` `WriteNLRI`, change `if addPath && n.SupportsAddPath()` to `if addPath && n.SupportsAddPath() && n.PathID() != 0`, rerun the same command (the harness rebuilds the image from the tree), confirm the checker fails on FRR's missing `AddPath ID: RX 0,` path, restore, confirm green, and record the red |
| `radius-admin-eap-unsigned-freeradius` | `test/interop-radius/scenarios/`, checker `internal/le/interoplab/radius/check_unsigned_eap.go` | FreeRADIUS 3.2.10, through `test/interop-radius/unsigned-relay.pl` in its container (FreeRADIUS's `rad_encode` always signs over UDP) | EAP answer without Message-Authenticator is discarded, logged and counted; the same login with the signature kept completes | green 2026-10-23 (with the four other radius scenarios) |
| `ldp-init-unknown-tlv-frr` | `test/interop/scenarios/`, checker `internal/le/interoplab/bgp/check_ldp_init_unknown_tlv.go` | FRR ldpd; an nft rule in ze's container rewrites one TLV type in FRR's real Init (FRR cannot send an unknown TLV) | unknown U=0 TLV in Init draws an Unknown TLV notification FRR receives, and the Init is ignored (no OPERATIONAL until the rewrite is removed) | |

## Files to Modify
- `internal/component/radius/client.go`, `packet.go` - discard EAP answer without Message-Authenticator
- `internal/plugins/ldp/lib.go`, `wire.go` - exhaustion error; unknown U=0 TLV
- `internal/component/iface/config_apply.go` and every `Backend.GetLink` - typed not-found
- `internal/core/cos/cos.go` - not-built refusal from the dependency spec
- `internal/component/bgp/rib/route.go`, `bgp/format.go`, `plugins/rib/rib_replay.go`, `rib_pipeline.go`, `rib.go`, `format/text_json.go`, `plugins/nlri/vpn/vpn.go`, `internal/core/bgp/nlri/inet.go` - ADD-PATH presence from the capability
- every producer named in the Owned journal rows table
- `internal/le/verify/engine/stages.go` - one stage line
- `docs/architecture/testing/verify-freshness-scope.md`, `docs/contributing/ze-go-style.md` - the new stage and the annotation rule

## Files to Create
- `internal/le/arch/refusebydefault/` - `register.go`, `refusebydefault.go`, `actions.go`, tests and testdata
- `test/parse/cos-profile-not-built-refused.ci`, `test/plugin/show-bgp-peer-bare-list.ci`, `test/plugin/addpath-identifier-zero-forwarded.ci`
- `test/interop/scenarios/addpath-zero-id-frr/`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | no new config |
| YANG validation constraints | No | `validateYangType` default arm is fixed by refusing; S1 owns the decimal64 type |
| YANG custom validators | No | none added |
| CLI commands/flags | Yes | `./le arch refuse-by-default check` and `report`, registered through `leroot` |
| CLI grammar (keyword before value) | Yes | verbs follow `arch enumeration` |
| Editor autocomplete | N-A | le tool, no editor surface |
| Functional test for new RPC/API | N-A | no new RPC |
| Pipe completeness | Yes | shape `ShapeDoc` like `arch enumeration` |
| Env var registration | No | none |
| Doctor check for runtime dependencies | Yes | l2tp and traffic-usage health become measured; their doctor output changes |
| Prometheus counters/metrics | Yes | discard counters for RADIUS and IS-IS reuse existing per-protocol drop counters where present |
| BGP family surface (new SAFI / capability / attribute) | No | ADD-PATH presence only |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | refusals of existing features |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | le only, documented in its help |
| 4 | API/RPC added/changed? | Yes | gNMI Get refusal of unsupported encoding in `docs/architecture/api/` gNMI page |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | Yes | `docs/guide/health-checks.md` for Unknown health |
| 7 | Wire format changed? | Yes | ADD-PATH identifier 0 presence in the wire page that describes ADD-PATH |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc3579.md`, `rfc3031.md`, `rfc5036.md`, `rfc5303.md`, `rfc7911.md`, `rfc9494.md`, `rfc2205.md` rows and `docs/features/rfc-status.md` |
| 10 | Test infrastructure changed? | Yes | `docs/architecture/testing/verify-freshness-scope.md` stage list |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | Yes | `docs/contributing/ze-go-style.md`: refusal over default, the annotation |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | Yes | the subsystem telemetry pages of any counter added |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | declared by the files named here and affected: `docs/architecture/ldp/mpls-ldp.md` (label exhaustion refusal, U=0 TLV notification), `docs/architecture/route-types.md` (ADD-PATH identifier 0 presence), `docs/architecture/traffic/cos-plugin.md` (refusal with no resolver), `docs/features/interfaces.md` (link read error no longer reads as absent), `docs/research/l2tpv2-ze-integration.md` (health reads the data plane; research page, re-read to confirm it states no current behaviour). Re-run `./le spec citation anchors spec plan/release/fixes/spec-refuse-instead-of-defaulting.md` per phase |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | verify `show bgp peer` examples on the web terminal page |

## Implementation Steps

1. **Phase: Wiring** - the check skeleton registered, the stage line added, fixtures written; `TestRefuseByDefaultRefusesEachRuleFixture` red
2. **Phase: the check** - the four rules over the change set and `report` over the tree; AC-1 to AC-7
3. **Phase: F2 protocol rows** - RADIUS, LDP, IS-IS, RSVP, BFD, gNMI, label exhaustion, withdrawal and IRR cap rows; each red first, RFC quote above the refusal, discrimination record per `ai/rules/rfc-compliance.md`
4. **Phase: F3 ADD-PATH** - one presence accessor, the seven value tests replaced, interop scenario
5. **Phase: F1 rows** - typed not-found and not-built refusals, callers updated
6. **Phase: F4 rows** - measured state and Unknown health
7. **Phase: report clean** - `./le arch refuse-by-default report` lists none of the owned producers

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | every owned row has its regression test and a red recorded |
| Correctness | every refusal matches the quoted RFC sentence's action (discard, notify, refuse); no refusal where the RFC says ignore (LDP U=1) |
| Data flow | refusals cross the plugin boundary as error answers |
| Rule: principles | no new zero-as-answer introduced while fixing; each annotation names its guard |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| the check | `./le arch refuse-by-default check` over a fixture commit refuses |
| owned rows fixed | `./le arch refuse-by-default report` plus each row's test |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | RADIUS EAP answers without Message-Authenticator discarded; LDP and IS-IS malformed input discarded without state change |
| Fail open | no health or readiness check can answer Healthy without a measured read |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the producer; if misunderstood, back to research |
| Lint failure | Fix inline |
| Functional test fails | Check the AC: wrong AC to design, correct AC to implement |
| Audit finds a missing AC | Back to the relevant phase |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights
- The not-built half of the family is already designed in the dependency spec; this spec owns the refusal shape, not a second not-built mechanism

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Change-set check with a tree report | a tree-wide gate red from day one | the tree holds instances owned by other specs; a tree-wide red blocks every commit; `arch compound-guard` is the precedent |
| Annotation names the guard | a bare `nolint` style escape | `ai/rules/principles.md` requires a relied-on zero to be named as a guard |
| ADD-PATH presence from the negotiated capability | a sentinel identifier | RFC 7911 reserves no value |
| `show bgp rib ... graph` answers `{"graph":{"nodes":[...],"edges":[...]}}` and refuses over `graph.MaxNodes`; the CLI draws the box through a text renderer keyed by the envelope key `graph`, registered from `bgp-rib` | keep the pre-rendered box text as a JSON string | `ai/rules/cli.md`: a payload is structured data, never finished text; `\| json` must show the structure |

Owner approval (Thomas, 2026-10-24): the functional tests rib-graph, rib-graph-filtered and rib-graph-best (fixture `ribGraph13`, `internal/test/fixture/plugin_fixture_13_rib.go`) may change to assert the structured payload instead of the pre-rendered box text.

## Known Limitations
- The rows of this family owned by `spec-plugin-optional-dependency-refusal` (2026-10-20 census rows: auto-load hard dependency, mrt and NTP not built, ddos/exabgp/flowexport undeclared BGP, redistribute and ls_export free strings) stay there

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "quoted requirement"` above each refusal this spec adds, using the
sentences quoted in Required Reading.

## Owned journal rows

Owner approval (Thomas, 2026-10-24): the doctor `collectSchemaListeners` row removes the hardcoded listener fallback and deletes `TestDoctorGnmiListenerIsProbed` (`TestDoctorGnmiDefaultEndpointIsProbed` stays); this spec's fix also covers AC-19 of `spec-daemon-runtime-one-off-defects.md`.

| Class file | Date | Surface (short) | Family |
|------------|------|-----------------|--------|
| `silent-fall-through.md` | 2026-10-21 | `Server.appendDataPlaneWriter` starts with no FIB writer | F1 |
| `silent-fall-through.md` | 2026-10-20 | `sendPostStartupTo` failure logged at DEBUG | F1 |
| `silent-fall-through.md` | 2026-10-20 | `LIB.allocateLabelLocked` returns a bound label | F2 |
| `silent-fall-through.md` | 2026-10-19 | gNMI `Server.Get` ignores `encoding` | F2 |
| `silent-fall-through.md` | 2026-10-18 | `verifyResponseMessageAuthenticator` true when absent | F2 |
| `silent-fall-through.md` | 2026-10-17 | `BGPConsumer.InjectRoute` warning read as announced=0 | F1 |
| `silent-fall-through.md` | 2026-10-15 | `cos.Resolve` nil maps with no resolver | F1 |
| `silent-fall-through.md` | 2026-10-13 | mrt peer state handlers' failed type assertion | F1 |
| `silent-fall-through.md` | 2026-10-13 | `yangTypeToValueType` maps a union list key to `TypeString`, so `ValidateListKey` never checks its member types | F2 |
| `silent-fall-through.md` | 2026-10-12 | doctor `collectSchemaListeners` falls back to a hardcoded list | F1 |
| `silent-fall-through.md` | 2026-10-08 | debug `setModuleSetting` drops trailing keywords | F2 |
| `silent-fall-through.md` | 2026-10-07 | `captureRawStart` answers done when it cannot capture | F1 |
| `silent-fall-through.md` | 2026-10-02 | rib family with no splitter dropped at Debug | F1 |
| `silent-fall-through.md` | 2026-09-21 | `validateYangType` default arm accepts | F2 |
| `constant-reported-as-measured-state.md` | 2026-10-13 | from-Established FSM arm reports `connection lost` for every cause | F2 |
| `declared-format-contradicts-payload.md` | 2026-08-19 | `RawJSON.MarshalJSON` quotes invalid JSON instead of refusing | F2 |
| `field-carries-two-meanings.md` | 2026-10-23 | text `AppendOpen` writes the OPEN's own AS as `remote as` | F3 |
| `zero-value-as-valid-answer.md` | 2026-10-22 | `appendNLRIJSONValue` writes a placeholder route on decode failure | F2 |
| `zero-value-as-valid-answer.md` | 2026-10-19 | `interfaceExists` any error reads as absent | F1 |
| `zero-value-as-valid-answer.md` | 2026-10-19 | DHCPv6 `dadUnreported` on VPP | F4 |
| `zero-value-as-valid-answer.md` | 2026-10-19 | VPP `detailsToInfo` empty addresses | F4 |
| `zero-value-as-valid-answer.md` | 2026-10-09 | LG `routes/protocol/{name}` 0 routes for a name | F1 |
| `zero-value-as-valid-answer.md` | 2026-09-20 | `serializeRouteItem` path ID compared with zero | F3 |
| `zero-value-as-valid-answer.md` | 2026-09-20 | `vpnToJSON`, `LabeledUnicast.String` path ID compared with zero | F3 |
| `zero-value-as-valid-answer.md` | 2026-09-08 | ddos `emitCleared` zero target never closes | F3 |
| `zero-value-as-valid-answer.md` | 2026-09-06 | `NewINET` path ID 0 as absent | F3 |
| `zero-value-as-valid-answer.md` | 2026-08-10 | gr egress nil state read as nothing stale | F3 |
| `zero-value-as-valid-answer.md` | 2026-08-12 | `readOneHwmon` drops a 0.0 reading | F3 |
| `zero-value-as-valid-answer.md` | 2026-09-25 | resolve `DefaultConfig` read failure as missing | F1 |
| `zero-value-as-valid-answer.md` | 2026-10-08 | `buildParamMeta` error becomes nil | F1 |
| `zero-value-as-valid-answer.md` | 2026-10-19 | web interface detail swaps config for live on error | F1 |
| `zero-value-as-valid-answer.md` | 2026-10-23 | web interface list reads any `ListInterfaces` error as no backend | F1 |
| `zero-value-as-valid-answer.md` | 2026-10-23 | bgp-rib name selector matches no session | F1 |
| `zero-value-as-valid-answer.md` | 2026-10-23 | KEEPALIVE metadata AS0 for a dynamic peer | F3 |
| `zero-value-as-valid-answer.md` | 2026-10-23 | message before negotiation carries AS 0 for a dynamic peer, never "not yet known" | F3 |
| `zero-value-as-valid-answer.md` | 2026-10-23 | web interface counters partial reads any `GetStats` error as no counters | F1 |
| `guard-addition-drops-what-it-refuses.md` | 2026-10-03 | `buildWithdrawalPayload` returns nothing for two families | F2 |
| `guard-addition-drops-what-it-refuses.md` | 2026-10-08 | `prefixListFromIRR` cut and counted success | F2 |
| `constant-reported-as-measured-state.md` | 2026-10-08 | LG `transformRoutes` `primary` from an absent key | F4 |
| `constant-reported-as-measured-state.md` | 2026-10-03 | BMP Loc-RIB ORIGIN literal IGP | F4 |
| `memo-suppresses-a-change-it-cannot-see.md` | 2026-10-23 | bgp-rib `sameBest` ignores an attribute-only change of the winner | F4 |
| `green-that-could-not-have-been-red.md` | 2026-08-31 | trafficusage `ebpfSupported` constant nil | F4 |
| `green-that-could-not-have-been-red.md` | 2026-08-31 | l2tp `checkHealth` cannot fail | F4 |
| `silent-fall-through.md` | 2026-10-20 | `getConfigPathPlugins` starts plugins after `ErrMissingDependency` | F1 |
| `silent-fall-through.md` | 2026-10-20 | MRT table dump and `show system` NTP block skipped when not built | F1 |
| `silent-fall-through.md` | 2026-10-20 | ddos responder, exabgp bridge, flowexport enrichment declare no BGP dependency | F1 |
| `silent-fall-through.md` | 2026-10-20 | redistribute `destination`, ls_export `source` not checked against the registry | F1 |
| `silent-fall-through.md` | 2026-10-23 | fib-vpp `newBackend` falls back to the `mockBackend` test double when VPP is unreachable | F1 |
| `zero-value-as-valid-answer.md` | 2026-10-23 | VPP `readVPPIfaceStats` any stats read error read as no counters | F4 |
| `zero-value-as-valid-answer.md` | 2026-10-23 | `answering.answered` read failure answered as not answered | F1 |
| `zero-value-as-valid-answer.md` | 2026-10-23 | rs `extractWireNLRIRecords` unknown encoding context answered as no wire UPDATE | F1 |
| `constant-reported-as-measured-state.md` | 2026-10-23 | `checkVPPHealth` healthy when the VPP socket is absent | F4 |
| `zero-value-as-valid-answer.md` | 2026-10-24 | rib `showPipeline`, `bestPipeline` parse error answered as an `{"error":...}` payload with status done | F1 |
| `guard-enumerates-instead-of-subtracting.md` | 2026-10-24 | `commandRegistrars` does not classify `RegisterTextRenderer` / `MustRegisterTextRenderer`, added by this spec's 238bdb62e9; `TestEveryCommandRegistrarIsMatched` red | - |

54 rows.

## Rows found already fixed

| Class file | Date | Surface (short) | Evidence (2026-10-23) |
|------------|------|-----------------|-----------------------|
| `silent-fall-through.md` | 2026-09-14 | `xfrmAEADName` default arm | `xfrmAEADName` (`ike/dataplane/xfrm_linux.go`) now returns `ErrNotSupported` for an unmapped name; the 128-bit ICV is documented as true of every mapped entry |
| `zero-value-as-valid-answer.md` | 2026-09-01 | mvpn `hasRD` | `hasRD` is gone; `MVPN.rdPresent` (`nlri/mvpn/types.go`) records presence at parse |
| `zero-value-as-valid-answer.md` | 2026-10-14 | `buildVPNNLRIBytes` path ID | `update_build_vpn.go` tests `ub.AddPath` alone; the row's own Fix cell says fixed |
| `constant-reported-as-measured-state.md` | 2026-08-22 | rpki `validation-enabled` and `running` literals | `rpki.go` writes `rp.active.Load()` and `rp.validationEnabled()` |
| `silent-fall-through.md` | 2026-09-21 | LDP `DecodeInit` skips unknown U=0 TLV | `DecodeInit` (`ldp/wire.go`) returns `unknownTLVError` and `(*Session).ignoreUnknownTLV` sends `statusUnknownTLV` (verified at HEAD 5762f6f2be); this spec added `TestLDPInitUnknownTLVUClearNotifies` (RFC5036-3.3-1 negative) and its record |
| `zero-value-as-valid-answer.md` | 2026-10-05 | IS-IS invalid three-way state becomes the zero action | `ReceiveHello` (`isis/adjacency/fsm.go`) rejects before mutation since af69577952, tagged `TestRFC5303InvalidStateLeavesAdjacencyUntouched` (RFC5303-3.2-7); this spec added the drop count in `handleIIH` (`Transport.CountDrop`) and `TestISISHelloInvalidThreeWayStateDiscarded` |
| `zero-value-as-valid-answer.md` | 2026-09-21 | rsvpte `handlePath` admits PATH without SENDER_TSPEC | d07c29546f: `handlePath` (`rsvpte/engine.go`) refuses `!msg.HasSenderTSpec`; `TestRFC2205PathWithoutSenderTSpecRefused`, `TestRFC2205PathWithoutSenderTSpecDropped` |
| `constant-reported-as-measured-state.md` | 2026-09-21 | rsvpte `rroProtectionFlags` from config | callers gate `rroProtectionFlags` on `engine.bypassEstablished` (`rsvpte/frr.go`, `engine.go`, `reservation.go`); `TestRFC4090FlagsClearUntilBypassEstablished`, `TestRFC4090RefreshTracksBypassState` (bypass set DOWN) |

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

### Goal Gates (MUST pass)
- [ ] AC-1..AC-13 all demonstrated
- [ ] Wiring Test table complete
- [ ] Targeted tests pass; full gate once over the closing commits
- [ ] Every A-N confirmed or broken
- [ ] Every item this spec did not do is a spec of its own, named here

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean
- [ ] **Commit A:** code + tests + docs + edited spec + journal rows re-pointed
- [ ] **Commit B:** `remove plan/release/fixes/spec-refuse-instead-of-defaulting.md` only
