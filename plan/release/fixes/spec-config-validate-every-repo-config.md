# Spec: config-validate-every-repo-config

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | config |
| Priority | P1 |
| Release reason | RFC 8212 correctness: configs the repository ships and drives (perf DUT, demos, interop scenarios, netlab goldens, `.ci` bodies) left EBGP directions closed and exchanged no routes, and nothing caught it. `ze config validate` now warns (e2af3bbac6); every config the repository uses must pass it |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-31 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Thomas asked: "we should have a test running validate on all the configs
generated / used". An RFC 8212 correctness defect decides that this runs (no
owner approval needed): the perf lab DUT config stopped carrying routes when
RFC 8212 enforcement landed, and no test loaded it to notice.

Already landed:

| Commit | What |
|--------|------|
| e2af3bbac6 | `ze config validate` warns `config-bgp-rfc8212-no-policy` for each EBGP direction RFC 8212 closes (exit stays 0) |
| 1611f99e41 | `TestRepositoryConfigsValidate` (`internal/component/config/cli/validate_corpus_test.go`), red at landing with 593 findings |
| d9a3f94bc5 | 13 standalone configs state their policy (593 to 556 findings) |
| 664be57056 | `test/perf/configs/ze.conf` states its import and export policy |

This spec finishes the work: the sweep's over-reports are removed by derived
rules (never a list), every real defect it names is fixed, and the sweep is
green with zero findings. The 556 findings of the last run are in the session
scratch `tmp/session/2026-10-31-204407/scratch/sweep-findings-556.txt`; every
one is accounted for in Acceptance Criteria (the per-bucket counts sum to 556).

### Handoff enumeration (nothing dropped)

The previous agent's handoff (`tmp/session/2026-10-31-204407/scratch/handoff-config-validate-sweep.md`)
plus the brief that commissioned this spec carry 18 items. Each maps to ACs.

| # | Item | Source | AC |
|---|------|--------|----|
| 1 | Interop `@NAME@` placeholders | handoff sweep 1 | AC-8 (4 findings, not 6), plus AC-9 (36 PKI placeholder findings the handoff missed) |
| 2 | Host verdicts: firewall default backend, gokrazy interface commit | handoff sweep 2 | AC-7 (26 findings) |
| 3 | FRR `vtysh.conf` misread because `service` is a Ze root | handoff sweep 3 | AC-10 (3 findings) |
| 4 | Reload `.ci` with a deliberately refused config and exit 0 | handoff sweep 4 | AC-16 (22 findings) |
| 5 | `tmpfsForRecord` `$PORT3` expansion bug, journal row owed | handoff sweep 5 | AC-19, owned journal row |
| 6 | 302 RFC 8212 findings in `.ci` bodies | handoff fixes | AC-3 |
| 7 | netlab generator for `contrib/netlab/golden/r1.conf`, `r3.conf` | handoff fixes | AC-4 |
| 8 | 80 parse errors in `test/exabgp-compat/native/*.conf` | handoff fixes, corrected by the brief | AC-5 |
| 9 | Demo step fragments need validation in sequence | handoff fixes | AC-13 (21 findings) |
| 10 | `interface xfrm` unknown (far-end.conf, 4-xfrm.conf) | handoff fixes | AC-6: corrected, it is a host verdict, not a config defect |
| 11 | `bgp-delayopen-frr` missing prefix maximum | handoff fixes | AC-14 |
| 12 | `bgp-session-protection-frr` unknown `bgp filter` | handoff fixes | AC-14 |
| 13 | Three refused migrate outputs | handoff fixes | AC-12 |
| 14 | "The rest" (plugin-verify, yang-missing, bgp-peer, ...) | handoff fixes | AC-11, AC-15, AC-17, AC-18 (62 findings, each named) |
| 15 | Scoped lint on `./internal/component/config/cli/...` | brief | AC-21 |
| 16 | `./le arch tier check` for the sweep's import of `internal/exabgp/migration` | brief | AC-21 |
| 17 | `TestUnknownRedistributeFamilyRefusesLoad` red, unrelated, journal row | brief | AC-20, owned journal row |
| 18 | Sweep green with zero findings, and the mutation proof against 664be57056 | brief | AC-1, AC-2 |

Corrections to the handoff, verified at the producer:

| Handoff claim | What the code says |
|---------------|--------------------|
| Nothing reads `test/exabgp-compat/native/*.conf`, delete needs consent | False. `TestParseAllConfigFiles` (`internal/component/bgp/config/loader_test.go`) parses them under `curatedNativeExampleFixtures` and `legacyExampleFixtureReason`; `internal/component/config/serialize_test.go` and `internal/component/bgp/reactor/config_direction_test.go` read them too. No file is deleted |
| `interface xfrm` is unknown (a config defect) | `list xfrm` and `list pppoe-client` (and `container mirror`) carry `ze:os "linux"` in `internal/component/iface/yang/ze-iface-conf.yang`; `yangToNode` (`internal/component/config/yang_schema.go`) prunes a node whose `ze:os` differs from `runtime.GOOS`. On macOS the parser then says "unknown field". A host verdict, and a misleading operator message |
| 6 placeholder findings | 4 `@ZE_LINK_LOCAL@`/`@FRR_LINK_LOCAL@` findings, plus 36 `%%PKI_B64:<file>%%` findings in 18 `test/interop-ipsec` scenarios |
| `$PORT3` in 4 `.ci` bodies | 5 files: `test/plugin/reload-listener-rejected.ci`, `test/plugin/bmp-lg-http-routes.ci`, `test/reload/bgp-rpki-added-live.ci`, `test/reload/bgp-rpki-attach-live.ci`, `test/reload/ipsec-added-live.ci`. Runner expansion of `$PORT2`/`$PORT` only also applies to exec strings, stdin and env (`internal/test/runner/runner_exec.go`) |

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md`, `ai/doors/config-yang.md`, `ai/doors/bgp-engine.md`, `ai/doors/le-tooling.md`, `ai/doors/system-plugins.md`, `ai/doors/forwarding.md`, `ai/doors/appliance.md`, `ai/doors/core.md` - the subsystems this spec names
  -> Constraint: `ze config validate` never calls a live `OnConfigVerify`; it runs YANG checks and each `InProcessConfigVerifier` (config door, Traps)
  -> Constraint: a test that mutates a package registry restores it (core door, Traps); the sweep linking test-only plugins must not leave registrations behind
  -> Constraint: never import `internal/le` from product code (le-tooling door); a helper both sides need lives outside `internal/le`
- [ ] `docs/architecture/config/yang-config-design.md` - validation layers, where a validator runs, `ze:os` and `ze:backend`
  -> Decision: `ze:os` prunes at schema-build time because GOOS is fixed for the binary (`yang_schema.go`, comment above `getBackendExtension`)
- [ ] `docs/architecture/testing/ci-format.md` - `.ci` block grammar, where a refusal marker would be declared
- [ ] `docs/architecture/testing/interop.md` - scenario rendering and placeholders
- [ ] `docs/guide/command-reference.md` - `ze config validate` codes the operator sees

### RFC Summaries (Scope: config, RFC-driven)
- [ ] `rfc/short/rfc8212.md` - the default-reject rule the warning reports
  -> Constraint: an EBGP direction with no explicit policy passes no routes (RFC 8212 Section 3)
- [ ] `rfc/full/rfc8955.txt` Section 4.2.2.11 - DSCP component (applies to IPv6 by RFC 8956 Section 3)
  -> Constraint: "The six least significant bits contain the DSCP value.  All other bits SHOULD be treated as 0." Only a SHOULD, so Ze refusing ExaBGP's `traffic-class 101` is stricter than ExaBGP with no MUST behind it

**Key insights:**
- The sweep calls `runValidation`, the exact path `ze config validate` takes, so a finding is what an operator would see.
- Over-reports fall in four classes, each removable by a rule derived from a fact already declared: the host (ze:os, OS default backend), the harness (placeholders, port expansion, test-only plugins), the carrying test (deliberate refusal, migrate input, merge fragments) and the classification (non-Ze files).

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/config/cli/validate_corpus_test.go` (243L) - `TestRepositoryConfigsValidate`: walks the tree (skipping `sweepSkippedDirs`), reads every `.conf` and the `.ci` bodies `collectCorpusCarriers` yields, classifies a body by `firstConfigKeyword` against the schema and `registry.GatedConfigRootRequirements`, migrates non-Ze bodies with `migration.ParseExaBGPConfig`/`MigrateFromExaBGP`/`SerializeTree`, skips a `.ci` matching `refusedExit` (non-zero exit), excuses an RFC 8212 warning when the `.ci` has `expect=` naming 8212 or the scenario directory name holds 8212, ignores codes in `hostVerdictCodes` (only `config-kernel-capability`), copies the runner's port rule in `expandRecordPorts`
- [ ] `internal/component/config/cli/cmd_validate.go` - `runValidation`: YANG, `kernelcap.Refuse` reported as `config-kernel-capability` (documented as a host verdict), MCP, gNMI, `config.VerifyPluginConfig` reported as `config-plugin-verify`, BGP, `addRFC8212Warning`
- [ ] `internal/component/bgp/config/loader_test.go` - `TestParseAllConfigFiles`, `curatedNativeExampleFixtures` (8 files), `legacyExampleFixtureReason` (prefix and name rules)
- [ ] `internal/component/config/yang_schema.go` - `yangToNode` returns nil for a node whose `ze:os` is not `runtime.GOOS`
- [ ] `internal/component/config/parser_freeform.go` - "unknown field in %s: %s" for a name the schema lacks
- [ ] `internal/component/config/backend_gate.go` - `validateBackendFeatures`, "backend feature gate: no backend configured at <path>"
- [ ] `internal/component/iface/register.go`, `internal/component/firewall/engine.go`, `internal/component/traffic/register.go` - "no backend configured and no OS default available"
- [ ] `internal/component/vpp/cpuset.go` - CPU inventory read from `/sys/devices/system/cpu/online`
- [ ] `internal/test/runner/runner_exec.go` - `tmpfsForRecord` and four other sites expand `$PORT2` then `$PORT`, nothing else
- [ ] `internal/le/interoplab/bgp/prepare.go` - `zeLinkLocalPlaceholder`, `frrLinkLocalPlaceholder`, rendered from the lab's IPv6 prefix
- [ ] `internal/le/interoplab/ipsec/helpers.go` - `pkiPlaceholder` (`%%PKI_B64:<file>%%`)
- [ ] `internal/le/test/netlab/netlab.go` - `Checker.validateGolden` runs `ze config validate` on each golden and reads only the exit code, so a warning passes
- [ ] `contrib/netlab/ze/bgp.j2` - the template that renders the BGP block of each golden
- [ ] `internal/component/bgp/config/loader_create_test.go` - `TestUnknownRedistributeFamilyRefusesLoad`, red: the redistribute filter validator refuses unknown filter "V4" before `ExtractRedistributeRules` reaches its "no final" path
- [ ] `demos/terminal/topics/ipsec/configure.tape` - steps `load file absolute merge` each numbered fragment in order

**Behavior to preserve:**
- `TestParseAllConfigFiles`, `serialize_test.go` and `config_direction_test.go` behave as today over `test/exabgp-compat/native/`
- `ze config validate` exit codes; `config-bgp-rfc8212-no-policy` stays a warning
- The sweep's fatal guards: zero Ze configs or zero migrated configs fails the run (R-6)
- Every `.ci` keeps proving what it proved; an expectation it loses goes through `ze-test-judge`

**Behavior to change:**
- The sweep reports zero findings; its exclusions are each derived from a declared fact
- An operator validating a config on a host whose OS prunes a node is told the node exists only on the named OS, not "unknown field"
- A verifier verdict about the host (OS default backend, host inventory read) carries a code that says so
- The runner refuses or reserves `$PORT3` instead of silently producing `<port>3`
- The netlab template, demo fragments, interop scenarios, `.ci` fixtures and ExaBGP migration listed in the ACs are fixed

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Offline validation path | `internal/component/config/cli/cmd_validate.go` `runValidation` | The whole `ze config validate` verdict | Extends: the sweep keeps calling it, and two host-verdict codes are added beside `config-kernel-capability` |
| Host verdict code | `cmd_validate.go` `kernelcap.Refuse`, `internal/core/diagnostic/codes.go` `config-kernel-capability` | A verdict documented as about THIS host | Extends: the same shape for an OS-pruned node and for an OS default backend or host inventory; the sweep's `hostVerdictCodes` keeps reading codes, never message text |
| Diagnostic code registry | `internal/core/diagnostic/codes.go` | One entry per code with title, description, examples | Extends: new codes register here, so `ze show diagnostic <code>` explains them |
| `ze:os` pruning | `internal/component/config/yang_schema.go` `yangToNode`, `getOSExtension` | Drops the node from the schema on another OS | Extends: the schema remembers the pruned name and its OS so the parser can say why the name is refused |
| Backend feature gate | `internal/component/config/backend_gate.go` `validateBackendFeatures` | Refuses an empty backend and unsupported nodes | Extends: the empty-backend refusal, and the components' "no OS default" errors, are marked as host verdicts |
| Corpus carrier reader | `collectCorpusCarriers` (shared with `TestFormatCorpusRoundTrip`) | Yields each `.ci` tmpfs `.conf` and stdin=config body | Extends: carries the block's refusal marker and the test's load sequence |
| Runner port expansion | `internal/test/runner/runner_exec.go` `tmpfsForRecord` and four sibling sites | Expands `$PORT2` then `$PORT` | Replaces the sweep's copy (`expandRecordPorts`) with one exported expansion both use, which also answers for `$PORT3` |
| Native fixture classification | `internal/component/bgp/config/loader_test.go` `curatedNativeExampleFixtures`, `legacyExampleFixtureReason` | Curated fixtures must parse; legacy ones are named with a reason | Extends: moves to one declaration both `TestParseAllConfigFiles` and the sweep read; no second list |
| Interop placeholder rendering | `internal/le/interoplab/bgp/prepare.go` link-local placeholders; `internal/le/interoplab/ipsec/helpers.go` `pkiPlaceholder` | Renders scenario files before a lab run | Extends: the placeholder grammar and the renderer move to a leaf outside `internal/le` that both interoplab and the sweep call |
| Test-only plugin composition | `internal/test/plugins/all` | The plugin set the functional `ze` build links under `zetest` | Extends: the sweep links the same composition, so `fakeredist` and `fakedrain` resolve as they do in the `.ci` run |
| Gated root table | `registry.GatedConfigRootRequirements` | Names the tags a root needs | Does not change: the sweep keeps reporting a root its build lacks |
| `.ci` refused-exit rule | `validate_corpus_test.go` `refusedExit` | Skips a `.ci` whose command expects a non-zero exit | Extends: a block the test declares refused is asserted refused, not skipped |
| netlab golden check | `internal/le/test/netlab/netlab.go` `Checker.validateGolden` | Validates goldens by exit code | Does not change: the template fix makes goldens clean; the sweep catches the warning the exit code does not |
| ExaBGP migration | `internal/exabgp/migration` `ParseExaBGPConfig`, `MigrateFromExaBGP` | Turns an ExaBGP config into Ze config | Extends: `family { all; }` and an ExaBGP-accepted DSCP value migrate to a valid Ze config |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `go test -run TestRepositoryConfigsValidate` in `internal/component/config/cli` under the full tag set, and the `ze config validate` command for the operator-visible codes
- Input: repository files (`.conf`, `.ci` bodies, ExaBGP configs)

### Transformation Path
1. Walk the tree, collect carriers with what each carrying test declares (refused, RFC 8212 subject, migrate input, merge sequence)
2. Render harness placeholders through the shared renderer, expand ports through the runner's own function
3. Classify: Ze body, ExaBGP body (migrate), non-Ze body, native legacy fixture (from the shared classification)
4. Validate through `runValidation`; for a merge sequence, validate the cumulative config after each step
5. Judge each diagnostic: error, unexcused RFC 8212 warning, host verdict (counted and logged), declared refusal that did not refuse (finding)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Test to product validation | `runValidation` in-package call | No |
| Sweep to interop harness | Shared leaf renderer outside `internal/le` | No |
| Sweep to functional runner | Exported port expansion in `internal/test/runner` | No |

### Integration Points
- `runValidation` - unchanged entry; new host codes added inside
- `collectCorpusCarriers` - extended with declared facts
- `internal/test/plugins/all` - blank-imported by the sweep test

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | N-A, test tooling |
| Registration over hardcoding, outbound | No | |
| Registration over hardcoding, inbound: the sweep holds no list of files, directories or scenarios; every exclusion derives from a declaration (ze:os, verifier host code, renderer grammar, carrying test, shared classification) | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Every one of the 26 host-backend findings validates on Linux | the messages name the missing OS default or `/sys` | A real defect hides behind the host code | Run the sweep on the Linux host (scoped package test) | unvalidated |
| A-2 | The 11 ze:os findings validate on Linux | `ze:os "linux"` on each pruned node | Same | Same run | unvalidated |
| A-3 | The 22 AC-16 bodies are refused on purpose | test names and their expectations | Marking hides a defect | Read each `.ci` expectation; the sweep asserts the refusal | unvalidated |
| A-4 | Adding explicit policy to a `.ci` body does not change what that test proves | most are parse and UI tests | An expectation changes | `ze-test-judge` on each changed expectation | unvalidated |
| A-5 | A Ze config never opens with `service <WORD>` | the `service` root is a block | AC-10's shape rule misclassifies | Grep the Ze corpus | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Host codes become an escape hatch for real defects | a finding moves to a host code on Linux | AC-1 requires the Linux run to show no host-verdict skip outside `config-kernel-capability` |
| R-2 | netlab is not installed on the Mac, so goldens cannot be regenerated | `Checker.findNetlab` fails | Regenerate on a host with netlab; the template and goldens land in one commit |
| R-3 | Linking test-only plugins changes the schema the sweep sees | new findings | They are the functional `ze`'s schema, which is what the `.ci` run uses |
| R-4 | 126 `.ci` files edited by hand drift from their tests | functional reds | Run each touched suite (named under Goal Gates) |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Test fixtures and demos; the operator-visible change is the two new `ze config validate` codes |
| How is it reverted? | Per-commit revert |
| Who else touches this path? | `spec-bgp-link-local-only-next-hop` (interop link-local scenarios), RFC 8212 policy work in `bgp/reactor` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ze config validate` on a config with an OS-pruned node | → | `yangToNode` pruned-name record, parser refusal | `TestValidateReportsNodeOfAnotherOS` (`internal/component/config/cli`) |
| `ze config validate` with no backend and no OS default | → | verifier host-verdict marking in `runValidation` | `TestValidateReportsHostBackendVerdict` (`internal/component/config/cli`) |
| Repository files | → | `TestRepositoryConfigsValidate` | itself, plus `TestSweepDerivesEachExclusion` (one case per exclusion rule) |
| `.ci` with `$PORT3` | → | runner port expansion | `TestRecordPortExpansionRefusesUnreservedPort` (`internal/test/runner`) |

## Acceptance Criteria

Finding counts are from `sweep-findings-556.txt`; they sum to 556.

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `TestRepositoryConfigsValidate` under the full tag set (`./le test unit config` tags), on macOS and on the Linux host | Zero findings on both. The log states how many configs were validated, migrated, declared refused, skipped as legacy native fixtures, and skipped per host-verdict code; on Linux the only host-verdict code that may appear is `config-kernel-capability` |
| AC-2 | Revert 664be57056's change to `test/perf/configs/ze.conf` | The sweep goes red naming `test/perf/configs/ze.conf` and `config-bgp-rfc8212-no-policy`; restoring it turns green. Both runs recorded in the spec |
| AC-3 | 302 RFC 8212 findings in 126 `.ci` files (`test/parse` 55, `test/ui` 40, `test/plugin` 23, `test/reload` 6, `test/firewall` 2) | Each body states its import and export policy explicitly, or the test is about RFC 8212 and carries an `expect=` naming 8212. Any expectation the edit changes is judged by `ze-test-judge` |
| AC-4 | `contrib/netlab/golden/r1.conf`, `r3.conf` (4 findings) | `contrib/netlab/ze/bgp.j2` renders explicit policy for EBGP peers; the goldens are regenerated from it and the netlab checker agrees |
| AC-5 | 80 findings in legacy `test/exabgp-compat/native/*.conf` (41 `api-*`, 35 `conf-*`, `example-healthcheck`, `extended-nexthop`, `parse-multisession`, `unknown-message`) | The sweep's treatment derives from the one classification `TestParseAllConfigFiles` uses, moved out of `loader_test.go` into a declaration both read: curated fixtures are validated, legacy ones are logged with their reason. No file is deleted; `TestParseAllConfigFiles`, `serialize_test.go` and `config_direction_test.go` behave as before |
| AC-6 | 11 findings on `ze:os "linux"` nodes (`xfrm`, `pppoe-client`, `mirror`): `demos/terminal/showcase-lab/far-end.conf`, `demos/terminal/topics/ipsec/4-xfrm.conf`, 2 `test/interop-ipsec` and 3 `test/interop-pppoe` scenarios, `test/parse/iface-netlink-accepts-pppoe-client.ci`, `test/parse/iface-vpp-accepts-mirror.ci`, `test/plugin/show-mtu-ike-probe.ci`, `test/plugin/show-mtu-oversized-tunnels.ci` | `ze config validate` on another OS reports a host-verdict code naming the node and the OS that carries it, with the fix (validate on that OS); the sweep counts it as a host verdict; on Linux all 11 validate |
| AC-7 | 26 host-backend findings: interface "backend feature gate: no backend configured" (gokrazy `ze.conf`, `test/install/bootstrap-config-valid.ci`, 2 demo vrrp topics, `test/parse/dhcp-set-format{,-multi}.ci`, 13 `test/reload` `.ci`), firewall "no OS default" (`test/parse/config-diff-list-order.ci` x2, `test/ui/doctor-firewall-nftables.ci`), traffic-control "no OS default" (`test/parse/traffic-empty-backend.ci`, `test/traffic/traffic-reload-{apply,qdisc-tc}.ci`), vpp CPU inventory (`test/parse/vpp-config-full.ci`) | Each verdict carries a host-verdict code the verifier sets, not a message match; the sweep counts it; on Linux all 26 validate |
| AC-8 | 4 link-local placeholder findings (`bfd-link-local-frr`, `bfd-link-local-two-links-frr`, `bgp-rfc2545-linklocal-nexthop-frr`, `bgp-vpn-frr`) | The sweep renders the placeholders through the renderer interoplab uses, from its single declaration, and the rendered configs validate |
| AC-9 | 36 PKI placeholder findings (18 `test/interop-ipsec` scenarios, `bgp-rpki` and `ike` each) | Same: `%%PKI_B64:<file>%%` is rendered from the scenario's own file by the shared renderer, and the configs validate |
| AC-10 | 3 FRR `vtysh.conf` findings (`test/interop`, `test/interop-ipsec`, `test/interop-l2tp`) | The Ze-or-not classification reads the first statement's shape, not only its keyword, so `service integrated-vtysh-config` is not a Ze body; no Ze config changes classification (A-5) |
| AC-11 | 7 findings naming test-only plugins (`test/plugin/drain-before-stop-internal.ci`, `redistribute-chain-narrow-reload.ci` x3, `redistribute-late-join-configadd.ci` x3) | The sweep links `internal/test/plugins/all`, the functional build's composition; the bodies validate |
| AC-12 | 3 refused migrate outputs: `test/exabgp-compat/old-4.2/etc/conf-flow.conf` and `old-5.0/etc/conf-flow.conf` (`traffic-class 101` exceeds 63), `internal/exabgp/bridge/testdata/exabgp-old/neighbor.conf` (`family { all; }`) | `ze exabgp migrate` output validates for all three. `all` migrates to the families ExaBGP means by it. For the DSCP value, ExaBGP's behaviour wins (RFC 8955 Section 4.2.2.11 states only a SHOULD), read from the ExaBGP source and quoted in Design Insights; the ExaBGP fixtures are not edited |
| AC-13 | 21 findings in demo topic fragments (`demos/terminal/topics/` bfd 3, ipsec 3, irr 7, ospf 3, rpki 2, vrrp 3) | Each topic's fragments are validated cumulatively in the order its tape loads them, onto the tape's base config; a step the tape shows refused is declared so in the tape and asserted refused; every other step validates (fragments fixed where they do not) |
| AC-14 | `test/interop/scenarios/bgp-delayopen-frr/ze.conf` (prefix maximum missing), `bgp-session-protection-frr/ze.conf` (`bgp filter` unknown, line 25) | Both configs fixed to today's grammar and validate; each scenario's assertion still proves its subject |
| AC-15 | 11 findings in `.ci` merge fragments: `test/reload/bgp-bfd-added-live.ci`, `bgp-bfd-profile-edit-live.ci`, `bgp-filter-irr-loaded-over-ssh.ci` (2 fragments), `bgp-rpki-attach-live.ci` | Each fragment is validated merged onto the config the test loaded before it, in the test's order; the merged configs validate |
| AC-16 | 22 findings in `.ci` bodies refused on purpose with exit 0: `test/plugin/bgp-update-delay-validation.ci`, `config-validate-agrees-with-boot.ci` (`shapes/unknown_internal.conf`), `session-editor-load-refused.ci`; `test/reload/bgp-bfd-profile-reload-deleted.ci`, `commit-verify-reject.ci`, `config-reload-invalid-validator.ci`, `mgmt-guard-reload-refuses-nonloopback.ci`, `pki-reference-reload-broken.ci`, `reload-bad-config.ci`, `static-bfd-profile-reload-deleted.ci` (2), `static-bfd-profile-reload-passive.ci`, `tx-iface-wireguard-invalid-bad-public-key.ci`, `tx-iface-wireguard-invalid-no-private-key.ci` (2), five `tx-ipsec-*-requires-*.ci`; `test/ui/doctor-gnmi-mcp-exposure.ci` (2) | The carrying test declares the refusal on the block, in the `.ci` format documented in `ci-format.md`; the sweep asserts each declared body IS refused, and a declared body that validates is a finding. A body found not deliberate is fixed instead |
| AC-17 | 23 findings in `.ci` fixtures: `test/parse/config-diff-list-order.ci` peers x3, `config-fmt-comments.ci`, `config-fmt-operator-order.ci`, `config-fmt-value-meaning.ci` x2; `test/ui/cli-completion-full-config.ci`, `cli-completion-value-reference.ci` x5, `cli-config-dump-json-sensitive.ci` x2, `cli-config-dump-sensitive.ci` x2, `cli-config-dump-strip-private.ci` x2, `doctor-config-claims.ci` x3; `test/plugin/lg-graph-lab/ze-bgp.conf` (`environment tcp`) | Each fixture validates, its test still asserting what it did (judged where an expectation changes), or, where the invalid config is the test's subject, it is declared refused as in AC-16 |
| AC-18 | `test/parse/policy-root-migrate.ci` `old.conf` (1) | A body the carrying test feeds to `ze config migrate` is validated as that command's output, derived from the test's own command line |
| AC-19 | `$PORT3` in 5 `.ci` files | The runner refuses an unreserved `$PORTn` with a message naming the file and the fix, or reserves it; the 5 tests use only reserved ports and pass; the sweep calls the runner's exported expansion and holds no copy |
| AC-20 | `TestUnknownRedistributeFamilyRefusesLoad` | Green, its config reaching the "no final" path (a defined filter with no final), so it proves the error path it names |
| AC-21 | Gates the previous agent left owed | Scoped `./le go lint` over `./internal/component/config/cli/...` (and every package this spec changes) is clean; `./le arch tier check` accepts the sweep's imports (`internal/exabgp/migration`, `internal/test/plugins/all`, the new leaf packages) |
| AC-22 | A sweep exclusion rule disabled one at a time | `TestSweepDerivesEachExclusion` shows each rule (host code, placeholder render, port expansion, test-only plugins, declared refusal, merge sequence, migrate input, legacy native classification, non-Ze shape) is what turns its findings away; with it removed they return |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRepositoryConfigsValidate` | `internal/component/config/cli/validate_corpus_test.go` | AC-1..AC-18 | |
| `TestSweepDerivesEachExclusion` | `internal/component/config/cli/validate_corpus_test.go` | AC-22 | |
| `TestValidateReportsNodeOfAnotherOS` | `internal/component/config/cli/cmd_validate_host_test.go` | AC-6 code, message and fix text, with an injected OS | |
| `TestValidateReportsHostBackendVerdict` | `internal/component/config/cli/cmd_validate_host_test.go` | AC-7 code from the verifier, not the message | |
| `TestRecordPortExpansionRefusesUnreservedPort` | `internal/test/runner/runner_exec_test.go` | AC-19 | |
| `TestParseAllConfigFiles` | `internal/component/bgp/config/loader_test.go` | AC-5 unchanged behaviour over the shared classification | |
| `TestUnknownRedistributeFamilyRefusesLoad` | `internal/component/bgp/config/loader_create_test.go` | AC-20 | |
| migration tests for `family all` and DSCP | `internal/exabgp/migration/*_test.go` | AC-12 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| migrated `traffic-class` | decided by AC-12 from ExaBGP's behaviour | per decision | N/A | per decision |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| every `.ci` AC-3, AC-15..AC-19 touches | `test/parse`, `test/ui`, `test/plugin`, `test/reload`, `test/firewall`, `test/traffic`, `test/install` | Unchanged scenarios still pass | |

## Files to Modify
- `internal/component/config/cli/validate_corpus_test.go` - derived exclusions, cumulative validation, declared refusal assertion
- `internal/component/config/cli/cmd_validate.go` - map host verdicts to their codes
- `internal/component/config/yang_schema.go`, `internal/component/config/parser_freeform.go` - remember an OS-pruned name and refuse it with its OS
- `internal/component/config/backend_gate.go`, `internal/component/iface/register.go`, `internal/component/firewall/engine.go`, `internal/component/traffic/register.go`, `internal/component/vpp/cpuset.go` - mark host verdicts
- `internal/core/diagnostic/codes.go` - register the new codes
- `internal/component/bgp/config/loader_test.go` - read the shared native classification
- `internal/component/bgp/config/loader_create_test.go` - AC-20 fixture
- `internal/test/runner/runner_exec.go` - one exported expansion, `$PORT3`
- `internal/le/interoplab/bgp/prepare.go`, `internal/le/interoplab/ipsec/helpers.go` - call the shared renderer
- `internal/exabgp/migration` - `family all`, DSCP value
- `contrib/netlab/ze/bgp.j2`, `contrib/netlab/golden/r1.conf`, `contrib/netlab/golden/r3.conf`
- demo tapes and fragments under `demos/terminal/topics/` and `demos/terminal/showcase-lab/` as AC-13 finds
- `test/interop/scenarios/bgp-delayopen-frr/ze.conf`, `test/interop/scenarios/bgp-session-protection-frr/ze.conf`
- the `.ci` files named in AC-3, AC-15..AC-19
- `docs/architecture/testing/ci-format.md`, `docs/guide/command-reference.md`, `docs/architecture/config/yang-config-design.md`, `docs/functional-tests.md`

## Files to Create
- `internal/test/labconfig/` - placeholder grammar and renderer shared by interoplab and the sweep (name settled at implementation)
- `internal/test/exabgpnative/` - the native fixture classification, moved from `loader_test.go`
- `internal/component/config/cli/cmd_validate_host_test.go`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | no new leaf |
| YANG validation constraints | No | none added |
| YANG custom validators | No | none added |
| CLI commands/flags | No | none added |
| CLI grammar (keyword before value) | N-A | no command |
| Editor autocomplete | N-A | no leaf |
| Functional test for new RPC/API | N-A | no RPC |
| Pipe completeness | N-A | no command |
| Env var registration | No | none |
| Doctor check for runtime dependencies | No | no runtime dependency; new codes register in `codes.go` |
| Prometheus counters/metrics | No | none |
| BGP family surface | No | none |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md`: the two host-verdict codes of `ze config validate` |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | No | |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented, changed, or newly proven? | No | |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md`, `docs/architecture/testing/ci-format.md` (refusal marker, port placeholders) |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | Yes | `docs/architecture/config/yang-config-design.md` (ze:os refusal) |
| 13 | Route metadata keys? | No | |
| 14 | Prometheus counters? | No | |
| 15 | Registered plugin, event, command changed? | No | |
| 16 | Changed source referenced by doc anchors? | Yes | `docs/architecture/config/syntax.md` (declared by the config parser: update if the ze:os refusal changes what it says about unknown fields), `docs/architecture/core-design.md` (declared by changed files: section 14a gains the host-verdict code), `docs/features/ai-first.md` (declared by changed files: unaffected unless it describes `ze config validate` codes; check and say). Rerun `./le spec citation anchors spec plan/release/fixes/spec-config-validate-every-repo-config.md` at implementation |
| 17 | Docs show config examples for this area? | Yes | `contrib/netlab/README.md` if it shows a golden |

## Implementation Steps

1. **Phase: Wiring** - host-verdict codes in `runValidation` and `codes.go`, the two wiring unit tests failing first
2. **Phase: Sweep derivations** - shared classification, shared renderer, runner expansion, test-only plugins, statement-shape classification, declared refusal, merge sequence, migrate input; `TestSweepDerivesEachExclusion`
3. **Phase: Product fixes** - ExaBGP migration (AC-12), redistribute test fixture (AC-20)
4. **Phase: Config fixes** - netlab template and goldens, interop scenarios, demo fragments and tapes, `.ci` bodies (AC-3, AC-13..AC-18), each suite run as it is touched
5. **Phase: Proof** - AC-1 on macOS and the Linux host, AC-2 mutation walk, AC-21 gates

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line, and the bucket counts still sum to the finding total |
| Correctness | No exclusion matches message text; each reads a code, a declaration or a grammar |
| Data flow | The sweep holds no path, directory or scenario list |
| Rule: no-layering | `expandRecordPorts` and the native classification map are deleted, not kept beside the shared ones |
| Rule: testing | Every changed `.ci` expectation carries a `ze-test-judge` verdict |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Sweep green | `./le job run label sweep command go test -tags <full tags> -run 'TestRepositoryConfigsValidate$' -v ./internal/component/config/cli/` on macOS and Linux |
| Mutation proof | revert and restore of `test/perf/configs/ze.conf`, both outputs pasted |
| No copy of the runner rule | grep for `expandRecordPorts` finds nothing |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | The host-verdict codes cannot be reached by a config error on the host it describes (a Linux-only node on Linux is never a host verdict) |

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

- The netlab checker validates goldens by exit code, and RFC 8212 is a warning with exit 0, so the checker could never have caught it; the sweep is the check that does.

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `silent-fall-through.md` | 2026-10-31 | runner expands only `$PORT2`/`$PORT`, `$PORT3` becomes `<port>3` |
| `earlier-guard-hides-the-better-error.md` | 2026-10-31 | `TestUnknownRedistributeFamilyRefusesLoad` stopped at unknown filter before "no final" |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Host verdicts carry a code set by the producer | Match message text in the sweep; run the sweep on Linux only | A hook or test judges what a thing is, never its wording; the operator on a Mac also deserves the honest code |
| The carrying test declares a deliberate refusal, and the sweep asserts it | A list of refused files; infer from test names | One declaration beside the test; an assertion keeps the marker honest |
| Native fixtures keep their existing classification, shared | Delete the legacy files; a second list in the sweep | Readers exist; one declaration (principles) |
| Placeholder renderer moves to a leaf outside `internal/le` | Import interoplab from the config test | The le-tooling door forbids product-side imports of `internal/le` and the tier gate judges direction |

## Known Limitations
- The legacy native fixtures stay "awaiting native conversion" as `legacyExampleFixtureReason` already records; converting them is outside Thomas's request, which is that every config the repository uses passes validation, and the sweep keeps them visible by logging each with its reason.

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

### Goal Gates (MUST pass)
- [ ] AC-1..AC-22 all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests pass: the scoped package tests of every changed package, and the functional suites each touched `.ci` belongs to (`./le test parse`, `ui`, `plugin`, `reload`, `firewall`, `traffic`, `install`), with `./le repo changed scope print both` naming the rest. The full verification is Thomas's release check
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (N-A: no wire change; the touched interop scenario configs are named for the main thread to run)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/release/fixes/spec-config-validate-every-repo-config.md` only, in the same `./le commit create` script
</content>
</invoke>
