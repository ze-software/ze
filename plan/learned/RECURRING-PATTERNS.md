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

### `c_silent_ignore` rejects bare `default:`

**Symptom.** A `Write` or `Edit` is rejected by `c_silent_ignore`
(`.claude/hooks/pretool-writeedit.py`), even though the `default:` branch body
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

### `c_check_existing_patterns` blocks duplicate type or function names

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

### Feature implemented but not wired to any user entry point

**Symptom.** Feature code exists, unit tests pass, but no user action
(CLI command, web page, config option, API call) reaches the code.
The feature is invisible in production.

**Cause.** Implementation phases are ordered feature-first: storage,
parsing, logic, then "wire it up" as the last phase. By that point
the session is context-starved and rationalizes that wiring is
someone else's job. The wiring check fires only at review/completion
time, when the architecture may not easily accommodate it.

**Evidence.** This is the project's most recurring defect class.
488 -> 498 (looking-glass decorator wiring), plus numerous instances
flagged by the user across sessions. Occurrence count lives in
`plan/journal/unwired-feature.md`.

**The `pkg/` blind spot, and how to count a family (2026-08-22,
spec-record-answers-1-sdk-path).** A move from `internal/` to `pkg/` is a one-way
door. It also moves the symbol OUT of the gate below.
`check_cross_package_wiring` in `scripts/dev/validate.py` collects symbols only
from changed files under `internal/` and `cmd/`. So a `pkg/` export is unwired
where it is most expensive and least checked. One such move made three symbols
public and two had no caller: `CheckRowArity`, which only its own package called,
and `DirectBridge.HasDispatchCommandAnswer`, whose one caller was a test.

The accessor is the instructive half, because it survived two reviews. Two things
protected it. It LOOKED symmetric: eight sibling `Has*` methods sat beside it, so
the set read as a family that works this way. And its own doc comment argued that
deleting it would leave the registry drift guard blind to the answer slot.

Neither is evidence. **Count the family instead of arguing about it.** Eight of
the nine accessors guarded exactly one product call each, and this one guarded
none. That makes it the member with no caller, not a set with a convention.
**A doc comment defending a symbol is its author's belief, not a reason the
symbol is needed** (`ai/rules/evidence.md`). This comment stated a premise you
can make false: give the drift guard a real dispatcher, and it exercises the slot
instead of asking about it.

A review's proposed repair can be wrong. This one was. Calling
`DispatchCommandAnswer` inside the guard segfaulted, because that test built a
bare `&Server{}` and the slot's handler is bound to the server it was wired from.
The working fix reused a real-dispatcher setup already in the same file.

**Extra check for a `pkg/` change.** When you move or add anything under `pkg/`,
grep each new exported symbol yourself. The wiring gate will not:
`grep -rn 'Symbol' --include='*.go' . | grep -v _test.go`. When the symbol is one
of a family, count the family's callers before you accept that it belongs there.

**Avoid it by.**
1. Spec design: fill the Wiring Test table with concrete entry points
   before implementation starts.
2. `/ze-implement` step 4: create entry point skeleton + failing
   wiring test BEFORE any feature code. Phase 1 in the spec template
   is always wiring.
3. `/ze-review` step 1: wiring check runs first, blocks the rest of
   the review if any symbol is unreachable.
4. `architecture.md` item 5: name the entry point and file:line
   before writing any feature code.

**Gated by.** `make ze-doc-wiring-check`, which runs `check_wiring` in
`scripts/dev/verify_wiring_docs.py`. It fails when a new exported symbol under
`internal/` or `cmd/` has no non-test caller. Read the scope literally: the gate
covers `internal/` and `cmd/` and nothing else, so `pkg/` is caught at review
time or not at all.

**Recover if you hit it.** Identify every unwired symbol via
`grep -rn 'Symbol' internal/ cmd/ --include="*.go" | grep -v _test.go`.
Wire each one to its entry point before claiming done.

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
`ai/rules/protocol.md`.

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

### A bound that adds before it compares is defeated by the value it bounds

**Symptom.** A guard reads a length or a count out of untrusted input, adds a
header or an offset to it, and compares the sum against a maximum. Every ordinary
input is refused correctly. One input near the top of the integer's range wraps
the sum to a small number, passes the comparison, and the guard is inoperative
for exactly the value it exists to refuse.

**Cause.** The arithmetic runs before the bound does. `answerFieldWidth`
(`pkg/plugin/rpc/message.go`) returned `uint64(header) + size` for a counted text
whose `size` came off the wire, so `answerLineWidth` reported a small width for a
huge stated count and `scanStatedLine` never reached its refusal. Measured:
`#7 row 18446744073709551595:x` stated a width of 7. The sibling of the
signed-subtraction entry above, and the same root: a value that wraps is judged
after the wrap, not before it.

**Evidence.** spec-record-answers-2-only-encoding, 2026-08-22, Review Gate ISSUE
1. Reachable from any peer on the plugin mux connection and from the SSH exec
channel. It was found in the NEWEST layer of the change, by the independent
review, and not in the parts three phases had already reversed and re-read.

**Avoid it by.** Bounding the value the input STATES, before any arithmetic
touches it, and returning it unmodified once it fails. `answerFieldWidth` now
returns the stated count when that count alone passes `MaxMessageSize` and never
adds the header to it, and `answerLineWidth` returns that width rather than
`at + width`, which is the second place the same sum can wrap. Then check the
whole expression: one guarded addition proves nothing if a caller adds an offset
to the result. Write the test at the type's limit, not at a plausible large
number: the case that fails is `MaxUint64` minus a header, and nothing smaller
reproduces it.

**Recover if you hit it.** Grep the guard's own arithmetic for `+` on anything
the caller did not produce. Every sum over a wire-stated number is a candidate,
including the ones in the caller.

---

### The zero value as a valid-looking answer

**Symptom.** A guard passes, a loop exits, or a branch is skipped, and the
outcome looks like a decision rather than a miss. Nothing is logged. The failure
surfaces later as a lost route, an unauthenticated bind, or a peer refused for a
reason nobody can trace back.

**Cause.** An identity-bearing value whose zero is also a legitimate answer is
tested with `== 0`, `len(...) == 0`, or an untyped presence check, and the zero
selects the permissive branch. The variable carries two meanings -- "nothing was
asked for" and "the answer happens to be zero" -- and the code collapses them.

**Six instances found in one session (2026-07-27), all real defects, three of
them shipping:**

| Where | The zero | What it silently did |
|-------|----------|----------------------|
| `cmd/ze/hub/mgmt_guard.go` | empty `addrs` slice | Per-address loop ran zero times, refused nothing, and the builder then bound `0.0.0.0:3443` unauthenticated |
| `adj_rib_in` replay cut | `maxMsgID == 0` read as "no cut" | 0 is the ORDINARY value (measured 39/40 runs); replay ran unbounded and a peer received the same UPDATE twice |
| `rs/server_handlers.go` | `lastIndex == 0` read as "converged" | It means ZERO ROUTES REPLAYED; the delta loop broke on iteration zero and a prefix reached neither rail |
| `reactor/session_open_validation.go` | `settings.PeerAS == 0` | Dynamic peers have no configured AS at OPEN time, so `internal` was always false and RFC 6286 Section 2.2's self-identifier rejection never fired for them |
| `reactor/peer.go` `claimPeerAS` | `remote.ASN4 > 0` on a RECEIVED open | Never set by `UnpackOpen`; dead branch, so every 4-byte-AS peer claimed under AS_TRANS and legitimate peers in different ASes refused each other |
| `ike/engine/doctor.go` | `ParseIPsecConfig` error returning nil | `ze doctor` reported `"ready": true`, exit 0, on a config with an unparseable esp-group AND a missing interface |

**Fix.** Carry PRESENCE separately from VALUE. A pointer (`*uint64`), a
`(value, ok)` pair, or an explicit `tracked bool` beside the field. Where the
producer cannot answer at all, say so -- `ai/rules/evidence.md`: a
guard that neither denies nor speaks does not exist.

**Why it recurs.** Each instance looked locally reasonable, and several carried
a COMMENT asserting the safety property they did not provide ("the config
system's error to report", "RFC 6793 handling"). A comment is its author's
belief, not a decision record (`ai/rules/evidence.md`). Reading the
comment is not reading the producer.

**Detection gap, unfilled.** `evidence.md` names the pattern but
nothing greps for it. A mechanical check over `== 0` / `len(x) == 0` guards on
identity-bearing fields would have caught most of the six. That check does not
exist yet and is smaller than the defects it prevents.

**Corollary: independent review is what actually found these.** Four specs went
through review in that session and all four were blocked by it; three concealed
live defects behind green artifacts, passing tests and clean-looking audits. In
one case the author had written the comment explaining why a synchronous release
was necessary and still missed the two sibling call sites needing the same fix.
`ai/rules/planning.md`'s claim that self-review is not review is not
theoretical.

### An `enabled` gate on config extraction discards the service's security settings

**Symptom.** An operator writes `token` or `tls true` in a service block, starts
the service by env var or CLI flag rather than by config, and gets an
unauthenticated plaintext listener. Nothing is logged, and reading the config
back shows exactly what they wrote.

**Cause.** The extractor answers "does config start this service?" and "what are
this service's settings?" with one boolean. `enabled != true` returns early, so
the settings behind that gate are never parsed, and the caller runs the service
anyway on zero values. For every management surface in Ze, the zero value of the
auth settings is *no authentication* (see "The zero value as a valid-looking
answer", above -- this is the config-extraction form of it).

**Two instances, on two services:**

| Service | Found by | Fix |
|---------|----------|-----|
| `environment.mcp` (2026-07) | a strengthened `test/plugin/task-identity-scope.ci`; the old one asserted per-principal isolation while both principals were the same anonymous identity | `ExtractMCPSettings` / `ExtractMCPConfig` |
| `environment.looking-glass` (2026-08) | a security reviewer subagent tracing the TLS flag end to end | `ExtractLGSettings` / `ExtractLGConfig` |

**Fix.** One private `extractXBlock(tree) (Config, enabled, present bool)` that
parses everything and gates nothing, plus two exported callers that each answer
one question. Neither can inherit the other's meaning.

**Why it recurs.** The second instance was written by someone who had read the
first, recorded in the same spec file, one screen above the code being changed.
The gate reads correctly at the call site; the loss is invisible unless you ask
what a caller does with `ok=false`. The shape lives in
`internal/component/config/loader_extract.go`: `ExtractMCPSettings` and
`ExtractLGSettings` answer "what is configured", `ExtractMCPConfig` and
`ExtractLGConfig` answer "is it enabled".

**Detection.** The unit test asserts on a block that is deliberately NOT enabled
and checks the settings survived. A functional test that starts the service from
the config file cannot see the defect; it has to start the listener the way the
operator did.

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
- 1062 (three redistribute-late-join `.ci` passed with the late-join
  replay disabled — the fixture was fine, but the route reached the peer
  via an ALTERNATE production path, not the replay under test; caught
  only by disabling `handleReplayBatch` and seeing all three stay green.
  The reactor does not persist routes across reconnects itself
  (`internal/component/bgp/reactor/peer.go`), so a reconnect is not
  a clean genuinely-new-peer isolation).
- spec-record-answers-1-sdk-path, AC-7, 2026-08-22 (the acceptance
  criterion said the socket and the bridge produce one answer, and the
  test named for it drove a hand-written stub handler inside the
  TRANSPORT package. It proved the two transports carry one answer and
  never that the two producers build one, so `plugin.AnswerFor`, the
  whole in-process producer, reached the review gate with no test at
  all, and neither did its one caller. The criterion looked covered
  because a passing test named it. The variant to watch for: two
  implementations of one decision, where a test exercises the plumbing
  between them and nothing holds them to the same answer).

**Avoid it by.** Before citing a test as evidence that feature F
works, name the single file and line in production code whose removal
would make the test fail. If you cannot name a specific `file.go:line`,
the fixture is wrong — the test proves only that its own setup is
self-consistent. For a `.ci`/`.et` guarding specific behavior, do not
just NAME that line — DISABLE it (early `return` / no-op / `if true {
return }`) and confirm the test flips RED, then revert. A functional
test that stays green with the producing function disabled guards
nothing, even when its fixture is real. See
`ai/rules/testing.md` "Mutation-Verify the Test Actually Gates".

**Recover if you hit it.** Rebuild the fixture from real production
output. For `.ci` tests, capture the fixture from a live run, not from
the test's own expectation. If the behavior is not observable
end-to-end (a duplicate is suppressed, an alternate path delivers the
same effect), guard it with a UNIT test that inspects the producing
value directly and design the `.ci` to remove the alternate path
(inject with no peers, use a genuinely-new peer, not a reconnect).

---

### `net.Pipe()` deadlocks on sequential write-then-read

**Symptom.** A test that uses `net.Pipe()` hangs indefinitely on the
first `Write`.

**Cause.** `net.Pipe()` is zero-buffer. `conn.Write(x)` blocks until
some goroutine calls `conn.Read(y)` on the paired endpoint. Sequential
`Write(x); Read(y)` deadlocks even when both endpoints share a
goroutine.

**Evidence.** 210 (yang-ipc-plugin), 264 (bgp-chaos-inprocess),
459 (plugin-tcp-transport), 609 (l2tp-6b-auth), 647 (bmp-5-sender-compliance).

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

**Symptom.** `make ze-precommit-verify` is green. A package that imports
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
`go clean -testcache` before `make ze-precommit-verify`, OR touch one file
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
Grep-audit test in `internal/test/sim/` enforces this for reactor/FSM code.

**Recover if you hit it.** Replace `time.Now()` with `c.Now()` (where
`c` is the injected clock); replace `time.NewTimer(d)` with
`c.NewTimer(d)`. Extend the grep-audit test if the package is not
currently covered.

---

### New test type added but not back-filled to existing code

**Symptom.** A new test type, technique, or infrastructure is
introduced (fuzz target, property test, mutation gate, `-race` sweep,
clock-injection audit, `.et` editor test, QEMU integration harness),
and it is applied only to the code written alongside it. Equivalent
already-written code never receives the new test type, so a class of
existing code stays uncovered by a technique built specifically to
catch its failure mode.

**Cause.** The new test type ships with the feature that motivated it.
The session's scope is that feature, so "apply it everywhere it
applies" reads as out of scope. No step forces a backward sweep of
existing call sites, so coverage grows only forward from the
introduction date.

**Evidence.** User-reported 2026-06-14. Recurs because the
discovery-updates rule historically wired a new test type *forward*
(where to place new tests) but said nothing about *backward*
application to existing code. Related: the periodic-test-sweep
categories -- pure functions with only integration coverage, platform
code assumed untestable, missing test-infra support.

**Avoid it by.** When you add a new test type or technique, in the same
work: (1) name the set of existing code it applies to (package glob,
symbol kind, or call-site pattern); (2) either back-fill that set, or
record the uncovered remainder as an explicit, tracked backlog in the
spec or handoff -- never leave it implicit. A grep- or registry-driven
audit that enumerates every applicable site beats per-file judgement.
See `ai/rules/testing.md` "Back-Fill New Test Types".

**Gated by.** `make ze-test-sensitivity-check`, partly. Its tag-orphan ratchet
catches a `_test.go` whose build tag no `go test` supplies, so a new test type
that nothing runs fails. It cannot tell whether an applicable site was skipped,
so the back-fill sweep above is still yours to run.

**Recover if you hit it.** Run the new test type's selector across the
whole applicable set, triage the gaps, and file the remainder as
tracked work. `/ze-hunt` can enumerate applicable sites for
grep-detectable test types.

### An acceptance criterion written as an absence cannot be met

**Symptom.** A criterion reads "a grep for the retired name returns nothing".
The implementer runs it, gets a page of hits, and judges each one alone. The
criterion is never demonstrably met, and the review ends up writing the rule the
criterion should have carried.

**Cause.** A removal leaves survivors that are correct, and some of them MUST
spell the removed string. A fixture proving the name is gone has to name it. A
YANG revision log records what a past revision did. A release note exists for
the operator who types the retired spelling. Another daemon's command still
works in that daemon. A record of the change describes what happened. An absence
criterion outlaws all five, so no tree can satisfy it, and the hits that ARE
wrong hide among the ones that are right.

**Evidence.** spec-cli-show-bgp-is-the-command, AC-9, 2026-08-21. It asked for
nothing outside git history and the spec itself. The tree held five kinds of
correct survivor. Four of the five were declared after the fact and all four
held, but six OPEN specs that named the retired command as a LIVE command in
their own acceptance criteria sat in the same grep output and were noticed only
when the review looked. The same shape is recorded for TESTS in
`ai/rules/interop-and-goal-validation.md`: a test asserting an absence passes
when the mechanism is deleted.

**Avoid it by.** Writing the exception list INTO the criterion, at the moment
you write the criterion, never after the first grep. State it positively: name
the population that must move (the callers), then name the kinds that stay (an
absence assertion, a changelog entry, a release note, another product's
spelling, a record of the change). A criterion an implementer can run without
judgment is one an implementer can meet.

**Recover if you hit it.** Sort every survivor into a kind and write the kinds
into the criterion. Then read the survivors no kind covers. Those are the
misses, and they are what the absence phrasing was hiding.

---

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
it, or explicitly record the deferral in the source's `plan/deferrals/<source>.md`
shard with a named destination spec.

**Recover if you hit it.** Read the entire summary for "future",
"deferred", "not yet wired"; pick up the work.

### An inventory command is not the population it reports on

**Symptom.** You build the list of every command under a prefix from
`make ze-command-list`, register something against each entry, and ship. Live
paths the list never named keep the old behavior, and no gate goes red.

**Cause.** Ze resolves a typed command from THREE registries: the builtin RPCs,
the plugin names, and the local handlers `registry.MustRegisterLocal` owns
(`internal/component/bgp/cli/register.go`). `collect`
(`scripts/inventory/commands.go`) walks `AllBuiltinRPCs` and the streaming
prefixes, which is the first of the three. A verification tool blind to two
thirds of the population answers "complete" about the third it can see.

**Evidence.** spec-cli-show-bgp-is-the-command, 2026-08-21, Review Gate
BLOCKER-1. Ten live paths under `show bgp` (five rpki commands, two rs, two
adj-rib-in, healthcheck, plus decode and encode) kept inheriting the parent's
column orders and its summary and peers aliases, so the completer offered the
peers alias on ROA output. Measured with `AliasesForCommand` and
`ColumnsForCommand`, not argued. The class file
`plan/journal/gate-excludes-part-of-its-population.md` held 54 rows that day.

**A spec's own file list is an inventory, and it goes stale as the spec's phases
land (2026-08-22, spec-record-answers-2-only-encoding).** Files to Modify is
written before the work, from what the author saw, and it is then read for
eight phases as though it were the population. Every phase of that spec found a
file the list omitted, and two of them were load-bearing SPEAKERS of the wire
being rewritten: `test/scripts/ze_api.py`, the harness reader 156 fixtures reach,
and `internal/exabgp/bridge`. Neither appears anywhere in the list. The tell is
the same as above: the list is a record of a search, and the population is
whoever the runtime actually routes through. Derive it from the seam rather than
from the plan. For a wire, that is `grep -rn` for the writer and the reader
across EVERY language in the tree, the test harnesses included, not the Go files
the spec happened to open. A phase that finds a missing file should widen the
list in the same commit, which is what that spec did, rather than treating the
find as a one-off.

**Avoid it by.** Deriving the population from what the RUNTIME resolves, never
from an inventory. Then registering at the SHALLOWEST path of each branch rather
than at each leaf. A branch root prefixes every leaf under it, the leaves nobody
has written yet included, and the spellings that carry a selector in the middle:
`commandMatchesPrefix` (`internal/component/command/column_order.go`) resolves
the string the operator TYPED, so `show bgp peer detail` is not a prefix of
`show bgp peer 192.0.2.1 detail` and a per-leaf registration never applies to it.
Ten branch entries replaced fifteen leaf entries and covered every path the
fifteen missed.

**Recover if you hit it.** Drive the assertion from the operator's spelling, not
from the registration's. A unit test that spells the paths the way the
registration spells them cannot see the hole.
`TestChildCommandsDoNotInheritTheSummaryOrder` drives each branch, each command
beneath it, and each selector spelling, and dropping one branch from the list
turns seven subtests red.

---

### A renamed command leaves its parameter schema keyed to the old rpc name

**Symptom.** A command keeps answering after a rename and quietly loses its
schema. `ze yang doc <command>` prints no "Parameters (output):" block, and the
hub's per-command metadata carries no input parameters.

**Cause.** Two YANG modules spell the same rpc and only one of them holds the
`ze:command` binding. `AllRPCDocs`
(`internal/component/config/yang/cli/tree.go`) looks up
`paramIndex[doc.WireMethod]`, and `buildParamMeta`
(`cmd/ze/hub/command_meta.go`) compares `rpc.Name` against the rpc half of the
wire method. Both lookups MISS silently: the map returns the zero value and the
loop finds no match, so the command renders with no parameters instead of
failing. Renaming the command container and its wire method without renaming the
`rpc` in the matching `-api` module is what breaks the key.

**Evidence.** spec-cli-show-bgp-is-the-command, phase 3, 2026-08-21. The spec's
Files to Modify never listed `internal/component/bgp/yang/ze-bgp-api.yang`.
`rpc summary` there had to become `rpc overview` to follow the `ze-bgp:overview`
wire method, or `show bgp` would have answered correctly with no payload schema
behind it. `TestDocCommandWithOutputParams` pins it.

**Avoid it by.** Grepping the bare rpc NAME across every YANG module when you
rename a command, not only the wire-method string. The `-api` and `-cmd` modules
are separate files, and a rename touches both.

**Recover if you hit it.** Run `ze yang doc <command>` and check the parameter
block is still there. A command whose schema vanished still answers, so no
functional test sees it.

---

---

## Workflow traps

### A full disk reads as a code breakage

**Symptom.** `make ze-precommit-verify` reports `[build failed]` against dozens of
packages you never touched, and the functional suite fails a few unrelated
tests. It looks like a concurrent session broke the tree.

**Cause.** The host disk is full, and the real message sits above the FAIL
lines. It reads `no space left on device`, from `mkdir /tmp/go-build.../`
or from `link: mapping output file failed`. Two caches grow without bound
and nothing prunes them. They are the Go build cache (`go env GOCACHE`)
and the Docker build cache that the interop, perf, and appliance targets
fill. Measured 2026-07-30 on a 98G volume: 30G of Go cache and 15G of
Docker build cache, 99% full, 1.8G free.

**Evidence.** Observed 2026-07-30 during `spec-rfcgate-1b-rfc7296-pilot`.
Three functional tests failed with it, and each one passes with headroom.
`bfd-auth-meticulous-persist` reported "the sequence was never flushed",
which is a zefs write that had no room. The other two were
`test/web/interface-mac-override.wb` and `vpp-hugepages-qemu`. An
unprivileged `du` will NOT find the Docker half, because `/var/lib/docker`
is root-owned and is silently counted as zero.

**Avoid it by.** Reading the FIRST error in the stage log, not the FAIL
summary. When a build fails in packages your diff never reaches, run
`df -h /` first.

**Recover if you hit it.** `go clean -cache` and `docker builder prune -f`
reclaim the two caches with no loss, because both are caches. Check
`docker system df` first. Leave Docker IMAGES and VOLUMES alone. The
interop peer images are expensive to rebuild, and another session CAN be
mid-run.

---

### Another session's test edit reddens your RFC gate

**Symptom.** `make ze-rfc-check` fails with the violation
`rfc/requirements/<stem>.md is stale vs its sources`. You changed no tagged
test. `ze-doc-verify`, `ze-doc-wiring-check`, `ze-generated-files-check`
and `TestRFCLedgerFresh` all go red at the same time, because each one
reads the same freshness fact.

**Cause.** Each RFC's `file:line` records sit in one generated file under
`rfc/requirements/`. One session adds or removes a single line above a
tagged test. That shifts the line numbers and stales that RFC's file for
every other session. This repository runs concurrent sessions by design,
so the window is not rare.

The split bounds the damage to the RFCs whose tests moved. Before it, one
line shift staled the whole ledger.

**Evidence.** Twice on 2026-07-30 during `spec-rfcgate-1b-rfc7296-pilot`.
Both times the whole diff was line numbers in `internal/component/mcp/`,
which a different session was editing. `diff` of the regenerated ledger
named the owning package in one line each time.

**Avoid it by.** Running `make ze-rfc-index-update` immediately before you start
a verify, not earlier in the session. Diff the regenerated files to see
WHOSE tests moved: if every changed row names a package you never touched,
the staleness is not yours. To read one RFC's rows and leave the index shut,
run `python3 scripts/dev/rfc_requirements.py --show <stem>`.

**Recover if you hit it.** Regenerate, then verify. Do not hand-edit the
ledger, and do not reach for an override first: the gate is correct and
the file really is stale. If a concurrent session is actively editing
tagged tests, check the modification times of its files and start your
verify while it is quiet.

---

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
grepping the codebase. See also `.claude/rules/memory.md`
`feedback_verify_specs_against_code` and the `ai/rules/quality.md`
"Learned Summary Verification" section.

**Recover if you hit it.** Audit the spec against the code. Update or
close the spec.

---

### Concurrent session corrupts another session's WIP

**Symptom.** `make ze-precommit-verify` fails with compile errors in a
file you did not touch. `git status` shows modifications you do not
recognise. Another session's commit picked up your uncommitted files.

**Cause.** Multiple Claude sessions share the repo working tree.
`git add` from any session stages files visible to every other
session's `git commit`. The first session to commit takes any
staged file, regardless of origin.

**Evidence.** 581 (sysctl-0-plugin: another session's commit
`fd5ebbb5` picked up our in-progress edits). 396 (bgp-monitor),
438 (event-stream), 444 (fleet-config), 477 (zefs-key-registry),
483 (exabgp-bridge-muxconn). 605, 627, 633 (concurrent `make ze-precommit-verify`
corrupted the shared log file).

**Avoid it by.**
1. `CLAUDE.md` already forbids `git add` / `git commit` from the Bash
   tool; commits only via a script the user runs.
2. Before invoking `make ze-precommit-verify`, `git status` and confirm
   only expected files appear as modified.
3. Only one `make ze-precommit-verify*` may run at a time across the tree;
   `verify-lock.sh` enforces this via `flock`.

**Recover if you hit it.** `git stash` is forbidden (see
memory rule `feedback_parallel_sessions_no_stash`). Identify which
session owns each modification (by file topic) and coordinate
manually.

---

### Research subagents leaving `.go` files in `tmp/`

**Symptom.** `make ze-precommit-verify` fails with compile errors in files
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

### Refactoring removes a feature by over-generalizing its category

**Symptom.** A feature stops working after a structural refactor. The
commit message groups it with genuinely removed code under a shared
label ("config-mutation commands that bypass the editor"), but the
feature does not actually belong to that category.

**Cause.** During a batch cleanup, features are classified by surface
similarity rather than by what they do. `update bgp peer prefix` was a
data-refresh command that proposed config changes through the editor,
but it was grouped with `set bgp peer with/save` (which genuinely
bypassed the editor) and removed together. The shared label ("bypasses
the editor") was wrong for one of the three.

**Evidence.** 904 (`update bgp peer prefix` removed in `6c19edc32` as
part of the command-surface-ownership refactor; restored 12 days later
after the operator had no way to refresh max-prefix limits from
PeeringDB, leaving login warnings with no actionable fix).

**Avoid it by.** Before removing code in a batch, classify each item
independently: name what the code does (not what it looks like) and
what user operation it serves. If an item serves a distinct user need
from the others in the batch, it does not belong in the batch. Ask:
"if I remove this, what does the operator do instead?" If the answer
is "nothing" or "do it manually per peer," the removal needs its own
justification.

**Recover if you hit it.** Restore from git history. Adapt to
current APIs (the old code's dependencies may have changed). The
restoration is usually straightforward because the feature's runtime
dependencies survive the refactor -- only the entry point and glue
are deleted.

### A design premise nobody measured, built on for three phases

**Symptom.** A spec justifies a shape with a performance claim. Phases land on
top of it. Somebody finally measures, the claim is false, and the reversal has to
unpick every later phase that assumed the shape.

**Cause.** The claim reads like a fact because it is written in the same voice as
the constraints around it, and nothing in the workflow asks for its measurement.
A design rationale is evidence for the reader exactly as much as a doc comment
defending a symbol is (`ai/rules/evidence.md`): it records what its author
believed. The measurement was as available before phase 3 as after it. It took
two minutes.

**Evidence.** spec-record-answers-2-only-encoding, 2026-08-22. The spec specified
a length-prefixed answer id, `#<len>:<id>`, so a reader reaches the next field
by arithmetic rather than by scanning. Measured, it was SLOWER: 8.1 to 9.2 ns
against 3.2 to 3.5 ns for a fused digit loop over plain `#42 `, zero allocations
either way, and two bytes wider on every line of a million-row walk. The reader
still had to check the space that closes the field and still had to call
`ParseUint` on the slice it had just measured, which IS the cost of the simple
form. Phases 3, 4 and 5 had built on it; the owner measured it and reversed it in
phase 6 (`9313b7d5e`), and the same premise had to be unpicked again from every
counted field (`50468ee34`).

**Avoid it by.** Treating a performance rationale in a spec as an ASSUMPTION with
an A-N row, not as a constraint. Where the claim decides a shape that later phases
will build on, the benchmark is the FIRST phase, before the shape exists: write
both forms as two functions in a scratch `_test.go`, run `go test -bench`, and put
the numbers in the spec. The cost of measuring is minutes; the cost of not
measuring is every phase that inherits the shape. The general form: **the earlier
a claim sits in a dependency chain, the cheaper it is to check and the more
expensive it is to leave.**

**Recover if you hit it.** Reverse at the layer that states the shape, not at the
call sites. Then re-read every LATER phase for the same premise spreading under
another name: the base-36 outer length on every counted field was the id's
rationale applied to a second population, and it survived the first reversal by a
commit.

---

### A phase runs the suites its files sit in, not the suites its change reaches

**Symptom.** A phase is green and lands. A functional test in a suite nobody ran
has been red since it landed, and the phases after it do not see the red either,
because each one picks its suites the same way.

**Cause.** The suite is chosen from the directory the edit is in. A registration
change under `internal/component/bgp/plugins/cmd/peer/` reads as BGP work, so
the package unit tests and the plugin suite get run. What the change actually
alters is column rendering and pipe aliases, and that is what `test/ui/` covers.

**Evidence.** spec-cli-show-bgp-is-the-command, 2026-08-21, Review Gate
BLOCKER-2. `test/ui/show-column-order-absent-unchanged.ci` was RED on HEAD
across four landed phases. Its stated purpose is that the prefix lookup does not
leak a column order onto a command that declared none, which is exactly what
phase 1 broke. Phase 5 was verification only and ran no suite at all. The review
found it by running `make ze-functional-ui-test`.

**Avoid it by.** Choosing the suite from the SURFACE the change alters, not from
the package it edits. A registration that decides how output renders is a
`test/ui/` change wherever its Go file lives.

**Recover if you hit it.** Run the suite over HEAD before you assume your
working tree caused the red. A test that was already red names the phase that
broke it, and `git log` over the fixture says whether the fixture or the code
moved.

---

---

## How to use this document

At session start, scan headings. At each commit, re-scan for the two
or three headings relevant to the change you made — most entries name
a specific check you can run in under a minute.

If you hit a symptom not listed here and it recurs (two or more
learned summaries), add an entry. The threshold for listing is not
"this happened once"; it is "this has happened more than once and
cost at least one session to diagnose."
