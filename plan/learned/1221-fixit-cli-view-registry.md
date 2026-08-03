# 1221 -- CLI live-view registry: migrate dashboard/ping/traceroute off per-feature Model fields

## Context

`plan/spec-fixit-cli-view-registry.md`. The bubbletea `cli.Model`
(`internal/component/cli/model.go`) hardcoded a per-feature field + five switch
sites (fields, `Update` message switch, both command-dispatch chains, key
handling, render) for each rich live view (dashboard, ping, traceroute) -- the
exact anti-pattern `ai/rules/plugins.md` names and bans. Replaced
with a client-side view registry the Model discovers, mirroring the daemon-side
`RegisterMonitorProvider` in `internal/component/plugin/server/handler.go`.

## What was built

- `internal/component/cli/view_registry.go`: an `activeView` lifecycle interface
  (`update`/`render`/`key`/`release`), a `viewSpec` registration descriptor, a
  `viewMsg` marker so one `case viewMsg` arm routes all view ticks, `RegisterView`/
  `RegisteredViews`, `resolveView` longest-prefix match, and `prefixMatch` copied
  verbatim from `handler.go matchesPrefix` (word-boundary: `monitor bgpx` must not
  match `monitor bgp`). `Model` carries one `activeView` handle + a generic
  `viewFactories map[string]any` instead of per-view fields.
- Design 1 (resolved in the spec): registrants live in-package via `init()` in
  `register_view_*.go`, so render/update/state stay in `cli` and `cli` imports no
  ping/traceroute engine (the `model_ping.go` factory-contract inversion is
  preserved). The `RegisterView`-in-`init()` MUST live in a `register*.go` file --
  the `pretool-writeedit.py` hook hard-blocks `RegisterView` inside `init()` in any
  other file (`ai/patterns/registration.md`). That is why the registrants are in
  `register_view_*.go`, not literally in `model_*.go`.
- Consumers (`cmd/ze/hub/session_factory.go`, `internal/component/cli/client/main.go`)
  iterate `cli.RegisteredViews()` and inject each factory by key.

## Lessons

1. **A view-switch must release the OUTGOING view's context, or it leaks.**
   Review found (BLOCKER) that starting a second view while a `| log`-mode view is
   live overwrote `activeView` without cancelling the old view's `context.CancelFunc`
   -- an orphaned goroutine/channel. HEAD leaked identically (the old dispatch called
   `startX` with no stop-first), so it was pre-existing, but the spec's Security
   Review mandated no-leak-on-switch. Fix: in `handleEnter` dispatch,
   `prev := m.activeView; cmd := spec.start(&m, input); if prev != nil && m.activeView != prev { prev.release() }`.
   `release()` is a **cancel-only** teardown (no scrollback/viewport side effects,
   unlike the Esc/q `stop*` path). The `m.activeView != prev` test is true iff
   `start` installed a new view, so a FAILED start (bad args / nil factory, which
   return before the `activeView` assignment) leaves the old view running -- do NOT
   tear down the current view when the new command is invalid.

2. **An AC-4-style "no new per-feature field" ratchet must be an allowlist, not a
   name denylist.** The first guard banned the substrings `dashboard/ping/traceroute`;
   a future `bfdSession *bfdState` field would slip past, so the AC's literal "flags
   ANY new per-feature view field" was unmet. Replaced with a field-name ALLOWLIST
   (`knownModelFields` in `model_test.go`): reflect over `Model`, fail on any
   unrecognized field. This forces a conscious decision on every new field (register
   a viewSpec, or add to the allowlist) -- the `TestShowSchemaHasNoBGPPluginCommands`
   mechanical-backstop style the rule demands.

3. **Copy the mirror's function body AND its calling contract.** `prefixMatch` is
   byte-identical to `handler.go matchesPrefix`, but the daemon lowercases input
   before calling; the client path is case-sensitive (consistent with the old
   `isXCommand` `HasPrefix` literals, so not a regression). When mirroring a helper,
   note the surrounding normalization, not just the function.

## Known-minor follow-ups (non-blocking NITs from review)

- The factory getters (`pingFactory()` etc.) funnel both "no factory registered"
  and "wrong factory type" into the same `"not available (no daemon connection)"`
  status; a genuine registration/type bug reads as a connectivity problem.
- `injectViewFactories` (both consumers) hard-switches on the three known view
  keys, so a future registered view still needs that switch updated to receive its
  factory (Design 1 keeps concrete factory construction in the consumer).

Links: `ai/patterns/registration.md` (init + registry + longest-prefix) ·
`ai/rules/plugins.md` ("Registration over hardcoding (the CLI client too)") ·
mirror `internal/component/plugin/server/handler.go` (`RegisterMonitorProvider`/`matchesPrefix`).

## Files

None recorded.
