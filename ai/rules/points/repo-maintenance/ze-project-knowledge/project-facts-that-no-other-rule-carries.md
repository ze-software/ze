---
kind: directive
level: MUST
stage:
---
- **Family registration** is dynamic via `PluginRegistry.Register()` -- never enumerate, validate format only.
- **Config pipeline**: File -> Tree -> `ResolveBGPTree()` -> `map[string]any` -> `reactor.PeersFromTree()`. Files: `internal/component/bgp/config/{resolve,peers}.go`, `.../reactor/config.go`.
- **Linter hook** (`auto-lint` in `.claude/hooks/posttool-writeedit.py`) runs gofmt + `goimports -format-only` on Edit/Write (imports are no longer auto-removed) -- still add import + usage in the same edit.
- **Arch-0**: 4 components (Engine, ConfigProvider, PluginManager, Subsystem). Subsystem != Plugin (BGP daemon = subsystem; bgp-rib/rs/gr = plugins). Stream system = pub/sub backbone (`internal/component/plugin/server/dispatch.go`). Interfaces in `pkg/ze/`.
- **YANG choice/case**: `mandatory true` and inner-choice exclusivity NOT enforced by the walker. Plugins using `choice` add Go-side validation in their parser. `ze config validate` does not invoke `OnConfigVerify`.
- **Constants for command/status names** -- literals catch typos at compile time. Editor commands: `internal/component/cli/model.go`. Plugin status: `plugin.StatusDone`/`StatusError`.
- **Proximity**: `bgp/handler/` is a middleman; handlers belong in `bgp/plugins/`. ALL RPCs need YANG. See `ai/rules/plugins.md`.
- **LSP** at session start for Go nav -- more precise than grep for call chains and interface impls.
- **Inventory**: `make ze-inventory [--json]` imports `plugin/all` and queries real registries. Use for plugin counts, RPC totals, family coverage.
- **SDK type aliases** (`pkg/plugin/sdk/sdk_types.go` re-exporting `rpc.*`) are intentional -- external plugins import only `sdk`. Not identity wrappers.
- **No filtered/noexport route tracking** -- Ze does not store import-filtered or export-filtered routes (unlike BIRD's "import keep filtered on"): the RIB pipeline has scope keywords (sent/received/sent-received) and filter stages, but no "filtered" scope. The birdwatcher-compatible endpoints `/routes/filtered/{name}` and `/routes/noexport/{name}` return empty lists for compatibility; if filtered tracking ever lands, point them at the real store.
- **Gokrazy appliance owns process lifecycle** -- ze deploys as a gokrazy appliance: no systemd, no init system, no package manager. Any external process ze depends on (VPP or future dependencies) MUST be exec'd, supervised, and cleaned up by ze itself; ze MUST NOT be designed around an OS-level process manager.
- **Stress injector is in-memory Go**: the BGP UPDATE stream for stress scenarios 01-05 is generated inside `ze-test peer --mode inject` and streamed over the TCP socket after the OPEN handshake. Extend the Go injector for new scenarios with a pool-friendly byte builder, one pre-allocated buffer, one TCP writer, and a keepalive goroutine. `test/stress/` is the Python harness (`harness.py`, `run.py`, `scenarios/`).
- **CLI dispatch discoverability gaps**: (1) no one-shot command against a RUNNING daemon (`ze cli -c "summary"` shape). `ze show` and `ze run` use SSH (`sshclient.ExecCommand`) internally but expose no shell one-liner. The offline-config half is covered by `ze config show <file> [path...]`. (2) `ze help --ai --api` prints YANG RPC names (`ze-bgp:overview`), not the dispatch strings users type. (3) No way to list the Dispatcher's match keys. `reactor.ExecuteCommand()` accepts strings undiscoverable without reading source. The highest-value fix is the one-shot daemon command (SSH port 2222, credentials from the zefs database).
