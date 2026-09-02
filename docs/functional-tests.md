# Ze Functional Test System

## Overview

Functional tests exercise release-gate behavior across BGP wire encoding and decoding, plugin behavior, config parsing, reloads, UI/editor flows, managed config, L2TP, firewall, policy routing, LDP, RSVP-TE, IS-IS, OSPF, OSPFv3, web UI, and install flows.

> For how the runner schedules and executes tests (the three execution engines, concurrency, reporting) and the web `.wb` test format, see [`architecture/testing/runner-architecture.md`](architecture/testing/runner-architecture.md).

```bash
# Quick start
./le functional gating             # Run all release-gate suites
./le functional encode             # Encoding tests only
./le functional plugin             # Plugin tests only
./le functional reload             # Reload tests only
```

## Release Gate Coverage

The stage list is **not reproduced here**. It lives in `stagesForMode`
(`internal/le/verify/engine/stages.go`) and nowhere else. Each shard of
`.github/workflows/verify.yml` reads that same list through
`./le verify current mode full-list`, so a gate absent from the Go table runs
nowhere and a gate added to it needs no duplicate list in this page. Run
`./le verify current mode full-list` to read the current full-mode list.

An earlier version of this paragraph enumerated the stages and went eight
stages out of date. Keep the producer in Go and the prose at this level.

Broadly, `./le verify current mode full` runs the static gates first, then
`./le verify deps vulnerability` before unit, functional, and ExaBGP stages.
`./le verify current mode changed` substitutes changed-only lint and unit
stages. Both modes run the same vulnerability scan. It needs network access to
the live Go vulnerability database. `TestStagesForModeBranchesAgree` and
`TestStagesForModeMatchesGolden` in `internal/le/verify/engine` pin the two lists.

Both modes use the shared admission code in `internal/le/job` and
`internal/le/verify/lock`. A verify run continues across top-level stage
failures and writes:

Each run writes its artifacts into its OWN directory, `tmp/verify/run-<start>-<mode>-<id>/`,
so two sessions that verify at the same time never overwrite each other's log or
failure index. The `run-dir` key in the failure index names the directory. At
most ten run directories are kept: the newest ones, plus every directory a
documented path below still points into. A run a published path points into is
never removed for age, because only a full run writes `tmp/ze-verify-full.json`
and ten cheaper `./le verify current mode changed` runs can follow it in one day.
The documented paths below stay
where they are: each is a symlink into the directory of the run that owns it.
The combined log is published when the run starts, so a waiting session can
follow it; the failure artifacts are published when the run ends.

| Artifact | Purpose |
|----------|---------|
| `tmp/verify/run-<start>-<mode>-<id>/` | One run's own artifacts: the combined log, the stage logs, and the failure index |
| `tmp/ze-verify.log` | Combined log of the newest run |
| `tmp/verify/run-<start>-<mode>-<id>/<nn>-<stage>.log` | Full log for one stage of that run. The failure index names it in `detail-log` |
| `tmp/ze-verify-failures.log` | Compact failure index to read first |
| `tmp/ze-verify-failures.json` | Machine-readable failure routing index |
| `tmp/ze-verify-full.json` | The same index, written by the FULL mode only. It is the record a commit carrying Go is gated on, and it is separate because a `./le verify current mode changed` run in another session republishes the shared path |
| `tmp/ze-verify.status` | Freshness fingerprint for the last run |
| `tmp/ze-verify-manifest.txt` | One fingerprint per path that differs from `HEAD`, so a session can ask about its own file list rather than the whole tree |

Several sessions share this checkout, so a whole-tree answer is STALE almost
always, and almost always for somebody else's file. What the scoped answer
covers, and the four conditions it never widens, is
[`docs/architecture/testing/verify-freshness-scope.md`](architecture/testing/verify-freshness-scope.md).

A scoped run also judges fewer Staticcheck matrix rows.
`./le staticcheck-feature-matrix check` derives one row per feature tag in
`feature-gates.txt` plus `all_features` and `core_only`, 38 rows today, and each
row is a full-module analysis. `scopeFeatureMatrix`
(`internal/le/staticcheckfeaturematrix.Answer`) keeps a row when it omits no tag
or omits a tag the change reached, so a change local to `ze_ssh` judges 3 rows
rather than 38. `all_features` and `core_only` judge the combinations Ze ships
and are never subtracted. Running the target on its own, with
`ZE_VERIFY_SCOPE_TAGS` unset, judges every row. The rules, the four inputs that
widen the scope back to 38, and the measured cost are
[`architecture/testing/verify-freshness-scope.md`](architecture/testing/verify-freshness-scope.md).

Suite selection is NOT scoped: every functional suite runs on every verify,
whatever the change set says. `go list -deps ./cmd/ze` links 562 of the module's
646 packages, so no static signal attributes a `.ci` file to a package.
`plan/spec-verify-scope-5-suite-coverage-map.md` derives that map by observing
what a suite executes.

The functional test target runs 24 suites: encode, plugin, parse, decode, reload,
ui, editor, managed, l2tp, firewall, policy, ipsec, ldp, rsvpte, isis, ospf, ospfv3,
web, install, appliance, l2tp-wire, isis-wire, ospf-wire, runner.

`./le repository check` is a fast (~0.2s) post-verify check that catches recurring
implementation mistakes: stale source anchors, line-number anchors, unwired
exported symbols, and incomplete spec AC tables. Run it after `./le verify current mode full` passes,
before presenting work as complete.
<!-- source: internal/le/verify/engine/stages.go -- StagesForMode -->
<!-- source: internal/le/verify/deps/actions.go -- Actions -->
<!-- source: internal/le/repository/actions.go -- Answer -->
<!-- source: internal/le/verify/engine/run.go -- Run, RunMode -->
<!-- source: internal/le/job/answer.go -- Answer -->
<!-- source: internal/le/verify/status/answer.go -- Answer -->
<!-- source: internal/le/staticcheckfeaturematrix/actions.go -- Answer -->
<!-- source: internal/le/functional/actions.go -- Answer -->

The following shipped test suites are **not in the default release gate** and
must be run manually:

| Suite | Runner | Why not gated |
|-------|--------|---------------|
| Static routes | `./le functional static` | Separate route-installation fixture |
| Traffic control | `./le functional traffic` | Requires traffic-control platform support |
| Flow export | `./le functional flow-export` | Requires Linux packet-sampling support for release evidence |
| VPP | `./le functional vpp` | Uses the Go VPP stub in `internal/test/cli/cmd_vpp_stub.go` |
| L2TP wire | `./le functional l2tp-wire` | Wire-level fixture separate from release-gate L2TP daemon scenarios |
| OSPFv2 wire | `./le functional ospf-wire` | Wire-level codec fixture separate from release-gate OSPF runtime scenarios |
| IS-IS wire | `./le functional isis-wire` | Wire-level codec fixture separate from release-gate IS-IS runtime scenarios |
| BGP interop | `./le integration interop` (`INTEROP_SCENARIO=<name>` for one) | Requires Docker peer daemons and image builds. The runner fails when Docker is unavailable |
| IPsec interop | `IPSEC_INTEROP_SCENARIO=<name> ./le integration interop-ipsec` | Requires strongSwan in privileged Docker containers |
| Chaos web | `./le functional web` | Browser and dashboard scenarios |

Every suite action routes through `internal/le/job`, which runs it now,
queues it behind a heavy job already in flight, or attaches it to an equivalent
run. Queue a runner selection with
`./le job run label <label> command bin/ze-test <suite> <selection>`.
<!-- source: internal/le/functional/actions.go -- Actions -->
<!-- source: internal/le/integration/actions.go -- Actions -->

Clean release-candidate evidence can be run with `./le evidence release-candidate`.
The action refuses a dirty worktree, clones the repository into an ephemeral
Docker container, mirrors the CI dependency setup, and runs
`./le verify current mode full` there.

`ze-deployment-*` targets are deployment-grade external evidence. Some use
Docker because the local runner needs a Linux system service; Docker-specific
targets say so in the target name.

Real VPP daemon evidence is separate from the stub-backed VPP functional suite:
`./le deployment vpp-test` starts VPP in a privileged Docker container and
proves `fib-vpp` add and withdraw against VPP's real FIB, traffic policers,
and the IKE IPsec dataplane backend.

External L2TP peer evidence is separate from the in-tree L2TP fixture suite:
`./le deployment l2tp-test` starts Ze and a real `xl2tpd` LAC in a
privileged Docker container, then proves the control tunnel and incoming-call
session are established. Full PPP/NCP/kernel dataplane peer evidence is a
separate Linux-only target: `./le deployment l2tp-ppp-test` creates Ze and
LAC network namespaces joined by a veth underlay, then requires `xl2tpd`,
`pppd`, `ping`, `/dev/ppp`, `iproute2`, and PPPoL2TP kernel support.
`./le deployment docker-l2tp-ppp-test` runs a peer-isolated Docker lab with Ze,
a real `xl2tpd`/`pppd` peer, FRR for BGP redistribution, and an independent Go
RADIUS peer. Its four scenarios cover Ze as LNS and initiator, PPP LCP/IPCP,
kernel `pppN` interfaces, dataplane traffic, route redistribution, and RADIUS
accounting attributes. The lab refuses to run a PPP scenario when `/dev/ppp`,
`ip l2tp`, or the `l2tp_ppp`/`pppol2tp` module is missing. A single scenario is
selected with `ZE_L2TP_INTEROP_SCENARIO=<name>`.

`./le deployment docker-pppoe-accel-test` runs two PPPoE roles against independent
peers. Ze acts as a client against the Alpine
[accel-ppp](https://accel-ppp.org/) access concentrator, then as an access
concentrator against `pppd` with the `rp-pppoe` plugin. The checkers require
PADI/PADO/PADR/PADS discovery, bidirectional LCP, CHAP-MD5 acceptance and
refusal, IPCP address assignment, kernel `pppN` state, dataplane traffic, and
clean teardown. The Docker form needs host-kernel `/dev/ppp` and `pppoe`
support. `ZE_PPPOE_INTEROP_SCENARIO=<name>` selects one role. See
`docs/labs/pppoe-interop.md`.

`./le qemu vrrp-keepalived-test` runs the VRRP interop lab against keepalived
2.3.1 on one L2 segment. Three network namespaces are bridged so a passive
observer can prove the virtual IP moves at layer 2. The action boots Ze's
runtime kernel through the native QEMU harness and uses the shared verified
kernel cache.
Keepalived state is read from its `notify_*` markers and every timing assertion
is measured from tcpdump wire timestamps against the acceptance bands in
`plan/spec-vrrp-6-interop.md`, never from wall clock.

The Linux-only VRRP runtime `.ci` tests carry `option=needs-linux`: they skip on
Darwin and unprivileged Linux, then run inside `./le qemu all-tests`.

### Which pipeline runs each QEMU lab

`.github/workflows/qemu-nightly.yml` invokes the registered `./le qemu`
actions. `./le qemu all-tests` covers the Linux functional suites;
`vrrp-keepalived-test`, `pppoe-accel-test`, and `pppoe-test` own their protocol
labs. `TestEveryWorkflowNativeActionExists` checks the other direction: every
`./le` action a workflow names is registered in the Go action tables. Nothing
today refuses a REGISTERED action that no workflow reaches, so a new lab needs
its workflow job added by hand. The full workflow map is
`docs/architecture/testing/ci-workflows.md`.

<!-- source: .github/workflows/qemu-nightly.yml -- protocol-labs, runtime-kernel-labs -->
<!-- source: internal/le/workflowcheck/workflowcheck_test.go -- TestEveryWorkflowNativeActionExists -->

<!-- source: internal/test/cli/register.go -- subcommand registry -->
<!-- source: internal/test/cli/cmd_bgp.go -- chaos-web suite -->
<!-- source: internal/le/evidence/actions.go -- Actions -->
<!-- source: internal/le/qemu/actions.go -- Actions -->
<!-- source: internal/le/qemu/alltests.go -- allTestsRun -->

---

## Quick Start

```bash
# List available tests with run number, total, one-based id, and name
ze-test bgp encode --list
ze-test ui --list

# Run specific tests by id or exact name
ze-test bgp encode 4 5 6
ze-test ui 4

# Run all tests
ze-test bgp encode --all
ze-test ui --all

# Resume a suite after a timeout or interrupted run
ze-test bgp plugin --start 42
ze-test editor --start 42

# Stress test selected tests
ze-test bgp encode --count 10 1 2

# Run tests under development from test/draft/<suite> instead of test/<suite>
ze-test bgp plugin --draft --all
```
<!-- source: internal/test/cli/cmd_bgp.go -- zeTestParseRunCLI, zeTestPrintRunUsage -->
<!-- source: internal/test/cli/ci_runner.go -- RunCISubcommand common options -->
<!-- source: internal/test/cli/cmd_editor.go -- cmdEditorMain common options -->

---
## Writing a Test: Draft First

**Never write or iterate on a `.ci` inside `test/<suite>/`.** That directory is
live: every `./le verify current mode full` in the checkout runs it, including runs by other
sessions working on unrelated things, who then have to work out whether your
half-written test is their regression. The same applies to CHANGING an existing
test.

Drafts live in `test/draft/<suite>/`, which is gitignored and skipped by every
repo-wide gate:

```bash
$EDITOR test/draft/plugin/my-test.ci        # 1. write
ze-test bgp plugin --draft -a               # 2. run only drafts
./le stress-repro run suite "bgp plugin --draft" test 1 any-failure
mv test/draft/plugin/my-test.ci test/plugin/  # 3. promote when green
ze-test bgp plugin -a                       # 4. now it is real
```

`--draft` swaps the discovery root; without it you always get the real tests.
Suite discovery is a non-recursive glob, so the incubator is invisible to it for
free; the six gates that walk `test/` recursively each skip it explicitly, and
`TestDraftDirIsInvisibleToRepoGates` fails if one of them stops. Adding a new
repo-wide `.ci` scanner means skipping `test/draft/` in it and adding a row to
that test.

Nothing in the incubator is gated, so promote early rather than polishing for
days against no accept-only check, no sleep ratchet, and no frame-length
validation. Full contract and workflow: `test/draft/README.md`, or the
`/ze-test` skill.
<!-- source: internal/test/runner/draft_dir.go -- SuiteDir, isDraftPath, DraftDirName -->
<!-- source: internal/test/runner/draft_dir_test.go -- TestDraftDirIsInvisibleToRepoGates ratchet -->

### Changing a live test that already passes

An edit to a `.ci` or `.et` under `test/` that removes an `expect=`, empties a
needle, or inverts a `reject=` is a weakening.
`internal/le/testweakened` detects it, and a whole `.ci` file is one test, named by
its file stem.

Write the row in `test/weakened.md` BEFORE the edit, then make the edit, then
name `test/weakened.md` in the commit. The file is replaced per commit and never
accumulates. The full route, and what a reason has to say, is in
`docs/contributing/testing.md`, "When a test must be weakened".

An RFC-tagged test is stricter still: any behavior change to it needs the user's
own approval, and no row in `test/weakened.md` satisfies that. See
"RFC Requirement Tags" below.

---
## Functional Suite Inventory

Every `ze-test` functional suite uses the same selection contract after the suite
name: `--list`, `--all`, `--start ID`, `--pattern TEXT`, or positional
`ID_OR_NAME...`. `--list` prints each test as `N/TOTAL ID NAME`; `ID` is
one-based, so the full-suite progress number and runnable id match. Each run
prints one completion line per test with elapsed time, `N/TOTAL`, result, id,
and name, plus periodic progress while tests are still running.
<!-- source: internal/test/runner/selection.go -- Selection -->
<!-- source: internal/test/runner/display.go -- Status, TestFinished -->
<!-- source: internal/test/cli/cmd_web.go -- webMain parallel via ParallelRunner -->

| Suite | Command | Files | How it works |
|-------|---------|-------|--------------|
| Encode | `ze-test bgp encode` | `test/encode/*.ci` | Builds Ze and `ze-peer`, starts peers, then checks BGP wire output from configured routes. |
| Plugin | `ze-test bgp plugin` | `test/plugin/*.ci` | Runs Ze with embedded process/API fixtures, injects commands or plugin events, then checks BGP, stdout/stderr, syslog, HTTP, or file expectations. |
| Parse | `ze-test bgp parse` | `test/parse/*.ci` | Runs foreground config validation commands and checks exit code plus stdout/stderr expectations. |
| Decode | `ze-test bgp decode` | `test/decode/*.{ci,test}` | Feeds BGP message bytes to decode commands and compares JSON output with volatile fields normalized. |
| Reload | `ze-test bgp reload` | `test/reload/*.ci` | Starts Ze, rewrites config, sends SIGHUP, then checks post-reload behavior. |
| UI | `ze-test ui` | `test/ui/*.ci` | Runs foreground CLI commands and checks terminal output and exit status. |
| Editor | `ze-test editor` | `test/editor/**/*.et` | Runs headless editor keystroke scripts through the CLI testing harness. |
| Managed | `ze-test managed` | `test/managed/*.ci` | Exercises managed config, hub, auth, and fleet workflows through `.ci` process tests. |
| L2TP | `ze-test l2tp` | `test/l2tp/*.ci` | Runs L2TP control-plane scenarios over loopback UDP with fake test plugins where needed. |
| Firewall | `ze-test firewall` | `test/firewall/*.ci` | Exercises firewall configuration and daemon behavior through `.ci` process tests. |
| Policy | `ze-test policy` | `test/policy/*.ci` | Exercises policy-routing configuration and daemon behavior through `.ci` process tests. |
| IPsec | `ze-test ipsec` | `test/ipsec/*.ci` | Exercises the `vpn ipsec` show, monitor, and clear surfaces and the IKEv2 SA lifecycle through `.ci` process tests. `ipsec-eap-nak-unacceptable-type` reads the daemon log after an authenticator offered an EAP method the peer does not run, so an operator sees the type Ze asked for instead of a dead SA. `ipsec-eap-md5-challenge` runs `authentication { mode eap-md5 }` on both daemons, so an operator reaches EAP MD5-Challenge and the tunnel establishes. Its AUTH payloads come from SK_pi and SK_pr, which RFC 7296 Section 2.16 prescribes for a method that establishes no shared key. The tests that read the kernel XFRM tables declare `option=needs-linux:caps=net-admin`, run under QEMU, and share the `ipsec-xfrm` exclusive group described below. |
| Web | `ze-test web` | `test/web/*.wb` | Runs `.wb` browser scripts in parallel (cap 4) with a per-test server and an isolated `agent-browser --session`. `option=server:kind=` picks which of Ze three htmx interfaces the test drives: the web UI (default), the looking glass (a daemon plus a `ze-test peer` sink, so its pages have a session to report), the looking glass with a failing engine (`lg-no-engine`, served by `ze-test lg`), or the chaos dashboard (`ze-chaos`). |
| Install | `ze-test install` | `test/install/*.ci` | Exercises offline install command and installer helper behavior. |
| Static | `ze-test static` | `test/static/*.ci` | Exercises static route installation and reload add/remove behavior. |
| Traffic | `ze-test traffic` | `test/traffic/*.ci` | Exercises traffic-control configuration and daemon behavior. |
| Flow export | `ze-test flow-export` | `test/flow-export/*.ci` | Exercises sFlow, NetFlow, and IPFIX export behavior. |
| VRRP | `ze-test vrrp` | `test/vrrp/*.ci` | Exercises the vrrp YANG augment under interface units, the plugin's cross-leaf verifier (mandatory vrid and its 1..255 range, duplicate vrid or virtual-address per unit+family, the operator-assigned priority 255 rejection, the version-dependent interval encodings, accept-mode as VRRPv3-only, the IPv6 first-address link-local rule, and the VPP backend rejection), and the doctor/explain surface. Each command asserts its own exit code via `cmd=...:exit=N`, because `expect=exit:code=` only ever reaches a file's last quick-exit `ze` command. Tests that boot a daemon carry `option=needs-linux` and run only under QEMU. |
| VPP | `ze-test vpp` | `test/vpp/*.ci` | Runs stub-backed VPP scenarios and checks the stub request log. |
| L2TP wire | `ze-test l2tp-wire` | `test/l2tp-wire/*.ci` | Exercises L2TP wire-level encode/decode and malformed-packet handling. |
| IS-IS wire | `ze-test isis-wire` | `test/isis-wire/*.ci` | Exercises IS-IS wire-level decode and malformed-PDU handling. |
| OSPFv2 wire | `ze-test ospf-wire` | `test/ospf-wire/*.ci` | Exercises OSPFv2 packet/LSA wire-level decode and malformed-packet handling. |
| OSPF | `ze-test ospf` | `test/ospf/*.ci` | Exercises release-gate OSPF config validation, interface ISM config leaves including passive and loopback records, NSM config leaves including `mtu-ignore`, LSDB flooding/retransmit/purge logic, SPF route installation via Loc-RIB/sysrib ECMP membership updates, inter-area ABR Type 3/4 summary origination, area ranges, summary withdraw, border-router snapshots, daemon route snapshot wiring, admin-distance arbitration, and raw-socket doctor diagnostics, plus the RFC 5250 opaque carrier, RFC 3630/5392 Traffic Engineering, the RFC 7770 Router Information LSA (`ospf-ri-*.ci`, and `test/ospfv3/ospfv3-ri-originate.ci` for the v3 engine), and the RFC 7684 Extended Prefix/Link Opaque LSAs (`ospf-ext-register.ci`, `ospf-ext-prefix-originate.ci`, `ospf-ext-link-originate.ci`, `ospf-ext-prefix-receive.ci`, `ospf-ext-subtlv-hook.ci`, `ospf-ext-decode.ci`), with FRR interop scenarios (`ospf-ri-frr`, `ospfv3-ri-frr`, `ospf-ext-prefix-link-frr`) run under QEMU. |
| Chaos | `ze-test bgp chaos` | `test/chaos/*.ci` | Runs Ze plus chaos peers end-to-end through the BGP `.ci` runner. |
| Chaos web | `ze-test bgp chaos-web` | `test/chaos-web/*.ci` | Runs chaos dashboard HTTP endpoint checks through the BGP `.ci` runner. |
| ExaBGP compatibility | `ze-test exabgp` | `test/exabgp-compat/encoding/*.ci` | Runs the ExaBGP compatibility fixtures through the Go `ze-test` runner, starts the mock BGP peer, runs the ExaBGP wrapper client, and checks the expected wire output. |
| Runner | `ze-test runner` | `test/runner/*.ci` | Exercises the `.ci` orchestration grammar and the native verify-freshness, structural-red attribution, verification-debt, and change-set-selection fixtures. The compiled Go fixtures use throwaway Git repositories and need no interpreter. |
<!-- source: internal/test/cli/cmd_bgp.go -- BGP suite routing -->
<!-- source: internal/test/cli/ci_runner.go -- shared .ci suites -->
<!-- source: internal/test/cli/cmd_editor.go -- .et suite runner -->
<!-- source: internal/test/cli/cmd_web.go -- .wb suite runner -->
<!-- source: internal/test/cli/cmd_vpp.go -- VPP stub-backed suite runner -->
<!-- source: internal/test/cli/cmd_exabgp.go -- ExaBGP compatibility runner -->

### Per-suite wall-clock budget

Each suite runs under `timeout`, so a stuck subprocess cannot hold the run open.
`timeout` puts the suite in its own process group and signals the whole group,
which kills leaked `ze` daemons and mock servers with it.

| Variable | Default | What it sets |
|----------|---------|--------------|
| `ZE_SUITE_TIMEOUT` | `600s` | The budget a suite gets when it has no budget of its own. `timeout` accepts a bare number of seconds or an `s`, `m`, `h`, or `d` suffix |
| `ZE_SUITE_TIMEOUT_PLUGIN` | `1500s` | The `plugin` suite's own budget |
| `ZE_SUITE_KILL_AFTER` | `10s` | How long after SIGTERM the group gets SIGKILL |
| `ZE_SUITE_WARN_PERCENT` | `80` | The percentage of the budget that makes a green suite print a warning |

Override any of them on the command line:

```bash
ZE_SUITE_TIMEOUT=1200s ./le functional gating
ZE_SUITE_TIMEOUT_PLUGIN=1800s ./le functional plugin
```

#### One suite's budget is its own

`ZE_SUITE_TIMEOUT` protects the other 23 suites, so a slow suite must not raise
it for all of them. A suite that needs more wall clock gets a
`ZE_SUITE_TIMEOUT_<SUITE>` of its own instead, and `Suite.budget` answers with
that budget everywhere: the `timeout` that kills the suite, the runtime line,
the warning arithmetic, and the variable name the reports tell you to raise.

The `plugin` suite is the one suite that has such a budget today. It holds 663
`.ci` tests, and it measured 855s on 2026-08-19 against the 600s shared cap that
had been killing it. The budget is derived from that measurement: the warning
point (80% of the budget) must sit 40% above 855s, or a busy box warns on every
run and the warning names no creep. That gives 855 * 1.40 / 0.80 = 1496s,
rounded up to the whole minute. The kill then lands at 1.75x the measurement,
which is a wedged suite and not a busy box.

Adding a suite to that family is one line: a `BUDGET_DEFAULTS` entry in
`internal/le/functional.Answer`. Setting `ZE_SUITE_TIMEOUT_<SUITE>` in
the environment gives any suite one with no edit at all, because the variable
name is derived from the suite's name.

The retired orchestration required four edits that a test had to hold in step,
since the budget could be spelled in one place and not another: the run would
then say 1500s while the kill landed at 600s.

`./le functional gating` prints one runtime line per suite, and a table of all
of them at the end:

```
      suite plugin took 855s of its 1500s budget (57%)
      suite encode took 431s of its 600s budget (71%)

──── suite runtimes (default budget 600s, warning level 80%) ────
  plugin 855s of 1500s (57%)
  encode 431s of 600s (71%)
```

Two conditions get their own line. A suite that uses `ZE_SUITE_WARN_PERCENT` of
its budget prints `BUDGET WARNING` and stays green. A suite that reaches its
budget is killed, `timeout` returns 124, and the run prints `BUDGET EXPIRED`
with the suite name, the budget, and the variable that owns it:

```
BUDGET EXPIRED  suite plugin reached its 1500s wall-clock budget (ZE_SUITE_TIMEOUT_PLUGIN) and was killed. The test failures above are that kill, not the product.
```

Read that line before you read the test failures above it. A killed suite
reports the tests that were still running as failures, and those failures say
nothing about the product. The same expiry lands in
`tmp/ze-verify-failures.json` as a group of kind `timeout`.

A budget that is raised and never watched creeps back to its cap, so the
warning exists to make the creep visible while the suite is still green.
<!-- source: internal/le/functional/actions.go -- Answer -->
<!-- source: internal/le/functional/actions.go -- Answer -->

---


## Unit Tests

Unit tests are standard Go `_test.go` files throughout the codebase. Running
all ~349 packages with `-race` takes ~5 minutes. During development, use
component-group targets to test only the area you changed:

| Command | Scope | Approximate time |
|---------|-------|------------------|
| `./le test-unit bgp` | `./internal/component/bgp/...` | about 1:30 |
| `./le test-unit core` | `./internal/core/...` | about 30 seconds |
| `./le test-unit plugins` | `./internal/plugins/...` | about 40 seconds |
| `./le test-unit config` | `./internal/component/config/...` | about 20 seconds |
| `./le test-unit cli` | `./internal/component/cli/...` | about 10 seconds |
| `./le test-unit installer` | `./internal/install/...` behind the `ze_installer` tag | about 10 seconds |
| `./le verify current mode full` | Every unit group plus repository gates | about 5 minutes |

The first five groups run with the test-only `CGO_ENABLED=1 go test -race` path
on Linux and Darwin. These race-built test executables never ship or serve as
release/build evidence.

The `installer` group carries no `-race` flag and targets Linux. On another
host `go test` cross-compiles a Linux binary it cannot start, so the group runs
`go vet` over the same files. `./le qemu all-tests` runs them for real inside
the Alpine virtual machine.

### Property-based tests (stdlib `testing/quick`)

Invariant checks over generated inputs, using the standard library's
`testing/quick` (no third-party property engine). Each uses a fixed RNG seed and
a bounded iteration count so CI runs are deterministic and cannot time out. They
run in the ordinary unit passes (no build tag):

| Test | Package | Invariant |
|------|---------|-----------|
| `TestListenerConflictProperties` | `internal/component/config` | listener-conflict symmetry, port/family/protocol independence, wildcard dominance, `FindListenerConflict` ↔ pairwise equivalence (conflict is provably NOT transitive; `TestListenerConflictNotTransitive` pins the wildcard counterexample). |
| `TestMigrationRoundTripProperty` | `internal/exabgp/migration` | any valid ExaBGP config → migrate → serialize re-parses as valid Ze config with every neighbor preserved. |
| `TestForwardPoolOrderingProperty` | `internal/component/bgp/reactor` | batch order preservation, supersede-key determinism, malformed-body robustness, and exactly-once delivery under concurrent dispatch (`-race`). |
| `TestFilterChainRandomUpdatesProperty` | `internal/component/bgp/reactor` | `buildModifiedPayload` never panics and only emits well-formed UPDATE bodies over random payloads+ops; `LessOrder` is a strict total order. |

### Evidence / release-tier stress and perf tests (build-tagged, out of `./le verify current mode full`)

Resource-heavy, host-dependent tests stay out of the pre-commit gate. Their
native integration actions supply the required build tags:

| Command | Test | Tag | What it drives |
|---------|------|-----|----------------|
| `./le integration stress-web` | `TestWebConcurrentEditStress` | `stress` | Concurrent editor sessions mutate and commit one config under the race detector |
| `./le integration stress-fleet` | `TestFleetManyClientsPerf` | `fleetperf` | 128 managed clients authenticate and synchronize against the real listener |
<!-- source: internal/le/integration/gates.go -- Table -->

The BGP route stress harness is a native integration action. It creates two
network namespaces, starts Ze or BIRD, and drives the exact five-scenario
registry with `ze-test peer --mode inject`. It needs root, `ip`, `ethtool`,
`tcpdump`, and BIRD for the baseline scenario.

```bash
./le integration stress
STRESS_SCENARIO=03-session-flap ./le integration stress
STRESS_SCENARIO=05-profile-1m ZE_PPROF=1 ./le integration stress
```

The registry covers four bulk route counts from 100,000 to 1,000,000, sequential
IPv4 and IPv6 peers, ten flap cycles followed by a final injection, the BIRD
baseline, and the 1,000,000-route CPU, heap, and goroutine profile run.
<!-- source: internal/le/integration/stress.go -- native scenario registry and runner -->
<!-- source: internal/le/integration/stressbird.go -- BIRD baseline runner -->

### netlab template render check (`./le netlab render-check`, out of `./le verify current mode full`)

`contrib/netlab/` mirrors the netlab daemon integration: the daemon definition, the
Jinja2 templates that emit ze configuration, one reference topology, and the committed
render under `contrib/netlab/golden/`. The check builds a scratch lab from that mirror
and runs `netlab create`. It then compares each rendered node configuration against its
golden file, and runs `ze config validate` on each golden. It never writes to the
golden file. `./le netlab render-update` rewrites the golden files.

It is out of `./le verify current mode full` because it needs netlab installed, and `./le verify current mode full` must run on
a machine that has neither netlab nor Jinja2. **A missing netlab is an error exit, not a
skip:** a check that passes without its dependency reports "no drift" about a render it
never performed. `NETLAB=/path/to/netlab` names a specific install.

`test/plugin/netlab-lab-profile.ci` is the second half and runs inside the normal plugin
suite, because it needs no netlab. It reads `contrib/netlab/golden/r3.conf` through
`ZE_REPO_ROOT`, starts a daemon from it, logs in as the user that render declared, and
parses `show bgp peer list | json compact` with `json.loads`. The render check proves the
templates still emit what ze accepts. The functional test proves ze still runs what they
emitted.
<!-- source: internal/le/netlab/actions.go -- Actions -->
<!-- source: test/plugin/netlab-lab-profile.ci -- golden read, daemon start, json compact -->

### Allocation-ceiling gate (`./le verify deps alloc`)

`./le verify deps alloc` runs the registered hot-path `ReportAllocs`
benchmarks with `-benchmem` at a bounded benchtime and enforces each
`allocs/op` ceiling. Allocation counts are machine-independent. A hot-path
benchmark opts in through `perf.AllocCeilings`; the gate fails when a registered
benchmark is absent from the output.

Timing regressions run separately through `bin/ze-perf track --check`, while
the Docker DUT matrix uses the native Go runner:
`go run ./cmd/ze-perf-run --build --test`.
<!-- source: internal/perf/allocgate.go -- AllocCeilings, checkAllocCeilings -->
<!-- source: internal/le/verify/deps/actions.go -- Actions -->
<!-- source: cmd/ze-perf-run/main.go -- main -->

### Privileged kernel-state tests under QEMU (`option=needs-linux`)

`.ci` tests tagged `option=needs-linux` skip on non-Linux hosts and unprivileged
Linux, then run as root inside `./le qemu all-tests`. CAP_NET_ADMIN and real
interfaces are available in that guest. `TestCapabilityGatedTestsHaveANativeVMHome`
fails when a capability-gated test has no registered QEMU path.
<!-- source: internal/le/qemu/alltests.go -- allTestsRun -->
<!-- source: internal/test/runner/record_parse.go -- capability gate -->
<!-- source: internal/test/runner/record_parse.go -- caps=net-admin gate and skip reason -->

The `traffic` suite is enrolled
in `allTestsRun.Run` in `internal/le/qemu/alltests.go`; `test/traffic/traffic-boot-qdisc-tc.ci` and
`traffic-reload-qdisc-tc.ci` assert real `tc qdisc show` kernel state after boot and
after a reload (the check `001`/`002` document as deferred). The chaos iface
fault family (`iface-link-flap`, `iface-addr-remove`) has a netns-scoped
integration tests supplied by the applicable `./le integration` action, plus
the `test/chaos/iface-link-flap.ci` scenario.

Suites that mutate shared, un-namespaced kernel state run serially in
`./le qemu all-tests`. The guest gives each test the QEMU root namespace rather
than the host namespace. The policy and firewall suites therefore cannot alter
the operator's nftables state.
<!-- source: internal/le/qemu/actions.go -- Answer -->

Dropping a whole suite to `-p 1` is not always the right tool. When only a
*cluster* of tests inside a large suite contends, they declare
`option=exclusive:group=<name>`: members of one group never run concurrently with
each other, while every unrelated test in the suite keeps running in parallel.
The `plugin` suite is 690 tests and five clusters inside it contend, so serializing
all of it would cost minutes per QEMU run.

The ddos tests are the motivating case (`option=exclusive:group=ddos-flood`). Each
one floods a victim address and its daemon's detector picks the victim by
top-destination-bytes over the *interface* those counters belong to, which in the
VM's single root namespace is the same loopback for every test. Unique victim
addresses do not partition that view -- they only stop the `EADDRINUSE` bind
collision -- so a sibling's concurrent flood is indistinguishable from the test's
own: `ddos-detect-characterize` resolved `127.0.0.4`, which belongs to
`ddos-detect-mitigate`, and `ddos-direction` resolved no victim at all and fell
back to `direction=remote`. Non-overlap is the only property that fixes it.
`TestContendingFunctionalTestsDeclareExclusiveGroup` ratchets the invariant so a
new needs-linux ddos test cannot land without it.

The BFD cluster (`option=exclusive:group=bfd-ports`) is the same shape with a
different shared resource, and it contends on every host rather than only in the
VM. BFD listens on the ports RFC 5881 and RFC 5883 *fix* -- 3784 for control and
3785 for echo -- so every BFD test's daemon binds the same wildcard tuple. They
co-exist only because `ze.bfd.test-parallel=true` sets `SO_REUSEPORT`, and the
kernel then hashes each inbound datagram to one socket in that group: a control
reply or a reflected echo meant for one daemon is delivered to a sibling's
instead. `bfd-echo-handshake` was the visible casualty, failing in CI with
`ze_bfd_echo_rtt_us histogram has no buckets` because its reflections were landing
on another BFD test's daemon. A port number an RFC fixes is precisely the address
unique config cannot partition. Membership is "declares `ze.bfd.test-parallel`",
which is the same ratchet's third cluster.
<!-- source: internal/component/bfd/transport/udp_linux.go -- applySocketOptions, SO_REUSEPORT under ze.bfd.test-parallel -->

The ipsec cluster (`option=exclusive:group=ipsec-xfrm`) contends on the kernel's
own tables. `ip xfrm state` and `ip xfrm policy` are node-wide, so a test that
reads them cannot tell its own SPIs and selectors from a sibling's. The two rekey
tests watch the SPI set for a make-before-break replacement arriving, and
`ipsec-teardown-leaves-nothing` asserts both tables are *empty*, which any
concurrent tunnel falsifies. Unique prefixes partition the policy reads and
nothing partitions the state reads, because an SPI is random. Run together before
the group existed, `ipsec-child-rekey-xfrm` read the narrowing test's replacement
SPIs and reported `POLICY-MOVED`, while `ipsec-child-rekey-xfrm-narrowing` read
that test's SPIs and reported `REKEY-ACCEPTED`. Both verdicts were about the other
test's kernel state. Membership is "declares
`option=needs-linux:caps=net-admin`", which is the same ratchet's fourth cluster.

The firewall-irr cluster (`option=exclusive:group=firewall-irr-nft`) contends on
two node-wide resources at once. The first is the nftables ruleset. These tests
cannot vary the table names they program: `ifaceTableName` is the Go constant
`ze_irr_iface`, and the config-derived tables collide as `ze_wan` and `ze_lan`.
The backend's `Apply` lists the node's tables and deletes every `ze_*` table it is
about to program, so one daemon removes a sibling's live table and the sibling
then reads back terms it never wrote. The second is the persisted prefix store.
`DefaultConfigDir` derives it from the running binary's path, so every daemon in
the run opens one `database.zefs`, and a sibling that fetches `AS-TEST` destroys
the cold-cache precondition `firewall-irr-cold-cache-recovers` asserts on its
first line. Measured on 2026-08-22 in one privileged container: run in parallel 2
of 12 failed, run serially 12 of 12 pass. Membership is "declares
`option=needs-linux:caps=net-admin`", which is the same ratchet's fifth cluster.
<!-- source: internal/component/firewall/plugins/irr/sets.go -- ifaceTableName, the constant no .ci can vary -->
<!-- source: internal/plugins/firewall/nft/backend_linux.go -- Apply, ListTables then DelTable over the desired ze_* names -->
<!-- source: internal/core/paths/paths.go -- DefaultConfigDir, binary-derived config dir shared by every daemon in the run -->

The FlowSpec cluster (`option=exclusive:group=flowspec-nft`) is the ratchet's
sixth and seventh rows and one group, because two suites program one object. The
FlowSpec bridge names its table `ze_flowspec` and nothing in a `.ci` can vary
that name, so every member that reaches the kernel writes and deletes the same
table: two at once delete each other's table mid-assertion. The `firewall` row
seeds `table inet flowspec`, the name an older ze build wrote, and any concurrent
ze daemon removes it on its first reconcile. Two `plugin` siblings stay outside
the selector on purpose: `flowspec-fw-add` and `flowspec-fw-withdraw` assert wire
bytes and touch no kernel. One exposure stays open and is cheaper than closing
it: the `firewall-irr` cluster holds a different group, and two groups do not
serialize against each other, so an irr daemon's sweep can still take a seeded
table. Each seeding test requires the daemon IT launched to log the removal, so
that race fails the test rather than passing it.
<!-- source: internal/plugins/flowspec-firewall/state.go -- tableName, the constant no .ci can vary -->
<!-- source: internal/component/firewall/legacy_tables.go -- legacyTables, the names any ze daemon removes once -->

Related: a test that binds a *chosen* port must take it from the runner's per-test
range (`$PORT`, `$PORT2`), never a literal. `bfd-echo-handshake` hardcoded
telemetry port 19274, which sits outside every range the runner hands out, so a
sibling whose shifted range covered it collided and ze logged `metrics server
failed to start: address already in use` -- a red with nothing to do with the
subject of the test.
<!-- source: internal/test/runner/record_parse.go -- option=exclusive parsing, ExclusiveGroup -->
<!-- source: internal/test/runner/parallel.go -- per-group lock taken before the concurrency semaphore -->
<!-- source: internal/test/runner/exclusive_group_test.go -- TestContendingFunctionalTestsDeclareExclusiveGroup ratchet -->
<!-- source: internal/plugins/policyroute/translate.go -- policyRoutingTable = "ze_pr" -->
<!-- source: internal/plugins/policyroute/marks.go -- fwmarkBase deterministic per process -->
<!-- source: test/traffic/traffic-boot-qdisc-tc.ci -- needs-linux tc qdisc assertion -->
<!-- source: internal/chaos/peer/simulator_actions_iface_linux.go -- iface fault executor -->

### How a suite's concurrency is chosen

A `.ci` suite runs `-p N` tests at once. Where N comes from depends on the suite:

| Suite | Source of `-p` | Value |
|-------|----------------|-------|
| `plugin`, `encode` | `ZE_PLUGIN_PARALLEL`, `ZE_ENCODE_PARALLEL` (`internal/le/functional.Answer`) | derived from the host: the core count, floored at 8 |
| `reload`, `managed` | the native suite table in `internal/le/functional/suites.go` | 1. They share the kernel routing table |
| `vpp` | the command's own default | 1 |
| the other bgp-runner suites | `runner.DefaultParallelConcurrent` | 20 |
| the 22 `registerCIRoot` suites | `runner.DefaultSuiteConcurrency` | 2x the core count, floored at 8 |

The floor is 8 for both derivations, and it is one measured figure rather than a
round number: it is what the `plugin` suite has been running at on GitHub's
4-vCPU hosted runner. A host at or below it therefore gets exactly what CI runs
today, which is why deriving the value changed nothing in CI.

The two derivations differ above the floor, and the difference is the shape of
the suite. `DefaultSuiteConcurrency` scales at 2x the core count because a
WAIT-bound suite spends most of its wall clock waiting for daemons, and because
its predecessor was "all at once": every suite declared 0, a non-positive
`Parallel` means `len(selected)`, and `ze-test ospf --all` launched 97 daemons
until a GitHub job died mid-suite on 2026-07-26 (exit 143, the runner agent
itself killed). `plugin` is CORE-bound instead, so it caps at 1x. Measured on a
32-core box against the suite's 4545s sum of per-test medians:

| `-p` | wall clock | speedup | parallel efficiency |
|------|-----------|---------|---------------------|
| 8 | 589.5s | 7.7x | 96% |
| 16 | 322.5s | 14.1x | 88% |
| 32 | 216.5s / 166.0s | 23.8x | 74% |
| 64 | 196.5s | 23.1x | 36% |

64 sits inside the two-run spread at 32, buys no measurable wall clock, and costs
pass rate. Neither figure transfers to the 22 `registerCIRoot` suites: that sweep
never measured them.

An explicit value still wins over the derivation:
`ZE_PLUGIN_PARALLEL=8 ./le functional plugin` pins the suite, and `-p 0`
on the runner command line selects every test at once.

Raising concurrency moves flakes before it moves wall clock, so a deadline the
harness cannot see is fixed first. `ParallelTimeoutHeadroom` widens every budget
the runner measures a child against, and it cannot reach a deadline the child
enforces INSIDE its own binary: `ze-test mcp` waited a fixed 10s for the daemon's
listener, and six of one 32-way run's failures were that one message. The runner
publishes `ze.test.parallel-factor` into every `cmd=` child's environment so such
a deadline scales from the same source of truth.

The runner does NOT read the job-admission budget. `internal/le/job.Answer` admits
several jobs on a shared box, and a suite still sizes itself for the whole
machine, so concurrent sessions can oversubscribe it.

<!-- source: internal/le/functional/actions.go -- Answer -->
<!-- source: internal/test/runner/parallel.go -- SuiteConcurrencyFloor, DefaultSuiteConcurrency, ParallelTimeoutHeadroom, ParallelFactorEnv -->
<!-- source: internal/test/cli/cmd_bgp.go -- the bgp runner's -p default -->
<!-- source: internal/test/cli/cmd_vpp.go -- the vpp suite's -p default of 1 -->
<!-- source: internal/test/cli/cmd_mcp.go -- the MCP readiness deadline, scaled by ChildParallelFactor -->

### Netns launch mode for netlink `.ci` suites (host-safe firewall/policy/OSPF)

The netlink functional suites (`firewall`, `policy`, `ospf`, `ospfv3`, and any
suite whose `ze` daemon needs `CAP_NET_ADMIN` for nft/FIB) carry
`option=skip-os:value=darwin` and program real kernel state. Run unprivileged the
nft backend hard-fails; run with caps in the host namespace they reprogram the
operator's real firewall. An **opt-in per-test network-namespace launch mode**
makes them runnable host-safely on Linux:

```bash
./le qemu netns-test
./le qemu netns-test suites firewall
```

The native action provisions capabilities, runs each suite in a disposable
network namespace, and proves the guest's root nftables state is unchanged.

A test that declares `option=netns-link` runs **only** in this mode. The option is
a prerequisite, not a hint: the links it names (`eth0`, `eth1`, `nbma0`, `ptmp0`)
are provisioned inside the throwaway namespace and must never be created on a real
host, so off netns mode the test is SKIPped with a reason naming these targets.
That covers the 8 `test/ospf`, 3 `test/ospfv3` and 1 `test/policy` tests listed in
`allTestsRun.Run` in `internal/le/qemu/alltests.go`; they carry `needs-linux` as well and use
the same registered QEMU action path.

<!-- source: internal/test/runner/caps.go -- applyNetnsLinkGate, skipReasonNetnsLink -->

#### Kernel-capability plugin subset

The `system-kernel-log-show` plugin test opens `/dev/kmsg` and needs
`CAP_SYSLOG`. It runs in the privileged guest population through
`./le qemu all-tests`; an unprivileged host run skips it with the required
capability in the reason.

The namespace runner locks its goroutine to one OS thread, enters a disposable
network namespace, and starts every child from that thread. The Go fixture
drivers inherit that namespace with the daemon. Cleanup restores the original
namespace and deletes the disposable one, so host firewall and link state stay
unchanged.

<!-- source: internal/test/runner/netns_linux.go -- enterTestNetns, testNetnsName -->
<!-- source: internal/test/runner/netns_linux_test.go -- TestNetnsLaunchChildInheritsNamespace -->
<!-- source: internal/test/runner/runner_exec.go -- runOrchestrated -->
<!-- source: internal/le/qemu/actions.go -- Actions -->

### In-process integration tests (feeds that can't cross the plugin boundary)

Some chains are driven by `internal/core/observation`, a **process-local** feed
(a `.ci` cannot inject into it, because a config-loaded plugin runs isolated from
the engine's in-engine consumers). These are proven by in-process Go integration
tests that compose the real production types in one process and drive them with
synthetic observations. The anomaly facts→judgment→response chain
(`trafficfeature` → `anomaly/detect` → `anomaly/shape`) is proven this way by
`TestChainFactsToResponse`; it is `-short`-skippable (drives real 1s ticks).
<!-- source: internal/plugins/anomaly/detect/chain_integration_test.go -- TestChainFactsToResponse -->

### Development Workflow

Use the narrowest test scope that covers your change, then widen before committing:

```
single test  →  single package  →  component group  →  ./le verify current mode full
```

```bash
./le job run label one-config-test command go test ./internal/component/config/system/... -run TestMyThing
./le test-unit config
./le verify current mode full
```

#### Test binaries are isolated from the development binary

Every native functional action builds `ze`, `ze-test`, and `ze-stripped` into a
session-owned temporary `bin/` directory. It sets the binary paths before the
suite starts, freezes that set for the run, and removes it afterward. Editing
or rebuilding `bin/ze` cannot change a suite already in flight.

The binaries must live below a directory named `bin` or `sbin`, because Ze
derives its config and database directory from its executable path. The runner
prepends the isolated directory to `PATH`, so `.ci` fixtures continue to invoke
the canonical bare names.

<!-- source: internal/le/functional/binaries.go -- Prepare, Release -->
<!-- source: internal/test/runner/runner.go -- prebuilt binary environment -->
<!-- source: internal/test/runner/runner_exec.go -- bare-name resolution -->

`./le verify current mode full` is the pre-commit gate. It runs a two-pass strategy:
1. Cached full pass (all packages, no `-race`; completes quickly when nothing changed)
2. Race pass on changed groups only (detects data races in what you touched)
3. All functional test suites
4. ExaBGP compatibility

On failure, read `tmp/ze-verify-failures.log` first. It is a compact routing
index with one block per failing stage and group. Each group names its `Rerun`
command, `Detail log`, and `Parallel` value, so a reader can choose related
failures without opening the full combined log. Open the `Detail log` path it
names for full evidence for that group. That path is inside the run's own
directory, so it stays correct after another session's run publishes its own
failure index. Use
`tmp/ze-verify.log` only when the whole combined run is needed.
Automation should read `tmp/ze-verify-failures.json`.
<!-- source: internal/le/testunit/actions.go -- Actions -->
<!-- source: internal/le/verify/engine/run.go -- Run, RunMode -->

---

## RFC Requirement Tags

A test that enforces an RFC 2119 MUST-level requirement tags itself, so the
coverage gate can bind the requirement to the proof that enforces it. The tag
names a stable requirement id (allocated in `rfc/short/*.md`, e.g.
`RFC7606-7.1-1` is §7.1, first requirement) and a mandatory polarity, `positive`
or `negative`. In Go tests the tag is a `//` comment; place it on the function
doc comment when the function tests one requirement, or inline at the table case
when one function covers many:

```go
// RFC requirement: RFC7606-7.1-1 positive -- valid ORIGIN length 1 is accepted
func TestRFC7606OriginValueIGP(t *testing.T) {

// RFC requirement: RFC7606-7.1-1 negative -- ORIGIN length 2 is treated as withdraw
func TestRFC7606MalformedOriginLength(t *testing.T) {
```

In a `.ci` file the tag is a line-start `#` comment with the same fields, and
must not sit inside a `terminator=` block (there `#` is raw file content, not a
comment):

```
# RFC requirement: RFC7606-7.1-1 negative -- malformed ORIGIN withdraws the route
```

Rules the gate enforces:

- **Polarity is mandatory and never inferred.** A tag with no `positive` or
  `negative` word fails to parse.
- **Every gated MUST needs BOTH polarities.** A negative-only test passes if the
  code rejects everything; a positive-only test passes if it accepts everything.
  Only the pair pins behavior to the requirement. A requirement that is genuinely
  testable only one way carries a `{single-polarity: positive|negative; why}`
  annotation on its summary line instead.
- **`./le rfc check` is the gate.** For every MUST-level requirement of an
  enrolled RFC (its summary's `## Meta` table declares `Enrolment: enrolled`) it
  requires the positive/negative pair, or a reasoned `{gap}` /
  `{not-applicable}` / `{single-polarity}` annotation. It scans
  Go `_test.go` files and `.ci` files under `internal/`, `pkg/`, and `test/`.
  `./le rfc index-update` renders each RFC's requirement to test rows into
  `rfc/requirements/<stem>.md`. It renders the index over them into
  `ai/RFC-REQUIREMENTS.md`.
- **Do not edit a tagged test to make it pass.** Once a test carries an
  `RFC requirement:` tag its behavior cannot change without the owner's approval.
  Write it as one row in `test/rfc-changed.md`, and commit that file with the
  change. Fix the code instead. The native weakening check reads the file and
  blocks the edit until a row names the test.

<!-- source: internal/le/testweakened/proposed.go -- RFC-tagged carrier approval -->

### What the tags do not cover

Tags bind the requirements a summary **lists**. Nothing in the paragraphs above
can see an obligation nobody wrote down, so a green gate is bounded by what was
extracted. That boundary is closed separately, by a per-RFC **extraction
sign-off** at `rfc/extraction/<stem>.json`: every normative site of the RFC's own
text is mapped to a requirement id or excluded with a reason, and the gate
re-derives the site inventory from the source and re-checks the arithmetic on
every run.

```
./le rfc extraction-create stem rfcNNNN    # skeleton to session scratch; classify, then move it in
./le rfc check                    # re-derives and judges
./le rfc extraction-status        # JSON counts, per register, plus the backlog
```

Only dispositions are authored: sites, sections, quotes and the register are
derived, so a hand-typed count cannot exist and an unclassified site cannot be
hidden. The skeleton writer emits only unclassified entries, so generating
artifacts makes the gate redder rather than greener. A sign-off is required
before enrolling a stem that was not enrolled at HEAD; RFCs enrolled earlier are
grandfathered and published as a counted backlog in `ai/RFC-REQUIREMENTS.md`.
Contract: `rfc/extraction/README.md`.

<!-- source: internal/le/rfc/actions.go -- Answer -->

### What the tags cannot say: the audit record

A tag proves a LINK exists. Nothing in it can say whether the test would FAIL if the
implementation stopped complying. That judgement is what a compliance claim actually rests on.
It is recorded per requirement in `rfc/audit/<rfc>.json` by `/ze-rfc-audit`, and the gate
now reads it.

The verdict is one of five closed values. `enforced` is the only one that means proven, and it
requires a cited test and both polarities. `weak` (tagged and green but cannot fail on
non-compliance), `wrong` (asserts something the RFC does not say), `unimplemented` (the code does
not comply — needs a `code` map naming the producing function) and `not-applicable` (no reachable
code path can satisfy or violate it — needs a `no_code_path` reason and an agreeing
`{not-applicable}` annotation) each subtract the requirement from the published **proven** count
in `ai/RFC-REQUIREMENTS.md` while still exiting 0.

That split is deliberate. Recording a finding is free and green. Erasing one is red. Two moves
are therefore blocked:

- A `weak` or `wrong` verdict cannot be deleted, and it cannot be silently upgraded to `enforced`.
- A verdict that existed at HEAD cannot vanish.

Audit coverage is monotonic per requirement id, because otherwise deleting the verdict is the
cheapest route from red to green.

<!-- source: internal/le/rfc/actions.go -- Answer -->

A verdict goes stale when what it judged changes, in one of three ways, and only one of them
wants a human:

```
./le rfc reseal                   # clears SHIFTED: a line shift or a sibling edit
./le rfc index-update                    # then re-render the ledger
/ze-rfc-audit <rfc>                  # clears STALE: the tagged test itself changed
```

`SHIFTED` means the tagged unit is byte-identical and only the file around it
moved. The unit is the enclosing top-level Go function or the whole `.ci`,
`.et`, or native interop fixture.

`./le rfc reseal` is the only thing that writes `rfc/audit/` without a human edit.
`./le rfc check` is read-only, and `./le rfc index-update` touches `ai/RFC-REQUIREMENTS.md` and
`rfc/requirements/` alone. A re-stamp can therefore never happen as a side effect of
unrelated work.

<!-- source: internal/le/rfc/actions.go -- Answer -->

The definition of "the tagged unit" lives in one place, `internal/le/rfc.Answer`, which
both this gate and the edit-time `rfc-tagged-test` guard import. A second copy that drifted would
let the gate re-seal a verdict against a fingerprint the guard does not compute.

<!-- source: internal/le/rfc/actions.go -- Answer -->

### What a verdict cannot replay: the discrimination record

An audit verdict is a person's judgement, written on the day they read the test.
`rfc/discrimination/<stem>.json` records a machine-replayable answer to the same
question: a named break of the producing code, and the observation that the tagged unit
went RED under that break and green again after it. `./le rfc check` replays the
fingerprints on every run, so a rewritten unit, a reworded claim or a moved producer
voids the record instead of outliving it.

`./le rfc discriminate-record` is the only writer, and it writes only a red it observed
itself. For a `.ci` tag it disables the producing function, runs ONE test of ONE suite,
requires a failure that NAMES that test, and puts the producer back byte for byte. That
is the walk `docs/contributing/testing.md` prescribes under "A test that exists is not a
test that gates", with the result kept. For an interop tag it runs the one scenario the
checker declares, and the record cites the numbered assertion it rests on.

A tag you ADD owes a record in the same change. The two proof routes, the closed escape
vocabulary and the refusals are in `docs/contributing/rfc-conformance-gates.md`.

<!-- source: internal/le/rfc/discriminate_action.go -- recordDiscrimination -->

Full authoring guidance: `ai/skills/ze-rfc.md` (id allocation and annotations),
`ai/skills/ze-rfc-audit.md` (letter-and-spirit audit, the verdict vocabulary and the four
freshness states), and `docs/contributing/rfc-implementation-guide.md`.

---

## Test Types

### 1. Encode Tests (`test/encode/`)

Static route tests - routes defined in config, sent at session establishment.

**Files:**
- `*.ci` - Expected messages and config reference
- `*.conf` - Ze configuration

### 2. Parse Tests (`test/parse/`)

Config parsing tests verify configurations parse correctly.

**Files:** All parse tests use `.ci` format with embedded config.

Parse coverage configs live in `test/parse/coverage-*.ci`. They are positive
parse/validate coverage for realistic multi-feature configurations, such as
IXP peering, large peer sets, RPKI policy, and redistribution. Run a specific
coverage case by name, for example
`./le job run label parse-ixp command bin/ze-test bgp parse coverage-ixp-peering`.

BGP interop scenarios live under descriptively named directories in
`test/interop/scenarios/`. Run all with `./le integration interop`, or one with
`INTEROP_SCENARIO=<name> ./le integration interop`. Scenario files use the
default `172.30.0.x` lab prefix, then render into an available `/24`. Concurrent
runs retry successive subnets. Use `ZE_INTEROP_SUBNET_INDEX=N` or
`ZE_INTEROP_SUBNET_PREFIX=A.B.C.` to force a prefix. A scenario whose config
enables BMP gets the compiled collector personality on the run's `.6` address
before Ze starts.
`rpki-server` starts `ze-test rpki --bind 0.0.0.0` on the run's `.7` address.

The tree is not BGP-only. A scenario that includes `keepalived.conf` gets a real
keepalived on the run's `.8` address, and `vrrp-mastership-keepalived` is the
scenario that uses it: Ze at priority 200 and keepalived at 100 contend for one
virtual IP on the run's `.100` address. It asserts who owns that address, read
with `ip -o -f inet addr` inside each container, over three phases -- Ze holds it
alone for longer than keepalived's Active_Down_Interval, keepalived takes it over
inside Skew_Time when Ze is sent SIGTERM (which is Ze's RFC 9568 Section 6.4.3
Priority 0 advertisement being accepted, not a timeout), and Ze preempts and
takes it back. Before it existed, VRRP had 150 unit tags and no executed interop.

<!-- source: internal/le/interoplab/bgp/checkers.go -- vrrp-mastership-keepalived -->

**Positive tests** (expect success):
```
# test/parse/simple-v4.ci
stdin=config:terminator=EOF_CONF
bgp {
    peer test-peer {
        remote {
            ip 127.0.0.1;
            as 65533;
        }
        router-id 10.0.0.2;
        local-as 65533;
    }
}
EOF_CONF

cmd=foreground:seq=1:exec=ze bgp validate -:stdin=config
expect=exit:code=0
```

**Negative tests** (expect failure):
```
# test/parse/route-refresh-no-process.ci
stdin=config:terminator=EOF_CONF
bgp {
    peer test-peer {
        remote {
            ip 10.0.0.1;
            as 65002;
        }
        router-id 1.2.3.4;
        local-as 65001;
        capability { route-refresh; }
    }
}
EOF_CONF

cmd=foreground:seq=1:exec=ze bgp validate -:stdin=config
expect=exit:code=1
expect=stderr:contains=route-refresh requires process with send { update; }
```

### 3. MCP Tests (`test/plugin/mcp-*.ci`)

End-to-end scenarios for the MCP transport. The runner launches a ze
daemon with `--mcp <port>` in the background and `ze-test mcp` in the
foreground; assertions come from `expect=exit:code=...` and
`expect=stdout|stderr:contains=...`.

`ze-test mcp` speaks MCP revision `2026-07-28`. Every message it sends is its
own HTTP POST to `/mcp` carrying the `MCP-Protocol-Version`, `Mcp-Method` and
(where required) `Mcp-Name` headers plus a `params._meta` block: there is no
handshake, no session id, and no GET stream to set up.

<!-- source: internal/test/cli/cmd_mcp.go -- ze-test mcp driver -->

Driver flags:

| Flag | Purpose |
|------|---------|
| `--port <port>` | MCP server port (required) |
| `--token <token>` | Bearer token, sent on every request |
| `--timeout <duration>` | Connection timeout (default 10s) |
| `--tasks` | Declare the `io.modelcontextprotocol/tasks` extension in every request's `_meta.clientCapabilities` |
| `--elicit <modes>` | Declare the `elicitation` capability in every request. `form`, `url`, `form,url`, or `empty` for the `{}` shape the specification equates with form mode only. Omitted, the capability is not declared at all |

<!-- source: internal/test/cli/cmd_mcp.go — cmdMcp flag set -->
<!-- source: internal/test/cli/cmd_mcp_mrtr.go — parseElicitCapability -->

`--elicit` takes modes, not a bare boolean, because the capability is
mode-structured. A client that declares `url` supports elicitation, and a server
must never send it a form-mode request. `--elicit url` and `--elicit form` are
therefore different tests, not two spellings of one.

There is no `--resources` flag. `resources` is a `ServerCapabilities` member,
not one of the five `ClientCapabilities` members (`experimental`, `roots`,
`sampling`, `elicitation`, `extensions`). A conformant client therefore never
declares `resources`, and the daemon serves `resources/list` and
`resources/read` to every caller.

<!-- source: internal/component/mcp/resources.go — resourcesList, resourcesRead -->

There is no `--ui` flag either. The MCP Apps extension is declared per request
through `probe-meta clientCapabilities`, because its settings object is what the
test is usually varying:

```
probe-meta clientCapabilities {"extensions":{"io.modelcontextprotocol/ui":{}}}
probe tools/list
```

<!-- source: internal/component/mcp/apps.go — clientSupportsUIApps -->

**Multi Round-Trip Requests.** When the daemon answers
`resultType: "input_required"`, the client builds `inputResponses` from the
queued answer. The client then retries the ORIGINAL request under a new JSON-RPC
id, up to four rounds. Without a queued answer the call fails instead. No round
trip is therefore taken by accident. A test that did not ask for one still sees
the interim result as a failure.

| Directive | Effect |
|-----------|--------|
| `elicit-answer accept <value>` | Supply `<value>` under the property name the server's `requestedSchema` declares |
| `elicit-answer decline` / `cancel` | Answer with that action. Both are terminal |
| `elicit-answer omit` | Retry carrying an empty `inputResponses`, which drives the server's re-ask path |
| `elicit-answer none` | Forget the queued answer |
| `elicit-extra <key>` | Also send an `inputResponses` entry under `<key>`, which no server asked for. `-` clears it |

The client reads the answer's field name off the server's `requestedSchema`, and
it assumes no name. The client also refuses to answer an elicitation whose
`mode` it did not declare. Both rules keep the client an independent reading of
the protocol, not a restatement of the daemon. A server that sent a form-mode
request to a url-only client therefore fails the run with a named error. The
client does not accommodate it quietly.

<!-- source: internal/test/cli/cmd_mcp_mrtr.go — send retry loop, answerElicitRequest, elicitDirective -->

`tools-order-stable [<calls>]` calls `tools/list` `<calls>` times (default 3) <!-- doc-links: ignore (JSON-RPC method name, not a path) -->
against an unchanged daemon and prints
`tools-order stable=<true|false> calls=<n> tools=<n> digest=<hex>`. The
comparison is over the RAW bytes of the `tools` array, not over the tool names.
A wobbling action enum or a drifting description leaves every name in place, and
it still defeats client caching. Names alone would therefore under-assert the
acceptance criterion (a byte-identical array, "including every action enum and
every description string"). The digest on the stable line is a SHA-256 prefix
over that array, so a `.ci` can assert the byte comparison actually ran.

On drift the line names the diverging call. It then names the first differing
tool index with both name sequences. When the names are identical and only the
payload moved, it names the byte offset and both digests instead.

The driver compares the responses, and a `.ci` expectation does not, because
Go's RE2 has no backreference. "These two responses are identical" cannot be
written as a pattern.

<!-- source: internal/test/cli/cmd_mcp_calls.go — toolsOrderStable, toolsList, toolsDigest, firstDifference -->

**Colons in expectation values.** `ParseKVPairs` splits a `.ci` directive on
`:`, but only where a colon introduces a real key token. A real key token is a
letter, then letters, digits, `-` or `_`, then `=`. An ordinary colon stays in
the value, so `contains=error: no such peer` asserts the whole sentence.
`json=`, `text=`, `hex=` and `pattern=` are consumed whole and keep everything
after them.

The shape that still splits is a value carrying something that looks like a key:
`contains=note:level=high` breaks at `:level=`. That split is deliberate,
because it is how the engine-step form `contains=aes-cbc:timeout=25` keeps
working. A needle of that shape therefore needs `pattern=`. `pattern=` must come
last on the line, because it consumes the remainder.

**This was not always so, and the history is the reason to keep the rule in
mind.** Until 2026-07-30 every colon split, so a `contains=` needle was silently
cut at its first colon: `contains="cacheScope":"public"` asserted only
`"cacheScope"`. A sweep found **203 assertions across 15 suites** weakened that
way. The re-armed assertions then exposed a security test,
`test/appliance/appliance-push-image-escape.ci`, that had never once exercised
the path-traversal guard it was named for. Its symlink resolved to a nonexistent
file, so the code returned "not found" long before the escape check. And the
truncated needle `error` accepted that result.

<!-- source: internal/test/ci/ciformat.go — ParseKVPairs complexKeys, splitOnKeyBoundary -->


### 3c. Task Tests (`test/plugin/task-*.ci`)

Eight files cover the `io.modelcontextprotocol/tasks` extension:
`task-cancel.ci`, `task-extension-advertised.ci`, `task-forbidden.ci`,
`task-identity-scope.ci`, `task-no-extension.ci`, `task-removed-methods.ci`,
`task-rib-routes.ci` and `task-update-ack.ci`.

Most pass the `--tasks` flag to `ze-test mcp`, which declares the
`io.modelcontextprotocol/tasks` identifier under
`_meta.clientCapabilities.extensions` on every request.
`task-no-extension.ci` deliberately omits the flag. It is the A/B twin of
`task-rib-routes.ci`, with the same peer and the same tool. It asserts that a
client which never declared the extension still gets its answer synchronously,
not a task handle.

Task creation is server-directed. The client sends an ordinary `tools/call`, and <!-- doc-links: ignore (JSON-RPC method name, not a path) -->
the daemon decides from the command's `ze:task-support` YANG annotation whether
to answer with a task handle. There is no per-call opt-in field to set.

| Directive | Purpose |
|-----------|---------|
| `task-call <tool> [<args>]` | Ordinary `tools/call` the server must answer with `resultType: "task"`. Prints the taskId | <!-- doc-links: ignore (JSON-RPC method name, not a path) -->
| `call-sync <tool> [<args>]` | Ordinary `tools/call` the server must answer synchronously (`resultType: "complete"`, no taskId) | <!-- doc-links: ignore (JSON-RPC method name, not a path) -->
| `task-get <id>` | Call `tasks/get`, print the status |
| `task-result <id>` | Print the result a terminal task carries, read off `tasks/get` |
| `task-update <id> [<json>]` | Call `tasks/update` with optional `inputResponses`. Requires an empty acknowledgement |
| `task-cancel <id>` | Call `tasks/cancel`. Requires an empty acknowledgement |
| `task-wait <id> <state>` | Poll `tasks/get` until the state matches |

<!-- source: internal/test/cli/cmd_mcp.go -- taskDirective -->

`$LAST` substitutes the most recent directive output (usually the taskId from
`task-call`). `task-update` and `task-cancel` deliberately do not update it,
because both return an empty acknowledgement rather than an identifier.

Tasks are polled, never pushed. `2026-07-28` has no server-to-client stream on
this transport, so `task-wait` polls `tasks/get` and does not wait on a
notification. The surviving method set is exactly `tasks/get`, `tasks/update`
and `tasks/cancel`. `tasks/list` and `tasks/result` are gone, and the terminal
payload now rides on `tasks/get`. `task-removed-methods.ci` asserts that the two
removed names answer as unknown methods, and it probes the surviving three in
the same run.

<!-- source: internal/component/mcp/streamable_tools.go -- callTool, createTask, tasksUpdate -->
<!-- source: internal/component/mcp/tasks.go -- TaskInfo.toWire -->

### 3d. Conformance Probes (`probe-*` directives)

The conformance surface -- header validation, version rejection, malformed
`_meta`, and the 405 on GET and DELETE -- is driven with the driver's
`probe-*` directives, which build deliberately-malformed requests that the
ordinary directives cannot express.

Each `probe-*` line queues one deviation; the next `probe` applies every queued
deviation, prints one result line, then clears the queue. A `probe` with nothing
queued sends a fully conformant request, which is how a test asserts the success
shape (`resultType`, `serverInfo`).

| Directive | Purpose |
|-----------|---------|
| `probe-header <name> <value\|->` | Set a request header verbatim, with no sentinel encoding; `-` omits the header |
| `probe-meta <key> <value\|->` | Set a `params._meta` field; `-` omits it. Short keys expand to their `io.modelcontextprotocol/` names |
| `probe-method <verb>` | HTTP verb for the next probe (default POST) |
| `probe-body <json\|->` | Send this exact request body; `-` sends an empty body |
| `probe <method> [<json params>]` | Send one request; prints `probe status=<http> code=<jsonrpc\|ok\|none> [data=<json>] [result=<json>] message=<text>` |

`MCP-Protocol-Version` is derived from the `_meta` protocolVersion value, so
`probe-meta protocolVersion <old>` sends a consistent pair that tests version
rejection (`-32022`), while `probe-header MCP-Protocol-Version <old>` sends a
header/body mismatch that tests `-32020`.

<!-- source: internal/test/cli/cmd_mcp.go -- probe-* stdin directives -->

### Forward-Path Claims

Claims about forwarding, per-destination egress filters, or wire-visible
re-advertisement must drive a real forward path. In practice that means the
`.ci` file must load a plugin that calls `ForwardUpdate()` or
`ForwardUpdatesDirect()` (for example `bgp-rs`) and then assert on the
destination peer with `expect=bgp:...` or another deterministic wire-visible
signal.

A two-peer setup by itself is not evidence. If no forwarding-capable plugin is
loaded, a destination `ze-peer` may establish while no egress filter ever runs.
Those files must be marked `partial` or `blocked` instead of claiming full wire
coverage.

There is also a known single-`ze-peer` multi-IP timing limitation for some
multi-destination scenarios. When a test needs one `ze-peer` process to keep
multiple local-IP sessions alive long enough for deterministic wire assertions
and that timing remains flaky, the file should name that exact blocker and stay
`partial`/`blocked` until the fixture support exists.

### Test-Only Internal Plugins (`internal/test/plugins/`)

Some `.ci` tests need a synthetic Go-side producer to drive features whose
real producer requires privileged or external infrastructure. `fakel2tp`, for
example, drives L2TP route-change events without requiring kernel L2TP, PPP,
or a peer daemon. These plugins live under `internal/test/plugins/<name>/`
and are loaded only by DUT builds compiled with the `zetest` tag. Normal
production builds do not import `internal/test/plugins/all` and do not expose
these registry entries.

First occupant: `internal/test/plugins/fakeredist/`. Pattern:

| File | Role |
|------|------|
| `fakeredist.go` | Package state, command parser, batch builder/emitter |
| `store.go` | Current-set tracking + `reemitAll` for redistribute late-join replay |
| `register.go` | Plugin registration + `OnExecuteCommand` dispatcher + `ReplayRequest` subscription |
| `fakeredist_test.go` | Unit tests for the command surface |

fakeredist tracks the routes it has emitted (`store.go`) and subscribes to
`redistevents.ReplayRequest`, re-emitting its current live set tagged with the
echoed `ReplayID` so the late-join `.ci` tests (`redistribute-late-join*.ci`) can
drive a peer-up replay in-process, the same way the real producers (static,
connected, l2tp) do.

The aggregator at `internal/test/plugins/all/all.go` blank-imports every
test-only internal plugin. `cmd/ze/plugins_zetest.go` imports that aggregator
only under the `zetest` build tag so functional tests can exercise fakeredist
and fakel2tp without polluting normal `cmd/ze` builds.

<!-- source: internal/test/plugins/fakeredist/register.go -- pattern reference -->
<!-- source: internal/test/plugins/all/all.go -- test-only aggregator -->
<!-- source: cmd/ze/plugins_zetest.go -- zetest-only import -->

### 4. Reload Tests (`test/reload/`)

Config reload tests - verify SIGHUP-triggered reload behavior end-to-end.

**Files:** All reload tests use `.ci` format with embedded config and tmpfs alternate configs.

**How they work:**
1. Daemon starts with initial config, establishes BGP session
2. Test peer verifies initial messages
3. `action=rewrite` replaces config file with alternate version in tmpfs
4. `action=sighup` sends SIGHUP to daemon PID (read from `daemon.pid` in tmpfs)
5. Daemon reloads config — peers restart if settings changed
6. Test peer verifies reconnection and new messages

**Example:**
```
# Initial config establishes session with one route
stdin=ze-bgp:terminator=EOF_CONF
bgp { peer loopback { remote { ip 127.0.0.1; } ... nlri { ipv4/unicast add 192.168.1.0/24; } } }
EOF_CONF

# Alternate config with two routes
tmpfs=config2.conf:terminator=EOF_CONF2
bgp { peer loopback { remote { ip 127.0.0.1; } ... nlri { ipv4/unicast add 192.168.1.0/24; ipv4/unicast add 10.0.0.0/24; } } }
EOF_CONF2

option=tcp_connections:value=2
expect=bgp:conn=1:seq=1:hex=...   # Initial route
action=rewrite:conn=1:seq=2:source=config2.conf:dest=ze-bgp.conf
action=sighup:conn=1:seq=2
expect=bgp:conn=2:seq=1:hex=...   # Both routes after reload
```

### 5. VPP Tests (`test/vpp/`)

VPP functional tests are outside the default release gate. Run them with:

```bash
./le functional vpp
```

The suite uses the compiled Go VPP API stub. Each test gets a fresh Unix socket,
so it needs no DPDK, vfio, root, or Python helper.
<!-- source: internal/test/cli/cmd_vpp_stub.go -- cmdVPPStub, runVPPStub -->
<!-- source: internal/test/cli/cmd_vpp.go -- cmdVpp -->

Real-daemon evidence uses `./le deployment vpp-test`. It starts VPP in Docker
and checks FIB, traffic, MPLS, and IKE/IPsec dataplane behavior.
`./le deployment vpp-iface-test` covers GRE and WireGuard interface behavior.
<!-- source: internal/le/deployment/actions.go -- Actions -->

### 6. Backend Apply-Path Unit Tests (Go `_test.go`)

Backends that talk to an out-of-process IPC surface (VPP GoVPP, a netconf server,
any future RPC-backed kernel) expose a narrow unexported operation interface
(`vppOps`, `netconfOps`, ...) with only the methods the Apply path needs. The
production adapter wraps the live transport; unit tests substitute a scripted
fake that records calls and can fail on the Nth request.

`internal/plugins/traffic/vpp/` is the reference implementation:

- `ops.go` defines `vppOps` (6 methods: `dumpInterfaces`, `dumpPolicers`,
  `policerAddDel`, `policerDel`, `policerDeleteByName`, `policerOutput`).
- `backend_linux.go` exposes an internal `applyWithOps(ops, desired)` entry
  point; `ops_linux.go` holds the `govppOps{ch api.Channel}` production
  adapter. `Apply` builds one through `newGovppOps` (`timeout_linux.go`)
  around the channel it opened and calls `applyWithOps`. The constructor
  installs the GoVPP reply deadline before it returns, so no request goes out on
  an unbounded channel. `ops_construction_test.go` is what keeps the constructor
  the only builder: it parses the package's own sources and fails on a
  `govppOps` built outside it. That scan reads the three forms that name the
  type directly and says so in its own comment, so it is a ratchet against the
  inline literal this replaced rather than a proof of impossibility.
- `apply_test.go` defines `fakeOps` (records a `[]string` of labeled calls,
  supports `failOnNthAddDel` for deterministic partial-failure tests) and
  covers the create/update/undo/reconcile/orphan branches without a running
  VPP daemon.

Use this pattern when adding a new backend whose Apply path would otherwise
require full-stack integration tests to cover every undo / reconcile branch.

<!-- source: internal/plugins/traffic/vpp/ops.go -- vppOps interface -->
<!-- source: internal/plugins/traffic/vpp/backend_linux.go -- applyWithOps, Apply -->
<!-- source: internal/plugins/traffic/vpp/ops_linux.go -- govppOps adapter -->
<!-- source: internal/plugins/traffic/vpp/timeout_linux.go -- newGovppOps, the constructor Apply uses -->
<!-- source: internal/plugins/traffic/vpp/apply_test.go -- fakeOps + Apply-path tests -->

### 7. Decode Tests (`test/decode/`)

BGP message decoding tests verify that wire bytes decode to the expected JSON.
They use `.ci` files for command-based tests and still accept legacy `.test`
fixtures.
<!-- source: internal/test/runner/decoding.go -- DecodingTests.Discover, DecodingRunner.Run -->

**Format:**
```
stdin=payload:hex=<hex-encoded-bgp-message>
cmd=foreground:seq=1:exec=ze bgp decode --json --family <family> -:stdin=payload
expect=json:json=<expected-json>
```

JSON validation compares parsed objects field by field with key order ignored,
volatile fields removed, and `peer`/`neighbor` naming normalized.
<!-- source: internal/test/runner/decoding.go -- compareJSON, normalizeNeighborSection -->

### Install Tests (`test/install/`)

Install functional tests cover the offline install surface and the appliance
kernel, initrd, and ISO actions. The kernel builder is native Go in
`internal/appliance/kernelbuilder`; Docker and QEMU are backends of the same
driver.

```bash
./le functional install
./le qemu install-test
./le qemu install-iso-test
./le qemu install-scenarios-test
./le qemu install-ventoy-test
```

The QEMU actions self-skip with one visible line when the operator-supplied
installer kernel or a required host tool is unavailable. A real failure exits
non-zero.

<!-- source: internal/test/cli/register.go -- install CI root -->
<!-- source: internal/appliance/kernelbuilder/driver.go -- Build -->
<!-- source: internal/appliance/kernelbuilder/worker.go -- RunWorker -->
<!-- source: internal/le/functional/actions.go -- Actions -->
<!-- source: internal/le/qemu/actions.go -- Actions -->

### IS-IS Tests (`test/isis/`)

IS-IS functional tests cover the native IS-IS component (ISO/IEC 10589, RFC 1195
/ 5305 / 5301): config parse -> YANG schema -> NET/system-id validation ->
component startup. They exercise the config-wiring surface only; live adjacency,
LSDB, SPF, and FRR interop run as QEMU integration tests (raw L2 needs
`CAP_NET_RAW` on a Linux veth) and are owned by the runtime sibling specs.

| File | What it verifies |
|------|------------------|
| `isis-config.ci` | A valid `isis { net ... }` block validates through the real YANG schema and the `isis-net` custom validator; a structurally invalid NET is rejected |
| `isis-adjacency.ci` | The per-interface adjacency leaves (circuit-type, hello-interval, hold-multiplier, priority, level) validate on a point-to-point and a broadcast circuit; a hold-multiplier of 0 is rejected by the YANG range. Live adjacency (two nodes reaching Up) is proven by the `TestISISAdjacencyUp` unit test and the `adjacency_integration_linux_test.go` QEMU integration test |
| `isis-doctor-raw-socket.ci` | The `isis-raw-socket` doctor check reports the raw-socket / `CAP_NET_RAW` readiness of the transport |
| `isis-flooding.ci` | The CSNP and PSNP PDUs the flooding child builds for LSDB synchronisation (whole-range CSNP with a TLV 9 LSP-entry; PSNP ack/request TLV 9) decode through `ze isis decode`. Live reliable flooding (a three-node line converging, the SRM flood timer, CSNP/PSNP request/ack, the P2P initial CSNP, purge re-flood) is proven by the in-memory engine wiring tests (`TestISISLSDBSync`, `TestISISFloodSRMTimer`, `TestISISCSNPGapRequest`, `TestISISPSNPAck`) plus the QEMU integration tests and the FRR interop scenarios (isis-13) |
| `isis-auth.ci` | The authentication config surface validates through the real YANG schema: named `key-chains` with a `key-id`, an algorithm enum (cleartext / hmac-md5 / hmac-sha-256), a `$9$`-encoded secret, and per-level (area/domain) + per-interface (IIH) `auth-key-chain` references; an invalid algorithm enum is rejected. Live sign-on-send / verify-on-receive, wrong-key rejection, TLV-10-first ordering, the LSP checksum-after-sign interaction, authenticated purges, hitless rotation, and constant-time compare are proven by the unit tests (`TestISISAuthSignVerify*`, `TestISISAuthReject`, `TestISISAuthLSPChecksumAfterSign`, `TestISISAuthPurge`, `TestISISAuthRotation`, `TestISISAuthSecretEncoding`) plus the QEMU integration tests and the `isis-auth-frr` interop scenario (isis-13) |
| `isis-redist-bgp.ci` | The redistribution config surface validates through the real YANG schema: `redistribute { destination bgp { import isis } }` is accepted (the single source name `isis` is registered with the source registry, so the `redistribute-source` validator/completion accept it), and `redistribute { destination isis { import connected/static/bgp } }` is accepted (`destination isis` is a runtime-validated free-form list key); `destination isis { import isis }` validates but is a runtime no-op (loop prevention). Live route flow (an IS-IS SPF route in the BGP RIB; a connected/static/BGP prefix as a TLV 135 entry in an IS-IS LSP and in a peer's RIB) is proven by the unit tests in `internal/plugins/isis/redistribute` (`TestISISProducerRegistered`, `TestISISRegisterSource`, `TestISISRedistSourceToBGP`, `TestISISRedistSourceWithdrawToBGP`, `TestISISRedistConsumerConnected`/`Static`/`BGP`/`Withdraw`/`UpDownBit`/`Name`/`LogsFailure`, `TestISISConnectedAdvertise`, `TestISISRedistSelfImportRejected`, `TestISISRedistRegistrationOrder`, `TestISISRedistMetricBoundary`) plus the QEMU `isis-redist-frr` interop scenario |
| `isis-ipv6.ci` | A dual-stack `isis { ... }` config (per-interface `address-family ipv6-unicast`) validates through the real YANG schema, the IPv6 SPF + install pass is wired (a second `BuildRoutesV6` over the shared tree feeding an IPv6-family Loc-RIB Installer), and `show isis route ipv6` returns an empty list with no adjacency (no phantom IPv6 routes). Live dual-stack flow (TLV 232 link-local in the Hello, TLV 236 in the LSP, an fe80:: next-hop IPv6 route in the kernel, IPv6 redistribution both ways) is proven by the unit tests (`TestISISOriginateTLV236`, `TestISISOriginateTLV232Scope`, `TestISISProtocolsSupportedDualStack`, `TestISISIIHTLV232LinkLocal`, `TestISISIPv6SPFNextHop`, `TestISISIPv6LinkLocalNextHop`, `TestISISIPv6RouteLocRIBInsert`, `TestISISIPv6MetricAboveMaxIgnored`, `TestISISRedistConsumerIPv6`, `TestISISRedistSourceIPv6`) plus the QEMU `isis-dualstack-frr` interop scenario (isis-13) |
| `isis-show.ci` | The full `show isis <noun>` / `clear isis <action>` surface dispatches through the engine: with a NET and a passive interface (so the engine starts without `CAP_NET_RAW`), `show isis neighbor` returns an empty array, `show isis database` carries the own LSP, `show isis database detail` expands TLVs, `show isis route` is present, `show isis interface` reports the passive circuit, `show isis hostname` maps the local System ID to the configured name (TLV 137), `show isis spf-log` is present, and `clear isis adjacency` / `clear isis counters` return a status payload (no `unknown command`). Proxy arg-rejection and render shape are unit-tested in `cmd_show_test.go` / `show_test.go` |
| `isis-doctor.ci` | The IS-IS doctor codes are explainable (`ze explain doctor-isis-net-missing` / `doctor-isis-system-id-mismatch` / `doctor-isis-raw-socket`), and `ze doctor --json` against a config whose `system-id` disagrees with the NET emits `doctor-isis-system-id-mismatch`. The net-missing and raw-socket firing paths are unit-tested (`doctor_test.go`, isis-3 `transport/doctor_test.go`) |

The seven FRR interop scenarios (`test/interop/scenarios/isis-{p2p,lan-dis,dualstack,auth,convergence,redist,purge-reorig}-frr`)
exercise the protocol against a live FRR `isisd` over the shared Docker bridge: P2P
adjacency + convergence, LAN DIS election + pseudo-node LSP, IPv4+IPv6 reachability,
HMAC-MD5 authentication, link-down reconvergence, IS-IS<->BGP redistribution, and
re-origination above a purge of Ze's own LSP (ISO/IEC 10589 clause 7.3.16.4 c):
`isis-purge-reorig-frr` floods a purge of Ze's own LSP at a sequence Ze never
issued, and FRR must end up holding Ze's LSP at a HIGHER sequence with a live
holdtime, with Ze still in FRR's Level-1 topology.
They are the goal-validation evidence for the IS-IS umbrella and run under the
Linux Docker interop harness (`test/interop/daemons` has `isisd=yes`), not on darwin.

Run with `./le functional isis`. The offline wire-decode suite is separate:
`./le functional isis-wire`.
<!-- source: internal/le/interoplab/bgp/check_special.go -- checkISISOwnLSPPurge -->
<!-- source: internal/test/cli/register.go -- isis CI suite registration -->
<!-- source: test/isis/isis-config.ci -- config validation evidence -->
<!-- source: test/isis/isis-adjacency.ci -- adjacency config-surface evidence -->
<!-- source: test/isis/isis-flooding.ci -- flooding evidence -->
<!-- source: test/isis/isis-auth.ci -- authentication evidence -->
<!-- source: test/isis/isis-ipv6.ci -- dual-stack evidence -->

The OSPF FRR interop scenarios exercise the unified OSPF engine against a live FRR
`ospfd`/`ospf6d` over the shared Docker bridge. The OSPFv3 set proves a P2P and a broadcast
(DR/BDR + Network-LSA + Link-LSA) adjacency, ABR inter-area summary install
(`ospfv3-multiarea-frr`: FRR installs Ze's area-1 prefix `2001:db8:a1::/64` as an inter-area
route), stub-area ABR default origination (`ospfv3-stub-frr`: FRR installs the Ze-originated
`::/0`, no Type-5 leak), broadcast DR route install (`ospfv3-broadcast-frr`), and ASBR
redistribution into AS-External / NSSA Type-7. Route assertions on the shared LAN use a
passive dummy interface carrying a unique global prefix that exists only on Ze, since a
segment prefix connected on both sides cannot be asserted as a route. They run under the
Linux Docker interop harness only (raw IPv6 proto 89 over `ff02::5`), not on darwin.
<!-- source: internal/le/interoplab/bgp/checkers.go -- OSPFv3 scenario checks -->
<!-- source: internal/le/interoplab/bgp/check_extras.go -- OSPFv3 database checks -->
<!-- source: internal/plugins/ospf/origination_v6_stub.go -- v6 stub default origination (v6ApplyAreaTypePolicy) -->

---

## CLI Reference

Common suite form:

```
ze-test bgp <suite> [options] [ID_OR_NAME...]
ze-test <suite> [options] [ID_OR_NAME...]
```

Common selection options:

| Option | Meaning |
|--------|---------|
| `--list`, `-l` | List available tests as `N/TOTAL ID NAME` with one-based ids |
| `--all`, `-a` | Run all tests in the suite |
| `--start ID` | Run the test matching `ID` or exact name, then every later test in suite order |
| `--pattern TEXT` | Run tests whose id, name, or path contains `TEXT` |
| `ID_OR_NAME...` | Run only the listed tests |

Common run options:

| Option | Meaning |
|--------|---------|
| `-t`, `--timeout N` | Timeout per test for `.ci` BGP/VPP runners |
| `-p`, `--parallel N` | Max concurrent tests for `.ci` runners |
| `-v`, `--verbose` | Show verbose failure output |
| `-q`, `--quiet` | Minimal output |
| `-s`, `--save DIR` | Save logs for BGP/VPP `.ci` runners |
| `--port N` | Base port for BGP/VPP `.ci` runners |
| `-c`, `--count N` | BGP `.ci` stress mode, run each selected test N times |
| `--server ID`, `--client ID` | BGP `.ci` manual split-debug modes |
<!-- source: internal/test/cli/cmd_bgp.go -- zeTestParseRunCLI, zeTestPrintRunUsage -->
<!-- source: internal/test/cli/ci_runner.go -- RunCISubcommand -->
<!-- source: internal/test/cli/cmd_vpp.go -- zeTestParseVPPCLI, zeTestPrintVPPUsage -->
<!-- source: internal/test/cli/cmd_editor.go -- cmdEditorMain -->
<!-- source: internal/test/cli/cmd_web.go -- cmdWebMain -->

### Replaying a captured BGP session

`ze-test replay` is a tool root, not a suite: it takes one capture file instead
of test ids.

```
ze-test replay [--json] [--local-as N] [--peer-as N] [--router-id N] <capture-file|->
```

A capture file of `-` is read from stdin, so a capture piped from the machine
that produced it needs no temporary file.

The file comes from a peer with `capture { enabled true; }` set
(`docs/guide/configuration.md`, Protocol Event Capture). Replay opens a
`Session`, gives it a fake clock, and feeds every captured message through
`Session.ReadAndProcess`, the same function the daemon's read loop calls. It
then reports the FSM states, the prefixes each UPDATE announced and withdrew,
the config operations the capture recorded, and any NOTIFICATION the session
sent back.

The session identity comes from the capture header. The three flags override it
and exist for a file written before the header carried one; a wrong local AS
turns an iBGP session into an eBGP one and the replay stops reproducing the run.

`--json` emits the report as one object, which is what a developer diffs between
two builds to bisect a fix.

<!-- source: internal/test/cli/cmd_replay.go -- runReplay, replayIdentity -->
<!-- source: test/plugin/bgp-capture-replay.ci -- capture then replay, end to end -->

---

## Test IDs

Each test has a one-based decimal id printed by `--list`. The id is the stable
selector to pass positionally or to `--start`; in a full-suite list or run it
matches the `N` in `N/TOTAL`. A line like `42/120  42  bgp-open` means "test id
42 is the 42nd test in a 120-test suite."
<!-- source: internal/test/runner/record.go -- GenerateNick -->
<!-- source: internal/test/runner/record_collection.go -- Tests.List -->

**Examples:**
```bash
# Run test id 4
ze-test bgp encode 4

# Run tests 1, 41, and 42
ze-test bgp encode 1 41 42

# Resume at id 42
ze-test bgp encode --start 42
```

---

## .ci File Format

The `.ci` file is the **source of truth** for bidirectional testing. Full format documentation: [`docs/architecture/testing/ci-format.md`](architecture/testing/ci-format.md)

```
# Tmpfs: embed config inline
tmpfs=test.conf:terminator=EOF_CONF
peer test-peer { remote { ip 127.0.0.1; } ... }
EOF_CONF

# Options
option=file:path=test.conf
option=asn:value=65000

# Commands and expectations
cmd=api:conn=1:seq=1:text=update text nhop set 10.0.1.1 nlri ipv4/unicast add 10.0.0.0/24
expect=bgp:conn=1:seq=1:hex=FFFF...
expect=json:conn=1:seq=1:json={...}
```

### Line Types

| Action | Example | Description |
|--------|---------|-------------|
| `tmpfs=` | `tmpfs=file.conf:terminator=EOF` | Embed file content inline |
| `option=` | `option=file:path=test.conf` | Test configuration |
| `cmd=` | `cmd=api:conn=1:seq=1:text=...` | API command |
| `cmd=background:...:name=` | `cmd=background:seq=1:exec=ze -:name=responder` | Start a background process under a handle |
| `cmd=stop:` | `cmd=stop:seq=3:name=responder[:signal=kill\|term]` | Terminate a named background process mid-test (SIGKILL default) |
| `expect=bgp:` | `expect=bgp:conn=1:seq=1:hex=...` | Expected wire bytes |
| `expect=json:` | `expect=json:conn=1:seq=1:json=...` | Expected JSON |
| `expect=stdout:` | `expect=stdout:contains=text` | Substring match in stdout |
| `expect=stderr:` | `expect=stderr:pattern=...` or `contains=...` | Regex or substring match in stderr |
| `expect=syslog:` | `expect=syslog:pattern=...` | Regex pattern in syslog |
| `reject=stderr:` | `reject=stderr:pattern=...` | Fail if stderr matches regex |
| `reject=syslog:` | `reject=syslog:pattern=...` | Fail if syslog matches regex |
| `action=notification:` | `action=notification:conn=1:seq=1:text=...` | Send NOTIFICATION |
| `action=rewrite:` | `action=rewrite:conn=1:seq=2:source=config2.conf:dest=ze-bgp.conf` | Rewrite config file |
| `action=sighup:` | `action=sighup:conn=1:seq=2` | Send SIGHUP to daemon |
| `action=sigterm:` | `action=sigterm:conn=1:seq=2` | Send SIGTERM to daemon |

### Waiting without sleep (the quiesce barrier)

Prefer a completion signal over a fixed `time.sleep` (the ci-sleep ratchet in
`internal/le/doc/wiring.Answer` counts sleeps in `test/**/*.ci`). Existing
options: `ze_api.wait_for_event` / `wait_for_shutdown` / `wait_for_post_startup`
(block on a delivered event / bye / post-startup RPC), `expect=event`, and
`http=wait`.

For "I changed something, now assert its downstream effect", use the **quiesce
barrier**: `ze_api.quiesce()` (or the `request quiesce` command) sends
`ze-system:quiesce`, which blocks until every registered subsystem has drained
its pending async work (the BGP forward pool among them) and then replies. So
`send(route); quiesce()` guarantees the route is on the peer wire with no sleep.
See `docs/architecture/api/commands.md` "Quiesce Barrier".

<!-- source: internal/test/fixture/misc_fixture_observers.go -- quiesce -->
<!-- source: internal/component/plugin/server/quiesce.go -- ze-system:quiesce handler -->

#### Payload-predicate waits

When the wait is "block until an *observed payload* matches a condition" (not a
one-shot barrier), use a payload-predicate wait so the test blocks exactly until
the condition holds instead of a guessed duration:

- `ze_api.wait_until(predicate, attempts=20, delay=0.25)` — poll an arbitrary
  `predicate()` (e.g. kernel FIB state via `ip route show`) until true.
- `api.dispatch_until(cmd, predicate)` — re-dispatch `cmd` until
  `predicate(result)` is true; returns the winning result dict.
  `dispatch_until_done(cmd)` is the `status=="done"` special case.
- `api.wait_for_event(timeout, predicate)` — return the first delivered event
  whose decoded (JSON) form satisfies `predicate` (`predicate=None` keeps the
  legacy "first event of any type" behavior).

The symmetric first-class form for `.ci` engine steps is the declarative
predicate grammar `expect=output:matches=<regexp>` / `absent=<substr>` /
`json=<path>=<value>` (and `expect=stream:matches=`); see
`docs/architecture/testing/ci-format.md` "Engine Steps".

<!-- source: internal/test/fixture/fixture.go -- Poll, Dispatch -->
<!-- source: internal/test/runner/engine_steps.go -- parseEngineExpectContains, engineOutputSatisfied -->

#### Reading a record answer

Every `dispatch-command` answer is a sequence of lines. The Go SDK and fixture
drivers read the document head, records, and terminator through the framed RPC
connection. The record-answer fixtures use `sdk.Records`, so the engine and the
test share the production framing implementation.
<!-- source: internal/test/fixture/plugin_fixture_11.go -- record answer fixture -->
<!-- source: pkg/plugin/rpc/answer_write.go -- WriteDocumentAnswer -->

#### Declaring a pipe alias

`ze_api.declare_pipe(command, name, expansion, description="")` adds one entry to
the `pipes` list of the Stage 1 message. It names a CLI pipe alias over one of
this plugin's own commands, so call it after `declare_command` for that command
and before `declare_done`.

The alias only SELECTS among the keys the command answers with, so the fake
plugin must answer with a payload that carries them. Register the answer with
`api.on_execute_command(handler)`. Type the alias with `ze cli -c`, which is
where the daemon resolves the chain. `ze cli` with no command argument resolves
it in the client process, where no declared alias is registered.

A bad declaration fails the plugin's whole Stage 1 registration. The engine then
stops the plugin before it can report the refusal, so assert it in the daemon
log.
`test/plugin/plugin-pipe-alias.ci` drives the accepted path, and
`test/plugin/plugin-pipe-alias-collision.ci` drives the refusal.
<!-- source: internal/test/fixture/plugin_fixture_11.go -- pipe alias fixtures -->
<!-- source: pkg/plugin/rpc/types.go -- PipeDecl -->

#### Writing a record answer: the Go SDK test plugin

`ze-test record-plugin` is a Go SDK plugin whose command handlers answer with a
walk. A `.ci` spawns it the way it spawns the engine-step executor:

```
plugin {
	external record-plugin {
		run "ze-test record-plugin"
		encoder json
	}
}
```

It registers six commands, and each one is one property of the record path.

| Command | What it produces |
|---------|------------------|
| `show test records walk` | 300 rows of 60000 bytes, so the walk streams AND the collection is wider than one 16 MB wire message |
| `show test records fault` | 12 rows, one of them wider than any line can carry, so the answer reports 11 applied and 1 rejected |
| `show test records document` | 2 rows that each fit one line and collapse into a document that does not, so the DOCUMENT is what the answer rejects |
| `show test engine answer` | what the plugin read from the engine's own streamed answer to `system command list` |
| `show test records table` | 300 rows against a declared column schema, so the head says `tab` and each row is a positional array |
| `show test records object` | the same 300 rows with no schema declared, so the head says `map` and each row carries its own names |
<!-- source: internal/test/cli/cmd_record_plugin.go -- cmdRecordPlugin, recordRows, recordColumnRows, recordTableColumns, engineAnswerReader -->

The last two are a PAIR, and neither means anything alone. They answer the same
data through handlers that differ only in `plugin.Records.Fields`, so the
document an operator receives from one is the document they receive from the
other. That equality is what says the head carried the column names and the
consumer zipped each positional row against them.

Six `.ci` files drive it from the operator's seat over `ze cli`.

| Test | Proves |
|------|--------|
| `test/plugin/plugin-owned-command-streams.ci` | every row of a plugin-owned command's walk reaches the operator, one line each |
| `test/plugin/plugin-reads-engine-answer.ci` | a plugin reads a streamed engine answer through `Plugin.DispatchCommandAnswer` and acts on it |
| `test/plugin/plugin-command-partial-fault.ci` | applied rows and rejected rows reach the operator together |
| `test/plugin/plugin-command-document-too-wide.ci` | a collapsed document no line can carry is rejected by name, and the answer still reaches its terminator |
| `test/plugin/answer-payload-unchanged.ci` | every row an operator receives, on both wire shapes, is byte for byte the bytes `recordRow.AppendTo` wrote |
| `test/plugin/stream-answer-renders-table.ci` | a declared column schema reaches the head, the rows travel as values alone, and the operator renders them as a table |

`answer-payload-unchanged.ci` is the one that reads the PAYLOAD rather than the
frame. `recordRow.AppendTo` writes each row with no marshaler, so the test builds
the expected bytes from the two numbers the walk is sized by. It then compares
them literally. What it exists to catch is a frame that parses cleanly and hands
the operator a byte the producer never wrote.

`| raw` is the rendering it uses. `| ndjson` decodes and re-encodes each record
on purpose, which alphabetizes the producer's key order.
<!-- source: internal/component/command/render_records.go -- writeRecordJSON -->

The row sizes are load-bearing, and each test states its own preconditions. The
two readings of a walk are the same document by design, so a fixture of ordinary
rows would pass with the record path removed. A collection wider than one wire
message, and a row wider than one line, are the two payloads whose readings
differ. A short walk whose rows each fit and whose collapse does not is the
third, and it separates the bound on a row from the bound on the document.
<!-- source: pkg/plugin/rpc/answer_write.go -- WriteRecordAnswer, boundedRecord -->

How much of a walk a test READS is load-bearing too. The engine holds 256 answer
lines for a consumer that has not been scheduled yet, and abandons the answer
past that. A chain reading a whole streamed plugin answer is therefore at the
mercy of the scheduler, and fails under load.

`stream-answer-renders-table.ci` reads through `| first 100`, which stops inside
the 256 the queue guarantees. It measures the column schema it was written for
rather than the queue. The bound is recorded in
`plan/journal/bound-too-small-for-its-own-burst.md`.
<!-- source: pkg/plugin/rpc/mux.go -- answerQueueDepth, ErrAnswerQueueFull -->

#### Reading the exec channel's answer frame

The SSH exec channel puts the rendering on stdout and the answer FRAME on
stderr. `ze cli` reads that frame and shows an operator the rendering. A test
that drives `ze cli` therefore sees what the frame became, never the frame.

To read the frame bytes, drive the daemon with OpenSSH instead:

1. Generate an ed25519 key pair in the test.
2. Put the public half in the user's `public-keys` block.
3. Run `ssh -i ./testkey -p <port> <user>@127.0.0.1 "<command>"`. Keep stdout
   and stderr apart.
4. Add `-o LogLevel=ERROR`. Without it the client's own known-hosts notice
   lands on the same stream.

`ssh` parses nothing and declares nothing. Its stderr is the daemon's frame,
byte for byte.

`test/plugin/exec-answer-unconditional.ci` drives the native frame checker. It
parses lengths as byte counts and rejects malformed document, stream, error,
and unknown-command frames.
<!-- source: internal/test/fixture/plugin_fixture_06.go -- fixture06FrameCheck -->

| Test | Proves |
|------|--------|
| `test/plugin/exec-answer-unconditional.ci` | a client that declares nothing is framed, with the kind at offset zero and every count a count of bytes |

<!-- source: internal/component/ssh/answer.go -- answerFrame, writeExecAnswer -->
<!-- source: test/plugin/ssh-pubkey-auth.ci -- the key pair this reuses -->

### Tmpfs (Virtual File System)

Tmpfs allows embedding config files directly in `.ci` files:

```
tmpfs=peer.conf:terminator=EOF_CONF
peer test-peer {
    remote {
        ip 127.0.0.1;
        as 65533;
    }
    local-as 65533;
}
EOF_CONF

option=file:path=peer.conf
```

At runtime, Tmpfs files are written to a temp directory. This enables self-contained tests without separate `.conf` files.

### Directive Placement

Test directives belong to one of two scopes:

| Scope | Consumer | Placement |
|-------|----------|-----------|
| Test runner | The `ze-test` process itself (seeds `proc.Env`, drives orchestration) | File level, outside any `stdin=...` block |
| `ze-peer` stdin | The `ze-peer` subprocess reading its stdin at runtime | Inside the `stdin=peer:terminator=X` block |

Only `expect=bgp:...`, `expect=json:...`, `expect=exit:...`, `action=...`, `option=timeout:...`, `option=open:...`, `option=update:...`, `option=tcp_connections:...`, and `option=conn_map:...` are valid inside `stdin=peer:` blocks. The `option=timeout`, `option=open`, `option=update`, `option=tcp_connections`, and `option=conn_map` forms are consumed by `ze-peer` from its stdin and must stay in-block so the subprocess receives them.

`option=conn_map:value=router-id` sorts each accepted connection batch by the BGP router ID in OPEN. `option=conn_map:value=remote-ip` sorts each batch by the TCP source address, which stays stable when reload tests intentionally change router IDs. With `conn_map`, `option=tcp_connections:value=N` is the batch size; if expectations remain after one batch, `ze-peer` accepts another batch and continues with the next `conn=N` rules.

**`option=env:var=K:value=V` is consumed by the test runner (it appends to `proc.Env` when spawning `ze`/`ze-peer`/helper processes) and therefore MUST live at file level, outside any `stdin=peer:` block.** Placing it inside the block used to be silently dropped — the directive would be handed to `ze-peer`, which ignores it, and the target process would never see the variable. The parser now rejects this at `bin/ze-test <suite> -list` time with an error naming the exact directive and pointing at this section.

<!-- source: internal/test/runner/record_parse.go — parseAndAdd peer-block loop -->

**Correct placement:**

```
# option=env belongs ABOVE the stdin=peer block.
option=env:var=ze.log.bgp.server:value=debug

stdin=peer:terminator=EOF_PEER
option=timeout:value=15s
option=open:value=inspect-open-message
expect=bgp:conn=1:seq=1:hex=FFFF...
EOF_PEER
```

**Rejected at parse time:**

```
stdin=peer:terminator=EOF_PEER
option=timeout:value=15s
option=env:var=ze.log.bgp.server:value=debug   # <-- PARSE ERROR
expect=bgp:conn=1:seq=1:hex=FFFF...
EOF_PEER
```

### Logging Tests

Tests can verify logging behavior using `option=env:`, `expect=stderr:`, `reject=stderr:`, and `expect=syslog:`.

**Example: Verify server subsystem logs to stderr**
```
option=file:path=mytest.conf
option=env:var=ze.bgp.log.server:value=debug

expect=bgp:conn=1:seq=1:hex=FFFF...
expect=stderr:pattern=subsystem=server
```

**Example: Verify DEBUG messages are filtered at INFO level**
```
option=file:path=mytest.conf
option=env:var=ze.bgp.log.server:value=info

expect=bgp:conn=1:seq=1:hex=FFFF...
reject=stderr:pattern=level=DEBUG
```

**Example: Verify syslog backend**
```
option=file:path=mytest.conf
option=env:var=ze.bgp.log.server:value=debug

expect=bgp:conn=1:seq=1:hex=FFFF...
expect=syslog:pattern=subsystem=server
```

When `expect=syslog:` is present, the test runner automatically:
1. Starts a test-syslog UDP server on a dynamic port
2. Sets `ze.log.backend=syslog` and `ze.log.destination=127.0.0.1:<port>`
3. Validates patterns against captured syslog messages after test

#### Syslog Testing Architecture

```
┌──────────────────────────────────────────────────────────────────┐
│                      TEST RUNNER (runner.go)                      │
│                                                                   │
│  1. Parse .ci file                                                │
│     └── Found: expect:syslog:subsystem=server                     │
│                                                                   │
│  2. Start testsyslog server (UDP, dynamic port)                   │
│     └── syslog.New(0).Start(ctx) → port 54321                 │
│                                                                   │
│  3. Auto-inject env vars for ze-bgp:                               │
│     └── ze.bgp.log.backend=syslog                                  │
│     └── ze.bgp.log.destination=127.0.0.1:54321                     │
│     └── ze.bgp.log.server=debug  (from option:env:)                │
│                                                                   │
│  4. Start ze bgp with config                                       │
└──────────────────────────────────────────────────────────────────┘
                                │
                                ▼
┌──────────────────────────────────────────────────────────────────┐
│                           ZEBGP                                   │
│                                                                   │
│  slogutil.Logger("server") reads env vars:                        │
│    - ze.bgp.log.server=debug → enabled at DEBUG                    │
│    - ze.bgp.log.backend=syslog → use syslog handler                │
│    - ze.bgp.log.destination=127.0.0.1:54321 → UDP target           │
│                                                                   │
│  logger.Debug("msg", "subsystem", "server", ...)                  │
│         │                                                         │
│         ▼                                                         │
│  slog.TextHandler → syslog.Writer → UDP packet                    │
└──────────────────────────────────────────────────────────────────┘
                                │
                      UDP: "<14>... subsystem=server ..."
                                │
                                ▼
┌──────────────────────────────────────────────────────────────────┐
│                      TESTSYSLOG SERVER                            │
│                                                                   │
│  Receives: "<14>Jan 19 ... ze-bgp: level=DEBUG subsystem=server"   │
│  Stores in: srv.messages[]                                        │
└──────────────────────────────────────────────────────────────────┘
                                │
                                ▼
┌──────────────────────────────────────────────────────────────────┐
│                   VALIDATION (after test)                         │
│                                                                   │
│  validateLogging() checks each expect:syslog: pattern:            │
│    if !syslogSrv.Match("subsystem=server"):                       │
│        return error("pattern not found")                          │
│                                                                   │
│  Match() does regex search across all captured messages           │
└──────────────────────────────────────────────────────────────────┘
```

**Key components:**

| Component | Location | Purpose |
|-----------|----------|---------|
| `syslog.Server` | `internal/test/syslog/` | UDP server capturing syslog messages |
| `option:env:` | `.ci` file | Sets env vars (e.g., `ze.bgp.log.server=debug`) |
| `expect:syslog:` | `.ci` file | Regex pattern to match in captured messages |
| Auto-injection | `runner.go` | Adds `backend=syslog` + `destination=host:port` |
| `validateLogging()` | `runner.go` | Checks patterns after test completes |

<!-- source: internal/test/syslog/testsyslog.go -- syslog UDP server -->
<!-- source: internal/test/runner/runner.go -- auto-injection and validateLogging -->

**Message format:** Syslog messages use Go's `slog.TextHandler` format with syslog framing:
```
<priority>timestamp hostname ze-bgp: level=DEBUG subsystem=server msg="..." key=value
```

Patterns should match the key=value pairs (e.g., `subsystem=server`, `level=DEBUG`).

### Raw Message Format

```
MARKER:LENGTH:TYPE:PAYLOAD
```

- MARKER: 16 bytes (all FF)
- LENGTH: 2 bytes (total message length)
- TYPE: 1 byte (1=OPEN, 2=UPDATE, 3=NOTIFICATION, 4=KEEPALIVE)
- PAYLOAD: Variable

### JSON Validation Format

The `N:json:` lines use ZeBGP plugin format (not ExaBGP envelope format):

**Unicast:**
```json
{"meta":{"version":"1.0.0","format":"ze-bgp"},"message":{"type":"update"},"origin":"igp","ipv4/unicast":[{"next-hop":"10.0.1.254","action":"add","nlri":["10.0.0.0/24"]}]}
```

**FlowSpec:**
```json
{"meta":{"version":"1.0.0","format":"ze-bgp"},"message":{"type":"update"},"origin":"igp","ipv4/flowspec":[{"action":"add","nlri":{"next-hop":"1.2.3.4","destination-ipv4":["192.168.0.1/32"],"string":"flow destination-ipv4 192.168.0.1/32"}}]}
```

**Supported families:** `ipv4/unicast`, `ipv6/unicast`, `ipv4/flowspec`, `ipv6/flowspec`

**Key differences from ExaBGP envelope:**
- Flat structure (no `neighbor.message.update` nesting)
- `meta.format` = "ze-bgp" (not `exabgp` version)
- Family arrays at top level with `action` field
- FlowSpec: `nlri` is object with components; unicast: `nlri` is string array

**Context fields ignored:** `peer`, `direction` (test-environment dependent)

---

## Test Execution Flow

### Encode Tests

```
1. Runner builds ze + ze-peer to temp dir
2. Starts ze-peer on unique port with .ci expectations
3. Starts ze bgp with config
4. ze bgp connects, sends OPEN, receives OPEN
5. ze bgp sends UPDATE messages (from static routes)
6. ze-peer validates messages against expectations
7. ze-peer prints "successful" or error
```

---

## Display Output

### Progress and per-test lines

During execution, TTY status updates include overall progress, longest-running
test timer, pass/running/fail/timeout counts, and pending ids. Non-TTY logs emit
periodic progress lines while tests are running.

```
progress 12/42 [5/20s] passed 12 running 4 [12, 13, 14, 15] pending 26
5.0s      12/42   4/8  running  12(5.0s), 13(5.0s), 14(5.0s), 15(5.0s)
```

Every completed test emits one line:

```
812ms    13/42  PASS  12  route-refresh-basic
```

| Field | Meaning |
|-------|---------|
| `progress N/TOTAL` | Completed tests, including skipped tests, out of selected tests |
| `[N/Ms]` | Longest running test: N seconds elapsed, M timeout |
| `running N [IDs]` | N tests currently executing, ids shown when at most five are running |
| `13/42` | One-based run number out of selected total for a completion line |
| `PASS  12  route-refresh-basic` | Result, decimal id, and test name |
<!-- source: internal/test/runner/display.go -- Status, TestFinished -->

### Section Header

Each test suite is framed by a section header, including top-level suites such
as `ui`, `managed`, `l2tp`, `firewall`, `policy`, `web`, and `install`:
```
═══════════════════════ encode ════════════════════════════════════════════════
```
<!-- source: internal/test/cli/cmd_bgp.go -- BGP suite headers -->
<!-- source: internal/test/cli/ci_runner.go -- top-level .ci suite headers -->

### Summary

On success:
```
pass  42/42  100.0%  3.2s
```

On failure:
```
fail  40/42  95.2%  3.2s  failed 2 [4, 9]  timeout 1 [12]
```

| Field | Format | Meaning |
|-------|--------|---------|
| Verdict | `pass` or `fail` | Green if all passed, red otherwise |
| Ratio | `N/M` | Passed / total completed tests |
| Rate | `N.N%` | Pass percentage |
| Time | `N.Ns` or `Nms` | Wall-clock elapsed |
| Failed | `failed N [ids]` | Only shown when greater than zero |
| Timeout | `timeout N [ids]` | Only shown when greater than zero |
<!-- source: internal/test/runner/display.go -- Summary -->

### Verify Failure Groups

In `ZE_VERIFY_MODE=1`, failed `.ci` suites emit native failure groups before
the full `TEST FAILURE` blocks. A group records the suite label, group id,
failure kind, related ids, compact summary, exact rerun command, detail log,
and parallelization hint. The top-level verify runner copies only bounded group
metadata into `tmp/ze-verify-failures.log`; the full evidence remains in the
stage log.
<!-- source: internal/test/runner/failure_group.go -- native functional groups -->
<!-- source: internal/test/runner/report.go -- detailed failure reports -->
<!-- source: internal/test/runner/display.go -- summary and debug hints -->

### Stress Test Mode

Use `--count N` (`-c N`) to run tests multiple times for benchmarking or detecting flaky tests:

```bash
# Run test C 10 times with timing
ze-test bgp plugin -c 10 C

# Run all encoding tests 5 times
ze-test bgp encode -c 5 -a
```

**Per-iteration timing** is shown during execution:
```
==> Iteration 1/10
==> Iteration 1: 5.2s

==> Iteration 2/10
==> Iteration 2: 4.8s
```

**Summary** shows per-test stats and overall timing:
```
STRESS TEST SUMMARY
═══════════════════════════════════════════════════════════════════════════════
Iterations: 10

ID       Pass   Fail    T/O        Min        Avg        Max    Rate
---------------------------------------------------------------------------
0          10      0      0      108ms      332ms      764ms  100.0%
1           8      2      0      115ms      400ms      900ms   80.0%
═══════════════════════════════════════════════════════════════════════════════
Iteration timing: min=4.8s avg=5.1s max=5.7s total=51.2s
Total: 20 iterations, 18 passed, 2 failed, 0 timed out (90.0% pass rate)
```

**Key metrics:**
- Per-test min/avg/max duration
- Per-test pass rate (color-coded: green=100%, yellow≥50%, red<50%)
- Iteration timing: min/avg/max/total wall-clock time

---

## Debugging Tests

### Run a single test

Native suite actions isolate their binaries. For one selection, queue the
runner through `./le job run`; set `ZE_BIN` and `ZE_TEST_BIN` only when testing
an explicitly pinned pair.

BGP suites use the `ze-test bgp <suite>` command shape:

```bash
ze-test bgp encode --timeout 60s --verbose 4
ze-test bgp plugin --server 4
ze-test bgp plugin --client 4
```

Top-level `.ci` suites use `ze-test <suite>` without the `bgp` prefix:

```bash
ze-test ui 4
ze-test managed 4
ze-test firewall 4
```

Resume from the last printed id after a timeout or interrupted run:

```bash
ze-test bgp plugin --start 42
ze-test ui --start 42
ze-test editor --start 42
```
<!-- source: internal/test/runner/selection.go -- Selection.Start -->

The compact verify index emits the smallest useful rerun command for each
failure group. Use that command before widening scope.

### Decode message bytes

```bash
# Decode UPDATE payload
ze bgp decode update 0000001540010100400200400304650165014005040000006400

# Decode full message
ze bgp decode raw FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF002D02...
```

---

## Adding New Tests

### Adding a local-data command

A command registered with `cmdregistry.MustRegisterLocalData` owes runtime
evidence, and `TestEveryLocalDataRegistrationHasAFunctionalCase`
(`internal/component/command/registry`) enforces it: the test derives every
production registration from the Go AST and fails when one has no row. Four
places move together, and three of them carry a count.

| Place | What to add |
|-------|-------------|
| `internal/test/localdatacoverage.Evidence()` | One `{Command, Evidence}` row. The command MUST carry a real pipe, and `Evidence` MUST be the exact registered path |
| The walk in the same file | An assertion over what the command answered. A row with no assertion proves the command ran, not that it answered anything |
| `localdatacoverage.CompletionMarker` and the two totals in `TestLocalDataCoverageEvidenceIsNonVacuousAndComplete` | The new counts |
| `test/ui/pipe-local-command.ci` | The new `COVERED: <evidence> [done]` line, in sorted order, before the completion marker |

The command owes no `wire-methods.snapshot` row, because it registers no RPC.
<!-- source: internal/test/localdatacoverage/localdatacoverage.go -- Evidence, CompletionMarker -->
<!-- source: internal/component/command/registry/local_data_functional_coverage_test.go -- TestEveryLocalDataRegistrationHasAFunctionalCase -->

### Option 1: Tmpfs (Recommended)

Single self-contained `.ci` file with embedded config:

```
# test/encode/mytest.ci
tmpfs=mytest.conf:terminator=EOF_CONF
peer loopback {
    remote {
        ip 127.0.0.1;
        as 1;
    }
    router-id 1.2.3.4;
    local-address 127.0.0.1;
    local-as 1;

    family {
        ipv4/unicast;
    }
    announce {
        ipv4 {
            unicast 10.0.0.0/24 next-hop 1.2.3.4;
        }
    }
}
EOF_CONF

option=file:path=mytest.conf
cmd=api:conn=1:seq=1:text=update text nhop set 1.2.3.4 nlri ipv4/unicast add 10.0.0.0/24
expect=bgp:conn=1:seq=1:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF002D020000001540010100...
```

### Option 2: Separate Files

```
# test/encode/mytest.conf
peer loopback {
    remote {
        ip 127.0.0.1;
        as 1;
    }
    router-id 1.2.3.4;
    local-address 127.0.0.1;
    local-as 1;

    family {
        ipv4/unicast;
    }
    announce {
        ipv4 {
            unicast 10.0.0.0/24 next-hop 1.2.3.4;
        }
    }
}
```

```
# test/encode/mytest.ci
option=file:path=mytest.conf
cmd=api:conn=1:seq=1:text=update text nhop set 1.2.3.4 nlri ipv4/unicast add 10.0.0.0/24
expect=bgp:conn=1:seq=1:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF002D020000001540010100...
```

### Generate expected bytes

Run with ExaBGP first to capture correct bytes, or use `ze bgp decode` to verify.

### Adding Negative Parsing Tests

To test that invalid configs are rejected with specific errors, create a `.ci` file:

```
# test/parse/my-error.ci
stdin=config:terminator=EOF_CONF
bgp {
    peer test-peer {
        remote {
            ip 10.0.0.1;
            as 65002;
        }
        router-id 1.2.3.4;
        local-as 65001;
        # ... invalid configuration ...
    }
}
EOF_CONF

cmd=foreground:seq=1:exec=ze bgp validate -:stdin=config
expect=exit:code=1
expect=stderr:contains=specific error message substring
```

**Regex match** (for variable parts like IPs, line numbers):
```
expect=stderr:regex=peer \d+\.\d+\.\d+\.\d+: route-refresh requires
```

The test passes if:
- `ze bgp validate` exits with code 1
- Stderr contains the expected substring OR matches the regex pattern

### Tagging tests to an RFC requirement

A test that enforces an RFC MUST can bind itself to the requirement so the coverage
gate can see it. Add an `RFC requirement:` tag naming the requirement id and the
polarity it exercises. It works in both test styles:

```go
// RFC requirement: RFC7606-7.1-1 negative -- ORIGIN length 2 is treated as withdraw
func TestRFC7606MalformedOriginLength(t *testing.T) { ... }
```

```
# test/plugin/rfc7606-withdraw.ci  -- must be a '#' comment at the start of a line
# RFC requirement: RFC7606-7.1-1 negative
```

- `polarity` is mandatory and must be `positive` or `negative`; a gated MUST needs
  both. Text after `--` is an optional note.
- In `.ci` files the tag is only recognised as a line-start `#` comment; `#` inside a
  `terminator=` block is file content, not a tag, and is skipped.
- On a table-driven test, put the tag on the line of the enforcing case, not only on
  the function, so deleting that case re-opens the coverage.
- Once tagged, a test's behaviour must not be edited without user approval. The
  native weakening check blocks it. Fix the code, not the test.
  <!-- source: internal/le/hookruntime/writeedit.go -- writeWeakening -->

`./le rfc check` verifies every enrolled MUST has its pair of tags, or a reasoned
annotation. It also verifies that both generated outputs are fresh. Those outputs are
`rfc/requirements/<stem>.md`, the requirement → test map for one RFC, and
`ai/RFC-REQUIREMENTS.md`, the index over them.
See `docs/contributing/rfc-implementation-guide.md` §9.7 and `ai/skills/ze-rfc.md`.

---

## Architecture

### Package: `internal/test/runner/`

| File | Purpose |
|------|---------|
| `color.go` | TTY-aware ANSI colors |
| `decode.go` | BGP message decoding for failure reports |
| `display.go` | Live progress display |
| `json.go` | JSON validation: transform envelope → plugin format |
| `limits.go` | ulimit check and auto-raise |
| `ports.go` | Dynamic port range allocation |
| `record.go` | Test record with state machine, Tmpfs file storage |
| `report.go` | AI-friendly failure reports |
| `runner.go` | Test execution engine, Tmpfs runtime support |
| `stress.go` | Iteration stats and timing for -c/--count |
| `timing.go` | Per-test timing baseline with auto-timeout |
| `timing_test.go` | Timing baseline tests |
| `parallel.go` | Parallel test execution |
| `tmpfs_test.go` | Tmpfs parsing integration tests |

<!-- source: internal/test/runner/color.go -- ANSI colors -->
<!-- source: internal/test/runner/decode.go -- BGP message decoding -->
<!-- source: internal/test/runner/display.go -- live progress display -->
<!-- source: internal/test/runner/json.go -- JSON validation -->
<!-- source: internal/test/runner/limits.go -- ulimit check -->
<!-- source: internal/test/runner/ports.go -- port allocation -->
<!-- source: internal/test/runner/record.go -- test record state machine -->
<!-- source: internal/test/runner/report.go -- failure reports -->
<!-- source: internal/test/runner/runner.go -- test execution engine -->
<!-- source: internal/test/runner/stress.go -- stress iteration stats -->
<!-- source: internal/test/runner/timing.go -- per-test timing baseline -->
<!-- source: internal/test/runner/parallel.go -- parallel test execution -->

### Package: `internal/test/tmpfs/`

| File | Purpose |
|------|---------|
| `tmpfs.go` | Tmpfs parser and writer. `WriteTo` is the one writer, so every suite honors `mode=` |
| `limits.go` | Configurable limits from environment |
| `security.go` | Path validation (traversal, escape) |

### Entry Point: `internal/test/cli/*.go`

<!-- source: internal/test/cli/register.go -- test runner entry point -->
<!-- source: internal/test/cli/cmd_bgp.go -- bgp test subcommand -->
<!-- source: internal/test/cli/cmd_syslog.go -- syslog server subcommand -->
<!-- source: internal/test/mock/rpki/rpki.go -- RPKI mock RTR subcommand -->

Subcommand-based CLI with `bgp` for BGP test execution, `syslog` for syslog server, and `rpki` for deterministic RPKI mock RTR server.

### ze-test rpki

Deterministic RTR (RFC 8210) cache server for RPKI functional tests. Auto-generates VRPs for all /8 prefixes based on the first octet modulo 3:

| First octet % 3 | VRP | Result (AS 65001) |
|-----------------|-----|-------------------|
| 0 | ASN=65001, maxLen=/32 | Valid |
| 1 | ASN=65099, maxLen=/32 | Invalid |
| 2 | No VRP | NotFound |

Usage: `ze-test rpki --port 3323 [--valid-asn 65001] [--invalid-asn 65099]`

### ze-test irr

Deterministic IRR whois server for firewall and BGP IRR functional tests. It
answers RPSL `!i` (AS-SET expansion) and `!a4`/`!a6` (prefix lookup) queries for
one known AS-SET, `AS-TEST`, and answers `D` (key not found) for everything else.

| Query | Reply |
|-------|-------|
| `!iAS-TEST` | members AS65001, AS65002, AS65003 |
| `!a4AS-TEST` | 10.0.0.0/24, 10.0.1.0/24, 172.16.0.0/16 |
| `!a6AS-TEST` | 2001:db8::/32 |
| anything else | `D` |

`--empty-after-first` answers each query once with its data and then with `D`
forever after. It models an IRR server that has a bad minute after a good
refresh, which is the case ze must not let empty a live filter.

Usage: `ze-test irr --port 4343 [--empty-after-first]`

<!-- source: internal/test/mock/irr/irr.go -- IRR mock whois subcommand -->

### Security

- Path traversal protection on `option:file:` fixture paths
- Process isolation via `Setpgid`
- Context timeouts on all execution
- Dynamic port allocation prevents conflicts

### ExaBGP Compatibility Test Ports

ExaBGP compatibility tests (`./le functional exabgp-test`) use OS-assigned
dynamic ports. The compiled `ze-test interop-bgp exabgp-server` personality
binds port 0 and prints `PORT <N>`. The Go runner passes that port to the
migrated Ze client. Use `--server ID --port N` and `--client ID --port N` for
split-terminal debugging.
<!-- source: internal/le/interoplab/bgp/exabgp_server.go -- wire server and PORT readiness -->
<!-- source: internal/test/cli/cmd_exabgp.go -- waitExaBGPPort and split debug modes -->

### ExaBGP Verify Output

The compatibility runner is integrated into `ze-test`, so it uses the same
`--list`, `--all`, `--start`, `--pattern`, per-test result lines, progress, and
summary format as the other functional suites. The public action is:

```bash
./le functional exabgp-test
```
<!-- source: internal/test/cli/cmd_exabgp.go -- native selection and progress output -->

The stage runs the suite through `uv run --with paramiko`, so `uv` must be on
PATH. `./le setup install` does not install it; the CI runner installs it in
`.github/workflows/verify.yml`. Without it the stage exits 127, and the report
names the child that could not start.
<!-- source: internal/le/functional/exabgp.go -- exaBGPCommands and exaBGPReport.Text -->

---

## Per-Test Timing Baseline

`ze-test` maintains a rolling timing baseline in `tmp/test-timings.json` that enables two features:
<!-- source: internal/test/runner/timing.go -- TimingEntry, LoadTimings, Timings.Save -->

**Auto-timeout:** Each test's timeout is calculated as `min(global_timeout, max(5s, 5x baseline_avg))`. A test that normally takes 500ms gets a 5s timeout instead of the default 15s. This catches hangs in seconds rather than waiting for the global timeout. Explicit `option=timeout:value=` in the `.ci` file always takes precedence.

**Slow detection:** Tests exceeding 2x their baseline average are flagged in the summary output. Investigate performance regressions before ignoring these warnings.

The baseline uses an exponential moving average (EMA, alpha=0.3) and requires 3 samples before it is used for auto-timeout. The timings file is capped at 10 MB; if it grows beyond that, timing data is reset.

---

## Route Delivery Synchronization

Plugin fixtures dispatch `request quiesce` through the Go SDK before they
inspect route delivery.
<!-- source: internal/test/fixture/misc_fixture_observers.go -- quiesce -->
<!-- source: internal/component/plugin/server/quiesce.go -- request quiesce / quiesceAll / registerReactorQuiescer -->
<!-- source: internal/component/bgp/reactor/reactor_api.go -- DrainPeerSync (bgp-peer-sync quiescer) -->
<!-- source: internal/component/bgp/reactor/forward_pool_barrier.go -- forward pool flush barrier -->

**When writing new plugin tests:**

| Pattern | Use |
|---------|-----|
| After a batch of `send()` calls | Call `wait_for_ack()` before checking results or sending dependent commands |
| Between independent `send()` calls | No synchronization needed (FIFO ordering per peer is guaranteed) |
| Before `wait_for_shutdown()` | Call `wait_for_ack()` to ensure all routes hit the wire |

**Do NOT use `time.sleep()` for forward delivery synchronization.** The flush barrier is deterministic and does not depend on timing. Use `time.sleep()` only for non-forward-pool concerns (session establishment, RPKI cache, event propagation).

---

## Editor Tests (.et format)

Editor tests run through `./le functional editor`; select one with `./le job run label editor-one command bin/ze-test editor ID_OR_NAME`.

<!-- source: internal/component/cli/testing/parser.go -- .et file parser -->
<!-- source: internal/test/cli/cmd_editor.go -- cmdEditorMain selection flags -->

### Key Directives

| Directive | Example | Purpose |
|-----------|---------|---------|
| `tmpfs=` | `tmpfs=test.conf:terminator=EOF` | Embed test files |
| `option=file:path=` | `option=file:path=test.conf` | Config file to load |
| `option=mode:value=` | `option=mode:value=command` | Command-only mode (no editor) |
| `option=history:store` | `option=history:store` | Enable zefs-backed history persistence |
| `option=storage:value=` | `option=storage:value=blob` | Run the editor on a zefs blob, as the daemon does, instead of the filesystem |
| `input=type:text=` | `input=type:text=show` | Type text |
| `input=enter/up/down/tab` | `input=enter` | Press named key |
| `expect=input:value=` | `expect=input:value=show` | Assert input buffer content |
| `expect=mode:is=` | `expect=mode:is=command` | Assert editor mode |
| `restart=` | `restart=editor` | Simulate exit + relaunch (blob store persists) |

---

## Fuzz Testing

Ze includes 72 fuzz targets covering all wire parsers, NLRI codecs, config
parsing, cryptographic operations, protocol state machines, IGP packet decoders
(IS-IS, OSPF), and receiver/server-facing parsers (BMP, RADIUS, DHCP, VRRP).
Fuzz tests catch crashes, panics, and memory corruption on malformed input.

The target list is not maintained at all. `./le fuzz` walks `internal/` for
`func Fuzz` when it runs, and emits one anchored `-fuzz=^<Name>$` invocation
per target against that target's exact package. A new fuzzer is included by the
existing action.

```bash
./le fuzz run                               # every target, 10s each
./le fuzz list                              # what would run, and where
FUZZ=FuzzParseUpdate PKG=./internal/... TIME=30s ./le fuzz run

```

`./le fuzz run` discovers the targets and runs them through the native Go
action.
<!-- source: internal/le/fuzz/actions.go -- Actions -->

Fuzz tests are not part of `./le verify current mode full` (they're time-bounded, not pass/fail
in the traditional sense). Run them periodically or before releases.

### Fuzz Target Areas

| Area | Targets | Examples |
|------|---------|---------|
| BGP messages | 6 | `FuzzParseHeader`, `FuzzUnpackOpen`, `FuzzUnpackUpdate`, `FuzzUnpackNotification` |
| BGP attributes | 7 | `FuzzParseOrigin`, `FuzzParseMED`, `FuzzParseASPath`, `FuzzParseCommunity` |
| NLRI codecs | 16 | `FuzzParseVPN`, `FuzzParseEVPN`, `FuzzParseFlowSpec`, `FuzzParseBGPLS`, `FuzzParseMUP` |
| Wire encoding | 5 | `FuzzParseIPv4Prefixes`, `FuzzParseIPv6Prefixes`, `FuzzParsePrefixes`, `FuzzRewriteASPath`, `FuzzParseNLRIs` |
| IS-IS | 3 | `FuzzISISDecodePDU`, `FuzzISISTLVIterator`, `FuzzISISRoundTrip` |
| OSPF | 7 | `FuzzOSPFDecodePacket`, `FuzzOSPFLSAIterator`, `FuzzOSPFTEBody`, `FuzzOSPFExtLinkBody` |
| Config parser | 2 | `FuzzConfigParser`, `FuzzTokenizer` |
| L2TP | 4 | `FuzzParseMessageHeader`, `FuzzAVPIterator`, `FuzzHiddenDecrypt`, `FuzzOnReceiveSequence` |
| BFD | 3 | `FuzzParseControl`, `FuzzParseAuth`, `FuzzAuthDigest` |
| PPP | 7 | `FuzzParseLCPPacket`, `FuzzParseLCPOptions`, `FuzzParseCHAPResponse`, `FuzzParsePAPRequest`, `FuzzParseFrame` |
| TACACS+ | 2 | `FuzzTacacsPacketUnmarshal`, `FuzzTacacsEncryptDecrypt` |
| Receiver/server parsers | 4 | `FuzzDecodeBMPTLV`, `FuzzDecodeRADIUSVSA`, `FuzzDHCPHandle`, `FuzzDecode` (VRRP) |
| Other | 6 | `FuzzHandleRoundTrip`, `FuzzInvalidHandle`, `FuzzParseAttributes`, `FuzzEncodeDecode`, `FuzzScanner`, `FuzzFSMEventSequence` |
<!-- source: internal/plugins/isis/packet/fuzz_test.go -- IS-IS packet fuzz targets -->
<!-- source: internal/plugins/ospf/packet/fuzz_test.go -- OSPF packet fuzz targets -->
<!-- source: internal/le/fuzz/actions.go -- Answer -->
<!-- source: internal/component/bgp/message/fuzz_test.go -- BGP message fuzz targets -->
<!-- source: internal/component/bgp/plugins/bmp/fuzz_test.go -- FuzzDecodeBMPTLV -->
<!-- source: internal/component/radius/fuzz_test.go -- FuzzDecodeRADIUSVSA -->
<!-- source: internal/plugins/dhcpserver/fuzz_test.go -- FuzzDHCPHandle -->
<!-- source: internal/core/bgp/attribute/builder_parse_fuzz_test.go -- attribute parser fuzz targets -->
<!-- source: internal/component/bgp/wireu/prefix_fuzz_test.go -- NLRI prefix fuzz targets -->
<!-- source: internal/component/config/fuzz_test.go -- config parser fuzz targets -->
<!-- source: internal/component/l2tp/ -- L2TP wire fuzz targets -->
<!-- source: internal/component/bfd/ -- BFD packet/auth fuzz targets -->
<!-- source: internal/component/l2tp/ppp/ -- PPP frame/protocol fuzz targets -->

### Writing a New Fuzz Target

```go
func FuzzParseMyProtocol(f *testing.F) {
    // Seed with known-good wire bytes
    f.Add([]byte{0x01, 0x02, 0x03})

    f.Fuzz(func(t *testing.T, data []byte) {
        // Must not panic on any input
        result, err := ParseMyProtocol(data)
        if err != nil {
            return  // Parse errors are expected
        }
        // Optionally: round-trip check
        encoded := result.Marshal()
        // ...
    })
}
```

---

## Live Tests (Docker + Internet)

Live tests run against real external infrastructure inside Docker containers.
They are **not** part of `./le verify current mode full` and require both Docker and internet access.

<!-- source: internal/component/bgp/plugins/rpki/rpki_live_test.go -- TestLiveRPKIValidation -->

```bash
./le integration live-rpki
```

### Build Tag

Live tests use `//go:build live`. Normal unit, functional, and pre-commit runs
exclude them. `./le integration live-rpki` supplies the tag and the
external infrastructure.

### RPKI Live Test

Starts a [stayrtr](https://github.com/bgp/stayrtr) container that fetches real-world
RPKI data, connects ze's RTR client, and validates known prefixes:

| Prefix | Origin AS | Expected | Owner |
|--------|-----------|----------|-------|
| `1.1.1.0/24` | 13335 | Valid | Cloudflare DNS |
| `8.8.8.0/24` | 15169 | Valid | Google DNS |
| `82.212.0.0/16` | 64496 | NotFound | No ROA coverage |
| `1.1.1.0/24` | 64496 | Invalid | Wrong origin for covered prefix |

**Requirements:** Docker, internet access (fetches ~5 MB rpki.json from Cloudflare).
**Timeout:** 180s (includes image pull, data fetch, RTR sync).
**Skip behavior:** Test skips gracefully if Docker is unavailable or image cannot be pulled.

---

## Integration Tests (Network Namespaces)

Integration tests exercise Linux dataplane packages against the real kernel inside
ephemeral network namespaces. They require `CAP_NET_ADMIN` (typically root) and are
excluded from all normal test targets.

<!-- source: internal/component/iface/integration_helpers_linux_test.go -- withNetNS, waitForEvent -->
<!-- source: internal/component/iface/config_integration_linux_test.go -- iface config ownership and rollback integration tests -->
<!-- source: internal/plugins/fib/kernel/integration_linux_test.go -- route ownership integration tests -->
<!-- source: internal/plugins/firewall/nft/integration_linux_test.go -- nft table ownership integration tests -->
<!-- source: internal/plugins/traffic/netlink/integration_linux_test.go -- tc qdisc restore integration tests -->

### Running on Linux

```bash
./le integration iface
./le integration fib
./le integration firewall
./le integration traffic
```

### Running on macOS (QEMU)

macOS cannot run these tests natively. Use the QEMU Alpine VM:

```bash
./le qemu all-tests
```

This is the standard workflow for macOS developers. The QEMU runner boots an
Alpine Linux VM, mounts the repo via virtio-9p, and runs `go test -tags integration`
inside the VM. First run downloads the Alpine ISO and Go toolchain (~1 min);
subsequent runs reuse the cache (~30s boot + test time).

See [testing/qemu-integration.md](architecture/testing/qemu-integration.md) for
details on how to write QEMU integration tests and add new packages.

The `trafficusage` eBPF/TCX tests in `internal/plugins/trafficusage/` carry
`//go:build integration && linux`. `./le qemu all-tests` runs them inside the
guest with the registered integration population.
<!-- source: internal/plugins/trafficusage/program_test.go -- BPF_PROG_TEST_RUN eBPF program tests -->
<!-- source: internal/plugins/trafficusage/attach_integration_linux_test.go -- veth + AF_PACKET + /metrics scrape -->

### Deployment Evidence

```bash
./le evidence release-candidate
./le deployment vpp-test
./le deployment l2tp-test
./le deployment l2tp-ppp-test
./le qemu install-test
./le qemu install-iso-test
./le deployment docker-l2tp-ppp-test
```

These actions are external evidence and declare their own prerequisites.
The L2TP peer actions require `xl2tpd`, `pppd`, `/dev/ppp`, `iproute2`, and
PPPoL2TP kernel support.

### Build Tag

Integration tests use `//go:build integration && linux`. Normal unit,
functional, and pre-commit runs exclude them. The native `./le integration`
and `./le qemu` actions supply the integration tag where their scenarios need
it.

### How They Work

Each test calls `withNetNS(t, func() { ... })` which:

1. Locks the goroutine to its OS thread (`runtime.LockOSThread`)
2. Creates a named network namespace (`netns.NewNamed`)
3. Switches into it (`netns.Set`)
4. Runs the test function (creating interfaces, addresses, etc.)
5. Restores the original namespace and deletes the test namespace in `t.Cleanup`

If namespace creation fails (missing `CAP_NET_ADMIN`), the test is skipped with `t.Skip`.

### Test Categories

| File | Covers | Tests |
|------|--------|-------|
| `manage_integration_linux_test.go` | Interface CRUD, addresses, MTU | 9 tests |
| `config_integration_linux_test.go` | Config apply ownership scope, reload deletion scope, rollback of created links | 3 tests |
| `monitor_integration_linux_test.go` | Netlink event monitoring | 5 tests |
| `sysctl_integration_linux_test.go` | Real /proc/sys writes | 2 tests |
| `mirror_integration_linux_test.go` | tc qdisc/filter setup | 5 tests |
| `dhcp_integration_linux_test.go` | DHCPv4 with in-process server | 2 tests |
| `migrate_integration_linux_test.go` | Make-before-break migration | 2 tests |

Additional dataplane integration packages:

| File | Covers | Tests |
|------|--------|-------|
| `internal/plugins/fib/kernel/integration_linux_test.go` | FIB route add/remove/replace, restart sweep, producer protocol isolation, flush-on-stop | 8 tests |
| `internal/plugins/firewall/nft/integration_linux_test.go` | nft table ownership, same-instance cleanup, restart reapply preservation | 2 tests |
| `internal/plugins/traffic/netlink/integration_linux_test.go` | real tc qdisc snapshot, persisted restore after backend restart | 1 test |
| `internal/plugins/iface/netlink/vlanqoslab_integration_linux_test.go` | VLAN QoS wire-level: egress PCP on wire, ingress PCP classification, DSCP-to-PCP full chain | 3 tests |
| `internal/plugins/trafficusage/program_test.go` | Pure-Go eBPF programs via BPF_PROG_TEST_RUN: per-IP and per-(port, protocol) byte accounting, ICMP port-0, non-IPv4/truncated pass-through, accumulation | 10 tests |
| `internal/plugins/trafficusage/attach_integration_linux_test.go` | Real TCX attach on a veth pair, AF_PACKET frame injection, clean Stop detach, `ze_traffic_usage_*` `/metrics` scrape | 2 tests |

### Shared Helpers

`integration_helpers_linux_test.go` provides:

| Helper | Purpose |
|--------|---------|
| `withNetNS(t, fn)` | Ephemeral namespace wrapper |
| `waitForEvent(t, bus, topic, timeout)` | Poll collectingBus for an event |
| `linkExists(name)` | Check if interface exists via netlink |
| `hasAddress(iface, cidr)` | Check if address is on interface |
| `requireLinkUp(t, name)` | Assert link is UP |
| `createDummyForTest(t, name)` | Create dummy with cleanup |
| `createVethForTest(t, name, peer)` | Create veth pair with cleanup |

---

## L2TP Tests

L2TP functional tests (`test/l2tp/`) verify tunnel lifecycle, session
negotiation, authentication, IP pool, and teardown over real loopback UDP.
Run with `./le functional l2tp`.

> **In the default release gate.** The in-tree L2TP `.ci` tests are included in
> `./le functional` and can be selected with `./le functional l2tp`.
> External-peer and PPP dataplane evidence use separate deployment actions.

```bash
ze-test l2tp --list    # List available tests
ze-test l2tp --all     # Run all tests
```

For external-peer evidence, run `./le deployment l2tp-test`. It uses a
real `xl2tpd` LAC to establish the L2TP control tunnel and incoming-call session
against Ze. It intentionally does not claim full PPP/NCP dataplane proof when
the Docker host lacks the `l2tp_ppp` kernel module.

For full PPP/NCP peer evidence, run `./le deployment l2tp-ppp-test` on a
Linux host or target runner with `xl2tpd`, `pppd`, `ping`, `/dev/ppp`,
`iproute2`, and PPPoL2TP kernel support. The target refuses skip-kernel-probe
mode, creates peer-isolated Ze and LAC network namespaces, starts Ze as LNS,
drives a real `xl2tpd`/`pppd` LAC across the veth underlay, waits for PPP
LCP/IPCP completion, verifies log field correctness (assigned address and pppN
interface name), checks the resulting Ze and LAC `pppN` address state, pings
the LNS through the PPP tunnel from the LAC namespace to prove dataplane
connectivity, observes subscriber route injection, and verifies teardown
returns both namespaces' kernel L2TP/PPP state to their initial snapshots.

On macOS, `./le deployment docker-l2tp-ppp-test` runs that same proof in
privileged Linux containers. Docker is only a Linux userspace wrapper here: the
test still fails unless the Docker host kernel has `/dev/ppp`, Generic Netlink
L2TP, and PPPoL2TP support.

For appliance evidence, run `./le deployment gokrazy-l2tp-ppp-test` on a
Linux host with QEMU and PPPoL2TP support. The native appliance kernel action
materializes the verified runtime kernel from its durable cache or builds it
through `internal/appliance/kernelbuilder`. The proof then builds a temporary
gokrazy image, boots it under QEMU, drives a real `xl2tpd` and `pppd` LAC, and
checks PPP/IPCP state, dataplane reachability, and route withdrawal.
<!-- source: internal/le/deployment/actions.go -- Actions -->
<!-- source: internal/appliance/kernelbuilder/driver.go -- Build -->
<!-- source: internal/component/l2tp/kernel_linux.go -- probeKernelModules -->

A ze appliance that dies before serving reports the reason on the serial
console: every fatal pre-serve startup failure is mirrored onto the slog
backend as `msg="startup failed"` with a `stage` attribute, and the appliance
logs through kmsg, which the kernel prints to the configured console. Without
this the failure was only on stderr, which the gokrazy supervisor captures
away from serial, and a first-boot crash-loop was undiagnosable from the
console.
<!-- source: cmd/ze/hub/main.go -- logStartupFailure -->
<!-- source: gokrazy/ze/config.json -- ze.log.backend=kmsg,stderr -->

The `test/ui/startup-failure-slog.ci` functional test pins the slog mirror
from the user entry point: a daemon started with an unparsable
`ze.web.listen` exits 1 with both the stderr print and the
`startup failed` slog line.

### Tunnel lifecycle

| Test | File | What it verifies |
|------|------|-----------------|
| SCCRQ handshake | `test/l2tp/handshake-sccrq.ci` | Python client sends SCCRQ hex, receives SCCRP |
| Full handshake | `test/l2tp/handshake-full.ci` | SCCRQ/SCCRP/SCCCN/ZLB exchange with challenge/response |
| Bad challenge | `test/l2tp/bad-challenge-response.ci` | Wrong challenge response triggers StopCCN RC=4 |

### Session lifecycle

| Test | File | What it verifies |
|------|------|-----------------|
| Incoming session (LNS) | `test/l2tp/session-incoming-lns.ci` | ICRQ/ICRP/ICCN exchange establishes session |
| CDN teardown | `test/l2tp/session-cdn-teardown.ci` | CDN tears down one session cleanly |
| StopCCN cascade | `test/l2tp/session-stopccn-cascade.ci` | StopCCN tears down tunnel and all its sessions |

### Authentication and IP pool

| Test | File | What it verifies |
|------|------|-----------------|
| Auth + pool | `test/l2tp/session-auth-pool.ci` | Full session with auth-local + pool allocation |
| Auth-local config | `test/l2tp/auth-local-config.ci` | Static user config parsed and auth works |
| RADIUS basic | `test/l2tp/auth-radius-basic.ci` | RADIUS Access-Request sent, session authenticated |
| RADIUS reject | `test/l2tp/auth-radius-reject.ci` | RADIUS Access-Reject fails the session |
| Pool basic | `test/l2tp/pool-basic.ci` | Pool allocates and releases addresses |
| Pool minimal range | `test/l2tp/pool-minimal-range.ci` | Single-address pool boundary case |
| Re-auth interval | `test/l2tp/reauth-interval-clamp.ci` | Safety floor clamps the re-auth interval |

### Config parsing

| Test | File | What it verifies |
|------|------|-----------------|
| Minimal config | `test/parse/l2tp-minimal.ci` | `l2tp { server main { port 1701 } }` parses |
| Bad port | `test/parse/l2tp-bad-port.ci` | `port 0` rejected |
| Unknown field | `test/parse/l2tp-unknown-field.ci` | Unknown key rejected with suggestion |
| Max sessions | `test/parse/l2tp-max-sessions.ci` | `max-sessions` value accepted |
| Auth policy | `test/parse/l2tp-auth-policy.ci` | `auth-method` and `allow-no-auth` values accepted |
| Hello retries | `test/parse/l2tp-hello-retries-parse.ci` | `hello-retries` (dead-peer threshold) accepted |
| Hello retries range | `test/parse/l2tp-hello-retries-range.ci` | `hello-retries 256` rejected (uint8 range) |

<!-- source: internal/test/cli/register.go -- l2tpCmd runner dispatch -->
<!-- source: internal/test/runner/record_parse.go -- .ci discovery and directive parsing -->

### L2TP scale tests

Scale tests (`internal/le/deployment/l2tpscale.go`) validate Ze's L2TP control plane at
2000 concurrent sessions across 10 tunnels. They run on loopback (no
root, no Docker, no kernel modules) and measure session establishment
rate, RADIUS round-trip handling, pool allocation correctness, and
teardown completeness.

The test tooling lives in `ze-test l2tp-scale`, which bundles a Go LAC
simulator (speaking the L2TP wire protocol) and an embedded mock RADIUS
server. The native deployment action starts Ze, runs the four-scenario
registry, checks each result, and retains the simulator's exact result bytes.

```bash
./le deployment l2tp-scale-test
L2TP_SCALE_SCENARIO=2k-sessions ./le deployment l2tp-scale-test
ze-test l2tp-scale --help
```

| Scenario | Native checker | What it validates |
|----------|----------------|-------------------|
| 2k sessions | `checkL2TPScale2K` | All 2000 sessions established |
| Clean teardown | `checkL2TPScaleCleanTeardown` | No resource leaks after teardown |
| Pool exhaustion | `checkL2TPScalePoolExhaustion` | Sessions beyond pool size rejected |
| Slow RADIUS | `checkL2TPScaleSlowRADIUS` | Sessions established under 500ms RADIUS delay |

<!-- source: internal/test/cli/cmd_l2tp_scale.go -- LAC simulator + mock RADIUS -->
<!-- source: internal/le/deployment/l2tpscale.go -- native scale registry and runner -->

---

**Updated:** 2026-08-27
