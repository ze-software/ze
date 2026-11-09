# Tooling one-off defects: the actual-root fix chain, 2026-11-09

Spec: `plan/post-release/engineering/spec-tooling-one-off-defects.md` (claimed,
in progress). Goal set by Thomas: close the spec, every AC met by its proof
obligation P-1 to P-41 through the real entry with attributed red, repair and
restored green, every owned journal row closed or moved to "Rows found already
fixed", Review Gate passed, two-commit closure.

## Where it stands

`TestAnswerAnswersTheInventory` (the actual repository) still refuses. Each
commit below removed one actual-root stop; the reported stop varies run to run,
because only the first refusal of the final fixpoint round is named.

| Commit | Analyzer capability added |
|--------|---------------------------|
| 6928ef7c82 | a call result's fields are the callee's returned fields; external results flattened |
| f0fe8070e1 | generic type methods execute; instantiation census for unbound type parameters |
| 7ae5e8afc4 | `for _, x = range c` stores c's contents |
| 0822a16666 | tuple operands type their fields; named results return their variables |
| 7c77569769 | assembly leaf without NOSPLIT |
| 3c5348f09d | assembly leaf conditional arms read branch-neutrally |
| 562a0305cf | indexed elements carry the proved element summary; direct element-field stores (storeLocation) |
| 75ad75aeab | interface receiver dispatches to the methods its value carries |
| 925dac4ff0 | bare generic export keeps its declaration binding (five header fixtures now pass) |

Package suite (`go test` of `internal/le/repo/inventory` with the full tag set,
skipping the five actual-root tests): one known failure left,
`TestCommittedFamilyHelpersRetainCanonicalNames/flowspec-projection`
("external mutation can change registration metadata through familySet"). It
predates this session. Suspect: a range element of `familySet` carries the
slice's own location through rangedValue's flattened collection, and the
location reaches the ambient pool through the exposed `family.Family.String`
receiver. Unverified: trace with a probe before fixing.

## Next

1. Thomas must allow restarting the actual-root run: the harness stopped the
   last one for low memory, and its rule is to restart only when asked.
2. Fix the family fixture (above), then continue the actual-root chain. The
   last seen stops were protobuf's validator (`impl/validate.go`,
   `st.mi.Desc.FullName()`) and `filedesc/desc_list.go` (`desc.Index()`), both
   interface method calls, which 75ad75aeab addresses; the next stop is
   unknown.
3. Then the remaining proof obligations: P-35 missing; P-10, P-12 (plan in
   `tmp/session/2026-11-07-095909/scratch/p12-plan.txt`, 15 interop checkers,
   5 RTR fixtures), P-18, P-36, P-26, P-32, P-33(1), P-4, P-14 partial; the
   owned journal rows; Review Gate; two-commit closure.

## Method notes

- Red walks: `tmp/session/2026-11-07-095909/scratch/redwalk.py` takes a JSON
  list of `{name, file, old, new, pkg, test, tags}`; MUTATION notes in test
  comments are parsed into it.
- Every changed existing test went to an independent `ze-test-judge`; approvals
  are in `tmp/commit-weakening-approved-54456e39.md`.
- Never name a new test file without checking it is free: one overwrite of a
  tracked test was caught by the commit gate and restored from HEAD.
- The inventory selects amd64 assembly on the arm64 Mac.
