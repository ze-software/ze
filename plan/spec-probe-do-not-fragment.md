# Spec: probe-do-not-fragment

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Depends | - |
| Phase | 6/6 |
| Handoff | - |
| Updated | 2026-09-15 |

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
| Ze ↔ kernel | `setsockopt` on the probe socket, `recvmsg` with `MSG_ERRQUEUE` | Yes: `TestOpenICMPInstallsDFMode` reads the option back with `getsockopt`; `TestProbeErrorQueueReportsNextHopMTU` reads 1400 off the queue (root, native netns) |
| Component ↔ CLI | the DF keyword on the existing ping and traceroute grammar | Yes: `TestPingDoNotFragmentReachesTheSocketOption`, `TestTracerouteDoNotFragmentReachesTheSocketOption`; `test/plugin/ping-do-not-fragment-reports-mtu.ci` through the daemon |
| Component ↔ doctor | a registered check for the ICMP probe dependency | Yes: `TestDoctorICMPProbeCheckReportsMissingCapability` finds the check through `diagnostic.DoctorChecksForPhase`; `test/plugin/doctor-icmp-probe-missing.ci` reads the code from `ze doctor` |

### Integration Points
- `internal/core/probe` gains the socket construction; ping and traceroute call it instead of opening their own.
- `internal/core/diagnostic` gains the probe dependency's diagnostic code.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | CLI keyword → `parsePingArgs` / `parseTracerouteArgs` → `probe.DFModeOfValue` → `probe.OpenICMP` (the one socket construction) → `setDFOptions` → kernel → `DrainErrorQueue` → the payload keys. `grep -rn ListenPacket internal/component/ping internal/component/traceroute` returns nothing: every prober opens through `openProbeConn`, which wraps `probe.OpenICMP` (`ping.go`, `traceroute.go`) |
| No unintended coupling (components stay isolated) | Yes | `internal/core/probe` imports `internal/core/diagnostic` for the doctor check (`./le tier check` OK, precedent `core/dnsserver`); ping and traceroute import `probe` only; neither imports the other. The payload crossing the plugin boundary is `map[string]any` of scalars (`tooBigResult`, `writeRefusedHop`), no pointer fields |
| No duplicated functionality (extends existing, does not recreate) | Yes | the socket construction existed twice (`doPingCtx`, `doTracerouteCtx`) and now exists once (`probe.OpenICMP`); the PMTU state machine is the kernel's, Ze parses no ICMP error (`errqueue_linux.go` reads `SockExtendedErr`); `dfControl` copies `bfd/transport/udp_linux.go` rather than a new sockopt helper |
| Zero-copy preserved where applicable (refs, not copies) | Yes | `DrainErrorQueue` reads into a caller-owned `[ErrQueueDrainMax]` bounded drain, `parseExtendedErr` slices the control buffer; `drainRefusals` collects into a stack array of `ErrQueueDrainMax` entries; `BuildICMPEcho` and the receive buffers are unchanged. No `fmt.Sprintf` on a per-packet path (every `fmt.Errorf` in the three packages is on an open or parse failure) |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | no new command: the keyword is a leaf on four existing YANG commands (`ze-ping-cmd.yang`, `ze-traceroute-cmd.yang`), read by the handlers that already own them. The doctor check registers itself from `internal/core/probe/register.go` `init()` through `diagnostic.RegisterDoctorCheck` and is found by `DoctorChecksForPhase`; the two codes are entries in the `codes.go` registry that `ze explain` and `doctor` already read. The `.ci` fixture registers from `internal/test/fixture/register_ping_df.go` `init()` through `fixture.Register` |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | Yes | searched `internal/`, `cmd/`, `pkg/` (`.go`, `.yang`, `.json`, `.txt`, tests excluded) for `do-not-fragment`, `doctor-icmp-probe`, `doctor-icmp-probe-unprivileged`, `next-hop-mtu`, `next-hop-mtu-reported`, `too-big`. `do-not-fragment`: the two YANG files (the grammar, from which completion, validation and the wiki catalog derive), `probe/df.go` (`DFKeyword`, the one spelling the handlers switch on), `codes.go` (the prose of the unprivileged code), the fixture pair. `doctor-icmp-probe*`: `probe/doctor.go` and `codes.go` only, the code registry itself. `next-hop-mtu*`: `probe/errqueue.go` (`FieldNextHopMTU`, `FieldNextHopMTUReported`), consumed as constants by `ping/cmd/stream.go` and `traceroute/cmd/traceroute.go`, and the fixture. `too-big`: `ping/cmd/stream.go` (`statusTooBig`), the fixture, and two unrelated firewall hits (`ze-firewall-conf.yang` ICMP type name). No validator, seed map, runner, help string or completion table names any of them: the CLI reads the enumeration from YANG (`./le doc check verify` shows `do-not-fragment` with both values in the live catalog for all four commands) |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Linux delivers the reported next-hop MTU as `ee_info` on the socket error queue when a probe exceeds the path and DF is set | the documented `IP_RECVERR` interface; every constant is present in the vendored `x/sys/unix` | the whole design collapses and Ze must parse ICMP errors itself, restoring the work this spec deleted | a QEMU test that clamps a link, sends an oversized DF probe, and asserts the MTU read back equals the clamp | confirmed (2026-09-15, native netns run as root: `TestProbeErrorQueueReportsNextHopMTU` and `...IPv6` read one entry with `ee_info` 1400 from the router, quoting the probe; the ordinary read returned `EMSGSIZE` first) |
| A-2 | `IP_PMTUDISC_PROBE` bypasses the kernel's cached PMTU, so a forced run measures the wire | the option is what `tracepath` uses for the same purpose | the `force` mode silently returns cached values and reports them as measured, which is the failure the mode exists to prevent | a QEMU test that poisons the cache with a wrong value, then asserts a forced probe disagrees with it | confirmed (2026-09-15, `TestProbeBypassCacheDisagreesWithPoisonedCache`: cache poisoned to 1400, clamp lifted to 1500, `IP_MTU` read 1400, honor-cache send of 1478 octets refused with a local entry of 1400, bypass-cache send of 1478 answered) |
| A-3 | The kernel has already matched the queued error to the socket that sent the probe, satisfying the RFC 8899 Section 4.6.1 validation MUST | the error queue is per-socket and the kernel demultiplexes on the quoted packet | Ze must validate the quoted packet itself, and an off-path forged PTB could steer a measurement | read the producing kernel path and assert in QEMU that an error for a different flow never appears on this socket | broken (2026-09-15, `TestProbeErrorQueueAndAnotherFlow`: the first socket held 1 entry about the other socket's flow. `raw_icmp_error` in `net/ipv4/raw.c` matches a raw socket by protocol, bound address and connected address only, and a probe socket is unconnected because traceroute needs answers from every router. The "if wrong" column is what shipped: every entry carries the quoted echo header, and both receive loops match identifier and sequence before believing the value) |
| A-4 | An unprivileged `SOCK_DGRAM` ICMP socket accepts `IP_MTU_DISCOVER` and `IP_RECVERR` | the options are IP-level, not raw-socket-level | the unprivileged fallback cannot measure and the feature needs `CAP_NET_RAW` after all | a QEMU test running as a non-root user inside `ping_group_range` | confirmed (2026-09-15, native netns run as root, `TestProbeUnprivilegedSocketReportsNextHopMTU`: with `CAP_NET_RAW` dropped from the thread and `ping_group_range` set to admit the process group, `OpenICMP` answered the datagram kind for both DF modes, the 1500-octet probe was refused at the 1400 clamp, the ordinary read woke with `EMSGSIZE`, and the queue held one entry reporting 1400 from the router, quoting the kernel-assigned identifier `Socket.Identifier` where Ze had written 0x1111. `TestProbeUnprivilegedSocketNeedsThePingGroupRange`: with the range at the disabled default both kinds are refused and the error carries EPERM and EACCES. `TestProbeUnprivilegedSocketIgnoresAnotherFlow`: the datagram kind's queue held 0 entries about another socket's flow, so the kernel matches per identifier there where A-3 found it does not on the raw kind) |

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
| `show ping <address> do-not-fragment honor-cache` from the CLI | → | the probe socket sets `IP_MTU_DISCOVER` | `TestPingDoNotFragmentReachesTheSocketOption` (`internal/component/ping/cmd/df_test.go`), with `TestPingDoNotFragmentBypassCacheReachesTheSocketOption` for the second value and `TestPingDoNotFragmentWithoutValueIsRefused` for the bare form |
| The same command, once its probes have run | → | `readPathMTU` (`probe.KernelPathMTU`) fills the summary's `path-mtu` (AC-6, wired at closure: the function had no non-test caller before) | `TestPingDoNotFragmentSummaryCarriesTheKernelEstimate` (`df_test.go`, through `handleShowPing`); through the daemon, `test/plugin/ping-do-not-fragment-reports-mtu.ci` asserts 1600 before the router answers and 1400 after |
| An oversized DF probe on a clamped link | → | the error-queue reader returns the reported MTU | `TestProbeErrorQueueReportsNextHopMTU` (`internal/core/probe/errqueue_integration_linux_test.go`, `integration && linux`) |
| Daemon start with no `CAP_NET_RAW` | → | the registered doctor check | `TestDoctorICMPProbeCheckReportsMissingCapability` (`internal/core/probe/doctor_test.go`) |
| `show traceroute <address> do-not-fragment honor-cache` | → | the same probe socket construction | `TestTracerouteDoNotFragmentReachesTheSocketOption` (`internal/component/traceroute/cmd/df_test.go`), with `TestTracerouteDoNotFragmentBypassCacheReachesTheSocketOption` and `TestTracerouteDoNotFragmentWithoutValueIsRefused` |

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
| 1 | runs `show ping <address> size 1500 do-not-fragment honor-cache` on a path clamped to 1400 | CLI → probe socket with `IP_MTU_DISCOVER` → error queue → payload naming the reported MTU | `test/plugin/ping-do-not-fragment-reports-mtu.ci` (fixture `plugin/ping-do-not-fragment`, both values) |
| 2 | runs the same on a box with no `CAP_NET_RAW` | CLI → unprivileged `SOCK_DGRAM` socket → same answer | `test/plugin/ping-do-not-fragment-unprivileged.ci` (same fixture, `capsh --drop=cap_net_raw`) |
| 3 | runs `doctor` on a box that can open no ICMP socket at all | doctor registry → probe dependency check → named diagnostic code | `test/plugin/doctor-icmp-probe-missing.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestDFModeZeroIsUnspecified`, `TestDFModeOfValue` | `internal/core/probe/df_test.go` | the typed mode's zero value is not a valid DF setting; each value word maps to its mode and any other word, the empty one included, is refused with `ErrDFValueUnknown` | green (2026-09-15) |
| `TestReportedMTUZeroIsNotAValue` | `internal/core/probe/errqueue_linux_test.go` | a zero next-hop MTU is returned as "no value reported", never as an MTU | red under a `// MUTATION-APPLIED` cut of `classifyReportedMTU`, green restored (2026-09-15) |
| `TestReportedMTUBelowIPv6MinimumIsDiscarded` | `internal/core/probe/errqueue_linux_test.go` | RFC 8201 Section 4: a value below 1280 on an IPv6 path is discarded | red under the same cut, green restored (2026-09-15) |
| `TestReportedMTUBelowSixtyEightIsNotDiscardedOnIPv4` | `internal/core/probe/errqueue_linux_test.go` | RFC 1191 Section 3 clamps the estimate and does not discard the message | red under the same cut, green restored (2026-09-15) |
| `TestProbeCapabilityAbsentOffLinux`, `TestErrorQueueAbsentOffLinux`, `TestKernelPathMTUAbsentOffLinux` | `internal/core/probe/probe_other_test.go` | the non-Linux stub reports absence rather than a zero | compiles under `GOOS=darwin go vet`; a darwin host run is owed |
| `TestPingDoNotFragmentReachesTheSocketOption`, `...BypassCache...`, `TestPingWithoutDoNotFragmentOpensWithDFOff`, `TestPingDoNotFragmentWithoutValueIsRefused` | `internal/component/ping/cmd/df_test.go` | wiring row 1, AC-1 at the constructor, and the bare keyword or an unknown value refused before any socket opens | red before the keyword parse (`socket constructor received DF mode off, want honor-cache`), green after (2026-09-15) |
| `TestTracerouteDoNotFragmentReachesTheSocketOption`, `...BypassCache...`, `TestTracerouteWithoutDoNotFragmentOpensWithDFOff`, `TestTracerouteDoNotFragmentWithoutValueIsRefused` | `internal/component/traceroute/cmd/df_test.go` | wiring row 4 and the same refusals on the traceroute grammar | same red and green (2026-09-15) |
| `TestPingRefusedByPathReportsNextHopMTU`, `...WithZeroNextHopMTUReportsNoValue`, `...RefusalQuotingAnotherProbeIsIgnored`, `...RefusedAtSendReportsCachedEstimate`, `TestPingSummaryCountsAndReportsRefusals` | `internal/component/ping/cmd/errqueue_test.go` | AC-3, AC-4, the A-3 defense and the `too-big-cached` row through the session loop with a fake queue | green (2026-09-15); written after the loop code, no separate red |
| `TestTracerouteRefusedByPathRecordsNextHopMTU`, `...RefusedWithoutValueRecordsNoMTU`, `...RefusalQuotingAnotherProbeIsIgnored`, `...RefusedAtSendRecordsCachedEstimate` | `internal/component/traceroute/cmd/errqueue_test.go` | the same four outcomes on a traceroute hop | green (2026-09-15); written after the loop code, no separate red |
| `TestOpenICMPInstallsDFMode` | `internal/core/probe/socket_linux_test.go` | `getsockopt` reads back `IP_PMTUDISC_DONT`, `DO` and `PROBE` for the three modes | green as root (2026-09-15); skips without `CAP_NET_RAW` |
| `TestProbeErrorQueueReportsNextHopMTU`, `...IPv6`, `TestProbeBypassCacheDisagreesWithPoisonedCache`, `TestProbeErrorQueueAndAnotherFlow`, `TestProbeDFBitOnTheWire` | `internal/core/probe/errqueue_integration_linux_test.go` (`integration && linux`) | wiring row 2, A-1, A-2, A-3, AC-5, AC-6, and the DF bit read off an `AF_PACKET` capture | green natively as root (2026-09-15); the QEMU run is owed |
| `TestOpenICMPFallsBackOnlyOnPrivilegeRefusal` | `internal/core/probe/socket_test.go` | the Security Review guard: a raw refusal that is not EPERM or EACCES reaches no fallback | green (2026-09-15) |
| `TestOpenICMPFallsBackOnPrivilegeRefusal`, `TestOpenICMPNamesBothRefusals`, `TestSocketTranslatesTheDatagramAddress` | `internal/core/probe/socket_test.go` | the fallback opens on EPERM and EACCES with the opener's identifier; both refusals are wrapped and both fixes named; the datagram kind takes `*net.IPAddr` only | green (2026-09-15) |
| `TestDoctorICMPProbeCheckReportsMissingCapability` (wiring row 3), `...ReportsTheFallback`, `...SilentWithRawSocket`, `...ReportsANonPrivilegeRefusal` | `internal/core/probe/doctor_test.go` | the check is found through `diagnostic.DoctorChecksForPhase` and answers each outcome by code; the codes resolve through `diagnostic.Lookup` | red before `doctor.go` existed (build failed on the undefined check), green after (2026-09-15) |
| `TestTracerouteRefusesTheDatagramSocket` | `internal/component/traceroute/cmd/df_test.go` | `show traceroute` and `show probe-round` refuse the datagram kind by name and close it | green (2026-09-15) |
| `TestProbeUnprivilegedSocketReportsNextHopMTU`, `...NeedsThePingGroupRange`, `...FallbackIsNotTriedForOtherRefusals`, `...IgnoresAnotherFlow` | `internal/core/probe/privilege_integration_linux_test.go` (`integration && linux`) | A-4, AC-7, the guard against the live datagram opener, and per-identifier matching on the datagram kind, each against the kernel | green natively as root (2026-09-15); the QEMU run is owed |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| probe payload size | 1-65507 | 65507 | 0 | 65508 |
| reported next-hop MTU, IPv4 | 68-65535 | 68 | 67 | N/A |
| reported next-hop MTU, IPv6 | 1280-65535 | 1280 | 1279 | N/A |

Confirmed 2026-09-15: the payload size bound is the existing `maxPingSize` check in `parsePingArgs` (unchanged); 68 and 67 are `TestReportedMTUBelowSixtyEightIsNotDiscardedOnIPv4` (67 raised to 68, 68 kept, 1279 reported on IPv4); 1280 and 1279 are `TestReportedMTUBelowIPv6MinimumIsDiscarded`; a zero is `TestReportedMTUZeroIsNotAValue`.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ping-do-not-fragment-reports-mtu` | `test/plugin/ping-do-not-fragment-reports-mtu.ci` | an operator pings with DF set over a clamped link and is told the reported MTU | PASS natively as root (2026-09-15, `option=needs-linux:caps=net-admin,net-raw`); red under a cut of `pmtuDiscValue` (`reply status ok, want too-big`); the QEMU run is owed |
| `ping-do-not-fragment-unprivileged` | `test/plugin/ping-do-not-fragment-unprivileged.ci` | the same, with no `CAP_NET_RAW` | PASS natively as root (2026-09-15); red under a cut of `listenDatagramICMP`; the QEMU run is owed |
| `doctor-icmp-probe-missing` | `test/plugin/doctor-icmp-probe-missing.ci` | doctor names the missing probe dependency by its diagnostic code | PASS natively as root (2026-09-15, `caps=net-admin`); red under a cut of `checkICMPProbeSocket`; the QEMU run is owed |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `probe-df-clamped-path` | `internal/core/probe/errqueue_integration_linux_test.go` (`integration && linux`, run by `./le qemu all-tests`), not `test/interop/scenarios/` | a Linux router with a deliberately clamped link MTU, in the middle of three network namespaces | a DF probe larger than the clamp is refused and the reported MTU equals the clamp, observed on the wire rather than from Ze's own report | green natively as root on 2026-09-15; the QEMU run under the runtime kernel is owed |

## Files to Modify
- `internal/component/ping/cmd/ping.go` - the socket construction moves to the probe layer and takes a DF mode
- `internal/component/ping/cmd/stream.go` - the receive loop gains the error-queue drain beside the echo-reply match
- `internal/component/traceroute/cmd/traceroute.go` - the same socket construction, and the DF keyword
- `internal/core/probe/icmp.go` - the probe layer gains the socket construction it does not own today
- `internal/core/privilege/check_linux.go` - left untouched (phase 5 decision): the socket choice reads the kernel's refusal at open, not a cached `CheckPrivileges` result, see Design Insights
- `internal/core/diagnostic/codes.go` - the probe dependency's diagnostic code
- `internal/plugins/ping-cmd/yang/ze-ping-cmd.yang` - the DF keyword on the ping grammar
- `internal/plugins/traceroute-cmd/yang/ze-traceroute-cmd.yang` - the DF keyword on the traceroute grammar
- `docs/architecture/diagnostics/active-probes.md` - DF, path MTU and the error queue, and the repair of the false Time Exceeded sentence. Declared by the `// Design:` header of `internal/component/ping/cmd/ping.go` and `internal/component/traceroute/cmd/traceroute.go`
- `docs/architecture/api/commands.md` - declared by the `// Design:` header of `internal/component/ping/cmd/stream.go` and `internal/core/probe/icmp.go`. The ping payload gains the reported MTU, so this page changes
- `docs/architecture/system-architecture.md` - unaffected (phase 5): `check_linux.go` is untouched and the page describes privilege dropping under `ze.user`, which this work does not change
- `docs/features/ai-first.md` - unaffected (phase 5): it describes the doctor mechanism and enumerates no code (`grep doctor-vrrp-raw-socket` finds none there); the two probe codes live in `internal/core/diagnostic/codes.go` and `docs/architecture/diagnostics/active-probes.md`
- `rfc/short/rfc792.md` - the scope paragraph and the Meta enrolment reason
- `docs/guide/command-reference.md` - the new keyword on two shipped commands
- `plan/spec-icmp-probe-privilege.md` - deleted at closure, absorbed here

## Files to Create
- `internal/core/probe/socket_linux.go` - the DF mode, the error-queue reader, and the kernel estimate
- `internal/core/probe/socket_other.go` - the stub that reports absence
- `internal/core/probe/doctor.go` - the registered probe dependency check
- `test/interop/scenarios/probe-df-clamped-path/` - the clamped-link scenario. Landed as `internal/core/probe/errqueue_integration_linux_test.go` instead, for the reason in Design Insights: that directory is the BGP interop suite's
- `internal/core/probe/df.go`, `errqueue.go`, `errqueue_linux.go`, `errqueue_other.go`, `register.go` - the mode, the error-queue reader and its stubs, the doctor registration (split out of `socket_linux.go` as the work grew)
- `internal/test/fixture/plugin_fixture_ping_df.go`, `register_ping_df.go`, and the three `test/plugin/*.ci` in Functional Tests

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | done: `leaf do-not-fragment { type enumeration { honor-cache; bypass-cache } }` on `show ping`, `resolve ping` (`internal/plugins/ping-cmd/yang/ze-ping-cmd.yang`) and `show traceroute`, `resolve traceroute` (`internal/plugins/traceroute-cmd/yang/ze-traceroute-cmd.yang`), revision 2026-09-15; `./le yang glue check`: 154 directories current |
| YANG validation constraints | Yes | done: the enumeration refuses an unknown value at the RPC layer (`invalid value "count", expected one of: honor-cache, bypass-cache`, observed through the daemon in package D), and the handlers refuse it again (`TestPingDoNotFragmentWithoutValueIsRefused`, `TestTracerouteDoNotFragmentWithoutValueIsRefused`) |
| YANG custom validators | N-A | an enumeration needs no `ze:validate`, and its completion is automatic |
| CLI commands/flags | Yes | done: the four existing handlers (`parsePingArgs`, `handleResolvePing`, `parseTracerouteArgs`, `parseResolveTracerouteArgs`) read the keyword; no new command |
| CLI grammar (keyword before value) | Yes | done: `do-not-fragment` is a keyword followed by one value from a closed set, `honor-cache` or `bypass-cache`, on every grammar that carries it. The bare form the design named was refused by the RPC layer and replaced (Key Design Decisions); `args[0]` on every command is still the target, as before |
| Editor autocomplete | Yes | done: automatic for a YANG enumeration leaf; the live catalog lists both values under `do-not-fragment` for all four commands (`./le doc check verify`, 2026-09-15) |
| Functional test for new RPC/API | Yes | done: the three `test/plugin/*.ci` in Functional Tests, fixture `plugin/ping-do-not-fragment` (`internal/test/fixture/plugin_fixture_ping_df.go`) |
| Pipe completeness | Yes | done: `next-hop-mtu`, `next-hop-mtu-reported` and the `too-big` statuses are fields of the existing reply rows and summary (`tooBigResult`, `summarizePingReplies`, `writeRefusedHop`), so `\| json`, `\| yaml` and `\| table` render them through the unchanged `ApplyPipes` path |
| Env var registration | N-A | no leaf under `environment/` |
| Doctor check for runtime dependencies | Yes | done: `checkICMPProbeSocket` (`internal/core/probe/doctor.go`), registered from `register.go`; codes `doctor-icmp-probe` and `doctor-icmp-probe-unprivileged` in `internal/core/diagnostic/codes.go`; `doctor_test.go` and `test/plugin/doctor-icmp-probe-missing.ci`. This is the surface `plan/spec-icmp-probe-privilege.md` existed to add |
| Prometheus counters/metrics | N-A | a diagnostic probe is operator-invoked and holds no continuous state worth a counter |
| BGP family surface (new SAFI / capability / attribute) | N-A | no BGP surface is touched |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | done: `docs/features.md`, row "Don't Fragment probes" after Core Diagnostics, with a source anchor (package D) |
| 2 | Config syntax changed? | N-A | no configuration leaf is added; the keyword is operational |
| 3 | CLI command added/changed? | Yes | done: `docs/guide/command-reference.md`, the `do-not-fragment honor-cache` examples on `show ping` and `show traceroute`, the keyword paragraph (two values, bare form refused), the payload paragraphs, and the "Requires CAP_NET_RAW" sentences replaced by the fallback paragraph (packages A, B, C, D) |
| 4 | API/RPC added/changed? | Yes | done: `docs/architecture/api/commands.md`, the ping and traceroute payload rows carry `next-hop-mtu` and `next-hop-mtu-reported` (package B) |
| 5 | Plugin added/changed? | N-A | no plugin is added; the YANG halves already exist |
| 6 | Has a user guide page? | Yes | done: `docs/architecture/diagnostics/active-probes.md`, sections "The Don't Fragment mode", "The error queue", "Bounds and privileges", "Reply matching" (packages A, B, C, D) |
| 7 | Wire format changed? | N-A | Ze constructs no new message; the DF bit is an IP header flag the kernel sets |
| 8 | Plugin SDK/protocol changed? | N-A | no SDK surface changes |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | done: `rfc/short/rfc792.md` scope paragraph and Meta enrolment reason rewritten (package D), `./le rfc index-update` regenerated the derived files, which stay untracked. RFC 1191 and RFC 8201 are NOT enrolled: the kernel implements them and Ze asserts the option it installs (owner decision, 2026-09-11) |
| 10 | Test infrastructure changed? | Yes | done: `docs/functional-tests.md`, subsection "A clamped path built by the test itself" after the caps table (package D) |
| 11 | Affects daemon comparison? | Yes | done: `docs/comparison.md`, Operations row "Don't Fragment path probing" (package D) |
| 12 | Internal architecture changed? | Yes | done: `docs/architecture/diagnostics/active-probes.md`, the false Time Exceeded sentence replaced by what `embeddedICMPOffset` and both loops do (package A) |
| 13 | Route metadata keys added/changed? | N-A | no route metadata |
| 14 | Prometheus counters added/changed? | N-A | none added |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | done: `docs/guide/status.md`, Infrastructure row "ICMP probe socket check" (package D). The shipping wiki catalog `../wiki/command-catalog.md` regenerated with `./le --name df wiki-catalog update` (the shared `bin/le` embedded the pre-edit YANG); `./le doc check verify` no longer reports the wiki drift |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | done: `./le spec citation anchors spec plan/spec-probe-do-not-fragment.md` named three pages that reach `codes.go` only (`as112-coordination`, `redistribution`, `vpp`), none of which describes the probe codes, so none owed; `./le docs-to-code index-check` reports two stale anchors (`text-format.md` `FamilyIPv4Unicast`, `formatting.md` `validCLIFormats`) that predate this spec and name no file it touched |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | done: the ping and traceroute examples in `docs/guide/command-reference.md` spell `do-not-fragment honor-cache`; the published site under `../gh-pages` (`reference/cli/`, `llms.txt`, `data/cli-commands.json`) is regenerated by `./le site build` at publish and still lists the four usages without the keyword (`./le --name df doc check verify`, 2026-09-15) |

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

Run inline by the closure agent (2026-09-15), a context that wrote none of the
diff, over the whole uncommitted diff of the file list in the Deliverables
Checklist. Pre-checks: `./le commit audit` clean (4 test files, no weakening);
`./le repository check` named `DrainErrorQueue` as exported with no
cross-package caller (fixed below) and four traceroute exports that predate
this spec (`HandleProbeRound`, `StreamProbeRound`, `SetTTL`), not this spec's.

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/probe-do-not-fragment-e9e5e97a-9494-4074-a088-c3bd059b2fb3.md` (55 files, verdict clean; the spec itself is not in the list, so this table could be written after the record) |
| `review check` | clean |
| Rounds | 2 |
| Reviewer lenses used | round 1: logic and wiring (every new exported symbol grepped for a non-test caller, the two receive loops and the drain read as producers), security and edge cases (bounds, the fallback guard, the reported-MTU floors, the `As4` panic path traced to `FamilyOf` on the source address) plus the style pass over every changed Go file; round 2: the three fixes and the call sites they touched |

### Run 1 (scope: the whole diff)
| Severity | Finding | File | Resolution |
|----------|---------|------|------------|
| BLOCKER | `probe.KernelPathMTU` had no non-test caller: AC-6 was a library function reached by nothing an operator types | `internal/core/probe/errqueue_linux.go` | wired: `doPingCtx` reads it after a DF batch through the `readPathMTU` seam and writes `path-mtu` on the summary; absent when the kernel holds no estimate. `TestPingDoNotFragmentSummaryCarriesTheKernelEstimate`; the DF `.ci` fixture asserts 1600 then 1400 through the daemon |
| ISSUE | the refused-send branch drained the error queue for its own LOCAL entry and discarded every other entry, so a router's answer for probe N queued beside the cache's refusal of probe N+1 was lost and probe N timed out | `internal/component/ping/cmd/stream.go` `runPingSession` | one `drainQueue` for both kinds, called by the sender and by the receiver's wake; `TestPingRefusedSendKeepsTheRouterAnswerAhead`; journal row in `plan/journal/error-path-discards-data-already-received.md` |
| ISSUE | `DrainErrorQueue` exported with its only caller inside the package (`Socket.DrainErrors`) | `internal/core/probe/errqueue_linux.go`, `errqueue_other.go` | unexported as `drainErrorQueue`; the two pages and `rfc/short/rfc792.md` renamed with it |
| NOTE | `drainErrorQueue` allocates its 1500-octet read buffer and the control buffer per drain | `errqueue_linux.go` | left: a drain runs once per refusal on a diagnostic path, not per packet |
| NOTE | the `.ci` fixture asserted the reported MTU but not the kernel estimate | `internal/test/fixture/plugin_fixture_ping_df.go` | `pingDFPathMTU` added with the AC-6 wiring |

### Run 2 (scope: the three fixes above and the call sites they touched)
| Severity | Finding | File | Resolution |
|----------|---------|------|------------|
| none | `go test -race` over the three packages green (`close-pkgs.log`), darwin and integration vets green, the two DF `.ci` PASS as root on the raw and the datagram socket with the new assertions (`close-df-ci.log`), `ip netns list` clean after | | |

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

- The interop scenario lives as the integration package rather than under `test/interop/scenarios/`: that directory is discovered by the BGP interop suite (`interoplab.Discover`) and every entry there needs a BGP checker, while the peer here is the Linux router in the middle namespace and the assertion is a socket read. The package is named in `integrationPackages` (`internal/le/qemu/alltests.go`) so `./le qemu all-tests` runs it.
- `DFOff` is an option, not the absence of one. `TestProbeDFBitOnTheWire` read the DF flag off an `AF_PACKET` capture on the router's link and found it SET on a probe opened with no option: Linux's default `IP_PMTUDISC_WANT` sets DF on every datagram that fits the path. So `DFOff` installs `IP_PMTUDISC_DONT`, and AC-1's "DF bit clear" is now true where "exactly as today" was not.
- The kernel matches a queued ICMP error to an unconnected raw socket by protocol only (A-3 broken), so the reader hands over the quoted echo header and both receive loops match identifier and sequence before believing the value. Connecting the socket would let the kernel match per flow but would stop a traceroute socket receiving Time Exceeded from intermediate routers.
- A refusal from the network wakes an ordinary read with `EMSGSIZE` once (`raw_err` sets `sk_err`), observed in every integration run; a refusal at send does not (`ip_local_error` queues without `sk_err`). The ping session therefore has one drainer, its main goroutine: the receiver signals on the read error, and the sender drains right after its own failed write, so the two never race for one entry.
- `KernelPathMTU` reads `IP_MTU` off a throwaway connected UDP socket: the option answers only on a socket holding a route, the estimate belongs to the route rather than the socket, and UDP connect needs no privilege and sends nothing.
- The socket choice reads the kernel, not `CheckPrivileges`. `OpenICMP` tries the raw socket and falls back on `EPERM` or `EACCES` (`privilegeRefused`, the named guard); a cached capability result would be a second declaration of a fact the open already answers, and could disagree with it after a capability change. So `internal/core/privilege/check_linux.go` stays as it is: it warns at startup, and the doctor check reports the socket state by code.
- The unprivileged socket is opened with `unix.Socket` + `unix.Bind` + `net.FilePacketConn` (`listenDatagramICMP`), because `net.ListenConfig` knows no datagram ICMP network and `golang.org/x/net/icmp`, whose `udp4` endpoint is this socket, is not vendored. The kernel assigns the echo identifier at bind and rewrites the id field of every echo sent, so `OpenICMP` returns a `*probe.Socket` whose `Identifier` is what a reply or queued error is matched on, and every prober reads it there instead of computing `pid & 0xffff`. The raw kind's identifier is two random octets per socket, so two probers open at once no longer share one identifier and one sequence space.
- The error queue quotes the echo from its ICMP header on both kinds (`net/ipv4/ping.c ping_err` passes `(u8 *)icmph` to `ip_icmp_error`, as `raw_err` does), so `quotedEcho` needs no kind branch; the integration run confirmed it.
- Traceroute refuses the datagram kind (`openRawProbeConn`, `errTracerouteNeedsRawSocket`): a ping socket delivers only echo replies to the ordinary read (`ping_rcv`), so Time Exceeded never reaches the trace loops and every hop would time out, which is a silently wrong answer. Serving traceroute on the datagram kind means reading hop answers off the error queue in all three trace loops, with `IP_RECVERR` on for `DFOff` too; that is separable work this spec does not carry. `show ping`, `monitor ping` and `resolve ping` run on both kinds.
- `Socket` speaks `*net.IPAddr` on both kinds and translates to and from the `*net.UDPAddr` a datagram conn wants, so the probers' destination and source-address checks stay one type. The x/net TTL wrappers take the concrete conn from `Socket.PacketConn`, because `socket.NewConn` type-switches on `*net.IPConn` and `*net.UDPConn`.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| The kernel discovers the path MTU; Ze installs the option and reads the result | Ze parses ICMP Type 3 Code 4 and ICMPv6 Type 2 itself | Linux already runs the RFC 1191 and RFC 8201 state machines and has already matched the error to the sending socket. Writing a second implementation adds a parser, a validation obligation, and a divergence, and `ai/rules/rfc-compliance.md` counts a delegated obligation as met. Owner decision, 2026-09-11 |
| The privilege work is absorbed rather than sequenced | leave `plan/spec-icmp-probe-privilege.md` standing and land it after | both specs rewrite the same socket construction, so sequencing them means writing it twice and reviewing it twice |
| A reported MTU of zero is its own outcome | return zero and let callers treat it as absent | RFC 1191 Section 3 makes a zero the old-router signal that a search MUST begin. A caller that reads it as "no information" loses the one message that says to search |
| An IPv4 value below 68 is reported, raised to 68 | discard it like the IPv6 rule, or report 67 as read | RFC 1191 Section 3 clamps the estimate and discards no message, so the report stands; the value handed on is the floor the RFC sets (RFC 791's 68), so no consumer acts on a number the RFC forbids |
| Payload keys `next-hop-mtu` and `next-hop-mtu-reported`, statuses `too-big` and `too-big-cached` | one key with zero for absent; one status for both refusals | a zero is never an answer, so presence is a separate boolean; a refusal at send never reached the wire and carries the cache's estimate rather than a router's answer, which the operator needs to tell apart |
| The fallback is taken on `EPERM` or `EACCES` only | fall back on any raw failure; read `CheckPrivileges` first | a datagram socket opened over a non-privilege failure answers as if the raw one had, on a host whose real defect nobody was told about; the kernel's refusal at open is the fact, and a cached capability result is a copy of it |
| Two doctor codes, `doctor-icmp-probe` and `doctor-icmp-probe-unprivileged` | one code with two messages | "no probe can run" and "ping runs degraded, traceroute cannot" are two facts an operator acts on differently, and `ze explain` answers by code |
| Traceroute refuses the datagram kind by name | run the trace and let every hop time out; serve hops off the error queue | the timeout is a silently wrong path; the error-queue trace is three loops of separable work, named in Design Insights, not folded into the DF spec |
| The identifier is the socket's, `Socket.Identifier` | keep `pid & 0xffff` in every prober | the datagram kind's identifier is the kernel's choice, so the socket is the only place that knows it; making the raw kind per-socket too gives concurrent probers separate identifiers |
| `do-not-fragment` takes one value, `honor-cache` or `bypass-cache`, and the bare keyword is refused | the design's bare keyword meaning `honor-cache` with an optional value word; a YANG `default` the RPC layer would apply to a keyword with no value | the bare form was written in package A and never reached a handler: the RPC layer (`internal/component/plugin/server/command.go`, keyword-value extraction) reads every declared leaf as keyword-then-value, so a bare keyword before another keyword is refused with `invalid value "count", expected one of: honor-cache, bypass-cache` and a trailing one with `do-not-fragment requires a value`, and Ze's command grammar has no bare-keyword leaf type. Honoring a `default` there is a generic grammar feature nobody asked for. So the grammar is keyword plus value, like every other operational keyword; the handlers refuse the bare form the same way (`DFModeOfValue` answers `ErrDFValueUnknown`), so the offline local parsers agree with the daemon; the YANG descriptions, the pages and the tests spell the value form (main-thread decision, 2026-09-15) |

## Known Limitations

- The probe is ICMP, so a path that treats UDP or ESP differently is not measured. `plan/spec-ike-padded-path-probe.md` carries the measurement that removes this limitation for a peer with a live IKE SA.
- RFC 1191 and RFC 8201 are not enrolled in the conformance ledger, because the kernel implements them (owner decision, 2026-09-11). If Ze ever implements its own PMTU state machine, they must be enrolled then.
- Traceroute runs on the raw socket only. Without `CAP_NET_RAW`, `show traceroute`, `monitor traceroute` and `show probe-round` refuse by name (`errTracerouteNeedsRawSocket`, `openRawProbeConn`) and `doctor-icmp-probe-unprivileged` says so, because the kernel delivers Time Exceeded to a raw socket and a datagram ICMP socket would time out at every hop. Ping runs on both kinds. Serving hops off the error queue on the datagram kind is separable work the main thread has put to the owner.
- A failed read of the error queue is reported as "no value reported" (`next-hop-mtu-reported` false, or the probe timing out), the same answer as an empty queue: neither prober package holds a logger, so the read failure itself is not surfaced. The row is truthful, and nothing reads a zero as an MTU.

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
  The Unit Tests, Functional Tests and Interop Tests tables above name each one with its file.
- [ ] Tests FAIL (paste output)
  Wiring (package A, before the keyword parse):

  ```
  socket constructor received DF mode off, want honor-cache
  ```

  Error queue (package B, `// MUTATION-APPLIED` cut of `classifyReportedMTU`):

  ```
  --- FAIL: TestReportedMTUZeroIsNotAValue (0.00s)
      IPv4: zero next-hop MTU outcome = mtu-reported, want mtu-unreported
  --- FAIL: TestReportedMTUBelowIPv6MinimumIsDiscarded (0.00s)
      1279 on IPv6 outcome = mtu-reported, want mtu-unreported (RFC 8201 Section 4 discards it)
  ```

  Doctor (package C, before `doctor.go` existed): the package failed to build on `undefined: doctorCheckName` in `doctor_test.go`.
  Functional (package D, cut of `pmtuDiscValue`):

  ```
  1/2  FAIL  10  ping-do-not-fragment-reports-mtu
  ZE-OBSERVER-FAIL: show ping 10.99.2.1 size 1500 do-not-fragment honor-cache count 1 timeout 3s: reply status ok, want too-big
  ```

- [ ] Tests PASS (paste output)
  Scoped `go test -race -count=1`, 2026-09-15, after the value-only grammar:

  ```
  ok  	github.com/ze-software/ze/internal/core/probe	1.046s
  ok  	github.com/ze-software/ze/internal/component/ping/cmd	1.117s
  ok  	github.com/ze-software/ze/internal/component/traceroute/cmd	1.071s
  ```

  Root integration run (`-tags integration -run 'TestProbe|TestOpenICMP'`, 14 `--- PASS`):

  ```
  ok  	github.com/ze-software/ze/internal/core/probe	2.217s
  ```

  Functional, natively as root through the isolated `ze`/`ze-test` pair:

  ```
  2/2  PASS  11  ping-do-not-fragment-unprivileged
  1/2  PASS  10  ping-do-not-fragment-reports-mtu
  1/1  PASS  3  doctor-icmp-probe-missing
  ```

- [ ] Boundary tests for all numeric inputs
  The Boundary Tests table names the test for each edge.
- [ ] Functional `.ci` tests for end-to-end behavior
  The three `test/plugin/*.ci` in Functional Tests.
- [ ] Interop tests for protocol features (or N-A with a reason)
  `probe-df-clamped-path` in Interop Tests, against the Linux router namespace.

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)

---

## Implementation Summary

### What Was Implemented
- `internal/core/probe`: `DFMode` (`df.go`), `OpenICMP` and `Socket` (`socket.go`), the Linux options and the unprivileged datagram socket (`socket_linux.go`), the error-queue reader with the two RFC floors and `KernelPathMTU` (`errqueue.go`, `errqueue_linux.go`), the non-Linux stubs that report absence (`socket_other.go`, `errqueue_other.go`), the doctor check and its registration (`doctor.go`, `register.go`), codes `doctor-icmp-probe` and `doctor-icmp-probe-unprivileged` in `internal/core/diagnostic/codes.go`.
- Ping: `do-not-fragment honor-cache|bypass-cache` on `show ping` and `resolve ping`, the receive loop's error-queue drain, the `too-big` and `too-big-cached` rows with `next-hop-mtu-reported` and `next-hop-mtu`, the summary's smallest reported value and `path-mtu` (`ping.go`, `stream.go`, `resolve.go`).
- Traceroute: the same keyword on `show traceroute` and `resolve traceroute`, the refused hop ending the trace, the datagram-kind refusal by name, and every trace loop reading its identifier from the socket (`traceroute.go`, `stream.go`, `probe_round.go`, `register.go`, `resolve.go`).
- YANG: the leaf on both modules, revision 2026-09-15. Tests, the fixture `plugin/ping-do-not-fragment` and the three `test/plugin/*.ci` in Functional Tests. `./le qemu all-tests` runs `internal/core/probe` (`internal/le/qemu/alltests.go`).

### Bugs Found/Fixed
- `DFOff` was the absence of an option and Linux's default set the DF bit on every datagram that fit the path: `DFOff` now installs `IP_PMTUDISC_DONT` (`TestProbeDFBitOnTheWire`).
- The kernel does not match a queued error to an unconnected raw socket (A-3): both loops match the quoted identifier and sequence (`TestPingRefusalQuotingAnotherProbeIsIgnored`, `TestTracerouteRefusalQuotingAnotherProbeIsIgnored`).
- Closure review: a refused send's drain discarded a router's answer queued beside it (`TestPingRefusedSendKeepsTheRouterAnswerAhead`).

### Documentation Updates
- `docs/architecture/diagnostics/active-probes.md` (sections "The Don't Fragment mode", "The error queue", "Bounds and privileges", "Reply matching"; anchors on `socket.go`, `socket_linux.go`, `errqueue_linux.go`, `doctor.go`, `ping.go`), `docs/guide/command-reference.md` (`show ping`, `show traceroute`, the payload paragraphs), `docs/architecture/api/commands.md` (traceroute payload row, already at HEAD), `docs/features.md`, `docs/comparison.md`, `docs/guide/status.md`, `docs/functional-tests.md`, `rfc/short/rfc792.md`.
- `./le doc check verify` at closure: one drift, the shipping wiki catalog (`../wiki/command-catalog.md`, a sibling checkout regenerated by package D and owed its own commit there) against the live catalog; nothing on the pages above.

### Deviations from Plan
- The interop scenario is the integration package `internal/core/probe/errqueue_integration_linux_test.go`, not a `test/interop/scenarios/` directory (Design Insights).
- `do-not-fragment` takes one value and the bare keyword is refused (Key Design Decisions).
- Traceroute refuses the datagram socket; serving hops off its error queue is `plan/spec-traceroute-unprivileged-socket.md` (Work Not Done).
- `KernelPathMTU` reaches the operator as the ping summary's `path-mtu` (closure review, BLOCKER 1); the design named the read as step 6 of the data flow and the implementation had left it uncalled.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | A-3: the kernel was assumed to match a queued ICMP error to the socket that sent the probe | `raw_icmp_error` matches a raw socket by protocol and bound address only; an unconnected probe socket receives every flow's errors | `TestProbeErrorQueueAndAnotherFlow` (root, native netns) | both loops match identifier and sequence off the quoted echo before believing a value |
| approach | the DF keyword was designed and written (package A) as a bare keyword meaning honor-cache | the RPC layer reads every leaf as keyword-then-value and refused the bare form; only the handlers' parsers accepted it | package D's first `.ci` through the daemon | the grammar is keyword plus value; journal row in `plan/journal/documentation-shows-config-the-parser-refuses.md` |
| approach | `KernelPathMTU` was written and tested as a library function with no caller | an acceptance criterion with no entry point is unreached code | closure review (`grep` for non-test callers) | wired into the DF ping summary as `path-mtu`, with the entry-point test and the `.ci` assertion |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Set the DF bit, honoring or bypassing the kernel's path-MTU cache | Done | `internal/core/probe/socket_linux.go` `pmtuDiscValue`, `dfControl` | `TestOpenICMPInstallsDFMode`, `TestProbeDFBitOnTheWire` |
| Receive the next-hop MTU a router reports | Done | `internal/core/probe/errqueue_linux.go` `drainErrorQueue`, `classifyReportedMTU` | `TestProbeErrorQueueReportsNextHopMTU` and the ping and traceroute loop tests |
| Reached by every prober through one socket construction | Done | `internal/core/probe/socket.go` `OpenICMP` | `grep -rn ListenPacket internal/component/ping internal/component/traceroute` returns nothing |
| Absorb the CAP_NET_RAW doctor check and the unprivileged fallback | Done | `internal/core/probe/doctor.go`, `socket.go` `privilegeRefused` | `TestDoctorICMPProbeCheckReportsMissingCapability`, `TestOpenICMPFallsBackOnlyOnPrivilegeRefusal`, `test/plugin/doctor-icmp-probe-missing.ci` |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestPingWithoutDoNotFragmentOpensWithDFOff`, `TestOpenICMPInstallsDFMode/v4-off-clears-the-bit`, existing ping and traceroute `.ci` unchanged | `DFOff` installs `IP_PMTUDISC_DONT` |
| AC-2 | Done | `TestProbeBypassCacheDisagreesWithPoisonedCache`; `pingDFFits` in the `.ci` fixture | |
| AC-3 | Done | `TestPingRefusedByPathReportsNextHopMTU`, `TestTracerouteRefusedByPathRecordsNextHopMTU`, `TestProbeErrorQueueReportsNextHopMTU`; the two DF `.ci` | |
| AC-4 | Done | `TestReportedMTUZeroIsNotAValue`, `TestPingRefusedWithZeroNextHopMTUReportsNoValue`, `TestTracerouteRefusedWithoutValueRecordsNoMTU` | |
| AC-5 | Done | `TestProbeBypassCacheDisagreesWithPoisonedCache`; `pingDFRefused` under `bypass-cache` in the `.ci` | |
| AC-6 | Done | `TestPingDoNotFragmentSummaryCarriesTheKernelEstimate`; `pingDFPathMTU` 1600 then 1400 in the `.ci`; `TestProbeBypassCacheDisagreesWithPoisonedCache` reads the value as root | wired at closure |
| AC-7 | Done | `TestProbeUnprivilegedSocketReportsNextHopMTU`, `TestOpenICMPFallsBackOnPrivilegeRefusal`, `test/plugin/ping-do-not-fragment-unprivileged.ci` | |
| AC-8 | Done | `TestDoctorICMPProbeCheckReportsMissingCapability`, `test/plugin/doctor-icmp-probe-missing.ci` | |
| AC-9 | Done | `CGO_ENABLED=0 GOOS=darwin go vet ./internal/core/probe/` green at closure; `probe_other_test.go` compiles there | a darwin host run of the three stub tests is owed |
| AC-10 | Done | `rfc/short/rfc792.md` scope and Meta rows; `rfc/enrolled.txt` regenerated by `./le rfc index-update`, untracked | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| Every row of the Unit Tests table | Done | as named there | `go test -race` over the three packages green at closure (`close-pkgs.log`) |
| `TestPingRefusedSendKeepsTheRouterAnswerAhead`, `TestPingDoNotFragmentSummaryCarriesTheKernelEstimate` | Done | `internal/component/ping/cmd/errqueue_test.go`, `df_test.go` | added at closure for the two review findings |
| The three Functional Tests rows | Done | `test/plugin/` | the two DF `.ci` re-run as root at closure with the `path-mtu` assertions (`close-df-ci.log`); the doctor `.ci` unchanged since its root run |
| `probe-df-clamped-path` | Done | `internal/core/probe/errqueue_integration_linux_test.go` | green natively as root and in QEMU under the runtime kernel (main thread, `qemu-probe-int.log`) |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| Every file in Files to Modify and Files to Create | Done | `internal/core/privilege/check_linux.go`, `docs/architecture/system-architecture.md`, `docs/features/ai-first.md`: untouched, reasons in Files to Modify; `test/interop/scenarios/probe-df-clamped-path/`: landed as the integration package instead |

### Audit Summary
- **Total items:** 4 requirements, 10 ACs, 4 test rows, the file lists
- **Done:** all
- **Partial:** none
- **Skipped:** none
- **Changed:** 4, in Deviations

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| A probe can ask a question about packet size: the DF bit is set on the wire | interop, integration package | `TestProbeDFBitOnTheWire` reads the DF flag off an `AF_PACKET` capture on the router's link, green as root and in QEMU under the runtime kernel |
| The kernel's cache is honored or bypassed as asked | integration package | `TestProbeBypassCacheDisagreesWithPoisonedCache`: cache poisoned to 1400 with the clamp lifted, honor-cache refused at 1400, bypass-cache answered |
| The next-hop MTU a router reports reaches the operator | functional | `test/plugin/ping-do-not-fragment-reports-mtu.ci`: `show ping 10.99.2.1 size 1500 do-not-fragment honor-cache` answers `too-big`, `next-hop-mtu` 1400 on the reply and the summary, and `path-mtu` 1400; red under a cut of `pmtuDiscValue` (`reply status ok, want too-big`) |
| Ze asks the kernel rather than parsing ICMP errors | data correctness | `errqueue_linux.go` parses `SockExtendedErr` only; `grep -rn 'icmpv4FragNeeded' internal/component/traceroute` shows the raw datagram is skipped under DF so the queue records the hop |
| The privilege work lands with the socket construction | functional, security negative test | `test/plugin/ping-do-not-fragment-unprivileged.ci` under `capsh --drop=cap_net_raw` (red under a cut of `listenDatagramICMP`); `test/plugin/doctor-icmp-probe-missing.ci` (red under a cut of `checkICMPProbeSocket`); `TestProbeUnprivilegedFallbackIsNotTriedForOtherRefusals` |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| Traceroute on the unprivileged datagram socket: hop answers read off the error queue in the three trace loops, with `IP_RECVERR` on for `DFOff` too, in place of today's refusal by name | the datagram kind delivers only echo replies to the ordinary read, so a trace on it would time out at every hop; serving hops off the queue is three loops of separable work the main thread put to the owner, who had not answered at closure | `plan/spec-traceroute-unprivileged-socket.md` |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/core/probe/{df,socket,socket_linux,socket_other,errqueue,errqueue_linux,errqueue_other,doctor,register}.go` | Yes | `wc -l internal/core/probe/*.go` at closure lists all nine with their tests |
| `internal/test/fixture/plugin_fixture_ping_df.go`, `register_ping_df.go` | Yes | same listing |
| `test/plugin/ping-do-not-fragment-reports-mtu.ci`, `ping-do-not-fragment-unprivileged.ci`, `doctor-icmp-probe-missing.ci` | Yes | same listing; the two DF `.ci` ran at closure |
| `plan/spec-traceroute-unprivileged-socket.md` | Yes | written at closure, status `skeleton` |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1..AC-5, AC-7, AC-8 | the tests named in the Implementation Audit | `go test -race -count=1` over `internal/core/probe`, `internal/component/ping/cmd`, `internal/component/traceroute/cmd`: ok, ok, ok (closure, `close-pkgs.log`) |
| AC-6 | `path-mtu` on the DF summary | `TestPingDoNotFragmentSummaryCarriesTheKernelEstimate` in that run; `1/2 PASS ping-do-not-fragment-reports-mtu`, `2/2 PASS ping-do-not-fragment-unprivileged` as root with `pingDFPathMTU` asserting 1600 then 1400 (`close-df-ci.log`) |
| AC-9 | the darwin build compiles and reports absence | `CGO_ENABLED=0 GOOS=darwin go vet ./internal/core/probe/` ok at closure |
| AC-10 | the ledger row | `grep -n 'constructs' rfc/short/rfc792.md` finds the rewritten scope sentence; `rfc/enrolled.txt` is ignored by git and regenerated |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `show ping ... do-not-fragment honor-cache` and `bypass-cache` | `test/plugin/ping-do-not-fragment-reports-mtu.ci` | Yes: read; the fixture sends 1300 (answered), then 1500 under each value, and asserts `too-big`, both MTU keys and `path-mtu` |
| The same without CAP_NET_RAW | `test/plugin/ping-do-not-fragment-unprivileged.ci` | Yes: read; `capsh --drop=cap_net_raw` with a `GUARD:` refusal while CapEff still holds it |
| `doctor` with no ICMP socket at all | `test/plugin/doctor-icmp-probe-missing.ci` | Yes: read by the package D agent and run as root; asserts the JSON `code` row and `ze explain` |
| `show traceroute ... do-not-fragment` | unit only (`TestTracerouteDoNotFragmentReachesTheSocketOption`, `TestTracerouteRefusedByPathRecordsNextHopMTU`) | the refused hop through the daemon shares the socket, the drain and the fixture topology with ping; no traceroute `.ci` carries the keyword |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `TestProbeErrorQueueReportsNextHopMTU` (root, native and QEMU) |
| A-2 | confirmed | `TestProbeBypassCacheDisagreesWithPoisonedCache` |
| A-3 | broken | `TestProbeErrorQueueAndAnotherFlow`; the "if wrong" column shipped: identifier and sequence matched in both loops (Mistake Log) |
| A-4 | confirmed | `TestProbeUnprivilegedSocketReportsNextHopMTU`, `...NeedsThePingGroupRange`, `...IgnoresAnotherFlow` |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| `active-probes.md`: the DF mode, the error queue outcomes, the socket kinds, the traceroute refusal, `path-mtu` | `df.go`, `errqueue.go` `ErrQueueOutcome`, `socket.go` `OpenICMP`, `traceroute.go` `openRawProbeConn`, `ping.go` `doPingCtx` read at closure | Yes |
| `command-reference.md`: the keyword, the two values, the payload keys | `parsePingArgs`, `parseTracerouteArgs`, `tooBigResult`, `writeRefusedHop`, `summarizePingReplies` | Yes |
| `features.md`, `comparison.md`, `status.md` rows | `checkICMPProbeSocket`, `codes.go` | Yes |
| `rfc/short/rfc792.md` scope and Meta | `errqueue_linux.go` parses no ICMP message; `embeddedICMPOffset` matches the quoted echo | Yes |
| No `docs/architecture/api/commands.md` ping row exists to update | `grep -n 'show ping' docs/architecture/api/commands.md` returns nothing | Yes |
