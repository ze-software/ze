# CI Sleep Justification

**When:** Adding, editing, or reviewing any `time.sleep(` in a `test/**/*.ci` functional test. Every sleep must carry a comment explaining why it is there.
**Severity:** advisory

## Rule

Every `time.sleep(` call in a `.ci` test MUST have an explanatory comment on the
line directly above it, or trailing it on the same line. A bare sleep with no
comment is rejected. This is the qualitative companion to the **Sleep ratchet**
(`ai/rules/testing.md`), which caps how MANY sleeps exist: this rule caps how many
are unexplained. Two reasons:

1. A blind sleep hides real races. A reader cannot tell whether it is safe (a
   bounded poll interval that already blocks on a real condition) or a guessed
   duration that will flake under load.
2. When a sleep is deliberately left un-converted, the reason (deliberate timer,
   a Linux-only effect verifiable only under QEMU, an effect with no queryable
   readiness signal) is knowledge that must live next to the code, not in a
   reviewer's head.

Prefer converting the sleep to a deterministic wait
(`ze_api` `wait_until` / `wait_for_event` / `dispatch_until`, see
`ai/rules/testing.md` "Python Observer API"). Only when conversion is not possible
does the sleep stay, and then it MUST be justified.

## What counts as justified

The comment must state which of these the sleep is:

| Kind | What the comment should say |
|------|-----------------------------|
| Bounded poll interval | Name the real condition the enclosing loop breaks/returns on ("poll interval; the loop above breaks when the nft table appears"). This is already a deterministic wait; the sleep is only its granularity. |
| Deliberate timer | The delay itself IS the behaviour under test ("the 3s verify hold IS the concurrency race window; do NOT convert"). |
| Timeout under test | The sleep waits out a fixed internal timeout that the test asserts ("the 5s vpp WaitConnected timeout IS the behaviour under test"). |
| needs-linux effect | A dataplane effect (tc/qdisc/nft/kernel FIB) with no readback in the driver, convertible only after a QEMU run ("needs-linux; no queryable signal that the qdisc was programmed"). |
| No readiness signal | The awaited effect exposes no queryable state to this driver ("backgrounded ze gets no ZE_READY_FILE marker; hold until OnConfigure emits the asserted log line"). |

Placement (mechanical): one `#` comment line directly above the sleep, indented to
match the sleep exactly (these are Python heredocs; wrong indentation is a syntax
error). No em dashes in the comment text.

## Enforcement

- **Blocking gate:** `check_ci_sleep_justification` in
  `scripts/dev/verify_wiring_docs.py`, run by `make ze-verify-wiring-docs` (and the
  inventory make gate). Scoped to CHANGED `.ci` files: a session is responsible for
  the sleeps in the tests it touches. Fails (exit 1) listing every unjustified
  `file:line`.
- **Edit-time nudge:** `c_ci_sleep_justification` in
  `.claude/hooks/pretool-writeedit.py` warns (non-blocking) when a Write/Edit of a
  `.ci` introduces a `time.sleep(` with no comment on the line above or trailing it.

## Related

- `ai/rules/testing.md` -- Sleep ratchet (count cap) + the `ze_api` deterministic waits.
- `plan/spec-fixit-redistribute-establishment-stall.md` -- the P0 that blocks converting the redistribute establishment sleeps.
- `plan/learned/1232-fixit-reject-fence-observability.md` -- the missing signal behind the external-plugin refuse/warn sleeps.
