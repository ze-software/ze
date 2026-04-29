# DANGER -- ABSOLUTE PROHIBITIONS

**These rules override everything. No exceptions. No rationalization. No "the task requires it."**

## git push is FORBIDDEN -- no exceptions
- NEVER run `git push`. Not to main, not to any branch, not from any worktree.
- There is no scenario where pushing is acceptable. The user pushes manually.

## git commit, git add, git rm: FORBIDDEN from the Bash tool
- NEVER invoke `git commit`, `git add`, `git rm`, `git restore --staged`,
  or `git stash` from a Bash tool call. Sessions share staging; cross-commits
  result. Commit only via a script the user triggers.
- Script pattern (add + commit + `-F message-file`, no heredocs) lives in
  `ai/rules/git-safety.md` under "Commit Rules". Read it before writing any
  commit script.
- Never `--no-verify`, never `--no-gpg-sign`.

## Destructive git commands are FORBIDDEN
- NEVER: `git reset`, `git checkout -- <file>`, `git restore`, `git clean`, `git revert`
- NEVER: `git push --force`, `git push -f`
- NEVER: `git stash drop`, `git stash clear`
- To undo something: write the command to `tmp/delete-SESSION.sh`, tell the user, and STOP.

## Worktree agents must not touch main
- Work on your own branch. Commit there. Done.
- NEVER merge, cherry-pick, rebase, or copy into main.

## Worktrees: only on instruction or to keep worktree work on worktree
- NEVER spawn a worktree agent on your own initiative. Only use worktrees when the user explicitly instructs it.
- If work originated in a worktree, keep it there. Do not move worktree work to the main working tree.

## On violation: STOP immediately
"The task requires it" is not valid. Nothing overrides these prohibitions.

---

# Ze - {{TOOL}} Instructions

## Core Architecture

Ze is a **Network OS** in Go with its own BGP implementation and interface configuration. "Ze" = "The" with a French accent (predecessor: ExaBGP).

**Small core + registration pattern.** Components and plugins register at startup via `init()` in `register.go`. Core discovers them through registries -- never imports directly. Registration is the unifying pattern: families, capabilities, CLI commands, config validators, web routes all register the same way.

**Components** (`internal/component/`) are independent unless they explicitly depend on each other: bgp, cli, config, dns, iface, lg, managed, mcp, ssh, telemetry, web, bus, hub, authz, engine.

**Plugins** (`internal/plugins/`) handle domain policy (RIB, route reflection, graceful restart, NLRI families). Communication: JSON events down, text commands up.

**CLI** -- SSH-accessible network OS CLI: YANG-modeled config editor with modes, completion, diff, commit, history, dashboard, monitoring.

**Web** -- HTMX-based UI: config editor, admin, SSE live updates, ASN decorators.

**Looking Glass** -- peer/route viewer with birdwatcher-compatible API, topology graph, SSE streaming.

**Config** -- YANG-modeled. File -> Tree -> `ResolveBGPTree()` -> `map[string]any` -> `reactor.PeersFromTree()`.

**Key wire abstractions:** `WireUpdate` (lazy-parsed, zero-copy), `PackContext` (negotiated capabilities), `ContextID` (same = forward unchanged), pool dedup (per-attribute, refcounted), buffer-first (`WriteTo(buf, off) int`).

## Programs

| Binary | Purpose |
|--------|---------|
| `ze` | Network OS: bgp, cli, config, hub, iface, exabgp migrate, plugin, schema, signal, completion |
| `ze-chaos` | Chaos testing orchestrator: fault injection, scheduling |
| `ze-perf` | Performance benchmarking: UPDATE throughput tracking |
| `ze-analyse` | MRT/RIB analysis: attributes, communities, density, dump |
| `ze-test` | Functional test runner: bgp, editor, peer, mcp, web, rpki, managed |

## Source Layout

| Area | Location |
|------|----------|
| Components | `internal/component/` (bgp, cli, config, dns, iface, web, lg, ssh, ...) |
| BGP engine | `internal/plugins/bgp/` (reactor, FSM, wire, message, capability) |
| Plugin impls | `internal/plugins/bgp-rib/`, `bgp-rs/`, `bgp-gr/`, `bgp-nlri-*/` |
| Plugin infra | `internal/plugin/` (registry, process, hub, SDK) |
| Programs | `cmd/ze/`, `cmd/ze-chaos/`, `cmd/ze-perf/`, `cmd/ze-analyse/`, `cmd/ze-test/` |
| Tests | `test/` (.ci), `*_test.go` |

## Before You...

| Action | Read first |
|--------|-----------|
| Edit CLAUDE.md or AGENTS.md | Edit `ai/INSTRUCTIONS.md` (single source), then `make ze-ai-instructions` |
| Start a session | `rules/session-start.md` |
| Design or implement anything | `ai/rules/design-context.md` -- grep ze before proposing, never default to trained instincts |
| Write any code | `ai/rules/before-writing-code.md`, relevant `ai/patterns/` |
| Write a backend or config translator | `ai/rules/exact-or-reject.md` -- no silent approximation, lossy translation rejects at verify |
| Touch wire encoding | `ai/rules/buffer-first.md` |
| Touch registration | `ai/patterns/registration.md` |
| Add CLI/web/plugin/config | `ai/patterns/{cli-command,web-endpoint,plugin,config-option}.md` |
| Write help text, usage strings, error messages, or docs that enumerate things | `ai/rules/derive-not-hardcode.md` -- derive from the registry/map, never re-hardcode; return structured data, not pre-formatted strings |
| Write tests | `ai/rules/testing.md`, `ai/rules/tdd.md` |
| Implement an RFC | `ai/rules/rfc-compliance.md`, `rfc/short/` |
| Write a spec | `ai/rules/planning.md`, `plan/TEMPLATE.md` |
| Commit | `ai/rules/git-safety.md` -- `make ze-verify` |
| Run any test/build/lint command | `rules/bash-output.md` -- no pipes, read log after |
| Delete / overwrite any user-visible file | `ai/rules/never-destroy-work.md` -- ask first, always |
| Look up anything | `ai/INDEX.md` (keyword->doc, keyword->RFC) |
| Understand architecture | `docs/architecture/core-design.md` |
| Check past decisions | `ai/LEARNED-INDEX.md` -> `plan/learned/` |
| Understand why the code is shaped this way | `plan/learned/DESIGN-HISTORY.md` |
| Check if you are about to hit a known trap | `plan/learned/RECURRING-PATTERNS.md` |
| Understand why a hook rejected your code | `plan/learned/HOOK-FRICTION.md` |
