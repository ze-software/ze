# 926 -- isis-0-umbrella

## Context
`isis-0` is the umbrella spec for native IS-IS (ISO/IEC 10589, RFC 1195 / 5305 /
5308 / 5301 / 5303 / 5304 / 5310 / 2966 / 3787 / 3786) in Ze: a link-state IGP
running directly over Layer 2, computing per-level shortest paths, installing the
results into the kernel FIB, and interoperating with the BGP engine through
redistribution. The umbrella scoped the work into 13 child specs (isis-1 types
.. isis-13 cli/diag/interop) and owns the cross-spec contracts: the two distinct
route paths (Loc-RIB install vs `redistevents` redistribution), the PDU receive
dispatcher, the TLV inventory and TLV 135/236 entry layout, the metrics
namespace, and the test/interop wiring. The 13 children were implemented (sibling
sessions) and verified; this umbrella records that the whole vertical slice is in
place and integrated. Closing it is the integration sign-off, not a from-scratch
build.

## What was implemented
- Full IS-IS component under `internal/component/isis/` with the planned layered
  packages: `types`, `packet` (PDU + TLV codecs, Fletcher checksum, auth sign/
  verify), `transport` (AF_PACKET raw L2 behind a backend interface), `circuit`
  (RX/TX/timers, DIS election), `adjacency` (FSM + table), `lsdb` (store,
  origination, aging, flooding, SNP, pseudo-node), `spf` (Dijkstra, leak,
  install), `redistribute` (source + consumer + events producer), `yang`, plus
  the root engine files (register/server/circuits + the `*_wiring.go` cross-
  package glue: lsdb/flooding/spf/dis/auth/redist).
- Component registration + SDK lifecycle (`register.go`: `registry.Registration`
  with `RunEngine=runISISEngine`, `OnConfigure`/`OnConfigApply`/`OnStarted`/
  `OnExecuteCommand`); `ze-isis-conf.yang` (system-id/NET, levels, per-interface
  metric/hello/circuit-type/passive, auth refs) and `ze-isis-cmd.yang` (show/
  clear tree binding the central `ze-show:`/`ze-clear:` namespaces, LDP-style).
- FIB install via Loc-RIB insertion: SPF emits one `locrib.Path` per equal-cost
  next-hop (distinct `Instance`), `Source` = IS-IS ProtocolID, `AdminDistance` =
  115 (existing `rib.admin-distance.isis` leaf), through `InsertForward`
  (`spf/install.go`, mirroring BGP `rib_bestchange.go`). sysrib/locrib were
  extended to expand a Loc-RIB path-group into `BestChangeEntry.ECMPPaths`
  (`internal/plugins/sysrib/sysrib_ecmp_pathgroup_test.go`), so ECMP next-hops
  survive to the kernel instead of collapsing on the protocol-keyed map.
- Redistribution as a separate path: a SINGLE source `isis`
  (`redistevents.RegisterProtocol("isis")` + `RegisterProducer`, plus
  `configredist.RegisterSource` via a `mustRegister` wrapper) and a
  `RedistConsumer` for connected/static/BGP into IS-IS LSPs.
- L1 + L2 hierarchy with RFC 2966 up/down leaking, P2P + broadcast circuits
  (DIS election + pseudo-node LSPs), dual-stack IPv4 (TLV 135) + IPv6 (TLV
  232/236), and authentication (TLV 10 cleartext / HMAC-MD5 type 54 / generic
  crypto HMAC-SHA type 3, per-interface and per-level key chains).
- CLI (`show isis neighbor/database/route/interface/hostname/spf-log`, `clear
  isis`), Prometheus metrics (the full canonical `ze_isis_*` set), doctor checks
  (`doctor-isis-raw-socket` CAP_NET_RAW, NET/system-id sanity), and docs
  (`docs/guide/isis.md`, `docs/architecture/wire/isis.md`, metrics/comparison/
  features rows).
- `test/isis/*.ci` functional suite (registered in
  `internal/test/cli/register.go` + `mk/test-functional.mk`), `test/isis-wire/`
  offline PDU decode, Linux-only QEMU integration tests
  (`*_integration_linux_test.go` in the root and `transport` packages, wired into
  `scripts/evidence/qemu-all-tests.sh`), and six FRR interop scenarios under
  `test/interop/scenarios/isis-*-frr/` with an FRR isisd helper in
  `test/interop/interop.py`.

## Key decisions
- Two route paths kept strictly separate (the central umbrella contract): the
  FIB path is SPF -> `locrib.Path` -> sysrib `OnChange` -> fibkernel (`RTPROT_ZE`),
  exactly as BGP installs; `redistevents` feeds ONLY the redistribute-orchestrator
  toward BGP and never touches the FIB. Conflating them was the main design
  hazard and is explicitly checked in the Critical Review checklist.
- Single IS-IS admin distance (115) on `locrib.Path.AdminDistance`. `locrib.Path`
  has no protoType/level field, so RFC 5308 multi-level preference (L1-up >
  L2-up > L2-down > L1-down then metric) is resolved INSIDE IS-IS SPF, which
  publishes one winning Path per prefix. Per-level distance vs other protocols is
  future work needing a `locrib.Path` protoType field.
- Single redistribution SOURCE name `isis` (not `isis-l1`/`isis-l2`):
  `RouteChangeBatch` has no level field and the orchestrator derives the source
  from `ProtocolName(b.Protocol)`, so a second protocol ID would also defeat the
  generic self-import loop-prevention (`route.Origin == importingProtocol`).
  Single `isis` keeps self-import auto-rejected and matches the single distance.
- ECMP made a committed deliverable, not a limitation: because sysrib keys
  `s.routes[key]` by protocol string and replays only the single best Path, the
  Loc-RIB path-group expansion in sysrib/locrib was required and delivered.
- Lazy raw-bytes LSDB (buffer-first): LSPs stored as raw bytes + parsed metadata;
  TLVs parsed on demand for SPF/CLI; unknown TLVs re-flood verbatim
  (ISO/IEC 10589 7.3.14).
- One file per TLV FAMILY (core/IPv4/IPv6/auth/opaque), a middle path between
  bio-rd's one-file-per-TLV scatter and FRR's monolith.

## Gotchas / traps
- The PDU receive dispatcher is owned by isis-4 `server.go`, keyed by the 5-bit
  PDU type; the transport (`isis-3`) delivers `(ifindex, pdu)` after stripping
  802.3+LLC and holds NO protocol switch. Do not push protocol knowledge into the
  transport layer.
- Padding-then-authentication ordering has exactly ONE owner (the engine, not the
  transport): build PDU with TLV 8 padding to MTU -> insert/sign TLV 10 (which
  signs the padded bytes, per RFC 5304) -> compute Fletcher checksum (LSPs) ->
  hand final bytes to transport, which adds only framing and MUST NOT pad/alter.
- TLV 135 up/down bit lives in the CONTROL octet (0x80), not in the 32-bit
  metric; TLV 236 flags are U|X|S|Reserve(5) MSB-first (RFC 5308 sec 2). The
  sub-TLV-length octet is present ONLY when the sub-TLV-present bit is set. Getting
  the bit position wrong silently corrupts interop.
- Fletcher checksum needs the ISO two-step adjustment (guide sec 12.1); it is
  vector-tested in `packet/checksum_test.go` before any runtime relied on it.
- `.ci` naming: the umbrella Functional Tests table uses `isis-redist-bgp.ci`
  (present); the redistribution INTEROP scenario directory is `isis-redist-frr/`
  (an extra over the four originally tabled scenarios), and `isis-convergence-frr/`
  is an additional scenario for link-down reconvergence (AC-9 on the wire). The
  spec's interop table named four scenarios (p2p/lan-dis/dualstack/auth); all four
  exist plus the two extras.

## Interop validation pending Linux execution
The whole tree builds on darwin and linux (`go build ./...` exit 0 both GOOS),
all `internal/component/isis/...` packages pass under `-race` (exit 0), and
`go vet ./internal/component/isis/...` is clean. The on-the-wire interop ACs
(AC-1 P2P 3-way, AC-2 multi-node convergence/install, AC-3 LAN DIS, AC-4 L1<->L2
leak, AC-5 IPv6, AC-6 auth, AC-9 hold-timer down + withdraw, AC-10 FRR interop)
are demonstrated for their protocol LOGIC by unit/functional tests on darwin, and
their END-TO-END wire behaviour is captured in six FRR scenarios
(`test/interop/scenarios/isis-{p2p,lan-dis,dualstack,auth,convergence,redist}-frr`)
plus the QEMU veth integration tests (`*_integration_linux_test.go`, wired into
`scripts/evidence/qemu-all-tests.sh`). These scenarios and integration tests are
WRITTEN but their EXECUTION is pending a Linux/QEMU host (raw AF_PACKET + FRR
isisd cannot run on the darwin development host). Interop validation is therefore
pending Linux execution; the implementation itself is complete.

## Files

None recorded.
