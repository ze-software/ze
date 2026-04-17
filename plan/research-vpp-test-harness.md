# Research: VPP Python API Stub Harness for `.ci` Integration Tests

## Status: Design sketch (pre-spec)

**Promotion path:** when work starts, rename to `plan/spec-vpp-7-test-harness.md`,
fill in the full `plan/TEMPLATE.md` scaffold, add to `spec-vpp-0-umbrella.md`
child-spec table.

## Why this is needed

Phases vpp-1 (lifecycle) and vpp-2 (FIB) are in the tree with unit tests and
config-parse `.ci` tests (`test/parse/vpp-config-*.ci`) and a plugin-load
`.ci` test that exercises the mock-backend fallback
(`test/plugin/fib-vpp-plugin-load.ci`). None of those prove that the
GoVPP wire path works: the `govppBackend.routeAddDel` call that builds an
`IPRouteAddDel` message and writes it to `/run/vpp/api.sock` is never
exercised in CI.

The umbrella spec's `## Wiring Test` table names six `.ci` tests under
`test/vpp/` (`001-boot` through `007-coexist`). None exist. A production-
style test would need either a real VPP container (heavy, brittle) or a
stub that speaks enough of VPP's binary API to make ze's GoVPP client
happy.

This document specifies the stub.

## Wire format (what the stub must speak)

Source: `vendor/go.fd.io/govpp/adapter/socketclient/socketclient.go` lines
39-44 (socket name), 356-360 (hard-coded message IDs), 485-553 (frame
writer), 611+ (reply header shape).

### Frame envelope

Every message on the wire is preceded by a 16-byte header:

| Offset | Length | Field | Value |
|--------|--------|-------|-------|
| 0..7 | 8 | (padding / pool reuse) | Stub writes zeros. |
| 8..11 | 4 | payload length | big-endian uint32 |
| 12..15 | 4 | (padding) | Stub writes zeros. |

Payload immediately follows the header.

### Request body

| Offset | Length | Field |
|--------|--------|-------|
| 0..1 | 2 | MsgID (big-endian uint16) |
| 2..5 | 4 | ClientIndex (big-endian uint32) |
| 6..9 | 4 | Context (big-endian uint32) |
| 10.. | variable | request-specific encoded fields |

### Reply body

| Offset | Length | Field |
|--------|--------|-------|
| 0..1 | 2 | MsgID |
| 2..5 | 4 | Context (echo from request) |
| 6.. | variable | reply-specific fields (first is usually `Retval` int32) |

## Handshake the stub must support

GoVPP's `Client.open()` runs this sequence on connect:

1. Client sends `sockclnt_create` with hard-coded MsgID **15**. Body:
   client name string (VPP uses length-prefix null-terminated strings;
   GoVPP encodes a fixed-length string by default).
2. Stub replies with `sockclnt_create_reply` containing:
   - `Response` int32 (0 = success)
   - `Index` uint32 (the ClientIndex the client must use on every later
     request)
   - `Count` uint16 (number of entries in the message table)
   - `MessageTable` [Count]`MessageTableEntry{MsgID uint16, MsgName
     [64]byte}` where `MsgName` is `<name>_<crc>` (VPP convention).

The `msgTable` negotiation is the main complexity: GoVPP looks up every
message by name, so the stub must declare every message ze wants to
send, with the CRC suffix that the generated `binapi` files embed.

**Shortcut:** the stub can scrape message names + CRCs out of the
vendored `vendor/go.fd.io/govpp/binapi/*/` packages at start-up (each
message has a `MessageName()` method returning the `<name>_<crc>`
string). That way the stub stays in sync with whatever VPP version ze
pins.

### Control messages

| Name | Direction | Stub behavior |
|------|-----------|---------------|
| `sockclnt_create` | client to VPP | Reply with success + message table. |
| `sockclnt_delete` | client to VPP | Reply success, close connection. |
| `control_ping` | client to VPP | Reply `control_ping_reply` with retval 0, vpe_pid = stub PID, client_index = 1. GoVPP uses ping to terminate dump replies. |

### FIB messages (phase vpp-2 coverage)

| Name | Stub behavior |
|------|---------------|
| `ip_route_add_del` | Parse `is_add`, table ID, prefix, paths. Append `(op, prefix, next_hop)` to an in-memory request log. Reply with retval=0. |
| `ip_route_add_del_v2` | Same, if/when ze upgrades. |
| `ip_route_dump` (future) | Stream current request log filtered by table ID, then a `control_ping_reply` to mark end. |

### Interface messages (phase vpp-4 coverage, stub extension)

`sw_interface_dump`, `sw_interface_set_flags`,
`sw_interface_add_del_address`, `create_loopback`, `delete_loopback`,
`create_vlan_subif`, `lcp_itf_pair_add_del_v3`, etc. Each handler: parse
just enough of the request to record the operation; reply with retval=0.

## Stub process contract

| Aspect | Design |
|--------|--------|
| Language | Python 3 stdlib only (socket, struct, threading, select). No pip deps. |
| Invocation | `python3 test/scripts/vpp-stub.py --socket /tmp/ze-test-<id>/api.sock --log /tmp/ze-test-<id>/vpp-requests.jsonl [--deadline Ns]` |
| Lifecycle | Starts, listens, accepts one client (ze), handles until deadline or SIGTERM, exits 0 on clean termination. |
| Request log | Each handled request appended as a single JSON line: `{"ts": "...", "msg": "ip_route_add_del", "context": N, "fields": {...}}`. Tests grep this log. |
| Msg table source | At start-up, import every generated `binapi/*.api.json` file (they ship with the VPP distribution and are also copied into `vendor/go.fd.io/govpp/binapi/<module>/*.ba.go`). Extract (name, CRC) tuples. |
| Concurrency | Single-threaded event loop with `select`. Fib-vpp sends requests sequentially; no need for goroutine-style parallelism. |
| Failure injection (future) | Optional `--inject "ip_route_add_del:retval=-1:after=3"` to fail the 4th and later requests. Useful for testing fib-vpp error handling. |

## Test harness integration

A new test category `test/vpp/` with its own runner, modeled on
`test/plugin/`:

| File | What it does |
|------|--------------|
| `test/vpp/001-boot.ci` | Start stub, start ze with `vpp.enabled=true` and a minimal DPDK section, assert `sockclnt_create` arrives and `control_ping` gets answered. |
| `test/vpp/002-fib-route.ci` | Start stub, start ze with a peer that sends a prefix, assert the stub's request log contains `ip_route_add_del is_add=true prefix=10.0.0.0/24`. |
| `test/vpp/003-fib-withdraw.ci` | Peer withdraws, stub log contains `is_add=false`. |
| `test/vpp/004-vpp-restart.ci` | Stub exits, ze reconnects, fib-vpp emits replay-request, stub sees full table re-pushed. |
| `test/vpp/005-mpls-push.ci` | (vpp-3 deferred) labelled unicast prefix leads to `mpls_route_add_del`. |
| `test/vpp/006-iface-create.ci` | (vpp-4) ze creates a loopback, stub sees `create_loopback`. |
| `test/vpp/007-coexist.ci` | fib-kernel programs kernel routes, fib-vpp programs the stub, both logs populated with the same prefix. |

### Runner

Adds a new subcommand: `ze-test vpp [001..007]`. Responsibilities:

| Step | Action |
|------|--------|
| 1 | Allocate a temp dir and socket path. |
| 2 | Start `vpp-stub.py` as a subprocess, pass socket path. Wait for socket file to appear (bounded). |
| 3 | Start ze with `vpp.api-socket = <allocated path>` and `vpp.enabled = true`. DPDK block stays empty (stub does not need to bind NICs). |
| 4 | Let the test script drive peers / config reloads. |
| 5 | SIGTERM ze, then stub. |
| 6 | Assert against `vpp-requests.jsonl` using grep-style helpers (`expect_request=name:field=value`). |

### YANG tweak required

The current YANG for `vpp.api-socket` accepts any string path. The stub
socket path will be per-test under `/tmp/`. Ze already validates the path
format (absolute, no `..`, under 108 chars) in
`internal/component/vpp/config.go` `validateSocketPath`; temp paths
satisfy that. No schema change needed.

### DPDK bypass

Today, `internal/component/vpp/dpdk.go` `BindAll` runs the vfio
unbind/bind dance on every configured interface. In a stub test we do
not want this (no real NICs present, no root privileges). Two options:

1. Skip DPDK binding when `vpp.dpdk.interface` is empty (today this is
   already the case: the loop iterates zero times). Run stub tests with
   zero DPDK interfaces.
2. Add a dev-only `vpp.test-mode` leaf that skips `BindAll` even when
   interfaces are declared. Rejected: dev-only config leaves are an
   anti-pattern (`rules/design-principles.md` "Explicit > implicit").

Go with option 1: stub tests declare no DPDK interfaces.

### Skipping VPP process exec

The VPPManager execs `/usr/bin/vpp` by default. In stub tests we want
ze to connect to the stub socket without execing a real VPP.

Two plausible paths:

1. **Inject the VPP binary path** as a config leaf (`vpp.binary-path`,
   default `/usr/bin/vpp`). In stub tests, set it to `/bin/sleep
   infinity` so the supervisor has something to wait on. Ze's connector
   opens the stub's socket regardless of what the "VPP binary" is doing.
2. **Split lifecycle from connection**: add `vpp.external` boolean; when
   true, the VPPManager skips `cmd.Start()` and only runs the connector
   + stats poller. Matches how real VPP deployments managed by systemd
   would work anyway (ze is not always the supervisor).

Option 2 is cleaner and has independent value (gokrazy vs systemd
deployments). Design decision lives in the promotion spec.

## Open questions (for the promotion spec)

| Question | Impact |
|----------|--------|
| Do we ship the stub under `test/scripts/` or `cmd/ze-test/vpp-stub/`? | Packaging. `cmd/` would make it part of the ze-test binary; `test/scripts/` keeps it out of the compiled binary. |
| Does the msg-table scan import `vendor/.../binapi` at Python runtime (via a pre-generated JSON index) or read `.api.json` files shipped with VPP? | Reproducibility vs VPP-version coupling. |
| Does the stub support the stats segment (shared memory, separate from binary API socket)? | Required for vpp-6 telemetry coverage; adds non-trivial mmap scaffolding. Recommend: defer. |
| Do we add `vpp.external` (option 2 above) or live with `vpp.binary-path` (option 1)? | User-visible config surface. |
| Should stub failures (injected retvals) drive a separate sub-category like `test/vpp-fault/`? | Test layout. Mirror `test/chaos-web/` precedent. |

## Estimated size

| Component | Approximate LOC |
|-----------|-----------------|
| `test/scripts/vpp-stub.py` (core + control + FIB) | 600-800 |
| `test/vpp/` runner integration in `cmd/ze-test/` | 300-500 Go |
| First three `.ci` tests (`001-boot`, `002-fib-route`, `003-fib-withdraw`) | 200 LOC `.ci` each |
| `vpp.external` config plumbing, if chosen | 100-150 Go + YANG + unit test |

## Phase Summary

| Phase | Scope |
|-------|-------|
| 1 | Stub core + control (`sockclnt_create`, `control_ping`, `sockclnt_delete`). `test/vpp/001-boot.ci`. |
| 2 | FIB handlers (`ip_route_add_del`, `ip_route_add_del_v2`). `002-fib-route.ci`, `003-fib-withdraw.ci`, `007-coexist.ci`. |
| 3 | `vpp.external` config path (or equivalent) so stub tests run without execing VPP. |
| 4 | Interface handlers for vpp-4 (`006-iface-create.ci`). |
| 5 | Restart scenario (`004-vpp-restart.ci`). |
| 6 | MPLS handlers when vpp-3 unblocks (`005-mpls-push.ci`). |

## References

- `vendor/go.fd.io/govpp/adapter/socketclient/socketclient.go` — wire format, hard-coded msg IDs, handshake.
- `internal/component/vpp/conn.go` — GoVPP AsyncConnect state machine.
- `internal/plugins/fibvpp/backend.go` — `govppBackend.routeAddDel` builds the `IPRouteAddDel` the stub must parse.
- `docs/research/vpp-deployment-reference.md` — API client table listing every GoVPP module ze targets.
- `plan/spec-vpp-0-umbrella.md` Wiring Test table — the seven `.ci` tests this harness enables.
