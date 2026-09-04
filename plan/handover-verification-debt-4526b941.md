# Handover: clearing the verification debt of session 4526b941

Written 2026-09-04 by the session that incurred it. Read this before you start;
it names what the debt is, what it is NOT, and the one decision that has to be
made before any of it can be cleared.

## What the verification proved (2026-09-04, session 2d2bc99a)

The gate has now run. `./le commit debt-clear` verified `36e9a2f31be7` in a
detached worktree, the full 48-stage population: **22 stages red, 0 rows
cleared, 2459 still open.** Read the section below with that in front of it.

**The claim under this heading is false.** "No row means a test failed" was
written before anything ran. Two of this session's own commits are red at HEAD:

- **`557f401028` broke the tracked build.** It committed a consumer,
  `internal/component/plugin/process/process.go:656`, that calls
  `p.acceptor.RootPEM()`, while the file DEFINING that method,
  `internal/component/plugin/ipc/tls.go`, stays uncommitted in this shared
  checkout. HEAD's copy of that file does not contain `RootPEM` at all. This is
  the exact thing `ai/rules/precommit-verify.md` forbids, and it is what a full
  native verification exists to catch. About twelve of the 22 reds are this one
  break: lint in every flavor, all six staticcheck parts, tracked-build,
  platform-vet, unit-cached, alloc, and the .ci cache warm.
- **`297b790446` changed byte-pinned bytes and left the pin.** The committed
  `test/interop-l2tp/scenarios/04-radius-acct-attrs/ze.conf` hashes to
  `2635d839...`; `test/interop-l2tp/parity_test.go:70` still pins `5773b223...`,
  last touched by `eae2825926`. `TestNativeConfigBytesArePinned` is red. The
  section below calls that scenario "corrected and UNVERIFIED". It is verified
  now.

**Do not try to clear the first one by committing the producer.** `tls.go` is
the middle of a live refactor that replaces fingerprint pinning with a CA root
trust model: it deletes `CertFingerprint`, `TLSConfigWithFingerprint` and
`GenerateSelfSignedCert` and changes `NewPluginAcceptor` and `StartListeners`,
and it moves with `tls_test.go`, `acceptor.go`, the untracked `leaf.go` and
`server/managed_serve.go`. Landing any subset makes HEAD worse. It belongs to
whoever owns the pki work.

**`./le commit debt-clear` exits 0 on a red run.** `clearDebt`
(`internal/le/commit/actions.go:311`) returns 1 only when `report.Verify` is
nil, so a red population that produced a report still exits 0. The verdict is
the `verify-worktree: full exit=` line, never the status.

**The decision below is settled by the tooling, not by you.** `debt-clear` calls
`verify.Run(..., Options{Commit: "HEAD"})` itself, which IS the detached-worktree
run. There is no in-place option to refuse, and `./le verify worktree` first
only buys the same verdict for a second full population.

**`debt-clear` is repo-wide.** It collects gate names from every shard and, on a
green, clears every open row carrying one. That is all 2459 rows across 111
shards, not this session's 47.

**The discovery-index rows are cleared of their blocker.** HEAD's
`ai/PACKAGE-MAP.md` was stale against HEAD's own tree, not against uncommitted
work: `c54d97dcdb` moved the EAP peer to `internal/core/eap` and the map kept the
`internal/component/ike/eap` row, so `discovery-index/check` was a guaranteed red
for every commit made since. `f2917cde4` carries that one row and stage 37 now
reports "checked 753 packages, ai/PACKAGE-MAP.md up to date". The full
regeneration this file asks for below is still owed, once
`internal/le/interoplab/radius` and `internal/component/lg/register.go` land.

**Disk.** The sweep reclaimed one abandoned verify worktree and spared
`20260903T071206...-150bac9b3c75`: 23h43m old, at least 7.4G, 264 deleted files,
owner pid 74412 dead. It was spared for holding uncommitted changes, but those
are deletions from an interrupted run. On a 22G disk it is the cheapest space
available.

## What you are taking on

`plan/verification-debt/4526b941.md` holds 47 open rows across the commits one
session made on 2026-09-03 and 2026-09-04. Every row exists because the commit
gate refuses to drop a local commit and records the gate it could not prove
instead. **No row means a test failed.** They mean a gate never ran.

| Rows | Gate owed | What it really means |
|------|-----------|----------------------|
| 23 | full native verification (not FRESH-green) | `./le verify status` has no status file at all for this checkout. Not stale, never written |
| 14 | full native verification over this commit's Go | the same absence, recorded per commit that carried Go |
| 10 | discovery-index freshness | `ai/PACKAGE-MAP.md` is stale against packages OTHER sessions committed. Each of these carries a `stale-index-ok` reason saying the commit added no package |

The 23 and the 14 are the same fact counted two ways: one full green native
verification over this tree clears both families. The 10 are different and are
discussed below.

## The decision that comes first

**This is a shared checkout.** At the time of writing, `git status` shows around
140 modified files and several untracked ones, and almost none of them belong to
session 4526b941. Other sessions are actively editing `internal/component/pki`,
`internal/component/l2tp/plugins/authradius`, `internal/test/fixture` and more.

That matters because a full native verification verifies THE TREE, not a commit.
Run it now and you are verifying other people's work in progress, and it will
fail on their half-finished packages rather than on anything this debt is about.
It already would have: on 2026-09-03 `internal/component/pki` did not compile for
about fifteen minutes while another session was mid-edit, and a cross-build in
that window failed with `"strings" imported and not used`.

So decide, with Thomas, which of these you are doing:

1. **Verify a clean tree.** `./le verify worktree` runs in a detached worktree
   against a fixed commit, which is the only way to get a verdict about the
   commits rather than about the current tree. This is the right answer and it
   is what the rows ask for.
2. **Wait for the tree to quiesce** and verify in place. Cheaper to set up,
   worthless if anyone commits while it runs.

Do not pick 2 because it is quicker. A green from a tree containing four
sessions' uncommitted work does not certify these commits, and clearing rows on
it would be worse than leaving them open.

## How to clear a row once a gate is green

Rows are cleared ONLY through `./le commit debt-clear` after the named gate
exits 0. Do not hand-edit `plan/verification-debt/4526b941.md`. The file is the
record, not the mechanism, and an edited row is indistinguishable from a
cleared one to the next reader.

## The 10 discovery-index rows are not yours to fix by regenerating

Each carries a `stale-index-ok` reason of the same shape: the commit added no
package, `ai/PACKAGE-MAP.md` was unmodified in the tree, and it was stale
against packages other sessions had committed that day. Running
`./le discovery-index update` inside one of those commits would have swept other
sessions' package additions into it, which is why it was not done.

The right clear for these is a single regeneration of `ai/PACKAGE-MAP.md` on a
quiet tree, committed on its own, by whoever is doing tree hygiene. It is not
per-commit work and it is not blocked on the verification above.

## What these commits actually are

So you know what you would be certifying. Session 4526b941 landed, in order:

| Commit | Subject |
|--------|---------|
| `bab29e430` | the radius-acct-wire peer fixture performs a real CHAP-MD5 exchange |
| `643c160d8` | that test's config states the auth it performs (owner-approved RFC-tagged change) |
| `007b7d724` | the runtime kernel conntrack symbol is `NF_CT_NETLINK`, not `NF_CONNTRACK_NETLINK` |
| `557f40102` | the engine directory is a PATH fallback for spawned plugins, not an override |
| `297b79044` | the flap test's overlap assertion is scoped to rounds that overlapped |
| `4f91e6119` | a fixit spec for the flap stimulus (premise later proved wrong) |
| `055b97a29` | the flap overlap counter was blind to the block it counts |
| `48ad84d9c`, `e17e5dc3d` | that spec and its journal row corrected where they named the wrong cause |
| `9fb102360` | `ze_iface_config_applies_total` |

Functional evidence already taken, all in QEMU on the arm64 runtime kernel 7.2,
none of it a substitute for the native verification the rows ask for:

- `test/l2tp` suite: 23 of 23.
- `test/plugin/ddos-detect-characterize` and `ddos-incident-confidence`: green,
  each red before its fix.
- `test/plugin/iface-link-flap-during-commit`: six green runs across two fixes.
- Host `bgp plugin` suite: nine failures, all nine compared BY NAME against a
  clean HEAD worktree. Four fail identically at HEAD; the fifth passes three of
  three in isolation in both trees and fails only under suite parallelism, in
  both. None is attributable to these commits. Compare by name, never by test
  id: ids are positional and a shared tree has uncommitted `.ci` files, so id
  576 named a different test in each tree.

## Traps this session actually hit

- **`df` is aliased to GNU `gdf` here** and rejects `-g`. A disk check written
  as `df -g` silently produced an empty string and fired a false alarm. Use
  `df -k / | awk 'NR==2{print int($4/1048576)}'`.
- **Disk.** A runtime kernel build needs 40G free and the guard enforces it. The
  tree was at 20G when this was written. `colima ssh -- sudo fstrim -v
  /var/lib/docker` returned 6.8 GiB without deleting anything; `docker system
  prune` would destroy other sessions' build caches, so it was not run.
- **A `pgrep -f "<pattern>"` waiter matches its own command line** and never
  exits. One ran for two hours here.
- **The runner truncates a failure report to the FIRST 30 lines**
  (`truncateOutput`, `internal/test/runner/report.go`), so a diagnostic printed
  late in a fixture is relayed and then cut. It looks exactly like output that
  was never produced.
- **The daemon runs at WARN in every `.ci` run.** The runner's `SLOG_LEVEL` is
  dead code; `slogutil` reads only `ze.log*`. The iface component's subsystem is
  `interface`, so the knob is `ze.log.interface` and `ze.log.iface` is wrong.
- **`bin/le` is older than committed sources** and has been warning on every
  invocation. `./le --update` fixes it; it was left alone because it is shared.

## What is still open beside the debt

- `test/interop-l2tp/scenarios/04-radius-acct-attrs` was corrected in
  `297b79044` and is UNVERIFIED. It needs a Linux host with `l2tp_ppp`; colima's
  kernel has none and there is no QEMU guest lab for that suite. Session
  `fix-and-radius` owns it now and has the full diagnosis.
- RADIUS accounting-only is unconfigurable: configuring a server for accounting
  also claims the single auth slot. Row in
  `plan/journal/unwired-feature.md`. Another session is taking it.
- `plan/spec-fixit-flap-test-cannot-build-its-own-stimulus.md` is resolved but
  not closed. `./le commit create` refuses a `remove` of a spec with no
  independent-review artifact.
