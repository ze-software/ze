# Spec: le-subject-first-command-tree

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Depends | - |
| Phase | 1c/5 |
| Handoff | - |
| Updated | 2026-09-24 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`le` registers 91 commands today (`./le '|' json`, 2026-09-24). Their names mix
two grammars. Some are subject-first (`verify lint`, `spec status`). Most are
one hyphenated word (`go-extract`, `test-unit`, `iana-asn`), and some are only a
verb (`changed`, `tracked`). Beside `le`, developer programs carry their own
entry points: `ze-test`, `ze-chaos`, `ze-perf` and `ze-analyze` (build tags on
`cmd/ze`), and `cmd/ze-gok`, `cmd/ze-perf-run` and `cmd/ze-terminal-pty`.

The owner approved the design on 2026-09-24. Every `le` command becomes
`./le <subject> <action>`, keyword before value, the same grammar as the `ze`
CLI (`ai/rules/cli.md`). The developer programs fold into `le`. The help screen
can still group commands by kind (workflow, gate, generate, suite, report) as a
view. The names are subject-first.

Other sessions run in this checkout now and call the old names. So the work has
three ordered phases:

| Phase | What lands | Old names |
|-------|------------|-----------|
| 1 | Every new name is registered. Every package moves to the directory its new name predicts. Every program is reachable through `le`. The `le-test` binary and the `le.test.*` variables exist | still work; each prints one stderr line that names the new name |
| 2 | Every caller of an old name moves to the new name, driven by the rename map | still work |
| 3 | Every old name, old program, old build tag and old variable is deleted, and a gate refuses any tracked file that names one | gone |

**Phase 1 and Phase 2 are an owner-approved, time-bounded exception to
`ai/rules/no-layering.md` (owner decision, 2026-09-24).** Old and new names
coexist only because peer sessions call the old names while this work runs.
Phase 3 is the removal, and it is part of this spec, not a follow-up. The spec
does not close until Phase 3 lands.

**This spec SUPERSEDES `spec-le-command-namespaces`, closed and removed (owner decision,
2026-09-24).** Its still-open acceptance criterion is folded in below
("Folded in from the superseded spec"). Three of its decisions are reversed
here and dropped: "the test-* family is not split", "docvalid and docs-to-code
are left alone", and its AC-14 "every stage writes the same log file name it
wrote before the rename". Its AC-13 exception list ("`test` is the only
exception") is dropped with the first of them. The superseded file is closed by
the main thread, not by this spec's implementation.

### Goals

| # | Goal |
|---|------|
| G-1 | Every approved new name works |
| G-2 | No tracked file names an old name |
| G-3 | The old names and the standalone developer builds are gone |
| G-4 | The test harness still ships as a cross-compiled artifact for containers and VMs, under its new name `le-test` |

### The rename map (one declaration)

The map is declared ONCE, in Go, in `internal/le/leroot` (see Key Design
Decisions). Phase 1 registers the command aliases from it, the Phase 2 report
reads it, and the Phase 3 gate reads it. No other surface lists old names.

A command row is a word sequence. An alias rewrites the leading words of argv
and dispatches the result again, so one row can rename a command, merge two
commands, or split the verbs of one command.

#### Commands whose name does not change

`commit`, `session`, `setup`, `worktree`, `scratch`, `weekly`, `job`, `rfc`,
`verify`, `verify deps`, `verify status`, `verify summary`, `spec citation`,
`spec roadmap`, `spec status`, `plugin boundary`, `plugin declarations`,
`plugin imports`, `config claims`, `config coercion`, `yang glue`,
`yang migration`, `doc check`, `doc wiring`, `site`, `site facts`, `site wiki`.
That is 27 commands.

#### Renamed commands

| Old words | New words | New package directory under `internal/le/` |
|-----------|-----------|---------------------------------------------|
| `verify lock` | `job` | none: `verify/lock` is deleted; its `run label <l> command <argv>` grammar is already `job run` |
| `spec session` | `spec` | `spec` (the `session` word flattens; every action stays: current, claim, release, state, review, wip, model) |
| `journal` | `spec journal` | `spec/journal` |
| `evidence` | `verify evidence` | `verify/evidence` |
| `go-extract` | `go extract` | `go/extract` |
| `module` | `go module` | `go/module` |
| `go-version` | `go version-pin` | `go/versionpin` |
| `verify lint` | `go lint` | `go/lint` |
| `platform-vet` | `go vet-platforms` | `go/vetplatforms` |
| `staticcheck-feature-matrix` | `go staticcheck` | `go/staticcheck` |
| `repository` | `repo` | `repo` (verbs check, tree-check, generate, generated-check unchanged) |
| `repository tracked-build` | `repo tracked-build` | `repo/trackedbuild` |
| `tracked` | `repo tracked-le` | `repo/trackedle` |
| `changed` | `repo changed` | `repo/changed` |
| `working-tree` | `repo working-tree` | `repo/workingtree` |
| `inventory` | `repo inventory` | `repo/inventory` |
| `arch-map` | `repo arch-map` | `repo/archmap` |
| `discovery-index` | `repo package-map` | `repo/packagemap` |
| `feature-tags` | `repo feature-tags` | `repo/featuretags` |
| `source-rewrite` | `repo rewrite` | `repo/rewrite` |
| `tier` | `arch tier` | `arch/tier` |
| `enumeration` | `arch enumeration` | `arch/enumeration` |
| `fs-persistence` | `arch fs-persistence` | `arch/fspersistence` |
| `iface-resolution` | `arch iface-resolution` | `arch/ifaceresolution` |
| `cli-grammar` | `cli grammar` | `cli/grammar` |
| `ci-dispatch` | `cli dispatch` | `cli/dispatch` |
| `dash-stdio` | `cli stdio` | `cli/stdio` |
| `command ownership` | `cli ownership` | `cli/ownership` |
| `command list` | `cli list` | `cli/list` |
| `wiki-catalog` | `cli catalog` | `cli/catalog` |
| `port-defaults` | `config ports` | `config/ports` |
| `yang leaf-mentions` | `config unread-leaves` | `config/unreadleaves` |
| `docs-to-code check` and `docs-to-code index-check` | `doc index check` | `doc/index` (D-5: one `check` covers both generated files) |
| `docs-to-code update` and `docs-to-code index-update` | `doc index write` | `doc/index` (D-5: one `write` regenerates both) |
| `docvalid` | `doc yang-contract` | `doc/yangcontract` |
| `consistency` | `doc consistency` | `doc/consistency` |
| `ste` | `doc ste` | `doc/ste` |
| `terminal-demo` | `site terminal-demo` | `site/terminaldemo` |
| `web-assets` | `web assets` | `web/assets` |
| `vendor-web` | `web vendor` | `web/vendor` |
| `htmx-upgrade` | `web htmx` | `web/htmx` |
| `ai skills-sync` | `ai sync write` | `ai/sync` (D-4) |
| `ai sync-check` | `ai sync check` | `ai/sync` |
| `ai sync-preview` | `ai sync preview` | `ai/sync` |
| `rules` | `ai rules` | `ai/rules` |
| `hook-check` | `ai hooks` | `ai/hooks` |
| `digest` | `ai digest` | `ai/digest` |
| `token-economy` | `ai tokens` | `ai/tokens` |
| `protocol-skeleton` | `rfc skeletons` | `rfc/skeletons` |
| `test-unit` | `test unit` | `test/unit` |
| `functional` | `test functional` | `test/functional` |
| `integration` | `test integration` | `test/integration` |
| `deployment` | `test deployment` | `test/deployment` |
| `qemu` | `test qemu` | `test/qemu` |
| `fuzz` | `test fuzz` | `test/fuzz` |
| `stress-repro` | `test stress-repro` | `test/stressrepro` |
| `test-helper` | `test fixture` | `test/fixture` |
| `netlab` | `test netlab` | `test/netlab` |
| `mutation` | `test mutation` | `test/mutation` |
| `test-health` | `test health` | `test/health` |
| `test-sensitivity` | `test sensitivity` | `test/sensitivity` |
| `test-weakened` | `test weakened` | `test/weakened` |
| `test-chaos` | `chaos selftest` | `chaos/selftest` |
| `perf-bench run` | `perf run` | `perf` (D-1: `perf` is one area whose verbs are listed in D-1) |
| `perf-bench suggestion-report` | `perf suggest` | `perf` |
| `perf-bench record` | `perf record` | `perf` |
| `perf-bench history-record` | `perf history-record` | `perf` |
| `perf-bench evidence-record` | `perf evidence-record` | `perf` |
| `build-artifacts host` | `build host-driver` | `build/hostdriver` |
| `build-artifacts installer-amd64` | `build installer amd64` | `build/installer` |
| `build-artifacts installer-arm64` | `build installer arm64` | `build/installer` |
| `gokrazy-gosum` | `build gosum` | `build/gosum` |
| `iana-asn` | `data asn-delegation` | `data/asndelegation` |

#### Programs folded into `le`

| Old program | Built today by | New entry | Package under `internal/le/` |
|-------------|----------------|-----------|------------------------------|
| `ze-test` (typed by a person) | `cmd/ze` with tag `ze_test` | `./le test harness <root> <args>` | `test/harness` |
| `ze-chaos` | `cmd/ze` with tag `ze_chaos` | `./le chaos run <args>` | `chaos/run` |
| `ze-perf run` | `cmd/ze` with tag `ze_perf` | `./le perf send` (D-1) | `perf` |
| `ze-perf report`, `ze-perf track` | `cmd/ze` with tag `ze_perf` | `./le perf report`, `./le perf track` | `perf` |
| `cmd/ze-perf-run` | `go run ./cmd/ze-perf-run` | `./le perf run` (D-1: the same program as `perf-bench run`) | `perf` |
| `bin/ze-perf-linux` in the perf sender container | `perfrunner`, tags `ze_perf ze_bgp` | a linux `le` mounted at `/usr/local/bin/le`, running `le perf send` (D-2) | `perf` |
| `ze-analyze` | `cmd/ze` with tag `ze_analyze` | `./le mrt <subcommand> <args>` | `mrt` |
| `cmd/ze-gok` | `internal/le/deployment` builds it and execs it | `./le build gokrazy <gok args>` | `build/gokrazy` |
| `cmd/ze-terminal-pty` | `internal/le/terminaldemo` builds it for the demo container | the `pty` action of `./le site terminal-demo` | `site/terminaldemo` |

#### The harness binary (owner amendment, 2026-09-24)

| Old | New | Phases 1 and 2 | Phase 3 |
|-----|-----|----------------|---------|
| `bin/ze-test` | `bin/le-test` | every builder writes `le-test`, and also `ze-test` as a hard link to the same file | only `le-test` is written |
| `bin/ze-test-linux-<arch>` | `bin/le-test-linux-<arch>` | both names, same rule | only the new name |
| `test/interop/ze-test-linux`, `/usr/local/bin/ze-test` in `test/interop/Dockerfile.ze` | `test/interop/le-test-linux`, `/usr/local/bin/le-test` | the image carries both names | only the new name |

How the `le-` prefix meets the personality selection, read at the producers:

| Mechanism | What it does with `le-test` |
|-----------|-----------------------------|
| `defaultDispatch` (`cmd/ze/dispatch.go`) | `LookupRoot("le-test")` is nil in a `ze_test` build, so argv[0] selects the harness root, as for `ze-test` today |
| `binarySuffixRoot` (`cmd/ze/dispatch.go`) | answers `test` for both `ze-test` and `le-test`, and no build registers a root `test`, so no change. Phase 3 deletes the function |
| `invokedBuildName` (`cmd/ze/le_build_name.go`) | the file is not named `le`, so it answers `le-test` |
| `refuseWrongBuildName` (`cmd/ze/le_build_name.go`) | acts only when root `le` is registered; a `ze_test` build registers none, so a harness child of a `./le --name x` session is never refused |
| the launcher `le` | `./le --name test` builds `bin/le-test/le`, a DIRECTORY at the path of the harness FILE. `--name test-linux-<arch>` clashes the same way. D-3 refuses both |

#### The harness variables (owner decisions, 2026-09-24)

Every variable the harness owns moves to the `le.` key and the `LE_` spelling.
A variable the PRODUCT reads keeps its `ze.` key.

| Old key (`ZE_` spelling) | New key (`LE_` spelling) | Owner, read at the producer | Phases 1 and 2 | Phase 3 |
|--------------------------|--------------------------|-----------------------------|----------------|---------|
| `ze.test.bin` (`ZE_TEST_BIN`) | `le.test.bin` (`LE_TEST_BIN`) | harness: registered in `internal/test/runner/runner.go` | both read, new wins | old gone |
| `ze.qemu.test.bin` (`ZE_QEMU_TEST_BIN`) | `le.qemu.test.bin` (`LE_QEMU_TEST_BIN`) | harness: registered in `internal/le/qemu/run.go` | both read, new wins | old gone |
| `ze.test.no.build` (`ZE_TEST_NO_BUILD`) | `le.test.no.build` (`LE_TEST_NO_BUILD`) | harness: registered in `internal/test/runner/runner.go`, read by `internal/test/cli/cmd_bgp.go` and `cmd_web.go`, set by `internal/le/functional`, `qemu`, `stressrepro` | both read, new wins | old gone |
| `ze.test.bgp.port` (`ZE_TEST_BGP_PORT`) | unchanged | PRODUCT: `envKeyTestPort` in `internal/component/bgp/reactor/reactor_peers.go` reads it, so the daemon and the harness `peer` agree on one port. Registered in `internal/test/cli/cmd_peer.go` | unchanged | unchanged |
| `ze.test.vpp.cli.socket` | unchanged | PRODUCT: `internal/component/vpp/trace_linux.go` | unchanged | unchanged |

The other `ze.test.*` keys are fixtures in `internal/core/env` unit tests and
stay.

**Why the two binary-path variables are not merged.** They carry different
binaries in one run. `.github/workflows/qemu-nightly.yml` sets both in one step,
the host harness and the target-architecture harness. In code,
`netnsGuestBinaries` (`internal/le/qemu/netns_linux.go`) reads
`ZE_QEMU_TEST_BIN` on the HOST, default `bin/ze-test-linux-<guest arch>`, and
forwards the value INTO the guest as `ZE_TEST_BIN` (`guestTestBinKey`,
`internal/le/qemu/guest_linux.go`). The runner reads `ze.test.bin` as the
binary it runs in its own environment, default `bin/ze-test`. On a host whose
architecture is not the guest's, one variable cannot hold both values.

**Direct reads that bypass the registry fold into the registered entry.** Three
reads name a variable by its raw spelling instead of its registered key:
`envOr("ZE_TEST_BIN", ...)` in `internal/le/qemu/alltests.go`,
`settingFromEnv("ZE_QEMU_TEST_BIN", ...)` in `internal/le/qemu/netns_linux.go`,
and `guestTestBinKey` in `internal/le/qemu/guest_linux.go`. Each moves to
`env.Get` of the registered `le.` key.

**Defect the rename fixes.** The variable `ze.qemu.test.bin` has two defaults.
`internal/le/qemu/run.go` registers `bin/ze-test-linux-arm64`, and
`netnsGuestBinaries` uses `bin/ze-test-linux-` plus `qemuGuestArch()`. The
registered entry becomes the only reader, with the default derived from
`qemuGuestArch()`.

### Owner decisions (2026-09-24)

These answer the six questions the first draft left open.

| # | Decision | What the code showed | Consequence |
|---|----------|----------------------|-------------|
| D-1 | `perf` is ONE area. `perf-bench run` and `cmd/ze-perf-run` are one program, `perf run`. The other perf-bench verbs keep their words under `perf`, except `suggestion-report`, which becomes `perf suggest`. The ze-perf subcommands keep their names under `perf`, except `run`, which becomes `perf send` | `perf-bench run` runs the cross-DUT suite through `perfrunner.New` (`internal/le/perfbench/bench.go`); `cmd/ze-perf-run` runs it through `perfrunner.RunCLI`, which takes `--build`, `--test` and DUT names (`internal/test/perfrunner/run.go`) | the verbs of `perf` are `run`, `suggest`, `record`, `history-record`, `evidence-record`, `send`, `report`, `track`. Collision check: the only shared word was `run` (perf-bench `run`, ze-perf `run`), resolved by `send`. `report` (ze-perf) and `suggestion-report` (perf-bench) do not collide once the second is `suggest`. `perf run` keeps the keywords of `perf-bench run` and gains the build, test and DUT selection that `RunCLI` accepted |
| D-2 | No standalone perf binary. The perf runner cross-compiles `le` for linux, with `CGO_ENABLED=0` and the target architecture of the container, and bind-mounts it where `bin/ze-perf-linux` is mounted today. Inside the container the sender runs `le perf send ...` | the launcher `le` already builds with `CGO_ENABLED=0` and the tags `ze_le` plus every tag in `feature-gates.txt` (`build_le`), so the BGP code that `perf send` needs is linked | the `ze_perf` tag, `bin/ze-perf`, `bin/ze-perf-linux`, `ZE_PERF_BIN` and `cmd/ze-perf-run` are deleted in Phase 3. `.github/workflows/perf-nightly.yml` moves to `./le perf track --check ...` in Phase 2 |
| D-2a | The file inside the container is named `le` | `defaultDispatch` selects the personality with `LookupRoot(binaryName())` (`cmd/ze/dispatch.go`), so a file named `le` selects root `le`. `refuseWrongBuildName` (`cmd/ze/le_build_name.go`) returns 0 when `ZE_LE_BUILD_NAME` is unset, and a container started without the launcher environment has it unset | `le perf send` answers inside the container with no launcher environment (AC-27) |
| D-3 | The launcher `le` refuses `--name test`, `--name test-linux-*`, and any name whose `bin/le-<name>` is a path a `bin/le-test*` artifact uses, with a message that names the artifact | `check_name` in `le` accepts any name of letters, digits, dot, underscore and hyphen | perf reserves no name, because D-2 leaves no perf artifact under `bin/le-*` |
| D-4 | `ai sync write`, `ai sync check`, `ai sync preview` | a bare area lists its actions and writes nothing, so the write needs a verb | as the rename map shows |
| D-5 | `doc index check` and `doc index write`, like the other generated files (`repo feature-tags`, `web assets`, `plugin imports`, `yang glue`) | `docs-to-code` generates TWO files with four verbs: `check` and `update` for `ai/DOCS-TO-CODE.md`, `index-check` and `index-update` for `ai/CODE-TO-DOCS.md`; `index-check` also checks that every `<!-- source: -->` anchor resolves | `check` runs both file checks and the anchor check; `write` regenerates both files. Both old verbs of each pair map to one new verb |
| D-6 | The build tag `ze_test` becomes `le_test` in Phase 3 | the harness is `cmd/ze` with `ze_test` (`cmd/ze/ze_test_register.go`); `feature-gates.txt`, `./le repo feature-tags`, the workflows, the Dockerfiles and the le builders name it | the tag is a map row and a gate pattern. `zetest` (no underscore, `cmd/ze/plugins_zetest.go`) is a DIFFERENT tag, for the test plugins of the daemon under test, and is not renamed. The nftables table name `ze_test` (`internal/component/firewall/validate_test.go`, `docs/architecture/testing/qemu-integration.md`) is not a tag and is a declared gate exception |

#### The keywords of `perf run` (D-1, AC-24)

`perf run` keeps the keyword grammar of `perf-bench run` (`dutKeyword` and the
`run` action in `internal/le/perfbench/actions.go`) and takes over the options of
`perfrunner.RunCLI` (`internal/test/perfrunner/run.go`) as keywords. No `--build`
or `--test` option remains, because `ai/rules/cli.md` allows only `--help`,
`-h`, `--version` and `-V` in the option register.

| Old form | Where it was accepted | New form | Behavior |
|----------|-----------------------|----------|----------|
| `perf-bench run` (no keyword) | `perfbench` | `perf run` | builds the images, measures every DUT, records the history marker. This is what `perf-bench run` did, because it called the runner with `--build --test` (`measureArgs`) |
| `perf-bench run dut <names>` | `perfbench` | `perf run dut <names>` | one quoted value names one DUT or several, unchanged (`splitDUTs`) |
| positional DUT names after the options | `RunCLI` | `perf run dut <names>` | the same selection; an unknown name is refused by `validateDUTs`, as today |
| `--build --test` together | `RunCLI` | `perf run` (no `step` keyword) | both steps |
| `--build` alone | `RunCLI` | `perf run step build` | builds the Docker images of the selected DUTs, measures nothing, records nothing |
| `--test` alone | `RunCLI` | `perf run step test` | measures with the images that exist, skips a DUT whose image is absent (`imageExists`), records the history marker |
| neither option | `RunCLI` refused it with exit 2 | not reachable | a bare `perf run` runs both steps |

The `step` keyword takes one value, `build` or `test`. Any other value is
refused by name with exit 2. `step` and `dut` combine in either order. The host
`ze-perf` build that `perf-bench run` did first (`buildPerf`) is replaced by the
cross-build of `le` for linux (D-2).

#### What `doc index` checks (D-5, AC-29)

| Verb | Runs | Read at |
|------|------|---------|
| `doc index check` | 1. the check of `ai/DOCS-TO-CODE.md` against the tree, as `docs-to-code check` ran it (`Check` in `internal/le/docstocode/report.go`), 2. the anchor check that `docs-to-code index-check` ran (`CheckCodeIndex` in `internal/le/docstocode/codetodocs_report.go`): every `<!-- source: -->` anchor names a path that exists and symbols that are declared, 3. the check of `ai/CODE-TO-DOCS.md` against its rendering from the tree | fails when any of the three fails; one answer carries the result of all three |
| `doc index write` | the rewrite of `ai/DOCS-TO-CODE.md` (`Update`) and of `ai/CODE-TO-DOCS.md` (`UpdateCodeIndex`) | writes both files |

Step 3 is new behavior that D-5 asks for: `CheckCodeIndex` never compared
`ai/CODE-TO-DOCS.md` with its rendering. It compares the rendered bytes with
the file, as step 1 does for `ai/DOCS-TO-CODE.md`. Both files are untracked and
generated on demand (`internal/le/discoveryindex/sources.go`), so "stale" means
the working-tree file differs from the rendering. A missing file is generated,
as `Check` does today for `ai/DOCS-TO-CODE.md`.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/system-architecture.md` "Build personalities" - how `ze` and `le` share `cmd/ze`
  → Constraint: a normal `ze` build imports no `internal/le` package. The `ze_le` companion build imports `internal/le/register.go` and exposes it under `ze le`. Anything `internal/le` imports is linked into every `ze_le` build of `ze`.
  → Decision: the page is SILENT on the `ze_test`, `ze_chaos`, `ze_perf` and `ze_analyze` personalities and on `binarySuffixRoot`. That gap authorized reading `cmd/ze/dispatch.go`, `cmd/ze/ze_*_register.go`, `cmd/ze/ze_chaos_run.go` and `cmd/ze/le_build_name.go`. The page gains a paragraph on the harness personality in Phase 1 and loses the other three in Phase 3.
- [ ] `ai/INDEX.md` "Dev Tools" and "Add a development tool" - the le operator contract
  → Constraint: "a space in the name is a directory level": `le verify lint` lives at `internal/le/verify/lint/`. `TestEveryCommandIsFoundAtThePathItsNamePredicts` (`internal/le/group_test.go`) enforces it in both directions over two levels.
  → Constraint: `./le '|' json` (the manifest) is the authority for the command inventory. The rename map must not become a second inventory of NEW names, so it holds only old-to-new rows.
- [ ] `docs/guide/developer-setup.md` - the launcher contract
  → Constraint: `./le --name <n>` writes `bin/le-<n>/le`, and the platform fallback writes `bin/le-<uname -s>-<uname -m>/le`. Every `bin/le-*` name is launcher territory, so `--name test` collides with the `bin/le-test` file. D-3 resolves it: the launcher refuses the harness names.
- [ ] `docs/architecture/config/environment.md` and `internal/core/env/registry.go`, `env.go` - the env registry
  → Constraint: `MustRegister` does not filter on a `ze.` prefix, so `le.test.bin` registers. Lookup normalizes case and separators, so `LE_TEST_BIN` and `le.test.bin` are one variable.
  → Constraint: `Get(key)` resolves an alias to its canonical key, reads the canonical spelling, and reads the alias spelling ONLY when the CALLER passed the alias key. `warnDeprecated` fires on the canonical entry's `Deprecated` field. So an `Aliases` entry on `le.test.bin` does NOT make an environment that sets only `ZE_TEST_BIN` visible to `Get("le.test.bin")`. See Key Design Decisions.
- [ ] `spec-le-command-namespaces` - the previous rename; closed 2026-09-24 as superseded by this spec, file removed
  → Constraint: `commandWordsMax = 2` (`internal/le/leroot/dispatch.go`): a command is at most two words after `le`. Every new name here has two words or fewer, and `./le test harness bgp ...` resolves `test harness` and hands `bgp ...` to the tool.
  → Constraint: `TestNoMemberShadowsItsNamespaceRootVerb` refuses a member whose name is a verb of its namespace root. Checked for this map: `verify` (worktree, current, reds, list) against `evidence`; `site` against `terminal-demo`; `rfc` against `skeletons`; `repo` (check, tree-check, generate, generated-check) against its nine members; `spec` (current, claim, release, state, review, wip, model) against citation, roadmap, status and journal. No clash.
  → Decision: this spec REVERSES three recorded decisions of that spec: "The test-* family is not split", "docvalid and docs-to-code are left alone", and AC-14 "every stage writes the same log file name". The owner's 2026-09-24 approval is the authority. `leNamespaceExempt` (`internal/le/cligrammar/cligrammar.go`) carries the test-* and go-* reasoning in its comment and becomes empty.
- [ ] `plan/spec-le-builds-every-personality.md` - Status `skeleton`
  → Constraint: the tags of a harness build derive from `featuretags.DaemonBuildTags`, never a literal list. `./le test harness` builds `bin/le-test` through that helper.
- [ ] `docs/contributing/ze-go-style.md` - read in full before the design
  → Constraint: no compatibility shim once the migration ends. The alias layer is the approved exception, and Phase 3 deletes it whole.

**Key insights:**
- Three phases in one spec. The command alias layer is one mechanism driven by one map: an old word sequence is rewritten to a new one and dispatched again.
- The harness cannot be linked into `le`. Its roots (`bgp`, `web`, `lg`, `mcp`, `peer`, ...) collide with `ze` roots in a `ze_le` build, and `MustRegisterRootHandler` panics on a duplicate. So `./le test harness` execs `bin/le-test`.
- `ze-perf` and `ze-terminal-pty` also run inside containers. The harness stays a binary (`le-test`); the perf sender and the PTY driver become `le` itself, mounted into the container (D-2, A-4).
- `le` already links `gokrazy/tools/gok`, `internal/appliance`, `internal/perf` and `internal/test/perfrunner` (`go list -deps ./internal/le`, 2026-09-24). The new imports are `internal/chaos/orchestrator`, `internal/perf/cli` and `internal/analyze`.
- A verify stage rename renames its log file, because `stageLogPath` (`internal/le/verify/engine/run.go`) flattens the command name.
- The env registry reads an alias spelling only when the caller asks for the alias, so "both variable names read" needs its own mechanism.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/leroot/leroot.go` - `Register(name, group, answer, meta)` records the group and registers a LocalData handler at `le <name>`. `RegisterActions` records the action table. A name is registered once.
- [ ] `internal/le/leroot/dispatch.go` - `resolve` offers at most `commandWordsMax` (2) words to `registry.LookupLocalData`, and the longest match wins. `members` lists a namespace from `Commands()`. `Dispatch` answers a bare namespace token with its members and exit 1.
- [ ] `internal/le/register.go` - blank-imports every area once, then its own `init` registers the root `le`. Its `init` runs after the `init` of every imported package.
- [ ] `internal/le/group_test.go` - `directoryFor` (a space is a level, a hyphen is removed), `TestEveryCommandIsFoundAtThePathItsNamePredicts`, `TestNoRegisteredLeCommandExceedsTwoWords`, `TestNoMemberShadowsItsNamespaceRootVerb`, `TestEveryCommandRegistersItsOwnAnswerShape`.
- [ ] `internal/le/cligrammar/cligrammar.go` - feeder 4 scans registered roots from Go source against a `floor.Roots` count. Feeder 6 checks the hyphenated le names, with `leNamespaceExempt` holding test-* and go-extract/go-version.
- [ ] `internal/le/verify/lock/answer.go` - `verify lock run label <l> command <argv>` calls `job.NewIn(root).Run`. It is `job run` without the `quiet` keyword.
- [ ] `internal/le/spec/session/actions.go` - a bare call answers the current spec; verbs `wip`, `model`, `current`, `claim`, `release`, `state`, `review`.
- [ ] `internal/le/verify/engine/run.go` `stageLogPath`, `stages.go` - stage names are command strings; the log file name is the flattened stage name.
- [ ] `cmd/ze/dispatch.go` - `defaultDispatch` tries `registry.LookupRoot(binaryName())`, then argv[0] as a root, then `binarySuffixRoot`.
- [ ] `cmd/ze/le_build_name.go` - `refuseWrongBuildName` and `invokedBuildName`, described in the harness table above.
- [ ] `cmd/ze/ze_test_register.go`, `ze_perf_register.go`, `ze_analyze_register.go`, `ze_chaos_run.go` - one blank import each, under `ze_test`, `ze_perf`, `ze_analyze`, `ze_chaos`.
- [ ] `internal/test/cli/dispatch.go`, `register.go` - about 30 ROOTS (`bgp`, `editor`, `web`, `peer`, `lg`, `mcp`, `rpki`, `engine-steps`, `record-plugin`, ...). Test daemons spawn some of them; people do not type them.
- [ ] `internal/perf/cli/register.go` - root `perf`, subcommands `run`, `report`, `track`, and a `--version` answer.
- [ ] `internal/analyze/register.go` - root `analyze`, 15 subcommands: attributes, communities, count-attrs, aspath, mrt-dump, statistics, filter, inject, replay, convert, export, record, show, routes, serve.
- [ ] `internal/chaos/orchestrator/register.go` - root `chaos`, calling `cLIRun(args)`.
- [ ] `cmd/ze-gok/main.go` - wraps `github.com/gokrazy/tools/gok` and registers `ze.gok.debug` and `ze.gok.kernel-package`. `internal/appliance/cmd_build.go` already runs gok in-process. The only exec of the binary is `buildGokrazyImage` (`internal/le/deployment/gokrazyimage.go`).
- [ ] `cmd/ze-perf-run/main.go` - calls `perfrunner.New(root, ...).RunCLI(args)`.
- [ ] `cmd/ze-terminal-pty/main.go` - calls `terminaldemo.RunPTY`. The demo container execs it (`demoBinary("ze-terminal-pty")` in `internal/le/terminaldemo/entrypoint.go` and `validate_runtime.go`). It is host-built (`internal/le/terminaldemo/actions.go`).
- [ ] `internal/le/functional/binaries.go` - builds `ze`, `ze-stripped`, the harness (`ZeTest = "ze-test"`, `internal/le/functional/suites.go`), `ze-chaos` when `extras.Chaos`, and `le` when `extras.LE`. It exports `ZE_TEST_BIN` and `ZE_TEST_NO_BUILD`.
- [ ] `internal/test/perfrunner/run.go` - `ZE_PERF_BIN` default `bin/ze-perf`; builds `bin/ze-perf-linux`; runs `/usr/local/bin/ze-perf run` in a container.
- [ ] `internal/le/qemu/netns_linux.go`, `guest_linux.go`, `run.go`, `alltests.go` - the harness variables, described above.
- [ ] `internal/le/perfbench/actions.go`, `bench.go` - verbs `run`, `suggestion-report`, `record`, `history-record`, `evidence-record`; `run` takes the optional keyword `dut <names>` (`splitDUTs`, `validateDUTs`), builds the host `ze-perf` (`buildPerf`), calls the runner with `--build --test` (`measureArgs`), and records a history marker.
- [ ] `internal/test/perfrunner/run.go` `RunCLI` - GNU options `--build` and `--test` (at least one, else exit 2), then positional DUT names (none means every DUT; no match exits 1).
- [ ] `internal/le/docstocode/actions.go`, `report.go`, `codetodocs_report.go` - `check` compares `ai/DOCS-TO-CODE.md` with its rendering and generates it when absent (`Check`); `index-check` checks that anchor paths exist and anchor symbols are declared (`CheckCodeIndex`), and does NOT compare `ai/CODE-TO-DOCS.md` with its rendering; `update` and `index-update` rewrite the two files. Neither file is tracked.
- [ ] `internal/core/env/env.go` `Get`, `internal/core/env/registry.go` `MustRegister`, `resolveAlias`, `warnDeprecated` - described under Required Reading.
- [ ] `internal/component/bgp/reactor/reactor_peers.go` `envKeyTestPort`, `internal/component/vpp/trace_linux.go` - product readers of `ze.test.*` keys.
- [ ] `internal/le/hookruntime/bash.go` - an argv[0] of `ze-test` or `ze-test-*` is the functional runner, refused outside job admission.
- [ ] `internal/le/weekly/answer.go` - `channels` holds the publication channel name `ze-test`, which is not the binary.
- [ ] `le` (root launcher) - `check_name`, `--name`, the platform fallback directory.

**Behavior to preserve:**
- Every action of every command: its write marker, answer shape, group, exit codes and structured output. A rename changes the words that reach the handler and nothing the handler does.
- The pipe operators on every command (`| json`, `| yaml`, `| table`).
- Job admission for every heavy command, including the harness under its new name.
- `le-test` is built with `ze_test` plus the feature tags from `featuretags`, is cross-compiled for containers and QEMU guests, and is never linked into `le`.
- `ze-installer`, `ze-serial-shell` and `ze-host` keep their names. They are target binaries, or `ze` built for the host.
- The Prometheus metric names `ze_chaos_*` (`internal/chaos/report/metrics.go`) do not change.
- The env keys `ze.gok.debug`, `ze.gok.kernel-package`, `ze.chaos.*`, `ze.test.bgp.port` and `ze.test.vpp.cli.socket` do not change. They name a subsystem or a product setting, not the harness.

**Behavior to change:**
- Every name in the rename map, the program entries, the harness binary name and the three harness variables.
- The `verify` stage names, so their log file names (`01-go-lint-run.log` where it was `01-verify-lint-run.log`).
- `leNamespaceExempt` becomes empty.
- The two defaults of `ze.qemu.test.bin` become one (defect).
- Phase 3 deletes `binarySuffixRoot`, the `ze_chaos` and `ze_analyze` build tags and their register files, `cmd/ze-gok`, `cmd/ze-terminal-pty`, the aliases and the old variable reads. Per D-1 and D-2 it also deletes the `ze_perf` build tag and `cmd/ze/ze_perf_register.go`, `cmd/ze-perf-run`, `bin/ze-perf`, `bin/ze-perf-linux` and `ZE_PERF_BIN`. Per D-6 it renames the harness build tag `ze_test` to `le_test`.
- `perf run` accepts every option `cmd/ze-perf-run` accepted, as keywords (D-1, AC-24). See "The keywords of `perf run`" under Owner decisions.
- `docs-to-code` becomes `doc index` with two verbs (D-5, AC-29). See "What `doc index` checks" under Owner decisions.
- The launcher `le` refuses the harness names (D-3, AC-28).

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- argv to the `le` personality: `./le <words>`, `ze le <words>` (a `ze_le` build), or a CI step. Format: shell words.
- the environment of a harness process: `LE_TEST_BIN`, `LE_QEMU_TEST_BIN`, `LE_TEST_NO_BUILD`, and their `ZE_` spellings in Phases 1 and 2.

### Transformation Path
1. `defaultDispatch` (`cmd/ze/dispatch.go`) resolves root `le` from the binary name, or argv[0] `le`.
2. `leroot.Dispatch` resolves the longest registered command in the first two words.
3. A NEW name reaches the `Answer` of its area directly.
4. An OLD name (Phases 1 and 2) reaches the alias handler. The handler finds the longest map row whose old words prefix the command plus its argv, prints one stderr line that names the new words, and calls `Dispatch` again with the new words plus the remaining argv. The exit code is the code of the new command.
5. `./le test harness <argv>` builds `bin/le-test` when it is absent, through job admission, then execs it with `<argv>` and returns its exit code.
6. A harness variable read goes through one resolver per variable: the `le.` key first, then (Phases 1 and 2) the `ze.` key, which warns once that it is deprecated.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| le process to harness process | exec of `bin/le-test` with the trailing argv; the exit code passes through | No |
| host to container or QEMU guest | the cross-compiled `le-test-linux-<arch>` file; `LE_QEMU_TEST_BIN` on the host, forwarded as `LE_TEST_BIN` in the guest | No |
| Claude Code hooks to le | `.claude/settings.json` runs `le ai hooks <action>` | No |
| CI to le | `.github/workflows/*.yml` steps; `verify.yml` reads the stage list from `./le verify list mode full` | No |

### Integration Points
- `leroot.Register` and `leroot.RegisterActions` - every moved package keeps its registration, with the new name.
- `internal/le/register.go` - blank-imports the moved packages at their new paths; its `init` registers the aliases after every area.
- `internal/le/verify/engine/stages.go` - the stage table names commands by string, and takes the new names.
- `internal/le/doc/check/links.go` `sweepTracked` - the tracked-file walk the retired-name gate reuses.
- `featuretags.DaemonBuildTags` - the harness build tags.
- `internal/core/env` `EnvEntry.Deprecated` - the one-time warning for an old variable spelling.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | an alias dispatches again through `leroot.Dispatch` and never calls an area handler directly; the three raw variable reads move to `env.Get` |
| No unintended coupling (components stay isolated) | Yes | the harness is exec'd, not imported, so its roots never enter a `ze_le` build |
| No duplicated functionality (extends existing, does not recreate) | Yes | `verify lock` merges into `job`; `build gokrazy` reuses the in-process gok path of `internal/appliance/cmd_build.go` |
| Zero-copy preserved where applicable (refs, not copies) | N-A | tooling, no wire path |
| Registration over hardcoding, outbound | Yes | every new command registers through `leroot.Register` in its own package |
| Registration over hardcoding, inbound | Yes | searched lists that hold names by literal: `internal/le/verify/engine/stages.go`, `leNamespaceExempt`, `areasWithoutAnActionTable` (`internal/le/actions_test.go`), `internal/le/completeness_record_test.go`, `internal/le/repository/trackedbuild/matrix.go`, `internal/le/verify/lint/matrix.go`, the `ze_test`/`ze_chaos` exemption in `internal/le/command/ownership/commandownership.go`. Each is edited in the phase that renames its names; the alias layer and the gate derive from the one map |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | An alias registered from the `init` of `internal/le/register.go` sees every new name, because that `init` runs after all blank imports | Go init order; `internal/le/register.go` already registers root `le` there | the alias of a name whose area has not registered misroutes | `TestEveryRetiredNameRunsItsNewCommand` over the full map | confirmed, moot (Phase 1a): no alias is registered. `leroot.Dispatch` reads the map through `retiredRewrite` (`internal/le/leroot/retired.go`) at call time, after every `init`, and rewrites only when the row's new command is registered, so `internal/le/register.go` needs no change |
| A-2 | A directory named `go` under `internal/le` builds. Only a package NAMED `go` is illegal, and no `register.go` lives at `internal/le/go` | Go spec: `go` is a keyword; directory names are free | the `go` family needs another directory | `go build ./internal/le/go/...` in Phase 1 | unvalidated |
| A-3 | `internal/test/cli` cannot be imported into `internal/le` | root `bgp` is registered by `internal/component/bgp/cli/register.go` and by `internal/test/cli/register.go`; `MustRegisterRootHandler` panics on a duplicate (`internal/component/command/registry/registry.go`) | an in-process harness would be simpler | read of both registrations, 2026-09-24 | validated |
| A-4 | The host-built `le` runs inside the terminal-demo container, as `ze-terminal-pty` does today | both are host-built Go binaries from one toolchain (`internal/le/terminaldemo/actions.go`) | the demo image needs its own `le` build | `./le site terminal-demo check-all` after the Phase 1 move | unvalidated |
| A-5 | `le.` keys register in `internal/core/env` | `MustRegister` has no prefix filter; grep of `ai/rules/config.md` found no prefix rule (2026-09-24) | registration refuses `le.` keys | `TestHarnessVariablesReadBothNames` registers and reads `le.test.bin` | unvalidated |
| A-6 | Rewriting a historical record (journal row, learned file, spec text) from an old command name to its new name keeps its meaning, because the command still exists under the new name | owner goal G-2 names every tracked file | a record changes meaning | review of the Phase 2 diff of `plan/journal/` and `plan/learned/` | unvalidated |
| A-7 | A linux `le` cross-built with `CGO_ENABLED=0` runs in the perf sender container and links the BGP code `perf send` needs | `build_le` in the launcher `le` sets `CGO_ENABLED=0` and the `feature-gates.txt` tags | the container needs another payload | AC-26 and AC-27 | unvalidated |
| A-8 | `ze.test.bgp.port` is a product setting, because the daemon reads it | `envKeyTestPort` in `internal/component/bgp/reactor/reactor_peers.go` | an `le.` key would put a harness name into the product, or the two processes would read different ports | owner review of this spec | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A hook command moves and every Claude session breaks on its next tool call | a hook error on the first call after the commit | Phase 1 keeps `hook-check` as an alias; `.claude/settings.json` moves to `ai hooks` in Phase 1 with the `internal/le/hookcheck` parity test green before the commit |
| R-2 | The alias stderr line breaks a test that asserts an empty stderr | a red test that names an old command | that test is a Phase 2 caller; move it to the new name |
| R-3 | Go code that builds argv from string literals (`"le", "functional"`) is invisible to a text search | the alias stderr line in a test or CI log | the gate reads Go string-literal sequences after `"le"` as well as text forms (AC-15) |
| R-4 | Peer sessions hold a `bin/le` from before Phase 1 and call new names it does not know | "unknown command" for a new name in a peer session | docs publish new names only in Phase 2, after Phase 1 has landed |
| R-5 | Verify stage log names change, and CI or fixtures read the old ones | the failure index or a fixture looks for `NN-verify-lint-run.log` | the stage list is derived (`./le verify list mode full`); fixtures that name a file are Phase 2 callers |
| R-6 | le build time grows from the new imports | the Phase 1 cold build is slower than the baseline | measure and record (AC-21); an unacceptable delta goes to the owner as a number |
| R-7 | A peer session's spec text names old commands after Phase 3 | the gate names file and line in that session's commit | the old name fails loudly, never silently |
| R-8 | The `--name test` directory and the `bin/le-test` file collide | a launcher build error | D-3: the launcher refuses the name (AC-28) |
| R-9 | `cli grammar` feeder 4 counts registered roots against `floor.Roots`; deleting roots `perf`, `analyze` and `chaos` lowers the count | a floor refusal from `./le cli grammar` | lower the floor by exactly the deleted roots, and give the count in the commit message |
| R-10 | The harness is renamed in about 1500 tracked files, most of them `.ci` files that exec `ze-test` | the Phase 2 report | one mechanical rewrite driven by the map, reviewed as one diff |
| R-11 | A shell exports `ZE_TEST_BIN` and a stale `LE_TEST_BIN` at once, and the new name wins | a run uses an unexpected binary | the deprecation warning names both spellings when both are set |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Developer and agent workflows: hooks, CI workflows, verify stages, functional, interop and QEMU suites. Nothing user-visible in the shipped `ze` |
| How is it reverted? | Phases 1 and 2 are additive and revert per commit. Phase 3 reverts as one commit |
| Who else touches this path? | Every session in this checkout calls `./le`. `plan/spec-le-every-area-dispatches-through-one-table.md`, `plan/spec-le-one-verb-one-job.md` and `plan/spec-le-builds-every-personality.md` (skeletons) edit the same packages |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le <new words>` for every map row | → | the `Answer` of the moved area | `TestEveryNewNameResolvesToItsArea` in `internal/le/rename_test.go` |
| `./le <old words>` in Phases 1 and 2 | → | the alias handler in `internal/le/leroot/retired.go` | `TestEveryRetiredNameRunsItsNewCommand` in `internal/le/leroot/retired_test.go` |
| `./le test harness bgp --list` | → | the exec of `bin/le-test` in `internal/le/test/harness` | `TestHarnessExecsLeTestWithTheTrailingArgv` in `internal/le/test/harness/harness_test.go` |
| `ZE_TEST_BIN` or `LE_TEST_BIN` in a runner environment | → | the resolver of `le.test.bin` in `internal/test/runner/runner.go` | `TestHarnessVariablesReadBothNames` in `internal/test/runner/runner_test.go` |
| `./le chaos run --help` | → | the `internal/chaos/orchestrator` CLI entry | `TestChaosRunReachesTheOrchestrator` in `internal/le/chaos/run/run_test.go` |
| `./le mrt statistics <file>` | → | `Dispatch` of `internal/analyze` | `TestMrtReachesEveryAnalyzeSubcommand` in `internal/le/mrt/mrt_test.go` |
| `./le build gokrazy <gok args>` | → | the gok wrapper moved from `cmd/ze-gok` | `TestBuildGokrazyPreparesTheInstance` in `internal/le/build/gokrazy/gokrazy_test.go` (the table of `cmd/ze-gok/main_test.go`, moved) |
| a `.ci` file that execs `le chaos run` | → | the functional runner with `extras.LE` | `test/chaos/smoke-chaos.ci` after its Phase 2 edit |
| `./le doc check retired-commands` | → | the sweep over tracked files | `TestRetiredCommandSweepFindsAnInjectedName` in `internal/le/doc/check/retired_test.go` |
| `ze le go lint run` in a `ze_le` build | → | the same area through `ze` | `internal/test/fixture/ui_fixture_le_subject_first_dispatch.go` |
| `./le perf run step build dut ze` | → | the `run` action of `internal/le/perf`, then the perf runner | `TestPerfRunAcceptsRunnerOptionsAsKeywords` in `internal/le/perf/perf_test.go` (AC-24) |
| `./le perf suggest`, `perf record`, `perf history-record`, `perf evidence-record`, `perf send`, `perf report`, `perf track` | → | the action table of `internal/le/perf`; the last three reach `internal/perf/cli` | `TestPerfVerbsAnswerAsTheirPredecessors` in `internal/le/perf/perf_test.go` (AC-24, AC-25) |
| a perf suite run | → | the linux `le` cross-build and mount in `internal/test/perfrunner/run.go` | `TestPerfRunnerMountsLinuxLe` in `internal/test/perfrunner/run_test.go` (AC-26) |
| `le perf send --help` from a file named `le` with `ZE_LE_BUILD_NAME` unset | → | `defaultDispatch` and `refuseWrongBuildName` in `cmd/ze` | `TestPerfSendAnswersWithoutLauncherEnv` in `cmd/ze/le_build_name_test.go` (AC-27) |
| `./le --name test`, `./le --name test-linux-amd64` | → | `check_name` in the launcher `le` | `TestLeLauncherRefusesHarnessNames` in `cmd/ze/root_launcher_test.go` (AC-28) |
| `./le doc index check`, `./le doc index write` | → | `internal/le/doc/index` | `TestDocIndexCheckCoversBothFilesAndAnchors` in `internal/le/doc/index/index_test.go` (AC-29) |
| a harness build in the Phase 3 tree | → | `TestBuildTags` in `internal/test/runner/runner.go` and `cmd/ze/le_test_register.go` | `TestHarnessTagIsLeTest` in `internal/test/runner/runner_test.go` (AC-30) |
| `./le go`, `./le test`, `./le spec` | → | `leroot.Dispatch` answering a bare namespace token | `TestBareNamespaceTokenListsItsMembers` in `internal/le/leroot/namespace_test.go` (AC-31, existing) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `./le <new words> <action> <args>` for every row of the rename map and every unchanged name | runs the action the old name ran, with the same output, write behavior and exit code |
| AC-2 | `./le '|' json` after Phase 1 | lists every new name and no old name; the group of each command is the group its old name had |
| AC-3 | `./le <old words> <rest>` in Phases 1 and 2 | writes one stderr line that names the new words, then answers exactly as `./le <new words> <rest>`, with the same exit code |
| AC-4 | a row that splits verbs: `./le build-artifacts installer-arm64` | answers as `./le build installer arm64` |
| AC-5 | a row that merges: `./le verify lock run label x command true` | answers as `./le job run label x command true` |
| AC-6 | any registered new command | its package is at the directory `directoryFor` predicts, and no directory under `internal/le` registers an old name |
| AC-7 | `./le test harness <root> <args>` | builds `bin/le-test` when it is absent, with `ze_test` plus the feature tags, through job admission; execs it with `<root> <args>`; returns its exit code. `./le test harness` alone lists the harness roots |
| AC-8 | a functional, interop, QEMU or stress run in Phases 1 and 2 | the builders write `le-test` and the `ze-test` hard link; the interop image holds `/usr/local/bin/le-test`; the QEMU guest receives `le-test-linux-<arch>` |
| AC-9 | each of `le.test.bin`, `le.qemu.test.bin`, `le.test.no.build`, set under the `LE_` spelling, the `ZE_` spelling, or both, in Phases 1 and 2 | the reader uses the value; when both are set, the `LE_` value wins; a value that came from a `ZE_` spelling prints one deprecation line naming the `LE_` spelling |
| AC-10 | `internal/le/qemu/alltests.go`, `netns_linux.go`, `guest_linux.go` | no raw read of a harness variable remains; `le.qemu.test.bin` has one default, derived from the guest architecture |
| AC-11 | argv[0] of `le-test` or `le-test-<suffix>` in a Bash tool call | the pretool hook treats it as the functional runner and requires job admission, as for `ze-test` |
| AC-12 | `./le chaos run <args>` | behaves as `ze-chaos <args>` did, including `--in-process --web` from a `.ci` file |
| AC-13 | `./le mrt <subcommand> <args>` for each of the 15 analyze subcommands, and `./le mrt` alone | each runs its subcommand; the bare word lists all 15 |
| AC-14 | `./le build gokrazy`, `build installer amd64`, `build installer arm64`, `build host-driver`, `build gosum` | each runs what `ze-gok`, `build-artifacts installer-amd64`, `installer-arm64`, `host` and `gokrazy-gosum` ran; `internal/le/deployment` no longer compiles `./cmd/ze-gok` |
| AC-15 | `./le doc check retired-commands report` (Phases 1 and 2) | lists, per map row, the tracked files and lines that name the old form, as structured data. It matches `./le <old>`, `le <old>`, `ze le <old>`, `$CLAUDE_PROJECT_DIR/le <old>`, a Go string-literal sequence `"le"` followed by the old words, the old program names, the build tags `ze_chaos`, `ze_analyze`, `ze_perf` and `ze_test` (never `zetest`, never `ze_chaos_` metric names), the harness file names, `bin/ze-perf`, `bin/ze-perf-linux`, `ZE_PERF_BIN`, and the three old variables in any case or separator form |
| AC-16 | `./le doc check retired-commands` (Phase 3) | exits 1 and names file and line for every match outside `vendor/`, the file that declares the map, and the declared exceptions (the `ze-test` channel in `internal/le/weekly/answer.go`, the nftables table name `ze_test`); exits 0 on the Phase 3 tree; runs as a stage of `./le verify current mode full` |
| AC-17 | the Phase 3 tree | every old name answers `unknown command` and exit 1; `cmd/ze` has no `ze_chaos`, `ze_analyze` or `ze_perf` register file; `binarySuffixRoot` is gone; `cmd/ze-gok`, `cmd/ze-terminal-pty` and `cmd/ze-perf-run` are gone; nothing reads `ZE_PERF_BIN`; the tracked-build and lint matrices hold no deleted tag; no builder writes `ze-test`; no `ze.test.bin`, `ze.qemu.test.bin` or `ze.test.no.build` entry is registered |
| AC-18 | `./le verify list mode full` | every stage names a new command, and each log file is named after the new command |
| AC-19 | `./le cli grammar` | green with `leNamespaceExempt` empty |
| AC-20 | every hook event in `.claude/settings.json` | runs `le ai hooks <action>`, and the `internal/le/hookcheck` parity and fixture tests pass |
| AC-21 | a cold build of `bin/le` before and after Phase 1 | both times are recorded in this spec, with the machine named |
| AC-22 | `./le arch tier check` and `./le repo feature-tags check` after Phase 1 | both green with the new imports into `internal/le` |
| AC-23 | the Programs table in `ai/INSTRUCTIONS.md` | the `ze-analyse` misspelling is gone from Phase 2; after Phase 3 the table lists `ze`, `le-test` and the target binaries only |
| AC-24 | `./le perf run` with the keywords of `perf-bench run`, and with the build, test and DUT selection `cmd/ze-perf-run` accepted | runs the cross-DUT suite as `perf-bench run` and `go run ./cmd/ze-perf-run` did; `./le perf suggest`, `perf record`, `perf history-record` and `perf evidence-record` answer as the perf-bench verbs they replace |
| AC-25 | `./le perf send`, `perf report`, `perf track` on the host | each runs what `ze-perf run`, `ze-perf report` and `ze-perf track` ran; `./le perf track --check test/perf/history/ze.ndjson` gives the exit code `bin/ze-perf track --check` gave, and `.github/workflows/perf-nightly.yml` runs it from Phase 2 |
| AC-26 | a perf suite run | the runner cross-compiles `le` for linux with `CGO_ENABLED=0` and the container's architecture, mounts it at `/usr/local/bin/le`, runs `le perf send` in the sender container, and writes no `bin/ze-perf` or `bin/ze-perf-linux`. The le-linux cross-build time and the ze-perf-linux build time it replaces are both recorded in this spec, with the machine named |
| AC-27 | `le perf send --help` in a container started with no launcher environment (`ZE_LE_BUILD_NAME` unset) | answers with exit 0: the file named `le` selects root `le` and `refuseWrongBuildName` does not refuse |
| AC-28 | `./le --name test`, `./le --name test-linux-amd64`, and a name whose `bin/le-<name>` equals a `bin/le-test*` artifact path | the launcher exits 2 and names the artifact that owns the path; any other valid name still builds `bin/le-<name>/le` |
| AC-29 | `./le doc index check` and `./le doc index write` | `check` fails when `ai/DOCS-TO-CODE.md` or `ai/CODE-TO-DOCS.md` is stale or an anchor does not resolve, as `docs-to-code check` plus `index-check` did; `write` regenerates both files |
| AC-30 | the Phase 3 tree | the harness is built with the tag `le_test`; no file under `cmd/`, `feature-gates.txt`, `.github/`, the Dockerfiles or `internal/le` names the tag `ze_test`; `zetest` is unchanged |
| AC-31 | folded from the superseded spec (its AC-11): `./le go`, `./le test`, `./le spec` or any other bare namespace token | lists the members of the namespace with their descriptions; an unknown first word still prints `unknown command` and exits 1. The bare token exits 0 (owner decision, 2026-09-24): `Dispatch` changes from 1 to 0 and `TestBareNamespaceTokenListsItsMembers` pins 0 |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestEveryNewNameResolvesToItsArea` | `internal/le/rename_test.go` | AC-1, AC-2: every new name of every map row is registered and resolves | |
| `TestEveryRetiredNameRunsItsNewCommand` | `internal/le/leroot/retired_test.go` | AC-3, AC-4, AC-5: the stderr line, the same payload under `| json`, the same code | |
| `TestRetiredNamesAreNotInTheManifest` | `internal/le/leroot/retired_test.go` | AC-2 | |
| `TestRenameMapRowsAreDisjoint` | `internal/le/leroot/retired_test.go` | no old words are also a new name; no two rows share old words | |
| `TestEveryCommandIsFoundAtThePathItsNamePredicts` (updated) | `internal/le/group_test.go` | AC-6: aliases are not commands | |
| `TestHarnessExecsLeTestWithTheTrailingArgv` | `internal/le/test/harness/harness_test.go` | AC-7 and the exit code passthrough | |
| `TestHarnessBuildUsesFeatureTags` | `internal/le/test/harness/harness_test.go` | AC-7: tags from `featuretags` | |
| `TestHarnessVariablesReadBothNames` | `internal/test/runner/runner_test.go` | AC-9: each of the three variables, each spelling, both set, the warning | |
| `TestQemuHarnessVariableHasOneDefault` | `internal/le/qemu/run_test.go` | AC-10: the defect | |
| `TestBashHookAdmitsLeTest` | `internal/le/hookruntime/bash_test.go` | AC-11 | |
| `TestChaosRunReachesTheOrchestrator` | `internal/le/chaos/run/run_test.go` | AC-12 | |
| `TestMrtReachesEveryAnalyzeSubcommand` | `internal/le/mrt/mrt_test.go` | AC-13, derived from the analyze subcommand registry | |
| `TestBuildGokrazyPreparesTheInstance` | `internal/le/build/gokrazy/gokrazy_test.go` | AC-14 | |
| `TestRetiredCommandSweepFindsAnInjectedName` | `internal/le/doc/check/retired_test.go` | AC-15, AC-16: one case for each match form, including a Go literal sequence and a lower-case dotted variable key | |
| `TestRetiredCommandSweepHonorsDeclaredExceptions` | `internal/le/doc/check/retired_test.go` | the `ze-test` channel and the `ze_chaos_` metric prefix are not matches | |
| `TestStageLogPathFollowsTheNewName` | `internal/le/verify/engine/run_test.go` | AC-18 | |
| stage log name pins (updated) | `internal/le/verify/stagelogname_test.go` | AC-18: the pinned file names follow the new stage names | |
| `TestPerfRunAcceptsRunnerOptionsAsKeywords` | `internal/le/perf/perf_test.go` | AC-24: each row of "The keywords of `perf run`" reaches the runner with the steps and DUTs it names; a `step` value other than `build` or `test` exits 2 and names the value | |
| `TestPerfVerbsAnswerAsTheirPredecessors` | `internal/le/perf/perf_test.go` | AC-24, AC-25: every perf verb is registered, `suggest` answers as `suggestion-report` did, and `send`, `report`, `track` reach `internal/perf/cli` with the trailing argv and its exit code | |
| `TestPerfRunnerMountsLinuxLe` | `internal/test/perfrunner/run_test.go` | AC-26: the cross-build is `GOOS=linux`, `CGO_ENABLED=0`, the container's `GOARCH`; the mount target is `/usr/local/bin/le`; the sender argv starts `le perf send`; no `bin/ze-perf*` path is written | |
| `TestPerfSendAnswersWithoutLauncherEnv` | `cmd/ze/le_build_name_test.go` | AC-27: a file named `le` with `ZE_LE_BUILD_NAME` unset answers `perf send --help` with exit 0 | |
| `TestLeLauncherRefusesHarnessNames` | `cmd/ze/root_launcher_test.go` | AC-28: `test` and `test-linux-amd64` exit 2 and name the artifact; a valid other name still builds | |
| `TestDocIndexCheckCoversBothFilesAndAnchors` | `internal/le/doc/index/index_test.go` | AC-29: a stale `ai/DOCS-TO-CODE.md`, a stale `ai/CODE-TO-DOCS.md` and an unresolved anchor each fail `check`; `write` regenerates both files | |
| `TestHarnessTagIsLeTest` | `internal/test/runner/runner_test.go` | AC-30: `TestBuildTags` starts with `le_test` and holds no `ze_test` | |
| `TestBareNamespaceTokenListsItsMembers` (existing, updated) | `internal/le/leroot/namespace_test.go` | AC-31: members listed with descriptions, exit 0 (changed from 1); an unknown first word answers `unknown command` and exit 1 | |
| `TestRetiredCommandSweepHonorsDeclaredExceptions` (extended) | `internal/le/doc/check/retired_test.go` | AC-15, AC-16: `zetest` and the environment spelling `ze_test_bgp_port` are not matches of the tag `ze_test` | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| words in a command name | 1-2 | 2 | 0 (a bare `./le` answers the manifest) | 3 (`TestNoRegisteredLeCommandExceedsTwoWords`) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `le-subject-first-dispatch` | `internal/test/fixture/ui_fixture_le_subject_first_dispatch.go` | a developer runs a new name and an old name and gets the same answer; the old one adds its stderr notice | |
| `smoke-chaos` (edited) | `test/chaos/smoke-chaos.ci` | the chaos suite runs through `le chaos run` | |
| `dashboard-counters` (edited) | `test/chaos-web/dashboard-counters.ci` | the chaos web dashboard runs through `le chaos run --in-process --web` | |
| interop `bgp` scenarios | `test/interop/scenarios/` | the interop image runs `le-test` | |

### Interop Tests (Scope: protocol)
N-A: tooling. The interop suites run as regression proof that the harness rename reaches the container (AC-8).

## Files to Modify
- `internal/le/leroot/dispatch.go`, `leroot.go` - the alias dispatch entry; aliases hidden from `Commands()`; `Dispatch` answers a bare namespace token with exit 0 (AC-31)
- `internal/le/register.go` - new import paths; alias registration after all areas
- every package in the rename map - moved to its new directory, registration name changed
- `internal/le/verify/engine/stages.go`, `run.go` - stage names; the `stageLogPath` comment that pins the old file name
- `internal/le/cligrammar/cligrammar.go` - `leNamespaceExempt` emptied and its comment rewritten; `floor.Roots` in Phase 3
- `internal/le/group_test.go`, `internal/le/actions_test.go`, `internal/le/completeness_record_test.go` - names
- `internal/le/functional/binaries.go`, `suites.go`, `exabgp.go` - harness name and variables; the `ze-chaos` build removed in Phase 3
- `internal/le/qemu/netns_linux.go`, `guest_linux.go`, `run.go`, `alltests.go`, `run_exec.go` - harness name; variables through the registry; one default
- `internal/le/interoplab/bgp/names.go`, `run.go` - harness file name
- `internal/le/integration/stress.go`, `stressbird.go`, `internal/le/stressrepro/run.go`, `process.go`, `internal/le/deployment/l2tpscale.go`, `vppevidence.go`, `gokrazyimage.go` - harness name and variables; gok in-process
- `internal/le/terminaldemo/*.go` (moved to `site/terminaldemo`) - the `pty` action, no `cmd/ze-terminal-pty` build, `ze-test` in scenarios
- `internal/le/hookruntime/bash.go` - `le-test` admission
- `internal/le/perfbench/*.go` - moved to `internal/le/perf` (D-1); `suggestion-report` becomes `suggest`; `run` gains the `step` keyword; `buildPerf` and `perfTags` are deleted in favor of the le linux cross-build (D-2)
- `internal/test/perfrunner/run.go` - `RunCLI` and its `flag` parsing deleted with `cmd/ze-perf-run`; the runner builds and mounts a linux `le` at `/usr/local/bin/le` and runs `le perf send` in the sender container; `ZE_PERF_BIN`, `bin/ze-perf` and `bin/ze-perf-linux` go (D-2)
- `internal/perf/cli/register.go` and its `run` subcommand - reached as `perf send`, `perf report`, `perf track` from `internal/le/perf` (D-1)
- `internal/le/verify/stagelogname_test.go` - pins the stage log file names; takes the new names (AC-18)
- `feature-gates.txt` - checked in Phase 3 for `ze_test` (D-6 names it). On 2026-09-24 it holds no `ze_test` line and no comment naming it (`grep -c`), so the edit is empty unless a peer session adds one. The harness tag literal lives in `TestBuildTags` (`internal/test/runner/runner.go`), which takes `le_test` in Phase 3
- `internal/test/runner/runner.go` `TestBuildTags` - the `ze_test` literal becomes `le_test` in Phase 3 (D-6). The environment spelling `ze_test_bgp_port` in `internal/test/runner/runner_exec.go` is the product key `ze.test.bgp.port`, not the tag, and the gate must not match it
- `cmd/ze/ze_test_register.go` - renamed `cmd/ze/le_test_register.go`, build tag `le_test`, in Phase 3 (D-6). `cmd/ze/plugins_zetest.go` (tag `zetest`) is not touched
- the le files that name `ze_test` (grep 2026-09-24): `internal/le/functional/binaries.go`, `internal/le/terminaldemo/actions.go`, `internal/le/deployment/vppevidence.go`, `internal/le/interoplab/zebuild.go`, `interoplab/bgp/run.go`, `interoplab/l2tp/l2tp.go`, `interoplab/pppoe/pppoe.go`, `internal/le/verify/lint/matrix.go`, `internal/le/repository/trackedbuild/matrix.go`, `internal/le/command/ownership/commandownership.go` - `le_test` in Phase 3 (D-6)
- `internal/le/docstocode/*.go` - moved to `internal/le/doc/index`; four verbs become `check` and `write`; `check` gains the comparison of `ai/CODE-TO-DOCS.md` with its rendering (D-5)
- `internal/le/repository/trackedbuild/matrix.go`, `internal/le/verify/lint/matrix.go`, `internal/le/command/ownership/commandownership.go` - deleted tags in Phase 3
- `internal/test/runner/runner.go`, `internal/test/cli/cmd_bgp.go`, `cmd_web.go`, `internal/test/sessionpath/sessionpath.go` - the `le.test.*` keys and their resolvers
- `internal/chaos/orchestrator/register.go`, `cli.go`; `internal/perf/cli/register.go`; `internal/analyze/register.go` - root registration removed in Phase 3; an exported entry kept for le
- `cmd/ze/dispatch.go` - `binarySuffixRoot` deleted in Phase 3
- `cmd/ze/main.go` - the build-tag comment
- `cmd/ze/ze_chaos_run.go`, `ze_chaos_main_test.go`, `ze_analyze_register.go`, `ze_perf_register.go` (D-2) - deleted in Phase 3
- `cmd/ze-perf-run/` - deleted in Phase 3 (D-1: `perf run` replaces it)
- `le` (launcher) - `check_name` refuses `test`, `test-linux-*` and any name whose `bin/le-<name>` is a `bin/le-test*` artifact path, exit 2, naming the artifact (D-3)
- `cmd/ze/root_launcher_test.go` - the launcher refusal test (AC-28)
- `.claude/settings.json` - hook commands (Phase 1)
- `.github/workflows/perf-nightly.yml`, `qemu-nightly.yml`, `evidence-nightly.yml`, `govulncheck.yml`, `codeql.yml` - new names, harness name, `LE_` variables
- `test/interop/Dockerfile.ze`, `test/interop/Dockerfile.stayrtr`, `test/interop-l2tp/Dockerfile.ze`, `test/interop-pppoe/Dockerfile.ze` - harness name and comments
- `.ci` files under `test/` that exec `ze-test` or `ze-chaos` - Phase 2
- `ai/INSTRUCTIONS.md` (canonical for `AGENTS.md`), `ai/INDEX.md`, `ai/rules/*.md`, `ai/rules/points/**`, `ai/skills/*.md` (canonical for `.claude/skills`), `ai/agents/*`, `.claude/rules/*.md` - Phase 2
- `docs/**` that names an old command, program or variable - in the commit that moves the family where the page describes it, else Phase 2

## Files to Create
- `internal/le/leroot/retired.go` - the rename map and the alias handler
- `internal/le/leroot/retired_test.go`
- `internal/le/rename_test.go`
- `internal/le/test/harness/register.go`, `harness.go`, `harness_test.go`
- `internal/le/chaos/run/register.go`, `run_test.go`
- `internal/le/mrt/register.go`, `mrt_test.go`
- `internal/le/perf/` - one area, moved from `internal/le/perfbench` (D-1): `register.go`, the action table with `run`, `suggest`, `record`, `history-record`, `evidence-record`, `send`, `report`, `track`, and `perf_test.go`. No sub-directories: every verb is an action of the one `perf` command
- `internal/le/doc/index/` - moved from `internal/le/docstocode` (D-5), with `index_test.go`
- `internal/le/build/gokrazy/register.go`, `gokrazy.go`, `gokrazy_test.go` (from `cmd/ze-gok`)
- `internal/le/build/installer`, `build/hostdriver` (from `buildartifacts`)
- `internal/le/doc/check/retired.go`, `retired_test.go`
- `internal/test/fixture/ui_fixture_le_subject_first_dispatch.go`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | le commands are LocalData handlers, not YANG RPCs |
| YANG validation constraints | N-A | no YANG |
| YANG custom validators | N-A | no YANG |
| CLI commands/flags | Yes | every package in the rename map; `internal/le/leroot/retired.go` |
| CLI grammar (keyword before value) | Yes | subject-first names; `./le cli grammar` feeder 6 with `leNamespaceExempt` empty |
| Editor autocomplete | N-A | le has no editor completion |
| Functional test for new RPC/API | Yes | `internal/test/fixture/ui_fixture_le_subject_first_dispatch.go`; the edited `.ci` files |
| Pipe completeness | Yes | aliases dispatch through `Run`, so pipes apply; `TestEveryRetiredNameRunsItsNewCommand` checks `| json` |
| Env var registration | Yes | `le.test.bin` and `le.test.no.build` in `internal/test/runner/runner.go`; `le.qemu.test.bin` in `internal/le/qemu/run.go` |
| Doctor check for runtime dependencies | N-A | no new runtime dependency of `ze`; `le-test` is a development artifact |
| Prometheus counters/metrics | N-A | none added; `ze_chaos_*` unchanged |
| BGP family surface (new SAFI / capability / attribute) | N-A | no protocol change |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | developer tooling only |
| 2 | Config syntax changed? | No | no config |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` (the `ze-perf` and `ze-analyze` sections), `ai/INDEX.md` "Dev Tools" |
| 4 | API/RPC added/changed? | No | no RPC |
| 5 | Plugin added/changed? | No | no plugin |
| 6 | Has a user guide page? | Yes | `docs/guide/developer-setup.md`, `docs/guide/chaos-testing.md`, `docs/guide/benchmarking.md`, `docs/guide/mrt-analysis.md`, `docs/guide/appliance.md` |
| 7 | Wire format changed? | No | no wire change |
| 8 | Plugin SDK/protocol changed? | No | no SDK change |
| 9 | RFC behavior implemented, changed, or newly proven? | No | no RFC behavior |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md`, `docs/architecture/testing/runner-architecture.md`, `docs/architecture/testing/ci-format.md`, `docs/architecture/testing/tracked-build-gate.md`, `docs/architecture/testing/qemu-integration.md`, `docs/architecture/config/environment.md` (the `le.` keys) |
| 11 | Affects daemon comparison? | No | no daemon behavior |
| 12 | Internal architecture changed? | Yes | `docs/architecture/system-architecture.md` "Build personalities", `docs/architecture/overview.md`, `docs/architecture.md`, `docs/DESIGN.md`, `docs/architecture/appliance/gokrazy-build-pins.md`, `docs/architecture/chaos-web-dashboard.md` |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | No | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | `ai/INDEX.md` native command inventory; `ai/INSTRUCTIONS.md` Programs table and Source Layout build tags |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED: run `./le spec citation anchors spec plan/spec-le-subject-first-command-tree.md` at the start of each phase. Pages DECLARED by `// Design:` headers of files this spec changes, each updated in the phase that moves the file: `docs/architecture/cli/command-namespacing.md` (le names become subject-first, the test-* exception goes), `docs/architecture/core-design.md` (`verify lock` merges into `job`; le dispatch), `docs/architecture/testing/interop.md` (the harness is `le-test` in the interop image), `docs/architecture/testing/verify-freshness-scope.md` (stage names and log file names) |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | every `./le` example under `docs/` and `ai/`, driven by the AC-15 report |

### Discovery (`ai/rules/repo-maintenance.md` Mechanical Checklist)
| Question | Answer |
|----------|--------|
| Where does an agent look first? | `ai/INDEX.md` "Dev Tools", which states subject-first naming and names `./le '|' json` as the inventory |
| What rule prevents regression? | `ai/INDEX.md` "Add a development tool" step 1 (subject-first; a space is a directory level) and `TestEveryCommandIsFoundAtThePathItsNamePredicts` |
| What registry prevents drift? | the le command registry (the manifest); the rename map in `internal/le/leroot/retired.go` for old names |
| What verification proves it? | `./le doc check retired-commands` as a full-mode verify stage; `./le cli grammar` |

## Implementation Steps

1. **Phase 1a: Wiring** -- the map, the alias handler, the retired-name report
   - Tests: `TestEveryRetiredNameRunsItsNewCommand`, `TestRetiredNamesAreNotInTheManifest`, `TestRenameMapRowsAreDisjoint`, `TestRetiredCommandSweepFindsAnInjectedName`
   - Files: `internal/le/leroot/retired.go`, `internal/le/register.go`, `internal/le/doc/check/retired.go`
   - Verify: an old name with no new registration fails its test by name
2. **Phase 1b: Move one family at a time** -- `repo`, `arch`, `cli`, `config`, `doc`, `web`, `ai`, `rfc`, `spec`, `verify`, `go`, `site`, `data`, `build`, `test`, `chaos`. For each family: move the packages, change the registered names, update the blank imports and the stage table, and edit the pages that describe the family in the same commit
   - Tests: `TestEveryNewNameResolvesToItsArea`, `TestEveryCommandIsFoundAtThePathItsNamePredicts`, the tests of the moved packages
   - Verify: `go test` of the moved packages under `./le job run`; old names still answer
   - The `ai` family also moves `.claude/settings.json` to `le ai hooks` (R-1)
3. **Phase 1c: Programs and variables** -- `test harness` with the `le-test` build and the `ze-test` hard link; `chaos run`; `mrt`; `build gokrazy`; the terminal-demo `pty` action; the three `le.` variables with both spellings read; the three raw reads folded into the registry; the one `le.qemu.test.bin` default; the hook admission of `le-test`. The `perf` area (D-1, D-2): `internal/le/perfbench` moves to `internal/le/perf`, `run` gains the `step` keyword and the `dut` selection of `RunCLI`, `suggest` replaces `suggestion-report`, `send`, `report` and `track` reach `internal/perf/cli`, and the perf runner cross-builds a linux `le` with `CGO_ENABLED=0` and mounts it at `/usr/local/bin/le`. The launcher `le` refuses the harness names (D-3)
   - Tests: the program and variable rows of the Wiring Test table; `TestQemuHarnessVariableHasOneDefault`; `TestBashHookAdmitsLeTest`; `TestPerfRunAcceptsRunnerOptionsAsKeywords`; `TestPerfVerbsAnswerAsTheirPredecessors`; `TestPerfRunnerMountsLinuxLe`; `TestLeLauncherRefusesHarnessNames`
   - Verify: AC-7 to AC-14 and AC-24 to AC-28; measure AC-21 and the two build times of AC-26
4. **Phase 2: Callers** -- run `./le doc check retired-commands report`, rewrite every match to its new form, family by family, in canonical sources only (`ai/INSTRUCTIONS.md`, `ai/skills/`, never `AGENTS.md` or `.claude/skills/`), then `./le ai sync write`. Fix the `ze-analyse` misspelling. Move `.github/workflows/perf-nightly.yml` to `./le perf track --check test/perf/history/ze.ndjson` (D-2, AC-25)
   - Verify: the report is empty outside the declared exceptions
5. **Phase 3: Removal** -- delete the aliases, the old programs (`cmd/ze-gok`, `cmd/ze-terminal-pty`, `cmd/ze-perf-run`), the build tags `ze_chaos`, `ze_analyze` and `ze_perf` with their register files, `binarySuffixRoot`, the `ze-test` hard link, `bin/ze-perf`, `bin/ze-perf-linux`, `ZE_PERF_BIN`, the old variable entries and their fallback reads; rename the build tag `ze_test` to `le_test` (D-6) in `cmd/ze/ze_test_register.go` (renamed `le_test_register.go`), `TestBuildTags`, the le builders, the matrices, the workflows and the Dockerfiles, leaving `zetest` alone; turn the report into the gating `check` verb and add it to the full verify stage list; lower `floor.Roots` by the deleted roots
   - Tests: `TestRetiredCommandSweepFindsAnInjectedName` moves from report to gate; every old name answers `unknown command`; `TestHarnessTagIsLeTest`
   - Verify: AC-16, AC-17, AC-19, AC-23, AC-30

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every map row has a new registration, an alias in Phases 1 and 2, and no trace in Phase 3 |
| Correctness | An alias returns the exit code and payload of the new command, never a code of its own; a variable read never returns the `ZE_` value when an `LE_` value is set |
| Naming | New directories follow `directoryFor`; package names do not shadow the standard library (`ai/sync` is package `aisync`) |
| Data flow | Aliases re-enter `Dispatch`; the harness is exec'd, never imported |
| Rule: no-layering | The alias layer holds no logic beyond word rewriting and variable fallback, and Phase 3 deletes it whole |
| Rule: principles, one declaration | The map is the only list of old names; the gate and the aliases read it |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| every new name registered | `./le '|' json` compared with the map |
| no old name in the tree | `./le doc check retired-commands` exits 0 |
| the harness ships as `le-test` | `test/interop/Dockerfile.ze` and `qemu-nightly.yml` name `le-test`; an interop run passes |
| the old programs are gone | `ls cmd/` shows no `ze-gok` and no `ze-terminal-pty`; `cmd/ze` has no `ze_chaos` or `ze_analyze` file |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | the alias handler passes argv through unchanged after the rewritten words; no shell interpolation |
| Exec path | `test harness` execs a path under `bin/` of the checkout, or `LE_TEST_BIN`; it never resolves the harness from `PATH` |
| Hook safety | a hook that cannot resolve its command blocks the session, so `.claude/settings.json` moves only with its parity test green |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- `le` already links gok, `internal/appliance`, `internal/perf` and `perfrunner`, so most of the cost of "programs into le" is already paid. The measurable delta is chaos, `perf/cli` and `analyze`.
- Three of the standalone programs are target binaries under another name: the harness, ze-perf (the container sender) and ze-terminal-pty (the demo container). The host-versus-target split in `ai/INSTRUCTIONS.md` decides their fate better than their names do.
- AC-26 build times, measured 2026-09-25 on the dev host (AMD EPYC 7351, 32 threads, x86_64, `GOOS=linux CGO_ENABLED=0`): the le linux cross-build (`ze_le` plus every feature gate, 133 MB) takes 45.4 s from an empty GOCACHE and 1.1 s warm; the ze-perf-linux build it replaces (`ze_perf ze_bgp`, 36 MB) took 65.4 s from an empty GOCACHE and 1.9 s warm. Each cold figure is one run, taken one after the other on a host other sessions were loading, so the cold pair carries noise; the claim it supports is that the le cross-build costs no more than the build it replaces. The runner prints the le build time on every run (`perfrunner.Runner.buildLinuxBinary`).

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| The rename map is a Go table in `internal/le/leroot/retired.go`: rows of old words to new words, program names, file names and variable keys | a data file; aliases declared in each package | one declaration read by the alias layer, the report and the gate (`ai/rules/principles.md`); `leroot` already owns name resolution; a per-package alias scatters the list the gate needs |
| An alias rewrites leading words and dispatches again | register each old name with the new handler | re-dispatch handles renames, merges (`verify lock` to `job`) and verb splits (`build-artifacts`) with one mechanism, and cannot drift from the group, actions or shape of the new command |
| Aliases are hidden from `Commands()` and the manifest | list them with a "renamed" description | the help screen then teaches only the new names, and the directory and floor tests see only real commands |
| An alias prints one stderr line | silent aliases | the line is how Phase 2 finds callers that a text search cannot see (Go argv slices, CI logs) |
| `./le test harness` execs `bin/le-test` | import `internal/test/cli` into `internal/le` | the import panics in a `ze_le` build (duplicate root `bgp`, A-3); the harness must exist as a file for containers and VMs anyway |
| The Phase 3 gate is a new `doc check retired-commands` check beside `doc check links` | a new verb of `cli grammar` | `cli grammar` reads registries and never tracked text; `sweepTracked` in `internal/le/doc/check/links.go` already walks every tracked file and is a full-mode stage |
| `build gokrazy` runs gok in-process | keep building and exec'ing `ze-gok` | `internal/appliance/cmd_build.go` already runs gok in-process, and `le` already links it |
| `LE_TEST_BIN` and `LE_QEMU_TEST_BIN` stay two variables | one variable | they carry different binaries in one run (owner decision, 2026-09-24; "Why the two binary-path variables are not merged") |
| In Phases 1 and 2 each harness variable has TWO registered entries: the `le.` entry, and the `ze.` entry with `Deprecated` naming the `le.` key. One resolver per variable reads the `le.` key first and the `ze.` key second. Phase 3 deletes the `ze.` entry and the second read | register the `ze.` key in `Aliases` of the `le.` entry (the owner's stated mechanism) | `env.Get` reads an alias spelling only when the caller passes the alias key (`internal/core/env/env.go`), so an alias on `le.test.bin` does not see an environment that sets only `ZE_TEST_BIN`. `MustRegister` also panics when an alias equals a registered key, so the old key cannot be both. Changing `env.Get` to scan alias spellings would change a core package every product setting uses, to serve a migration that Phase 3 deletes |
| `ze.test.bgp.port` keeps its `ze.` key | rename it with the harness | the daemon reads it (`internal/component/bgp/reactor/reactor_peers.go`), so it is a product setting, and both processes must read one name |
| The new directories `go`, `test`, `build`, `arch`, `cli`, `web`, `data`, `chaos`, `perf` hold no `register.go` of their own | register the bare words as enumerators | same reason as the superseded `spec-le-command-namespaces`: `Dispatch` answers a bare namespace token from the registry |

## Known Limitations

- Verb names inside a command are not renamed, except where D-1 and D-5 rename them.

## Folded in from the superseded spec

`spec-le-command-namespaces` AC-1 to AC-10, AC-12, AC-15, AC-16 and
AC-17 are delivered in the tree: the two-word names resolve, and
`TestDispatchBoundsTheLookupAtTwoWords`, `TestEveryCommandIsFoundAtThePathItsNamePredicts`,
`TestEveryCommandRegistersItsOwnAnswerShape` and
`TestNoMemberShadowsItsNamespaceRootVerb` pin them. This spec keeps those tests.

| Old AC | Fate here |
|--------|-----------|
| AC-11 (bare namespace token lists its members, exit 0) | still open: folded in as AC-31. The superseded spec's closure found that `Dispatch` exits 1 and `TestBareNamespaceTokenListsItsMembers` pins 1. The owner ruled exit 0 on 2026-09-24 |
| AC-13 (le feeder, `test` the only exception) | kept as a feeder, exception list dropped: AC-19 |
| AC-14 (every stage keeps its log file name) | REVERSED: AC-18. `internal/le/verify/stagelogname_test.go` pins the old names and changes with the stage names |
The bare namespace exit code is carried here as AC-31, not in the superseded spec. The owner ruled on 2026-09-24: a bare namespace token exits 0.

## RFC Documentation (Scope: protocol)

N-A: Scope is tooling.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test (N-A when Scope is tooling or docs, which delete that section)
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes. It runs every stage against a COMMIT in a throwaway worktree, which is the pre-commit gate (`ai/rules/git-safety.md`). An in-place `./le verify current` is void the moment the tree moves under it
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied: lint fixed rather than disabled, the focused check for the changed behavior run once with its OUTPUT PASTED, and any red named with the one-line reason it is scaffolding
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/spec-le-subject-first-command-tree.md` only, in the same `./le commit create` script (commit A preserves the spec in history)
