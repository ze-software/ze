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
|------|--------|
| TestRFC7606Section54PropagatesUnknownBGPLSType | Thomas authorized the commit of these twelve files on 2026-08-30, in answer to a report naming each change and separating this one from the eleven mechanical ones. The known NLRI beside the unknown one was `lsWireNLRI(1, 0x02, 0x00)`, a Node NLRI whose two-octet body cannot hold the Protocol-ID and the eight-octet Identifier RFC 9552 Section 5.2 gives it. It is `lsNodeNLRI(65001)` now, which builds the same type 1 with its Protocol-ID, its Identifier and a Local Node Descriptors TLV. RFC7606-5.4-1 negative is still proven, and proven for the right reason: the assertion is unchanged, that `enforceRFC7606` returns `RFC7606ActionNone` and the MP_REACH survives byte-identical with the unknown type 99 still in it. What changed is that the surviving evidence can no longer come from the syntactic walk of Section 8.2.2 discarding a malformed companion, which is the rule the old fixture risked measuring instead. |
| TestNewStreamable_OAuth_AcceptsValidToken | Same authorization, and the edit is `httptest.NewRequest` becoming `httptest.NewRequestWithContext(t.Context(), ...)` at each request the test builds. RFC8707-5-1 positive and RFC8707-2-3 positive rest on the assertions below those calls, that a token whose `aud` matches the canonical audience authenticates and that the trailing-slash-divergent form matches. Neither the request method, the target, the body nor any assertion changed: the request now carries the test's context so it is cancelled with the test. |
| TestNewStreamable_OAuth_RejectsMissingBearer | Same authorization, same mechanical cause. RFC9728-5.1-1 positive still rests on the 401 challenge's `WWW-Authenticate` carrying `resource_metadata`, asserted on the recorder below the call. |
| TestNewStreamable_OAuth_MetadataEndpoint | Same authorization, same mechanical cause. RFC9728-3.1-1 positive, RFC9728-2-1 positive and RFC9728-2-2 positive still rest on the served document: an unauthenticated GET reaches the well-known path, and the body carries `resource` and an `authorization_servers` array. |
| TestNewStreamable_OAuth_AcceptsSlashDivergentAudience | Same authorization, same mechanical cause. RFC8707-2-3 in both polarities still rests on the same two audiences, the trailing-slash variant accepted and the scheme-downgraded other resource refused. |
| TestOAuth_Authenticate_MissingHeader | Same authorization, same mechanical cause. RFC9728-5.1-1 positive still rests on the challenge the handler writes when no Authorization header is present. |
| TestStreamable_MetadataEndpoint_Gated | Same authorization, same mechanical cause. RFC9728-3.1-1 negative still rests on the well-known path not being served when AuthMode is not OAuth, and RFC9728-2-1 and RFC9728-2-2 positive on the document `writeResourceMetadata` emits when it is. |
| checkAddPathReadvertiseCollision | Same authorization. The interop checker's literals became the names in `names.go`, and each constant carries the exact string it replaced: `peerFRR` is "frr", `cmdVtysh` is "vtysh", `peerPrefixFirst` is "10.99.0.0/24", the value the retired `addPathEvidencePrefix` held. The lab commands, the prefix queried and the assertion, that FRR holds two paths for it, are unchanged, so the ADD-PATH re-advertisement evidence is the same run over the same bytes. |
| checkOTCWithdrawal | Same authorization, same substitution: `zeLabAddress` is "172.30.0.2" and `injectPrefixFirst` is "10.10.0.0/24". The negative operation list is built with one `make` and three appends instead of a literal followed by two appends, which changes its order not at all: route-absent, session, the relay extras, session. The OTC withdrawal evidence is the same sequence against the same peer. |
| checkReflectorWithdrawal | Same authorization, same substitution of `peerFRR`, `cmdVtysh` and `zeLabAddress` for the literals they carry. The reflector withdrawal scenario runs the same vtysh queries against the same neighbor and asserts the same absence. |
