# 1249 -- feature-gate child 10: BGP compile-out (ze_bgp)

## Context

Ze could already drop the looking glass, ssh, web, gNMI, MCP, both API transports,
the Prometheus exporter, four routing protocols and VRRP at build time, but not
BGP -- the one subsystem whose absence a hardened appliance most wants, and by far
the largest attack surface. The blocker was structural, not conceptual: BGP-less
operation already worked at RUNTIME (a reactor-free `PluginCoordinator`, a live
no-`bgp{}` startup path), but 27 always-on files imported something under
`internal/component/bgp`, so the linker kept the whole subtree in every binary.
The goal was a `ze` built without `ze_bgp` that speaks only OSPF/IS-IS/static
routes and links zero BGP symbols, with the default build unchanged.

## Decisions

- **Gate the whole subtree, not just the speaker.** Codec and engine drop
  together, so the binary is genuinely BGP-free. The cost is un-fusing the codec
  from its always-on consumers, which is the bulk of the work below.
- **Three techniques instead of source tags, in preference order** -- transitive
  package drop, core-leaf move, IoC seam. Chosen over sprinkling `//go:build
  ze_bgp` on every file that touches BGP: tags would have spread across the hub,
  the config CLI and the schema CLI, and each one is a place a future edit can
  silently re-pin the subtree. Result: **2 hand-written tagged files**
  (`tree_bgp.go`, `dispatch_bgp.go`) plus the generated `all_ze_bgp.go`, and
  **zero** tagged files inside `internal/component/bgp` itself.
- **Three core leaves, chosen as leaves rather than whole packages.**
  `internal/core/bgp/routeaction` (the route-action/verb vocabulary sysrib and
  the three FIB backends share), `internal/core/bgp/msgtype` (the message-type
  codes MRT classifies by), `internal/core/bgp/ribevents` (the best-change
  contract sysrib and flow-export subscribe to). Moving the containing packages
  instead would have dragged `plugin/registry` into `internal/core` and broken
  the tier gate.
- **Symbols de-stuttered during the lift** (`bgptypes.RouteActionAdd` ->
  `routeaction.Add`) over keeping the old names behind an import alias. Every call
  site was being rewritten anyway, so the better API cost nothing extra, and the
  rewrite is compiler-verified end to end.
- **No compatibility aliases anywhere** (`ai/rules/no-layering.md`): the old
  `message.MessageType` and `bgptypes.RouteAction` were deleted, not re-exported,
  and all ~390 references rewritten.
- **`internal/component/config/infra` as the extractors' home**, not
  `internal/component/config` as the spec assumed -- see Gotchas.
- **`scripts/` added to dep_audit's non-production prefixes** alongside the
  planned `internal/perf/`. Both are build tooling reached only through their own
  binaries, exactly the rationale that already exempted `internal/chaos/` and
  `internal/test/`.

## Consequences

- A bare `ze_core` binary links **zero** `internal/component/bgp` symbols;
  dropping only `ze_bgp` from the full feature set takes the binary from 70M to
  60M. `ze-chaos` and `ze-perf` drive an in-process reactor, so the Makefile
  forces `ze_bgp` on for them; `ze-analyse` and `ze-test` are unaffected.
- `dep_audit --check` now actively asserts 59 gated packages have no always-on
  importer, including `bgp/{config,reactor,types,message,grmarker}` which are not
  in `all.go` at all. That is a standing ratchet, not a one-off nm measurement.
- Any tool that ENUMERATES a runtime registry must now run with the shipped
  feature tags. The Makefile has a `GO_RUN` for exactly those tools, and the
  docvalid tests derive the tag set from `feature-gates.txt`. This was already
  latently wrong for isis/ospf/vrrp; gating BGP is what made it visible, because
  BGP owns most of the family and command registry.
- `cmd/ze/hub` tests run twice (full tags and bare `ze_core`), so any hub test
  whose fixture uses a gated config root now fails in the second pass. Two did;
  both now use an always-on `interface{}` fixture and are protocol-independent as
  a result.

## Gotchas

- **The spec's A-2 was wrong about the destination.** The `bgp/config` extractors
  need `authz` and `aaa`, and BOTH already import `internal/component/config` --
  so putting them there is an import cycle. They live in a CHILD package,
  `internal/component/config/infra`, which `config` never imports. Check the
  direction of the dependency you are joining, not just whether the code "looks
  like config".
- **Removing an always-on import can unlink an `init()` nobody else pulls in.**
  `bgp/config` registers the reactor factory `bgp/plugin` calls at OnConfigure,
  and it was linked ONLY as a side effect of the hub importing it for
  `ExtractSSHConfig`. Deleting that import would have shipped a daemon where every
  `bgp{}` config failed with `errBgpNoReactorFactoryRegistered`. The obvious fix
  (blank-import it from `bgp/plugin`) is an import cycle in test, because
  `bgp/config`'s own tests import `plugin/all` which imports `bgp/plugin`. It is
  linked from `cmd/ze/dispatch_bgp.go` instead: a `package main` root can never be
  imported back, so that edge is always safe. **After deleting an always-on
  import, ask what that package's `init()` was providing.**
- **A feature-gated file is still an always-on pin for a DIFFERENT gate.**
  `cmd/ze/hub/service_ssh.go` (`//go:build ze_ssh`) imported `bgp/config`.
  `dep_audit.file_requires_tag` is per-tag and flags it -- correctly, since a
  `ze_ssh`-on / `ze_bgp`-off build genuinely fails to compile. The spec's
  hand-built inventory missed all three such files; rebuilding it from
  `dep_audit.collect_edges` found 27 where the spec listed about 15.
- **An absent-config test can pass for the wrong reason.** The first
  `TestBuildTag_BGP_AbsentRejectsBGPConfig` probe used leaf names that do not
  exist, so it "proved" the schema was gone by rejecting invalid syntax. Pair
  every absent-rejection test with a present-build test asserting the SAME literal
  parses, and keep the literal in one untagged file so the halves cannot drift.
- **A nil seam needs a correct no-feature behavior, not just a nil check.**
  `ze config validate` fails CLOSED when it meets a `bgp{}` block with no resolver
  registered, because that combination means the schema and the seam have drifted
  -- silently validating nothing would be worse than the error.
- **Mechanical renames inside RFC-tagged tests trip the audit-freshness gate.**
  The `msgtype`/`routeaction` rewrite touched 18 rfc7606-tagged files and staled 8
  audit verdicts. The rename was proved behavior-neutral by normalizing the diff
  under the renaming and confirming the add/delete multisets cancel; the verdicts
  were then re-stamped with fresh fingerprints and that reasoning recorded in
  `reaudit_note`. Note the hook that would normally block such an edit never
  fired: it is a Write/Edit hook, and the rename was applied by a script.
- **Two pre-existing reds surfaced and were fixed rather than parked**
  (`ai/rules/no-parking.md`): `session-end-scratch.sh` called `_sid_safe`, which
  the session-id shim refactor had deleted, so session scratch dirs were never
  cleaned; and the audit-relaxation fixture symlinked the hook but not the `lib/`
  it imports at module scope. Both blocked `ze-verify`, so both were in scope.
- **Two tools read `git ls-files` but parse from disk**, so any uncommitted test
  deletion broke them before that deletion could be committed. Both now skip an
  index entry with no file on disk -- narrowly, for not-exist only.
- **Ask which binaries HOST a test surface, not just which ones RUN the feature.**
  The spec's A-4 cleared `ze-test` because it does not run a reactor. But the
  `.et` editor suite runs the whole TUI headless INSIDE the `ze-test` binary, so
  the config schema it edits is whatever THAT binary linked --
  `internal/test/runner.TestBuildTags` derives the manifest tags for the DUT `ze`
  it builds, not for itself. Built with `-tags ze_test` alone it lost the bgp
  schema and 100+ `.et` cases failed with `unknown path: bgp`. Every `ze-test`
  build line now carries `$(ZE_FEATURES)`, which also covers every future gate.
- **Attribute a functional red against a real baseline, never by inference.**
  The plugin suite failed 6 tests under its own parallelism that all passed
  alone. Rather than calling them flakes, a clean HEAD baseline was extracted
  (`git archive HEAD` into `tmp/`, `ze-test` built by that tree's own Makefile)
  and the same invocations run A/B: baseline failed `[97, 224, 398, 458]` on the
  full suite and 3/3 on the isolated `222 458` pair, versus the working tree's
  `[85, 97, 222, 224, 398, 458]` and 2/3. Same cluster, same symptom
  (`connection closed before completion`), working tree no worse -- logged in
  `plan/known-failures/bgp-plugin-dest-peer-teardown-cluster.md` with that
  evidence rather than an assertion.

## Files

- `feature-gates.txt` (59 `ze_bgp` lines), `internal/component/plugin/all/all_ze_bgp.go` (generated)
- `internal/core/bgp/{routeaction,msgtype,ribevents}/`, `internal/core/rib/igpcost/`
- `internal/component/config/infra/{hook,ssh,authz,bgp}.go`
- `cmd/ze/dispatch_bgp.go`, `internal/component/config/yang/cli/tree_bgp.go`
- `internal/component/plugin/registry/interfaces.go` (RIB-dump + packet-decoder seams)
- `cmd/ze/hub/{infra_setup,ssh_infra,service_web,main}.go`, `internal/component/config/cli/cmd_{dump,diff,validate}.go`
- `Makefile` (`GO_RUN`, ze-chaos/ze-perf tags), `scripts/dev/dep_audit.py`, `.github/workflows/codeql.yml`
- `cmd/ze/hub/build_tag_bgp_{present,absent,probe}_test.go`, `build_tag_protocols_absent_test.go`
