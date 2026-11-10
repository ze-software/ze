# Handover: closing near-done specs (2026-11-10)

**Spec:** none claimed. This session closed specs one per agent; no claim remains.
**Branch:** main (commits local, unpushed; Thomas pushes at a point he chooses).
**Goal:** close the open specs whose implementation is already committed. The
seven easy ones are closed. Three medium ones are next.

## Status

### Done (committed)

| Spec | Closing commits |
|------|-----------------|
| spec-lint-backlog-9-config-and-forwarding | `dca392f162`, `74481d95fb` |
| spec-lint-backlog-2-bgp-plugins | `1f4d542608`, `68d7c9e541` |
| spec-lab-docker-cleanup-and-go-test-containers | `e6ce0f9c40`, `091c33f079` |
| spec-lint-backlog-1-bgp-engine | `7c2e0fbdba`, `c7d8391a17` |
| spec-lint-backlog-7-system-and-routing-plugins | `b972ef8113`, `673bcaf92d` |
| spec-lint-backlog-5-hub-and-policy | `0fa6d5f922`, `b1065c830f` |
| spec-lint-backlog-6-access-and-appliance | `7eb86b64a2` (interim), `1b98435bc8`, `84b796f5e3` |

Tooling fix landed on the way, because it blocked every lint closure:
`b032323956` "le: a scoped lint judges only its own inputs". A scoped
`./le go lint run scope` used to abort on any edit anywhere in the tree; it now
refuses only when a path its own population reads changes by content
(`job.ChangedInputs`, `lintPopulation` in `internal/le/go/lint/planinputs.go`).
Independent review CLEAN at round 3; ze-test-judge approved the seven parity
tests whose seam changed.

Owner decisions taken this session:
- lab-docker Ctrl-C proof of `verify evidence release-candidate`: the three
  fake-runner tests were accepted as AC-8 evidence (test-level, not live daemon).
- lab-docker review: a fresh one-round review of the closure files, not a
  re-record of another session's round 10.
- lint-6: the rangeValCopy findings in `ike/engine/unmatched*.go` were fixed
  before release (not left to spec-ipsec-spd-final-discard-vpp-ospf), with
  RFC4301-4.4.1-3 re-recorded from hand-written breaks.

### In progress

Nothing of this session is uncommitted. Every dirty file in the tree at
handover time belongs to other sessions.

### Remaining: the next closures (from the 2026-11-09 triage)

| Spec | What remains | Rating |
|------|--------------|--------|
| `plan/release/validation/spec-lint-backlog-3-test-tooling.md` | Scoped lint over `./internal/test/... ./internal/le/... ./internal/chaos/... ./tools/freertr-entrypoint/...`; tagged tests for `internal/test/{fixture,plugins/fakefilter,runner,userns,editorcase,cli}` and a race run of `internal/le/commit` and `internal/le/test/weakened` (earlier run hit a 240 s timeout); review round 7 pending | medium |
| `plan/release/validation/spec-rfc-verdict-fix-ike-eap.md` | Review Gate not run; rfc4555 and rfc2759 SHIFTED verdicts may need `./le rfc reseal`; Work Not Done must name `plan/release/validation/spec-ipsec-rfc9190.md` for RFC9190-2.3-1/2.3-4; its "`./le verify changed` is owed" row is wrong under the rules and is rewritten at closure | medium |
| `plan/post-release/features/spec-route-policy-0-umbrella.md` | Phase 4 has no review round and no AC-17 evidence (cite `spec-filter-wire-0-umbrella`'s closure, commit `0c3c86b319`, or run the BGP policy `.ci` and `bgp-policy-*` interop); AC-12/AC-13 status cells are stale; one owned journal row open in `plan/journal/comment-describes-superseded-behaviour.md` (the `ze:filter` marker documented as discovery but never read) | medium |

Hard ones, not quick wins: `spec-rfc-verdict-fix-access` (six Ze-kernel lab
acceptance runs), `spec-plugin-reload-applies-or-refuses` (five open journal
rows, an unreviewed round-4 fix), `spec-refuse-instead-of-defaulting` (two
producers still answer nil instead of refusing; an interop scenario never run).

### Not done here, and who owns it

- 22 lint findings in IKE (14 hugeParam on a 288-byte peer type, 5 intrange,
  3 goconst) come from another session's uncommitted RFC 4301 DF/ICMP-selector
  work in `internal/component/ike/ipsec/` and new untracked files. That session
  owes its own lint before committing.
- `TestEkuPeerCertificateIsNotJudgedByTLSExtendedKeyUsage` fails inside another
  session's in-progress RFC 4945 EKU edit of `remote_cert_eku_test.go`.
- Journal rows written this session, each owned by an open spec:
  `TestBridgeSetSTPKernel` NUL byte (parser-refuses-a-form-its-producer-emits,
  owned by spec-doctor-coverage-2-home-cpe-batch);
  `TestPPPoEClientAlreadyAbsentPair` (package-red-not-reproduced, owned by
  spec-ppp-l2tp-aaa-one-off-defects); kernelbuilder raw `docker`
  (gate-red-where-nothing-blocks-on-it, owned by spec-tooling-one-off-defects).
- `TestOrdinaryRefreshDoesNotDoublePrepend` is a deliberate red owned by
  `plan/release/fixes/spec-bgp-refresh-reapplies-export-policy.md` (ready).
- `./le spec citation` is red on `spec-oauth-login-0-umbrella` citing the
  missing `spec-mgmt-oauth-bearer-and-web-sso`, another session's.

## Lessons that apply to the next closures

- Every lint-backlog scope gained new findings from later commits by other
  sessions; fix them as closure work. golangci-lint caps repeats at 10 per
  linter (`.golangci.yml` `max-same-issues`), so a fixed batch uncovers the next.
- A scoped lint still refuses, correctly, while another session edits a package
  the scope depends on. Wait for a quiet tree (no dirty Go file touched for a
  few minutes) and run it then; don't retry in a loop.
- `./le --name session ...` builds le from the working tree, so another
  session's half-written `internal/le` edit breaks it. Use the shared `./le`
  now that `b032323956` is committed.
- Closure agents run no tests; the main thread runs every test, lint and
  record, one CPU job at a time, and sends results back by message.
- `plan/release/validation/spec-lint-backlog-0-umbrella.md` collects one row per
  child; commit only your own row with `hunks`.
- A commit refused on `tmp/commit-weakening-approved-<ns>.md` or
  `tmp/commit-rfc-approved-<ns>.md` "has no table header" means a stale
  approval file from an older format in that namespace; if its approvals belong
  to a commit that already landed, move it aside.

## Files already handled (no need to re-read)

- `internal/le/go/lint/planinputs.go`, `internal/le/job/inputsnapshot.go`: the
  scoped-lint population and content-change guard, committed in `b032323956`.
- `docs/contributing/running-commands.md`, `docs/architecture/testing/verify-freshness-scope.md`:
  updated for that guard.

## Then

Start a fresh session, pick one of the three medium specs above, and run
`/ze-close` on it through one agent, with every test, lint and record run in
the main thread.
