# Review For Issues

Quick single-pass review (~2 min) for bugs, edge cases, security gaps, and missing tests in uncommitted changes.

This review answers: **"What can go wrong that nobody planned for?"**

See also: `/ze-review-deep` (exhaustive multi-agent review), `/ze-review-spec` (spec completeness check), `/ze-review-docs` (documentation accuracy)

## Steps

1. **Wiring verification (FIRST — before any other analysis):** For every new function, type, handler, route, config option, CLI command, or plugin introduced in the diff, prove it is reachable from a user entry point. This is the FIRST step because it catches the project's most recurring defect class (see `plan/learned/RECURRING-PATTERNS.md`). If new code has no caller in production, nothing else in this review matters.

    For each new symbol, answer: **"What user action reaches this code?"** If you cannot name one, it is a BLOCKER.

    | New code type | Wiring check |
    |---------------|-------------|
    | Exported function/method | `grep` or LSP `findReferences` for at least one caller outside its own file and test files |
    | Struct / type | Same: at least one non-test consumer |
    | HTTP handler / web route | Registered on a mux (`srv.Handle`, `mux.HandleFunc`, etc.) and reachable from `hub/main.go` or `web/server.go` |
    | CLI command | Registered via `cmdregistry.MustRegisterLocal` or `cmdregistry.RegisterRoot` in a `register.go` with a blank import chain to `main.go` |
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

3. **Identify changed files:** Run `git diff --name-only HEAD` to find all modified files.
4. **Read the actual code:** For every changed file, read the diff. Understand what changed.
5. **Understand intent via history:** For each changed region, run `git log --oneline -5` and `git blame` on the modified lines. Understand WHY the old code existed. Flag if the change removes a guard, workaround, or constraint that was added deliberately.
6. **Check code comments:** Read WARNING, INVARIANT, NOTE, and TODO comments in modified files. Verify the changes do not violate stated invariants or ignore documented constraints.
7. **Trace data flow:** For each changed component, trace data from entry through transformations to exit. Verify boundaries are respected.
8. **Apply edge case techniques:** Apply EVERY technique in the table below to every changed component.
9. **Security review:** Apply the security checklist to every user-controlled input.
10. **Allocation review:** Check every `make()` in changed code for unbounded sizes.
11. **Plugin traversal check:** If config structure changed, grep for all code reading the old structure.
12. **Project rules cross-check:** For each changed file, verify compliance with applicable rules:

| Changed code touches | Check against |
|---------------------|---------------|
| Wire encoding/decoding | `buffer-first.md` -- WriteTo(buf, off), no append/make in encoding |
| New goroutine | `goroutine-lifecycle.md` -- long-lived worker, not per-event |
| Naming (types, JSON keys, YANG) | `naming.md`, `json-format.md` -- kebab-case JSON, ze- prefix |
| Plugin code | `plugin-design.md` -- proximity, YANG required, import rules |
| CLI handler | `cli-patterns.md` -- flag.NewFlagSet, exit codes, stderr for errors |
| Config parsing | `config-design.md` -- fail on unknown keys, no version numbers |
| New data wrapper/struct | `design-principles.md` -- lazy over eager, no identity wrappers |

13. **Filter false positives:** Before reporting, discard findings that match any of these:

| False positive | Why discard |
|----------------|-------------|
| Pre-existing issue (present before this diff) | Not introduced by these changes |
| Linter/compiler-catchable (imports, types, formatting) | `make ze-lint` catches these separately |
| Issue on unmodified lines | Out of scope for this review |
| Intentional behavioral change clearly related to the broader diff | Not a bug, it is the point |
| General quality concern not tied to a specific bug | Too vague to act on |
| Contradicts a project rule but has an explicit override comment in code | Intentional exception |

    **Never discard wiring or functional-test findings.** An unwired symbol is not a false positive, a pre-existing issue, or a quality concern. It is dead code in production. A missing functional test is not a quality concern; it is a coverage gap. Wiring BLOCKERs from step 1 and functional-test BLOCKERs from step 2 always survive this filter.

14. **Interop and goal validation check:** If the diff implements or modifies protocol behavior (BGP capability, NLRI family, session behavior, wire format, authentication), verify per `ai/rules/interop-and-goal-validation.md`:
    - Does an interop test scenario exist that proves this works with another daemon?
    - If the spec has a Goal Validation table, is every goal backed by concrete evidence?
    Missing interop test for protocol work is a BLOCKER. Empty goal validation for a completed feature is an ISSUE.

15. **Report findings** as a numbered list with severity:
    - **BLOCKER:** Bug that will cause incorrect behavior, crash, or security vulnerability
    - **ISSUE:** Missing test, edge case not handled, or quality problem
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

## Rules

- Do NOT fix anything. Report findings only.
- Do NOT check spec completeness -- that is `/ze-review-spec`.
- After the user reviews your list, they will tell you which to fix.
- No cap on review passes. Run a fresh pass whenever the code has changed since the last one, and keep running passes until a pass finds nothing. "I already reviewed this" is not a reason to stop -- fixes introduce new code, new code needs review.
