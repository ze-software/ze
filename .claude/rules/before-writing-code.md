# Before Writing Code

**BLOCKING:** Complete before writing any code, tests, or documentation.
Rationale: `.claude/rationale/before-writing-code.md`

```
[ ] 1. Read pattern cookbook — if touching CLI, web, plugin, config, or tests: read `.claude/patterns/<domain>.md` first. See `.claude/INDEX.md` "I Want To..." table.
[ ] 2. Search for existing implementations (Grep/Glob) — extend if found
[ ] 3. Know source files — use digests if available, read + write digest if not
[ ] 4. Verify file paths exist (Glob/Grep)
[ ] 5. Buffer-first check (wire encoding) — see `rules/buffer-first.md`
[ ] 6. Lazy-first check — can the consumer use existing wire type methods directly? See `design-principles.md` "Lazy over eager"
[ ] 7. Bulk-edit check — modifying >2 files with the same pattern? Change ONE first, test it, confirm it works, THEN apply to the rest. Never assume a pattern works across files without validation.
[ ] 8. Sibling call-site audit — when adding a guard/fallback/retry to ONE call site of a function, grep all OTHER call sites of that function and apply the same change in the same commit. See "Sibling Call-Site Audit" below.
```

Before any spec: READ source files, document current behavior, preserve by default.

## Memory Lifecycle Tracing

Before implementing any buffer, pool, or allocation: trace the complete memory lifecycle first.
- Where is memory allocated? Who holds it? When is it copied? When released?
- Ze lifecycle: allocate at receive (Incoming Peer Pool), share read-only through forwarding,
  copy only when egress filters modify (Outgoing Peer Pool), release after TCP write.
- The acquisition point defines the design: "every dispatch" vs "only on modification" are
  fundamentally different. A pool that provides buffers is not a counter.
- Look at filter code and `buildModifiedPayload` to understand WHERE modification happens
  before deciding WHERE buffers come from.

**Red flags:** new file without checking for similar existing ones; function that might duplicate existing; can't name 3 related files.

## Sibling Call-Site Audit

When you add a guard, fallback, retry, or special case to a call site of a
shared function, **grep every other call site of that function in the same
commit**. Apply the same fix to every site that needs it.

The blob-store fallback fix (`d029a94d` + `5f66e4f5`) added a missing
filesystem fallback to `store.ReadFile` -- in *one* call site. Three sibling
call sites were missed. They masked five plugin tests for six days.

| Trigger | Required action |
|---------|----------------|
| Adding a `nil` check on a result | Check every other caller for the same nil case |
| Adding a fallback when an external system is unavailable | Check every other caller of the same dependency |
| Adding a retry / backoff | Check every other caller that does the same I/O |
| Adding error wrapping with new context | Check every other caller wrapping the same error |
| Replacing direct call with helper | Check every other caller that should also use the helper |

**Mechanical check:**

```
# What function did you just add a guard to?
fn="store.ReadFile"
# Show every call site
grep -rn "$fn" internal/ pkg/ cmd/ --include="*.go"
```

For each match: does it need the same guard? If yes, fix it now. If no,
state in the commit message *why* this caller is exempt (e.g., "config-tx
path uses an in-memory tree, not the blob store"). Silence is bug bait.

**Why this is a separate rule from "bulk-edit check" (item 7):** bulk-edit
covers "I am making the same change to N files I already know about."
Sibling-audit covers "I am making a change to ONE file and may not know
which OTHER files need it." The first is a discipline; the second is a
discovery step.
