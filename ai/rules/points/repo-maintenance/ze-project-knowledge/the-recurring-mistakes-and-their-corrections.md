---
kind: directive
level:
stage:
---
- **"Linux-only tests can't run on this macOS host / need hardware" is a LIE** (RECURRING, ZERO TOL). Ze HAS a QEMU Alpine-VM harness: `option=needs-linux` `.ci` tests SKIP on native darwin and RUN under `make ze-qemu-needs-linux-test` / `ze-qemu-all-test`; kernel/netlink/nft/veth/loop tests run via `make ze-qemu-integration-test` and the `ze-qemu-<feature>-test` targets. A Linux-only test that FAILS (not skips) on native darwin is missing its `option=needs-linux` marker (fix: add it, then run it in QEMU), never "environmental / unfixable here". NEVER attribute a Linux test red to "darwin env" or "needs docker/qemu we don't have": we HAVE QEMU. Run it. `ai/rules/platform-linux.md`.
- **Feature not wired** (RECURRING, ZERO TOL). Unit tests != wiring. Name the user entry point. `ai/rules/completion.md`.
- **Daemon command without offline CLI** (sysctl-0). Every `CommandDecl` plugin needs `cmd/ze/<name>/` offline entry point.
- **Wrong production path** (rib-04). Grep ALL implementations; trace the consumer's call chain.
- **Count-only test assertions** (addpath-rib). Assert on content (keys/values), not `Len()`.
- **Wrapper struct pattern** (alloc-4). Pass raw bytes + existing iterators. Never wrap data in accessor types.
- **Tests-pass != done** (RECURRING). Tests are step 10 of 12. Continue to docs/spec/summary/audit. `ai/rules/quality.md`.
- **Mechanism-not-behavior test** (prefix-limit). Assert the AC, not a code-path proxy. No-op passes = wrong test. `ai/rules/testing.md`.
- **"Pre-existing" failures** (RESOLVED). Blocks your goal: fix now. Does not: spec it, close the work in hand, ask Thomas whether that spec runs. `ai/rules/completion.md`.
- **Plugin placement anchor bias** (jsonrpc). "Delete the folder" test. Cross-cutting -> `internal/component/`. Domain -> `bgp/plugins/`. Infra -> `internal/core/`.
- **Docs from assumption** (RECURRING). Read source before any factual claim. `ai/rules/writing.md` Source Anchors.
- **Spec deleted without committing** (lg-overhaul, ZERO TOL). TWO commits: (A) code+spec, (B) `git rm` spec + add summary. `ai/rules/planning.md`.
- **Reinventing repo contents** (lg-overhaul). Grep before writing new infra; `third_party/` and components often already have it. `ai/rules/architecture.md`.
- **Spec claimed complete with gaps** (lg-0..4). Learned summary with "future X" = spec NOT done. Audit each AC. `ai/rules/completion.md`.
- **Stale deferrals** (redist-phase2). Grep code before creating phase-N spec from open deferrals. `ai/rules/planning.md`.
- **Worktree copy into main** (ZERO TOL). Commit in worktree; merge/cherry-pick only. `check_worktree_copy` in `.claude/hooks/pretool-bash.py` enforces.
- **Same-day blocker fix** (cmd-4, RECURRING). Real adversarial review: race on reactor code, grep renamed-name consumers, grep sibling call sites, break production to confirm .ci test fails. `ai/rules/quality.md`.
- **Substring collision in bulk edits** (iface-tunnel). Longest prefix first, or add non-name context. Grep for mangled names after.
- **Vendor != upstream** (iface-tunnel). Verify against `vendor/<lib>/`, not upstream docs. Cite vendor path in the spec.
- **Naive reconciliation drops live state** (iface-tunnel). Diff against previous config; act on the delta. Pass `previous` explicitly.
- **Invented config shape** (iface-tunnel). Grep existing `*-conf.yang` for the closest analog before defining new endpoint shapes.
- **Scratch `.go` in `tmp/`** (iface-tunnel). `go test ./...` walks `tmp/`. Research agents use `.txt` or build-tagged dirs.
- **CLI grammar from container nesting, not wire method** (as112-cli-audit). Operator-facing command words come from the YANG `container` tree; `ze:command "ze-X:Y"` is the INTERNAL RPC name and is deliberately different (e.g. `ze-bgp:peer-teardown` = command `request peer teardown`). Never infer command syntax from wire-method names. Top-level operational verb is `request` (`request <object> <action>`); reads are `show`/`monitor`. `ai/rules/writing.md`.
- **ExaBGP migration sync** (exabgp-compat-sync). When ExaBGP adds a new SAFI or route type, three things need updating: (1) `exabgp.yang` schema container, (2) `flexSafis` list or a dedicated `convert*ToUpdate` in `migrate_routes.go`, (3) compat test files (`.ci` + `.conf`). `ai/patterns/bgp-family.md` Section 5b.
