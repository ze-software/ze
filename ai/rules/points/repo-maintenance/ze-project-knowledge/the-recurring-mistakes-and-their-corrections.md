---
kind: directive
level: MUST
stage:
---
- **"Linux-only tests can't run on this macOS host / need hardware" is a LIE** (RECURRING, ZERO TOL). Ze HAS a QEMU Alpine-VM harness: `option=needs-linux` `.ci` tests SKIP on native darwin and RUN under `make ze-qemu-needs-linux-test` / `ze-qemu-test-all`; kernel/netlink/nft/veth/loop tests run via `make ze-qemu-integration-test` and the `ze-qemu-<feature>-test` targets. A Linux-only test that FAILS (not skips) on native darwin is missing its `option=needs-linux` marker (fix: the marker MUST be added, then the test MUST be run in QEMU), never "environmental / unfixable here". A Linux test red MUST NOT be attributed to "darwin env" or "needs docker/qemu we don't have": we HAVE QEMU, and the test MUST be run there. `ai/rules/platform-linux.md`.
- **Feature not wired** (RECURRING, ZERO TOL). Unit tests != wiring. The user entry point MUST be named. `ai/rules/completion.md`.
- **Daemon command without offline CLI** (sysctl-0). Every `CommandDecl` plugin MUST have a `cmd/ze/<name>/` offline entry point.
- **Wrong production path** (rib-04). ALL implementations MUST be grepped; the consumer's call chain MUST be traced.
- **Count-only test assertions** (addpath-rib). Assertions MUST be on content (keys/values), not `Len()`.
- **Wrapper struct pattern** (alloc-4). Raw bytes and existing iterators MUST be passed. Data MUST NOT be wrapped in accessor types.
- **Tests-pass != done** (RECURRING). Tests are step 10 of 12. Work MUST continue to docs/spec/summary/audit. `ai/rules/quality.md`.
- **Mechanism-not-behavior test** (prefix-limit). The AC MUST be asserted, not a code-path proxy. No-op passes = wrong test. `ai/rules/testing.md`.
- **"Pre-existing" failures** (RESOLVED). Blocks your goal: it MUST be fixed now. Does not: spec it, close the work in hand, ask Thomas whether that spec runs. `ai/rules/completion.md`.
- **Plugin placement anchor bias** (jsonrpc). "Delete the folder" test. Cross-cutting -> `internal/component/`. Domain -> `bgp/plugins/`. Infra -> `internal/core/`.
- **Docs from assumption** (RECURRING). Source MUST be read before any factual claim. `ai/rules/writing.md` Source Anchors.
- **Spec deleted without committing** (lg-overhaul, ZERO TOL). TWO commits MUST be made: (A) code+spec, (B) `git rm` spec + add summary. `ai/rules/planning.md`.
- **Reinventing repo contents** (lg-overhaul). Existing code MUST be grepped before writing new infra; `third_party/` and components often already have it. `ai/rules/architecture.md`.
- **Spec claimed complete with gaps** (lg-0..4). Learned summary with "future X" = spec NOT done. Each AC MUST be audited. `ai/rules/completion.md`.
- **Stale deferrals** (redist-phase2). Code MUST be grepped before a phase-N spec is created from open deferrals. `ai/rules/planning.md`.
- **Worktree copy into main** (ZERO TOL). Work MUST be committed in the worktree, and it MUST reach main only via merge or cherry-pick. `check_worktree_copy` in `.claude/hooks/pretool-bash.py` enforces.
- **Same-day blocker fix** (cmd-4, RECURRING). A real adversarial review MUST race on reactor code, grep renamed-name consumers, grep sibling call sites, and break production to confirm the `.ci` test fails. `ai/rules/quality.md`.
- **Substring collision in bulk edits** (iface-tunnel). The longest prefix MUST be matched first, or non-name context MUST be added. Mangled names MUST be grepped for afterward.
- **Vendor != upstream** (iface-tunnel). Behavior MUST be verified against `vendor/<lib>/`, not upstream docs. The vendor path MUST be cited in the spec.
- **Naive reconciliation drops live state** (iface-tunnel). The new config MUST be diffed against the previous config, and the delta MUST be acted on. `previous` MUST be passed explicitly.
- **Invented config shape** (iface-tunnel). Existing `*-conf.yang` files MUST be grepped for the closest analog before new endpoint shapes are defined.
- **Scratch `.go` in `tmp/`** (iface-tunnel). `go test ./...` walks `tmp/`. Research agents MUST use `.txt` or build-tagged dirs.
- **CLI grammar from container nesting, not wire method** (as112-cli-audit). Operator-facing command words come from the YANG `container` tree; `ze:command "ze-X:Y"` is the INTERNAL RPC name and is deliberately different (e.g. `ze-bgp:peer-teardown` = command `request peer teardown`). Command syntax MUST NOT be inferred from wire-method names. Top-level operational verb is `request` (`request <object> <action>`); reads are `show`/`monitor`. `ai/rules/writing.md`.
- **ExaBGP migration sync** (exabgp-compat-sync). When ExaBGP adds a new SAFI or route type, three things MUST be updated: (1) `exabgp.yang` schema container, (2) `flexSafis` list or a dedicated `convert*ToUpdate` in `migrate_routes.go`, (3) compat test files (`.ci` + `.conf`). `ai/patterns/bgp-family.md` Section 5b.
