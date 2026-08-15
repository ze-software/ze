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
| Parallelism | 8 |
| CPU/GC burners | 32, on a 16-CPU host |
| Race build | no |
| Result | **not reproduced** |
| Capture | `tmp/stress-repro/bgp-plugin-forward-overflow-two-tier-20260815-134636.log` (scratch, not durable) |
| Test variant | the working tree's, NOT the 2026-08-08 one. See "What was actually run" below |

`--any-failure` was set, so ANY non-zero exit would have counted, not only a
crash signature. The burner and parallelism profile is the one that surfaced
the other failures in this suite on 2026-08-08.

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

What did NOT change: all 50 `ordered=` needles, `expect=exit:code=0`, every
`reject=` line, and the 20s timeout. So the assertions the 2026-08-08 failure
tripped are intact and were evaluated 80 times. The forwarding path under test
is the same one; its observers and its peer roles are not.

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
`mk/session.mk` builds the canonical binaries into a per-session directory.
`ensure_binaries` checks only that the files exist. The tool's own docstring
describes this incident happening once before, in the opposite direction: a fix
under test looked "still reproducing" because the run never contained it.

**So export the binaries before trusting any verdict from this tool**, and
treat a reproduction whose symptom is a CONFIG PARSE ERROR as a stale binary
until proven otherwise. `plan/future/spec-stress-repro-refuses-a-stale-binary.md`
proposes making the tool refuse rather than rely on the caller remembering.

## Next step

Not "run it again the same way": that has been done and it answers no. The next
run that could say something new is one of these, in this order.

1. Against the COMMITTED variant of the `.ci`, which is the fixture that actually
   failed. The 80 invocations ran the working tree's migrated copy (above), so
   this is the cheapest run that closes a real gap rather than adding volume.
2. Under `--race`, which changes scheduling enough to surface ordering bugs that
   a plain build hides, and which this attempt did not use.
3. On Linux rather than darwin. The original failure came from GitHub Actions,
   and the two platforms differ in loopback bind timing, which this suite has
   already been bitten by elsewhere.
4. With `--burners` and `--parallel` raised beyond the settings above, which is
   what the tool's own miss message suggests.

Until one of those reproduces it, there is nothing to root-cause. The test is
NOT quarantined, NOT weakened and NOT skipped: it runs in the suite on every
invocation, and a second occurrence gets appended here with its capture.
