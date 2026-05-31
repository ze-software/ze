# Known Failures

Pre-existing test failures tracked here per `ai/rules/git-safety.md` ("Before Any
Commit" → pre-existing failures >10 min): logged, not blocking unrelated commits.

_None currently open._

## Resolved

### 2026-05-31 — dispatch single-marshal + stale plugin lists (15 packages)

**Resolved 2026-05-31.** The 15 packages that began failing once `make ze-verify`
was runnable again (after the `tmp/go.mod` sentinel landed) are all green. Fixes,
by class:

1. **`single-marshal OnExecuteCommand` (commit 30b025270).** Command handlers now
   return structured `any`; the SDK marshals once. Tests that did
   `assert.Contains(t, data, "substring")` were comparing against a `map`/`[]byte`/
   typed slice (key/element match, never substring). Fixed by asserting on the
   marshaled JSON string: `adj_rib_in`, `healthcheck`, `rs`, `fib/kernel`,
   `fib/p4`, `fakeredist`.
2. **Stale registration / section lists.** Added the new plugins
   (`bgp-filter-aspath-length`, `flow-export`, `ldp`, `rsvp-te`) to the expected
   sets in `cmd/ze` and `internal/component/plugin/all`, and `platform` to the
   `cmd/ze/host` section list.
3. **Migration serializer keyword gap (commit 3da416d31).** The `internal` plugin
   keyword landed with updated goldens but `migrate_serialize.go` still emitted
   `external`. It now emits `internal` for built-in (`use`) plugins, `external`
   for `run` processes.
4. **Multi-line YANG descriptions.** `cmd/ze/completion/words.go` now collapses
   descriptions to their first line so shell completion stays one row per
   candidate; `internal/component/config/yang` description-propagation assertions
   updated to the enriched strings.
5. **CLI grammar catch-up (committed refactors 336cb2472 modes, 72d268c77 view
   consolidation).** `summary` doc lookup → `show summary` (canonical verb-first
   path); `option changes` is a display column, not a pipe redirect (only `blame`
   redirects); 7 `.et` files updated to the shipped grammar (`exit` switches
   config→operational, `show | blame` / `show | changes [all]` for views,
   `disconnect` requires an active session in completions).
