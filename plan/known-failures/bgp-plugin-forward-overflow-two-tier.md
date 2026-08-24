### `ze-test bgp plugin` forward-overflow-two-tier -- one failure on 2026-08-08, NOT reproduced since

Observed ONCE, during the repair of GitHub Actions run 31225029268, and not one
of that run's own failures. The symptom was an `ordered:` needle against a
246-byte `fwdBucketMerge` UPDATE. It was never attributed to any change in the
tree.

`spec-fixit-forward-overflow-two-tier-flake` was written to answer one question
first: does it reproduce at all? It closed on 2026-08-15 on this shard's answer,
so the spec is no longer in the tree. This shard records that answer, which is
the condition `ai/rules/completion.md` sets for recording a failure instead of
fixing it. A shard without a reproduction attempt on the record would not have
been legitimate, and the spec said so itself.

## The attempt, 2026-08-15

| Setting | Value |
|---------|-------|
| Tool | `scripts/dev/stress-repro.py "bgp plugin" --test forward-overflow-two-tier --any-failure` |
| Invocations | 80 |
| Concurrent `ze-test` processes | 8 (tool default, `max(2, NCPU//2)`, NOT chosen) |
| CPU/GC burners | 32 (tool default, `2*NCPU`, NOT chosen) |
| `ZE_PLUGIN_PARALLEL` | **never set.** See "What the attempt did NOT do" |
| Run alone (no burners, no concurrency) | **never run** |
| Race build | no |
| Result | **not reproduced** |
| Capture | `tmp/stress-repro/bgp-plugin-forward-overflow-two-tier-20260815-134636.log` (scratch, not durable) |
| Test variant | the working tree's, NOT the 2026-08-08 one. See "What was actually run" below |

`--any-failure` was set, so ANY non-zero exit would have counted, not only a
crash signature. That part of the attempt is as sensitive as this tool gets.

## What the attempt did NOT do

The spec named two conditions, and neither was run. Recording this is the whole
point of the shard, so it is stated before the result is used.

**`ZE_PLUGIN_PARALLEL` was never raised, and the reason first recorded here was
wrong.** `--parallel` is a DIFFERENT knob: its own help calls it "concurrent
invocations per round", and it is the `max_workers` of the `ThreadPoolExecutor`
that launches whole `ze-test` processes. Plugin-level parallelism inside one
runner was never raised, which is what the spec's Task asked for.

An earlier draft of this shard said `run_once` (`scripts/dev/stress-repro.py`)
cannot set it, because that function builds the child environment with
`ze.bin`, `ze.test.bin`, `ZE_TEST_NO_BUILD` and `GOTRACEBACK`. That reasoning
does not hold: the same function opens with `env = dict(os.environ)`, so the
variable is INHERITED from the caller. The real reason raising it changes
nothing here is different and simpler. `ZE_PLUGIN_PARALLEL` is a make variable
(`mk/test-functional.mk`) that becomes `-p N` on the runner's command line, and
`-p` bounds how many `.ci` tests run CONCURRENTLY. With one test selected there
is nothing to run it beside, so the knob is inert for a single-test stress run
whatever its value.

**So the spec's condition means the WHOLE suite at high `-p`**, where this test
contends with the other 600, not this test alone under a raised number. That is
a different run, and it is cheap.

**It was never run alone.** Both captures used burners and concurrency. The
Task asked for the isolated case first, and no run answers it.

**The two numbers were defaults, not a reconstruction.** `parallel = args.parallel
or max(2, ncpu // 2)` and `nburn = args.burners if args.burners else 2 * ncpu`.
On a 16-CPU host that is exactly the 8 and 32 in the table. An earlier draft of
this shard called them "the profile that surfaced the other 2026-08-08
failures". That was not true and has been removed: nothing was carried over
from those runs.

So the negative is real but NARROWER than the spec asked for. It says this test
survived 80 oversubscribed process-parallel runs. It says nothing about the
isolated case, and nothing about raised plugin parallelism.

## What was actually run, and how far the negative reaches

The negative bounds the test AS IT STOOD ON 2026-08-15, and that is not the
configuration that failed on 2026-08-08. `test/plugin/forward-overflow-two-tier.ci`
carried an uncommitted edit (mtime 04:52, before the 13:46 capture) belonging to
another session's config-grammar migration. It is more than a rename:

| Change | Effect on what the run exercised |
|--------|----------------------------------|
| `process overflow-test { receive [ update state ] }` removed from the first peer | the external `overflow-test` plugin is still DECLARED but attached to no peer, so it no longer observes the forwarded stream |
| `attach process rs` added to BOTH peers | the run exercises the route-server path, which the 2026-08-08 run did not |
| `attach process adj-rib-in` added to both peers | adj-rib-in now attaches by config instead of by `--plugin` |
| `exec=ze --plugin ze.bgp-adj-rib-in -` became `exec=ze -` | the plugin set comes from the config, not the command line |

What did NOT change TEXTUALLY: all 50 `ordered=` needles, `expect=exit:code=0`,
every `reject=` line, and the 20s timeout. The `ordered=` needles are the ones
the 2026-08-08 failure tripped, and they were evaluated 80 times. The forwarding
path under test is the same one; its observers and its peer roles are not.

One caveat on the `reject=` lines, unverified: the detached `overflow-test`
plugin is also the source of the `ZE-OBSERVER-FAIL` output and the deterministic
drain. Whether a declared-but-unattached external plugin still starts decides
whether those lines stayed LIVE or merely stayed present. Nobody established it.
The `ordered=` claim does not depend on this; the "every `reject=` line" claim
does.

This is why the next step below names the committed variant. A negative measured
on a changed fixture is a real negative about a real test, and it is not
interchangeable with a negative about the fixture that failed.

## A false reproduction, and how to avoid repeating it

The FIRST run of that same command reported `*** REPRODUCED on invocation 1 ***`
and the reproduction was an artifact. The capture showed `unknown field in
peer: attach`, a config grammar the working tree parses and a day-old binary did
not. Nothing about the suspected flake was exercised.

The cause is a documented trap the tool still carries: `_bin_from_env`
(`scripts/dev/stress-repro.py`) honours `ZE_BIN` / `ZE_TEST_BIN` but FALLS BACK
to `bin/ze`, and in this repository that path is stale by construction, because
`mk/helper-session.mk` builds the canonical binaries into a per-session directory.
`ensure_binaries` checks only that the files exist. The tool's own docstring
describes this incident happening once before, in the opposite direction: a fix
under test looked "still reproducing" because the run never contained it.

**So export the binaries before trusting any verdict from this tool**, and
treat a reproduction whose symptom is a CONFIG PARSE ERROR as a stale binary
until proven otherwise. `plan/future/spec-stress-repro-refuses-a-stale-binary.md`
proposes making the tool refuse rather than rely on the caller remembering.

## Next step

Not "run it again the same way": that has been done and it answers no. The first
two entries are not new ideas, they are the spec's own conditions, still unrun.

1. With `ZE_PLUGIN_PARALLEL` well above the core count. This is the setting the
   spec named and the attempt never set. `stress-repro.py` cannot set it, so
   export it around the runner, or run `ze-test` directly.
2. Alone: one invocation, no burners, no concurrency. The Task asked for the
   isolated case and no run answers it.
3. Against the COMMITTED variant of the `.ci`, which is the fixture that actually
   failed. The 80 invocations ran the working tree's migrated copy (above).
4. Under `--race`, which changes scheduling enough to surface ordering bugs that
   a plain build hides, and which this attempt did not use.
5. On Linux rather than darwin. The original failure came from GitHub Actions,
   and the two platforms differ in loopback bind timing, which this suite has
   already been bitten by elsewhere.
6. With `--burners` and `--parallel` raised beyond the defaults above, which is
   what the tool's own miss message suggests.

Until one of those reproduces it, there is nothing to root-cause. The test is
NOT quarantined, NOT weakened and NOT skipped: it runs in the suite on every
invocation, and a second occurrence gets appended here with its capture.
