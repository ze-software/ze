# Recurring Patterns

Traps that appear across multiple learned summaries. Read this document at
session start and again whenever you are about to write new code — most
entries describe mistakes the corpus has recorded 5+ times.

Each entry lists:

- **Symptom** — how the trap presents itself.
- **Cause** — the underlying reason it keeps happening.
- **Evidence** — the learned summaries where this pattern appeared.
  Read at least one before concluding this entry applies to your
  situation.
- **Avoid it by** — concrete action phrased so there is one
  interpretation.
- **Recover if you hit it** — what to do after the symptom appears.

Companion documents: `DESIGN-HISTORY.md` (why the code is shaped this way)
and `HOOK-FRICTION.md` (hook-specific workarounds, the most frequent
pattern below).

---

## Tooling friction

### Auto-linter strips newly added imports between Edits

**Symptom.** After an `Edit` that adds an import, the next `Edit` that
uses the imported identifier fails to compile with
`undefined: <identifier>`. Re-adding the import produces the same result.

**Cause.** `auto_linter.sh` runs `goimports -w` on every successful
`Edit` and `Write`. `goimports` deletes any `import` whose identifier is
not referenced in the current file content. Two-Edit sequences
(Edit 1 adds import, Edit 2 adds usage) leave the file in a state where
the import is unused between the two Edits.

**Evidence.** Observed at least 25 times: 288, 410, 437, 440, 449, 450,
462, 477 (twice), 482, 503, 507, 526, 544, 546, 548, 551, 553, 562, 578,
600, 606, 609, 633, 634, 635.

**Avoid it by.**
1. Add the import AND at least one usage of its identifier in a single
   `Edit` call, OR
2. Use `Write` to deliver the whole file in one call.

**Recover if you hit it.** Re-add the import and the usage together in
a single `Edit`.

---

### `block-silent-ignore.sh` rejects bare `default:`

**Symptom.** A `Write` or `Edit` is rejected by hook
`block-silent-ignore.sh`, even though the `default:` branch body
returns an error, logs a warning, or panics.

**Cause.** The hook regex is `default:\s*$`, which matches any
`default:` line followed only by whitespace and end-of-line. The hook
does not inspect the branch body.

**Evidence.** Observed at least 30 times: 259, 288, 292, 389, 447, 451,
477, 503, 513, 514, 534, 548, 555, 556, 559, 560, 561, 562, 563, 574,
584, 585, 594, 595, 596, 598, 606, 614, 621, 627, 631, 634, 635. This
is the single most frequent tooling trap in the corpus.

**Avoid it by.**
1. Rewrite the `switch` as an `if`/`else if`/`else` chain, OR
2. Put the body on the same line: `default: return errUnknown`.

**Recover if you hit it.** Apply (1) or (2) above; do not attempt to
suppress the hook.

---

### `check-existing-patterns.sh` blocks duplicate type or function names

**Symptom.** `Write` of a new `.go` file is rejected because the first
exported `type` or `func` identifier already exists somewhere under
`internal/`.

**Cause.** The hook greps all of `internal/` for the first declared
exported identifier. Generic names (`Config`, `Engine`, `State`,
`Manager`, `Session`, `New`, `Resolver`, `Header`, `Secret`, `Service`,
`Registry`, `Validator`, `Store`) collide almost always.

**Evidence.** Observed at least 15 times: 324, 419, 425, 477, 503, 513,
533, 555, 584, 586, 594, 598, 603, 620, 633.

**Avoid it by.**
1. Use a package-qualified name (`WebConfig` not `Config`,
   `BFDSession` not `Session`), OR
2. `bash` a stub file with a non-colliding first type, then `Edit` the
   real content in.

**Recover if you hit it.** Rename the first declared identifier;
leave later identifiers alone.

---

## Correctness traps

### Silent fall-through in parser or dispatch

**Symptom.** A parser accepts an unknown keyword and silently chooses a
wrong branch; the resulting wire output is malformed; no error is
logged.

**Cause.** A `switch` or dispatch table with no matching case falls
through to a default branch that picks the most common value (e.g.
"treat unknown SAFI as unicast"). The caller has no signal that the
input was unknown.

**Evidence.** At least 20 times across the corpus. Specifically:
046 (parseSAFI fell through to unicast → malformed wire),
047 (parseSAFI again, different code path),
099 (handleReceived silently wrong shape),
108 (route-refresh config silently not applied),
187 (family string stored un-normalised),
189 (`--plugin` flag intercepted as subcommand),
190 (test runner `--json` flag silently ignored),
191 (test runner `cmd:` vs `cmd=` silent mismatch).

**Avoid it by.** When writing a `switch` on input strings, codes, or
kinds: omit `default:`, list all valid cases explicitly, and write a
post-switch `else` (or explicit `if`) that returns an error naming the
unknown value and the valid set.

**Recover if you hit it.** Add the explicit rejection path. Reinforces
`rules/exact-or-reject.md`.

---

### Fallible function returns `(nil, nil)`

**Symptom.** A function returns `(nil, nil)` on an error path. Callers
that do `if err != nil` first and use the result unconditionally
panic with nil-pointer dereference, OR silently treat no-result as
success.

**Cause.** A failure path was written with `return nil, nil` instead of
`return nil, err`. The most common source: `sync.Once` or `OnceValue`
caches the first invocation's result — if that result was `(nil, err)`,
the second call returns the cached `(nil, nil)` and the error is gone.

**Evidence.**
- 079 (`sync.Once` + errors caches the first result; second call loses
  the error).
- 397 (`directResultResponse` returned `(nil, nil)` on marshal error;
  SDK interpreted as success with empty body).

**Avoid it by.**
1. In a function that can fail, every code path must end with either
   `return nil, err` or `return value, nil`.
2. Never use `sync.Once` / `sync.OnceValue` wrapping a callback that
   returns `(value, error)` — use explicit state plus a mutex.

**Recover if you hit it.** Replace `return nil, nil` with the
appropriate error return; rewrite any `sync.Once` guard around fallible
code to use explicit state.

---

### Comments claiming synchronization that no caller provides

**Symptom.** Under concurrent load, a function that "just works" in
single-threaded tests corrupts state, races the race detector, or
produces intermittent failures.

**Cause.** The function has a comment like "externally synchronized"
or "caller holds the lock" — but no caller actually holds that lock.
The comment was aspirational and was never enforced.

**Evidence.** 279 (`writeMessage` claimed external synchronization; the
keepalive timer called it concurrently with `sendInitialRoutes` from an
independent goroutine).

**Avoid it by.** Do not write synchronization comments without a
runtime assertion. If you find such a comment while reading code,
grep every caller and verify the claim. When in doubt, add the lock.

**Recover if you hit it.** Add the missing lock; delete the false
comment.

---

### Hardcoded enumeration counts in tests

**Symptom.** Every feature addition breaks one or more test files with
an assertion like `assert len(rpcs) == 14`. Merges between sessions
conflict when two sessions add different features.

**Cause.** A test asserts a literal count of registered items (RPCs,
commands, peers, plugins). The literal must be kept in lockstep with
reality — across sessions, across features.

**Evidence.** Observed at least 10 times: 278, 318, 374, 375, 396, 400,
431, 448 (4 times in quick succession across the cmd refactor), and
the `TestAllPluginsRegistered` / `TestAvailablePlugins` pair that
breaks whenever a plugin is added.

**Avoid it by.** Assertions that count registered items MUST read
the count from the registry, not a literal. If you need a regression
gate, use `>= min_expected` against a checked-in floor and document
what removing an entry is meant to look like.

**Recover if you hit it.** Replace the literal with a registry query.
If the test's intent was to detect removal, keep a `>= N` check with
a comment naming the intent.

---

### Bulk rename scripts corrupt context-sensitive uses

**Symptom.** After a bulk `sed`/`perl`/Python rename of identifier
`foo` to `bar`, tests that passed on the old name fail with bizarre
errors: missing map keys, wrong slog attributes, wrong YANG leaves.

**Cause.** The rename substitution matched the identifier when it
appeared as:
- a map-literal string key (`{"foo": foo}` → `{"bar": bar}`);
- an `slog` key-value argument (`slog.Info(..., "foo", foo)` → `"bar", bar`);
- a `GetContainer("foo")` / `Get("foo")` argument;
- a YANG leaf name referenced from Go as a string;
- a cross-reference in `// Related:` / `// Design:` comments.

**Evidence.** 537 (family rename: renamed `family` to `fam`, which hit
map keys, slog kv pairs, `GetContainer` arguments, and migration
fallthrough). 133, 135, 137, 138, 395 (bulk sed on `.md` files
corrupted "before/after" examples and bulk sed on registration lines
over-deleted).

**Avoid it by.** Before `--apply` of any bulk rename, review the preview
diff for:
1. map-literal string keys where key matches the variable name;
2. `slog` `"<key>", <value>` pairs;
3. `GetContainer` / `Get` / `RegisterModule` string arguments;
4. YANG leaf names used as strings in Go;
5. cross-reference comments and docs that quote the old name as an example.

If any match is ambiguous, do not use bulk rename — edit file-by-file.

**Recover if you hit it.** Revert the bulk rename. Re-do the rename
manually for each affected file.

---

### Signed subtraction for sequence-number ordering

**Symptom.** A sequence-number comparison works correctly for most
diff values, but fails at the exact half-space boundary
(e.g. `diff = 32768` for 16-bit seqnum).

**Cause.** `int16(a - b) < 0` mis-classifies the boundary value as
"before". The correct form is unsigned distance:
`uint16(b - a) <= 0x7FFF`.

**Evidence.** 595 (L2TP reliable `seqBefore` bug).

**Avoid it by.** Sequence-number ordering in a modular space of
bit-width N (16 for L2TP Ns/Nr, 32 for BGP Update-id): use
`uint_N(b - a) <= max_N / 2`, not `int_N(a - b) < 0`. Write a TDD
test case that exercises `diff = max/2` to force the correct form.

**Recover if you hit it.** Replace the signed comparison with the
unsigned form; add the boundary test case.

---

## Testing traps

### Test passes against broken production path

**Symptom.** A test is green. The production path it claims to cover
is silently wrong. The green test provides false confidence.

**Cause.** The test's fixture or stub diverges from what production
actually produces. The test self-validates against its own setup.

**Evidence.**
- 030 (old-vs-new comparison test where both sides were broken for
  reflector attrs).
- 125 (ExaBGP migration tests used Ze syntax as input — no ExaBGP-
  migration code was exercised).
- 340 (count-only map assertion passed by coincidence when wrong
  parsing produced colliding zero-prefix keys).
- 396 (handler unit tests used a flat JSON shape that production
  never produced).
- 483 (`.ci` test used a `cmd=api` syntax the real parser did not
  accept; route came through a different code path).
- 362 (watchdog `.ci` flakiness was masked because the checker
  framework's `(conn, seq)` grouping hid ordering violations).

**Avoid it by.** Before citing a test as evidence that feature F
works, name the single file and line in production code whose removal
would make the test fail. If you cannot name a specific `file.go:line`,
the fixture is wrong — the test proves only that its own setup is
self-consistent.

**Recover if you hit it.** Rebuild the fixture from real production
output. For `.ci` tests, capture the fixture from a live run, not from
the test's own expectation.

---

### `net.Pipe()` deadlocks on sequential write-then-read

**Symptom.** A test that uses `net.Pipe()` hangs indefinitely on the
first `Write`.

**Cause.** `net.Pipe()` is zero-buffer. `conn.Write(x)` blocks until
some goroutine calls `conn.Read(y)` on the paired endpoint. Sequential
`Write(x); Read(y)` deadlocks even when both endpoints share a
goroutine.

**Evidence.** 210 (yang-ipc-plugin), 264 (bgp-chaos-inprocess),
459 (plugin-tcp-transport), 609 (l2tp-6b-auth).

**Avoid it by.** One of:
1. Start the reader goroutine before any `Write` call.
2. Wrap every `Write` in its own goroutine.
3. Use a buffered substitute (a pair of `net.TCPConn` from
   `net.Listen("tcp", "127.0.0.1:0")` — chaos in-process uses this
   pattern, see 264).

**Recover if you hit it.** Refactor the test to follow pattern (1), (2),
or (3).

---

### Typed nil is non-nil when assigned to an interface

**Symptom.** A function that takes an interface parameter checks
`if iface == nil` and the check returns `false` even when the caller
passed a nil concrete pointer.

**Cause.** `var p *Concrete = nil; fn(p)` passes an interface value
whose type descriptor is non-nil; the interface is not nil.

**Evidence.** 244 (typed `*mockReactor` nil passed into interface
parameter of test helper).

**Avoid it by.** In test helpers that are supposed to pass nil into
production code, declare the parameter with the interface type and
pass a typed-nil interface:

```go
var r plugin.ReactorLifecycle // not *mockReactor
fn(r) // r is genuinely nil
```

**Recover if you hit it.** Change the parameter type to the
interface, not the concrete pointer.

---

### Package-level registry contamination across tests

**Symptom.** Test A passes in isolation. Test B passes in isolation.
Running A then B in the same `go test` binary, B fails with "decoder
not registered" or "unknown capability".

**Cause.** Test A (or its cleanup) called `Reset()` on a package-level
registry, leaving the registry empty for Test B. The registered
decoders lived as package-init side effects; they do not re-register
between tests.

**Evidence.** 240 (plugin-engine-decode), 533 (bgp-boundary-cleanup:
`Snapshot`/`Restore`/`Reset` in registry must include every new
global).

**Avoid it by.** Any test that mutates a package-level registry MUST
capture the state via `Snapshot()` before its first mutation and
restore via `t.Cleanup(func() { registry.Restore(snap) })` registered
before the first mutation.

**Recover if you hit it.** Add the Snapshot/Restore pair; do not
rely on test isolation.

---

### `go test` cache hides compile breaks in dependent packages

**Symptom.** `make ze-verify-fast` is green. A package that imports
your modified file fails to compile at the next build.

**Cause.** `go test` caches the compile result per package. Modifying
file X invalidates the cache for X's package, but not for packages
that transitively import X. If X's change broke a consumer's
type signature, the broken consumer stays cached and the test result
is stale.

**Evidence.** 394 (phase 3 forward-congestion), 457 (phase 2), 613
(vpp-2-fib).

**Avoid it by.** After modifying any exported identifier (type,
function signature, constant, interface method), run
`go clean -testcache` before `make ze-verify-fast`, OR touch one file
in every importing package to force recompile.

**Recover if you hit it.** Clean the test cache and re-run.

---

### `time.Now()` bypasses injected clocks

**Symptom.** A chaos or virtual-time test hangs on a timer that
should have already fired. Running the same path in a unit test
with real time works.

**Cause.** Code inside a package that accepts `clock.Clock` called
`time.Now()`, `time.Since(...)`, `time.NewTimer(...)`, or
`time.AfterFunc(...)` directly. The direct call bypasses the injected
clock; virtual time does not advance through it.

**Evidence.** 275 (spec-forward-pool: `time.Since(estAt)` bypassed
simulated clock; fixed by using `clock.Now().Sub(estAt)`),
341 (operational-commands: same trap in many handlers),
457 (forward-congestion phase 2).

**Avoid it by.** In any package that accepts a `clock.Clock`
parameter or constructs a `clock.Clock` field, every call that returns
a monotonic or wall-clock time MUST go through the clock instance.
Grep-audit test in `internal/sim/` enforces this for reactor/FSM code.

**Recover if you hit it.** Replace `time.Now()` with `c.Now()` (where
`c` is the injected clock); replace `time.NewTimer(d)` with
`c.NewTimer(d)`. Extend the grep-audit test if the package is not
currently covered.

---

## Multi-source-of-truth traps

### YANG module registered but not in `yang_schema.go`

**Symptom.** A new plugin's top-level config block is rejected at
parse time as "unknown top-level keyword", even though the plugin
calls `yang.RegisterModule()` in its `init()`.

**Cause.** Two registrations are required:
1. `yang.RegisterModule(...)` in an `init()` inside the module's
   `schema/register.go` — makes the module available to the loader.
2. An explicit module-name entry inside `YANGSchemaWithPlugins()` in
   `internal/component/config/yang_schema.go` — builds the schema.

The parser does not discover modules from (1) alone.

**Evidence.** 488 (looking-glass), 556 (bfd-1-wiring), 577
(gokrazy-2-ntp).

**Avoid it by.** Every new top-level config block touches both
`register.go` (registers `init()`) AND `yang_schema.go` (adds to the
module list). Treat them as one atomic change.

**Recover if you hit it.** Add the module to `YANGSchemaWithPlugins()`.

---

### Env var registered in two places drifts

**Symptom.** Changing an env var default in one file has no effect at
runtime; another file registered the same key and wins.

**Cause.** `env.MustRegister` silently overwrites duplicate keys. The
winner is the last `init()` to run. Different binaries (daemon, editor,
test helpers) import different packages, so the winner differs per
binary.

**Evidence.** 476 (env-registry-consistency), 506
(listener-6-compound-env), 628 (env-cleanup: duplicate `ze.config.dir`
in `main.go` and `ssh/client.go` kept intentionally, with comment).

**Avoid it by.** Every env var should have exactly one `init()` that
registers it. If a second package needs the key and cannot import the
first (test binary, circular dep), duplicate it with a comment
pointing at the canonical registration and this entry.

**Recover if you hit it.** Grep for `MustRegister(<key>` to find every
registration site; reconcile to one or document the duplication.

---

### Plugin list hardcoded in two test files

**Symptom.** Adding a new plugin breaks `TestAllPluginsRegistered` or
`TestAvailablePlugins`. Fixing one fails the other.

**Cause.** Two independent test files list the expected plugins:
- `internal/component/plugin/all/all_test.go` (`TestAllPluginsRegistered`)
- `cmd/ze/main_test.go` (`TestAvailablePlugins`)

**Evidence.** 513 (healthcheck), 556 (bfd-1-wiring), 579
(gokrazy-4-resilience), 580 (gokrazy-0-umbrella).

**Avoid it by.** Adding a new plugin requires updating both files in
the same commit. Platform-specific plugins (`iface-dhcp` is Linux-only)
require bidirectional platform-aware checks.

**Recover if you hit it.** Fix both files. Consider a future refactor
to read the list from the registry, but no session has done this yet.

---

### "Future X" in a learned summary proves the spec is NOT done

**Symptom.** A learned summary for spec-N says "future work: wire
X" or "decorator wiring requires populating Y". Session N+1 finds
that the feature is not actually end-to-end functional.

**Cause.** The spec claimed completion without every AC being wired
through production code. The summary faithfully records the gap —
which means the spec was closed prematurely.

**Evidence.** 488 → 498 (looking-glass: summary 488 said "future
decorator wiring requires populating GraphNode.Name"; 498 is the
overhaul that fixed it — code existed, was not wired).

**Avoid it by.** If you are about to write a learned summary that
contains the phrase "future X", "requires Y in a follow-up", or
"deferred to N": the spec is not done. Do not close it. Either wire
it, or explicitly record the deferral in `plan/deferrals.md` with a
named destination spec.

**Recover if you hit it.** Read the entire summary for "future",
"deferred", "not yet wired"; pick up the work.

---

## Workflow traps

### Claiming completion while stale specs persist

**Symptom.** A spec says "What Remains: Phase N (YANG only)". Grep
shows Phase N fully implemented with unit tests and pipeline
integration.

**Cause.** The spec was not updated as the feature landed. Multiple
sessions edited the code; no session updated the spec. The spec's
"What Remains" block is a historical artefact, not a status.

**Evidence.** 590 (cmd-1-rr-nexthop), 591 (cmd-3-multipath),
592 (cmd-9-ops), 593 (cmd-2-session-policy) — all four `cmd` series
specs audited on 2026-04-14 were found to have stale "What Remains"
sections.

**Avoid it by.** Never trust a spec's "What Remains" section without
grepping the codebase. See also `rules/memory.md`
`feedback_verify_specs_against_code` and the `rules/quality.md`
"Learned Summary Verification" section.

**Recover if you hit it.** Audit the spec against the code. Update or
close the spec.

---

### Concurrent session corrupts another session's WIP

**Symptom.** `make ze-verify-fast` fails with compile errors in a
file you did not touch. `git status` shows modifications you do not
recognise. Another session's commit picked up your uncommitted files.

**Cause.** Multiple Claude sessions share the repo working tree.
`git add` from any session stages files visible to every other
session's `git commit`. The first session to commit takes any
staged file, regardless of origin.

**Evidence.** 581 (sysctl-0-plugin: another session's commit
`fd5ebbb5` picked up our in-progress edits). 396 (bgp-monitor),
438 (event-stream), 444 (fleet-config), 477 (zefs-key-registry),
483 (exabgp-bridge-muxconn). 605, 627, 633 (concurrent `make ze-verify`
corrupted the shared log file).

**Avoid it by.**
1. `CLAUDE.md` already forbids `git add` / `git commit` from the Bash
   tool; commits only via a script the user runs.
2. Before invoking `make ze-verify-fast`, `git status` and confirm
   only expected files appear as modified.
3. Only one `make ze-verify*` may run at a time across the tree;
   `verify-lock.sh` enforces this via `flock`.

**Recover if you hit it.** `git stash` is forbidden (see
memory rule `feedback_parallel_sessions_no_stash`). Identify which
session owns each modification (by file topic) and coordinate
manually.

---

### Research subagents leaving `.go` files in `tmp/`

**Symptom.** `make ze-verify-fast` fails with compile errors in files
under `tmp/` that are unrelated to any active spec.

**Cause.** Research subagents fetched third-party source (e.g. vendor
tree samples) into `tmp/` and saved them as `.go`. `go test ./...`
walks the module root; `tmp/*.go` is compiled like any other package.

**Evidence.** 557 (iface-tunnel: `tmp/netlink-research`,
`tmp/vendor-pull`). 610 (vpp-7-test-harness: stray `.go` files).
619 (fmt-1-text-update: `tmp/my-vpp.go`, `tmp/my-config.go`).

**Avoid it by.** Research subagents MUST save fetched Go source as
`.txt` or inside a build-tagged directory (`//go:build ignore` at top
of file is not sufficient; the path must be excluded or the extension
must not be `.go`).

**Recover if you hit it.** Rename the offending files to `.txt`; the
Go toolchain ignores them.

---

## How to use this document

At session start, scan headings. At each commit, re-scan for the two
or three headings relevant to the change you made — most entries name
a specific check you can run in under a minute.

If you hit a symptom not listed here and it recurs (two or more
learned summaries), add an entry. The threshold for listing is not
"this happened once"; it is "this has happened more than once and
cost at least one session to diagnose."
