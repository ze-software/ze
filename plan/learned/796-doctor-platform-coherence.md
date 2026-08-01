# 796 -- Platform-Aware Doctor Coherence Checks

## Context

`ze doctor` checks ran independently without knowing the runtime platform.
`checkPlatform()` detected gokrazy/systemd/container/plain-linux/darwin but
discarded the result. This allowed silent coherence gaps: two independent
configs could conspire (gokrazy NTP excluded + Ze NTP disabled = no clock sync)
without any diagnostic being emitted.

## Decisions

- Thread `*host.PlatformInfo` as a local through `runChecks()` to individual
  checks, over a standalone `checkPlatformCoherence()` cross-cutting function.
  Each check is an independent unit that should know its execution context.
  Coherence logic belongs where the domain knowledge lives.
- Error severity on gokrazy (Ze owns everything), warning on systemd/container
  (external NTP possible), no diagnostic on Darwin/unknown. Over uniform
  severity across platforms, because the confidence level differs.
- Nil-safe platform parameter with graceful degradation, over requiring non-nil.
  Platform detection can fail; existing behavior must be preserved.
- Private env overrides (`ze.test.doctor.platform`, `ze.test.doctor.machine-id-path`)
  for deterministic functional tests, over depending on host state.

## Mistakes

- Initial design proposed a standalone `checkPlatformCoherence()` that would
  cross-cut all checks. User pointed out this creates a separate layer that
  duplicates domain knowledge. Each check should be independently context-aware.
- QEMU/root execution exposed a false-negative in NTP privilege tests: UID 0
  bypasses `CAP_SYS_TIME` parsing. Added `currentUID` test seam.
- Naming: used "Coherence" suffix on function names (`checkResolvConfCoherence`,
  `checkNTPPersistPathCoherence`). Existing check names describe the thing
  checked, not the analysis type. Renamed to `checkResolvConfPath`,
  `checkNTPPersistPath`. Similarly `platformMismatchDiagnostic` -> `platformMismatch`
  to match existing constructor style (`tcpListener`, `udpListener`).

## Patterns

- When threading context through a check pipeline, pass as a function parameter
  (not a global). Doctor checks are independent units; globals couple them.
- Functional tests for platform-specific behavior need env overrides to be
  deterministic. The host running `ze doctor` may be any platform.
- Linux-only checks need: implementation in `checks_linux.go`, stub in
  `checks_other.go`, unit tests in `checks_linux_test.go`, QEMU integration
  test with `//go:build integration && linux`, and a `.ci` functional test
  with `option=skip-os` for non-Linux.

## Files

None recorded.
