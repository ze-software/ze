# Reproducing Load-Dependent (Flaky-in-Full-Verify) Failures

**When:** when a functional-test failure appears only in a full `make ze-verify` run and will not reproduce in isolation
**Severity:** advisory
**Related:** fix-dont-record, no-parking, testing

## Directives

**This rule is about DIAGNOSING such a failure. The outcome is always a fix.**
Load-dependence is the diagnosis -- the test asserts on elapsed time instead of
on state -- and `ai/rules/fix-dont-record.md` bans recording it as a
`plan/known-failures/` shard, bans "passes in isolation" as a conclusion, and
bans raising the timeout. Reproduce here, then go fix the timing assumption.

Some failures only surface under the scheduling and GC pressure of the full
~22-suite run (many concurrent `ze` daemons on all cores). Rerunning the single
suite never triggers them, and looping the full suite to hunt the bug is
impractical (minutes per run, low hit rate). The verify aggregator also
truncates the crashing daemon's goroutine stack to ~2 lines, so the crash site
is usually lost.

## Use the stress reproducer, not the full suite

`scripts/dev/stress-repro.py <suite>` recreates that pressure cheaply: CPU + GC
"burner" processes oversubscribe every core while many concurrent copies of one
suite loop, and it captures the FIRST failure's complete, untruncated output.

```
python3 scripts/dev/stress-repro.py rsvpte --iterations 80         # hunt + capture the stack
python3 scripts/dev/stress-repro.py rsvpte --race                  # data race self-reports its two accesses
python3 scripts/dev/stress-repro.py bgp --burners 32 --parallel 8  # more pressure
python3 scripts/dev/stress-repro.py "bgp plugin" --test 97 --any-failure  # sub-suite, one test, assertion flake
```

`<suite>` and `--test` are both split on whitespace, so a sub-suite and a
multi-token selector reach `ze-test` exactly as you would type them by hand.

**A crash is not the only reproduction.** By default only a CRASH signature
(panic / `DATA RACE` / runtime error) counts, and everything else is discarded
down to the last 500 bytes. An assertion flake -- a test whose `expect=` pattern
is merely missed under load -- exits non-zero with no crash signature, so pass
`--any-failure` or the run reports "not reproduced" while quietly throwing the
evidence away.

It sets `GOTRACEBACK=all` so a panic dumps every goroutine (the one racing on
the corrupt buffer shows up next to the crasher), reuses the prebuilt
`bin/ze`/`bin/ze-test` via `ze.bin` + `ZE_TEST_NO_BUILD` (no rebuilds under
load), and writes the full capture to `tmp/stress-repro/<slug>-<ts>.log`. Exit
0 = reproduced, 1 = not reproduced, 2 = setup error.

**`ZE_TEST_NO_BUILD=1` means the run tests whatever `bin/ze` already is.** After
changing daemon source, rebuild before you trust a verdict -- otherwise a fixed
bug still "reproduces" against the stale binary. `bin/ze-test <suite> <test>`
once (no `ZE_TEST_NO_BUILD`) rebuilds both binaries.

## Rules

- **Never loop `make ze-functional-test` / `make ze-verify` to hunt a flake.**
  Use the stress reproducer against the suspected suite.
- **Static-clear the hypothesized site before trusting it.** Read the function
  that PRODUCES the crash (the reslice, the buffer allocation), not a byte-count
  inference (`ai/rules/no-fabrication.md`). The `rsvpte-lsp` "cap-512
  share-registry" diagnosis in `plan/known-failures/` was inference from the
  5448-byte payload size and did not survive reading the producers — the send
  path is `json.Marshal` + `append` with no 512-cap buffer.
- **If it will not reproduce under stress AND the site is statically clear,**
  suspect misattribution (the aggregator tagged another concurrent suite's crash
  to this one) or an already-landed fix, rather than "fixing" a phantom. That is
  the one case a shard may record, and only while you are still driving it
  (`ai/rules/fix-dont-record.md`). It does not apply once you can name load as
  the cause: that is a mechanism, and it gets fixed.
- A genuine reproduction's log (`tmp/stress-repro/…`) carries the real stack —
  attach it when filing or fixing the bug.
