# The routing-protocols door: OSPF, IS-IS, LDP, RSVP-TE, MPLS and BFD

<!-- door-base: internal/plugins/ospf internal/plugins/isis internal/plugins/ldp internal/plugins/rsvpte internal/component/mpls internal/component/bfd -->

Read `ai/doors/system.md` first, then `ai/doors/forwarding.md` for the path a
route takes after a protocol selects it. This page covers the protocol engines.
Each one talks to another vendor's router, so the RFC and the peer judge the
work, not the tests in this repository alone.

## Where it sits

- **Each protocol is an edge plugin that owns its whole engine:** wire codec,
  adjacency or session FSM, database, timers and route computation. The engine
  core holds no protocol logic (`ai/digests/ospf.md`, `ai/digests/isis.md`,
  `ai/digests/mpls-signaling.md`). OSPF runs one engine per OSPFv2 instance and
  per OSPFv3 address family over shared codecs (`ai/digests/ospf.md`).
- **BFD is a component** (`internal/component/bfd`) that BGP, OSPF and static
  routes use as clients (`docs/architecture/bfd.md`). Its subpackage layout is
  the reference skeleton for a new protocol (`ai/rules/protocol.md`,
  `docs/architecture/protocol-skeleton.md`).
- **`internal/component/mpls` is read-only.** It dumps the kernel MPLS table
  for `show mpls forwarding` (`internal/component/mpls/doc.go`).
- **Every one can be compiled out:** `ze_ospf`, `ze_isis`, `ze_ldp`,
  `ze_rsvpte`, `ze_mpls`, `ze_bfd` (`feature-gates.txt`).

## Who depends on it from outside

| Outside party | What they hold | The contract |
|---------------|----------------|--------------|
| A protocol peer: FRR, BIRD, freeRouter, a vendor router | Adjacencies, sessions, LSAs, LSPs, label bindings, PATH and RESV state | The RFC text in `rfc/full/`, never the summary (`ai/rules/rfc-compliance.md`). Interop scenarios prove it against a real peer (`docs/architecture/testing/interop.md`) |
| The operator | `ospf`, `isis`, `ldp`, `rsvp-te`, `bfd` config and the `show` commands | The YANG schema and the guides: `docs/guide/ospf.md`, `isis.md`, `mpls.md`, `rsvp-te.md`, `bfd.md` |
| The forwarding plane | The kernel or VPP FIB, shared with other writers | Routes reach it only through the Loc-RIB and sysrib. Labels reach it only through `(mpls-fib, entry)` events (Boundaries) |
| Other protocols on the box | BGP, static, redistribution consumers | Administrative distance in the Loc-RIB (`rib { distance }`), and the redistribution bus `redistevents`, which never installs a route (`internal/plugins/isis/spf/install.go`, package comment) |
| The public conformance ledger | Each tagged test's claim | `RFC requirement:` tags and their discrimination records, judged by `./le rfc check` (`docs/contributing/rfc-conformance-gates.md`) |

## Boundaries

- **A protocol never writes a kernel route.** OSPF and IS-IS SPF insert
  `locrib.Path` values (`internal/plugins/ospf/spf/install.go` and
  `internal/plugins/isis/spf/install.go`, `Installer.insertPath`). sysrib
  selects and `fib-kernel` or `fib-vpp` programs. No file under `ospf`,
  `isis` or `ldp` imports netlink. `rsvpte/transport_linux.go` uses netlink
  only to read routes and links.
- **Label entries go through the bus.** LDP and RSVP-TE emit
  `(mpls-fib, entry)` events (`internal/core/mplsfib/events.go`, `Apply`), and
  `fib-kernel` is the only code that programs them
  (`ai/digests/mpls-signaling.md`).
- **The plugin can run forked.** In a subprocess `locrib.Default()` is nil, so
  the installer needs its `RouteSink`, which ships each batch over the
  `route-install` RPC (`docs/architecture/forked-route-install.md`). An
  installer with neither sink drops every route with no error.
- **The BFD client lookup is in-process only.** `api.GetService` answers nil in
  a forked plugin, and external support is out of scope by the package's own
  comment (`internal/component/bfd/api/registry.go`). Every caller MUST check
  for nil, as that comment requires.
- **Data-plane choice is a declaration.** A route producer declares
  `NeedsDataPlane` and the engine resolves the writer
  (`internal/component/plugin/registry/registry.go`). Do not add a FIB plugin
  name to `Dependencies` (Traps).
- **Redistribution and installation are two buses.** `redistevents` feeds
  other protocols' advertisement. The Loc-RIB feeds the FIB
  (`ai/digests/fib-programming.md`).

## RFC and interop obligations

- **Quote the RFC above the code that implements it**, with number and
  section, and name the section at each caller (`ai/rules/rfc-compliance.md`).
- **A tag you add or change carries a discrimination record** written by
  `./le rfc discriminate-record` (`docs/contributing/rfc-conformance-gates.md`).
- **RFC scope is frozen.** The owner decided on 2026-10-08: no new enrolment
  and no new `rfc/short/` summary. Fix RFCs already in scope only. A spec
  records it this way: `plan/immediate/spec-ospf-max-metric-startup-shutdown-windows.md`.
- **A wire-visible change owes an interop scenario.** OSPF, OSPFv3, IS-IS, LDP
  and BFD scenarios against FRR and BIRD live in `test/interop/scenarios/`,
  named for the protocol, the case and the peer. RSVP-TE uses freeRouter in
  `test/interop-rsvpte/scenarios/`. Prove each one red with the fix reverted
  and the image rebuilt (`ai/rules/interop-and-goal-validation.md`).

## Read before code

| Read | Question it answers |
|------|---------------------|
| `docs/architecture/ospf/README.md`, `ospf-8-spf-rib.md` | The OSPF page set, and how SPF results reach the Loc-RIB |
| `docs/architecture/isis/isis-9-spf-rib.md`, `isis-4-component-config.md` | IS-IS route install and config |
| `docs/architecture/ldp/mpls-ldp.md`, `docs/architecture/rsvpte/mpls-rsvp-te.md` | Label distribution and LSP setup |
| `docs/architecture/mpls/mpls-kernel.md` | How labels reach the kernel, and its two test traps |
| `docs/architecture/bfd.md`, `docs/architecture/ospf/bfd-client.md` | The BFD service and how a protocol subscribes |
| `docs/architecture/forked-route-install.md` | Route install from a subprocess |
| `docs/architecture/testing/interop.md` | Suites, scenario layout, the discrimination walk |
| `ai/digests/ospf.md`, `isis.md`, `mpls-signaling.md`, `fib-programming.md` | The current code flow |
| `ai/patterns/plugin.md` | Registration, the five-stage protocol, events |
| `ai/rules/rfc-compliance.md`, `ai/rules/interop-and-goal-validation.md`, `ai/rules/protocol.md` | What a protocol change owes |

## Traps

- **OSPF, IS-IS and LDP name `fib-kernel` in `Dependencies`**, so a VPP
  deployment loads the kernel writer (`plan/journal/plugin-list-hardcoded.md`,
  and `ldp/register.go` carries the same name).
- **OSPFv2 and OSPFv3 share code, and logic written for one was wrong in the
  other:** an OSPFv2 LSA body originated by the OSPFv3 engine, an SR flag read
  for IPv4 only (`plan/journal/guard-added-to-one-half-of-a-pair.md`).
- **A missing key chain ran the interface unauthenticated**, in OSPF and again
  in IS-IS (`plan/journal/silent-fall-through.md`).
- **A decoder without a default arm skipped unknown objects** that the RFC says
  to reject: RSVP-TE `DecodeMessage`, LDP `DecodeInit`, OSPF SR TLVs
  (`plan/journal/silent-fall-through.md`).
- **An absent mandatory field became zero and was used:** RSVP admitted an LSP
  at zero bandwidth, IS-IS turned an invalid three-way state into an action
  (`plan/journal/zero-value-as-valid-answer.md`).
- **An interop assertion had no way to pass** because it read an FRR command that
  FRR 10.3.1 does not have (`plan/journal/green-that-could-not-have-been-red.md`).
- **A transit LSP never came up** because the kernel refused an MTU metric on
  AF_MPLS (`plan/journal/kernel-refuses-what-the-installer-sends.md`).

## Gates

| Command | What it judges |
|---------|----------------|
| `./le test ospf`, `ospfv3`, `isis`, `ldp`, `rsvpte`, `bfd` | Functional `.ci` suites |
| `./le test wire ospf`, `./le test wire isis` | Wire-level decode suites |
| `./le test unit plugins` | Race-instrumented unit tests |
| `./le test integration interop` with `INTEROP_SCENARIO=<name>` | One FRR or BIRD scenario |
| `./le test integration interop-rsvpte` | The freeRouter RSVP-TE suite |
| `./le rfc check` | Requirement tags, quotes and discrimination records |
| `./le plugin boundary check` | No plain in-process call into engine state |
