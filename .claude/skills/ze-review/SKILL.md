# Review For Issues

Quick single-pass review (~2 min) for bugs, edge cases, security gaps, and missing tests in uncommitted changes.

This review answers: **"What can go wrong that nobody planned for?"**

See also: `/ze-review-deep` (exhaustive multi-agent review), `/ze-review-spec` (spec completeness check), `/ze-review-docs` (documentation accuracy)

## Steps

1. **Identify changed files:** Run `git diff --name-only HEAD` to find all modified files.
2. **Read the actual code:** For every changed file, read the diff. Understand what changed.
3. **Understand intent via history:** For each changed region, run `git log --oneline -5` and `git blame` on the modified lines. Understand WHY the old code existed. Flag if the change removes a guard, workaround, or constraint that was added deliberately.
4. **Check code comments:** Read WARNING, INVARIANT, NOTE, and TODO comments in modified files. Verify the changes do not violate stated invariants or ignore documented constraints.
5. **Trace data flow:** For each changed component, trace data from entry through transformations to exit. Verify boundaries are respected.
6. **Apply edge case techniques:** Apply EVERY technique in the table below to every changed component.
7. **Security review:** Apply the security checklist to every user-controlled input.
8. **Allocation review:** Check every `make()` in changed code for unbounded sizes.
9. **Plugin traversal check:** If config structure changed, grep for all code reading the old structure.
10. **Project rules cross-check:** For each changed file, verify compliance with applicable rules:

| Changed code touches | Check against |
|---------------------|---------------|
| Wire encoding/decoding | `buffer-first.md` -- WriteTo(buf, off), no append/make in encoding |
| New goroutine | `goroutine-lifecycle.md` -- long-lived worker, not per-event |
| Naming (types, JSON keys, YANG) | `naming.md`, `json-format.md` -- kebab-case JSON, ze- prefix |
| Plugin code | `plugin-design.md` -- proximity, YANG required, import rules |
| CLI handler | `cli-patterns.md` -- flag.NewFlagSet, exit codes, stderr for errors |
| Config parsing | `config-design.md` -- fail on unknown keys, no version numbers |
| New data wrapper/struct | `design-principles.md` -- lazy over eager, no identity wrappers |

11. **Filter false positives:** Before reporting, discard findings that match any of these:

| False positive | Why discard |
|----------------|-------------|
| Pre-existing issue (present before this diff) | Not introduced by these changes |
| Linter/compiler-catchable (imports, types, formatting) | `make ze-lint` catches these separately |
| Issue on unmodified lines | Out of scope for this review |
| Intentional behavioral change clearly related to the broader diff | Not a bug, it is the point |
| General quality concern not tied to a specific bug | Too vague to act on |
| Contradicts a project rule but has an explicit override comment in code | Intentional exception |

12. **Report findings** as a numbered list with severity:
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
- Maximum 3 review passes. If issues remain after 3 passes, list them and stop.
