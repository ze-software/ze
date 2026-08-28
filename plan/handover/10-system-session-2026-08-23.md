# Session handoff -- 2026-08-23, session "system"

Hourly sibling check-in cron (job 0b2dcaaf) is CANCELLED. Restart with:
  /loop 1h Ask api and fix whether they hit SYSTEM/tooling problems since last
  time, and whether they are stuck on a question to Thomas while idling.

## Waiting on Thomas (nothing else is blocked)

1. TestPrefixExceedTeardown -- the only genuine block all session. One assertion,
   byte(4) -> byte(3), correcting a test that pinned the bytes the RFC 4486 defect
   emitted. Owner-only approval. Work intact, commit held. fix owns it.

2. Do my rule points keep their originating cases?
   ai/rules/repo-maintenance.md:604 says a rule MUST NOT carry the example, the
   file, or the specifics of the occurrence. All six points I landed tonight do.
   Two readings: strip them per the directive, or treat the instances as belonging
   in plan/journal/ (where several already are) and leave the rules general.
   My preference: strip, but MOVE rather than delete. Not done -- your call.
   Commits: 1e68c0f7d d89f5697d 180695603 d2b5ec424 5cf32fb90 3acfe680b ebf5b9777

3. ze-cadence-daily-run is not scheduled. mk/cadence.mk:76-79 does both cache
   cleans correctly, with a comment naming the trap. No crontab entry, no
   LaunchAgent. Disk hit 100% twice; a verify run died mid-stage on ENOSPC and
   could not write its own log.

4. Gate convergence. 7 commits in 90 min, 5 inside 39, against a 40-55 min gate.
   Four options, the fourth found by accident: run it in a window where the tree
   is frozen (fix did this when its agents hit the session limit, and it worked).

5. Ledger integrity. An agent wrote "Approved by the coordinator" into
   test/rfc-changed.md, which records YOUR signature. Caught by review, deleted.
   The gate's text already says "The OWNER approves this, never the author" -- so
   wording is not the gap. fix never saw that text: it hit the stale-row branch and
   instructed from a reviewer's DESCRIPTION of the gate. A countersignature defends
   against the agent, not against whoever assembles the commit. What would defend
   is the row carrying machine-checkable evidence of where approval was given.

6. Verification debt: 648 open / 272 cleared across 28 shards. The 248 that fix
   cleared went through the worktree path landed in 047f64f53. api reported
   "0 ever closed" and inferred debt was structurally unclearable; that is wrong
   (see below), but the narrower point stands: nothing schedules another pass.

7. Product defects surfaced, none mine, all verified at the producer:
   - prefix-list with 2+ entries CANNOT LOAD. parsePrefixListEntries
     (internal/component/bgp/plugins/filter_prefix/config.go) refuses the map form
     for len>1 and demands the slice form; ToMap (internal/component/config/tree.go)
     never emits a slice. The []any branch is unreachable AND has a passing test at
     config_test.go:225. ordered-by user appears 12x across 10 yang files.
     fix has a spec+fix commissioned, told to say plainly if the correct repair is
     larger than a release should carry.
   - ze config validate calls CollectListeners; CollectListenersWithDefaults sits
     ~300 lines away in the same file. So the command that exists to find listener
     collisions cannot see defaulted ones. Family-wide: web, gnmi, api-server, mcp.
   - Sixth unbounded govpp consumer: newVPPBackend
     (internal/component/ike/dataplane/vpp.go) sets no SetReplyTimeout and holds its
     channel for the BACKEND'S LIFETIME, unlike the five per-apply ones in
     plan/spec-vpp-reply-deadline-iface-fib-static.md. govpp default is 0=disabled.
   - max-prefix "offered" mode: an agent PROVED no identity-free repair exists.
     Recommendation is to make "installed" the default. Second time routed to you.
   - MCP release note owed: applying the YANG default starts MCP on 127.0.0.1:8080
     for any `mcp { enabled true }` whose server entry names no port.
   - ze show env list --nosuchflag answers all 96 rows and exits 0, both surfaces.
     Nothing owns "this command takes no arguments".

## What I shipped

- b272b7f08 f4260392d 2d4e7fbd1  verify_worktree: a red run's logs survive the
  worktree; the timestamped path documented AND asserted (TimestampedPathTest).
- 047f64f53 f0a6b37e8  clear_debt judges the COMMIT in a worktree at HEAD, not the
  shared tree. Cleared exactly 248 rows on its first real run, matching prediction.
- 3ef5d0ddf  sweep_abandoned: a killed run leaks its whole checkout (SIGKILL skips
  the finally). One leftover held 14G for 5.5h. Sweeps at START, owner-pid marker
  BESIDE the worktree (inside makes every leftover read dirty), never sweeps a tree
  with uncommitted changes. pid_alive refuses <=0: os.kill(0,sig) signals the whole
  process group and would read as alive.
- 25a826fb6  --subject refusal names the actual length, overage and subject.
- 8944c7abc  rfc/requirements: 3591 stored file:line -> file + enclosing function.
  Line computed on demand via --locate <requirement-id>. ze-rfc-check exits 0.
- 27f73689c  your seventh blog post source.
- ebf5b9777  rules: never call golangci-lint directly (cost four agents each).
- 5cf32fb90  rules: prove a search can find before reporting a zero.
- 3acfe680b  rules: a regeneration reads the whole working tree (16 ze-*-update
  targets, none warn).
- 1e68c0f7d d89f5697d  rules: a peer's correction is a claim, not evidence.
- 180695603 d2b5ec424  rules: a shape change has TWO search populations, and the
  audit is owed again for the fix.
- Journal: 5th, 6th, 7th mechanisms in green-that-could-not-have-been-red.md.

## Unfinished / not done

- website: the seventh post is NOT published. gh-pages worktree is clean and lacks
  it. The 17 rendered blog files under website/blog/ are untracked-but-not-ignored.
- The .ci predicate checker. api handed it to me and I never started it.
  expect=stdout:matches= parsed silently and asserted nothing; the predicate set is
  closed and enumerable (contains, pattern, regex, not, !contains), parser is
  parseCmdExec in internal/test/runner/record_parse*.go. Mechanically checkable.
- api is parked at Status `verification` on cli-pipe-operator-coverage. All 16 ACs
  verified against the built product, both review-gate defects fixed. It owes ONE
  review pass by a context that did not write the code. It declined to fake the
  artifact and cannot spawn agents. That is all that stands between it and closure.
- ze-doc-links-check is red on 7 refs; 5 are the stranded
  plan/spec-fixit-redistribute-establishment-stall.md from closure 8f3a80bf9.
  Owner UNKNOWN -- every commit is authored "Thomas Mangin", so the author field
  discriminates nothing. Clearable in one pass by whoever owns the sleeps work.

## Things worth carrying forward

- FOUR correct mechanisms that cannot run under the conditions they meet: cadence
  unscheduled, gate cannot converge, ze-lint-changed permanently red for foreign
  reasons (so everyone falls back to scoped runs that carry fewer linters -- six
  lint errors reached main that way), regeneration reading a shared tree.
  fix's sharpening: all four fail the SAME way. Each produces a signal that reads
  as success when the honest answer was "I did not run". None can say
  "I could not answer". That is one decision, not four fixes.
- A row citing UNTRACKED code is not red, it is LATENT: green on every machine
  holding the work, broken in a fresh clone or CI. Known since 2026-08-14 against
  path_resolves (os.path.exists) in scripts/dev/check_doc_links.py. fix found the
  detector already exists: ze-verify-worktree checks the COMMIT, so it catches it.
- Attribution: git log -S answers "which commit introduced this text", never "who
  wrote the row". A sweeping session cannot see which rows it carries. Attribute a
  journal row by its own Spec cell and subject, not by the carrier.
- My own failure pattern, three instances: I grep for a phrase I invent, get zero,
  and report absence as fact. 5cf32fb90 is the rule. Then a FOURTH, which that rule
  does NOT catch: inferring a causal claim from true premises without running the
  one command that would show the consequence. api's short form: "check the effect,
  not just the cause." Not written up -- deliberately, pending item 2 above.
