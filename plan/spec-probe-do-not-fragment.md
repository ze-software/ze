# Spec: probe-do-not-fragment

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-11 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Ze's active probes cannot ask a question about packet size. `doPingCtx`
(`internal/component/ping/cmd/ping.go`) opens a bare ICMP socket with no control
function and no socket option, so every probe is emitted with the Don't Fragment
bit clear and is silently fragmented by the kernel when it exceeds the path.
`IP_PMTUDISC_PROBE`, `IP_MTU_DISCOVER` and `IP_RECVERR` appear nowhere in
`internal/`. The receive loop in `internal/component/ping/cmd/stream.go` discards
every datagram that is not an echo reply, so an ICMP error naming a next-hop MTU
is dropped before anything can read it.

The goal is one capability on the probe layer, reached by every prober: set the
DF bit, choose whether the kernel's path-MTU cache is honored or bypassed, and
receive the next-hop MTU a router reports. Linux already implements the RFC 1191
and RFC 8201 state machines and hands the reported MTU to a socket through its
error queue, so Ze asks the kernel rather than parsing ICMP errors itself
(owner decision, 2026-09-11). What Ze owes is the option it installs and the
value it reads back, and that is what this spec's tests assert.

This spec also absorbs `plan/spec-icmp-probe-privilege.md`, which is a skeleton
covering the doctor check for `CAP_NET_RAW` and the unprivileged `SOCK_DGRAM`
ICMP fallback. Both edit the same socket construction this spec rewrites, so
landing them separately means writing that construction twice.

`plan/spec-path-mtu-diagnostic.md` is the first consumer and depends on this one.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/diagnostics/active-probes.md` - the canonical page for the probe sockets this spec changes
  → Constraint: probes are in-daemon raw sockets with no shell-out, and `x/net/ipv4` supplies TTL control; a subprocess prober is not an option here
  → Decision: the page is SILENT on DF, on path MTU, and on ICMP error handling, which is the gap authorizing this work
  → Constraint: the page states "Time Exceeded verification is not done ... parsing is skipped on purpose", which DISAGREES with `doTracerouteCtx` (`internal/component/traceroute/cmd/traceroute.go`) and `stream.go`, both of which call `embeddedICMPOffset` and match the embedded id and seq. The sentence is repaired in this spec, not reported
- [ ] `docs/architecture/plugin/plugin-system.md` - cross-boundary value types
  → Constraint: a payload crossing a component or plugin boundary carries no pointer fields
- [ ] `ai/rules/platform-linux.md` - this spec is Linux-only code
  → Constraint: a QEMU integration test is mandatory and "needs hardware" is never a reason to skip one
- [ ] `docs/contributing/ze-go-style.md` - the working standard for the Go written here
  → Constraint: a typed numeric enum whose zero means `Unspecified`, so a DF mode that nobody set is never a valid state
  → Constraint: the caller passes the option at the call site rather than relying on a library default

### RFC Summaries (Scope: protocol)
- [ ] `rfc/full/rfc1191.txt` - Path MTU Discovery, the IPv4 state machine the kernel runs
  → Constraint: Section 3, "A host MUST never reduce its estimate of the Path MTU below 68 octets." This is a clamp on the estimate, NOT a rule to discard a message
  → Constraint: Section 3, a Datagram Too Big message carrying zero in the Next-Hop MTU field is how an unmodified router signals it cannot tell you, and a host MUST be able to deal with it. Discarding it discards the message that starts the search
  → Decision: Ze does not implement this state machine. Linux does, and `ai/rules/rfc-compliance.md` judges conformance on the behavior the whole stack produces
- [ ] `rfc/full/rfc8201.txt` - Path MTU Discovery for IPv6
  → Constraint: Section 4, a node receiving a Packet Too Big reporting less than the IPv6 minimum link MTU must discard it, and must not reduce its estimate below that minimum. Unlike IPv4, discard IS required here
  → Constraint: Section 1.1 states the document uses lowercase for the RFC 2119 words. A code comment quoting it must not silently upper-case them
- [ ] `rfc/full/rfc8200.txt` - IPv6
  → Constraint: Section 5, "IPv6 requires that every link in the Internet have an MTU of 1280 octets or greater." This is the authority for the 1280 floor, not RFC 8201
- [ ] `rfc/full/rfc791.txt` - IPv4
  → Constraint: "Every internet module must be able to forward a datagram of 68 octets without further fragmentation." 68 is the IPv4 number with normative force
  → Decision: RFC 791's 576 is a REASSEMBLY minimum, not a path-MTU floor, and RFC 1191 Section 5 disclaims its own use of 576 as "not part of the protocol specification". 576 may be a default; it must never be published as conformance
- [ ] `rfc/short/rfc792.md` - ICMP, already enrolled
  → Constraint: the summary states ICMP error messages are out of scope because "ze's ping path does not originate or process them". Reading the error queue makes that false, and the file is repaired in this spec

**Key insights:**
- The kernel, not Ze, implements PMTU discovery. Ze installs `IP_MTU_DISCOVER` and `IP_RECVERR`, then reads what comes back.
- The reported next-hop MTU arrives as a `SockExtendedErr` with `ee_errno` of `EMSGSIZE` and the MTU in `ee_info`, already matched by the kernel to the socket that sent the probe.
- Every constant this needs is in the vendored `golang.org/x/sys/unix`: `IP_MTU_DISCOVER`, `IP_PMTUDISC_DO`, `IP_PMTUDISC_PROBE`, `IP_MTU`, `IP_RECVERR`, `IPV6_RECVERR`, `IPV6_PMTUDISC_PROBE`, `MSG_ERRQUEUE`, `SO_EE_ORIGIN_ICMP`, `SO_EE_ORIGIN_ICMP6`, `SockExtendedErr`. No new dependency.
- `internal/component/bfd/transport/udp_linux.go` is the working precedent for `net.ListenConfig.Control` plus `unix.SetsockoptInt`, and for the `_linux.go` / `_other.go` split.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/ping/cmd/ping.go` - `doPingCtx` opens `lc.ListenPacket("ip4:icmp", bindAddr)` with no `Control` and no socket option; `pingOpts.size` feeds `probe.BuildICMPEcho`
- [ ] `internal/component/ping/cmd/stream.go` - the receive loop drops every datagram that is not an echo reply; `rb[6:8]` is read as the reply sequence
- [ ] `internal/component/traceroute/cmd/traceroute.go` - `embeddedICMPOffset` is the IHL-aware embedded-header parser, and the Type 3 arm reads only the port-unreachable code
- [ ] `internal/core/probe/icmp.go` - `BuildICMPEcho` is the whole of Ze's ICMP construction; no error message is built or parsed
- [ ] `internal/component/bfd/transport/udp_linux.go` - the sockopt precedent: `ListenConfig.Control` setting `IP_TTL`, `IP_RECVTTL`, `SO_BINDTODEVICE`
- [ ] `internal/core/privilege/check_linux.go` - `CheckPrivileges` parses `CapEff` for `CAP_NET_RAW`, prints a warning at startup, and gates nothing

**Behavior to preserve:**
- Every existing `show ping`, `monitor ping` and `traceroute` payload key and its shape. The DF option is additive and absent by default.
- The existing id and seq matching on echo replies, which is the only positive confirmation a probe arrived.
- `probe.BuildICMPEcho` and its callers.

**Behavior to change:**
- Probe sockets gain a DF mode and an error-queue reader.
- The ICMP socket construction gains an unprivileged `SOCK_DGRAM` fallback and a doctor check, absorbed from `plan/spec-icmp-probe-privilege.md`.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- An operator's `ping` or `traceroute` command carrying the new DF keyword, or a caller inside the daemon asking the probe layer to measure one size.
- A probe size in octets, and a DF mode.

### Transformation Path
1. The command handler resolves the DF keyword to the typed mode.
2. The probe layer opens its socket through `ListenConfig.Control` and sets `IP_MTU_DISCOVER` and `IP_RECVERR` for the mode.
3. `BuildICMPEcho` fills the payload to the requested size; the datagram is sent.
4. A reply is matched by id and seq, as today, and is the positive answer.
5. A failure is drained from the error queue: origin, `ee_errno`, and `ee_info` carrying the reported next-hop MTU.
6. The kernel's own current estimate for the destination is read back with `IP_MTU`.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Ze ↔ kernel | `setsockopt` on the probe socket, `recvmsg` with `MSG_ERRQUEUE` | No |
| Component ↔ CLI | the DF keyword on the existing ping and traceroute grammar | No |
| Component ↔ doctor | a registered check for the ICMP probe dependency | No |

### Integration Points
- `internal/core/probe` gains the socket construction; ping and traceroute call it instead of opening their own.
- `internal/core/diagnostic` gains the probe dependency's diagnostic code.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Linux delivers the reported next-hop MTU as `ee_info` on the socket error queue when a probe exceeds the path and DF is set | the documented `IP_RECVERR` interface; every constant is present in the vendored `x/sys/unix` | the whole design collapses and Ze must parse ICMP errors itself, restoring the work this spec deleted | a QEMU test that clamps a link, sends an oversized DF probe, and asserts the MTU read back equals the clamp | unvalidated |
| A-2 | `IP_PMTUDISC_PROBE` bypasses the kernel's cached PMTU, so a forced run measures the wire | the option is what `tracepath` uses for the same purpose | the `force` mode silently returns cached values and reports them as measured, which is the failure the mode exists to prevent | a QEMU test that poisons the cache with a wrong value, then asserts a forced probe disagrees with it | unvalidated |
| A-3 | The kernel has already matched the queued error to the socket that sent the probe, satisfying the RFC 8899 Section 4.6.1 validation MUST | the error queue is per-socket and the kernel demultiplexes on the quoted packet | Ze must validate the quoted packet itself, and an off-path forged PTB could steer a measurement | read the producing kernel path and assert in QEMU that an error for a different flow never appears on this socket | unvalidated |
| A-4 | An unprivileged `SOCK_DGRAM` ICMP socket accepts `IP_MTU_DISCOVER` and `IP_RECVERR` | the options are IP-level, not raw-socket-level | the unprivileged fallback cannot measure and the feature needs `CAP_NET_RAW` after all | a QEMU test running as a non-root user inside `ping_group_range` | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Darwin and any non-Linux build has no `IP_MTU_DISCOVER`, so the package must split by platform or fail to compile | the first `GOOS=darwin` build after the socket change | `_linux.go` / `_other.go`, copying `internal/component/bfd/transport/udp_linux.go`; the stub reports the capability as absent rather than returning a zero MTU |
| R-2 | A zero from the error queue is read as "no MTU reported" when it is in fact RFC 1191's old-router signal, which is the message that MUST start a search | a path with a pre-1990 router measures as unmeasurable rather than triggering the search | the reader returns the reported value and a separate "was reported" flag, never a bare integer; a zero is a distinct outcome with its own name |
| R-3 | The absorbed privilege work expands this spec past its own scope | the diff is mostly privilege plumbing and no probe can set DF yet | the DF capability lands first and is independently testable; the privilege fallback is its own phase and can be cut to its own spec if it grows |
| R-4 | Adding a DF keyword to ping and traceroute changes two shipped command grammars | a functional test for an existing ping invocation changes behavior | the keyword is additive and optional; the existing `.ci` tests are run unchanged before the new ones are written |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | `show ping` and `traceroute`, both shipped operator commands. A wrong socket option makes every probe fragment silently, which reports a fragment size as a path MTU: a figure that looks measured and is not |
| How is it reverted? | Single commit revert. No config migration, no persisted state, no peer-visible protocol change beyond the DF bit on a diagnostic packet |
| Who else touches this path? | `plan/spec-path-mtu-diagnostic.md` is the first consumer. `plan/spec-icmp-probe-privilege.md` is absorbed here and is deleted by this spec's closure, not left to collide |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ping <address> do-not-fragment` from the CLI | → | the probe socket sets `IP_MTU_DISCOVER` | `TestPingDoNotFragmentReachesTheSocketOption` |
| An oversized DF probe on a clamped link | → | the error-queue reader returns the reported MTU | `TestProbeErrorQueueReportsNextHopMTU` |
| Daemon start with no `CAP_NET_RAW` | → | the registered doctor check | `TestDoctorICMPProbeCheckReportsMissingCapability` |
| `traceroute <address> do-not-fragment` | → | the same probe socket construction | `TestTracerouteDoNotFragmentReachesTheSocketOption` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A probe is requested with DF off | the datagram is emitted with the DF bit clear, exactly as today, and every existing payload key is unchanged |
| AC-2 | A probe is requested with DF set and the size fits the path | an echo reply is matched by id and seq and the probe reports success |
| AC-3 | A probe is requested with DF set and the size exceeds the path, and a router reports a next-hop MTU | the probe reports failure, the reported MTU, and that a value WAS reported |
| AC-4 | The same, but the router reports zero in the next-hop MTU field | the probe reports failure and that NO usable value was reported, as a distinct outcome from AC-3; the zero is never returned as an MTU |
| AC-5 | A probe is requested in bypass mode against a destination whose cached PMTU is deliberately wrong | the figure measured disagrees with the cached value, proving the cache was bypassed |
| AC-6 | The kernel's current estimate for a destination is requested | the value the kernel holds is returned, and its absence is distinguishable from a value of zero |
| AC-7 | The daemon runs without `CAP_NET_RAW` and the caller's group is inside `ping_group_range` | an unprivileged `SOCK_DGRAM` ICMP socket is opened and can still set the DF mode and read the error queue |
| AC-8 | The daemon runs without `CAP_NET_RAW` and outside `ping_group_range` | the registered doctor check reports the missing dependency with its own diagnostic code, rather than a socket error at the moment of use |
| AC-9 | The package is built for a non-Linux target | it compiles, and the capability reports itself absent rather than returning a plausible zero |
| AC-10 | `rfc/short/rfc792.md` after this work | its scope paragraph and its Meta enrolment reason no longer claim Ze does not process ICMP error messages, and `./le rfc index-update` has regenerated `rfc/enrolled.txt` from it |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | runs `ping <address> size 1500 do-not-fragment` on a path clamped to 1400 | CLI → probe socket with `IP_MTU_DISCOVER` → error queue → payload naming the reported MTU | `test-ping-do-not-fragment-reports-mtu` |
| 2 | runs the same on a box with no `CAP_NET_RAW` | CLI → unprivileged `SOCK_DGRAM` socket → same answer | `test-ping-do-not-fragment-unprivileged` |
| 3 | runs `doctor` on a box that can open no ICMP socket at all | doctor registry → probe dependency check → named diagnostic code | `test-doctor-icmp-probe-missing` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestDFModeZeroIsUnspecified` | `internal/core/probe/df_test.go` | the typed mode's zero value is not a valid DF setting | |
| `TestReportedMTUZeroIsNotAValue` | `internal/core/probe/errqueue_test.go` | a zero next-hop MTU is returned as "no value reported", never as an MTU | |
| `TestReportedMTUBelowIPv6MinimumIsDiscarded` | `internal/core/probe/errqueue_test.go` | RFC 8201 Section 4: a value below 1280 on an IPv6 path is discarded | |
| `TestReportedMTUBelowSixtyEightIsNotDiscardedOnIPv4` | `internal/core/probe/errqueue_test.go` | RFC 1191 Section 3 clamps the estimate and does not discard the message | |
| `TestProbeCapabilityAbsentOffLinux` | `internal/core/probe/probe_other_test.go` | the non-Linux stub reports absence rather than a zero | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| probe payload size | 1-65507 | 65507 | 0 | 65508 |
| reported next-hop MTU, IPv4 | 68-65535 | 68 | 67 | N/A |
| reported next-hop MTU, IPv6 | 1280-65535 | 1280 | 1279 | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-ping-do-not-fragment-reports-mtu` | `test/plugin/*.ci` | an operator pings with DF set over a clamped link and is told the reported MTU | |
| `test-ping-do-not-fragment-unprivileged` | `test/plugin/*.ci` | the same, with no `CAP_NET_RAW` | |
| `test-doctor-icmp-probe-missing` | `test/plugin/*.ci` | doctor names the missing probe dependency by its diagnostic code | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `probe-df-clamped-path` | `test/interop/scenarios/` | a Linux router with a deliberately clamped link MTU | a DF probe larger than the clamp is refused and the reported MTU equals the clamp, observed on the wire rather than from Ze's own report | |

## Files to Modify
- `internal/component/ping/cmd/ping.go` - the socket construction moves to the probe layer and takes a DF mode
- `internal/component/ping/cmd/stream.go` - the receive loop gains the error-queue drain beside the echo-reply match
- `internal/component/traceroute/cmd/traceroute.go` - the same socket construction, and the DF keyword
- `internal/core/probe/icmp.go` - the probe layer gains the socket construction it does not own today
- `internal/core/privilege/check_linux.go` - the `CAP_NET_RAW` result gates the socket choice instead of only printing
- `internal/core/diagnostic/codes.go` - the probe dependency's diagnostic code
- `internal/plugins/ping-cmd/yang/ze-ping-cmd.yang` - the DF keyword on the ping grammar
- `internal/plugins/traceroute-cmd/yang/ze-traceroute-cmd.yang` - the DF keyword on the traceroute grammar
- `docs/architecture/diagnostics/active-probes.md` - DF, path MTU and the error queue, and the repair of the false Time Exceeded sentence. Declared by the `// Design:` header of `internal/component/ping/cmd/ping.go` and `internal/component/traceroute/cmd/traceroute.go`
- `docs/architecture/api/commands.md` - declared by the `// Design:` header of `internal/component/ping/cmd/stream.go` and `internal/core/probe/icmp.go`. The ping payload gains the reported MTU, so this page changes
- `docs/architecture/system-architecture.md` - declared by the `// Design:` header of `internal/core/privilege/check_linux.go`. The privilege result stops being advisory and starts choosing the socket, which is a behavior this page describes
- `docs/features/ai-first.md` - declared by the `// Design:` header of `internal/core/diagnostic/codes.go`. Verify at implementation whether it enumerates the codes or only describes the mechanism; an enumeration gains the probe code, a description is named here as unaffected
- `rfc/short/rfc792.md` - the scope paragraph and the Meta enrolment reason
- `docs/guide/command-reference.md` - the new keyword on two shipped commands
- `plan/spec-icmp-probe-privilege.md` - deleted at closure, absorbed here

## Files to Create
- `internal/core/probe/socket_linux.go` - the DF mode, the error-queue reader, and the kernel estimate
- `internal/core/probe/socket_other.go` - the stub that reports absence
- `internal/core/probe/doctor.go` - the registered probe dependency check
- `test/interop/scenarios/probe-df-clamped-path/` - the clamped-link scenario

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/plugins/ping-cmd/yang/ze-ping-cmd.yang` and `internal/plugins/traceroute-cmd/yang/ze-traceroute-cmd.yang` for the DF keyword |
| YANG validation constraints | Yes | the DF keyword is an `enumeration`, so the grammar rejects an unknown mode before a handler sees it |
| YANG custom validators | N-A | an enumeration needs no `ze:validate`, and its completion is automatic |
| CLI commands/flags | Yes | the existing ping and traceroute handlers, no new command |
| CLI grammar (keyword before value) | Yes | `do-not-fragment` is a bare keyword and takes no value, so the rule is satisfied by construction |
| Editor autocomplete | Yes | automatic for a YANG enumeration leaf |
| Functional test for new RPC/API | Yes | `test/plugin/*.ci`, three scenarios listed above |
| Pipe completeness | Yes | the reported MTU is a payload field, so the existing ping and traceroute pipe handling renders it unchanged |
| Env var registration | N-A | no leaf under `environment/` |
| Doctor check for runtime dependencies | Yes | the ICMP probe socket is a runtime dependency: `internal/core/probe/doctor.go` plus a code in `internal/core/diagnostic/codes.go`, with unit and functional tests. This is the surface `plan/spec-icmp-probe-privilege.md` existed to add |
| Prometheus counters/metrics | N-A | a diagnostic probe is operator-invoked and holds no continuous state worth a counter |
| BGP family surface (new SAFI / capability / attribute) | N-A | no BGP surface is touched |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` |
| 2 | Config syntax changed? | N-A | no configuration leaf is added; the keyword is operational |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md`, ping and traceroute |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md`, the payload gains the reported MTU |
| 5 | Plugin added/changed? | N-A | no plugin is added; the YANG halves already exist |
| 6 | Has a user guide page? | Yes | `docs/architecture/diagnostics/active-probes.md` is the owning page |
| 7 | Wire format changed? | N-A | Ze constructs no new message; the DF bit is an IP header flag the kernel sets |
| 8 | Plugin SDK/protocol changed? | N-A | no SDK surface changes |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc792.md` loses its false out-of-scope claim. RFC 1191 and RFC 8201 are NOT enrolled: the kernel implements them and Ze asserts the option it installs (owner decision, 2026-09-11) |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md`, the clamped-link interop scenario is a new fixture shape |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md`, DF probing is a capability other daemons list |
| 12 | Internal architecture changed? | Yes | `docs/architecture/diagnostics/active-probes.md`, which is also where the false Time Exceeded sentence is repaired |
| 13 | Route metadata keys added/changed? | N-A | no route metadata |
| 14 | Prometheus counters added/changed? | N-A | none added |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | a doctor check is registered: `docs/guide/status.md` |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED: run `./le spec citation anchors spec plan/spec-probe-do-not-fragment.md` and name every result here before implementation closes |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | verify the ping and traceroute examples in `docs/guide/command-reference.md` against the changed grammar |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the DF keyword reaches a socket option
   - Tests: `TestPingDoNotFragmentReachesTheSocketOption`, `TestTracerouteDoNotFragmentReachesTheSocketOption`
   - Files: the two YANG grammars, the ping and traceroute handlers, `internal/core/probe/socket_linux.go` as a stub
   - Verify: the wiring test fails because the option is never set
2. **Phase: DF mode and the platform split**
   - Tests: `TestDFModeZeroIsUnspecified`, `TestProbeCapabilityAbsentOffLinux`
   - Files: `socket_linux.go`, `socket_other.go`
   - Verify: a Linux build sets the option, a darwin build compiles and reports absence
3. **Phase: the error queue**
   - Tests: `TestReportedMTUZeroIsNotAValue`, `TestReportedMTUBelowIPv6MinimumIsDiscarded`, `TestReportedMTUBelowSixtyEightIsNotDiscardedOnIPv4`, `TestProbeErrorQueueReportsNextHopMTU`
   - Files: `socket_linux.go`, `internal/component/ping/cmd/stream.go`
   - Verify: A-1 and A-3 are validated in QEMU against a clamped link, not asserted
4. **Phase: bypass mode**
   - Tests: the AC-5 cache-poisoning test
   - Files: `socket_linux.go`
   - Verify: A-2 is validated by disagreement with a deliberately wrong cached value
5. **Phase: privilege, absorbed from `plan/spec-icmp-probe-privilege.md`**
   - Tests: `TestDoctorICMPProbeCheckReportsMissingCapability`, `test-ping-do-not-fragment-unprivileged`, `test-doctor-icmp-probe-missing`
   - Files: `internal/core/probe/doctor.go`, `internal/core/privilege/check_linux.go`, `internal/core/diagnostic/codes.go`
   - Verify: A-4 is validated by running as a non-root user in QEMU
6. **Phase: the pages and the ledger**
   - Tests: `./le rfc check` after `./le rfc index-update`
   - Files: `docs/architecture/diagnostics/active-probes.md`, `rfc/short/rfc792.md`, and every row named at Documentation row 16
   - Verify: the false Time Exceeded sentence and the false ICMP out-of-scope claim are both gone, and `rfc/enrolled.txt` is regenerated rather than hand-edited

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness | A reported MTU of zero is a distinct outcome everywhere it is handled, and is never returned as a number. The IPv4 and IPv6 floor rules are different and neither is applied to the other family |
| Naming | The DF keyword is the same word on ping and on traceroute. The payload key naming the reported MTU is kebab-case and identical on both commands |
| Data flow | The socket construction exists once, in the probe layer. Neither ping nor traceroute opens its own socket after this work |
| Rule: `ai/rules/principles.md` | The absence of a reported MTU, the presence of a zero, and a real value are three outcomes with three names. No caller can read a zero as an answer |
| Rule: `ai/rules/platform-linux.md` | Every kernel-behavior assumption is validated by a QEMU test that observes the kernel, not by a unit test over Ze's own builder |
| Rule: `ai/rules/rfc-compliance.md` | Each test asserts the option Ze installs or the value Ze reads, which is what the stack-level ruling requires of a delegated obligation |

## Review Gate

<!-- Filled by /ze-review at implementation time, per .claude/rules/planning.md. -->

### Run 1
| Severity | Finding | File | Resolution |
|----------|---------|------|------------|

### Run 2
| Severity | Finding | File | Resolution |
|----------|---------|------|------------|

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| No probe opens its own socket | `grep -rn "ListenPacket" internal/component/ping internal/component/traceroute` returns only the probe-layer call |
| The DF option is really set | the QEMU test observes the DF flag on the wire, not Ze's own report |
| The non-Linux build compiles | `GOOS=darwin go build ./internal/core/probe/...` |
| The ledger is regenerated, not hand-edited | `./le rfc index-update` leaves `rfc/enrolled.txt` unchanged when re-run |
| The doctor code exists | `grep -n "icmp-probe" internal/core/diagnostic/codes.go` |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Input validation | A reported next-hop MTU arrives from the network. It is bounded by family before any arithmetic uses it, and a value outside the range is an outcome rather than a number |
| Resource exhaustion | The error queue is drained with a bound, so a host flooding ICMP errors cannot hold a probe goroutine in a loop |
| Authorization failing open | The unprivileged fallback MUST NOT be chosen silently when the raw socket fails for a reason other than privilege; a failure that is not a privilege failure stays a failure |
| Off-path forgery | A-3 covers it: if the kernel's per-socket matching does not satisfy RFC 8899 Section 4.6.1, Ze validates the quoted packet itself before believing a reported MTU |

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
| The kernel discovers the path MTU; Ze installs the option and reads the result | Ze parses ICMP Type 3 Code 4 and ICMPv6 Type 2 itself | Linux already runs the RFC 1191 and RFC 8201 state machines and has already matched the error to the sending socket. Writing a second implementation adds a parser, a validation obligation, and a divergence, and `ai/rules/rfc-compliance.md` counts a delegated obligation as met. Owner decision, 2026-09-11 |
| The privilege work is absorbed rather than sequenced | leave `plan/spec-icmp-probe-privilege.md` standing and land it after | both specs rewrite the same socket construction, so sequencing them means writing it twice and reviewing it twice |
| A reported MTU of zero is its own outcome | return zero and let callers treat it as absent | RFC 1191 Section 3 makes a zero the old-router signal that a search MUST begin. A caller that reads it as "no information" loses the one message that says to search |

## Known Limitations

- The probe is ICMP, so a path that treats UDP or ESP differently is not measured. `plan/spec-ike-padded-path-probe.md` carries the measurement that removes this limitation for a peer with a live IKE SA.
- RFC 1191 and RFC 8201 are not enrolled in the conformance ledger, because the kernel implements them (owner decision, 2026-09-11). If Ze ever implements its own PMTU state machine, they must be enrolled then.

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: validation rules, error conditions, state transitions, timer
constraints, message ordering, and every MUST/MUST NOT.

The enforcing code here is the family-dependent bound on a reported MTU: RFC 8201
Section 4 for the IPv6 discard, RFC 8200 Section 5 for the 1280 constant, RFC 1191
Section 3 for the IPv4 clamp and for the meaning of a zero, and RFC 791 for the 68
constant. RFC 8201 uses lowercase RFC 2119 words by its Section 1.1 convention, so
a comment quoting it keeps the lowercase.

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
- [ ] `./le verify worktree` passes. It runs every stage against a COMMIT in a throwaway worktree, which is the pre-commit gate (`ai/rules/git-safety.md`). An in-place `./le verify current` is void the moment the tree moves under it
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
