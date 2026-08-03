# 1257 -- A declared `.ci` timeout that did nothing

## Context

`test/appliance/vpp-hugepages-qemu.ci` builds a real appliance image and boots it
in QEMU. On every machine without KVM access the driver self-skips in a couple of
seconds, so that is all the suite had ever measured. The first host to run it
with `/dev/kvm` actually accessible watched it get killed at 15.0s, mid-build.

The obvious reading was a stale timing baseline: the per-test budget is
`min(global, max(5s, 5x baseline avg))`, and a baseline built from SKIP runs is
tiny. The documented fix is an explicit override, and `ai/rules/testing.md` states
plainly that "Explicit `.ci` `timeout=` overrides always win". Adding
`option=timeout:value=900s` changed nothing. Moving it to the top of the file
changed nothing.

It was not the baseline. `internal/test/runner/timing.go` had no entry for the
test at all, so the 15s was the suite's global default, and the record-level
option was being parsed and then discarded.

## Decisions

- **Fix the producer, not the one test.** `Run` (`runner_exec.go`) resolves
  `rec.Extra["timeout"]`, but it returns to `runOrchestrated` earlier, at the
  `len(rec.RunCommands) > 0` branch, and never reaches that code. Every test using
  a `cmd=` directive -- which is most functional tests -- therefore had any
  `option=timeout:` silently ignored. Patching the one `.ci` to use the
  per-command `cmd=foreground:...:timeout=900s` form would have worked and left
  the trap for the next person.
- **Extract the precedence into a pure function** (`resolveOrchestratedTimeout`
  in `runner_exec_util.go`) rather than adding a second copy of the lookup inside
  `runOrchestrated`. `runner_exec.go` is already over the 1000-line threshold, and
  a pure function is directly unit-testable where the orchestrated path is not.
- **Precedence, most specific first:** per-command `timeout=` on a foreground
  `cmd`, then record-level `option=timeout:value=`, then baseline-derived, then
  the global default.

## Consequences

- Other tests that declared a record-level timeout beside a `cmd=` directive now
  get the budget they asked for. `test/install/ze-kernel-no-modcache-mutation.ci`
  declares `option=timeout:value=30s` and had been running on the global default.
- A test that only ever skips teaches the timing baseline nothing useful. When a
  self-skipping test starts really running because the host gained a capability,
  expect the derived budget to be wrong by orders of magnitude, and prefer an
  explicit declaration over waiting for the baseline to catch up.
- Suite wall-clock rises on hosts where a previously-skipping QEMU proof now runs.
  That is the intended trade: `ai/rules/platform-linux.md` is explicit that a SKIP
  is not evidence.

## Gotchas

- **An option that is accepted and then discarded is worse than one that is
  rejected.** The `.ci` file reads as though the budget were set, the parser
  accepts the line without complaint, and the only symptom is a test dying at a
  number that appears in no file. A rejected option would have named itself in
  one run. This is the `evidence.md` principle applied to configuration:
  a knob that neither takes effect nor says anything does not exist.
- Two mechanisms with the same name existed (`option=timeout:value=` on the
  record, `timeout=` on the `cmd=` directive) and only the narrower one worked on
  the common path. When a setting has two spellings, check which code path
  actually reads each.
- The misleading first hypothesis was the timing baseline, because the symptom
  (15.0s, a suspiciously round 5x3s) fit it perfectly. `timing.go` had no entry
  for the test; reading the producer instead of pattern-matching the number would
  have got there faster.

## Files

- `internal/test/runner/runner_exec.go` -- `runOrchestrated` calls the resolver
- `internal/test/runner/runner_exec_util.go` -- `resolveOrchestratedTimeout`
- `internal/test/runner/runner_exec_util_test.go` -- `TestResolveOrchestratedTimeout`
- `test/appliance/vpp-hugepages-qemu.ci` -- declares the real cost of a real run
