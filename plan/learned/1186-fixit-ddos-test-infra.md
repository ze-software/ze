# 1186 -- fixit: ddos test-infrastructure (detect-mitigate rework + transit FORWARD-drop proof)

## Context
Two ddos test-infra follow-ups from `spec-ddos-direction-allowlist` (`plan/deferrals.md`
rows for `ddos-detect-mitigate.ci` rework and the AC-10 transit proof).
(A) `test/plugin/ddos-detect-mitigate.ci` had two hand-rolled `time.sleep` poll loops and
had never been proven green. (B) No functional `.ci` proved that a REMOTE (transit) victim
gets an nft FORWARD-hook drop when `ddos { local { forward-mitigation } }` is on; hook
selection was only unit-tested (`TestLocalHookByDirection`).

Both are QEMU-only (`option=needs-linux`). This session implemented both test files and
PARKED them: the environment forbids running the QEMU/functional suites, and every AC of
this spec is proven by a QEMU run, so the green-under-QEMU proof (AC-1, AC-4, AC-5, and
AC-6 execution) is left to the human/CI run recorded in the drain recipe.

## Decisions
- **Keep the `daemon.pid`/`daemon.ready` handshake + root driver.py (D-1), do NOT port to
  the in-daemon `ze_api` probe.** The handshake is alive (armed for background `ze`), and
  the assertion REQUIRES reading the nft ruleset (no dispatch surface carries the hook,
  `local/show.go`), which needs the privileged foreground driver the runner deliberately
  keeps root (`internal/test/runner/runner_exec.go`). Migrate only the two
  `time.sleep` poll loops to `ze_api.wait_until` (module-level import), mirroring the green
  pattern of record `test/vrrp/vrrp-instance-up.ci`.
- **Transit test structure follows the producer's once-per-generation emit.**
  `AttackDetected`/`AttackCharacterized` emit exactly once per attack generation at onset
  (`internal/plugins/ddos/detect/detector.go`, `characterize.go`); only
  `AttackOngoing` re-emits (empty target). So `hookForDirection` is consulted only at attack
  onset, and AC-4 (forward-mitigation ON -> FORWARD drop) and AC-5 (OFF -> defer, no drop)
  each need a DISTINCT generation. The single `ddos-transit-forward-drop.ci` uses two phases
  separated by an attack clear: Phase A drives one generation and asserts the nft FORWARD
  drop; the flood stops and the mitigation is withdrawn; the config reloads forward-mitigation
  OFF via SIGHUP (rewrite `ze-bgp.conf`, vrrp precedent); Phase B drives a fresh generation
  that must defer (proven by the `deferring to flowspec` stderr log) with no drop installed.
- **No `firewall {}` block (D-2).** `ApplyAll` autoloads nft on demand for plugin-owned
  tables (`internal/component/firewall/registry.go`), so ddos-local is the sole nft
  driver and the two-driver combination behind the R-1 deadlock is avoided.
- **Remote victim = a connected-but-unassigned address.** setup.py addresses the box end of
  a veth `203.0.113.1/24`, leaving `203.0.113.9` inside the connected /24 but owned by no
  interface, so `classifyDirection -> iface.AddressIsLocal` returns false (RTN_UNICAST, not
  RTN_LOCAL) -> `DirectionRemote`. A static neighbour lets the flood egress without ARP
  stalling on a host that does not exist.
- **Assert kernel state AND the responder log (D-4).** Driver reads `nft list ruleset` for
  `ze_ddos-local` + victim + `hook forward`; `expect=stderr` corroborates via
  `ddos-local: drop rule installed` / `hook=forward` (Phase A) and the defer log (Phase B).
- **Retarget the reworked test's victim to 127.0.0.4** (review finding) to remove the
  address overlap with `ddos-policy.ci` (127.0.0.2); direction uses 127.0.0.3.

## Consequences
- `test/.ci-sleep-baseline` lowered 127 -> 125 (two real `time.sleep(` loops removed; the
  new transit file adds none, using `wait_until` throughout).
- `TestLocalHookByDirection` stays the exhaustive hook-selection unit guard (AC-7, still
  green); the new `.ci` complements it end to end.
- Both `.ci` files are auto-discovered by `ze-test bgp plugin` (`tests.Discover(test/plugin)`),
  so AC-6 registration is automatic; the needs-linux filter picks them for the QEMU gate.

## Gotchas
- **Stderr assertions depend on the slog Text handler with color OFF.** Under the standard
  non-TTY QEMU capture, `hook=forward` and the defer token render plainly; a PTY or
  `ze.log.color=true` would dim the key and break the substring. Not a defect today.
- **Runtime links are QEMU-empirical, not statically provable here:** that the veth egress
  flood raises RX pps on the monitored end (zdd0p) enough to trip the detector, and that
  trafficusage `track-ip` records `203.0.113.9` as the dominant destination. The config
  monitors both zdd0 and zdd0p as a hedge. This is exactly AC-1's discovery territory.
- **Shared kernel table name `ze_ddos-local` across all ddos-local enforce tests** means a
  concurrently scheduled policy-exempt test whose `removeMitigation`+`ApplyAll` sweeps all
  `ze_*` tables could delete another ddos test's table under the parallel plugin gate. This
  is pre-existing R-7 / `spec-fixit-firewall-concurrency-deadlock` territory, not introduced
  here; the victim-address fix removes only the trafficusage-resolution overlap.
- **stale-deferral class:** both parent deferral rows were accurate when written (2026-07-12)
  and falsified one day later by unrelated commits (`dc082c288` armed the handshake,
  `c5273da42` added nft autoload). Re-verify a deferral's premise against the current tree
  before acting; date the evidence.

## Files
- `test/plugin/ddos-detect-mitigate.ci` -- reworked: `wait_until` migration, victim 127.0.0.4,
  refreshed STATUS header.
- `test/plugin/ddos-transit-forward-drop.ci` -- new: veth transit topology, Phase A/B proof.
- `test/.ci-sleep-baseline` -- 127 -> 125.
- Producers verified (unchanged): `internal/plugins/ddos/local/responder.go`,
  `internal/plugins/ddos/local/config.go`, `internal/plugins/ddos/detect/detector.go`,
  `internal/plugins/ddos/detect/characterize.go`, `internal/component/firewall/model.go`.
