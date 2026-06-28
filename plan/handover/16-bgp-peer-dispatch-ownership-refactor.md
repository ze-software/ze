# Handover: remove BGP command spelling from shared dispatch

Goal: keep public BGP peer syntax and BGP-specific selector behavior in BGP-owned code, while reducing `internal/component/plugin/server` and `cmd/ze/internal/cmdutil` to generic command plumbing.

## Current evidence

Read these first:

- `plan/spec-command-surface-ownership.md`
- `ai/rules/cli-grammar.md`
- `ai/patterns/cli-command.md`
- `docs/architecture/api/process-protocol.md:788-796`

Key ownership evidence:

- Owner-specific daemon command registration should live with the owner package, not central generic packages (`plan/spec-command-surface-ownership.md:153-165`)
- Internal dispatch should avoid string tokenizer round-trips where typed paths already exist (`plan/spec-command-surface-ownership.md:64-65`)
- Shared dispatch currently reconstructs peer-scoped subcommands by prepending `peer <sel>` (`internal/component/plugin/server/dispatch.go:138-150,602-606`)
- Shared dispatcher still performs generic inline selector extraction (`internal/component/plugin/server/command.go:381-456`)
- CLI helper still extracts inline selectors from command-tree shape before `run` (`cmd/ze/internal/cmdutil/cmdutil.go:193-270`)
- BGP peer read syntax is now documented as `show bgp peer <selector> detail|rib` (`ai/patterns/cli-command.md`, `internal/component/bgp/plugins/cmd/peer/schema/ze-peer-cmd.yang`)

## What went wrong

A partial refactor tried to generalize selector extraction in shared code before the owner boundaries were fully moved. Full `make ze-verify` then exposed the real blast radius:

- plugin tests still dispatch `peer peer1 detail` through generic internal APIs;
- BFD show tests still use positional `show bfd session 203.0.113.9` rather than a typed selector form;
- policy tests dispatch `show policy test peer ...` and now trip generic selector enforcement;
- generic `plugin/server` and `cmdutil` still need to infer too much from user-facing command spelling.

Concrete failing examples from `make ze-verify`:

- `peer peer1 detail` -> `unknown command`
- `show bfd session 203.0.113.9` -> `unknown command`
- `show policy test peer test-peer export ...` -> `requires a selector`

Those failures show that the refactor is not complete enough to commit as code.

## Recommended refactor order

1. **Do not start in `plugin/server`**
   - Treat `internal/component/plugin/server` as the last boundary to simplify, not the first place to redesign public grammar.

2. **Inventory owner-specific internal dispatch callsites**
   - Find every in-process caller using `ze-plugin-engine:dispatch-command` or `dispatch-command-args` for BGP peer reads and writes.
   - Classify each callsite:
     - exact owner command path already known;
     - only selector + subcommand fragment known;
     - should use a typed owner API instead of string dispatch.

3. **Fix BGP-owned callsites first**
   - Make BGP-internal callers send the exact owner command path they mean, or bypass string command dispatch entirely with a typed owner API.
   - Keep public syntax unchanged.
   - Do not teach shared dispatch new BGP spellings.

4. **Move non-BGP command families independently**
   - BFD, policy test, L2TP, subscriber, and similar surfaces need their own owner-backed selector decisions.
   - Do not generalize from BGP and accidentally rewrite unrelated command families.

5. **Only then shrink shared plumbing**
   - Once owner callsites stop depending on implicit `peer <sel>` reconstruction, remove the BGP-shaped prepend from `internal/component/plugin/server/dispatch.go`.
   - Once the command tree or typed APIs carry enough structure, simplify generic selector extraction in `command.go` and `cmdutil.go`.

## Files to inspect next

- `internal/component/plugin/server/dispatch.go`
- `internal/component/plugin/server/command.go`
- `cmd/ze/internal/cmdutil/cmdutil.go`
- `internal/component/bgp/plugins/cmd/peer/`
- `internal/component/bgp/plugins/cmd/rib/`
- `internal/component/bgp/config/resolve.go`
- `internal/component/cmd/bfd/`
- `internal/component/cmd/show/`
- `test/plugin/*.ci`
- `test/scripts/ze_api.py`

## Verification to rerun after each cut

Start narrow:

1. `go test ./internal/component/plugin/server ./cmd/ze/internal/cmdutil ./internal/component/bgp/plugins/cmd/peer ./internal/component/bgp/plugins/cmd/rib ./internal/component/bgp/config`
2. `make ze-verify`

Use the verify failure groups to keep scope honest. The current breakage is visible in the functional plugin stage, so do not trust unit tests alone.

## Guardrails

- Do not invent new user-facing peer mutation syntax without source-backed agreement.
- Do not normalize config-tree mutation into RPC verbs.
- Do not move owner tests into generic infrastructure packages.
- Do not commit the current partial code refactor until `make ze-verify` is green.
