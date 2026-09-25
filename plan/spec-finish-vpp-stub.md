# Spec: finish-vpp-stub

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-09-19 |

## Post-Compaction Recovery

Read this spec, `docs/functional-tests.md`,
`docs/architecture/traffic/followup-vpp-traffic.md`, and the native stub and
fixture producers named below. The July inventory is retained as a coverage
obligation, not a current handler count.

## Task

Extend the native VPP binary-API emulator at
`internal/test/cli/cmd_vpp_stub.go` and its compiled drivers at
`internal/test/fixture/misc_fixture_vpp.go`, with the `test/vpp/*.ci` coverage
needed for the full VPP request surface. The Python helper was retired on
2026-08-28; `le-test vpp-stub` is the current emulator. No Python helper is to be
restored.

The approved 2026-07-10 goal was complete, correct VPP support. This spec keeps
its full request-coverage, stateful dump, event, telemetry and fault-injection
obligations. Stub tests prove wiring and regressions; real-VPP deployment
evidence remains the semantic correctness gate. Neither replaces the other.

The original work includes interface lifecycle, all backend request handlers,
route add/withdraw/dump, process restart, tunnel and WireGuard state, SPAN/LCP,
traffic and firewall Apply, event subscription, stats-segment emulation, a
source-to-handler parity gate and runtime strict mode. It also owns the
silent-reply proof transferred by
`plan/immediate/spec-traffic-vpp-deferred-reply-timeout.md`: accept a traffic
request, withhold the reply, observe a bounded identifiable timeout, then prove
that another apply can proceed (AC-16).

The ready Python recipe has returned to design because its implementation
substrate was retired. Refresh the native request/handler difference before
implementation; preserve every original AC and account for every historical
inventory row. Existing native handlers reduce implementation work, not proof
obligations.

## Required Reading

### Architecture Docs
- [ ] `docs/functional-tests.md` - native VPP suite and compiled fixture execution
- [ ] `docs/architecture/traffic/followup-vpp-traffic.md` - apply-tier correctness remains real-VPP evidence
- [ ] `docs/architecture/traffic/fw-7b-backend-hardening.md` - traffic reply-timeout contract
- [ ] `docs/architecture/core-design.md` - backend and session ownership

### Source Contracts
- [ ] `internal/test/cli/cmd_vpp_stub.go` - `cmdVPPStub`, `newVPPStubState`, `serveVPPClient`, `handle`
- [ ] `internal/test/fixture/misc_fixture_vpp.go` - registered drivers, `startVPPStub` and request-log assertions
- [ ] `internal/test/cli/cmd_vpp.go` and `internal/le/functional/suites.go` - suite entry points
- [ ] `vendor/go.fd.io/govpp/adapter/statsclient/` - seqpacket, fd-passing and versioned shared-memory contract
- [ ] `internal/component/vpp/` and the iface/fib/static/traffic/firewall VPP backends - current request constructors and consumers

## Current Behavior (MANDATORY)

Source read on 2026-09-19: `startVPPStub` launches `le-test vpp-stub` with a
socket, JSONL log and deadline. `newVPPStubState` derives negotiated message
names and CRCs from the registered govpp API and assigns sorted IDs.
`handle` explicitly handles session setup, route add, MPLS, route lookup,
classify table, loopback creation and address add/delete requests.
`serveVPPClient` logs unhandled requests and emits a generic retval reply when
that reply name exists. Dump requests without such a reply finish without
details when the client sends its control ping.

The July count of 49 missing handlers is therefore not current: loopback and
address handlers now exist. The native command has socket/log/deadline/verbose
flags, but no strict, injection or stats-socket flags. This reading does not
claim that any scenario passed, and a current full request inventory is still
owed at design.

**Behaviour to preserve:** existing native fixtures, request-log fields,
message negotiation, and generic fallback for genuinely unknown traffic. Empty
initial tables remain valid. New strict runs must expose fallback use rather
than silently accepting it.

**Behaviour to change:** implement the remaining behaviours in AC-1 through
AC-16 on the native substrate, with no backend semantics weakened for a stub.

### Historical message inventory (2026-07-10)

Reply-shape legend: `retval` = reply carries only Retval i32 (the generic fallback produced a wire-correct reply on 2026-07-10, but logs `unhandled: true` and decodes no fields); `retval+X` = reply carries extra fields (fallback reply is undecodable -- request failed in the 2026-07-10 stub); `dump` = stream request answered by `*_details` messages (fallback sends nothing -- stream silently ends EMPTY).

Session layer (handled):

| Message | Constructed at | Reply shape | Stub on 2026-07-10 |
|---------|----------------|-------------|------------|
| sockclnt_create | govpp socketclient (vendored) | table reply | handled :285 |
| sockclnt_delete | govpp socketclient | retval | handled :292 |
| control_ping | govpp core (keepalive + stream terminator) | retval+pid | handled :300 |

FIB / routes (fib/vpp = `internal/plugins/fib/vpp`, static/vpp = `internal/plugins/static/vpp`, iface = `internal/plugins/iface/vpp`):

| Message | Constructed at | Reply shape | Stub on 2026-07-10 |
|---------|----------------|-------------|------------|
| ip_route_add_del | fib/vpp/backend.go,141,183; mpls.go,100; static/vpp/backend.go | retval+stats_index | handled :316 |
| ip_route_lookup_v2 | iface/fib.go | retval+route | handled :446 |
| mpls_route_add_del | fib/vpp/mpls.go,156 | retval+stats_index | handled :381 |
| sw_interface_set_mpls_enable | fib/vpp/mpls.go | retval | handled :427 |
| sr_steering_add_del | fib/vpp/srv6.go,95 | retval | MISSING |
| ip_route_v2_dump | iface/fib.go (ListKernelRoutes) | dump | MISSING |

Interface core (iface/vpp):

| Message | Constructed at | Reply shape | Stub on 2026-07-10 |
|---------|----------------|-------------|------------|
| create_loopback | ifacevpp.go (CreateDummy) | retval+sw_if_index | MISSING |
| delete_loopback | ifacevpp.go | retval | MISSING |
| create_vlan_subif | ifacevpp.go (CreateVLAN) | retval+sw_if_index | MISSING |
| delete_subif | ifacevpp.go | retval | MISSING |
| sw_interface_add_del_address | ifacevpp.go,536 | retval | MISSING |
| sw_interface_set_flags | ifacevpp.go,603 (SetAdminUp/Down) | retval | MISSING |
| sw_interface_set_mtu | ifacevpp.go | retval | MISSING |
| sw_interface_clear_stats | ifacevpp.go | retval | MISSING |
| sw_interface_set_mac_address | query.go | retval | MISSING |
| sw_interface_dump | query.go,57; traffic ops_linux.go; firewall backend_linux.go | dump (sw_interface_details) | MISSING |
| want_interface_events | monitor.go,134 | retval | MISSING |
| bridge_domain_add_del_v2 | ifacevpp.go (CreateBridge) | retval+bd_id | MISSING |
| sw_interface_set_l2_bridge | ifacevpp.go,710 (BridgeAdd/DelPort) | retval | MISSING |
| qos_egress_map_update | ifacevpp.go,418 | retval | MISSING |
| qos_mark_enable_disable | ifacevpp.go,428 | retval | MISSING |
| qos_record_enable_disable | ifacevpp.go | retval | MISSING |
| ip_neighbor_dump | neighbor.go (ListNeighbors) | dump | MISSING |

Interface tunnels / wireguard / mirror / LCP (iface/vpp, added by the wave):

| Message | Constructed at | Reply shape | Stub on 2026-07-10 |
|---------|----------------|-------------|------------|
| gre_tunnel_add_del | tunnel.go | retval+sw_if_index | MISSING |
| ipip_add_tunnel | tunnel.go | retval+sw_if_index | MISSING |
| ipip_del_tunnel | tunnel.go | retval | MISSING |
| vxlan_add_del_tunnel_v3 | vxlan.go | retval+sw_if_index | MISSING |
| wireguard_interface_create | wireguard.go | retval+sw_if_index | MISSING |
| wireguard_interface_delete | wireguard.go | retval | MISSING |
| wireguard_peer_add | wireguard.go | retval+peer_index | MISSING |
| wireguard_peer_remove | wireguard.go | retval | MISSING |
| wireguard_interface_dump | wireguard.go | dump | MISSING |
| wireguard_peers_dump | wireguard.go | dump | MISSING |
| sw_interface_span_enable_disable | mirror.go | retval | MISSING |
| lcp_itf_pair_add_del | lcp.go | retval | MISSING |

Traffic (traffic/vpp = `internal/plugins/traffic/vpp/ops_linux.go`):

| Message | Constructed at | Reply shape | Stub on 2026-07-10 |
|---------|----------------|-------------|------------|
| policer_add_del | ops_linux.go (also firewall :729) | retval+policer_index | MISSING |
| policer_del | ops_linux.go | retval | MISSING |
| policer_dump | ops_linux.go | dump | MISSING |
| policer_output | ops_linux.go | retval | MISSING |
| classify_add_del_table | ops_linux.go (also firewall :637) | retval+new_table_index | handled :517 |
| classify_add_del_session | ops_linux.go (also firewall :658) | retval | MISSING |
| policer_classify_set_interface | ops_linux.go (also firewall :701) | retval | MISSING |

Firewall (firewall/vpp = `internal/plugins/firewall/vpp/backend_linux.go`; NOT in skeleton scope, discovered at design):

| Message | Constructed at | Reply shape | Stub on 2026-07-10 |
|---------|----------------|-------------|------------|
| acl_add_replace | backend_linux.go | acl_index+retval | MISSING |
| acl_del | backend_linux.go | retval | MISSING |
| acl_dump | backend_linux.go | dump | MISSING |
| acl_interface_list_dump | backend_linux.go | dump | MISSING |
| acl_interface_set_acl_list | backend_linux.go | retval | MISSING |
| classify_set_interface_ip_table | backend_linux.go | retval | MISSING |
| nat44_ed_plugin_enable_disable | backend_linux.go | retval | MISSING |
| nat44_add_del_address_range | backend_linux.go | retval | MISSING |
| nat44_add_del_static_mapping_v2 | backend_linux.go | retval | MISSING |
| nat44_ed_add_del_output_interface | backend_linux.go | retval | MISSING |
| nat44_interface_add_del_feature | backend_linux.go | retval | MISSING |
| nat44_static_mapping_dump | backend_linux.go | dump | MISSING |

**Totals: 57 unique request messages; 8 handled, 49 missing.** All reply shapes above verified against the vendored binapi structs 2026-07-10 (e.g. CreateLoopbackReply `vendor/go.fd.io/govpp/binapi/interface/interface.ba.go`, PolicerAddDelReply `policer.ba.go`, ACLAddReplaceReply `acl.ba.go`, LcpItfPairAddDelReply retval-only `lcp.ba.go`; the 25 remaining "retval" replies batch-verified retval-only).

Beyond requests, two non-request surfaces are unemulated:

- **sw_interface_event** (server push, consumed at `monitor.go` via SubscribeNotification; enabled by want_interface_events) -- the stub never emits it, so the VPP-to-EventBus monitor path had no CI coverage in that inventory.
- **Stats segment** (seqpacket socket + SCM_RIGHTS fd + mmap, `statsclient.go`) -- a different protocol from the binary API; consumed by the telemetry poller (`telemetry.go`). No emulation; `vpp.go` degrades to a warning.

The historical inventory remains the minimum coverage account. At design,
classify each row against the current request producer and native handler,
include newly sent requests, and record any retired producer with evidence.
A vanished file name alone cannot discharge its behaviour.

## Data Flow (MANDATORY)

### Entry Point
`./le test functional vpp` runs `test/vpp/*.ci`; compiled fixtures start Ze, peers
where needed, and `le-test vpp-stub`. A parity check compares production request
construction against explicit emulator handling.

### Transformation Path
1. Ze sends govpp requests over the emulator socket.
2. Native handlers decode the fields the scenarios observe and update emulator state.
3. Dumps return that state; event subscriptions receive correctly framed events.
4. Fixtures assert both Ze's observable result and the request log.
5. Stats use a separate seqpacket socket and an fd-backed segment readable by govpp.
6. Fault modes return a configured retval or withhold a chosen reply; tests observe the backend result and continued availability.

### Boundaries Crossed
| Boundary | Contract | Proof owed |
|----------|----------|------------|
| Ze to native stub | negotiated govpp framing | all request handlers and strict mode |
| Native stub to Ze | dump/event frames and shared-memory stats | state readback, event and telemetry ACs |
| Source to parity gate | request type to wire name to explicit handler | missing-handler and non-request discrimination |

### Integration Points
The native CLI stub, compiled VPP fixtures, existing `test/vpp` discovery,
`internal/test/peer` for explicit withdraw, and the owning native repository
check for parity. Choose the parity implementation within the existing checker
architecture; the old proposal to create a second `repository.go` program is
superseded.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | Validation | Status |
|----|------------|-------|------------|--------|
| A-1 | The complete request set can be compared with explicit native handlers | govpp registered message names and production constructors | refresh each historical row and derive the current difference | unvalidated |
| A-2 | Restart reconnects to a fresh stub process within the current retry window | historical govpp reconnect design | read current connector and prove AC-4 | unvalidated |
| A-3 | Stats emulation can satisfy the vendored client | statsclient is the consuming parser | exercise interface, node and system counters through the metrics endpoint | unvalidated |
| A-4 | The native fault mode can isolate a missing reply from connection failure | AC-16 requires accepted-but-unanswered traffic | prove the request was accepted and the connection stayed up until timeout | unvalidated |

### Risks
| ID | Risk | Mitigation |
|----|------|------------|
| R-1 | Stub state differs from VPP semantics | retain real-VPP deployment evidence as the correctness gate |
| R-2 | Timing-dependent fixtures fail under load | bounded observation of state/log facts; no widened sleeps or weakened assertions |
| R-3 | Parity counts replies or misses aliased request constructors | map govpp RequestMessage types to wire names and test both missing and non-request cases |
| R-4 | A restart test accidentally claims monitor re-arming | AC-4 proves FIB reprogramming; separately inspect the current event re-subscription path before claiming it |
| R-5 | Config-unreachable requests disappear from coverage | retain parity coverage for them and document the reachability reason |

## Wiring Test

Each functional AC below must have a native driver and a discovered `.ci`.
The parity gate needs a fixture that is red for a missing request handler.
AC-16 needs a driver that receives no reply while the socket remains connected,
and then observes another apply after the timeout.

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `./le repo check` | Scans non-test `internal/` Go for binapi RequestMessage constructions, maps type -> wire name via vendored binapi, asserts each has an explicit native stub handler; RED while any request in the refreshed inventory is missing, GREEN at completion; wired into the `./le verify current mode full` check family; has `--selftest` + Go test like the sibling checks |
| AC-2 | ze applies `interface { backend vpp; loopback { unit ... address ... } }` against the stub (008) | Stub interface table allocates sw_if_index; create_loopback / sw_interface_set_flags / sw_interface_add_del_address / sw_interface_set_mtu handled with decoded fields in JSONL; `sw_interface_dump` streams the live table; 006 stays green (empty table at boot) |
| AC-3 | peer announces then withdraws 10.20.0.0/24 (003) | JSONL shows ip_route_add_del is_add=true then is_add=false for the prefix; driver exit 0; uses an explicit peer withdraw directive; reuse the current producer if already available |
| AC-4 | stub SIGKILLed and a new instance bound on the same socket within the reconnect window (004) | govpp reconnects (A-2); a route announced after restart appears in the NEW instance's JSONL; ze does not crash or exit |
| AC-5 | routes installed via ip_route_add_del, then `show route` dispatched (014) | Stub route table records add/del with state across calls; ip_route_v2_dump streams exactly the live routes per family; ze output contains the installed prefix |
| AC-6 | `--inject ip_route_add_del:retval=-1:after=N` (015) | First N adds succeed, N+1th returns retval=-1; fib-vpp logs the programming failure and does not count the route installed; ze stays up; flag syntax generalizes to any handled message |
| AC-7 | gre + ipip + vxlan tunnel config under backend vpp (009) | Each add handler returns a DISTINCT sw_if_index; JSONL fields include decoded src/dst (and vni for vxlan); delete handlers (ipip_del_tunnel, gre/vxlan is_add=false) handled |
| AC-8 | wireguard interface + one peer under backend vpp with `vpp.plugins.wireguard true` (010) | wireguard_interface_create logged (port, key presence -- never the key bytes), wireguard_peer_add returns peer_index and logs endpoint/allowed-ips; wireguard_interface_dump / wireguard_peers_dump stream recorded state |
| AC-9 | mirror config + lcp.enabled loopback (011) | sw_interface_span_enable_disable logged with src/dst sw_if_index + state; lcp_itf_pair_add_del logged with host if-name and netns; both retval=0 from real handlers |
| AC-10 | events armed, then admin-state change (013) | After want_interface_events, the stub emits sw_interface_event on each sw_interface_set_flags; ze's monitor translates it (observable via the ifacevpp event log line / EventBus-driven log); no event emitted before arming |
| AC-11 | traffic config with class filter reaching Apply against the stub (016) | Apply COMPLETES (no "vpp not connected"): sw_interface_dump resolves the seeded interface, policer_add_del returns policer_index, classify_add_del_session's HitNextIndex equals that index in JSONL, policer_classify_set_interface binds it; reconcile path (config removal) deletes what it created |
| AC-12 | firewall rules + nat44 under backend vpp reaching Apply (017) | acl_add_replace returns acl_index; acl_interface_set_acl_list, nat44_ed_plugin_enable_disable, nat44_add_del_static_mapping_v2 handled + logged; Apply completes against the stub |
| AC-13 | ze boots with metrics enabled + stub stats socket (012) | Stub serves seqpacket socket + memfd stat segment readable by govpp statsclient; `ze_vpp_stats_up` is 1 and `ze_vpp_interface_rx_packets` (seeded counters) present on the metrics endpoint |
| AC-14 | any new-suite stub run | Drivers pass `--strict`: stub exits nonzero at shutdown if any request hit the unhandled fallback; JSONL `unhandled:true` also asserted absent by drivers |
| AC-15 | remaining reachable iface messages (mtu/mac/clear-stats/neighbor/vlan+qos/sr_steering) | Real handlers (not fallback) log decoded fields; covered inside 008 (mtu/addresses/flags), 014 (neighbor dump alongside route dump), and the parity gate for the config-unreachable rest (bridge/l2 -- see Known Limitations) |
| AC-16 | The native stub accepts a traffic request and deliberately withholds its reply | Traffic Apply returns an identifiable reply-timeout within the configured bound; a subsequent apply can acquire the backend lock and proceed. Prove the bounded result fails when timeout binding is removed; retain AC-11 success-path coverage |

## Test Plan

The original proposed scenarios remain: iface-loopback, fib-withdraw,
vpp-restart, route-dump, fault-injection, iface-tunnel, iface-wireguard,
iface-mirror-lcp, iface-monitor-event, traffic-apply, firewall-acl-nat and
telemetry. Add the inherited traffic-reply-timeout proof for AC-16. Their
historical numeric labels in the AC table are design identifiers, not a claim
that files exist. Name the current files and drivers at the design gate.

Keep the existing five native fixtures. The peer withdraw must be an explicit
UPDATE withdrawal, not session-down purge. Exercise injection after zero and N
successful requests, reject a negative count, and preserve i32 retval bounds.
Stats must reach `GetInterfaceStats`, `GetNodeStats` and `GetSystemStats`.
WireGuard logs record key presence only, never secret bytes. Restart uses a
distinct log per process instance so evidence survives the old process.

### Security Review Checklist

| Check | Required evidence |
|-------|-------------------|
| Input validation | Validate frame lengths and field bounds before decoding. Malformed frames produce a reported error, not a panic |
| Resource exhaustion | Bound state growth and process lifetime, including deadline termination; no request can cause unbounded state growth |
| Socket permissions | Both API and stats sockets use mode 0600. Prove the new stats transport preserves this protection |

## Files to Modify

- `internal/test/cli/cmd_vpp_stub.go` and cohesive native stub files if needed
- `internal/test/fixture/misc_fixture_vpp.go` and the current VPP `.ci` drivers
- `internal/test/peer` only if explicit withdraw support is still missing
- The existing native repository checker and its action registration for parity
- `docs/functional-tests.md` for the native runbook, flags and evidence boundary

## Files to Create

The scenario files and any focused native source/test files selected at design.
No `test/scripts` file is a target.

## Key Design Decisions

Stateful tables and dumps, strict-mode fallback detection, a parity gate over
the full request population, organically triggered interface events and the
separate stats protocol remain required. Use registered govpp types and the
current native fixture conventions rather than Python parsing or embedded
Python drivers. The stats transport must work on its supported Linux runtime;
Python version and `socket.send_fds` assumptions no longer apply.

Keep the stub suite's existing gate membership while adding the parity gate to
normal verification. Promoting the whole runtime suite requires its own
stability evidence. Real-VPP Docker evidence remains required for semantics.

## Known Limitations

The emulator has no forwarding dataplane and cannot prove VPP semantics or
index-exhaustion behaviour. Config-rejected backend kinds still owe parity
coverage. No present-day handler count or runtime pass is asserted here.

## Implementation Steps

1. Complete the native inventory, exact test mapping and parity design. Account for every historical request and AC before returning this spec to ready.
2. Add parity discrimination, stateful interface handling and strict mode.
3. Complete FIB state, explicit withdrawal, faults and fresh-process restart.
4. Complete tunnel, WireGuard, SPAN/LCP and interface-event coverage.
5. Complete traffic/firewall Apply and accepted-but-unanswered timeout proof.
6. Complete stats emulation, all functional evidence, real-VPP evidence and documentation.
7. Complete independent review and normal closure only after the full obligation set is proven.

## Checklist

- [ ] Every historical inventory row is reconciled with current producers
- [ ] AC-1 through AC-16 have named tests and passing evidence
- [ ] Parity fails for a missing handler and passes with none missing
- [ ] Existing fixtures and real-VPP evidence remain valid
- [ ] `./le verify worktree` passes
- [ ] Independent review and closure requirements are complete
