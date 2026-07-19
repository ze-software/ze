# 1218 - fixit-pppoe-orphaned-tests

## Problem

`test/pppoe/` (3 `.ci`) was orphaned dead code: it could not parse AND nothing
ran it. Two independent failures cancelled into zero signal, so it survived unrun
from May to July 2026:

1. **Unparseable directive.** All 3 files carry `option=netns:veth=...`.
   `parseOption` (`internal/test/runner/record_parse.go`) has 12 cases (`file`,
   `asn`, `bind`, `timeout`, `tcp_connections`, `open`, `update`, `env`, `skip-os`,
   `needs-linux`, `skip-env`, `require-tag`) and no `netns` case, so the directive
   hits the fail-closed `default:` (`unknown option type %q`).
2. **Unregistered suite.** `internal/test/cli/register.go` roots 20 suites via
   `registerCIRoot`; `pppoe` is not among them, so nothing ever walks `test/pppoe/`.

Neither alone is silent: an unrooted dir fails no build (nothing walks it), and an
unknown option type errors only when something parses the file. Together they cancel.

## Decision

Adopted the spec's plan of record: **Option D (delete the 3 stale `.ci`) + C' (add
`TestCIRootsRegistered` recurrence guard).** No repair (Option A). RFC 2516 discovery
is already covered by `test/pppoe-interop/scenarios/01-pppoe-chap-ipv4/` and
`ze-qemu-pppoe-accel-test`; repairing the `.ci` would be net-new veth/topology
construction, not restoration.

## Key findings

- **A per-test netns LAUNCH mode DOES exist** (`internal/test/runner/netns_linux.go`,
  `enterTestNetns`, gated by `netnsModeActive()` — a global env-driven runner mode,
  "Fix B" of spec-netlink-ci-harness). It is NOT the `option=netns:veth=` `.ci`
  directive: that directive, and the veth topology it implies, were never built.
  Refines assumption A-1: the pppoe directive is genuinely unimplemented.

- **The guard cannot use the naive rule "every `.ci` dir is a `registerCIRoot`
  suite."** That false-positives on 8 dirs walked by big runners as *subcommands*,
  not root commands: `encode`, `plugin`, `reload`, `decode`, `parse`, `chaos`,
  `chaos-web` (via `ze-test bgp <sub>`, see `cmd_bgp.go`) and `exabgp-compat` (via
  `ze-test exabgp`, `cmd_exabgp.go` `predecessorTestDir`). These have no queryable
  root name. The working guard is:
  `registry.HasRootHandler(dir)` [covers all 20 `registerCIRoot` suites because
  name==subdir, plus `vpp` registered via `registerRoot`] **OR**
  `coveredByBigRunner(dir)`. To avoid re-hardcoding (a review ISSUE), the big-runner
  set is DERIVED from the production single source of truth, not copied: the 7
  bgp-subcommand dirs live in a package-level `var bgpCIRunnerDirs` in `cmd_bgp.go`
  that the dispatch's own argument validation (`if !bgpCIRunnerDirs[command]`) and the
  guard both consume; `exabgp-compat` is referenced via the existing
  `predecessorTestDir` const in `cmd_exabgp.go`. The guard also fails on a STALE entry
  (a claimed dir with no `.ci`) to keep those sources honest. Because the allowlist IS
  the dispatch source, a bad-faith "just add it to the allowlist" would also change
  real dispatch behavior, closing most of the false-green surface.

## Core insight

An orphaned test suite is invisible because two independent failures cancel. The
durable fix is not to repair the specific files but to install a structural guard
(`TestCIRootsRegistered`) that makes "a `.ci` directory no runner walks" a hard,
immediate test failure — converting a whole class of future silent orphans into
loud ones. Deleting the stale files is the one-off cleanup; the guard is the reuse.

## Traps for next time

- `internal/test/cli` and `internal/test/runner` are shared-tree hot spots: multiple
  concurrent sessions edit `runner_exec*.go`, `record_parse.go`, and the plugin
  composition root, so the package build oscillates between compiling and broken.
  Verify with a scoped `-run` test the instant `go vet` is green; do not trust a
  single lint snapshot.
- Deletion of git-tracked `.ci` is approval-gated (`ai/rules/never-destroy-work.md`);
  the spec's own DELETION-NEEDS-APPROVAL gate requires asking, and a parent-agent
  task instruction is not the user's consent.

## Tests

- `TestCIRootsRegistered` (`internal/test/cli/register_test.go`) — AC-1: every
  `test/<dir>` with `.ci` is rooted. RED while `test/pppoe/` exists; GREEN on deletion.
- `TestParseOptionUnknownStillErrors` (`internal/test/runner/record_parse_test.go`) —
  AC-4: `parseOption`'s fail-closed default still errors on `option=netns`, driven
  from the `parseAndAdd` entry point.
