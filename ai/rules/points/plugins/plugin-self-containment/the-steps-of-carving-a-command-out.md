---
kind: directive
level: MUST
stage:
---
1. **Handler:** add `func init() { pluginserver.RegisterRPCs(...) }` + the handler in the owner package. If the owner package is already blank-imported (it has a `register.go` found by the generator's `pluginDirs`, or sits in `rpcDirs`), the registration links with NO generator or manual-island change. The handler imports only `plugin` + `pluginserver` (+ the owner's own API), so it does not create an import cycle.
2. **Schema (container merge, NOT `augment`):** add `<owner>/yang/ze-<x>-cmd.yang`, a standalone module that re-declares the path from the root: `container show { container <x> { ... ze:command "ze-show:<x>"; } }`. The YANG loader unions same-named top-level containers across all registered modules, so the owner module needs no `import`/`augment` of the central schema and has no base-module coupling. Give it a unique `namespace`/`prefix` and `import ze-extensions`. Add the embed var + `yang.RegisterModule` call. A NEW `<owner>/yang/` package whose `register.go` imports `config/yang` is auto-discovered, so run `go run internal/le/pluginimports/pluginimports.go` to refresh `internal/component/plugin/all/all.go`.
3. **Schema location:** the command YANG MUST live in `<owner>/yang/` (top level, sibling of `cli`/`cmd`), and MUST NOT be nested under `<owner>/cmd/yang`.
4. **Both halves of the invariant:** the owner `yang/` gets a presence test asserting its command tokens ARE declared; the central verb schema test bans the moved tokens (below).
