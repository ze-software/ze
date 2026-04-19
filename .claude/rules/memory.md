# Ze Project Memory

Rationale: `.claude/rationale/memory.md`

## Maintenance (BLOCKING at session end)

1. **Dedup** against `.claude/rules/*.md`
2. **Stale**: drop entries referencing deleted files/functions
3. **Merge** related bullets, heading + 1-3 lines max
4. **Overflow** >5 lines -> `.claude/rationale/memory.md`
5. **Cap**: 200 lines hard

Consult `.claude/rationale/<name>.md` when the compressed rule leaves gaps.

## Project Knowledge (not in other rules)

- **Family registration** is dynamic via `PluginRegistry.Register()` -- never enumerate, validate format only.
- **Config pipeline**: File -> Tree -> `ResolveBGPTree()` -> `map[string]any` -> `reactor.PeersFromTree()`. Files: `internal/component/bgp/config/{resolve,peers}.go`, `.../reactor/config.go`.
- **Linter hook** (`auto_linter.sh`) runs goimports on Edit/Write -- add import + usage in the same edit.
- **Arch-0 restructuring**: 4 components (Engine, ConfigProvider, PluginManager, Subsystem). Subsystem != Plugin (BGP daemon is subsystem; bgp-rib/rs/gr are plugins). Stream system is the pub/sub backbone (`internal/component/plugin/server/dispatch.go`). Interfaces in `pkg/ze/`. Umbrella: `plan/learned/425-arch-0-system-boundaries.md`.
- **YANG choice/case**: `mandatory true` and inner-choice mutual exclusivity are NOT enforced by the walker. Plugins using `choice` must add Go-side validation in their parser. `ze config validate` does not invoke `OnConfigVerify`.
- **Constants for command/status names** -- string literals catch typos at compile time. Editor commands: `internal/component/cli/model.go`. Plugin status: `plugin.StatusDone`/`StatusError`.
- **Proximity**: `bgp/handler/` is a middleman; handlers belong in `bgp/plugins/`. ALL RPCs need YANG. See `rules/plugin-design.md`.
- **LSP** at session start for Go navigation -- more precise than grep for call chains and interface impls.
- **Inventory**: `make ze-inventory [--json]` imports `plugin/all` and queries real registries. Use for plugin counts, RPC totals, family coverage.
- **SDK type aliases** (`pkg/plugin/sdk/sdk_types.go` re-exporting `rpc.*`) are intentional -- external plugins import only `sdk`. Not identity wrappers.

## Mistake Log

Each entry: one-line lesson + rule pointer. Full root-cause lives in the linked learned summary.

- **Spec status at START, not end** (RECURRING, ZERO TOLERANCE). Hook `block-source-edit-spec-not-in-progress.sh` enforces. `rules/planning.md`.
- **Feature not wired** (RECURRING, ZERO TOLERANCE). Unit tests != wiring. Name the user entry point or it's not done. `rules/integration-completeness.md`.
- **Daemon command without offline CLI** (sysctl-0). Every `CommandDecl` plugin needs `cmd/ze/<name>/` offline entry point. Test `ze <plugin> <cmd>` from shell.
- **Wrong production path** (rib-04). Grep ALL implementations of a protocol step; trace the consumer's call chain.
- **Count-only test assertions** (addpath-rib). Assert on content (keys/values), not `Len()`.
- **Wrapper struct pattern** (alloc-4). Pass raw bytes + existing iterators. Never wrap data in accessor-method types.
- **Tests-pass != done** (RECURRING). Tests are step 10 of 12. Continue to docs/spec/summary/audit. `rules/quality.md`.
- **Mechanism-not-behavior test** (prefix-limit). Assert the AC text, not a code-path proxy. If a no-op passes, the test is wrong. `rules/tdd.md`.
- **"Pre-existing" failures** (RESOLVED). Fix in-session after primary task; log to `plan/known-failures.md` if >10 min. `rules/anti-rationalization.md`.
- **Plugin placement anchor bias** (jsonrpc). "Delete the folder" test. Cross-cutting -> `internal/component/`. Domain -> `bgp/plugins/`. Infra -> `internal/core/`.
- **Docs from assumption** (RECURRING). Read source before any factual claim. `rules/documentation.md` Source Anchors.
- **Spec deleted without committing** (lg-overhaul, ZERO TOLERANCE). TWO commits: (A) code+spec, (B) `git rm` spec + add summary. `rules/spec-preservation.md`.
- **Reinventing repo contents** (lg-overhaul). Grep before writing new infra; `third_party/` and other components often already have it. `rules/before-writing-code.md`.
- **Spec claimed complete with gaps** (lg-0..4). Learned summary saying "future X" is proof spec is NOT done. Audit each AC against running code. `rules/implementation-audit.md`.
- **Stale deferrals** (redist-phase2). Grep code before creating a phase-N spec from open deferrals. `rules/deferral-tracking.md`.
- **Worktree copy into main** (ZERO TOLERANCE). Commit in worktree; merge/cherry-pick only. Hook `block-worktree-copy.sh` enforces.
- **Same-day blocker fix** (cmd-4, RECURRING). Run adversarial review for real: race on reactor code, grep renamed-name consumers, grep sibling call sites, break production to confirm .ci test fails. `rules/quality.md`.
- **Substring collision in bulk edits** (iface-tunnel). Longest prefix first, or add non-name context. Grep for mangled names after.
- **Vendor != upstream** (iface-tunnel). Verify against `vendor/<lib>/`, not upstream docs. Cite vendor path in the spec.
- **Naive reconciliation drops live state** (iface-tunnel). Diff against previous config; only act on the delta. Pass `previous` explicitly.
- **Invented config shape** (iface-tunnel). Grep existing `*-conf.yang` for the closest analog before defining new endpoint shapes.
- **Scratch `.go` in `tmp/`** (iface-tunnel). `go test ./...` walks `tmp/`. Research agents must use `.txt` or build-tagged dirs.
