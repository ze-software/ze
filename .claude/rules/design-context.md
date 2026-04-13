# Design Context

**BLOCKING:** Before making any design decision (proposing communication mechanisms,
naming schemes, package placement, platform backends, lifecycle behavior), load the
relevant context from this checklist. Trained instincts about "how software works"
are wrong here. Ze has opinions.

Rationale: Session as-router (2026-04-13) made 7 wrong design recommendations because
the session started designing before understanding Ze's patterns. Each mistake required
the user to correct it. The context existed in docs but was not loaded.

## Tier 1: Always Read Before Any Design

These shape every design decision. Read before proposing anything.

| What | Where | What it prevents |
|------|-------|-----------------|
| Design principles | `rules/design-principles.md` | "Good enough" backends, translation layers, implicit behavior, premature abstraction |
| Plugin architecture | `rules/plugin-design.md` | Wrong package placement, import violations, wrong communication mechanism |
| Registration pattern | `patterns/registration.md` | Missing the modular core pattern (init + registry + blank import) |
| Existing core packages | `ls internal/core/` | Missing existing patterns like `internal/core/family/` for shared registries |

## Tier 2: Read When Designing a Specific Artifact

Match the artifact type to its pattern. Read BEFORE proposing a design.

| Artifact | Read | What it prevents |
|----------|------|-----------------|
| New plugin | `patterns/plugin.md` | Wrong structure, missing YANG, wrong callback |
| Cross-plugin communication | `pkg/ze/eventbus.go` + `internal/component/plugin/events.go` + one existing consumer (e.g., fibkernel) | Proposing DispatchCommand when EventBus is the pattern |
| Shared registry | `internal/core/family/` (read actual code) | Putting registry inside a plugin instead of core |
| Config option | `patterns/config-option.md` + `rules/config-design.md` | Missing env var registration, wrong YANG shape |
| CLI command | `patterns/cli-command.md` | Wrong dispatch structure |
| Platform-specific code | Existing backend splits (e.g., `fibkernel/backend_linux.go`, `ifacenetlink/sysctl_linux.go`) | Wrong build tag pattern, wrong abstraction level |
| Naming anything | `rules/naming.md` + grep existing code for analogous names | Inventing ze-specific names when kernel/standard names exist |

## Tier 3: Read When the Design Touches These Areas

| Area | Read | What it prevents |
|------|------|-----------------|
| Plugin startup timing | `internal/component/plugin/server/startup.go` (TopologicalTiers, runPluginPhase) | Hand-waving about timing instead of understanding tier ordering |
| Wire encoding | `rules/buffer-first.md` | Allocations in encoding code |
| Env vars | `rules/go-standards.md` (env section) + `internal/core/env/` | Using os.Getenv, missing MustRegister |
| JSON format | `rules/json-format.md` | Wrong key casing |
| Testing | `rules/testing.md` + `patterns/functional-test.md` | Missing .ci tests, wrong test structure |
| Daemon lifecycle | Existing OnStarted/OnAllPluginsReady in a similar plugin | Wrong callback choice, missing cleanup |

## Anti-Patterns This Rule Prevents

| Anti-pattern | What to do instead |
|-------------|-------------------|
| "Industry standard is X" | Grep ze for how it already does X |
| "This is good enough for dev" | Ze's principle: "Do it right." Darwin could be prod. |
| "Translation layer for cleaner API" | Ze's principle: "Explicit > implicit." Use native names. |
| "Put the registry where it's used" | Check if `internal/core/` has a pattern for this (family, env, metrics) |
| "DispatchCommand for cross-plugin calls" | Check if EventBus is already used for this pattern |
| "No cleanup needed on stop" | Ze owns what it touches. Check existing cleanup patterns. |
| "Defaults are suggestions" | Defaults are requirements. Log when overridden. |

## Mechanical Check

Before presenting any design option to the user:

1. Did I read how ze already handles a similar feature? (grep, not assume)
2. Did I check `internal/core/` for an existing shared pattern?
3. Did I read the relevant pattern file from `.claude/patterns/`?
4. Does my proposal contradict any principle in `rules/design-principles.md`?
5. Am I inventing a name when a standard/kernel/existing name exists?
