# RFC-tagged tests this commit changes

Each row records one owner approval for a change to a test that carries an
`RFC requirement:` tag. Such a test is the proof behind a public compliance
claim in `docs/features/rfc-status.md`, and `./le rfc check` counts it as
that proof. `ai/rules/testing.md` refuses every behavior change to one that the
owner did not approve. This file is the record of the approvals the owner gave.

Two native gates read it. The write hook in
`internal/le/hookruntime/writeedit.go` calls `weakened.Proposed` and refuses an
edit until this file names the test it changes. The commit gate in
`internal/le/commit/rfcchange.go` recomputes the tagged tests changed by the
prospective commit and refuses a missing or stale row.

Both use `internal/le/rfc/goscope.go` for tag carriers, function units, and
file-scope fallback, so they cannot disagree about what a row covers. A
reformat, a comment edit, and a Go import-only edit are not behavior changes. A
rename is.

The hook reads this file from disk, so the row is written BEFORE the edit. A row
written after the refusal buys nothing until the edit is made again.

A commit that changes no tagged test carries the table with no rows.

## Who writes the row

This is the one difference from `test/weakened.md`, and it decides everything
else. A weakening row is the author's own justification, and a reviewer reads it
to judge the author. A row here is the OWNER's decision, written down by the
author who asked for it.

An author cannot approve their own change. `ai/rules/testing.md` says it in one
line: a row in `test/weakened.md` does not authorize changing a tagged test,
because self-service justification is not user approval. A row here with no
answer from the owner behind it is a forgery, not a shortcut.

So the Reason column holds what the owner approved, not what the author wanted.

## The commit carries this file

`internal/le/commit/actions.go` refuses a commit that changes a tagged test and
does not name `test/rfc-changed.md` in its own `--file` list. A row that stays in
the working tree records nothing. The approval must sit in git history beside the
change it authorizes, because that is the only place a later reader can find it.

## The file is replaced per commit

Delete the rows of the last commit. Write the rows of this one. This file never
accumulates.

The reason is the mechanism it replaces, and `test/weakened.md` states it in
full: a justification explains one diff, and storing that record permanently is
what built the pile. The pile here is 255 `rfc-test-change-approved:` comments
across 120 test files. Nobody can read 255 approvals, so nobody reads them, so
writing one costs nothing.

Git history holds every past entry. `git log -p -- test/rfc-changed.md` shows the
rows of any commit beside the change they approved.

## The in-file marker is the old mechanism

`// rfc-test-change-approved: <date> <what and why>` is the record this file
replaces. It is a comment, so it stays in the test file after the diff it
explains is gone.

**No gate reads one.** The hook demanded a marker until 2026-08-19, and the
commit gate accepted one while it did, because a gate refusing it would have
refused every author for obeying the other gate. The hook reads this file now,
so both acceptances are gone: a marker approves nothing, and writing a new one
records nothing.

The sweep landed on 2026-08-19: 268 markers across 125 files, and 27 `test-relax:`
comments beside them, the mechanism `test/weakened.md` had already replaced. No
test carrier holds either token now.

The sweep is worth one paragraph, because deleting a retired token is not the
same as deleting what it said. Those 268 markers were 1475 lines of prose, and
about one block in six stated a fact about its own test that exists nowhere else:
a measured vacuity finding, a fixture precondition, a pointer to where coverage
moved. 57 of them survive as ordinary comments with the approval framing removed.
A mechanism can be retired without throwing away what people wrote under it.

## What this gate cannot see

The comparison is textual, and it is made against HEAD after comments and
whitespace are removed. Read the "what this gate saw" line in a refusal before
you write the row. The gate can be wrong, and the row is where you say so.

| The change | What the gate reports | Why |
|------------|-----------------------|-----|
| An assertion moved into a `t.Helper()` outside the tagged func | the tagged test changed | the scope cannot follow a call, which `tag_scope` records as its own known limit |
| The tagged test moved to a sibling file in the same commit | the test is gone from the old file | each path is compared against its own HEAD text, and no rename detection runs |
| The tagged test renamed | the old name is gone and nothing replaces it | a name is code, and telling a rename from a rewrite needs a Go parser |
| An assertion ADDED to a tagged test | the tagged test changed | any behavior change to the evidence needs the owner, and stronger is still different |
| A reformat, a comment edit, a Go import-only edit | nothing | the detector removes comments and whitespace before it compares |

The first three cost an author one row explaining that the gate was wrong. That
is the price of a gate that fails toward asking. `internal/le/testweakened/actions.go`
reported two such false findings on 2026-08-19, one for a helper moved to a
sibling file and one for an error check extracted into a `t.Helper()` that also
added an assertion. Both are the first row of this table.

## The test name

| Carrier | The name |
|---------|----------|
| Go, inside a top-level func | the enclosing `func TestXxx` |
| Go, with a tag no top-level func encloses | the file stem |
| `.ci` or `.et` | the file stem, because each such file is one test |

`internal/le/rfc/goscope.go` resolves each one, and the same resolver serves
`test/weakened.md`. `FunctionUnits` returns top-level Go functions with their
names, while `ScopeReader` treats every non-Go carrier as one file-scoped unit.

Row two is `tag_scope`'s own fallback. A tag can sit outside every function span:
a hoisted table, or a tag separated from its func by a blank line. The gate then
treats the whole file as the unit, because a narrower answer would be a guess.
The stem is the only name available, so a change in `a_test.go` is written as
`a_test`.

A bare name is accepted when it resolves to exactly one changed test in the
commit. Write `package.TestName` when it does not, where the package is the
directory holding the file.

## The reason

Name what the owner approved, and say why the tagged requirement is still
proven after the change. A reason that does not answer the second question
approves a compliance claim losing its evidence.

Every requirement id the gate attributes to a name is covered by that name's one
row. Quote the id, so a reader can open `rfc/short/` beside it.

| Test | Reason |
| ------ | ------ |
| TestRFC3748IKEv2RequiresKeyDerivingMethod | Thomas approved this on 2026-09-01, in the same message that ordered MD5-Challenge implemented and said he would re-approve both tests. The id is `RFC3748-7.10-3`, positive and negative, and it is still proven. The rule is "Methods that do not generate MSK ... MUST NOT be used with IKEv2", and it still holds: `NewEAPSession` (`internal/component/ike/engine/eap_auth.go`) maps `ipsec.AuthMode` to Type 26 or Type 13 and refuses everything else, and the peer switch in `fsm.go` does the same. The test asserted that refusal at `eap.NewSession`, which is the EAP framework's constructor rather than IKEv2's selection point, and RFC 3748 Section 5 obliges that constructor to accept Type 4. The assertions move to the two producers that enforce the rule, which is coverage those producers did not have before. |
| TestRFC7296EAPKeylessMethodsAreRefused | Same approval. The id is `RFC7296-2.16-5`, positive and negative. The row is the conditional "If EAP methods that do not generate a shared key are used, the AUTH payloads in messages 7 and 8 MUST be generated using SK_pi and SK_pr, respectively", and this test discharged it by proving the antecedent never fires: its own claim read "A keyless method never starts, which keeps the SK_pi and SK_pr AUTH mode unreachable". Both halves are now false. `eapAuthSecret` (`internal/component/ike/engine/eap_auth.go`) implements the consequent, and `TestEAPAuthOfNonKeyDerivingMethodUsesSKpiAndSKpr` (`internal/component/ike/engine/rfc7296_eap_nonkeying_auth_test.go`) drives it over a real MD5-Challenge exchange. The tag moves to that test, so the row is discharged by the proof owner ruling OR-E of 2026-07-30 required rather than by a vacuity argument. |
| TestRFC3748PeerNeverSendsNAK | Thomas approved this on 2026-09-02, answering a question that named this deletion explicitly. The test is DELETED. It carried `RFC3748-2.1-3 positive` and `RFC3748-7.10-3`, and its own claim asserted the behavior this spec exists to correct: that the peer "errors (never NAKs) on an unexpected type" and "must error instead". RFC 3748 Section 5.3.1 requires the opposite, a legacy Nak for an unacceptable authentication Type, so the assertion pinned a non-conformant path. Neither tag is lost. `RFC3748-2.1-3` is re-claimed with BOTH polarities by `TestRFC3748PeerNaksBeforeItCommitsToAMethod` (`internal/component/ike/eap/rfc3748_nak_test.go`), which pins Section 2.1's real boundary: the peer may Nak until it has answered a method Request, and discards afterwards. `RFC3748-7.10-3` is re-claimed with both polarities by the two tests in `internal/component/ike/engine/rfc3748_ikev2_method_selection_test.go`, at the producers that actually enforce it. Every re-homed polarity carries a discrimination record. |
| TestPeerDiscardsARequestOfAnotherType | Thomas approved this on 2026-09-02. The id is `RFC3748-2.1-4 positive` and it is still proven. The case drove an MS-CHAPv2 peer that had answered only the Identity Request, then sent a Type-13 Request and required a discard. That is now a legacy Nak, correctly: RFC 3748 Section 2.1's discard binds once a method is UNDER WAY, and Section 5.3.1 owes a Nak before then. The case now answers a real MS-CHAPv2 Challenge FIRST, so the method is genuinely under way, and only then sends the Type-13 Request and asserts the discard. The property it is named for is unchanged and is now tested at the state the RFC actually names.
| TestRFC3748ResponseTypeMatchesRequest | Same approval. The id is `RFC3748-4.1-5` and both polarities are still proven. Section 4.1 says a Response's Type "MUST either match that of the Request, or correspond to a legacy or Expanded Nak". The negative arm asserted the FIRST branch by requiring an error for a Type-99 Request; ze now takes the second branch and answers a legacy Nak, which the same sentence permits. The arm asserts the Nak Type instead, so the requirement's other branch is pinned where nothing pinned it before. The positive arm is untouched.
| TestRFC7296EAPKeyDerivingMethodsAreAccepted | Same approval. The id is `RFC7296-2.16-5 negative`, and this carrier is being UNTAGGED rather than changed in what it asserts: the tag moves to `TestEAPAuthOfKeyDerivingMethodStillUsesTheMSK` (`internal/component/ike/engine/rfc7296_eap_nonkeying_auth_test.go`), which drives the real AUTH construction. The body keeps every assertion it had. Its `eapmKeyDeriving` map, a hand-written third copy of which methods derive keys, is deleted in favor of the single declaration `eap.TypeDerivesKey`.
| TestEapAuthProducerIsKeyedByTheNegotiatedMSK | Same approval. No assertion moved. `ComputeAuthFromMSK` was renamed to `computeAuthFromSharedSecret` when the two AUTH constructions were merged into one, and this test only calls it. The rename follows a contract change: the function used to take an MSK and now takes any shared secret, an MSK, SK_pi, SK_pr or a PSK, which RFC 7296 Section 2.15 keys identically. `RFC7296-2.16-12` proves exactly what it proved before.
| TestEapAuthProducerOutputIsRefusedUnderAnotherKey | Same approval, same rename, same reason. `RFC7296-2.16-13` is unchanged in what it asserts; only the call-site name moved.
