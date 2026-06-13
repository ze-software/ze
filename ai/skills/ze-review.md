# Review For Issues

Quick single-pass review (~2 min) for bugs, edge cases, security gaps, and missing tests in uncommitted changes.

This review answers: **"What can go wrong that nobody planned for?"**

See also: `/ze-review-deep` (exhaustive multi-agent review), `/ze-review-spec` (spec completeness check), `/ze-review-docs` (documentation accuracy)

## Steps

0. **Automated pre-check:** Run `make ze-validate`. If it reports findings, fix them before proceeding. This catches the mechanical subset of steps 1-3 (stale source anchors, line-number anchors, unwired exports, spec AC completeness, CLI handler coverage) without manual review.

1. **Wiring verification (FIRST — before any other analysis):** For every new function, type, handler, route, config option, CLI command, or plugin introduced in the diff, prove it is reachable from a user entry point. This is the FIRST step because it catches the project's most recurring defect class (see `plan/learned/RECURRING-PATTERNS.md`). If new code has no caller in production, nothing else in this review matters.

    For each new symbol, answer: **"What user action reaches this code?"** If you cannot name one, it is a BLOCKER.

    | New code type | Wiring check |
    |---------------|-------------|
    | Exported function/method | `grep` or LSP `findReferences` for at least one caller outside its own file and test files |
    | Struct / type | Same: at least one non-test consumer |
    | HTTP handler / web route | Registered on a mux (`srv.Handle`, `mux.HandleFunc`, etc.) and reachable from `hub/main.go` or `web/server.go` |
    | CLI command | Registered via `registry.MustRegisterLocal` or `registry.RegisterRoot` in a `register.go` with a blank import chain to `main.go` |
    | Plugin | Has `register.go` with `registry.Register()`, appears in generated `all.go` (or will after `make generate`) |
    | Config option / YANG leaf | YANG module registered, leaf read by runtime code (not just parsed) |
    | Env var | `env.MustRegister()` call exists, `env.Get*()` call exists |
    | Metrics | Metric created AND updated somewhere reachable |
    | Event / send type | Listed in plugin `Registration.EventTypes`/`SendTypes`, at least one subscriber/caller |

    **Do not skip this step.** "The code compiles" and "tests pass" do not prove wiring. A function with zero callers outside tests is dead code in production. Report every unwired symbol as a BLOCKER finding.

    **If any wiring BLOCKER is found:** report it immediately. Do not proceed to the remaining review steps until the user acknowledges. Unwired code means the feature does not exist from the user's perspective, so reviewing its correctness, security, or edge cases is premature.

2. **Functional test coverage (BLOCKING — immediately after wiring):** For every new or changed user-facing behavior in the diff, verify a functional test (`.ci` or `.et`) exists that exercises the full path. Apply the mapping from `ai/rules/functional-test-gate.md`: match the change type to the required test directory and check for a test covering the behavior.

    | Change type | Required test |
    |-------------|--------------|
    | BGP wire behavior | `.ci` in `test/encode/` or `test/decode/` with hex match |
    | Plugin behavior | `.ci` in `test/plugin/` with API commands |
    | Config option | `.ci` in `test/parse/` |
    | CLI subcommand | `.ci` in `test/ui/` |
    | Web endpoint | `.ci` in `test/web/` |
    | Editor behavior | `.et` in `test/editor/` |
    | Config reload | `.ci` in `test/reload/` |

    For each user-facing behavior: **"Which functional test proves this works through the daemon?"** If none exists, report it as a BLOCKER. Unit tests alone do not satisfy this check.

    **Exception:** pure internal refactors with no user-visible effect, or changes where an existing functional test already covers the path.

3. **Documentation drift check (BLOCKING for user-visible or architecture changes):** For every changed source file and every changed behavior, verify documentation stayed current.

    | Change type | Required doc check |
    |-------------|--------------------|
    | Config/YANG/parser | `docs/guide/configuration.md`, `docs/architecture/config/syntax.md`, and any guide examples use accepted syntax |
    | CLI command/output | `docs/guide/command-reference.md` and command docs match handler grammar/output |
    | API/RPC/event/send type | `docs/architecture/api/commands.md`, `docs/architecture/api/process-protocol.md`, and plugin docs match types/handlers |
    | Plugin registration/inventory | Runtime inventory docs match registry or `bin/ze --plugins` output |
    | Architecture/data flow | Relevant `docs/architecture/*` claims match current source and have source anchors |
    | Metrics | Telemetry docs list metric names and labels |

    Also grep `docs/` for `source: <changed-file>` for every changed source file. If any anchored claim is stale or missing after the code change, report an ISSUE. If a user-visible behavior changed and no documentation was updated or explicitly proven unnecessary, report an ISSUE.

4. **Identify changed files:** Run `git diff --name-only HEAD` to find all modified files.
5. **Read the actual code:** For every changed file, read the diff. Understand what changed.
6. **Understand intent via history:** For each changed region, run `git log --oneline -5` and `git blame` on the modified lines. Understand WHY the old code existed. Flag if the change removes a guard, workaround, or constraint that was added deliberately.
7. **Removed-behavior audit:** For every line the diff DELETES or replaces, name the invariant or behavior it enforced. Then search the new code for where that invariant is re-established. If you cannot find it, that is a finding: a removed guard, a dropped error path, a narrowed validation, a deleted test that covered a real case. This step is distinct from step 6: step 6 asks "why did the old code exist?" This step asks "is the protection still there?"
8. **Check code comments:** Read WARNING, INVARIANT, NOTE, and TODO comments in modified files. Verify the changes do not violate stated invariants or ignore documented constraints.
9. **Trace data flow:** For each changed component, trace data from entry through transformations to exit. Verify boundaries are respected.
10. **Apply edge case techniques:** Apply EVERY technique in the table below to every changed component.
11. **Security review:** Apply the security checklist to every user-controlled input.
12. **Allocation review:** Check every `make()` in changed code for unbounded sizes.
13. **Logic correctness review:** Read every changed function and check for:

    | Check | What to look for |
    |-------|-----------------|
    | Inverted condition | `if err != nil` guarding the success path, `&&` vs `\|\|` swapped |
    | Wrong variable | Copy-paste where the second use still references the first variable |
    | Off-by-one | `<` vs `<=`, `len-1` vs `len`, loop starts at 1 when it should start at 0 |
    | Unreachable branch | `switch` case that can never match, `if` after an unconditional return |
    | Missing return/break | Fall-through in switch, early-return path that forgets cleanup |
    | Shadowed variable | `:=` in inner scope hiding an outer variable the function relies on |
    | Integer truncation | `uint16(bigValue)` silently wrapping, `int(uint32Val)` on 32-bit |
    | Nil dereference path | Method call on a receiver that could be nil (check callers) |

    For each function: does the code do what the function name says?

14. **Performance review:** Check changed code for unnecessary allocations and algorithmic issues, especially on hot paths (see `no-sprintf-alloc.md` "Hot Path Rule" for the list).

    | Check | What to look for |
    |-------|-----------------|
    | `fmt.Sprintf` / `fmt.Errorf` on hot path | Use `textbuf.Buffer`, `errors.New`, or append-based alternatives (see `no-sprintf-alloc.md`) |
    | `.String()` concatenation on hot path | Use `AppendTo` or `textbuf.Buffer` chain |
    | Allocation inside a loop | `make()`, `append()`, or string building per iteration when a single buffer outside the loop suffices |
    | Heap escape via interface boxing | Passing a concrete value through `any` or `interface{}` on a hot path |
    | Heap escape via closure capture | Closure capturing a local variable forces it to heap |
    | Redundant computation | Same derivation computed multiple times when it could be computed once and reused |
    | Missing precomputation | Value derived from configuration or negotiated state that does not change per-request but is recomputed on every call. Precompute at setup time, store on the struct, reuse on the hot path |
    | O(n^2) or worse | Nested loops over the same collection, linear scan inside a loop when a map lookup suffices |
    | Map with string key from known set | `map[string]V` where `map[uint16]V` or typed enum key would avoid hashing overhead |
    | `string([]byte)` for comparison | Compare bytes directly instead of converting to string |
    | Callee allocates what caller could provide | Function does `make([]byte, n)` when caller has a buffer in scope (see `memory-architecture.md`) |

    Cold paths (startup, config load, CLI one-shot) are exempt. Focus on hot paths as defined in `no-sprintf-alloc.md`.

15. **Plugin traversal check:** If config structure changed, grep for all code reading the old structure.
16. **Altitude check:** For each change, ask: is this fix at the right depth? A special case layered on shared infrastructure is a sign the underlying mechanism should be generalized instead. Prefer deepening the shared abstraction over adding per-caller workarounds. Report bandaid fixes as ISSUE with the deeper alternative named.
17. **Project rules cross-check:** For each changed file, verify compliance with applicable rules (steps 13-14 above cover logic and performance specifically; this step covers structural and convention rules):

| Changed code touches | Check against |
|---------------------|---------------|
| Wire encoding/decoding | `buffer-first.md` -- WriteTo(buf, off), no append/make in encoding |
| New goroutine | `goroutine-lifecycle.md` -- long-lived worker, not per-event |
| Naming (types, JSON keys, YANG) | `naming.md`, `json-format.md` -- kebab-case JSON, ze- prefix |
| Plugin code | `plugin-design.md` -- proximity, YANG required, import rules |
| CLI handler | `cli-patterns.md` -- flag.NewFlagSet, exit codes, stderr for errors |
| Config parsing | `config-design.md` -- fail on unknown keys, no version numbers |
| New data wrapper/struct | `design-principles.md` -- lazy over eager, no identity wrappers |

18. **Filter false positives:** Before reporting, discard findings that match any of these:

| False positive | Why discard |
|----------------|-------------|
| Pre-existing issue (present before this diff) | Not introduced by these changes |
| Linter/compiler-catchable (imports, types, formatting) | `make ze-lint` catches these separately |
| Issue on unmodified lines | Out of scope for this review |
| Intentional behavioral change clearly related to the broader diff | Not a bug, it is the point |
| General quality concern not tied to a specific bug | Too vague to act on |
| Contradicts a project rule but has an explicit override comment in code | Intentional exception |

    **Never discard wiring, functional-test, removed-behavior, logic, altitude, or hot-path performance findings.** An unwired symbol is dead code in production. A missing functional test is a coverage gap. A lost invariant from deleted code is a correctness regression. A logic bug (wrong condition, wrong variable, off-by-one) is a correctness defect. A bandaid fix at the wrong depth compounds maintenance cost. A hot-path allocation is measurable overhead. These always survive this filter.

    **PLAUSIBLE by default.** Do not discard a finding for being "speculative" or "depends on runtime state" when the state is realistic: concurrency races, nil on a rare-but-reachable path (error handler, cold cache, missing optional field), falsy-zero treated as missing, off-by-one on a boundary the code does not exclude, retry storms, regex that lost an anchor. These are real findings. Only discard when you can prove the scenario is impossible from the code (quote the guard, cite the type constraint, show the invariant).

19. **Interop and goal validation check:** If the diff implements or modifies protocol behavior (BGP capability, NLRI family, session behavior, wire format, authentication), verify per `ai/rules/interop-and-goal-validation.md`:
    - Does an interop test scenario exist that proves this works with another daemon?
    - If the spec has a Goal Validation table, is every goal backed by concrete evidence?
    Missing interop test for protocol work is a BLOCKER. Empty goal validation for a completed feature is an ISSUE.

20. **RFC compliance check:** If the diff implements or modifies protocol behavior covered by an RFC, verify the code against the RFC summaries in `rfc/short/`.

    **When to run:** The diff touches wire encoding/decoding, message handling, capability negotiation, state machine transitions, timer behavior, NLRI parsing, attribute handling, or any code with existing `// RFC NNNN` comments.

    **How to check:**
    1. Identify relevant RFCs: check the spec's Required Reading section, scan the diff for `// RFC` comments, and consult the Common RFCs table in `ai/rules/rfc-compliance.md`.
    2. Read every matching `rfc/short/rfcNNNN.md` summary.
    3. For each MUST/MUST NOT in the summary that applies to the changed code, verify the implementation enforces it. Check: is there a code path that violates the requirement?
    4. For each SHOULD in the summary, verify the implementation follows it or has an explicit reason not to.
    5. Verify that every MUST enforced in the changed code has a `// RFC NNNN Section X.Y: "quoted requirement"` comment directly above the enforcing code (per `ai/rules/rfc-compliance.md`).

    | Finding | Severity |
    |---------|----------|
    | Code violates a MUST/MUST NOT | BLOCKER |
    | Missing `// RFC` comment on a MUST enforcement | ISSUE |
    | Code ignores a SHOULD without justification | ISSUE |
    | MAY clause implemented without user decision | NOTE |

    **Skip this step** if the diff has no protocol code (pure config, CLI, web, docs).

21. **Report findings** as a numbered list with severity:
    - **BLOCKER:** Bug that will cause incorrect behavior, crash, or security vulnerability
    - **ISSUE:** Logic error, performance problem on hot path, missing test, edge case not handled
    - **NOTE:** Suggestion or minor observation

## Edge Case Techniques (MANDATORY)

Apply each technique to every changed component. These find bugs that happy-path review misses.

| Technique | What to do | Example |
|-----------|-----------|---------|
| **Read actual validation** | For every input validation, read what the function ACTUALLY accepts, not what the spec says it should. Does the code match the stated intent? | `unicode.IsLetter()` accepts CJK, but spec says "alphanumeric" |
| **"What if 1MB?"** | For every user-controlled string, ask: what happens if it is 1MB? Check for length bounds at the validation point. | Peer name with no length limit flows into every JSON response |
| **Degenerate valid input** | After verifying individual character checks pass, ask: what input passes ALL checks but is still wrong? | `"---"` passes `[a-zA-Z0-9_-]` char check but is a useless selector |
| **Symmetry check** | When validation exists for X, grep for all parallel paths that accept the same kind of input. Are they all validated? | Peer names validated but group names not |
| **Grep the old pattern** | When a structural change adds a new path, grep for ALL code that reads the old structure. Every hit is a potential miss. | `grep '["peer"]' plugins/` finds code that does not handle groups |
| **Both-levels-set** | For any inheritance/override mechanism, test with config at BOTH levels simultaneously. Verify override order is correct. | Group sets `role customer`, peer sets `role provider` -- which wins? |
| **Boundary enumeration** | For each validation rule: test last valid, first invalid, empty, max-length, and "looks valid but semantically wrong." | Name at exactly 255 chars (valid) vs 256 (invalid) |
| **Nil/missing path** | For every optional field or nullable reference, trace what happens when it is absent. Does the code handle nil gracefully? | `ctx.Reactor()` returns nil before daemon starts -- does `isKnownPeerName` crash? |

## Security Review (MANDATORY)

For every user-controlled input that enters the system:

| Check | Question |
|-------|----------|
| Character set | What characters does the validation ACTUALLY accept? (Read the code, not the comment) |
| Length | Is there a maximum length? What happens at the boundary? |
| Injection | Does this string flow into shell commands, SQL, JSON, or log formatting? |
| Resource | Can a malicious input cause unbounded allocation, CPU, or output size? |
| Confusion | Can two different inputs produce the same internal representation? (homoglyph, case folding) |
| Allocation | Does any `make(map, N)` or `make([]T, N)` use a user-controlled size? Can N be large enough to OOM? |

## Allocation Safety (MANDATORY)

For every `make()`, `append()`, or buffer allocation in changed code:

| Pattern | Risk | Mitigation |
|---------|------|------------|
| `make(map[K]V, len(userInput))` | OOM if input is huge | Cap size or validate input count before allocating |
| `make([]T, 0, userCount)` | OOM if capacity is attacker-controlled | Use a bounded initial capacity, grow via append |
| `append()` in unbounded loop over external data | Memory grows without limit | Check loop iteration count against a maximum |
| `json.Unmarshal` into `map[string]any` | Arbitrary nesting depth, arbitrary key count | Accept the risk (Go stdlib handles it) or limit input size |
| Slice/map built from config then held forever | Permanent memory if config is reloaded | Ensure old allocations are released on config reload |

**What to check:** trace every `make()` call in changed code. Is the size argument derived from trusted data (constants, YANG schema limits) or untrusted data (config file, JSON input, network)? If untrusted, is it bounded?

## Plugin Traversal Check

When the config structure changes (new container, new nesting level):

1. `grep -rn '"peer"]' internal/component/bgp/plugins/` -- find all peer config traversal
2. For each hit: does it also handle the new path?
3. For each plugin with multi-level handling: does per-item config correctly override parent defaults?
4. Check for the "both-set" test for each plugin

## Review Integrity

**Verify claims against source code, not docs.** Before stating what the code
does or comparing systems, read the implementation. Treat any existing
documentation as potentially stale. When multiple subsystems need verification,
spawn parallel agents to read each one.

**Fresh eyes on every pass.** Each review must examine the full diff, not just
the delta since the last review. Stacking reviews that only look at "what
changed since last review" produces diminishing returns. The best pattern is
alternating: implement, then full review with a different critical lens
(logic and security on one pass, performance and completeness on the next).

## Rules

- Do NOT fix anything. Report findings only.
- Do NOT check spec completeness -- that is `/ze-review-spec`.
- After the user reviews your list, they will tell you which to fix.
- **Regression test required per fix:** When fixing an issue found by this review, add a test that would have caught the problem during development. The issue exists because a test was missing; the fix is incomplete without one. If a regression test is genuinely impossible (e.g., the finding is a naming convention violation), note why in the fix. Otherwise, no test = not fixed.
- No cap on review passes. Run a fresh pass whenever the code has changed since the last one, and keep running passes until a pass finds nothing. "I already reviewed this" is not a reason to stop -- fixes introduce new code, new code needs review.
