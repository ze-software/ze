# Spec: netlab-integration

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Depends | - |
| Phase | 5/5 |
| Deferral shard | `-` (create `plan/deferrals/netlab-integration.md` on the first deferral) |
| Handoff | - |
| Updated | 2026-08-14 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**Ze cannot be used as a netlab device today, and the blocking reason is the container
image, not the daemon.**

netlab (`netlab.tools`) builds labs from a YAML topology and runs each node under
containerlab. It has two integration tiers. A *device* needs Ansible task lists per
module and a Vagrant box. A *daemon* (`netsim/daemons/<name>.yml`) is containerlab-only
and is one YAML file plus one directory of Jinja2 templates. BIRD, dnsmasq, VPP and
netscaler all use the daemon tier, and ze fits it.

Goal: produce the ze-side artifacts a netlab daemon integration needs, and mirror the
netlab-side artifacts in this repository so the templates are versioned beside the
config syntax they emit.

### What netlab requires of a daemon, and where ze stands

| netlab requirement | Ze today |
|---|---|
| Foreground start, config path on the command line | Met. Every interop lab already runs this shape |
| Config delivered as files, daemon reloads after the push | Met. SIGHUP re-reads the config file |
| netlab assigns interface addresses with Linux commands; the daemon uses them | Met. No interop scenario config carries an interface block |
| Routes reach the kernel FIB, because validation is `ping` | Met, behind an explicit `fib-kernel` plugin block and `NET_ADMIN` |
| A show command returning valid JSON for `netlab validate` | Met. `ze cli -c "show ... \| json compact"` over SSH exec |
| A container image netlab can build and run | **Not met.** See Current Behavior |

### Scope boundary

Included:
- A lab-grade container image recipe whose binary carries the default feature set, on a
  base that provides the shell utilities containerlab and netlab exec inside a node.
- `contrib/netlab/`: the daemon YAML, the `Dockerfile.j2`, and the Jinja2 config
  templates, mirrored here and rendered by a test in this repository.
- A documented minimal lab config profile, and the config-push contract (file plus
  SIGHUP).

Excluded:
- The upstream pull request to `ipspace/netlab`. The mirror is the source it copies from.
- LLDP. Nothing in ze sends or receives an LLDP frame. containerlab does not need it
  (links are veths named by the topology), so this costs only the validation tests that
  read neighbor tables. Recorded under Known Limitations.
- Publishing an image to a registry. netlab daemons set `clab.build: True` and build
  locally from `Dockerfile.j2`; `plan/spec-release-distribution.md` excludes containers
  from its scope boundary and this spec does not widen it.

## Required Reading

### Architecture Docs
- [ ] `docs/guide/docker.md` - the current published container story
  → Constraint: the page documents a scratch image with "no shell, no libc, no package
    manager" as a feature. A lab image contradicts that sentence, so the page must gain
    a second image rather than silently changing what the first one is.
- [ ] `ai/rules/repo-maintenance.md` - a new build target and a new tree need discovery
  → Constraint: `contrib/netlab/` must be reachable from `ai/INDEX.md` and must carry a
    check that proves the mirror still renders, or it drifts the first time ze config
    syntax changes.
- [ ] `ai/rules/completion.md` - the goal is a lab that runs, not files that exist
  → Constraint: rendering the templates is not evidence. The evidence is a lab that
    establishes a session and passes a `ping`.
- [ ] `plan/spec-fixit-ipsec-interop-cli-credentials.md` - the other lab-credentials spec
  → Decision: it owns scenario 10 of the IPsec interop lab. Research below shows the
    netlab path does not depend on it: users come from the running config, which a
    template renders. The two specs share a diagnosis, not a fix.

**Key insights:**
- The daemon tier is the whole integration surface: one YAML, one Dockerfile, a handful
  of `.j2` files, a test topology.
- `docker/Dockerfile` is not usable as that Dockerfile, for two independent reasons.
- Credentials are a config-rendering concern, not a ze code change.

## Current Behavior (MANDATORY)

**Source files read on 2026-08-14:**

- [ ] `docker/Dockerfile` - `golang:1.26-alpine` builder to `FROM scratch`, single
  `COPY` of the binary, `ENTRYPOINT ["/ze"]`, `EXPOSE 179 1790 8080`. The build line is
  `${ZE_TAGS:+-tags ${ZE_TAGS}}`, so an unset `ZE_TAGS` produces an **untagged** binary.
- [ ] `Makefile` `ze-docker` target - passes `--build-arg ZE_TAGS` only when `ZE_TAGS` is
  already set, and never derives it. `ZE_FEATURES` exists in the Makefile and is not
  threaded here.
- [ ] `feature-gates.txt` - the single source of truth for compile-out-able features.
  Every consumer derives from it.
- [ ] `test/interop/Dockerfile.ze` - the recipe that solves both problems. It derives
  `ZE_FEATURES` from `feature-gates.txt` with `awk`, builds `-tags "ze_core $ZE_FEATURES"`,
  and lands on `alpine:3.21` with `tini` and `python3`. Its comment states why: a
  `-tags ze_core` image rejects the `bgp` config root as an unknown top-level keyword.
- [ ] `cmd/ze/hub/main_reload.go` - `handleSIGHUPReload` stages a candidate by reading
  the config path from disk, then runs `doReload`. This is the config-push contract.
- [ ] `internal/plugins/fib/kernel/backend_linux.go` - `addRoute` programs the Linux FIB
  with protocol `rtprotZE`. Linux only; `backend_other.go` refuses.
- [ ] `internal/component/ssh/ssh.go` - `execMiddleware` returns before the TUI when the
  session carries a command, so `ssh user@host "show ..."` is non-interactive.
- [ ] `cmd/ze/hub/ssh_infra.go` - `UsersFunc` is documented as the running-config
  credential source; `cmd/ze/hub/main_servers.go` names `liveConfigUsers` and the
  `system` root it reads.
- [ ] `internal/component/ssh/yang/ze-ssh-conf.yang` - `system { authentication { user
  <name> } }` carries `plaintext-password`, an ephemeral leaf bcrypt-hashed into
  `password` on commit and then removed from the tree.
- [ ] `internal/component/authz/auth.go` - `LocalAuthenticator.Authenticate` and
  `CheckPassword`. Plaintext is always tried; the stored hash is accepted as a token
  only when the transport is trusted-local.
- [ ] `test/interop/scenarios/59-rfc7999-blackhole-frr/ze.conf` - a working lab config:
  `plugin { internal ... }` blocks for `bgp-rib`, `rib`, `fib-kernel` and `connected`, a
  `connected {}` block, `bgp { peer ... }`, and no interface block.

- [ ] `internal/component/config/password_hash.go` - `ApplyPasswordHashing(tree, schema)`
  walks the schema, and for every `LeafNode` with `Bcrypt` set it hashes the
  `plaintext-<name>` sibling into the canonical leaf and removes the plaintext.
  → Decision: the shared transform ALREADY EXISTS in the config component. Nothing has
    to be written or hoisted; it has one non-test caller.
- [ ] `internal/component/cli/editor_commit.go` - that one caller. It runs
  `config.RejectMaskedBcryptLeaves` then `config.ApplyPasswordHashing`, then drops the
  matching session entries and serializes.
- [ ] `internal/component/config/loader.go` - `LoadConfig`, the boot and SIGHUP path.
  It parses, then calls `refuseInvalidCustomSections`, then extracts plugins. It never
  calls `ApplyPasswordHashing`.
  → Constraint: the comment above `refuseInvalidCustomSections` records the SAME defect
    class, resolved before: `ValidateTreeAllModules` had one non-test caller, so every
    custom validator was bypassed at daemon start and at SIGHUP reload
    (`spec-fixit-config-validators-bypassed-at-startup`). The password transform is the
    next instance of it, and the fix goes in the same place.

**Behavior to preserve:**
- `docker/Dockerfile` stays a scratch image. Deployments depend on the static artifact
  and `docs/guide/docker.md` publishes it. Only its tag derivation changes.
- `readCredentials` and `CheckPassword` fail closed. Neither is weakened to make a lab
  pass.
- `feature-gates.txt` stays the single derivation point. A second hand-written tag list
  is the failure this spec must not introduce.
- The editor commit path keeps hashing and dropping the ephemeral leaf, and keeps
  writing a config file that never carries plaintext. Its behavior does not change.

**Behavior to change:**
- A config file loaded at boot or at SIGHUP honours `plaintext-<name>` siblings of
  `ze:bcrypt` leaves. Today it accepts them and leaves the canonical leaf empty, so
  every login for that user is refused with no diagnostic
  (`plan/journal/silent-fall-through.md`, 2026-08-14).
- `make ze-docker` builds with the default feature set rather than an untagged binary.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A netlab topology naming `device: ze` on a node, and `netlab up`.

### Transformation Path
1. netlab renders `contrib/netlab/ze/*.j2` against its own data model, producing ze
   config text.
2. containerlab starts the image with the rendered config mounted, and assigns interface
   addresses with Linux commands inside the node's namespace.
3. `ze start <config>` parses it (`zeconfig.LoadConfig`), and the `fib-kernel` plugin
   programs learned routes into the container's kernel FIB.
4. `netlab validate` runs `netlab_show_command` through `docker exec`, which reaches
   `ze cli -c "show ... | json compact"` and parses the JSON.
5. A config change re-renders the file and signals the daemon; `handleSIGHUPReload`
   re-reads it.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| netlab data model ↔ ze config syntax | Jinja2 templates in `contrib/netlab/ze/` | No |
| Container ↔ kernel FIB | `fib-kernel` plugin, needs `NET_ADMIN` | No |
| External tool ↔ CLI | SSH exec on 1790, JSON pipe | No |

### Integration Points
- `feature-gates.txt` - the lab image derives its tag list from it, as the interop image
  already does.
- `mk/*.mk` - a build target for the lab image.
- `ai/INDEX.md` - the discovery row for `contrib/netlab/`.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | netlab's daemon tier accepts a single config file rather than one file per module | `netsim/daemons/dnsmasq.yml` maps the daemon to one `/etc/dnsmasq.conf` and routes its other module keys at `.ignore` files. BIRD splits per module only because it can `include`; ze has no include keyword (no match for one in `internal/component/config`) | The templates must emit one file per module, and ze would need an include mechanism it does not have | A `netlab create` run against a real netlab install | **confirmed 2026-08-14** against netlab 26.08. A `ze.yml` mapping `ze: /etc/ze/ze.conf` plus `bgp: /etc/ze/bgp.ignore` passes `netlab create`; the generated `clab.yml` bind-mounts `node_files/r1/ze` at `/etc/ze/ze.conf`. Mapping only the daemon key fails with `Cannot find bgp configuration template`, so every declared module needs a key |
| A-2a | A user declared in the running config plus `ZE_SSH_USERNAME` / `ZE_SSH_PASSWORD` is enough for `ze cli -c` to run unattended, with no `ze init` and no zefs power user | `ssh_infra.go` `UsersFunc` is the running-config source | Credentials become a code change and the spec grows an auth surface | A daemon started from the rendered config on a clean config dir | **confirmed 2026-08-14**. Daemon logged `zefs power user unavailable` and still answered `ze cli --remote 127.0.0.1:2222 -c "show version"` with the version string, exit 0 |
| A-2b | The rendered config can carry `plaintext-password`, which ze hashes for us | The YANG description says the leaf is hashed into `password` on commit | The template must carry a precomputed bcrypt hash, which Jinja2 cannot compute, so the hash has to be a fixed lab constant in the daemon YAML | The same daemon run | **broken 2026-08-14, then FIXED by phases 1-2**. Hashing lived only in the editor commit path (`internal/component/cli/editor_commit.go`), which a boot-time file load never reached: `SSH auth failure` and exit 1 (`plan/journal/silent-fall-through.md`). `LoadConfig` now calls `ApplyPasswordHashing`, proven end to end by `test/plugin/config-plaintext-password-at-boot.ci`. A rendered config MAY carry `plaintext-password`; a precomputed hash is no longer required |
| A-3 | `netlab validate` can parse what `\| json compact` emits for the show commands ze would declare | `ai/rules/cli.md` JSON Format section; validate.md requires "valid JSON" | Each declared feature needs a new JSON show command before it can be declared | Run the show commands and parse the output with a JSON parser | **broken 2026-08-14, then FIXED in phase 4**. The pipe ran and the `--format` flag default re-rendered its JSON back to YAML on the `-c` path, so `netlab_show_command` would have emitted YAML with exit 0. `renderCommandOutput` (`internal/component/cli/client/main.go`) now gives an explicit format pipe precedence over the flag. `test/plugin/netlab-lab-profile.ci` parses the output with `json.loads` and was red first |
| A-4 | Every feature declared in the daemon YAML is proven by netlab's own integration tests, not by ze's | `docs/dev/integration-tests.md`: 200+ tests, run per device | An over-broad `features:` block claims support ze cannot demonstrate | Run `netlab up -d ze <test> --validate` per declared feature | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The mirrored templates drift from ze config syntax | A syntax change lands and nothing fails | A render-and-parse test: render the templates with fixture data, feed the output to the config parser |
| R-2 | Two image recipes diverge in their feature derivation | The lab image builds a binary the interop image would not | Both derive from `feature-gates.txt`; a check compares the derived tag lists |
| R-3 | A `features:` block promises more than ze demonstrates | netlab's integration tests fail after the artifacts are published | Declare the minimum first: initial plus BGP. Add a module only with a passing test |
| R-4 | The lab image is mistaken for the deployment image | Users run the alpine image in production | Distinct names and a docs table that states which is which |
| R-5 | Hashing at load makes every boot pay bcrypt, which is deliberately slow | Daemon start time grows with the number of users | The transform runs only where a `plaintext-` sibling exists. A config carrying hashes pays nothing |
| R-6 | Every reload re-hashes, and bcrypt salts randomly, so the same file yields a different tree each time | Reload churn, or a diff that is never empty | **Resolved by reading the reload path 2026-08-14, cost bounded and accepted.** The provider half is unaffected: `applyLoadedTreeToProvider` (`cmd/ze/hub/main_reload.go`) calls `SetRoot` for EVERY root unconditionally, so every watcher already re-fires on every SIGHUP whatever the content. The plugin half does diff: `ReloadConfig` (`internal/component/plugin/server/reload.go`) runs `config.DiffMaps(running, newTree)`, returns early when nothing changed, and matches each plugin's wanted roots against the changed ones. An unstable hash under `system` defeats that early return, and reaches any plugin whose wanted roots cover `system` or are wildcarded. The cost is a reload that does work instead of none, and only for a config carrying a plaintext password, which the editor path never writes |
| R-7 | The warning fires on every reload, not only at boot | Log volume on a daemon reloaded often | Decide whether the warning is per-load or per-changed-file, and test it |
| R-8 | The lab image and the deployment image diverge, so a defect reproduces in one and not the other | A lab passes and the same config fails in production, or the reverse | Both derive their tag list from `feature-gates.txt`, and `TestLabImageTagsMatchFeatureGates` compares them |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Nothing user-visible in the daemon. The failure mode is a lab that will not start, or templates that render invalid config |
| How is it reverted? | Single commit revert. No config migration, no wire-visible change |
| Who else touches this path? | `plan/spec-release-distribution.md` (excludes containers), `plan/spec-fixit-ipsec-interop-cli-credentials.md` (shares the credential diagnosis) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ze start <file>` where the file holds a `plaintext-` leaf | → | `LoadConfig` calling `config.ApplyPasswordHashing` | `TestLoadConfigHashesPlaintextPassword` |
| SIGHUP with the same file | → | `handleSIGHUPReload` then the same `LoadConfig` | `TestReloadHashesPlaintextPassword` |
| An SSH login as the config-declared user | → | `LocalAuthenticator.Authenticate` reading the hashed leaf | `netlab-lab-profile` (`.ci`) |
| `make ze-docker-lab` | → | the tag list derived from `feature-gates.txt` | `TestLabImageTagsMatchFeatureGates` |
| `netlab create` on the reference topology | → | `contrib/netlab/ze.yml` and its templates | `ze-netlab-render-check` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A config file declaring `system authentication user u { plaintext-password p }` is loaded at daemon start | `u` authenticates over SSH with password `p` |
| AC-2 | The same file is reloaded by SIGHUP | Same result. The reload path shares the loader, so it needs no branch of its own |
| AC-3 | The same file is loaded | The daemon logs one warning naming the file, because the plaintext secret stays on disk where the operator wrote it |
| AC-4 | The loaded tree is inspected after start | It carries the canonical hashed leaf and no `plaintext-` leaf |
| AC-5 | An existing config commits a plaintext password through the editor | Unchanged: hashed, plaintext dropped, serialized file carries no plaintext |
| AC-6 | `make ze-docker-lab` | An image whose `ze` runs `start <config>` and accepts a config whose root is `bgp` |
| AC-7 | `make ze-docker` with no `ZE_TAGS` set | The binary carries the default feature set, so a `bgp` config root is accepted |
| AC-8 | `netlab create` on `contrib/netlab/topology.yml` with `contrib/netlab/ze.yml` installed | Exit 0, and the generated `clab.yml` bind-mounts the rendered config at `/etc/ze/ze.conf` |
| AC-9 | The config rendered from that topology | `ze config validate` reports it valid, exit 0 |
| AC-10 | `ze cli -c "show bgp peer list \| json compact"` against a daemon started from that config | Output parses as JSON |
| AC-11 | Each module the daemon YAML declares (bgp, ospf, isis, bfd, static) | Has a `daemon_config` key and a template, so `netlab create` finds a configuration template for it |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestLoadConfigHashesPlaintextPassword` | `internal/component/config/loader_test.go` | AC-1, AC-4: after `LoadConfig` the canonical leaf holds a bcrypt hash matching the plaintext, and the `plaintext-` leaf is gone | PASS |
| `TestLoadConfigWarnsPlaintextRemainsOnDisk` | `internal/component/config/loader_test.go` | AC-3: one warning naming the file | PASS |
| `TestLoadConfigLeavesHashedPasswordAlone` | `internal/component/config/loader_test.go` | A file already carrying a hash and no plaintext sibling is untouched | PASS |
| `TestReloadHashesPlaintextPassword` | `cmd/ze/hub/main_reload_test.go` | AC-2: the SIGHUP path inherits the transform with no branch of its own, driving `diskConfigLoaders` (the loader the daemon installs) | PASS |
| `TestCommitPathPasswordHashingUnchanged` | `internal/component/cli/editor_commit_test.go` | AC-5: the committed file carries no plaintext and its canonical leaf validates the password the operator typed | PASS (green before and after: AC-5 is the no-regression half) |
| `TestCommitPathDropsEmptyPlaintextLeaf` | `internal/component/cli/editor_commit_test.go` | AC-5, the one input that CHANGED: an empty `plaintext-` leaf is no longer serialized into the committed file | PASS (added by the Review Gate, round 2) |
| `TestLoadConfigRefusesMaskedBcryptLeaf` | `internal/component/config/loader_test.go` | The load path runs `RejectMaskedBcryptLeaves` BEFORE `ApplyPasswordHashing`, so a masked canonical leaf cannot load as a credential | PASS (added by the Review Gate, round 1) |
| `TestLoadConfigDropsEmptyPlaintextLeaf` | `internal/component/config/loader_test.go` | AC-4 for the empty value | PASS (added by the Review Gate, round 1) |
| `TestLoadConfigNamesNoSourceItWasNotGiven` | `internal/component/config/loader_test.go` | AC-3: a caller that names no config source gets no invented name | PASS (added by the Review Gate, round 1) |
| `TestLabImageTagsMatchFeatureGates` | `scripts/dev/` sibling test | AC-6, AC-7: both image targets derive their tag list from `feature-gates.txt`, no second list | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A: this spec adds no numeric input | | | | |

### Functional Tests
<!-- The lab config profile is the user-facing artifact: a `.ci` proves a daemon
     started from it answers the show commands netlab validation depends on. -->
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `netlab-lab-profile` | `test/plugin/netlab-lab-profile.ci` | AC-9, AC-10: a daemon started from the committed render `contrib/netlab/golden/r3.conf` accepts it, an SSH login as the user that render declared succeeds, and `show bgp peer list \| json compact` returns parseable JSON. The config is READ from the golden through `ZE_REPO_ROOT`, never copied into the test, so a template change reaches it | PASS |
| `config-plaintext-password-at-boot` | `test/plugin/config-plaintext-password-at-boot.ci` | AC-1, AC-3: an operator writing `plaintext-password` in a config file can log in, the daemon warns naming the file, and the file is not rewritten. Moved out of `test/parse`: that runner gives a child no PATH entry for `ze`, so a daemon-plus-login script cannot run there | PASS |

## Files to Modify
- `internal/component/config/loader.go` - `LoadConfig` calls `ApplyPasswordHashing`
  beside `refuseInvalidCustomSections`, and warns that the plaintext stays on disk.
- `Makefile` - `ze-docker` passes the `ZE_FEATURES` it already computes (the
  `$(ZEBIN_TEST)` rule reads it), plus a new `ze-docker-lab` target.
- `docs/guide/docker.md` - two images, a table saying which is which.
- `ai/INDEX.md` - discovery row for `contrib/netlab/`.

## Files to Create
- `docker/Dockerfile.lab` - alpine base, `tini`, `iproute2`, tags passed in as a build arg.
- `contrib/netlab/README.md` - what this is, and how to install it into a netlab checkout.
- `contrib/netlab/ze.yml` - the daemon definition, `clab.build: False`, image `netlab/ze:latest`.
- `contrib/netlab/ze/ze.j2` - the whole running configuration.
- `contrib/netlab/ze/{bgp,ospf,isis,bfd,routing}.j2` - the module stubs AC-11 requires.
- `contrib/netlab/ze/Dockerfile.j2` - for the day the image is built by netlab.
- `contrib/netlab/topology.yml` - the reference topology the render check runs.
- `contrib/netlab/golden/{r1,r2,r3}.conf` - the committed render, three nodes because
  netlab runs an IGP only on internal links. `r3.conf` is the one the `.ci` runs.
- `test/plugin/netlab-lab-profile.ci`, `test/plugin/config-plaintext-password-at-boot.ci`.
- `docs/guide/netlab.md` - how to run ze under netlab.

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | No new leaf. `plaintext-password` already exists in `internal/component/ssh/yang/ze-ssh-conf.yang`; this spec makes an existing leaf work on a second path |
| YANG validation constraints | N-A | No new leaf |
| YANG custom validators | N-A | The transform is schema-driven through `ze:bcrypt`, not a validator |
| CLI commands/flags | N-A | No new verb. `ze cli -c` and the `\| json` pipe both exist |
| CLI grammar (keyword before value) | N-A | No new grammar |
| Editor autocomplete | N-A | No new leaf |
| Functional test for new RPC/API | Yes | `test/plugin/netlab-lab-profile.ci`, `test/plugin/config-plaintext-password-at-boot.ci` |
| Pipe completeness | N-A | The show commands netlab calls already route through the existing pipe machinery |
| Env var registration | N-A | `ze.ssh.username` and `ze.ssh.password` are registered in `internal/core/ssh/client/client.go` |
| Doctor check for runtime dependencies | N-A | The daemon gains no file path, socket, port, or binary. `contrib/netlab/` needs netlab and docker, which are developer tools and belong in the render check, not in `ze doctor` |
| Prometheus counters/metrics | N-A | No new observable state |
| BGP family surface | N-A | No SAFI, capability, or attribute |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` -- ze runs as a netlab device |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md` -- what `plaintext-password` does in a config FILE, which until now it did not |
| 3 | CLI command added/changed? | N-A | No verb changes |
| 4 | API/RPC added/changed? | N-A | None |
| 5 | Plugin added/changed? | N-A | None |
| 6 | Has a user guide page? | Yes | `docs/guide/netlab.md` (new) |
| 7 | Wire format changed? | N-A | None |
| 8 | Plugin SDK/protocol changed? | N-A | None |
| 9 | RFC behavior implemented, changed, or newly proven? | N-A | No protocol behavior changes |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` -- the render check and its tier |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` -- lab-tool support is a comparison axis, and BIRD has no IS-IS |
| 12 | Internal architecture changed? | Yes | `docs/DESIGN.md` "Config Pipeline": the load path applies the validators and then the password transform, before `ExtractPluginsFromTree` |
| 13 | Route metadata keys added/changed? | N-A | None |
| 14 | Prometheus counters added/changed? | N-A | None |
| 15 | Registered plugin, event, command, capability, or inventory changed? | N-A | None |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `docs/guide/docker.md` (image table, `ZE_TAGS`), `docs/guide/operator-access-rbac.md` (hashing anchor), `docs/DESIGN.md`, `docs/functional-tests.md` (`internal/test/tmpfs/` path), `docs/features.md` and `docs/guide/command-reference.md` (format pipe beats `--format`) |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/docker.md` run examples, `docs/guide/authentication.md`, `docs/guide/configuration.md`, `docs/guide/monitoring.md`, `docs/guide/config-reload.md` |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the config load path reaches the transform
   - Tests: `TestLoadConfigHashesPlaintextPassword`, `TestReloadHashesPlaintextPassword`
   - Files: `internal/component/config/loader.go`
   - Verify: both tests fail first, because `LoadConfig` never calls
     `ApplyPasswordHashing`. That failure IS the defect the journal row records
2. **Phase: The transform on the load path**
   - Tests: the two above, plus `TestLoadConfigWarnsPlaintextRemainsOnDisk` and
     `TestLoadConfigLeavesHashedPasswordAlone`
   - Files: `internal/component/config/loader.go`
   - Verify: green, and `TestCommitPathPasswordHashingUnchanged` still green. The editor
     path is not edited: it already calls the shared function
3. **Phase: The lab image**
   - Tests: `TestLabImageTagsMatchFeatureGates`
   - Files: `Makefile`, `docker/Dockerfile.lab`
   - Verify: `make ze-docker-lab` then run the image against a `bgp` config root. Also
     `make ze-docker` with `ZE_TAGS` unset, which must now accept the same config
4. **Phase: The contributed artifacts**
   - Tests: `netlab-lab-profile`, `config-plaintext-password-at-boot`
   - Files: everything under `contrib/netlab/`, the two `.ci` files
   - Verify: `netlab create` exits 0 on the reference topology, the rendered config
     matches the committed golden, and `ze config validate` accepts it
5. **Phase: Docs and discovery**
   - Files: `docs/guide/netlab.md`, `docs/guide/docker.md`, `docs/guide/configuration.md`,
     `docs/features.md`, `docs/comparison.md`, `docs/functional-tests.md`, `ai/INDEX.md`
   - Verify: `make ze-doc-test`, `make ze-verify-wiring-docs`

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:symbol |
| Correctness | The load path hashes exactly what the commit path hashes: same function, no second walk, no divergent prefix handling |
| Data flow | The transform runs before anything reads a credential, and before the plugin extraction that follows it in `LoadConfig` |
| No layering | The editor keeps ONE call to the shared function. If a second hashing implementation appears anywhere, the change is wrong (`ai/rules/no-layering.md`) |
| Uniformity | The fix sits beside `refuseInvalidCustomSections`, which is the same defect class already resolved once |
| Naming | The lab image name is distinct from the deployment image, in the Makefile and in the docs |
| Rule: `ai/rules/repo-maintenance.md` | `contrib/netlab/` is reachable from `ai/INDEX.md` and has a check that fails when it drifts |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Load path hashes the leaf | `make ze-test-pkg PKG=./internal/component/config` |
| SIGHUP path inherits it | `make ze-test-pkg PKG=./cmd/ze/hub` |
| Lab image runs a BGP-capable ze | `make ze-docker-lab` then the image with a `bgp` config |
| `make ze-docker` no longer ships a BGP-less binary | same, with `ZE_TAGS` unset |
| netlab accepts the daemon definition | `netlab create` in `contrib/netlab/`, exit 0 |
| Rendered config is valid ze config | `ze config validate contrib/netlab/golden/r1.conf` |
| Templates have not drifted | `make ze-netlab-render-check` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| The plaintext stays on disk | The load path does NOT rewrite the operator's file. The warning is what tells them, and it must name the file without printing the secret |
| Log redaction | The warning and any new log line pass through the existing redaction, so no password reaches a log (`internal/core/redact/redact.go`) |
| The lab credential is not a deployment credential | `contrib/netlab/` ships a well-known lab password. The docs must say so, and no default in ze itself may reference it |
| No weakening of the auth guards | `CheckPassword` keeps refusing an empty hash, and keeps accepting a hash-as-token only over trusted-local transport |

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

- **The single-file design is proven, and it costs one map entry per module.** netlab
  decides which config templates a node needs from `daemon_config`, not from the
  `features:` block. A module declared with no key fails `netlab create` outright. The
  dnsmasq pattern (real file for the daemon, `.ignore` file for every module) is what
  makes a daemon without an include directive work.
- **The rendered config carries a plaintext password, because phase 2 made the load path
  hash it.** This bullet previously said the opposite, on the strength of A-2b before it
  was fixed. Jinja2 cannot compute bcrypt, so a hash-only load path would have forced a
  fixed constant into the daemon YAML. It no longer does.
- **A first render found two config-syntax facts worth keeping**: `family ipv4/unicast`
  rejects a missing `prefix { maximum }`, and the SSH server is enabled under
  `environment { ssh { enabled } }`, not under `system`.
- The netlab daemon tier is deliberately small. BIRD's entire integration is one YAML
  file and fourteen Jinja2 files. The cost of a netlab integration is not the plumbing,
  it is the per-feature promise: every key under `features:` is checked by netlab's own
  integration suite.
- Ze has working IS-IS, which BIRD does not. As a container daemon that is a stronger
  feature declaration than any netlab daemon except FRR.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Mirror the netlab artifacts under `contrib/netlab/` | Upstream only; ze-side only with no PR | The templates emit ze config syntax, so they belong beside it and a ze test can render them. The upstream PR copies from the mirror |

## Known Limitations
- **The lab has never been started.** The machine that wrote this integration has no
  containerlab, so `netlab up` and `netlab validate` were never run. No BGP session was
  established between two ze nodes under netlab, no route reached a FIB, and no `ping`
  ran. `contrib/netlab/ze.yml` declares five modules and only `bgp` has evidence beyond
  "the template renders and the render parses". The declaration is deliberate. A lab run
  on a machine with containerlab is owed before the upstream pull request is opened.
- No LLDP. Nothing in ze sends or receives an LLDP frame, so validation tests that read
  a neighbor table cannot pass.
- No published image. netlab daemons build locally from `Dockerfile.j2`, so this is not
  blocking, but a user cannot pull a ze image.
- netlab sends no SIGHUP. Ze reloads on SIGHUP, so `ze.yml` sets
  `features.initial.reload: false` and a configuration change in a running lab needs the
  signal by hand or a node restart.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-verify` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Deferral shard resolved: no live row without a destination

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `scripts/dev/review_gate.py`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)

## Implementation Summary

### What Was Implemented
- **The transform on the load path.** `LoadConfig`
  (`internal/component/config/loader.go`) calls `ApplyPasswordHashing` after
  `refuseInvalidCustomSections` and before `ExtractPluginsFromTree`, so the
  validators judge the tree the operator wrote and every credential reader sees the
  hash. `parseTreeWithYANG` returns the schema it already built, because
  `YANGSchemaWithPlugins` caches nothing. `ApplyPasswordHashing`
  (`internal/component/config/password_hash.go`) returns the dot-paths it hashed,
  and `warnPlaintextOnDisk` logs one warning naming the file, with the leaf paths
  through `redact.Command`.
- **SIGHUP inherits it with no branch of its own.** `diskConfigLoaders`
  (`cmd/ze/hub/main_reload.go`) lifts the loader closures `runYANGConfig` built
  inline, so a test drives the loader the daemon installs.
- **The lab image.** `docker/Dockerfile.lab` (alpine 3.21, `tini`, `iproute2`),
  `make ze-docker-lab` building `netlab/ze:latest`, and both Dockerfiles deriving
  the default-on tag list from `feature-gates.txt`. `ZE_TAGS` is now EXTRA tags on
  top of that set, never a replacement for it.
- **The contributed artifacts.** `contrib/netlab/`: the daemon YAML, the Jinja2
  templates that emit ze config, one reference topology, three committed goldens and
  a README. `scripts/dev/netlab_render_check.py` with `make ze-netlab-render-check`
  renders the mirror with a real netlab and validates each golden.
  `test/plugin/netlab-lab-profile.ci` runs a daemon from a golden.
- **Docs and discovery.** `docs/guide/netlab.md` (new), plus corrections across
  `docs/guide/docker.md`, `authentication.md`, `configuration.md`, `monitoring.md`,
  `config-reload.md`, `operator-access-rbac.md`, `command-reference.md`,
  `README.md`, `docs/DESIGN.md`, `docs/features.md`, `docs/comparison.md` and
  `docs/functional-tests.md`.

### Bugs Found/Fixed
- **`plaintext-password` was ignored by a config FILE load.** The hashing had one
  caller, the editor commit path, so a file carrying the leaf loaded with an empty
  canonical leaf and `CheckPassword` (`internal/component/authz/auth.go`) refused
  every login for that user. `ze config validate` called the file valid. Covered by
  `TestLoadConfigHashesPlaintextPassword` and
  `test/plugin/config-plaintext-password-at-boot.ci` (red first with
  `SSH auth failure username=lab`). Journal row:
  `plan/journal/silent-fall-through.md`.
- **`ze cli -c "show ... | json compact"` printed YAML.** `(*cliClient).Execute`
  handed the chain output to `printFormatted`, whose `default:` branch re-rendered
  the JSON as YAML under the `--format` flag default. `netlab_show_command` is that
  exact string, so every netlab validation over a ze node would have failed to parse
  with exit 0. Fixed at the source by `renderCommandOutput`
  (`internal/component/cli/client/main.go`) and `command.HasFormatPipe`. Covered by
  `TestRenderCommandOutputFormatPipeBeatsFormatFlag` and the `.ci`. Journal row:
  `plan/journal/silent-fall-through.md`.
- **The parse suite ignored `mode=` on a tmpfs file.** `setupWorkDir`
  (`internal/test/runner/parsing.go`) open-coded the write loop that
  `(*tmpfs.Tmpfs).WriteTo` owns and forced `0o644`, so `exec=./x.sh` died with
  "permission denied". Covered by `TestParsingSuiteHonorsTmpfsMode`. Journal row:
  `plan/journal/helper-bypassed-by-an-open-coded-copy.md`.
- **`make ze-docker` built an untagged binary.** With `ZE_TAGS` unset the image
  rejected a `bgp` config root as an unknown top-level keyword. Covered by
  `TestLabImageTagsMatchFeatureGates` (`scripts/dev/lab_image_tags_test.py`).
- **`check_doc_links.py` verified no citation into `contrib/`.** `candidate_paths`
  discards any span whose first segment is absent from `KNOWN_ROOTS`. `contrib` was
  added with `KnownRootTest`. Four tracked roots stay absent (`demos`, `docker`,
  `iso`, `tmp`); that is one decision covering all four and it belongs to the owner.
  Journal row: `plan/journal/gate-excludes-part-of-its-population.md`.

### Documentation Updates
- `docs/guide/netlab.md` (new): the user-facing page, carrying an explicit
  "what is proven, what is not" table. Anchors into `mk/test-integration.mk`,
  `cmd/ze/hub/main_reload.go` and `internal/component/config/loader.go`.
- `docs/guide/authentication.md`: new `### Passwords in a config file` section,
  anchored on `internal/component/config/loader.go` and `editor_commit.go`. Quotes
  the warning text and states the load path does not rewrite the file.
- `docs/guide/configuration.md`, `monitoring.md`, `config-reload.md`: the leaf rows
  and the basic-auth paragraph now cover the load path as well as the commit path.
- `docs/guide/docker.md`: a two-image table with measured sizes, and `ZE_TAGS`
  documented as EXTRA tags.
- `docs/features.md`, `docs/guide/command-reference.md`: a format pipe beats the
  `--format` flag.
- `docs/comparison.md`, `docs/features.md`, `docs/guide/README.md`,
  `docs/functional-tests.md`, `docs/DESIGN.md`: netlab support, the render check and
  its tier, and the two whole-tree transforms `LoadConfig` runs.
- `ai/CODE-TO-DOCS.md`, `ai/DOCS-TO-CODE.md` regenerated by `make ze-doc-index`.
  `ai/INDEX.md` and `ai/PACKAGE-MAP.md` carry the same work and are already
  committed (see Deviations).
- `make ze-doc-test`: source anchors pass, no documentation drift, discovery indexes
  fresh. One red, `rules-points: writing.md is stale`, which this work did not cause
  (see Pre-Commit Verification).

### Deviations from Plan
- `config-plaintext-password-at-boot.ci` lives in `test/plugin/`, not `test/parse/`.
  The parse runner gives a child process no PATH entry for `ze`, so a
  daemon-plus-login script cannot run there.
- The reference topology has three nodes and three goldens (`r1`, `r2`, `r3`), not
  the one the spec named. netlab runs an IGP only on INTERNAL links, so an iBGP pair
  plus an eBGP neighbour is the smallest topology that exercises the declared
  modules.
- The spec's Design Insights said the rendered config could not carry
  `plaintext-password`. Phase 2 made that false, and the bullet was corrected.
- **The editor commit path DID change, for one input, against the spec's
  "Behavior to preserve" line.** A Review Gate fix made `hashPlaintextSibling` delete
  a present-but-empty `plaintext-` leaf. `serializeSetMetaChild`
  (`internal/component/config/serialize_set.go`) writes every name present in the
  tree, so before the fix a commit of a candidate holding `plaintext-password ""`
  wrote that line into `config.conf`. It no longer does. The change was kept rather
  than reverted: the leaf is `ze:ephemeral` and AC-5's own words require the
  serialized file to carry no plaintext, so writing the leaf was the defect. Scoping
  the delete to the load path would need a second walk or a parameter, which the
  spec's Critical Review Checklist forbids. Pinned by
  `TestCommitPathDropsEmptyPlaintextLeaf`.
- `Makefile`, `ai/INDEX.md` and `ai/PACKAGE-MAP.md` hold this spec's edits and are
  ALREADY COMMITTED, in commit `80f0b8b57` from another session working the same
  checkout. The content is correct and preserved; only attribution is off. History
  is not rewritten to reclaim it (`ai/rules/git-safety.md`).

## Mistake Log

<!-- One table, one place. Ship the `none` row and either replace it or leave it
     deliberately: three separate empty tables produced three separate 67-82%
     untouched rates, because an empty table asks nothing.
     Kind: assumption (a broken A-N) | approach (a route abandoned) | escalation
     (a mistake frequent enough to deserve a rule). -->
| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | A-2b: a rendered config can carry `plaintext-password` and ze hashes it | Hashing had exactly one caller, `internal/component/cli/editor_commit.go`. A file load never reached it, so the login was refused with no diagnostic | A daemon started from the rendered config logged `SSH auth failure username=netlab` and `ze cli -c` exited 1 | Fixed at the source in phases 1-2. `LoadConfig` calls the same shared transform. A-2b is now confirmed |
| assumption | A-3: `\| json compact` on the `ze cli -c` path emits JSON | The `--format` flag default re-rendered the pipe's JSON as YAML and exited 0, so a consumer saw a parse error with no cause | `json.loads` failed in `test/plugin/netlab-lab-profile.ci` | Fixed at the source in phase 4 by `renderCommandOutput` and `command.HasFormatPipe`. A-3 is now confirmed |
| assumption | A-4: netlab's own integration suite proves every feature `contrib/netlab/ze.yml` declares | This machine has no containerlab, so `netlab up` and `netlab validate` never ran. Only BGP is demonstrated end to end, by a daemon started from a golden | Phase 4 tried to boot the lab and could not | Recorded truthfully rather than narrowed. The five-module declaration is the owner's choice, so the gap is stated in Goal Validation, in Known Limitations and in `docs/guide/netlab.md`, and the follow-up is named there |
| approach | A Review Gate fix (delete the empty ephemeral leaf) was believed to touch the load path only | The two paths share one function, so it changed what the EDITOR commits: a candidate holding `plaintext-password ""` no longer writes that line to `config.conf` | Review round 2 traced the shared function into `serializeSetMetaChild` | Kept, not reverted, because the new behavior is the one AC-5 describes. Recorded in Deviations and pinned by `TestCommitPathDropsEmptyPlaintextLeaf` |
| approach | Install the mirrored artifacts into the operator's netlab package to render them | netlab reads `./topology-defaults.yml` and `topology:templates` from the working directory, so a scratch lab needs no install at all | Writing `scripts/dev/netlab_render_check.py` | The check builds a scratch lab from the mirror and never writes into a netlab install |

## Implementation Audit

<!-- BLOCKING before the learned summary. See ai/rules/completion.md.
     Status: Done (with file:line) | Partial | Skipped | Changed.
     Partial and Skipped both require explicit user approval. -->

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Produce the ze-side artifacts a netlab daemon integration needs | Done | `contrib/netlab/ze.yml`, `contrib/netlab/ze/*.j2`, `docker/Dockerfile.lab` | The daemon tier needs one YAML, one template directory and an image. All three exist |
| Mirror the netlab-side artifacts here, so the templates are versioned beside the config syntax they emit | Done | `contrib/netlab/`, `scripts/dev/netlab_render_check.py` (`build_lab`, `compare`, `validate_golden`) | The check renders the mirror with a real netlab and diffs it against `contrib/netlab/golden/` |
| A lab-grade container image whose binary carries the default feature set | Done | `docker/Dockerfile.lab`, `Makefile` `ze-docker-lab` (committed in `80f0b8b57`) | Both Dockerfiles derive the tag list from `feature-gates.txt` |
| A documented minimal lab config profile and the config-push contract | Done | `docs/guide/netlab.md`, `contrib/netlab/golden/r3.conf` | File plus SIGHUP, and the page states netlab sends no SIGHUP itself |
| `contrib/netlab/` reachable from `ai/INDEX.md` with a check that proves the mirror still renders | Done | `ai/INDEX.md` rows 209, 450, 451 (committed in `80f0b8b57`); `make ze-netlab-render-check` | |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestLoadConfigHashesPlaintextPassword` (`internal/component/config/loader_test.go`), `test/plugin/config-plaintext-password-at-boot.ci` (index 151, PASS in the 633-test run) | Producer: `LoadConfig` (`internal/component/config/loader.go`) calling `ApplyPasswordHashing` |
| AC-2 | Done | `TestReloadHashesPlaintextPassword` (`cmd/ze/hub/main_reload_test.go`) | Drives `diskConfigLoaders`, the loader the daemon installs, then `doReload`. No `test/reload/*.ci`: see Deferrals and the Review Gate NOTE |
| AC-3 | Done | `TestLoadConfigWarnsPlaintextRemainsOnDisk`, `TestLoadConfigNamesNoSourceItWasNotGiven` | One warning for two leaves; no secret in it; a caller that names no source gets no invented name |
| AC-4 | Done | `TestLoadConfigHashesPlaintextPassword`, `TestLoadConfigDropsEmptyPlaintextLeaf` | The ephemeral leaf never survives the load, empty value included |
| AC-5 | Done, with one recorded change | `TestCommitPathPasswordHashingUnchanged` and `TestCommitPathDropsEmptyPlaintextLeaf` (`internal/component/cli/editor_commit_test.go`) | The first is green before and after by design. The second pins a deliberate change: an EMPTY `plaintext-` leaf is no longer serialized into the committed file. See Deviations |
| AC-6 | Done | `docker run --rm netlab/ze:latest config validate /g/r3.conf` exit 0 on 2026-08-14; `TestLabImageTagsMatchFeatureGates` | The image accepts a `bgp` config root |
| AC-7 | Done | `docker run --rm ze:latest config validate /g/r3.conf` exit 0 on 2026-08-14, image built with `ZE_TAGS` unset | Before this change the same command failed on an unknown top-level keyword |
| AC-8 | Done | `make ze-netlab-render-check` against netlab 26.08, recorded 2026-08-14: `netlab create` exit 0, 3/3 renders match `golden/` | netlab is not installed on this machine by default. The check errors loudly on absence and never skips |
| AC-9 | Done | Same run, 3/3 goldens validate; `test/plugin/netlab-lab-profile.ci` re-validates `r3.conf` with no netlab needed | |
| AC-10 | Done | `test/plugin/netlab-lab-profile.ci` (index 353, PASS) parses `show bgp peer list \| json compact` with `json.loads` and requires a non-empty `peers` key | Red first on that step. Producer: `renderCommandOutput` (`internal/component/cli/client/main.go`) |
| AC-11 | Done | `contrib/netlab/ze.yml` maps all six `daemon_config` keys and all five module templates exist; `netlab create` exit 0 on a topology declaring all five | Structure re-verified 2026-08-14; the `netlab create` half is the recorded run |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestLoadConfigHashesPlaintextPassword` | PASS | `internal/component/config/loader_test.go` | |
| `TestLoadConfigWarnsPlaintextRemainsOnDisk` | PASS | same | |
| `TestLoadConfigLeavesHashedPasswordAlone` | PASS | same | |
| `TestReloadHashesPlaintextPassword` | PASS | `cmd/ze/hub/main_reload_test.go` | |
| `TestCommitPathPasswordHashingUnchanged` | PASS | `internal/component/cli/editor_commit_test.go` | |
| `TestLabImageTagsMatchFeatureGates` | PASS | `scripts/dev/lab_image_tags_test.py` | 9 tests OK; run by `TestPythonUnitTests` |
| `TestLoadConfigRefusesMaskedBcryptLeaf` | PASS | `internal/component/config/loader_test.go` | Added by the Review Gate, round 1 |
| `TestLoadConfigDropsEmptyPlaintextLeaf` | PASS | same | Added by the Review Gate, round 1 |
| `TestLoadConfigNamesNoSourceItWasNotGiven` | PASS | same | Added by the Review Gate, round 1 |
| `TestCommitPathDropsEmptyPlaintextLeaf` | PASS | `internal/component/cli/editor_commit_test.go` | Added by the Review Gate, round 2, pinning the editor change Deviations records |
| `netlab-lab-profile` | PASS | `test/plugin/netlab-lab-profile.ci` | |
| `config-plaintext-password-at-boot` | PASS | `test/plugin/config-plaintext-password-at-boot.ci` | |
| `TestParsingSuiteHonorsTmpfsMode` | PASS | `internal/test/runner/tmpfs_test.go` | Covers the defect found on the way |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/component/config/loader.go` | Done | `LoadConfig` runs both halves of the pair, then warns |
| `Makefile` (`ze-docker`, `ze-docker-lab`) | Done | Already in git: another session's commit `80f0b8b57` absorbed it |
| `docs/guide/docker.md`, `ai/INDEX.md` | Done | `ai/INDEX.md` also in `80f0b8b57` |
| `docker/Dockerfile.lab` | Done | |
| `contrib/netlab/{README.md,ze.yml,topology.yml}`, `contrib/netlab/ze/*.j2` | Done | |
| `contrib/netlab/golden/*.conf` | Changed | Three goldens, not one. Recorded in Deviations |
| `test/plugin/netlab-lab-profile.ci` | Done | |
| `config-plaintext-password-at-boot.ci` | Changed | Lives in `test/plugin/`, not `test/parse/`. Recorded in Deviations |
| `docs/guide/netlab.md` | Done | |

### Audit Summary
- **Total items:** 5 requirements, 11 acceptance criteria, 12 tests, 9 file groups = 37
- **Done:** 35
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 2 (the golden count and the `.ci` location, both in Deviations)

## Goal Validation (BLOCKING)

<!-- Maps each goal from the Task section to proof it was achieved. "Tests pass"
     is not evidence for a goal; a named test with its output is.
     See ai/rules/interop-and-goal-validation.md for the required evidence per
     goal type, and for the vacuity traps: a test that would still pass with the
     behavior reverted proves nothing. -->
| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| A container image netlab can build and run, whose binary carries the default feature set (the one row the Task marked **Not met**) | functional, on the built image | `docker run --rm -v .../golden:/g:ro netlab/ze:latest config validate /g/r3.conf` exit 0, and the same for `ze:latest` built with `ZE_TAGS` unset, both on 2026-08-14. A `bgp` config root is the exact input the untagged binary used to reject |
| Produce the ze-side artifacts a netlab daemon integration needs | functional, against a real netlab | `make ze-netlab-render-check` against netlab 26.08: `netlab create` exit 0, 3/3 renders byte-match `contrib/netlab/golden/`, 3/3 goldens pass `ze config validate`. A missing netlab is an ERROR exit, never a skip (`find_netlab`, `scripts/dev/netlab_render_check.py`) |
| Mirror those artifacts here so the templates are versioned beside the config syntax they emit | drift gate | The same check is the gate. It reads `contrib/netlab/` and the committed goldens, so a ze config-syntax change that the templates do not follow fails it |
| A config delivered as a file produces a daemon an external tool can drive | functional `.ci` | `test/plugin/netlab-lab-profile.ci` (index 353 of 633, PASS): a daemon started from the committed render `golden/r3.conf`, an SSH login as the user THAT RENDER declared, and `show bgp peer list \| json compact` parsed by `json.loads` with a non-empty `peers` key. Red first on the `json.loads` step |
| A config file carrying `plaintext-password` produces a user who can log in | functional `.ci` | `test/plugin/config-plaintext-password-at-boot.ci` (index 151, PASS): red first with `SSH auth failure username=lab`. The daemon warns naming the file, and the file is not rewritten |
| **A lab that establishes a session and passes a `ping`** (the Required Reading constraint from `ai/rules/completion.md`: "rendering the templates is not evidence") | **NOT DEMONSTRATED** | This machine has no containerlab, so `netlab up` and `netlab validate` were never run. No BGP session was established between two ze nodes under netlab, no route reached a FIB, and no `ping` was run. `test/plugin/netlab-lab-profile.ci` says so in its own header: it runs ONE daemon and never establishes a session |
| **Each feature `contrib/netlab/ze.yml` declares (bgp, ospf, isis, bfd, static) passes netlab's integration test for it** | **NOT DEMONSTRATED** | Of the five, only `bgp` has evidence beyond "the template renders and the render parses", and that evidence stops at a single daemon answering a show command. `ospf`, `isis`, `bfd` and `routing.static` exist as blocks in `golden/r1.conf` and nothing more. The declaration is deliberate and is the owner's; it is recorded as a claim netlab's own suite will be the first thing to test |

**Surviving risk, carried out of this spec unmitigated.** R-3 ("a `features:` block promises more than ze demonstrates") is LIVE. Its stated mitigation was to declare the minimum first and add a module only with a passing test; the declaration is five modules and the passing tests are absent. Four surfaces state the gap so no reader is misled: `docs/guide/netlab.md` (its "what is proven, what is not" table), `docs/features.md`, `docs/comparison.md` (the cell reads `Yes (daemon, in-repo, unvalidated)`) and `contrib/netlab/README.md`, which is the file the upstream pull request copies from. **The follow-up is a lab run on a machine with containerlab, and it is owed before that pull request is opened.**

## Deferrals Resolved

<!-- Closure must leave no dangling row: deferral_unassigned_problems in
     scripts/dev/commit_helper.py WARNS (it does not block) on a live row with no
     destination -- act on the warning here, because nothing else will.
     The spec's own shard is git rm'd at closure ONLY when every row in it is
     terminal; a shard still holding a live row outlives its source spec and
     deferral_shard_removal_problems blocks its removal
     (ai/rules/planning.md). Account for every row here.
     If resolving a row empties a FOREIGN shard (its last live row becomes
     terminal), that shard is now residue and this closure removes it too. -->
| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| none: no shard was ever created | done | The spec metadata says `-`, and `ls plan/deferrals/netlab-integration.md` reports no such file. No row was deferred, so there is nothing to remove and no foreign shard was emptied |

## Review Gate

<!-- BLOCKING (ai/rules/planning.md). The review is INDEPENDENT: reviewer
     subagents or a fresh session over the actual diff, never your own inline
     reasoning about code you just wrote.

     The machine-checked artifact is the deliverable, not this table:
     scripts/dev/review_gate.py record --spec <spec> --rounds <N> ... then check.
     --rounds is the pass count and is required; more than three needs
     --rounds-reason naming the PRODUCT defect a later round found. A false
     statement in this record is a NOTE, never a reason for another round
     (ai/rules/planning.md).
     commit_helper.py runs `review_gate.py check` on the closure commit and
     refuses without a fresh, hash-pinned, CLEAN artifact. Record the artifact
     first; this table exists only to carry what was FOUND and FIXED forward
     into the learned summary. -->

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/netlab-integration-9beac809-ca2b-45fd-b3c3-9358d90729d3.md` |
| `review_gate.py check` | `review_gate: OK (13 code files, clean, hashes match ...)`, exit 0 |
| Rounds | 6. Rounds 3 to 6 were earned by a PRODUCT defect round 2 found: the round-1 fix that deletes an empty ephemeral leaf also changed what the editor COMMITS, through a shared function neither the fix nor its review had traced into `serializeSetMetaChild`. Rounds 3, 4 and 5 then found the old description of that behavior surviving in four more places, one of them the premise of a DIFFERENT open spec |
| Reviewer lenses used | Round 1, four independent agents over the complete uncommitted diff: (a) wiring, functional-test coverage, removed behavior, logic and guard audit, simplicity; (b) security, edge-case techniques, allocation safety; (c) documentation drift and discovery; (d) fresh re-derivation of every AC, wiring row and assumption. Rounds 2 to 6, one agent each, scoped to the previous round's fixes only |

Round 1 found 1 BLOCKER and 8 ISSUEs, over the complete diff. Round 2 examined only
those fixes and found 1 BLOCKER and 3 ISSUEs, including a product defect one of the
round-1 fixes introduced. Round 3 examined the round-2 fixes and found 1 ISSUE, a
stale test docstring that change left behind. Round 4 found the same false claim
again, on the exported doc comment. Round 5 stopped fixing instances and swept the
tree, which is what finally bounded the loop: it returned the last two survivors in
one answer, the unexported twin of the sentence round 4 had just fixed and the
Current Behavior premise of `plan/spec-password-weakness-warning.md`. Round 6
confirmed the class was closed.

**What the loop cost, and why it ran six times.** ONE behavior change to a shared
function was described in six places, and rounds 3 and 4 each fixed one instance and
declared victory. Fixing an instance of a false statement finds the next instance
one round later; sweeping for the CLASS ends it in one. That is this spec's Core
Insight seen from the other side: a one-caller function grows a second caller, and
every sentence written about it while it had one caller becomes a candidate lie,
including the sentences in other people's open specs.

### Findings fixed
<!-- Only BLOCKER and ISSUE. NOTEs do not block: record them and proceed.
     Every fix is new code that needs a fresh pass, so re-run until clean. -->
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | BLOCKER | `contrib/netlab/README.md` is the file the upstream pull request copies from, and it was the ONLY reader-facing surface carrying no statement that the lab was never booted. `docs/guide/netlab.md` names it the source of truth | `contrib/netlab/README.md` | A "No lab run" bullet under Known limitations: `netlab up` and `netlab validate` were never run, and no declared feature is validated by netlab's own suite |
| 2 | ISSUE | The load path took HALF a pair. `ApplyPasswordHashing` was wired without `RejectMaskedBcryptLeaves`, which both editor commit sites call first. A config whose `password` leaf held `SecretDataPlaceholder` loaded as if the placeholder were the hash, and the hash-as-token branch of `CheckPassword` (`internal/component/authz/auth.go`) authenticates a stored hash on a trusted-local transport. The placeholder is a public constant | `internal/component/config/loader.go` `LoadConfig` | The guard runs immediately before the transform, in the commit path's order. `TestLoadConfigRefusesMaskedBcryptLeaf` |
| 3 | ISSUE | `warnPlaintextOnDisk` invented `<stdin>` for a caller that named no source. Four production sites pass none, because their config comes from a blob store or a hub push and has no filesystem path. The operator was sent to the wrong place | `internal/component/config/loader.go` `warnPlaintextOnDisk` | An unnamed source is named as unnamed. `TestLoadConfigNamesNoSourceItWasNotGiven` |
| 4 | ISSUE | The same function called `redact.Command` on schema dot-paths, where it cannot match (it blanks the token AFTER a credential key, and a whole dot-path is one token), under a comment claiming it stopped a username reaching the log. A false safety claim stops the next reviewer asking | same | The inert call is removed and the comment states the real reason no secret can reach the attribute: `hashPlaintextSibling` deletes the value before the warning runs |
| 5 | ISSUE | "The load path does not rewrite the operator's file" was false at two boot sites. `applyEvolutions` (`cmd/ze/hub/main_evolve.go`) and `RecoverConfig` (`internal/component/config/stamp.go`) both re-serialize the loaded tree, and `applyEvolutions` calls `store.WriteVersion` FIRST, so the archived version keeps the original plaintext | `internal/component/config/loader.go`, `docs/guide/authentication.md` | Both now name the two exceptions and the archived plaintext |
| 6 | ISSUE | `docs/guide/authentication.md` described an `slog.Warn` for an oversize password that exists nowhere, and this spec widened what that passage covers: an oversize `plaintext-password` in a FILE now fails daemon start and fails a SIGHUP reload | `docs/guide/authentication.md` | A three-row table of the real outcomes, each checked against `hashPlaintextSibling`, `LoadConfig` and `runReload` |
| 7 | ISSUE | `docs/features.md` and `docs/guide/netlab.md` said netlab writes the file and the daemon reloads on SIGHUP. `contrib/netlab/ze.yml` sets `features.initial.reload: false` precisely because netlab sends no SIGHUP | both pages | Corrected: ze reloads on SIGHUP, netlab sends none, so a change in a running lab needs the signal by hand or a node restart |
| 8 | ISSUE | `docs/DESIGN.md` said both whole-tree transforms are shared with the editor commit path. `refuseInvalidCustomSections` is not: `editor_commit.go` makes no validation call at all | `docs/DESIGN.md` | Corrected, with anchors naming the producing symbols |
| 9 | ISSUE | The `docs/comparison.md` netlab cell read at parity with BIRD, whose integration is upstream and validated by netlab's suite | `docs/comparison.md` | The ze cell reads `Yes (daemon, in-repo, unvalidated)` |
| 10 | BLOCKER (round 2) | Finding 2's own fix reached further than intended. Deleting an empty `plaintext-` leaf in the SHARED `hashPlaintextSibling` also changed what the editor COMMITS: `serializeSetMetaChild` (`internal/component/config/serialize_set.go`) writes every name present in the tree with no ephemeral filter, so a candidate holding `plaintext-password ""` used to be written into `config.conf`. The spec's "Behavior to preserve" said the editor path does not change | `internal/component/config/password_hash.go`, `internal/component/cli/editor_commit.go` | Kept, not reverted: the leaf is `ze:ephemeral` and AC-5's own words require the serialized file to carry no plaintext, so writing it was the defect. Scoping the delete to the load path would need a second walk, which the Critical Review Checklist forbids. Recorded in Deviations and the Mistake Log, and pinned by `TestCommitPathDropsEmptyPlaintextLeaf` |
| 11 | ISSUE (round 2) | `docs/guide/authentication.md` said "Ze never edits a file the operator owns" | `docs/guide/authentication.md` | Replaced by "`LoadConfig` writes nothing", plus a paragraph naming `applyEvolutions` and `RecoverConfig` as the two steps that DO rewrite it, and the archive that keeps the plaintext |
| 12 | ISSUE (round 2) | The same page said an oversize `plaintext-password` makes the daemon refuse to start, unconditionally. `recoverableLoadError` (`cmd/ze/hub/main.go`) treats that error as recoverable, so `RecoverConfig` can start the daemon on a rollback version when the config stamp is newer than the binary | same | The row is qualified with that condition |
| 13 | ISSUE (round 2) | `TestLoadConfigRefusesMaskedBcryptLeaf` claimed to prove reject-before-hash, but its fixture carried no `plaintext-` sibling, so swapping the two calls left it green | `internal/component/config/loader_test.go` | The fixture carries the placeholder AND `plaintext-password "labsecret"`. Reject-first errors; hash-first would overwrite the placeholder and load clean |
| 14 | ISSUE (round 3) | The round-2 change left `TestApplyPasswordHashingEmptyPlaintext`'s header describing the old no-op behavior, and the test asserted nothing about the delete (`ai/rules/stale-comments.md`) | `internal/component/config/password_hash_test.go` | Header rewritten to both properties, and an assertion added that the ephemeral leaf is gone |
| 15 | ISSUE (round 4) | The SAME false statement survived on the exported doc every caller reads: `ApplyPasswordHashing` still said "No-op if the plaintext sibling is absent or empty". Rounds 3 and 4 each found this one claim in a different place | `internal/component/config/password_hash.go` | The doc was rewritten to separate the absent case (the no-op, and what makes the call idempotent) from the empty case (hashes nothing, still deleted). Two further false clauses in the same comment went with it: the returned paths are not fully ordered, because a list's entries come from a Go map, and `LoadConfig` writes no file rather than the file never being rewritten. Round 5 then swept the whole tree for the CLASS rather than fixing a third instance |
| 16 | ISSUE (round 5) | The sweep found the same sentence on the UNEXPORTED twin, three lines below the one round 4 had just fixed | `internal/component/config/password_hash.go`, `hashPlaintextSibling` | Rewritten to the same two cases |
| 17 | ISSUE (round 5) | The sweep found the claim outside this spec entirely: `plan/spec-password-weakness-warning.md` is OPEN and states the empty case as a no-op in its Current Behavior, in its post-wave verification paragraph, and in AC-5. It would have been implemented from a premise this change made false, and its line citations into `password_hash.go` are stale | `plan/spec-password-weakness-warning.md` | All three corrected, each marked as superseded on 2026-08-14 with the reason, and the stale line numbers dropped. That spec's design and scope were not touched |

### Findings recorded, not fixed (NOTE)
| Finding | Why it is not fixed here |
|---------|--------------------------|
| An explicit `mode=` on a tmpfs path is still ignored on the MAIN `.ci` path. `Record.TmpfsFiles` is `map[string][]byte`, so `record_parse.go` drops `f.Mode` and `runner_exec.go` rebuilds a mode from the extension. This spec fixed the same defect on the `test/parse` path | The fix carries `*tmpfs.Tmpfs` on `Record` and touches 8 non-test sites across 4 files, in the runner all 633 functional tests go through. It is not small enough to ride on a closing commit. Journal row: `plan/journal/helper-bypassed-by-an-open-coded-copy.md` |
| `HasFormatPipe` re-parses a string `ProcessPipesChecked` already parsed, and the precedence now has two call sites | Both delegate to ONE predicate, `hasFormatOp`, so the rule has one home. The named deeper shape adds a return value to `ProcessPipesChecked` and its two sibling wrappers, across three non-test call sites, to save one re-parse on a cold CLI path. That is more machinery, not less (`ai/rules/simplicity.md`) |
| A config carrying both `password <hash>` and `plaintext-password p` has the hash overwritten with no diagnostic | Identical on the load and commit paths, so the two do not diverge. Changing it changes the editor's behavior, which AC-5 fixes as unchanged |
| Every `LoadConfig` pays bcrypt per plaintext leaf, and the managed `Handler.Validate` closure calls it once per pushed config | Cost, not correctness. R-5 and R-6 already bound it: a config carrying only hashes pays nothing |
| `docker/Dockerfile.lab` pins no version for `tini` and `iproute2`, so the lab image is less reproducible than the `scratch` deployment image | A lab image, not a deployment artifact. Recorded for the day it is published |
| `ApplyPasswordHashing`'s doc says the returned paths are in walk order; `descend` ranges a Go map, so multi-entry order is random | The paths feed one log attribute. Nothing orders on them |

## Pre-Commit Verification

<!-- BLOCKING. Do NOT trust the audit above: re-verify independently and paste
     the evidence. For each row run a command (ls, grep, go test -run) now.

     EVERY sub-table needs at least one data row: pre_commit_verification_gaps
     in scripts/dev/commit_helper.py checks them one by one and names the empty
     ones. A row in Files Exist is not evidence for AC Verified.
     Not acceptable: "already checked", "should work", a pointer to the audit. -->

### Files Exist (ls)
`ls` run 2026-08-14, sizes as reported:
| File | Exists | Evidence |
|------|--------|----------|
| `docker/Dockerfile.lab` | Yes | 2.0K |
| `contrib/netlab/README.md` | Yes | 5.3K |
| `contrib/netlab/ze.yml` | Yes | 3.2K |
| `contrib/netlab/topology.yml` | Yes | 1.7K |
| `contrib/netlab/ze/*.j2` | Yes | 7 files: `ze.j2` 7.1K, `Dockerfile.j2` 1.1K, and the five module stubs `bgp`, `ospf`, `isis`, `bfd`, `routing` |
| `contrib/netlab/golden/*.conf` | Yes | `r1.conf` 2.9K, `r2.conf` 2.4K, `r3.conf` 1.7K |
| `test/plugin/netlab-lab-profile.ci` | Yes | 6.9K |
| `test/plugin/config-plaintext-password-at-boot.ci` | Yes | 4.8K. The spec's `test/parse/` path does NOT exist; both spec cells were corrected at closure |
| `docs/guide/netlab.md` | Yes | 6.5K |
| `scripts/dev/netlab_render_check.py` | Yes | 9.5K |
| `scripts/dev/lab_image_tags_test.py` | Yes | 8.1K |
| `internal/component/config/loader_test.go` | Yes | 8.3K |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | A file-declared user logs in over SSH | `make ze-test-pkg PKG=./internal/component/config` exit 0; `config-plaintext-password-at-boot` PASS at index 151 of the 633-test `make ze-plugin-test` run (exit 0) |
| AC-2 | SIGHUP inherits it | `TestReloadHashesPlaintextPassword` drives `diskConfigLoaders` and `doReload`; `make ze-test-pkg PKG=./cmd/ze/hub RUN=...` exit 0 |
| AC-3 | One warning naming the file, no secret | `TestLoadConfigWarnsPlaintextRemainsOnDisk` requires exactly 1 warning for 2 leaves and asserts neither secret appears |
| AC-4 | No `plaintext-` leaf survives | `TestLoadConfigHashesPlaintextPassword` plus `TestLoadConfigDropsEmptyPlaintextLeaf` for the empty-value case |
| AC-5 | The editor path still hashes, drops the plaintext, and serializes a file carrying none, for every input except an EMPTY `plaintext-` leaf, which it now drops instead of writing | `TestCommitPathPasswordHashingUnchanged` and `TestCommitPathDropsEmptyPlaintextLeaf`; `make ze-test-pkg PKG=./internal/component/cli` exit 0. The one changed input is in Deviations |
| AC-6 | The lab image accepts a `bgp` root | `docker run --rm netlab/ze:latest config validate /g/r3.conf` -> `configuration valid: /g/r3.conf`, exit 0 |
| AC-7 | `make ze-docker` no longer ships a BGP-less binary | Same command against `ze:latest`, exit 0 |
| AC-8 | `netlab create` exit 0 and the bind mount | `make ze-netlab-render-check` against netlab 26.08, 2026-08-14: 3/3 match |
| AC-9 | The render is valid ze config | Same run, 3/3 validate; `netlab-lab-profile.ci` re-validates `r3.conf` with no netlab |
| AC-10 | The show command output parses as JSON | `netlab-lab-profile` PASS at index 353; the `.ci` runs `json.loads` and requires a non-empty `peers` key |
| AC-11 | Every declared module has a key and a template | `contrib/netlab/ze.yml` maps 6 `daemon_config` keys; `ls contrib/netlab/ze/` shows all 5 module stubs |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `ze start <file>` with a `plaintext-` leaf | `test/plugin/config-plaintext-password-at-boot.ci` | Yes. Read: it starts a daemon, logs in as the config-declared user, and greps `SSH auth success`, which `internal/component/ssh/passwordauth.go` emits only inside the authenticated branch |
| SIGHUP with the same file | none (unit) | `TestReloadHashesPlaintextPassword` calls `diskConfigLoaders` itself, so it drives the loader the daemon installs rather than a hand-built copy. No `test/reload/*.ci`: recorded as a Review Gate NOTE |
| SSH login as the config-declared user | `test/plugin/netlab-lab-profile.ci` | Yes. It logs in as the user the RENDER declared, not a test-local user |
| `make ze-docker-lab` | `scripts/dev/lab_image_tags_test.py` | Yes, plus the built image validating a `bgp` config root |
| `netlab create` on the reference topology | `make ze-netlab-render-check` | Yes, against netlab 26.08 |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `netlab create` accepts a `ze.yml` mapping the daemon key plus one `.ignore` key per module; mapping only the daemon key fails with `Cannot find bgp configuration template` |
| A-2a | confirmed | A daemon started from the rendered config answered `ze cli -c "show version"` with exit 0 while logging `zefs power user unavailable` |
| A-2b | **broken, then fixed** | Hashing had one caller, the editor commit path. `LoadConfig` now calls the same function. Mistake Log row 1; Deviations records the corrected Design Insight |
| A-3 | **broken, then fixed** | The `--format` flag default re-rendered the pipe's JSON as YAML with exit 0. `renderCommandOutput` gives an explicit format pipe precedence. Mistake Log row 2 |
| A-4 | **broken** | The assumption was that netlab's own integration tests prove every declared feature. Nothing has been proven by netlab's suite: this machine has no containerlab, so `netlab up` and `netlab validate` never ran. One of the five declared modules (bgp) has evidence beyond rendering, and that evidence is a single daemon answering a show command. Mistake Log row 3; the gap is stated in Goal Validation, in Known Limitations, and on the four reader-facing surfaces including `contrib/netlab/README.md`. The five-module declaration is the owner's and was NOT narrowed |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| 1. New user-facing feature (`docs/features.md`) | The netlab row and the two-image Docker row; the SIGHUP sentence now matches `contrib/netlab/ze.yml` `features.initial.reload: false` | Yes |
| 2. Config syntax (`docs/guide/configuration.md`, `authentication.md`, `monitoring.md`, `config-reload.md`) | Checked against `LoadConfig` and `ApplyPasswordHashing`. No page still says hashing is commit-only. The oversize-password passage in `authentication.md` claimed an `slog.Warn` that does not exist and was replaced with the three real outcomes | Yes |
| 6. User guide page (`docs/guide/netlab.md`) | Its "what is proven, what is not" table states `netlab up` and `netlab validate` were never run | Yes |
| 10. Test infrastructure (`docs/functional-tests.md`) | The render check and its tier; the stale `internal/tmpfs/` heading corrected to `internal/test/tmpfs/` | Yes |
| 11. Daemon comparison (`docs/comparison.md`) | The ze cell reads `Yes (daemon, in-repo, unvalidated)`, so the table alone does not read at parity with BIRD's upstream, validated integration | Yes |
| 12. Internal architecture (`docs/DESIGN.md`) | Corrected: `ApplyPasswordHashing` is shared with the editor commit path, `refuseInvalidCustomSections` is not (`editor_commit.go` makes no validation call) | Yes |
| 16, 17. Source anchors and examples | Every new `<!-- source: -->` anchor resolves; `make ze-doc-test` exit 0 | Yes |
| 3, 4, 5, 7, 8, 9, 13, 14, 15 | N-A: no verb, RPC, plugin, wire format, SDK, RFC behavior, route metadata, metric or registry entry changed. `make ze-validate`'s registry checks report no new inventory drift | Yes |

## Core Insight

**Making ze reachable by an external tool found three defects, and each one was a
function with exactly one caller.** `ApplyPasswordHashing` was called by the editor
commit path and not by the file load path. `RejectMaskedBcryptLeaves`, its partner, is
still called by the commit path and by `ze config validate`, and this spec is what
finally gave it the third caller it needed. `(*tmpfs.Tmpfs).WriteTo` was called by every
test suite except the one that open-coded it.

The shape repeats: a transform is written for the path that first needed it, a second
path grows later, and nothing connects them. None of the three failed loudly. The
password leaf loaded and every login was refused; the tmpfs mode parsed and was ignored;
the format pipe ran and its output was re-rendered. All three returned success.

The lesson is about WHERE to look, not what to write. A one-caller transform in a
component that has more than one entry point is the defect, before anyone reads its
body. `make ze-validate` already reports an exported symbol with no cross-package
caller. The dangerous case is the opposite one: a symbol with exactly one caller inside
its own package, where a second entry point exists and does not use it.
