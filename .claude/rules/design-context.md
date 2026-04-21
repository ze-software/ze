# Design Context

**BLOCKING:** Before any design decision (communication mechanism,
naming, package placement, platform backend, lifecycle), load the
relevant context below. Trained instincts about "how software works"
are wrong here -- ze has opinions.

Incident: session as-router (2026-04-13) made 7 wrong recommendations
by starting to design before loading context.

## Tier 1: Always Read Before Any Design

| What | Where | Prevents |
|------|-------|----------|
| Design principles | `rules/design-principles.md` | "Good enough" backends, translation layers, implicit behavior, premature abstraction |
| Plugin architecture | `rules/plugin-design.md` | Wrong package, import violations, wrong comm mechanism |
| Registration pattern | `patterns/registration.md` | Missing init + registry + blank import pattern |
| Existing core packages | `ls internal/core/` | Missing patterns like `internal/core/family/` |

## Tier 2: When Designing a Specific Artifact

| Artifact | Read | Prevents |
|----------|------|----------|
| New plugin | `patterns/plugin.md` | Wrong structure, missing YANG, wrong callback |
| Cross-plugin comm | `pkg/ze/eventbus.go` + `internal/component/plugin/events.go` + one consumer (e.g. fibkernel) | DispatchCommand when EventBus is the pattern |
| Shared registry | `internal/core/family/` (read the code) | Registry inside a plugin instead of core |
| Config option | `patterns/config-option.md` + `rules/config-design.md` | Missing env var, wrong YANG shape |
| CLI command | `patterns/cli-command.md` | Wrong dispatch structure |
| Platform-specific | Existing splits (`fibkernel/backend_linux.go`, `ifacenetlink/sysctl_linux.go`) | Wrong build tag, wrong abstraction level |
| Naming | `rules/naming.md` + grep analogous names | Inventing ze-names when kernel/standard names exist |

## Tier 3: When the Design Touches These Areas

| Area | Read | Prevents |
|------|------|----------|
| Plugin startup timing | `internal/component/plugin/server/startup.go` (`TopologicalTiers`, `runPluginPhase`) | Hand-waving instead of tier ordering |
| Wire encoding | `rules/buffer-first.md` | Allocations in encoding |
| Env vars | `rules/go-standards.md` + `internal/core/env/` | `os.Getenv`, missing `MustRegister` |
| JSON format | `rules/json-format.md` | Wrong key casing |
| Testing | `rules/testing.md` + `patterns/functional-test.md` | Missing .ci tests, wrong structure |
| Daemon lifecycle | `OnStarted`/`OnAllPluginsReady` in a similar plugin | Wrong callback, missing cleanup |

## Anti-Patterns

| Anti-pattern | Instead |
|--------------|---------|
| "Industry standard is X" | Grep ze for how it already does X |
| "Good enough for dev" | "Do it right." Darwin could be prod |
| "Translation layer for cleaner API" | "Explicit > implicit." Use native names |
| "Put the registry where it's used" | Check `internal/core/` first |
| "DispatchCommand for cross-plugin calls" | EventBus is the pattern |
| "No cleanup needed on stop" | Ze owns what it touches |
| "Defaults are suggestions" | Defaults are requirements; log when overridden |

## Mechanical Check

1. Did I read how ze already handles similar? (grep, not assume)
2. Did I check `internal/core/` for an existing shared pattern?
3. Did I read the relevant `patterns/` file?
4. Does my proposal contradict `rules/design-principles.md`?
5. Am I inventing a name when standard/kernel/existing exists?
