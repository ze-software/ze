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
