---
name: ze-review
description: Review For Issues
---

# Review For Issues

Quick single-pass review (~2 min) for bugs, edge cases, security gaps, and missing tests in uncommitted changes.

This review answers: **"What can go wrong that nobody planned for?"**

See also: `/ze-review-deep` (exhaustive multi-agent review), `/ze-review-spec` (spec completeness check), `/ze-review-docs` (documentation accuracy)

## Delegation

`ai/rules/planning.md`: the main thread supervises, it does not run this
phase itself.

- **If you are the main thread:** spawn an agent to run this skill, hand it the
  spec path and the phase, then stop. Use `subagent_type: ze-read`, which costs
  about 6k fewer startup tokens per agent than the default
  (`ai/rules/context-economy.md`). Do not run the steps below inline. You do
  not need to ask permission first (`ai/INSTRUCTIONS.md`, STANDING REQUEST).
  Independent work goes out in ONE message with parallel `Agent` calls.
- **If you are that agent:** run the steps below. Resolve symbols with the LSP
  tool if your registry carries it and with `gopls` from Bash if it does not
  (`ai/rules/context-economy.md`). You cannot ask the user, so when you hit a
  STOP-and-ask condition, halt and put the question in your report for the main
  thread to carry.
- **Either way:** every claim in the report names the function that PRODUCES the
  behavior, as the file plus the symbol (`ai/rules/evidence.md`). The main
  thread verifies each one against source before acting; relaying a report
  unverified is fabrication with an extra hop. Report the conclusion and the
  evidence that would overturn it, never the search. Under 40 lines
  (`ai/rules/writing.md`).
- **Read the per-spec state file first.** When a spec is claimed,
  `tmp/session/session-state-<spec-stem>-<SID>.md` already carries each
  implementation phase's handoff: files changed with a digest, which AC-N are
  covered, what is green. `_find_latest_state_for_spec` in
  `.claude/hooks/lib/state-file.sh` resolves it. Read it before the diff. Do not
  re-read a source file only to learn what the digest already tells you.
- **Reading it NEVER reduces this review's independence or its lens count.** The
  digest says what was TOUCHED. It is not evidence. It never stands in for
  reading a file you must judge. Every finding still names the PRODUCING
  function, read from source (`ai/rules/evidence.md`). Every step and lens below
  runs in full.
- **Review is not a cost target.** It is the cheapest phase measured, and it
  prevents the most expensive one. Never skip a lens, a step, or a source read
  to make it shorter.

## Steps

0. **Pre-checks (complete all three, fix or report findings before proceeding):**
    - **Size the change.** Count the changed lines and files. Ask "is this change bigger than the problem it solves?" A diff bigger than its problem gets ONE finding: a BLOCKER naming the smaller change. The review stops there. Auditing the details of over-engineered code ratifies it, and every fix it drives earns another pass over more of it (`ai/rules/simplicity.md`, `ai/rules/context-economy.md`). Then match the process to the size: a few lines in one function earns one pass, never a loop.
    - `make ze-validate` — catches the mechanical subset of steps 1-3 (stale source anchors, line-number anchors, unwired exports, spec AC completeness, CLI handler coverage) without manual review.
    - `python3 scripts/dev/audit-test-relaxation.py` for uncommitted changes, or `python3 scripts/dev/audit-test-relaxation.py origin/main` to also cover work already committed but not yet pushed (this repo commits directly to main, so `main` is normally HEAD and auditing against it would compare nothing — the tool now refuses that with exit 2 rather than reporting clean). — flags tests that were deleted or weakened rather than the code being fixed. Treat its output as findings: every `[DELETED]` and `[WEAKENED]` is a **BLOCKER** unless the code was genuinely fixed and the test legitimately no longer applies; every `[RELAXED]` (a `// test-relax:` token) must be justified by a removed feature or replaced coverage — quote the reason and have the user confirm it. A test edited to match broken code IS the defect, not the fix. This pass exists because weakening tests to reach green is a recurring failure mode (see `ai/rules/testing.md`).

0.5. **Size the change (BEFORE step 1):** Count the changed lines and files. Ask "is this change bigger than the problem it solves?" A diff that is bigger than its problem gets ONE finding, a BLOCKER naming the smaller change, and the review stops there: auditing the details of over-engineered code ratifies it and drives another pass over more of it (`ai/rules/simplicity.md`, `ai/rules/context-economy.md`). Then match the process to the size: a few lines in one function earns one pass, not a loop.

1. **Wiring verification (FIRST — before any other analysis):** For every new function, type, handler, route, config option, CLI command, or plugin introduced in the diff, prove it is reachable from a user entry point. This is the FIRST step because it catches the project's most recurring defect class (see `plan/learned/RECURRING-PATTERNS.md`). If new code has no caller in production, nothing else in this review matters.

    For each new symbol, answer: **"What user action reaches this code?"** If you cannot name one, it is a BLOCKER.

    | New code type | Wiring check |
    |---------------|-------------|
    | Exported function/method | `grep` or LSP `findReferences` for at least one caller outside its own file and test files |
    | Struct / type | Same: at least one non-test consumer |
    | HTTP handler / web route | Registered on a mux (`srv.Handle`, `mux.HandleFunc`, etc.) and reachable from `hub/main.go` or `web/server.go` |
    | CLI command | Registered via `registry.MustRegisterLocal` or `registry.RegisterRoot` in a `register.go` with a blank import chain to `main.go` |
    | CLI command (completion) | Command appears in tab-completion (YANG command tree or plugin `CommandDecl` without `Hidden: true`). A command without completion is undiscoverable. See `ai/rules/cli.md` "Command Completion". |
    | Plugin | Has `register.go` with `registry.Register()`, appears in generated `all.go` (or will after `make generate`) |
    | Config option / YANG leaf | YANG module registered, leaf read by runtime code (not just parsed) |
    | Env var | `env.MustRegister()` call exists, `env.Get*()` call exists |
    | Metrics | Metric created AND updated somewhere reachable |
    | Event / send type | Listed in plugin `Registration.EventTypes`/`SendTypes`, at least one subscriber/caller |
    | Runtime dependency (file path, socket, listen port, kernel module, external binary, cert, procfs/sysctl, netlink) | A registered `ze doctor` check exists (plugin `Registration.DoctorChecks` or owning-package registration) AND a diagnostic code is registered in `internal/core/diagnostic/codes.go`. A new dependency with no doctor check is a BLOCKER: agents cannot verify host readiness before starting the daemon. See `ai/rules/repo-maintenance.md`. This surface is verified nowhere else in the pipeline. |

    **Do not skip this step.** "The code compiles" and "tests pass" do not prove wiring. A function with zero callers outside tests is dead code in production. Report every unwired symbol as a BLOCKER finding.

    **If any wiring BLOCKER is found:** report it immediately. Do not proceed to the remaining review steps until the user acknowledges. Unwired code means the feature does not exist from the user's perspective, so reviewing its correctness, security, or edge cases is premature.

    **Top-down feature walkthrough (MANDATORY for new features):** The bottom-up check above proves existing code is reachable. It cannot find code that should exist but was never written. For every new feature (new NLRI family, new plugin, new protocol, new command category), also trace TOP-DOWN:

    1. Enumerate every user-facing operation the feature enables (decode, encode, announce, withdraw, display, configure, filter, `ze bgp decode`).
    2. For each operation, trace the full path from user entry point through every component that must participate. Name every link in the chain.
    3. If any link has no implementation, that is a BLOCKER: "feature chain broken at [component]."

    | Feature type | Reference to compare against | Key registration/handler checklist |
    |-------------|-----------------------------|------------------------------------|
    | NLRI family | MUP plugin (`bgp/plugins/nlri/mup/`) | family registered, splitter registered, `registry.Registration{}` with RunEngine + InProcessNLRIDecoder + InProcessNLRIEncoder + InProcessRouteEncoder, command handler, functional test |
    | System plugin | An existing plugin of the same shape | `registry.Register()` in `register.go`, blank import in `all/all.go`, event types, send types, YANG schema |
    | BGP plugin | An existing BGP plugin of the same shape | `Registration{}` fields, hook functions, config handling |
    | Bridge command | An existing bridge command family | `parseFamilyToAFISAFI` case, `convertAnnounceFamily` regex, command parser, event translation |
    | CLI command | An existing CLI command | `registry.MustRegisterLocal`, handler, completion, YANG entry |

    Find the most similar existing feature. Diff its registrations, handlers, and tests against the new feature. Report anything the reference has that the new feature lacks as a BLOCKER: "missing [component] -- reference [feature] has it in [file] [symbol]."

2. **Functional test coverage (BLOCKING — immediately after wiring):** For every new or changed user-facing behavior in the diff, verify a functional test (`.ci` or `.et`) exists that exercises the full path. Apply the mapping from `ai/rules/testing.md`: match the change type to the required test directory and check for a test covering the behavior.

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
    | New feature, tool, make target, or verification gate | `ai/INDEX.md` keyword + task rows updated; `ai/LEARNED-INDEX.md` if the decision is structural; `ai/rules/repo-maintenance.md` if a new hook/gate. Per `ai/rules/repo-maintenance.md`. A feature that cannot be found from `ai/INDEX.md` or a discovery surface is an ISSUE. |

    Also grep `docs/` for `source: <changed-file>` for every changed source file. If any anchored claim is stale or missing after the code change, report an ISSUE. If a user-visible behavior changed and no documentation was updated or explicitly proven unnecessary, report an ISSUE.

4. **Identify changed files:** Run `git diff --name-only HEAD` to find all modified files.
5. **Read the actual code:** For every changed file, read the diff. Understand what changed.
6. **Understand intent via history:** For each changed region, run `git log --oneline -5` and `git blame` on the modified lines. Understand WHY the old code existed. Flag if the change removes a guard, workaround, or constraint that was added deliberately.
7. **Removed-behavior audit:** For every line the diff DELETES or replaces, name the invariant or behavior it enforced. Then search the new code for where that invariant is re-established. If you cannot find it, that is a finding: a removed guard, a dropped error path, a narrowed validation, a deleted test that covered a real case. This step is distinct from step 6: step 6 asks "why did the old code exist?" This step asks "is the protection still there?"

    **Test rewrite check (BLOCKING):** For every test file where the diff changes assertions (not just adds new ones), verify the OLD behavior is still tested. A test rewritten to cover a new issue while dropping the old assertion is a coverage regression, even when the assertion count stays the same. The hook cannot catch this (same structural shape); this step is the defense. Ask: "what did the old assertion prove, and where is that proof now?" If it is nowhere, report as BLOCKER: "test rewrite dropped coverage of [old behavior]." Rule: `ai/rules/testing.md` "Test Rewrite as Replacement."
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
    | Guard that fails open | A check whose miss/error/empty path returns the permissive value (allow, admin, nil error, "no violation") instead of denying. See `ai/rules/evidence.md` |
    | Valid-looking zero value | A bare map read (`m[k]`) or lookup whose zero result reads downstream as a legitimate answer: allow, match-nothing, success, count-of-1. `v, ok := m[k]` and handle `!ok`. Note a present-but-empty value passes `ok`: check `!ok \|\| len(v) == 0` when empty is also wrong |

    For each function: does the code do what the function name says?

    **Guard audit (BLOCKING when the diff adds or changes a guard).** For every check in the diff whose purpose is to reject, ask:

    1. **Does it fail closed?** Name the miss/error/empty path and the value it returns. If that value is permissive, it is a BLOCKER. A guard that neither denies nor logs does not exist.
    2. **Is the guard driven from its entry point in a test?** A unit test on the helper proves the helper, not that any caller reaches it with the input that matters. A green unit test on an uncalled guard is worse than no test: report as BLOCKER, "guard tested only via helper, no test drives it from [entry point]." Check the guard is reachable with the rejecting input at all: a constraint that cannot receive the value it rejects is inert.
    3. **Does the diff assert a safety property it does not prove?** Any doc, comment, or spec line in the diff claiming a check denies something ("RBAC denies privileged actions", "validated by YANG") must be traced to the producing function. If the diff does not prove it, report as BLOCKER: a false safety claim is the shield that stops the next reviewer asking. Per `ai/rules/evidence.md`.

    Never discard a finding here for being "unlikely": these degrade silently and each looks correct locally.

14. **Performance review:** Check changed code for unnecessary allocations and algorithmic issues, especially on hot paths (see `performance.md` "Hot Path Rule" for the list).

    | Check | What to look for |
    |-------|-----------------|
    | `fmt.Sprintf` / `fmt.Errorf` on hot path | Use `textbuf.Buffer`, `errors.New`, or append-based alternatives (see `performance.md`) |
    | `.String()` concatenation on hot path | Use `AppendTo` or `textbuf.Buffer` chain |
    | Allocation inside a loop | `make()`, `append()`, or string building per iteration when a single buffer outside the loop suffices |
    | Heap escape via interface boxing | Passing a concrete value through `any` or `interface{}` on a hot path |
    | Heap escape via closure capture | Closure capturing a local variable forces it to heap |
    | Redundant computation | Same derivation computed multiple times when it could be computed once and reused |
    | Missing precomputation | Value derived from configuration or negotiated state that does not change per-request but is recomputed on every call. Precompute at setup time, store on the struct, reuse on the hot path |
    | O(n^2) or worse | Nested loops over the same collection, linear scan inside a loop when a map lookup suffices |
    | Map with string key from known set | `map[string]V` where `map[uint16]V` or typed enum key would avoid hashing overhead |
    | `string([]byte)` for comparison | Compare bytes directly instead of converting to string |
    | Callee allocates what caller could provide | Function does `make([]byte, n)` when caller has a buffer in scope (see `performance.md`) |

    Cold paths (startup, config load, CLI one-shot) are exempt. Focus on hot paths as defined in `performance.md`.

15. **Plugin traversal + config-surface check:** If config structure changed, grep for all code reading the old structure. When the diff nests a config container, adds a plugin `show`/RPC command, registers a wire method, or adds a plugin-loading `.ci`, also apply the **Config-Surface & Command-Tree Checks** section (golden-snapshot sync, merged-node description parity, nested-ConfigRoot unwrap, needs-linux for dependency-pulling `.ci`).
16. **Altitude and simplicity check:** For each change, ask two questions. Both are about the amount of machinery, and they fail in opposite directions.

    **Too shallow (altitude):** is this fix at the right depth? A special case layered on shared infrastructure is a sign the underlying mechanism should be generalized instead. Prefer deepening the shared abstraction over adding per-caller workarounds. Report bandaid fixes as ISSUE with the deeper alternative named.

    **Too much (simplicity, `ai/rules/simplicity.md`):** is this the simplest FULLY CORRECT answer? For every construct in the table below that the diff adds, name the requirement or the second use case behind it. When neither exists, report as ISSUE, naming the construct and the simpler shape.

    | Construct in the diff | Report unless |
    |-----------------------|---------------|
    | New interface, or a new generic mechanism | A second implementation or a second call site exists in this diff or in the tree |
    | New config option, flag, or function parameter | A caller needs a different value, or an operator asked for the choice |
    | New wrapper, adapter, or layer | It transforms something (type conversion, error wrapping, defaults) |
    | New branch, guard, or error path | An input can actually produce that state |
    | New retry, cache, pool, or worker | A measurement, not an expectation, showed the problem |
    | A rewrite where a small change restores correctness | The small change was tried and does not restore it |

    **A simplicity finding never asks for less correctness.** Cutting an acceptance criterion, an RFC MUST, a test, or a guard is the opposite failure. It is already a BLOCKER under steps 2, 13, and 20. Quality is 0% compromise, and this step cuts machinery only.
17. **Project rules cross-check:** For each changed file, verify compliance with applicable rules (steps 13-14 above cover logic and performance specifically; this step covers structural and convention rules):

| Changed code touches | Check against |
|---------------------|---------------|
| Wire encoding/decoding | `performance.md` -- WriteTo(buf, off), no append/make in encoding |
| New goroutine | `goroutine-lifecycle.md` -- long-lived worker, not per-event |
| Naming (types, JSON keys, YANG) | `go-standards.md`, `cli.md` -- kebab-case JSON, ze- prefix |
| Plugin code | `plugins.md` -- proximity, YANG required, import rules |
| CLI handler | `cli.md` -- flag.NewFlagSet, exit codes, stderr for errors, tab-completion |
| Config parsing | `config.md` -- fail on unknown keys, no version numbers |
| New data wrapper/struct | `architecture.md` -- lazy over eager, no identity wrappers |
| Any new abstraction, option, layer, or parameter | `simplicity.md` -- the simplest fully correct answer, and nothing beyond it |

18. **Filter false positives:** Before reporting, discard findings that match any of these:

| False positive | Why discard |
|----------------|-------------|
| Pre-existing issue the goal does NOT depend on | Not introduced by these changes. If the goal depends on that path, "pre-existing" never excuses it: the test is dependency, never causation (`ai/rules/planning.md`, "Bounding the loop") |
| Linter/compiler-catchable (imports, types, formatting) | `make ze-lint` catches these separately |
| Issue on unmodified lines, when the goal does not depend on it | This review does not cover it. Never discard an always-in-scope class this way (`ai/rules/planning.md`, "Bounding the loop", which owns the list). An absence sits on no changed line, so this row would otherwise swallow every one of them |
| Intentional behavioral change clearly related to the broader diff | Not a bug, it is the point |
| General quality concern not tied to a specific bug | Too vague to act on. A simplicity finding from step 16 is NOT this: it names the construct, the file, and the simpler shape |
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

## Config-Surface & Command-Tree Checks

Failure modes seen building nested config surfaces and cross-module command
trees (the traffic / anomaly / ddos nesting). Apply whenever the diff adds or
nests a config container, adds a plugin `show`/RPC command, registers a new wire
method, or adds a `.ci` that loads a plugin. Each one broke silently or only
surfaced at daemon startup / on another OS -- not at compile time.

| Check | What to verify | Failure it prevents |
|-------|----------------|---------------------|
| **New wire method in the golden** | Every new `RegisterRPCs` WireMethod / `ze:command` is present (sorted) in `internal/component/plugin/all/testdata/wire-methods.snapshot`; a new plugin in `plugins.snapshot`; a new YANG provider in `yang-providers.snapshot`. | `TestRegisteredWireMethods` fails `unexpected wire-methods: ...`. The golden is hand-maintained -- `make generate` does NOT update it (learned 1046). |
| **Merged-node description parity** | When more than one module contributes the same container node (container-merge on a shared `show` parent, or `augment` into a shared config parent), the node's `description` is byte-identical across every contributing module. | `YANG command description mismatch node=X` warns on every startup. Fix: make the shared parent a pure namespace with one agreed description; put per-command text on the leaf command node. |
| **Nested ConfigRoot fully unwrapped** | A `ConfigRoot`/`WantsConfig` containing `/` (e.g. `traffic/usage`) is delivered as `{"traffic":{"usage":{...}}}`. The section reader unwraps EVERY segment -- never `root[configRoot]`, which looks up a literal `"traffic/usage"` key and silently yields an empty config. Doctors/completers use a path-aware lookup (`Tree.GetContainerPath`), never `GetContainer("<a/b>")`. | Config parses to empty with no error (plugin inert); a doctor/completer silently sees no container. |
| **needs-linux for dependency-pulling `.ci`** | A `test/plugin/*.ci` (or `reload`/`parse`) that configures a plugin whose `Registration.Dependencies` include `interface`/`firewall`/`vpp` (directly or transitively) sets `option=needs-linux`, or supplies a working backend. | The daemon starts the dependency plugin, which has no OS-default backend on darwin (`no backend configured and no OS default available`), so the test fails on macOS before the behavior under test runs. |

## Review Integrity

**Verify claims against source code, not docs.** Before stating what the code
does or comparing systems, read the implementation. Treat any existing
documentation as potentially stale. When multiple subsystems need verification,
spawn parallel agents to read each one.

**Fresh eyes on the FIRST pass.** Round 1 examines the full diff. A fresh lens
each round beats re-running one lens: logic and security on one pass,
performance and completeness on the next.

**Later rounds shrink, they do not repeat.** Round N+1 examines only the fixes
round N made and what those fixes touched. A full-diff pass every round never
converges, because a diff of any size always yields something new. The agent
is then unable to close work that is genuinely finished. A finding
outside the round's scope is still fixed when the goal this work exists to
achieve depends on it, when you are unsure whether it does, or when it belongs
to the always-in-scope classes. What those classes are, what happens to
everything else, and why none of it is parking are settled in ONE place:
`ai/rules/planning.md` "Bounding the loop". Do not restate the list or
the tests here. A second copy is how the corrected rule and the defective one
end up one hop apart, and an enumeration that falls short by one class reads as
permission to home the class it omitted.

## Rules

- Do NOT fix anything. Report findings only.
- Do NOT check spec completeness -- that is `/ze-review-spec`.
- After the user reviews your list, they will tell you which to fix.
- **Regression test required per fix:** When fixing an issue found by this review, add a test that would have caught the problem during development. The issue exists because a test was missing; the fix is incomplete without one. If a regression test is genuinely impossible (e.g., the finding is a naming convention violation), note why in the fix. Otherwise, no test = not fixed.
- No cap on the NUMBER of review passes, a hard bound on each one's SCOPE. Run a fresh pass whenever the code has changed since the last one, over the fixes that changed it. Stop when a pass finds nothing within its own scope. "I already reviewed this" is not a reason to stop. "A full re-read of the whole diff found something unrelated" is not a reason to continue (`ai/rules/planning.md`, "Bounding the loop").
