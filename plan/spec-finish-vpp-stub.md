# Spec: finish-vpp-stub

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | - |
| Phase | - |
| Updated | 2026-07-10 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `git log -p plan/deferrals.md` (pre-2026-07-06) - original deferral rows + evidence
4. `docs/architecture/traffic/followup-vpp-traffic.md` - the wave's VPP decisions and gotchas
5. The "Message inventory" table in Current Behavior below - it IS the scope; do not re-derive it

## Task

Extend `test/scripts/vpp_stub.py` (Python VPP binary-API emulator) and add the `test/vpp/*.ci` tests that depend on the richer stub. Unblocks AC-2..16 iface/telemetry coverage that is currently unit-only.

This is a consolidation skeleton created from verified deferral survivors (backlog triage 2026-07-06). Each item below was confirmed still-open against the codebase with a producing `file:line`. Split into phases when picked up; the sections after Task are lightweight scaffolding to be filled at design time.

**USER GOAL (2026-07-10, recorded verbatim):** "our goal is to implement things correctly and fully for VPP." This spec serves complete, correct VPP support. The stub exists so CI can exercise ze's VPP paths without hardware; real-VPP Docker evidence (`scripts/evidence/effective-vpp.py`, `scripts/evidence/effective-vpp-iface.py`, `mk/test-integration.mk` `ze-deployment-vpp-*` targets) proves correctness on real VPP. The 2026-07 followup wave added a large VPP surface (SPAN mirror, wireguard, LCP, gre/gretap/ipip/vxlan tunnels, traffic classify/policer) and vendored the govpp binapi, so the stub gap grew: design scope (2026-07-10) is the FULL request surface ze sends, not only the iface subset the skeleton listed.

### Work items (migrated from the 2026-07-06 deferral triage; `L#` = row in the pre-triage `plan/deferrals.md`)

- **Iface binary-API handlers (L192)** - stub lacks create_loopback / sw_interface_set_flags / sw_interface_add_del_address / sw_interface_dump / sw_interface_event; AC-2..16 stay unit-only until added.
- **Stats-segment emulation (L178,L181)** - no shared-memory stat-counter emulation (L178); blocks `test/vpp/012-telemetry.ci` (L181).
- **Fault injection (L179)** - `--inject ip_route_add_del:retval=-1:after=3` style error-path testing.
- **Route dump (L180)** - `ip_route_dump` streaming handler returning installed routes (state across calls).
- **fib-withdraw + restart `.ci` (L175)** - `003-fib-withdraw.ci` + `004-vpp-restart.ci` still absent (coexist already landed renamed).

### Additional work items (design 2026-07-10, from firsthand re-inventory)

- **Full-surface handlers** - the wave grew the sent-message set to 57 unique requests across five backends (iface, fib, static, traffic, firewall) plus the vpp component session layer; 49 have no handler. See the Message inventory table.
- **Parity gate** - a static check asserting every binapi request message constructed by ze has a stub handler, so the stub can never silently rot again (this wave is the existence proof of the rot).
- **`ze-test peer` withdraw directive** - `003-fib-withdraw.ci` needs the peer to send an explicit RFC 4271 withdraw; the peer today only has `send-route` (`internal/test/peer/message.go`).
- **Strict mode** - stub flag so a `.ci` run fails when ze sent a message the stub did not explicitly handle.
- **Event emission** - `sw_interface_event` push after `want_interface_events`, so the ifacevpp monitor path (`internal/plugins/iface/vpp/monitor.go`) is CI-testable.

## Post-Wave Corrections (2026-07-10)

Corrections against the 2026-07 followup wave (see `tmp/review-followup/context.md`, `tmp/review-followup/result-batch-00.md`), all re-verified firsthand at design time:

| # | Skeleton said | Reality after the wave | Evidence |
|---|--------------|------------------------|----------|
| 1 | HANDLERS at `test/scripts/vpp_stub.py` | HANDLERS dict now at `test/scripts/vpp_stub.py` (the wave inserted `handle_classify_add_del_table` at :517-546) | read 2026-07-10 |
| 2 | Scope = iface handlers + stats + inject + route dump + 2 `.ci` | Scope EXPANDED: wave added span/wireguard/lcp/gre/gretap/ipip/vxlan/tunnel_types binapi surface and traffic classify/policer messages; vendored govpp binapi packages; 49 of 57 sent request messages unhandled | Message inventory below |
| 3 | (not stated) | The wave chose real-VPP Docker evidence (`scripts/evidence/effective-vpp-iface.py`, `ze-deployment-vpp-iface-test`) instead of stub coverage for its new surface; the stub gap is this spec's to close | the followup-vpp-iface record, Consequences; `mk/test-integration.mk` |
| 4 | `test/vpp` has 001,002,005,006,007 | Confirmed: exactly `001-boot.ci`, `002-fib-route.ci`, `005-mpls-push.ci`, `006-iface-create.ci`, `007-fib-route-lookup.ci`; 003/004/008+ absent | `ls test/vpp/` 2026-07-10 |
| 5 | (not stated) | The followup-vpp-traffic-protocol record explicitly routed work here: "spec-finish-vpp-stub.md must add sw_interface_dump + policer_add_del handlers before any apply-tier .ci traffic test can run against the stub (A-6 is broken -- only classify_add_del_table was added)" | that record's Consequences |
| 6 | (not stated) | `test/traffic/020-vpp-accept-dscp-filter.ci` and `026-vpp-accept-multiclass.ci` name this spec as the blocker for apply-tier traffic `.ci` | read 2026-07-10 |

## Required Reading

### Source files / docs

- [ ] ~~`test/scripts/vpp_stub.py` (HANDLERS)~~ `test/scripts/vpp_stub.py` (HANDLERS) -- line drifted, see Post-Wave Corrections #1
  → Constraint: verify current behaviour against this source before designing. (Done 2026-07-10; full file read, see Current Behavior.)
  → Constraint: dispatch is by negotiated message name; unhandled requests hit the generic fallback at :609-617 (retval=0 i32 reply IF `<name>_reply` exists in the scraped table, logged with `"unhandled": true`; dump requests get NO reply so streams end empty at the client's control_ping).
  → Constraint: the stub truncates its JSONL log at startup (:152-154) -- a restarted stub instance on the same log path wipes prior evidence; `004-vpp-restart.ci` must use a distinct log per instance.
  → Constraint: message IDs are assigned deterministically (sorted names from `NEGOTIATED_ID_BASE`, :143-149), so IDs are stable across a stub restart -- reconnect-safe.
- [ ] `test/vpp/*.ci` (functional VPP tests)
  → Constraint: verify current behaviour against this source before designing. (Done 2026-07-10.)
  → Decision: all five use an embedded Python driver (tmpfs) that spawns vpp_stub + optionally ze-peer + ze and asserts on the stub's JSONL log; new tests follow the same driver pattern.
  → Constraint: `006-iface-create.ci` depends on the EMPTY dump behavior ("the dump is empty but the handshake succeeds") -- adding a `sw_interface_dump` handler must keep 006 green (empty table at boot is still a valid dump).
- [ ] `internal/plugins/iface/vpp/ifacevpp.go`, `internal/plugins/traffic/vpp/` (code the stub exercises)
  → Constraint: verify current behaviour against this source before designing. (Done 2026-07-10; message-by-message inventory below.)
- [ ] `docs/architecture/traffic/followup-vpp-traffic.md`
  → Decision: the wave's stance: stub-backed `.ci` = wiring proof, real-VPP evidence = correctness proof (1097: "Real-VPP evidence is the authoritative apply-tier validation (A-6: the stub cannot run a full traffic Apply)"). This spec keeps that split and closes the CI side.
  → Constraint: 1097 gotcha: the traffic `.ci` suite is timing/stderr-capture sensitive under load; do NOT raise sleep baselines; poll the stub JSONL with deadlines instead.
- [ ] `internal/test/cli/cmd_vpp.go`, `mk/test-functional.mk`, `mk/test-release.mk`
  → Constraint: `ze-test vpp --all` discovers `test/vpp/*.ci` (`cmd_vpp.go`); run by `make ze-functional-vpp-test`; the vpp suite is NOT in the gating `ze-functional-test` list (`mk/test-functional.mk` "platform deps or infra") and runs in `ze-evidence-functional-test` (release evidence).
- [ ] `vendor/go.fd.io/govpp/adapter/statsclient/statsclient.go`
  → Constraint: stats are NOT binary-API messages: the client dials a `unixpacket` (SOCK_SEQPACKET) socket, receives ONE fd via SCM_RIGHTS, fstats + mmaps it read-only, and reads a versioned (1 or 2) stat segment. Emulation = seqpacket listener + memfd with a v1/v2-conformant segment.
- [ ] `vendor/go.fd.io/govpp/core/connection.go`, `internal/component/vpp/conn.go`, `internal/component/vpp/vpp.go`
  → Constraint: govpp core auto-reconnects after a connection drop; ze passes maxAttempts=10, retryInterval=1s (`vpp.go` via `conn.go`), so `004-vpp-restart.ci` has a ~10s window to restart the stub on the same socket path.
  → Constraint: in external mode `runOnce` blocks on ctx (`vpp.go`); the restart loop and `EventReconnected` never fire, so post-restart re-arming (e.g. monitor re-subscribe) is NOT wired -- 004 scopes to fib re-programming only (see Known Limitations).

## Current Behavior (MANDATORY)

**Behavior to preserve (summary):** all existing stub handlers, CLI flags, and the JSONL log format; the 5 existing `test/vpp/*.ci` stay green; the generic unhandled fallback remains for unknown messages. Full "Behavior to preserve" block after the inventory tables below.

**Source files read:** (all read firsthand 2026-07-10)

- [ ] `test/scripts/vpp_stub.py` (724 lines, full read)
  -> Constraint: 8 handlers registered at :549-558: sockclnt_create, sockclnt_delete, control_ping, ip_route_add_del, ip_route_lookup_v2, mpls_route_add_del, sw_interface_set_mpls_enable, classify_add_del_table.
  -> Constraint: reply framing helper `build_reply` (:229-253) already supports RequestMessage (10-byte), ReplyMessage/EventMessage (6-byte) headers -- event push needs no new framing code.
  -> Constraint: message table is scraped from the vendored binapi (`scrape_binapi` :76-120), so handler names must match wire names exactly.
- [ ] `internal/plugins/iface/vpp/ifacevpp.go` (+ query.go, fib.go, neighbor.go, monitor.go, tunnel.go, vxlan.go, wireguard.go, mirror.go, lcp.go)
- [ ] `internal/plugins/traffic/vpp/ops_linux.go` (+ backend_linux.go, classify_linux.go, binapi_imports.go)
- [ ] `internal/plugins/fib/vpp/backend.go` (+ mpls.go, srv6.go)
- [ ] `internal/plugins/firewall/vpp/backend_linux.go` -- NOT in the skeleton's list; discovered at design: it sends 13 more unique messages (acl, nat44_ed, classify binding)
- [ ] `internal/plugins/static/vpp/backend.go` -- sends ip_route_add_del (:61), already handled
- [ ] `internal/component/vpp/conn.go`, `vpp.go`, `stats_conn.go`, `telemetry.go`
  -> Constraint: telemetry poller consumes `GetInterfaceStats` / `GetNodeStats` / `GetSystemStats` (`telemetry.go`) from a `core.StatsConnection` created by `connectStats` (`stats_conn.go`); stats connect failure only warns and disables telemetry (`vpp.go`), so today the stub environment always runs with `ze_vpp_stats_up` unset.
- [ ] `vendor/go.fd.io/govpp/codec/codec.go`
  -> Constraint: DecodeMsg recovers from panics (:48), so the fallback's 4-byte retval reply to a message whose reply carries extra fields (e.g. create_loopback_reply) surfaces as a request-level decode ERROR in ze, not a crash. Extra-field replies therefore REQUIRE real handlers.

### Message inventory (the scope; verified 2026-07-10, one row per unique request message ze constructs in non-test code)

Reply-shape legend: `retval` = reply carries only Retval i32 (the generic fallback happens to produce a wire-correct reply today, but logs `unhandled: true` and decodes no fields); `retval+X` = reply carries extra fields (fallback reply is undecodable -- request FAILS in ze today); `dump` = stream request answered by `*_details` messages (fallback sends nothing -- stream silently ends EMPTY).

Session layer (handled):

| Message | Constructed at | Reply shape | Stub today |
|---------|----------------|-------------|------------|
| sockclnt_create | govpp socketclient (vendored) | table reply | handled :285 |
| sockclnt_delete | govpp socketclient | retval | handled :292 |
| control_ping | govpp core (keepalive + stream terminator) | retval+pid | handled :300 |

FIB / routes (fib/vpp = `internal/plugins/fib/vpp`, static/vpp = `internal/plugins/static/vpp`, iface = `internal/plugins/iface/vpp`):

| Message | Constructed at | Reply shape | Stub today |
|---------|----------------|-------------|------------|
| ip_route_add_del | fib/vpp/backend.go,141,183; mpls.go,100; static/vpp/backend.go | retval+stats_index | handled :316 |
| ip_route_lookup_v2 | iface/fib.go | retval+route | handled :446 |
| mpls_route_add_del | fib/vpp/mpls.go,156 | retval+stats_index | handled :381 |
| sw_interface_set_mpls_enable | fib/vpp/mpls.go | retval | handled :427 |
| sr_steering_add_del | fib/vpp/srv6.go,95 | retval | MISSING |
| ip_route_v2_dump | iface/fib.go (ListKernelRoutes) | dump | MISSING |

Interface core (iface/vpp):

| Message | Constructed at | Reply shape | Stub today |
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

| Message | Constructed at | Reply shape | Stub today |
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

| Message | Constructed at | Reply shape | Stub today |
|---------|----------------|-------------|------------|
| policer_add_del | ops_linux.go (also firewall :729) | retval+policer_index | MISSING |
| policer_del | ops_linux.go | retval | MISSING |
| policer_dump | ops_linux.go | dump | MISSING |
| policer_output | ops_linux.go | retval | MISSING |
| classify_add_del_table | ops_linux.go (also firewall :637) | retval+new_table_index | handled :517 |
| classify_add_del_session | ops_linux.go (also firewall :658) | retval | MISSING |
| policer_classify_set_interface | ops_linux.go (also firewall :701) | retval | MISSING |

Firewall (firewall/vpp = `internal/plugins/firewall/vpp/backend_linux.go`; NOT in skeleton scope, discovered at design):

| Message | Constructed at | Reply shape | Stub today |
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

- **sw_interface_event** (server push, consumed at `monitor.go` via SubscribeNotification; enabled by want_interface_events) -- the stub never emits it, so the VPP-to-EventBus monitor path has zero CI coverage.
- **Stats segment** (seqpacket socket + SCM_RIGHTS fd + mmap, `statsclient.go`) -- a different protocol from the binary API; consumed by the telemetry poller (`telemetry.go`). No emulation; `vpp.go` degrades to a warning.

### .ci coverage today (verified 2026-07-10)

| Tier | Tests | What they prove | Gap |
|------|-------|-----------------|-----|
| test/vpp (stub-backed runtime) | 001-boot, 002-fib-route, 005-mpls-push, 006-iface-create, 007-fib-route-lookup | handshake; ip_route_add_del add path (plain + MPLS label); backend load with EMPTY dump; route lookup | 006 is load-only despite its name (comment :268-270: dump empty by design); no create/withdraw/restart/event/dump/fault/traffic/firewall/telemetry coverage |
| test/parse (offline) | iface-vpp-*.ci (11), fib-vpp-config-valid.ci, vpp-config-*.ci (8) | `ze config validate -` commit-gate accept/reject only; no daemon, no stub, no messages | parse-only by construction |
| test/traffic (runtime, NO stub) | 011-vpp-reject-hfsc, 012-vpp-not-connected, 020-vpp-accept-dscp-filter, 020-vpp-reject-dscp-filter, 024-vpp-reject-prio, 025-vpp-reject-mark, 026-vpp-accept-multiclass | verify-tier accept/reject; "accept" is proven by the 5s WaitConnected timeout logging "vpp not connected" (e.g. 020-accept:139-156) | apply tier never runs; the accept signal is the ABSENCE of VPP -- explicitly "blocked on A-6 stub work in plan/spec-finish-vpp-stub.md" (020:152-153) |
| test/firewall | none for vpp | -- | firewall/vpp has unit (fakeOps) + QEMU `go test -tags integration ./internal/plugins/firewall/vpp/...` (`mk/test-integration.mk`) but no functional `.ci` at all |
| real-VPP evidence (Docker, not CI) | `ze-deployment-vpp-test` -> `scripts/evidence/effective-vpp.py`; `ze-deployment-vpp-iface-test` -> `scripts/evidence/effective-vpp-iface.py` (`mk/test-integration.mk`) | semantic correctness on VPP 25.10 (policer/classify/dscp/multiclass; GRE + SPAN + wireguard) | requires Docker; not run per-commit |

### Stub wiring (how .ci runs reach the stub)

- The drivers embedded in each `test/vpp/*.ci` locate `test/scripts/vpp_stub.py` via the repo root and spawn it per-test with `--socket <tmp>/api.sock --log <tmp>/vpp-requests.jsonl --deadline N -v`.
- `ze-test vpp` (registered `internal/test/cli/register.go`, implemented `cmd_vpp.go`) discovers `test/vpp/*.ci`; `make ze-functional-vpp-test` = `bin/ze-test vpp --all` (`mk/test-functional.mk`).
- The vpp suite is non-gating: absent from the `ze-functional-test` suite list (`mk/test-functional.mk`), present in `ze-evidence-functional-test` (`mk/test-release.mk`).

**Behavior to preserve:**
- All existing behaviour of the listed files; this backlog work only adds the missing pieces named in the Task work items.
- The 5 existing test/vpp `.ci` stay green unchanged (006 depends on an empty-at-boot dump: a `sw_interface_dump` handler must return the stub's live table, which is empty when nothing was created/seeded).
- The generic unhandled fallback (:609-617) stays for genuinely unknown messages (e.g. future govpp-internal traffic); parity + strict mode make it loud, they do not remove it.
- Existing stub CLI flags and JSONL log format (drivers parse `msg`/`context`/`fields`).

**Behavior to change:**
- Only the specific gaps enumerated in the Task work items: add handlers/state/events/fault-injection/stats emulation to the stub, add the missing `test/vpp/*.ci`, add the parity gate, add the peer withdraw directive.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `test/vpp/*.ci` running `ze` against the Python `vpp_stub.py` binary-API emulator
- Statically: `make ze-vpp-stub-parity-check` scanning ze source + stub HANDLERS (no runtime)

### Transformation Path
1. `vpp_stub.py` gains a handler / stats-segment / fault-injection / dump capability
2. A `test/vpp/*.ci` drives `ze` against the enriched stub
3. The test asserts the VPP-facing behaviour end-to-end
4. (design 2026-07-10) Stateful flows: handler mutates stub state (interface table / route table / policer + classify registry / acl-nat registry / wireguard peers) -> later dump handlers stream that state back -> `ze` renders it (e.g. ListKernelRoutes) -> driver asserts on ze output AND on the JSONL log
5. (design 2026-07-10) Event flow: `want_interface_events` arms the connection -> a subsequent `sw_interface_set_flags` handler emits an `sw_interface_event` EventMessage frame -> govpp SubscribeNotification -> `monitor.go` translates to EventBus -> driver asserts the ze log line
6. (design 2026-07-10) Stats flow: stub serves a seqpacket socket, passes a memfd with a v1/v2 stat segment via SCM_RIGHTS -> `connectStats` (`stats_conn.go`) -> telemetry poller (`telemetry.go`) -> Prometheus metrics endpoint -> driver scrapes `ze_vpp_stats_up` + `ze_vpp_interface_*`

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| `ze` -> stub | GoVPP binary API over the stub socket | [ ] |
| stub -> test | stats segment / route dump responses | [ ] |
| stub -> `ze` (push) | sw_interface_event EventMessage frames (6-byte header, `build_reply` EventMessage branch :245) | [ ] |
| stub -> `ze` (shared memory) | SOCK_SEQPACKET + SCM_RIGHTS fd + mmap stat segment (`statsclient.go`) | [ ] |
| source tree -> parity gate | `scripts/checks/vpp_stub_parity.go` scans non-test Go for binapi request constructions and vpp_stub.py for HANDLERS keys | [ ] |

### Integration Points
- `test/scripts/vpp_stub.py`
- `internal/plugins/iface/vpp/`
- `internal/plugins/traffic/vpp/`
- (design 2026-07-10) `internal/plugins/fib/vpp/`, `internal/plugins/firewall/vpp/`, `internal/plugins/static/vpp/`, `internal/component/vpp/` (session, stats, telemetry)
- (design 2026-07-10) `internal/test/peer/` (send-withdraw directive), `internal/test/cli/cmd_vpp.go` (suite discovery, unchanged), `scripts/checks/` (parity gate), `mk/`/`Makefile` (gate target)

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Registration over hardcoding - new commands/views/families/handlers register and are core-discovered, not hardcoded into a core/shared package (`ai/rules/plugins.md`)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The verified `file:line` evidence in the Task items still holds at design time | 2026-07-06 backlog triage | Re-scope the item | grep/LSP at design time | confirmed (2026-07-10 full re-inventory; only drift: HANDLERS :517 -> :549, scope grew per Post-Wave Corrections) |
| A-2 | Extra-field fallback replies fail the request in ze without crashing it (decode error, not panic) | `vendor/go.fd.io/govpp/codec/codec.go` recover in DecodeMsg | Fallback would crash ze; handlers become even more urgent but tests must guard crashes | read of codec.go 2026-07-10 | confirmed |
| A-3 | Stub message IDs are stable across stub restart (same binapi -> same sorted assignment), so govpp reconnect works against a NEW stub process | `test/scripts/vpp_stub.py` deterministic assignment; govpp re-runs sockclnt_create on reconnect | 004-vpp-restart infeasible against a fresh process; would need in-process socket re-listen instead | read of stub + `connection.go` reconnect loop | confirmed (code-read; runtime-proven by 004 itself) |
| A-4 | The govpp reconnect window under ze's settings is ~10s (10 attempts x 1s), enough for a driver to restart the stub | `internal/component/vpp/vpp.go` Connect(ctx, 10, 1s); `conn.go` passes both to core.AsyncConnect | 004 flaky; driver must restart faster or the test is redesigned around a held socket | 004 driver logs reconnect timing; run under load | unvalidated |
| A-5 | With sw_interface_dump + policer/classify handlers, the traffic vpp Apply path completes against the stub (WaitConnected satisfied, name resolution finds the seeded interface) | 1096 Consequences names exactly these handlers as the A-6 blocker; Apply gates on `Connector.WaitConnected` + interface dump (`ops_linux.go`) | Apply needs more than listed; extend stub state until Apply completes; scope unchanged (parity list is closed) | 016-traffic-apply.ci green | unvalidated |
| A-6 | Python on CI supports SOCK_SEQPACKET + `socket.send_fds` (3.9+) for the stats emulation | statsclient requires seqpacket + SCM_RIGHTS (`statsclient.go`); repo CI uses modern python3 | Implement fd-passing via `sendmsg` ancillary data manually (works on any 3.x) | `python3 -c` probe in the 012 driver; implementation-time check | unvalidated |
| A-7 | The v1/v2 stat segment layout is implementable from the vendored statsclient sources alone (no VPP source needed) | `vendor/go.fd.io/govpp/adapter/statsclient/statseg_v1.go` / `statseg_v2.go` are the exact parser the emulation must satisfy | Ground-truth against real VPP via the Docker evidence env instead | 012-telemetry.ci green (GetInterfaceStats/GetSystemStats/GetNodeStats return the canned counters) | unvalidated |
| A-8 | Adding a `send-withdraw` update directive to ze-test peer is a modest extension of the existing send-route path | `internal/test/peer/message.go` RouteToSend + BuildRouteMsg pattern; `peer.go` SendRoutes plumbing | Fall back to session-down purge for 003 (peer disconnect withdraws routes) and file the directive as its own item | unit test on the new builder + 003 green | unvalidated |
| A-9 | An `sw_interface_event` emitted by the stub on admin-flag change reaches `monitor.go` via govpp SubscribeNotification with the 6-byte EventMessage framing the stub already produces | `build_reply` EventMessage branch (:245); monitor SubscribeNotification (`monitor.go`) | Adjust framing per govpp codec expectations for events; the reference is `codec.go` getOffset | 013-iface-monitor-event.ci green | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Scope drift when the umbrella is split into per-item specs | Item needs its own design doc | Split into a dedicated spec and re-point |
| R-2 | Daemon-based `.ci` timing sensitivity under load (1097 gotcha: WaitConnected + wait.py windows blow on a loaded box) | New tests pass solo, fail under `make ze-functional-vpp-test` with concurrent sessions | Poll JSONL with generous deadlines instead of raising sleeps (ci-sleep rule); keep the vpp suite non-gating; report pressure, never weaken assertions |
| R-3 | 004-vpp-restart flakiness: reconnect window vs stub restart latency | reconnect logs show attempts exhausted | Driver starts the replacement stub BEFORE the old socket's clients notice (unlink+bind is instant); assert via JSONL of the new instance; A-4 timing check |
| R-4 | Stub state model drifts from real VPP semantics (e.g. retval conventions, index reuse), giving false confidence | Stub-green test but real-VPP evidence red for the same flow | Recorded split: stub = wiring/regression proof only; real-VPP Docker evidence stays the correctness gate (Key Design Decisions); evidence scripts unchanged by this spec |
| R-5 | Parity gate false positives/negatives: message constructions in helpers, aliased imports, or reply/details/event structs miscounted as requests | Gate flags a non-request type, or misses a new message | Map Go type -> wire name via the vendored `GetMessageName`/`GetMessageType` (only RequestMessage types count); scan non-test files only; fixture-tested like `scripts/checks/iface_resolution.go`; `--selftest` mode |
| R-6 | Monitor re-arm after external-mode VPP restart is not wired (no EventReconnected in external mode, `vpp.go`), so a restart test asserting monitor behavior would fail for a reason outside this spec | 004 extended to events fails post-restart | 004 scopes to fib re-programming; monitor-after-restart recorded in Known Limitations as a candidate ze bug/follow-up, NOT silently absorbed into this spec |
| R-7 | Config commit-gates reject some kinds under vpp (bridge, veth), so their messages are stub-handled but not `.ci`-drivable from config | 008+ cannot produce bridge_domain_add_del_v2 | Parity covers the handler's existence; Known Limitations records which messages have no config-reachable driver today (bridge/l2 pair) |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `.ci` creates a VPP loopback via `ze` | → | stub `create_loopback` handler responds | `test/vpp/008-iface-loopback.ci` |
| `.ci` scrapes VPP stats | → | stub stats-segment emulation returns counters | `test/vpp/012-telemetry.ci` |
| peer withdraws a prefix | → | fib-vpp emits ip_route_add_del is_add=false; stub logs it | `test/vpp/003-fib-withdraw.ci` |
| stub killed + restarted mid-session | → | govpp reconnect; route sent post-restart logged by NEW stub instance | `test/vpp/004-vpp-restart.ci` |
| `.ci` configures gre/ipip/vxlan tunnels under backend vpp | → | tunnel handlers reply distinct sw_if_index; fields logged | `test/vpp/009-iface-tunnel.ci` |
| `.ci` configures a wireguard interface + peer | → | wireguard_interface_create / wireguard_peer_add handlers + dumps | `test/vpp/010-iface-wireguard.ci` |
| `.ci` configures mirror + lcp.enabled loopback | → | span + lcp_itf_pair_add_del handlers | `test/vpp/011-iface-mirror-lcp.ci` |
| admin-state change with events armed | → | stub pushes sw_interface_event; monitor.go -> EventBus | `test/vpp/013-iface-monitor-event.ci` |
| `show route` dispatch against installed routes | → | ip_route_v2_dump streams stub route state; ze renders | `test/vpp/014-route-dump.ci` |
| `--inject ip_route_add_del:retval=-1:after=N` | → | fib-vpp error path logs failure, no install | `test/vpp/015-fault-injection.ci` |
| traffic config (filter protocol/dscp, backend vpp) reaches Apply | → | policer_add_del + classify chain + policer_classify_set_interface against stub | `test/vpp/016-traffic-apply.ci` |
| firewall config (acl + nat44, backend vpp) reaches Apply | → | acl_add_replace/acl_interface_set_acl_list + nat44_* handlers | `test/vpp/017-firewall-acl-nat.ci` |
| any ze-sent binapi request without a stub handler | → | parity gate fails the build | `make ze-vpp-stub-parity-check` + `scripts/checks/vpp_stub_parity_test.go` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| ~~AC-1~~ | ~~(define per work item when this skeleton moves to `design`)~~ | ~~(define at design time)~~ superseded by AC-1..AC-15 below (design 2026-07-10) |
| AC-1 | `make ze-vpp-stub-parity-check` | Scans non-test `internal/` Go for binapi RequestMessage constructions, maps type -> wire name via vendored binapi, asserts each has a HANDLERS key in `vpp_stub.py`; RED while any of the 49 are missing, GREEN at completion; wired into the `ze-precommit-verify` check family; has `--selftest` + Go test like the sibling checks |
| AC-2 | ze applies `interface { backend vpp; loopback { unit ... address ... } }` against the stub (008) | Stub interface table allocates sw_if_index; create_loopback / sw_interface_set_flags / sw_interface_add_del_address / sw_interface_set_mtu handled with decoded fields in JSONL; `sw_interface_dump` streams the live table; 006 stays green (empty table at boot) |
| AC-3 | peer announces then withdraws 10.20.0.0/24 (003) | JSONL shows ip_route_add_del is_add=true then is_add=false for the prefix; driver exit 0; uses the new `send-withdraw` peer directive |
| AC-4 | stub SIGKILLed and a new instance bound on the same socket within the reconnect window (004) | govpp reconnects (A-3/A-4); a route announced after restart appears in the NEW instance's JSONL; ze does not crash or exit |
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

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | configures a loopback with an address under `backend vpp` | config -> iface config_apply CreateDummy (`config_apply.go`) -> ifacevpp create_loopback/set_flags/add_del_address -> stub | 008-iface-loopback.ci |
| 2 | BGP peer withdraws a route | peer UPDATE -> RIB -> sysrib best-change -> fib-vpp ip_route_add_del is_add=false -> stub | 003-fib-withdraw.ci |
| 3 | VPP restarts under ze (external supervisor) | socket drop -> govpp auto-reconnect (`connection.go` loop) -> new sockclnt_create -> route re-programmable | 004-vpp-restart.ci |
| 4 | inspects the VPP FIB via ze | show route dispatch -> ListKernelRoutes (`fib.go`) -> ip_route_v2_dump -> render | 014-route-dump.ci |
| 5 | VPP rejects a route (error path) | ip_route_add_del retval=-1 -> fib-vpp logs error, no install | 015-fault-injection.ci |
| 6 | configures gre/ipip/vxlan tunnels | config -> CreateTunnel (`tunnel.go`) -> gre/ipip/vxlan add -> stub sw_if_index | 009-iface-tunnel.ci |
| 7 | configures wireguard under vpp | config -> ConfigureWireguardDevice (`wireguard.go`) -> create + peer add -> stub | 010-iface-wireguard.ci |
| 8 | mirrors a port + shadows a loopback via LCP | config -> SetupMirror (`mirror.go`) / SetupLCPPair (`lcp.go`) -> stub | 011-iface-mirror-lcp.ci |
| 9 | watches interface state | VPP event -> monitor (`monitor.go`) -> EventBus consumers | 013-iface-monitor-event.ci |
| 10 | applies per-class traffic policy | traffic Apply -> policer + classify chain + binding (`classify_linux.go`) -> stub | 016-traffic-apply.ci |
| 11 | applies firewall + NAT | firewall Apply -> acl/nat44 messages -> stub | 017-firewall-acl-nat.ci |
| 12 | monitors VPP telemetry | stats segment -> poller (`telemetry.go`) -> Prometheus endpoint | 012-telemetry.ci |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| ~~(define at design time)~~ parity gate fixture tests | `scripts/checks/vpp_stub_parity_test.go` | gate finds a known-missing fixture message; passes on the real tree at completion; `--selftest` covers type->wire-name mapping incl. aliased imports | |
| send-withdraw builder/parse | `internal/test/peer/sendroute_parse_test.go` (extend) | `option=update:value=send-withdraw:prefix=X` parses; built UPDATE carries the prefix in withdrawn-routes per RFC 4271 | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `--inject` after=K | 0..N | 0 (fail immediately) | negative rejected at flag parse | N/A (large K = never fires) |
| `--inject` retval | any i32 | -1 typical | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| 012-telemetry, 003-fib-withdraw, 004-vpp-restart (new) (`.ci`) | test/vpp | VPP behaviour via the enriched stub | |
| 008-iface-loopback (new) | test/vpp | loopback lifecycle programmed into VPP (create/addr/flags/mtu/dump) | |
| 009-iface-tunnel (new) | test/vpp | gre + ipip + vxlan tunnels programmed | |
| 010-iface-wireguard (new) | test/vpp | wireguard interface + peer programmed; dumps reflect state | |
| 011-iface-mirror-lcp (new) | test/vpp | SPAN mirror + LCP pair programmed | |
| 013-iface-monitor-event (new) | test/vpp | interface event push reaches ze's monitor | |
| 014-route-dump (new) | test/vpp | show route returns stub-installed routes (+ neighbor dump) | |
| 015-fault-injection (new) | test/vpp | route programming failure surfaces, ze survives | |
| 016-traffic-apply (new) | test/vpp | apply-tier traffic (policer + classify + bind) -- unblocks the A-6 block named in test/traffic/020/026 | |
| 017-firewall-acl-nat (new) | test/vpp | apply-tier firewall acl + nat44 | |

### Interop Tests (MANDATORY for protocol features)
Not applicable with justification: the VPP binary API is not a wire protocol between routing daemons; there is no third-party peer to interop against. The interop analog for this surface is the real-VPP Docker evidence (`ze-deployment-vpp-test`, `ze-deployment-vpp-iface-test`), which stays authoritative for semantic correctness (see Key Design Decisions).

### Future (if deferring any tests)
- None planned. Any discovered-at-implementation deferral needs explicit user approval and a destination per `ai/rules/planning.md`.

## Files to Modify

- ~~`internal/plugins/iface/vpp/ifacevpp.go` - see Task work items~~ struck at design 2026-07-10: this spec does not modify the backends; they are the code the stub EXERCISES. Listed originally by the skeleton as context, not as edits.
- ~~`internal/plugins/traffic/vpp/backend_linux.go` - see Task work items~~ struck at design 2026-07-10: same reason.
- `test/scripts/vpp_stub.py` - stateful interface/route/policer/classify/acl-nat/wireguard tables; 49 new handlers; sw_interface_event emission; `--inject`; `--strict`; stats-segment serving (`--stats-socket`)
- `internal/test/peer/message.go`, `internal/test/peer/peer.go` (+ the option parser file that consumes `option=update:value=...`) - `send-withdraw` directive
- `Makefile` (or the owning `mk/*.mk`) - `ze-vpp-stub-parity-check` target, wired into the `_ze-verify-impl` check family (`Makefile:287`) next to the sibling checks; help text
- `docs/functional-tests.md` - vpp suite runbook: new tests, stub flags, stats emulation, parity gate

## Files to Create

- `scripts/checks/vpp_stub_parity.go` - parity gate (pattern: `scripts/checks/iface_resolution.go` -- `//go:build ignore` main, regexp scan, allowlist fixture, `--json`, `--selftest`)
- `scripts/checks/vpp_stub_parity_test.go` - gate self-tests
- `test/vpp/003-fib-withdraw.ci`
- `test/vpp/004-vpp-restart.ci`
- `test/vpp/008-iface-loopback.ci`
- `test/vpp/009-iface-tunnel.ci`
- `test/vpp/010-iface-wireguard.ci`
- `test/vpp/011-iface-mirror-lcp.ci`
- `test/vpp/012-telemetry.ci`
- `test/vpp/013-iface-monitor-event.ci`
- `test/vpp/014-route-dump.ci`
- `test/vpp/015-fault-injection.ci`
- `test/vpp/016-traffic-apply.ci`
- `test/vpp/017-firewall-acl-nat.ci`

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | no | test-infra only; no config surface added |
| CLI commands/flags | no | stub CLI flags are python argparse, not ze CLI |
| CLI grammar | no | no ze CLI commands added |
| Functional test for new RPC/API | yes | the 12 new `test/vpp/*.ci` above |
| Pipe completeness | no | no ze command output added |
| Env var registration | no | none |
| Doctor check for runtime dependencies | no | stub is test-only; no ze runtime dependency added |
| Prometheus counters/metrics | no | 012 consumes EXISTING `ze_vpp_*` metrics (`telemetry.go`); none added |
| Discovery updates (`ai/rules/repo-maintenance.md`) | yes | new gate + suite tests documented in `docs/functional-tests.md`; Makefile help lists the new target |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | no | test infrastructure only |
| 2 | Config syntax changed? | no | - |
| 3 | CLI command added/changed? | no | (make target documented via Makefile help) |
| 4 | API/RPC added/changed? | no | - |
| 5 | Plugin added/changed? | no | - |
| 6 | Has a user guide page? | no | `docs/guide/vpp.md` unchanged (no runtime behavior change) |
| 7 | Wire format changed? | no | stub emulates existing VPP wire format |
| 8 | Plugin SDK/protocol changed? | no | - |
| 9 | RFC behavior implemented/changed? | no | (003 exercises existing RFC 4271 withdraw handling; no behavior change) |
| 10 | Test infrastructure changed? | YES | `docs/functional-tests.md` (vpp suite runbook: new tests, stub state/flags, stats emulation, parity gate) |
| 11 | Affects daemon comparison? | no | - |
| 12 | Internal architecture changed? | no | (`docs/architecture/testing/ci-format.md` only if driver conventions change) |

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| D-1: Scope = the FULL request surface ze sends (57 messages incl. firewall/static/srv6), not the skeleton's iface subset | (a) iface-only per skeleton; (b) "messages with .ci demand" only | User goal 2026-07-10: "implement things correctly and fully for VPP". The wave proved partial stub coverage rots silently; a closed inventory + parity gate ends that class of drift |
| D-2: Stub becomes stateful (interface / route / policer / classify / acl-nat / wireguard tables) with `--seed-iface name[:index]` style pre-seeding | Canned per-message replies (status quo pattern, e.g. ip_route_lookup_v2) | Dumps must reflect prior adds (L180 explicitly: "state across calls"); traffic/firewall Apply resolves interfaces by dump, so ethernet-targeting tests need seeding; canned replies cannot express delete/reconcile paths |
| D-3: Parity gate as `scripts/checks/vpp_stub_parity.go` wired into `ze-precommit-verify` | (a) runtime-only detection (assert no `unhandled:true` in .ci logs); (b) Python checker script; (c) no gate | (a) only covers exercised paths -- exactly how the current gap grew; (b) breaks the repo's checks convention (`scripts/checks/*.go` with `--selftest` + Go test, cf. `iface_resolution.go`); (c) is the status quo that failed. Runtime strict mode is kept as a complement (AC-14), not the gate |
| D-4: Keep the generic unhandled fallback; add opt-in `--strict` (exit nonzero if any unhandled request was seen) | (a) delete the fallback; (b) fallback replies retval=-1 | (a) would hard-fail on future govpp-internal messages and break unrelated tests mid-transition; (b) misrepresents VPP (unknown-message errors are transport-level, not retval) and silently changes 5 existing tests. Strict mode gives new tests hard guarantees without destabilizing old ones |
| D-5: Stub-backed `.ci` = CI-runnable WIRING/REGRESSION proof; real-VPP Docker evidence = CORRECTNESS proof. Both required; neither replaces the other | (a) stub as the only gate; (b) real-VPP in CI | The stub cannot validate VPP semantics (R-4; 1096's INVALID_VALUE lesson was only findable on real VPP); Docker VPP is not per-commit-runnable in CI. The wave chose real-VPP evidence for its new surface; this spec closes the CI half of the split |
| D-6: Stats-segment emulation lives in `vpp_stub.py` behind `--stats-socket` (off by default) | (a) separate `vpp_stats_stub.py`; (b) Go helper inside ze-test | One VPP emulator process keeps `.ci` orchestration single-spawn; (b) would put VPP-protocol serving inside ze's own test binary, coupling emulator to consumer. Layout ground-truthed against vendored `statseg_v1.go`/`statseg_v2.go` (A-7); version choice (v1 vs v2) is an implementation detail so long as govpp accepts it |
| D-7: 003-fib-withdraw uses an explicit RFC 4271 withdraw via a new `send-withdraw` peer directive | Session-drop purge (peer disconnects; RIB withdraws all) | Explicit withdraw exercises the UPDATE-withdraw decode -> RIB remove -> sysrib -> fib path distinctly; session-drop conflates it with purge logic. Purge-on-drop can be asserted opportunistically in 004 (post-kill) if stable |
| D-8: 004-vpp-restart = external-mode stub process restart on the same socket, relying on govpp auto-reconnect (10x1s per `vpp.go`) | (a) in-stub socket bounce (drop + re-listen, same process); (b) ze-side vpp manager restart (non-external) | (a) doesn't prove message-table renegotiation with a fresh process (the real restart shape); (b) requires a real VPP binary. Distinct JSONL per instance avoids the startup truncation (:152-154) |
| D-9: sw_interface_event is emitted organically on every sw_interface_set_flags while armed (mirrors real VPP), not via a scripted trigger API | driver-triggered event injection endpoint on the stub | Zero new stub control surface; ze's own SetAdminUp during apply produces the trigger; deterministic for 013 |
| D-10: vpp suite stays NON-gating in `ze-functional-test`; the parity check IS gating in `ze-precommit-verify` | Promote the suite to gating now | R-2 (load sensitivity, 1097 gotcha) makes daemon-timing tests a bad commit gate today; the parity check is cheap, static, and catches the rot class. Revisit promotion once the grown suite proves stable in release evidence runs |
| D-11: Handlers decode and log the request fields ze's tests assert on (prefix, sw_if_index, state, indices), not full-message decoding | Full generic decode via scraping binapi field layouts | Field-offsets are hand-verified per message today (existing style, e.g. ip_route_add_del :316-378); full generic decode is a large correctness surface with no consumer; log what tests assert, extend per test need |

## Known Limitations

- The stub proves WIRING, not VPP semantics: retvals are scripted, no dataplane, no index-exhaustion/reuse realism. Real-VPP evidence (`ze-deployment-vpp-*`) remains the correctness gate (D-5).
- bridge/veth/xfrm under backend vpp are commit-gate rejected (`test/parse/iface-vpp-rejects-bridge.ci`, `-rejects-veth.ci`), so `bridge_domain_add_del_v2` / `sw_interface_set_l2_bridge` handlers are parity-covered but not config-drivable in a `.ci` today (R-7).
- Monitor re-arm after an external-mode VPP restart is not wired in ze (no EventReconnected in external mode, `vpp.go`); 004 scopes to fib re-programming. If 004's implementation confirms the monitor stays dead after reconnect, that is a ze bug to report + route to its own spec, not to fix silently here.
- The vpp suite remains outside the gating `ze-functional-test` list (D-10).
- `qos_*` messages fire only for VLAN identity qos-maps (non-identity rejected, `test/parse/iface-vpp-rejects-nonidentity-qos.ci`); their `.ci` coverage rides on the vlan leg of 008/009 only if a vlan unit is config-drivable there, else parity-only (record at implementation which).

## Implementation Steps

1. **Phase: split** - if the umbrella covers unrelated items, split into per-item specs first. (Design 2026-07-10: NOT split -- all items share the stub + suite; phases below sequence them.)
2. **Phase: design** - for the chosen item, re-verify the `file:line` evidence and fill the Data Flow / Wiring / AC sections above. (Done 2026-07-10.)
3. **Phase: wiring** - register entry points, write the failing wiring test.
4. **Phase: implement (TDD)** - write test, fail, implement, pass, per work item.
5. **Full verification** - `make ze-precommit-verify`.
6. **Complete spec** - fill audit tables, write `plan/learned/NNN-<name>.md`, two-commit closure.

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan, Message inventory |
| 3. Wiring phase | Wiring Test table -- Phase W below (parity gate RED) |
| 4. Implement (TDD) | Phases 1-7 below |
| 5. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` + `make ze-functional-vpp-test` |
| 6-9. Reviews | Critical Review Checklist below; fix; re-verify |
| 10-12. Deliverables/security/docs | Checklists below |
| 13. /ze-review gate | Review Gate section |
| 14. Present + close | Executive Summary; two-commit closure |

### Implementation Phases (design 2026-07-10)

Each phase: write test -> fail -> implement -> pass. Each ends with a self-critical review.

1. **Phase W: parity gate (wiring, MANDATORY FIRST)** -- `scripts/checks/vpp_stub_parity.go` + test + `make ze-vpp-stub-parity-check` wired into `ze-precommit-verify`. RED: reports exactly the 49 missing handlers from the inventory (cross-check the count; any delta = inventory drift, update this spec).
   - Tests: `vpp_stub_parity_test.go`; Verify: gate output lists 49
2. **Phase 1: stub state + iface core** -- interface table (create/delete loopback + vlan subif, flags, addresses, mtu, mac, clear-stats, l2/bridge, qos), `sw_interface_dump` streaming, `--seed-iface`, `--strict`.
   - Tests: `008-iface-loopback.ci` (new), existing 001/002/005/006/007 stay green
3. **Phase 2: FIB state + fault injection** -- route table recording add/del; `ip_route_v2_dump` + `ip_neighbor_dump`; `sr_steering_add_del`; `--inject msg:retval=N:after=K`; peer `send-withdraw` directive.
   - Tests: `003-fib-withdraw.ci`, `014-route-dump.ci`, `015-fault-injection.ci`, peer unit test
4. **Phase 3: restart** -- per-instance logs in the driver; assert reconnect + post-restart programming.
   - Tests: `004-vpp-restart.ci`; Verify: A-4 timing observed in logs
5. **Phase 4: tunnels / wireguard / mirror / LCP** -- handlers + state for gre/ipip/vxlan, wireguard (create/delete/peers/dumps), span, lcp.
   - Tests: `009-iface-tunnel.ci`, `010-iface-wireguard.ci`, `011-iface-mirror-lcp.ci`
6. **Phase 5: events** -- arm on want_interface_events; emit sw_interface_event on set_flags (D-9).
   - Tests: `013-iface-monitor-event.ci`; Verify: A-9
7. **Phase 6: traffic + firewall apply tier** -- policer (add/del/dump/output), classify session + bindings, acl (add/del/dumps/set-list), nat44 set.
   - Tests: `016-traffic-apply.ci`, `017-firewall-acl-nat.ci`; Verify: A-5; parity gate now GREEN
8. **Phase 7: stats segment** -- seqpacket + memfd + v1-or-v2 segment behind `--stats-socket`; canned interface/node/system counters.
   - Tests: `012-telemetry.ci`; Verify: A-6/A-7
9. **Functional tests** -> all 12 new `.ci` green under `make ze-functional-vpp-test`; docs (`docs/functional-tests.md`).
10. **Full verification** -> `make ze-precommit-verify` (includes the new parity gate).
11. **Complete spec** -> audit tables, `plan/learned/NNN-finish-vpp-stub.md`, two-commit closure.

### Critical Review Checklist (/implement stage 6)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1..AC-15 has implementation + test with file:line; parity gate GREEN with zero missing |
| Feature completeness | Every End-to-End User Story path runs; 016 proves the A-6 unblock claimed by test/traffic/020/026 comments |
| Correctness | Handler field offsets match the vendored binapi Unmarshal layouts (cite `.ba.go` lines in handler docstrings, existing style) |
| Data flow | Stub state mutations only via handlers; dumps read state, never fabricate; JSONL remains the single assertion surface for drivers |
| Registration over hardcoding | New .ci auto-discovered by `ze-test vpp` (directory scan, no list to edit); parity gate self-contained in scripts/checks |
| Rule: no-workarounds | No test weakened to pass (R-2: report load-pressure failures, do not raise sleeps or soften asserts) |
| Rule: ci-sleep | No raised sleep baselines; JSONL polling with deadlines |
| Secrets | Wireguard private keys never logged (AC-8) |

### Deliverables Checklist (/implement stage 10)

| Deliverable | Verification method |
|-------------|---------------------|
| 49 new handlers | `make ze-vpp-stub-parity-check` output: 0 missing |
| 12 new .ci | `ls test/vpp/*.ci` shows 17 files; `bin/ze-test vpp -l` lists them; `make ze-functional-vpp-test` green |
| parity gate wired | `grep ze-vpp-stub-parity-check Makefile mk/*.mk`; `make ze-precommit-verify` runs it |
| peer withdraw directive | `go test ./internal/test/peer/ -run '.*[Ww]ithdraw.*'` |
| strict mode | `python3 test/scripts/vpp_stub.py --help` shows `--strict`; a driver-level negative check |
| stats emulation | 012 green; `ze_vpp_stats_up 1` scraped in its driver output |
| docs | `git diff docs/functional-tests.md` non-empty |

### Security Review Checklist (/implement stage 11)

| Check | What to look for |
|-------|-----------------|
| Input validation | Stub parses untrusted-length frames: keep bounds checks before every struct.unpack (existing style :344-358); malformed frame = logged IOError, not crash |
| Secrets in logs | wireguard key bytes and any future auth material never written to JSONL (log presence booleans only) |
| Resource exhaustion | Stub state tables bounded by test lifetime + `--deadline` self-SIGTERM (existing); no unbounded growth per message |
| Socket permissions | Keep 0o600 chmod on both API and stats sockets (existing :664) |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Handler decode mismatch (wrong offsets) | Re-read the vendored `.ba.go` Unmarshal for that message; fix handler; add the offset citation |
| .ci flaky under load | R-2 protocol: report, poll harder, never weaken; keep suite non-gating |
| Apply needs an unlisted message | Inventory drift: add the row here, extend parity fixture, implement handler (scope stays closed-list) |
| A-N broken | Mistake Log row + Deviations; if design-invalidating, STOP and present |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights
<!-- LIVE -- write IMMEDIATELY when you learn something -->
- (design 2026-07-10) The generic fallback made the stub gap invisible: retval-only messages "worked" (wire-correct accidental reply), extra-field replies failed loudly only when a test exercised them, dumps failed SILENTLY as empty streams. Three different failure shapes from one fallback -- which is why a static parity gate, not more runtime probing, is the durable fix.

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

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|

### Files from Plan
| File | Status | Notes |
|------|--------|-------|

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:** (all require user approval)
- **Skipped:** (all require user approval)
- **Changed:** (documented in Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| Stub covers every message ze sends (correct + full VPP support, CI side) | static gate + functional tests | `make ze-vpp-stub-parity-check` 0 missing; `make ze-functional-vpp-test` all green |
| Apply-tier VPP paths CI-covered without hardware | functional tests | 008-017 `.ci` green; test/traffic 020/026 "blocked on A-6" comments retired (updated to point at 016) |
| Error/restart/telemetry paths covered | functional tests | 015, 004, 012 green |
| Real-VPP correctness unchanged and still authoritative | evidence run | `make ze-deployment-vpp-test` + `ze-deployment-vpp-iface-test` still green (no evidence-script change in this spec) |

## Review Gate

<!-- BLOCKING (ai/rules/planning.md Review Gate). Filled by /ze-implement's /ze-review gate: -->
<!-- the final review before closure, run AFTER the inline critical/security/doc reviews, over the complete diff. -->
<!-- Every BLOCKER and ISSUE (severity > NOTE) must be fixed, then re-run /ze-review. -->
<!-- Loop until the review returns 0 BLOCKER/0 ISSUE (only NOTEs, or nothing). Paste the final clean run. -->
<!-- NOTE-only findings do not block — record them and proceed. -->

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | [what /ze-review reported] | file:line | fixed in <commit/line> / deferred (id) / acknowledged |

### Fixes applied
- [short bullet per BLOCKER/ISSUE, naming the file and change]

### Run 2+ (re-runs until clean)
<!-- Add a new block per re-run. Final run MUST show zero BLOCKER/ISSUE. -->
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
- [ ] Every chosen work item has feature code + test
- [ ] Wiring Test table complete (concrete test names, none deferred)
- [ ] `make ze-standard-test` passes (lint + all ze tests)
- [ ] Registration over hardcoding respected
- [ ] AC-1..AC-15 all demonstrated; parity gate GREEN and wired into ze-precommit-verify
- [ ] End-to-End User Stories 1-12 each have a passing test
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`); broken ones in Mistake Log

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

## Notes
- Skeleton = captured intent, not a designed spec (see `ai/rules/planning.md`). Moves to `design` when someone picks it up.
- Design filled 2026-07-10 from firsthand re-verification of every producer cited above.
- user instruction 2026-07-10 authorized conversion to ready; goal statement: implement VPP correctly and fully.
