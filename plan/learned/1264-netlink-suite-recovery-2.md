# 1264 -- netlink-suite-recovery-2

## Context

Second recovery pass over the netlink functional suites (`plan/handover/21-netlink-suite-recovery.md`).
The first pass fixed the harness scaffolding and left five items: firewall 9 (per-element
nft timeout dropped), policy 5 (next-hop route unreachable in the per-test netns), six
hyphenated `ze.log.*` keys, the ospf/ospfv3 tail, and the ddos classifier. This pass, run on
macOS validating through `make ze-netns-qemu-test`, landed the fixable items and diagnosed the
rest to root cause. The goal was to turn "not diagnosed" into either a fix or a precise,
cited root cause a follow-up can act on without re-deriving it.

## Decisions

- **Per-element nft timeout: set `HasTimeout` on the parent set, not just the element.** The
  vendored `github.com/google/nftables` emits `NFTA_SET_ELEM_TIMEOUT` only when the parent
  `nftables.Set.HasTimeout` is true; `applySet` populated `el.Timeout` but never the flag, so
  every timeout was silently dropped. Extracted a pure `lowerSet` (over inlining in `applySet`)
  so the flag lowering is unit-testable off a real kernel.
- **Test interfaces belong in the netns, provisioned declaratively.** Added
  `option=netns-link:name=<if>[:address=<cidr>]`: the runner creates a dummy link inside the
  per-test namespace before `ze` launches. Chosen over teaching the daemon to fabricate
  interfaces (it must not) and over hardcoding interface names in the runner. Gated on netns
  mode so it is inert and host-safe on the default path.
- **The netns-qemu DUT daemon must build with `$(ZE_FEATURES)`, not a hand-picked tag subset.**
  A minimal tag list (`ze_ssh` only) broke the moment a test-only plugin's YANG imported a
  feature-gated module (`fakeddos/yang` imports `ze-ddos-detect-conf`, owned by `ze_ddos`):
  every config load failed "no such module". Aligned the target with its siblings and
  `internal/test/runner` `TestBuildTags` -- one derived source, no hand list.
- **A test that pins non-conformant behaviour is the bug, not the code.** `ospf-interface-runtime`
  asserted a loopback interface's ISM state is `down`; RFC 2328 sec 9.1 (and the code) say
  `loopback`. Corrected the test.
- **Config auto-semicolon is newline-only.** The tokenizer inserts `;` after a value only on a
  newline, so a compact inline block (`asn { local 65000 remote 65000 }`) is genuinely invalid;
  130 test files use the `;`/multiline form. Four ospf/ospfv3 files deviated -- fixed the configs,
  not the parser.

## Consequences

- `make ze-netns-qemu-test` now runs a policy subset (1-5) as well as firewall, and builds a
  correctly-featured DUT; it is the macOS-runnable regression guard for the netns launch mode.
  `009-set-element-timeout` stays excluded (crashes the Alpine kernel); the nft timeout path
  rests on the `lower_linux_test.go` unit test.
- `option=netns-link` is the vehicle for the remaining interface-missing ospf failures
  (50/58/68 + ospfv3 6/7): an **active** OSPF interface (nbma/ptmp/broadcast) needs a real link
  (`openConfiguredInterface`, `internal/plugins/ospf/instance.go`, sends non-passive/
  non-loopback through `openInterface`), so those tests must provision their interface and run
  under netns mode -- do not make OSPF tolerate a missing active link.
- Two ospf clusters are diagnosed but unfixed: the ldp-sync/multiaf 8 fail because the hub
  Orchestrator subsystem start path (`internal/component/plugin/server/subsystem.go`) never
  calls `SetAcceptor`, so an OSPF-only daemon (no `bgp` block) cannot start external plugins.
  Fixing it threads an acceptor through core startup -- its own change.

## Gotchas

- A green netns-qemu run that "passed" can be a daemon that failed EVERY config load: read the
  per-test error, not just the count. The mass firewall failure here was one build-tag bug, not
  22 regressions.
- The hyphenated `ze.log.*` fix is inert for pass/fail (rsvpte passes both ways) but real:
  `getLogEnv` splits on `.`, so `ze.log.ddos-detect` resolves nothing and the level stays WARN.
- One cause rarely explains a failing set: the ospf tail split into an RFC test bug, a config
  syntax bug, a missing-interface class, an acceptor-gap class, and an untraced instance-demux --
  five causes across 14 tests. Read each failure.

## Files

- `internal/plugins/firewall/nft/{lower_linux,backend_linux,lower_linux_test}.go`
- `internal/test/runner/{record,record_parse,netns_linux,netns_other,runner_exec}.go` + `netns_{linux,link}_test.go`
- `mk/test-integration.mk`, `scripts/evidence/netns_qemu.py`, `docs/architecture/testing/ci-format.md`
- `test/policy/005-next-hop.ci`, `test/ospf/{ospf-interface-runtime,ospf-nbma,ospf-ptmp}.ci`, `test/ospfv3/{ospfv3-nbma,ospfv3-ptmp}.ci`
- 14 `.ci` files: dotted `ze.log.*` keys (`test/plugin/ddos-*`, `test/rsvpte/*`, `test/firewall/ddos-local-withdraw`, `test/static/005-table-interface`)
