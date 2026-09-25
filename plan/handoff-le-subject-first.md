# Handoff: le subject-first command tree (2026-09-25)

The spec `spec-le-subject-first-command-tree` is implemented and closed.
Learned summary: `plan/learned/027-le-names-its-subject-first.md`.
Last commits: `c36fd3fab0` (closure), then the fixture and gate fix on top.

## What changed

- Every `./le` command is `./le <subject> <action>`. `./le` lists them; a bare
  group (`./le spec`, `./le test`) lists its members and exits 0.
- Old names are removed, not aliased. The map of old to new names lives in
  `internal/le/doc/check/retirednames.go`. `./le doc check retired-commands`
  fails on any old name outside declared exceptions and runs in full verify.
- The separate `ze-test`/`le-test` harness binary is gone: harness commands are
  `le test <name>` (`le test peer`, `le test fixture …`). `.ci` files use
  `exec=le test <name>`. Off-host images (interop, QEMU, VPP) carry a linux
  `le` built by `internal/le/linuxle`.
- `ze-chaos`, `ze-perf`, `ze-analyze`, `ze-gok`, `ze-perf-run` and
  `ze-terminal-pty` are gone: `le chaos run`, `le perf …`, `le mrt …`,
  `le build gokrazy`, `le site terminal-demo pty`.
- Framework packages moved: `internal/le/le/{action,path,root}`,
  `internal/le/go/toolchain`, `internal/le/spec/path` (package names unchanged).

## State at handoff

| Check | Result |
|---|---|
| `./le go lint run` | exit 0, every flavor |
| `./le doc check retired-commands` | 0 lines |
| functional ospf, runner | 76/76, 14/14 |
| functional ui | 266/268 in the last full run; both reds fixed after, proven by single-fixture runs, no full rerun since |
| `./le test unit all` | red only on the `pppoeclient` data race (another session's, journaled in `plan/journal/test-against-broken-path.md`) |

## Open items (not part of the rename)

1. Rerun `./le test functional ui` once to confirm 268/268.
2. `docs/features/test-health.md` and `test/health/latest.json` still name old
   paths. Regenerate with `./le test health update` after the session holding
   uncommitted edits to them commits. They are a declared gate exception until then.
3. Three orphaned `le test radius-mock` processes (parent pid 1) were left by an
   earlier functional run; cause not found.
4. 175+ local commits are not pushed. A push needs the owner's order and goes
   through `./le commit create … push "<authorisation>"`; the verification debt
   in `plan/verification-debt/c392fa50.md` must be cleared first.
5. `website/data/wiki.json` is built from the separate `../wiki` repository,
   which still names the old programs; fix it there, then `./le site wiki update`.
6. Journaled, not fixed: the pretool-bash hook's governed-tree guard cannot see
   paths produced at run time (`plan/journal/gate-excludes-part-of-its-population.md`).

## Where the detail is

Per-phase handoffs: `tmp/session/2026-09-24-2dab35f2-57c1-4bbd-a05a-00afe1b5f87d/state/`
(local to this machine). Test-change reasons: `test/weakened/c392fa50.md` history.
