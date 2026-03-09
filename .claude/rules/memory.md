# Ze Project Memory

Rationale: `.claude/rationale/memory.md`

## Maintenance (BLOCKING at session end)

Before committing:
1. **Dedup**: remove entries already in `.claude/rules/*.md`
2. **Stale**: remove entries referencing deleted files/functions
3. **Merge**: combine related bullets, heading + 1-3 lines max
4. **Overflow**: entries >5 lines → `.claude/rationale/memory.md`
5. **Cap**: 200 lines hard limit (system truncates after)

## When to Consult Rationale

Read `.claude/rationale/<name>.md` when a rule needs context, examples, or the compressed rule doesn't fully cover the situation.

## Project Knowledge (not in other rules)

### Family Registration
Families registered dynamically by plugins via `PluginRegistry.Register()` — not a static list.
Validate format (contains "/", non-empty parts) — never enumerate all families.

### Config Pipeline
File → Tree → `ResolveBGPTree()` → `map[string]any` → `reactor.PeersFromTree()`.
Key files: `component/config/resolve.go`, `component/config/peers.go`, `reactor/config.go`.

### Bash Timeout
Default 15000ms. Longer only for `make ze-verify`, `make ze-unit-test`.

### Linter Hook
`auto_linter.sh` runs goimports on Edit/Write. Add import + usage in same edit to avoid cascading removals.

### Architecture Restructuring (arch-0)
Umbrella spec: `docs/plan/spec-arch-0-system-boundaries.md`. Six phases.
Key decisions agreed with user:
- **5 components:** Engine (supervisor), Bus (content-agnostic pub/sub), ConfigProvider, PluginManager, Subsystem
- **Subsystem ≠ Plugin:** BGP daemon is a subsystem (owns TCP/FSM), bgp-rib/rs/gr are plugins
- **Bus is content-agnostic:** payload always `[]byte`, bus never type-asserts. Like RabbitMQ/Kafka.
- **Topics:** hierarchical with `/` separator (`bgp/update`, `bgp/events/peer-up`). Prefix-based subscription matching.
- **Interfaces in `pkg/ze/`** — public so external plugins can depend on them
- **ConfigManager is central authority** — editor (`ze config edit`), web UI, subsystems, plugins all use same interface
- **Performance matters** — user explicitly asked for performance-conscious design
- **`make ze-verify`** before closing spec/committing
- **Cross-check child specs against umbrella** after each phase

### Bash Timeout for ze-verify
`make ze-verify` needs timeout 120s (2 min) — runs lint + unit + functional + exabgp + chaos.

### Constants for Command/Status Names
String literals used as command names or status values must be constants — compiler catches typos that `case "sett":` would silently miss. Editor commands live in `config/editor/model.go`. Plugin status uses `plugin.StatusDone`/`plugin.StatusError`.

### Proximity Principle & Handler Location
`bgp/handler/` is a middleman — command handlers belong in `bgp/plugins/` (self-contained).
ALL RPCs need YANG — no "command module" category. Missing YANG is a bug, not a design choice.
"Delete the folder" is a mechanical check for proximity. See `rules/plugin-design.md`.

### SDK Type Aliases Are Intentional
`pkg/plugin/sdk/sdk_types.go` re-exports `rpc.*` types as `sdk.*` aliases. This is deliberate —
external plugin authors import only `sdk`, never `rpc`. Decouples public API from internal structure.
Do NOT flag these as "identity wrappers adding no value."

## Mistake Log

### Feature Not Wired (RECURRING — multiple specs)
- Write logic + unit tests, claim "done", but feature is NOT reachable from reactor/CLI/config.
- User cannot use the feature. Tests pass in isolation but nothing calls the code.
- Root cause: treat unit tests as proof of completion. Skip wiring into reactor, skip functional tests.
- **Rule:** `rules/integration-completeness.md` already says this. FOLLOW IT. Before claiming done: can a user reach this feature through config/CLI/API? If not, it is not done. A unit test is not a wiring test.

### Wrong Production Path (rib-04)
- Wrote spec pointing at `subsystem.go` stage-1 handler. Production path is `server_startup.go`.
- Root cause: found *a* handler, assumed it was *the* handler. Never traced the actual call chain.
- **Rule:** grep for ALL implementations of a protocol step, identify which one the consumer calls.

### Count-Only Test Assertions (addpath-rib)
- Test asserted `Len()==2` on map-backed store. Wrong parsing produced entries that deduped to same count.
- **Rule:** When testing wire parsing into map storage, assert on content (keys/values via Lookup) not just count.

### Wrapper Struct Pattern (alloc-4, three attempts)
- Attempt 1: eager `StructuredEvent` pre-computed FilterResult (N→1 when answer is N→0)
- Attempt 2: `UpdateHandle` wrapped raw data with lazy methods + cached fields (identity wrapper)
- Root cause: defaulted to "struct with accessor methods" instead of ze pattern: pass raw bytes, use existing iterators
- **Rule:** before creating any new type for data access, ask "can the consumer use existing wire types directly?"
