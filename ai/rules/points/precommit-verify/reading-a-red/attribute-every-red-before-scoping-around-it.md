---
kind: directive
level: MUST
stage:
---
**The red MUST be attributed, not assumed (BLOCKING).** "Known-red" means you
have identified the specific failing stage/test and confirmed it is pre-existing
(logged in `plan/known-failures/`) or owned by another active session. An
*undocumented* red is NOT scope-aroundable: treat it as possibly your own
regression until proven otherwise. Scope-to-changed has a blind spot -- it tests
packages you edited, not packages your edit breaks **transitively**: a new import
can break a different package's compile/test (a real case: `aihelp` broke
`bgp/config` through `plugin/all`), a config-driven gap surfaces only in a
consumer's tests (a missing YANG typedef failed `bgp/config`, not the plugin that
introduced it), and adding a plugin invalidates the `plugin/all` golden
snapshots. Before scoping around a red, `go test`/`vet` the reverse-dependency
closure of your changed packages, or run full `./le verify current mode full` once.
