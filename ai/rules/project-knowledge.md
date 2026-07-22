# Ze Project Knowledge

**When:** One-line lesson + rule pointer
**Severity:** advisory

## Project Knowledge (not in other rules)

- **Family registration** is dynamic via `PluginRegistry.Register()` -- never enumerate, validate format only.
- **Config pipeline**: File -> Tree -> `ResolveBGPTree()` -> `map[string]any` -> `reactor.PeersFromTree()`. Files: `internal/component/bgp/config/{resolve,peers}.go`, `.../reactor/config.go`.
- **Linter hook** (`auto-lint` in `.claude/hooks/posttool-writeedit.py`) runs gofmt + `goimports -format-only` on Edit/Write (imports are no longer auto-removed) -- still add import + usage in the same edit.
- **Arch-0**: 4 components (Engine, ConfigProvider, PluginManager, Subsystem). Subsystem != Plugin (BGP daemon = subsystem; bgp-rib/rs/gr = plugins). Stream system = pub/sub backbone (`internal/component/plugin/server/dispatch.go`). Interfaces in `pkg/ze/`. Umbrella: `plan/learned/425-arch-0-system-boundaries.md`.
- **YANG choice/case**: `mandatory true` and inner-choice exclusivity NOT enforced by the walker. Plugins using `choice` add Go-side validation in their parser. `ze config validate` does not invoke `OnConfigVerify`.
- **Constants for command/status names** -- literals catch typos at compile time. Editor commands: `internal/component/cli/model.go`. Plugin status: `plugin.StatusDone`/`StatusError`.
- **Proximity**: `bgp/handler/` is a middleman; handlers belong in `bgp/plugins/`. ALL RPCs need YANG. See `ai/rules/plugin-design.md`.
- **LSP** at session start for Go nav -- more precise than grep for call chains and interface impls.
- **Inventory**: `make ze-inventory [--json]` imports `plugin/all` and queries real registries. Use for plugin counts, RPC totals, family coverage.
- **SDK type aliases** (`pkg/plugin/sdk/sdk_types.go` re-exporting `rpc.*`) are intentional -- external plugins import only `sdk`. Not identity wrappers.
- **No filtered/noexport route tracking** -- Ze does not store import-filtered or export-filtered routes (unlike BIRD's "import keep filtered on"): the RIB pipeline has scope keywords (sent/received/sent-received) and filter stages, but no "filtered" scope. The birdwatcher-compatible endpoints `/routes/filtered/{name}` and `/routes/noexport/{name}` return empty lists for compatibility; if filtered tracking ever lands, point them at the real store.
- **Gokrazy appliance owns process lifecycle** -- ze deploys as a gokrazy appliance: no systemd, no init system, no package manager. Any external process ze depends on (VPP or future dependencies) must be exec'd, supervised, and cleaned up by ze itself; never design around an OS-level process manager.
- **Stress injector is in-memory Go** (decision 2026-04-16) -- the BGP UPDATE stream for stress scenarios 01-05 is generated in memory inside `ze-test peer --mode inject` and streamed over the TCP socket after the OPEN handshake; no file on disk, no bngblaster. Extend the Go injector for new scenarios (pool-friendly byte builder, single pre-allocated buffer, single TCP writer with keepalive goroutine). The standalone byte-level oracle and BNG Blaster have been removed now that the Go builder is trusted; `test/stress/` is the Python harness (`harness.py`/`run.py`/`scenarios/`).
- **CLI dispatch discoverability gaps** (2026-03-30 live debugging; spec candidates): (1) no one-shot command against a RUNNING daemon (`ze cli -c "summary"` shape) -- `ze show`/`ze run` use SSH (`sshclient.ExecCommand`) internally but expose no shell one-liner; the offline-config half is covered by `ze config show <file> [path...]` (49f04ffd3). (2) `ze help --ai --api` prints YANG RPC names (`ze-bgp:summary`), not the dispatch strings users type. (3) No way to list the Dispatcher's match keys. `reactor.ExecuteCommand()` accepts strings undiscoverable without reading source; highest-value fix is the one-shot daemon command (SSH port 2222, credentials from the zefs database).

## Mistake Log

One-line lesson + rule pointer. Full root-cause in the linked learned summary.

- **"Linux-only tests can't run on this macOS host / need hardware" is a LIE** (RECURRING, ZERO TOL). Ze HAS a QEMU Alpine-VM harness: `option=needs-linux` `.ci` tests SKIP on native darwin and RUN under `make ze-qemu-needs-linux-test` / `ze-qemu-all-test`; kernel/netlink/nft/veth/loop tests run via `make ze-qemu-integration-test` and the `ze-qemu-<feature>-test` targets. A Linux-only test that FAILS (not skips) on native darwin is missing its `option=needs-linux` marker (fix: add it, then run it in QEMU), never "environmental / unfixable here". NEVER attribute a Linux test red to "darwin env" or "needs docker/qemu we don't have" — we HAVE QEMU. Run it. `ai/rules/qemu-testing.md`.
- **Feature not wired** (RECURRING, ZERO TOL). Unit tests != wiring. Name the user entry point. `ai/rules/integration-completeness.md`.
- **Daemon command without offline CLI** (sysctl-0). Every `CommandDecl` plugin needs `cmd/ze/<name>/` offline entry point.
- **Wrong production path** (rib-04). Grep ALL implementations; trace the consumer's call chain.
- **Count-only test assertions** (addpath-rib). Assert on content (keys/values), not `Len()`.
- **Wrapper struct pattern** (alloc-4). Pass raw bytes + existing iterators. Never wrap data in accessor types.
- **Tests-pass != done** (RECURRING). Tests are step 10 of 12. Continue to docs/spec/summary/audit. `ai/rules/quality.md`.
- **Mechanism-not-behavior test** (prefix-limit). Assert the AC, not a code-path proxy. No-op passes = wrong test. `ai/rules/tdd.md`.
- **"Pre-existing" failures** (RESOLVED). Fix in-session after primary task; log to `plan/known-failures/` if >10 min. `ai/rules/anti-rationalization.md`.
- **Plugin placement anchor bias** (jsonrpc). "Delete the folder" test. Cross-cutting -> `internal/component/`. Domain -> `bgp/plugins/`. Infra -> `internal/core/`.
- **Docs from assumption** (RECURRING). Read source before any factual claim. `ai/rules/documentation.md` Source Anchors.
- **Spec deleted without committing** (lg-overhaul, ZERO TOL). TWO commits: (A) code+spec, (B) `git rm` spec + add summary. `ai/rules/spec-preservation.md`.
- **Reinventing repo contents** (lg-overhaul). Grep before writing new infra; `third_party/` and components often already have it. `ai/rules/before-writing-code.md`.
- **Spec claimed complete with gaps** (lg-0..4). Learned summary with "future X" = spec NOT done. Audit each AC. `ai/rules/implementation-audit.md`.
- **Stale deferrals** (redist-phase2). Grep code before creating phase-N spec from open deferrals. `ai/rules/deferral-tracking.md`.
- **Worktree copy into main** (ZERO TOL). Commit in worktree; merge/cherry-pick only. Hook `block-worktree-copy.sh` enforces.
- **Same-day blocker fix** (cmd-4, RECURRING). Real adversarial review: race on reactor code, grep renamed-name consumers, grep sibling call sites, break production to confirm .ci test fails. `ai/rules/quality.md`.
- **Substring collision in bulk edits** (iface-tunnel). Longest prefix first, or add non-name context. Grep for mangled names after.
- **Vendor != upstream** (iface-tunnel). Verify against `vendor/<lib>/`, not upstream docs. Cite vendor path in the spec.
- **Naive reconciliation drops live state** (iface-tunnel). Diff against previous config; act on the delta. Pass `previous` explicitly.
- **Invented config shape** (iface-tunnel). Grep existing `*-conf.yang` for the closest analog before defining new endpoint shapes.
- **Scratch `.go` in `tmp/`** (iface-tunnel). `go test ./...` walks `tmp/`. Research agents use `.txt` or build-tagged dirs.
- **CLI grammar from container nesting, not wire method** (as112-cli-audit). Operator-facing command words come from the YANG `container` tree; `ze:command "ze-X:Y"` is the INTERNAL RPC name and is deliberately different (e.g. `ze-bgp:peer-teardown` = command `request peer teardown`). Never infer command syntax from wire-method names. Top-level operational verb is `request` (`request <object> <action>`); reads are `show`/`monitor`. `ai/rules/documentation.md`.
- **ExaBGP migration sync** (exabgp-compat-sync). When ExaBGP adds a new SAFI or route type, three things need updating: (1) `exabgp.yang` schema container, (2) `flexSafis` list or a dedicated `convert*ToUpdate` in `migrate_routes.go`, (3) compat test files (`.ci` + `.conf`). `ai/patterns/bgp-family.md` Section 5b.
