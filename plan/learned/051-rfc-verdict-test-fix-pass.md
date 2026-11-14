# Learned: one predicate keeps a stamp and its gate in step, and "owned" means an AC row

`spec-rfc-verdict-test-fix-pass` was the parent of the RFC verdict pass: 1014
`weak` and `wrong` verdicts on 2026-09-28, split into eight children by
protocol group, each closed on its own (the last, services, in `22b8957163`).
The parent's own code is `mode rejudge` on `./le rfc audit-stamp`
(`2d078935e3`): a re-judged verdict replaces the recorded one with fresh
fingerprints, and an `upgrade_reason` is required when a finding becomes
`enforced` over byte-identical units. It also ran the narrowing audit
(`4c7f52aa06`) and the four cross-group verdicts (`fefe38b68c`, `cb4801b4cc`,
the second a D-8 fix of the IKE initiator's acceptance of a peer's ESP SPI 0).
On 2026-11-14, 19 `weak` verdicts and no `wrong` one remain, each left to the
spec that owns its fix.

## What outlives the spec

1. **One predicate for one decision is what keeps two surfaces in step.**
   `upgradeOverUnchangedUnits` (`internal/le/rfc/check_audit.go`) is called by
   the commit gate's `checkAuditFindings` and by the stamp's `rejudgeRefusal`,
   so the stamp refuses at write time what the gate would refuse at commit.
   When a later spec added the `partial` verdict, `findingUpgrade` learned it
   once and both surfaces followed. Four surfaces restate the rule in prose:
   the skill, `docs/contributing/rfc-conformance-gates.md`, the action's help
   and `docs/functional-tests.md`. Only `docs/functional-tests.md` had dropped
   `partial`, as had the doc comment of `findingUpgrade` itself, which named
   `weak` or `wrong` above a body that also tests `partial`. Three of them
   said a reason is "refused on any other move", when `rejudgeRefusal`
   accepts one on any upgrade to `enforced` and requires it only over
   byte-identical units, and the doc comment of `rejudgeRefusal` said the
   same. The code's behaviour needed no fix; the prose and those two comments
   did.
2. **A blocked-by list written at research is a floor.** A-4 held that 14
   named specs owned every verdict a test-only fix could not reach. On
   2026-11-14, eleven of the 19 remaining verdicts belonged to other specs and
   three to none until the closure homed them (`937c0bb8fb`), and the
   services child found three more during its own closure (RFC7950-7.9.4-1,
   RFC2131-4.1-7 and RFC2131-4.1-15, its A-2). P-3, which moves each blocked
   verdict into its owner's acceptance criteria as it is found, is what let
   every child close anyway.
3. **A criterion that says "named in another spec's acceptance criteria" is
   checked in the AC table.** The closure survey counted 16 ids as owned
   because a live spec mentioned each. Read against the AC tables, two sat only
   in an Owned journal rows table (RFC4271-5.1.2-2, RFC4271-8.2.2-15) and one
   only in a Task table behind a catch-all AC (RFC2205-3-40). A grep of
   mentions is the claim; the AC row is the evidence.
4. **A transfer is not a resolution.** RFC5882-4.4-1 was moved by owner
   decision to AC-24 of a BFD spec that then closed, and the verdict reads
   `partial`. A listing filtered on `weak` and `wrong` no longer shows it, so a
   transferred verdict is followed to its final state, not to its new owner.
   This closure found two that way: RFC5882-4.4-1, homed in AC-1 of
   `plan/post-release/features/spec-bfd-shared-session-same-remote-system.md`
   (`50b31b7804`), and RFC8050-4.1-1 (MRT dumps write no ADD-PATH Path
   Identifier), homed in AC-62 of `plan/release/fixes/spec-bgp-one-off-defects.md`.
