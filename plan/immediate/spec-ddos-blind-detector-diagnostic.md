# Spec: ddos-blind-detector-diagnostic

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | plugin |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-08-15 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**The problem.** ze cannot tell an operator that its ddos detector is armed and
blind. The plugin registers exactly one doctor check, `checkFlowSource`
(`internal/plugins/ddos/detect/doctor.go`), and that check reads the config tree
and nothing else: it warns `doctor-ddos-detect-no-flow-source` when ddos is
enabled with characterization on and neither `traffic/usage` nor `flow-export`
is a present container. It never touches the detector instance, the baseline,
the tick count, or any runtime counter.

**The failure it cannot see.** A source that is configured but dead passes this
check silently. The detector then runs against zero traffic, and every part of
it reports healthy:

- `(*baseline).Ready` (`internal/plugins/ddos/detect/baseline.go`) is
  `len(b.samples) >= b.window`, a count with no condition on the values, so a
  window of zeros makes the baseline "ready".
- `(*baseline).Threshold` on that baseline returns `max(0*multiplier, floor)`,
  which is the configured absolute floor. `config.Validate` forces the floor to
  be at least 1, so the threshold is a plausible non-zero number.

The result is a detector that looks warmed and armed, publishes a ready
baseline, and can never trigger, because nothing is arriving. Nothing in the
product says so. The operator discovers it during an attack.

**The goal.** A runtime readiness diagnostic that distinguishes "no attacks" from
"no traffic". The spec must decide:

1. **The signal.** Sustained zero (or near-zero) observed rate across a window
   long enough not to fire on a genuinely idle lab box. The tick count and the
   observed rates are both already on the detector; nothing new needs measuring.
2. **The surface.** A doctor check is the obvious home and the plugin already
   owns one, but a doctor check is pull-based and an operator has to run it. The
   alternative or addition is a log warning at the moment the condition is first
   met, and a field on the show surface. Which of the three, and whether the
   diagnostic is one-shot or latching, is the design question.
3. **What it says.** The useful message names the configured source and the fact
   that nothing has arrived, because the overwhelming majority of real instances
   of this are a misconfigured source rather than a genuinely silent network.
4. **The idle-box false positive.** A lab or standby router legitimately carries
   no traffic. The diagnostic must be a warning rather than an error, and it
   should be suppressible, or conditioned on the detector having been enabled
   deliberately.

**Related, and deliberately separate.** The baseline-readiness half of this
(`Ready` returning true on a window of zeros) is not itself a defect: the floor
saves the threshold, so nothing behaves incorrectly. It only becomes visible as
a problem once a diagnostic wants to distinguish a warm baseline from an empty
one, which is why it is noted here rather than in a fixit shard. If
`plan/immediate/spec-ddos-baseline-warmup-maturity.md` lands first, its maturity
accessor is the natural input to this check.

**Owning gate.** `go test -race ./internal/plugins/ddos/detect` for the
check itself, then `./le functional plugin`. `doctor_test.go` already exists in the
package and is the place the new check's unit coverage belongs.
