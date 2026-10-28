# Spec: privileged-tests-run-in-a-user-namespace

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Priority | P2 |
| Release reason | Release validation: about 390 kernel-touching tests skip on every unprivileged run, so ordinary package runs and agents' scoped evidence never execute them. Running them in a namespace of their own turns those skips into executed proof of shipped behavior. |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Approved | Thomas, 2026-10-28: "convert A and B". The group B mechanism (D-1) is the main thread's decision. The `/dev/ppp` scope addition (P2, P9) is Thomas's, relayed by the main thread on 2026-10-28 |
| Updated | 2026-10-28 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Ze's kernel-touching tests (links, routes, XFRM, nftables, conntrack, raw and
AF_PACKET sockets, MPLS) skip when the process lacks `CAP_NET_ADMIN` or
`CAP_NET_RAW`, and most sit behind the `integration` build tag for that reason
alone. On a developer host, in an agent's scoped run, and in the hosted CI unit
job they report `SKIP` or are not compiled, so the shipped kernel programming
they cover is proven only inside the QEMU guest.

`userns.Enter` (`internal/test/userns`) already re-executes one test as root
over a user and network namespace of its own, with no host privilege. Commit
88e01f1f6c converted the first file with it (the dhcpserver neighbour tests):
`Enter` first, a refused kernel operation becomes a failure rather than a skip,
and the file's tag dropped from `integration && linux` to `linux`.

This spec converts every remaining privilege-skipping test that a user
namespace can serve:

| Group | What it needs | Files | Mechanism |
|-------|---------------|-------|-----------|
| A | links, unnamed netns, raw and AF_PACKET sockets, `SO_BINDTODEVICE`, `IP_FREEBIND`, low ports, XFRM, nft, conntrack, MPLS sysctl | 67 | `userns.Enter` as it is today |
| B | a NAMED network namespace (`netns.NewNamed`, `ip netns add`), which needs a writable `/run/netns` | 61 | `userns.Enter` extended with a mount namespace and a private `/run/netns` (D-1) |
| D | `/dev/ppp` | 2 | host device grant through `./le setup install` (P2), then `Enter` |
| C | chown to another uid, an external freeRtr or FRR daemon | 4 | out of scope, stays privileged |

The goal: every group A, B and D test reports `PASS` under `go test -v` as an
unprivileged user on a Linux host, and the only privilege skips left in the tree
are group C's.

## Required Reading

### Architecture Docs
- [ ] `internal/test/userns/userns_linux.go` - the helper this spec extends
  → Decision: the child is selected by `-test.run ^<name>$`, so `Enter` stays the first statement of a TOP-LEVEL test, never of a helper or a subtest. A skipping helper is converted by adding `Enter` to every top-level test that reaches it.
  → Constraint: no `t.Parallel` in a test that calls `Enter` (the child re-runs one named test in a fresh process). The inventory below has none.
  → Constraint: `-test.gocoverdir` is passed to the child, so `./le rfc record` sees the code the child ran; the extension must keep that.
  → Constraint: the child maps only uid and gid 0 to the caller's ids, and Go writes `deny` to `/proc/<pid>/setgroups` for it.
- [ ] `docs/architecture/testing/qemu-integration.md` - "Build Tags" and the `userns.Enter` paragraph (~line 816)
  → Constraint: `integration && linux` files run only through `./le test qemu` (which supplies `-tags integration`); `linux` files run in the native unit groups too. A file that drops to `linux` therefore runs in ordinary unit runs and in the hosted CI unit job.
  → Decision: the paragraph is rewritten in P10 to describe the mount namespace and the private `/run/netns`.
- [ ] `docs/guide/developer-setup.md` ~449-520 - the `userns-unrestricted` and `kvm-access` setup checks
  → Decision: the new `ppp-access` check copies their shape: the same root route (`sudo -n`, terminal asks once, no terminal prints the commands), the `present`/`pending`/`missing`/`n/a` states, and the "group fixed at login" `pending` report.
- [ ] `docs/architecture/testing/ci-workflows.md` - line 5 (hosted runner grants root) and line 37 (the AppArmor userns tunable the unit job lifts)
  → Constraint: the hosted unit job already lifts `kernel.apparmor_restrict_unprivileged_userns`, so a file dropped to `linux` runs there in its namespace. The nightly `integration` job runs under `sudo`; `Enter` from a root parent still makes its own namespace (uid 0 maps to 0).
- [ ] `docs/contributing/testing.md` - the native test actions and the RFC approval route
  → Constraint: evidence runs through `./le job run label unit-pkg quiet command go test ...`, never a bare `go test`.
- [ ] `ai/rules/testing.md` - RFC-tagged tests, weakening, discrimination
  → Constraint: adding the `Enter` line changes a tagged unit's body; the edit hook locks it until an independent `ze-test-judge` approves and `./le rfc approve unit <package>.<TestName> reason "<verdict>"` records the row. The unit's sha changes, so its discrimination record is re-taken with `./le rfc record`.
- [ ] `plan/post-release/engineering/spec-lab-kernel-features-in-setup.md` - the neighbouring setup spec (ready, P3) that teaches setup which kernel modules and device nodes the Docker labs need
  → Decision: P2 owns only the host `/dev/ppp` permission and the user's group, which that spec does not cover. Where that spec's module registry has landed when P2 starts, P2 declares `ppp_generic` and `pppoe` there instead of writing its own `modules-load.d` drop-in.

**Key insights:**
- Verified on this host (Linux 7.2.9-ze, 2026-10-28): `unshare -rnm` then `mount -t tmpfs none /run/netns`, `ip netns add t1`, `ip netns exec t1 ip link add d0 type dummy` all succeed unprivileged, and the variant that mounts the tmpfs on `/run` and creates `/run/netns` succeeds too. The host `/run/netns` (root 0750) is untouched.
- Verified: supplementary groups survive the user namespace. `/etc/chatscripts` is `root:dip 2750`; inside `unshare -rn` the user (in `dip`) lists it, while `/proc/self/status` shows the unmapped groups as 65534 and `setgroups` reads `deny`.
- Kernel source (`drivers/net/ppp/ppp_generic.c`, `ppp_open`): the open checks `ns_capable(file->f_cred->user_ns, CAP_NET_ADMIN)`, the opener's own user namespace, which root in the child holds. `ppp_unattached_ioctl` (`PPPIOCNEWUNIT`) checks `ns_capable(net->user_ns, CAP_NET_ADMIN)`, the owning namespace of the child's netns, which is the child's. So once the file mode lets the user open `/dev/ppp`, the ioctls the D tests use are allowed in the namespace. Still to prove by experiment (P9).
- Ze's kernel builds every feature the inventory needs in (`/proc/config.gz`: `CONFIG_PPP=y`, `CONFIG_PPPOE=y`, `CONFIG_PPPOL2TP=y`, `CONFIG_L2TP=y`, `CONFIG_MPLS_ROUTING=y`, `CONFIG_XFRM_USER=y`, `CONFIG_NF_TABLES=y`, `CONFIG_DUMMY=y`, `CONFIG_VETH=y`, `CONFIG_MACVLAN=y`, `CONFIG_WIREGUARD=y`), so no module autoload question arises on Ze's own system.
- `/dev/ppp` here is `crw------- root root`; the user is already in `dip`.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/test/userns/userns_linux.go` - before the startup repair, `Enter` clones user, network and mount namespaces, maps uid/gid 0 to the caller, prepares private `/run/netns` and loopback, and checks the selected child's result. It incorrectly skips every startup error other than `*exec.ExitError`. D-9 replaces that classification with failure for every process error.
- [ ] `github.com/vishvananda/netns@v0.0.5` `NewNamed` - creates `/run/netns` when absent, unshares a new netns on the calling thread, creates `/run/netns/<name>` and bind-mounts `/proc/<pid>/task/<tid>/ns/net` onto it. In a user namespace without a private `/run/netns` the create fails with permission denied.
- [ ] `internal/plugins/dhcpserver/neighbor_integration_linux_test.go` at 88e01f1f6c - the conversion pattern.
- [ ] `internal/le/setup/system.go` (`kvmState`, `inKvmGroup`, `deviceOpenable`, `applyKvm`, `usernsSteps`, `privilegedStep`) and `internal/le/setup/setup.go` (`visitKvm`, `visitUserns`, the visit list at ~line 174) - the setup pattern P2 copies.
- [ ] Every file in the inventory, read for its skip and helper: the script that derived it followed each skipping helper to every caller file in its package.

**Behavior to preserve:**
- Every assertion of every converted test. A conversion adds the `Enter` guard and turns a privilege skip into a failure; it removes or loosens no assertion.
- Feature-absence skips that name a kernel feature or a module (for example `loadMPLSModules` in `fib/kernel`, "build a kernel with CONFIG_MPLS_ROUTING") stay skips: they say the kernel lacks the feature, not that the process lacks a privilege.
- Tests that must run WITHOUT privilege (for example `TestReplaceIPv6RPFUnprivileged` in `plugins/iface/netlink/rpf_linux_test.go`, skipped as root) get no `Enter`.
- `Enter`'s isolation, loopback setup, flag and coverage forwarding, and explicit selected-child skips for BFD and dhcpserver users. Startup refusal no longer skips: D-9 changes that contract.
- QEMU and the nightly `sudo` integration job keep running every converted file (a `linux` file is still compiled under `integration && linux`).

**Behavior to change:**
- `Enter` also gives the child a private mount namespace with a private `/run/netns`.
- `Enter` gains a non-Linux variant that runs the test in place.
- Every child-process error, including kernel namespace denial, fails the parent. The diagnostic preserves the error and conditionally names `./le setup install` and `userns-unrestricted` for namespace-policy failures (D-9).
- `./le setup` gains a `ppp-access` check and its install steps.
- Each inventory file: `Enter` in each top-level test that reaches a privilege skip, privilege skips become failures, build tag per the table.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `go test` (through `./le job run`, `./le test unit <group>`, `./le test qemu`) runs a converted top-level test in the parent process.
- `./le setup check` and `./le setup install` run the visit list in `internal/le/setup/setup.go`.

### Transformation Path
1. The parent's top-level test calls `userns.Enter`; outside the child it re-executes the test binary with `-test.run ^<name>$` in new user, network and mount namespaces.
2. In the child, `Enter` makes the mount tree recursively private, mounts a tmpfs on `/run/netns` (or, where the host has no `/run/netns`, on `/run`, then creates `/run/netns`), brings `lo` up, and returns true.
3. The test body runs as uid 0 over its own namespaces: named netns, links, sockets and XFRM state exist only there.
4. Any child-process error fails the parent with the error and captured output. On successful exit, the parent requires the selected test's result and reports PASS, FAIL, or its explicit SKIP.
5. For D tests, the open of `/dev/ppp` succeeds because the host node is `root:dip 0660` (P2) and `dip` survives the user namespace; the ioctls succeed because the child holds `CAP_NET_ADMIN` over its own netns.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Parent test process to child test process | re-exec of `os.Args[0]` with `-test.run`, `ZE_TEST_USERNS_CHILD=1`, the coverage directory | Yes: `TestEnterRunsTheTestAsRootWithLoopbackUp` and flag-forwarding smoke below |
| Child to kernel namespaces | `CLONE_NEWUSER`, `CLONE_NEWNET`, `CLONE_NEWNS`; tmpfs and bind mounts inside the child's mount namespace | Yes: `TestEnterGivesAPrivateNetnsDirectory` and `TestEnterServesIPNetns`, smoke below |
| `./le setup` to host root | the existing `runPrivileged` route (`sudo -n`, root, or printed commands) | No |

Current namespace smoke: session 644a4e78 scratch
`job-userns-smoke-b033df33.log`, ten tests PASS under `-race`. The private
directory test checks tmpfs magic, a device distinct from the parent's,
named-namespace create/lookup identity, and deletion.

### Integration Points
- `userns.Enter` - extended in place; every caller gets the mount namespace with no change of its own.
- `(*Setup).visitKvm` / `kvmState` / `applyKvm` - the model for `visitPPP`; the visit is added to the existing visit list in `setup.go`, the same way `visitKvm` and `visitUserns` are.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | tests keep calling the product code they call today; only the process they run in changes |
| No unintended coupling (components stay isolated) | Yes | `internal/test/userns` is imported only by tests |
| No duplicated functionality (extends existing, does not recreate) | Yes | extends `Enter`; no second namespace helper. Package-local helpers that unshare a netns themselves (`encapNetns`, `enterNamespace`, gtsm `newTestbed`) keep working nested inside `Enter`'s namespace |
| Zero-copy preserved where applicable (refs, not copies) | N-A | test infrastructure, no wire path |
| Registration over hardcoding, outbound | Yes | no core or shared package learns a test's name |
| Registration over hardcoding, inbound | Yes | the only list touched is the setup visit list, which already enumerates every host check by hand (`visitKvm`, `visitUserns`); `ppp-access` joins it as they did. Searched: `internal/le/setup/setup.go` visit list, `internal/le/setup/report.go` |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | An unprivileged user and mount namespace may mount a tmpfs on `/run/netns` and bind-mount a netns file onto a file in it | `unshare -rnm` experiment above; tmpfs is mountable in a user namespace | group B falls back to unnamed `netns.New()` per file (D-1) | `TestEnterGivesAPrivateNetnsDirectory`, current namespace smoke | validated |
| A-2 | A tmpfs mounted in the child never reaches the host | the child's mount namespace is private and owned by the child's user namespace | host `/run/netns` polluted | `TestEnterGivesAPrivateNetnsDirectory` compares the child's device against the parent's, current namespace smoke | validated |
| A-3 | Supplementary groups survive `Enter` with `setgroups` denied | `/etc/chatscripts` experiment above | D stays in C | P9 run of the D tests | validated by experiment |
| A-4 | Inside the namespace `/dev/ppp` opens and `PPPIOCNEWUNIT`, `PPPIOCGCHAN`, `PPPIOCUNBRIDGECHAN` behave as for root | `ppp_open` and `ppp_unattached_ioctl` capability checks quoted above | D stays in C, recorded with the errno | P9 `TestPADTRetainsUnitUntilOwnerCleanup` and `TestLACChannelBridge`: old session scratch `userns-P9/pppoeclient.log` and `l2tp.log`, both PASS | Open and unit reservation/reuse validated; bridge test checks a channel-number error but does not assert the exact ioctl errnos |
| A-5 | Each inventory test needs nothing a user namespace cannot give beyond `Enter` | read of every skip in the inventory | that test moves to the Reclassified table with its reason | its batch's run-once PASS | Broken for the observed cases in Reclassified during implementation; remaining batches still owe their runs |
| A-6 | A kernel feature built as a module autoloads from inside the namespace on a stock distribution kernel (hosted CI) | rtnetlink and nf_tables request modules on demand | the test meets its existing feature-absence skip, or fails naming the module | hosted CI unit job after P10 | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A converted test fails inside the namespace for a reason a skip hid (a real defect, or a sysctl the namespace starts with) | the batch's run-once shows FAIL | a product defect is fixed in its own commit (the 88e01f1f6c precedent); a namespace-specific need moves the test to its own row with the reason |
| R-2 | A test now in `linux` reddens the hosted CI unit job on a stock Ubuntu kernel (module, sysctl, AppArmor) | the CI unit job after the batch lands | name the red and cause in the commit body (pre-release rule); the per-file tag decision is revisited in a fix commit with the evidence |
| R-3 | Tagged units lose their discrimination record when the sha changes | `./le rfc check` reports a stale record | P3..P8 re-take every record in the same batch |
| R-4 | Two batches edit the same package | file lists overlap | batches are split by package directory; no file appears in two batches |
| R-5 | Re-exec per top-level test lengthens package runs | wall time of `./le test unit` groups | measured in each batch's evidence; nothing to cut unless it exceeds the five-minute command window, then the package's heavy file keeps `integration` with that reason |
| R-6 | The `dip` grant lets group members open `/dev/ppp` | review | `ppp_open` still requires `CAP_NET_ADMIN` in the opener's user namespace and `PPPIOCNEWUNIT` in the netns owner's, so membership creates PPP units only inside the user's own namespaces, never in the host netns |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Nothing a user sees: test infrastructure and a developer setup check. The worst case is red tests in ordinary runs, or a host `/dev/ppp` mode change |
| How is it reverted? | One commit per batch; each reverts alone. The `/dev/ppp` rule is one file under `/etc/udev/rules.d` |
| Who else touches this path? | Any session editing the inventory files; the RFC record tooling; `spec-lab-kernel-features-in-setup` in `internal/le/setup/` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `go test` of a top-level test | → | `userns.Enter` child with a private `/run/netns`, then `netns.NewNamed` | `TestEnterGivesAPrivateNetnsDirectory` (`internal/test/userns/userns_linux_test.go`) |
| `go test` of a group B package test | → | `withNetNS` (`netns.NewNamed`) inside `Enter` | `TestNetlinkIntegration_AddRoute` (`internal/plugins/fib/kernel/integration_linux_test.go`) |
| `go test` of a group A package test | → | `createDummyForTest` inside `Enter` | `TestListenDHCPLoopbackPXEOffer` (`internal/plugins/dhcpserver/socket_integration_linux_test.go`) |
| `./le setup check` | → | `(*Setup).visitPPP` | `TestPPPAccessTellsApartItsStates` (`internal/le/setup/setup_test.go`) |
| `go test` of a group D test | → | `/dev/ppp` open and `PPPIOCNEWUNIT` inside `Enter` | `TestPADTRetainsUnitUntilOwnerCleanup` (`internal/component/l2tp/pppoeclient/unit_reservation_integration_linux_test.go`) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A test calling `Enter`, as uid 1000, creates a named netns with `netns.NewNamed`, looks it up with `netns.GetFromName`, deletes it with `netns.DeleteNamed` | each call succeeds; the test reports `--- PASS` |
| AC-2 | The same child stats `/run/netns` | it is a tmpfs (statfs magic `TMPFS_MAGIC`) and its device differs from the device the parent saw for the host's `/run/netns`, passed to the child by the parent |
| AC-3 | A child runs `ip netns add <name>` and `ip netns exec <name> ip link add <dummy> type dummy` | both exit 0 |
| AC-4 | A child of a host that has no `/run/netns` | `/run/netns` exists in the child and AC-1 holds; proved by a test that points the helper's mount target at a missing directory under a temporary root |
| AC-5 | The kernel refuses the namespace, or any other startup error occurs | the parent fails, never skips, preserves the error, and names `./le setup install` and the `userns-unrestricted` check for a namespace-policy refusal. A selected child-test SKIP propagates only after a successful child process (D-9) |
| AC-6 | A mount step in the child fails | the test fails (not skips) naming the step and the errno |
| AC-7 | `GOOS=darwin` build of `internal/test/userns` and of `internal/component/traceroute/cmd` tests | both compile; on non-Linux `Enter` returns true at once and runs the test in place |
| AC-8 | Every group A, B and D file in the inventory, run once on this host as uid 1000 with `go test -v -run '<its tests>'` through `./le job run` (with `-tags integration` for a file that keeps the tag) | every top-level test that reached a privilege skip prints `--- PASS`; none prints `--- SKIP` except a feature-absence skip named in its batch's evidence |
| AC-9 | After P9, `git grep -nE 'Skipf?\(.*(CAP_NET_ADMIN\|CAP_NET_RAW\|CAP_SYS_ADMIN\|requires root\|operation not permitted)' -- '*_test.go'` | matches only the group C files and the out-of-scope list below |
| AC-10 | Every file whose "Build tag after" is `linux` | its first build line is `//go:build linux`, and `./le test unit <its group>` runs it |
| AC-11 | Every RFC-tagged unit a batch changed | its commit carries one `RFC-approved:` line for it, its discrimination record is re-taken, and `./le rfc check` reports no stale or missing record for its stem |
| AC-12 | `./le setup check` with `/dev/ppp` openable / absent / not openable with the user in `dip` in the group database / not openable and not in `dip` | `ppp-access` reports `present` / `present` with `n/a: no /dev/ppp` / `pending` naming the re-login or `sg dip -c '<command>'` / `missing` |
| AC-13 | `./le setup install` in the `missing` state with root reachable | it echoes and runs: write `/etc/udev/rules.d/60-ze-ppp.rules` holding exactly `KERNEL=="ppp", GROUP="dip", MODE="0660"`, `udevadm control --reload`, `chgrp dip /dev/ppp`, `chmod 0660 /dev/ppp`, and `usermod -aG dip <user>` when the user is not in `dip`; then reports `pending` until login or `present` |
| AC-14 | `./le setup install` with root out of reach (no terminal) | it prints those commands to run by hand, changes nothing, and reports `missing` |
| AC-15 | A host whose kernel has `ppp_generic` as an unloaded module | install loads it and makes it persistent (through the lab-kernel registry when landed, else `/etc/modules-load.d/ze-ppp.conf` naming `ppp_generic` and `pppoe`) |
| AC-16 | P9 experiment fails (A-4 broken) | the two D files stay in C, the spec records the failing call and errno, and AC-9 lists them with C |
| AC-17 | A test that needs more than `Enter` | it appears in the Reclassified table with the reason, and its file is not committed half-converted |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestEnterGivesAPrivateNetnsDirectory` | `internal/test/userns/userns_linux_test.go` | AC-1, AC-2; red before P1's extension (NewNamed fails with permission denied) | P1: written; red under an overlay that drops `CLONE_NEWNS` and the mount step (the device check fires: "/run/netns device 24 is the host's"; the tmpfs magic alone passed, because the host's /run is a tmpfs), green after |
| `TestEnterServesIPNetns` | `internal/test/userns/userns_linux_test.go` | AC-3 | P1: written; red under the overlay (`ip netns add` exit 1), green after |
| `TestEnterCreatesAMissingNetnsDirectory` | `internal/test/userns/userns_linux_test.go` | AC-4 | P1: written; red under the overlay (no `run/netns` created), green after |
| `TestEnterMountFailureNamesTheStep` | `internal/test/userns/userns_linux_test.go` | AC-6, through `mountNetnsDir` over a regular file: the error names `mount tmpfs on <target>` and wraps ENOTDIR; `childSetup` fails the test with it | P1: added; red under the overlay (nil error), green after |
| `TestEnterRunsTheTestAsRootWithLoopbackUp` | `internal/test/userns/userns_linux_test.go` | the existing contract (euid 0, lo up) the BFD and dhcpserver users rely on | P1: added; green before and after |
| `TestEnterRunsInPlaceOffLinux` | `internal/test/userns/userns_other_test.go` | AC-7 (compiled on darwin; run on Thomas's Mac) | P1: written; `GOOS=darwin go vet` of `internal/test/userns` and `internal/component/traceroute/cmd` exit 0; not run (owed on the Mac) |
| `TestEnterRefusalNamesTheFix` | `internal/test/userns/userns_linux_test.go` | AC-5 failure diagnostic preserves the error and names the conditional namespace-policy fix | PASS under `-race`; current session scratch `job-userns-smoke-b033df33.log` |
| `TestEnterRejectsAnInvalidExecutableName` | `internal/test/userns/userns_linux_test.go` | AC-5: a real re-exec gives Enter a NUL-containing executable name, rejected before clone, and requires FAIL rather than SKIP | Old classifier: FAIL because nested test skipped (`job-userns-startup-red-acee4c41.log`); repaired classifier: PASS under `-race` (`job-userns-startup-green-876efc0c.log`), both in session 644a4e78's scratch |
| `TestPPPAccessTellsApartItsStates` | `internal/le/setup/setup_test.go` | AC-12, with injected device and group probes like `TestKvmTellsApartTheThreeWaysItIsNotUsable` | |
| `TestPPPAccessInstallSteps` | `internal/le/setup/setup_test.go` | AC-13, AC-14, AC-15, over a recording shell | |
| every inventory test | the inventory files | AC-8: PASS in its namespace | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `/dev/ppp` mode | exactly 0660 | 0660 | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `./le setup check` run | the developer host | the developer sees `ppp-access` with its state and the fix command | |

No `.ci` applies: the user of this work is a developer running `go test` and `./le setup`; the run-once PASS of every inventory test through `./le job run` is the end-to-end proof.

### Interop Tests (Scope: protocol)
N-A: no wire-visible change; tests move into a namespace.

## Inventory

Derived 2026-10-28 by reading every test file matching a privilege skip
(`CAP_NET_ADMIN`, `CAP_NET_RAW`, `Geteuid() != 0`, `operation not permitted`,
`EPERM`, `netns.NewNamed`, `ip netns add`) and following each skipping helper
to every caller file in its package. "RFC tag lines" counts `RFC requirement:`
lines in the file: 31 files carry one, and at least 41 top-level tests carry one
in their doc comment (29 in group A, 12 in group B). The A and B files hold
about 390 top-level tests; each batch adds `Enter` only to those that reach a
privilege skip.

| # | File | Group | Batch | RFC tag lines | Build tag now | Build tag after |
|---|------|-------|-------|---------------|---------------|-----------------|
| 1 | `internal/chaos/peer/simulator_actions_iface_integration_linux_test.go` | B | P8 services | 0 | `integration && linux` | `linux` |
| 2 | `internal/component/bfd/transport/udp_device_linux_test.go` | A | P8 services | 0 | `linux` | unchanged |
| 3 | `internal/component/config/storage/open_test.go` | C | out of scope | 0 | none | unchanged |
| 4 | `internal/component/gtsm/netns_linux_test.go` | A | P8 services | 0 | `linux` | unchanged |
| 5 | `internal/component/gtsm/rfc5082_gtsm_linux_test.go` | A | P8 services | 9 | `linux` | unchanged |
| 6 | `internal/component/gtsm/rfc5082_listener_integration_linux_test.go` | A | P8 services | 5 | `integration && linux` | `linux` |
| 7 | `internal/component/iface/config_apply_resolve_integration_linux_test.go` | B | P4 iface-core | 0 | `integration && linux` | `linux` |
| 8 | `internal/component/iface/config_integration_linux_test.go` | B | P4 iface-core | 0 | `integration && linux` | `linux` |
| 9 | `internal/component/iface/device_owner_integration_linux_test.go` | B | P4 iface-core | 0 | `integration && linux` | `linux` |
| 10 | `internal/component/iface/dhcp_v6_link_flap_integration_linux_test.go` | A | P4 iface-core | 0 | `integration && linux` | `linux` |
| 11 | `internal/component/iface/dhcp_v6_selector_renew_integration_linux_test.go` | A | P4 iface-core | 0 | `integration && linux` | `linux` |
| 12 | `internal/component/iface/dispatch_resolve_integration_linux_test.go` | B | P4 iface-core | 0 | `integration && linux` | `linux` |
| 13 | `internal/component/iface/integration_helpers_linux_test.go` | B | P4 iface-core | 0 | `integration && linux` | `linux` |
| 14 | `internal/component/iface/link_flap_integration_linux_test.go` | B | P4 iface-core | 0 | `integration && linux` | `linux` |
| 15 | `internal/component/iface/manage_integration_linux_test.go` | B | P4 iface-core | 0 | `integration && linux` | `linux` |
| 16 | `internal/component/iface/migrate_integration_linux_test.go` | B | P4 iface-core | 0 | `integration && linux` | `linux` |
| 17 | `internal/component/iface/mirror_integration_linux_test.go` | B | P4 iface-core | 0 | `integration && linux` | `linux` |
| 18 | `internal/component/iface/mirror_packet_integration_linux_test.go` | B | P4 iface-core | 0 | `integration && linux` | `linux` |
| 19 | `internal/component/iface/monitor_integration_linux_test.go` | B | P4 iface-core | 0 | `integration && linux` | `linux` |
| 20 | `internal/component/iface/reconcile_learned_integration_linux_test.go` | B | P4 iface-core | 0 | `integration && linux` | `linux` |
| 21 | `internal/component/iface/registry_integration_linux_test.go` | B | P4 iface-core | 0 | `integration && linux` | `linux` |
| 22 | `internal/component/iface/resolve_integration_linux_test.go` | B | P4 iface-core | 0 | `integration && linux` | `linux` |
| 23 | `internal/component/iface/rfc9915_dhcp_v6_dad_integration_linux_test.go` | A | P4 iface-core | 2 | `integration && linux` | `linux` |
| 24 | `internal/component/iface/rfc9915_dhcp_v6_link_move_integration_linux_test.go` | A | P4 iface-core | 2 | `integration && linux` | `linux` |
| 25 | `internal/component/iface/slaac_integration_linux_test.go` | B | P4 iface-core | 0 | `integration && linux` | `linux` |
| 26 | `internal/component/iface/unit_integration_linux_test.go` | B | P4 iface-core | 0 | `integration && linux` | `linux` |
| 26a | `internal/component/iface/netlink_integration_import_test.go` | B | P4 iface-core | 0 | `integration && linux` | `linux`; backend-registration dependency, landed |
| 27 | `internal/component/ike/dataplane/encap_dualform_integration_linux_test.go` | A | P3 ike | 0 | `integration && linux` | `linux` |
| 28 | `internal/component/ike/dataplane/encap_formchange_integration_linux_test.go` | A | P3 ike | 0 | `integration && linux` | `linux` |
| 29 | `internal/component/ike/dataplane/encap_hybrid_integration_linux_test.go` | A | P3 ike | 0 | `integration && linux` | `linux` |
| 30 | `internal/component/ike/dataplane/encap_integration_linux_test.go` | A | P3 ike | 0 | `integration && linux` | `linux` |
| 31 | `internal/component/ike/dataplane/espform_linux_test.go` | A | P3 ike | 0 | `linux` | unchanged |
| 32 | `internal/component/ike/dataplane/kernelcap_linux_test.go` | A | P3 ike | 0 | `linux` | unchanged |
| 33 | `internal/component/ike/dataplane/rfc4301_fragment_integration_linux_test.go` | A | P3 ike | 1 | `integration && linux` | `linux` |
| 34 | `internal/component/ike/dataplane/xfrm_bypass_integration_linux_test.go` | A | P3 ike | 0 | `integration && linux` | `linux` |
| 35 | `internal/component/ike/dataplane/xfrm_integration_linux_test.go` | A | P3 ike | 0 | `integration && linux` | `linux` |
| 36 | `internal/component/ike/dataplane/xfrm_migrate_integration_linux_test.go` | A | P3 ike | 0 | `integration && linux` | `linux` |
| 36a | `internal/component/ike/dataplane/xfrm_forward_migrate_integration_linux_test.go` | A | P3 ike | 0 | `integration && linux` | `linux`; calls the converted migration fixture, landed in `ad1c2c3884` |
| 37 | `internal/component/ike/dataplane/xfrm_policy_owner_integration_linux_test.go` | A | P3 ike | 0 | `integration && linux` | `linux` |
| 38 | `internal/component/ike/dataplane/xfrm_readback_integration_linux_test.go` | A | P3 ike | 0 | `integration && linux` | `linux` |
| 39 | `internal/component/ike/dataplane/xfrm_rekey_policy_integration_linux_test.go` | A | P3 ike | 0 | `integration && linux` | `linux` |
| 40 | `internal/component/ike/dataplane/xfrm_reqid_overlap_integration_linux_test.go` | A | P3 ike | 0 | `integration && linux` | `linux` |
| 41 | `internal/component/ike/dataplane/xfrm_reqid_resolution_integration_linux_test.go` | A | P3 ike | 0 | `integration && linux` | `linux` |
| 42 | `internal/component/ike/dataplane/xfrm_teardown_integration_linux_test.go` | A | P3 ike | 0 | `integration && linux` | `linux` |
| 43 | `internal/component/ike/dataplane/xfrm_transport_integration_linux_test.go` | A | P3 ike | 0 | `integration && linux` | `linux` |
| 44 | `internal/component/ike/engine/child_policy_delete_integration_linux_test.go` | A | P3 ike | 0 | `integration && linux` | `linux` |
| 45 | `internal/component/ike/engine/rfc3948_encap_port_integration_linux_test.go` | A | P3 ike | 0 | `integration && linux` | `linux` |
| 46 | `internal/component/ike/transport/udp_df_integration_linux_test.go` | B | P3 ike | 0 | `integration && linux` | `integration && linux`: two capture tests need initial-user-namespace `SO_RCVBUFFORCE`; error-queue test converted |
| 47 | `internal/component/l2tp/bridge_integration_linux_test.go` | D | P9 ppp | 0 | `integration && linux` | `integration && linux`: needs the host `/dev/ppp` grant from P2 (D-8) |
| 48 | `internal/component/l2tp/pppoeclient/dialer_integration_linux_test.go` | A | P8 services | 0 | `integration && linux` | `linux` |
| 49 | `internal/component/l2tp/pppoeclient/unit_reservation_integration_linux_test.go` | D | P9 ppp | 0 | `integration && linux` | `integration && linux`: needs the host `/dev/ppp` grant from P2 (D-8) |
| 50 | `internal/component/mtu/cmd/search_integration_linux_test.go` | B | P8 services | 0 | `integration && linux` | `linux` |
| 51 | `internal/component/traceroute/cmd/resolve_test.go` | A | P8 services | 0 | none | none: kept, through the non-Linux `Enter` (D-4) |
| 52 | `internal/component/traceroute/cmd/traceroute_test.go` | A | P8 services | 0 | none | none: kept, through the non-Linux `Enter` (D-4) |
| 53 | `internal/core/dnsserver/freebind_integration_linux_test.go` | A | P8 services | 0 | `integration && linux` | `linux` |
| 54 | `internal/core/probe/errqueue_integration_linux_test.go` | B | P8 services | 0 | `integration && linux` | `linux` |
| 55 | `internal/core/probe/privilege_integration_linux_test.go` | A | P8 services | 0 | `integration && linux` | `linux` |
| 56 | `internal/core/probe/socket_linux_test.go` | A | P8 services | 0 | `linux` | unchanged |
| 57 | `internal/core/routewatch/integration_linux_test.go` | B | P8 services | 0 | `integration && linux` | `linux` |
| 58 | `internal/plugins/as112/freebind_integration_linux_test.go` | A | P8 services | 0 | `integration && linux` | `linux` |
| 59 | `internal/plugins/copp/copp_integration_linux_test.go` | A | P6 dataplane | 0 | `integration && linux` | `linux` |
| 60 | `internal/plugins/dhcpserver/socket_integration_linux_test.go` | A | P8 services | 0 | `integration && linux` | `linux` |
| 61 | `internal/plugins/fib/kernel/bgplu_mpls_integration_linux_test.go` | B | P6 dataplane | 0 | `integration && linux` | `linux` |
| 62 | `internal/plugins/fib/kernel/crossfamily_integration_linux_test.go` | B | P6 dataplane | 0 | `integration && linux` | `linux` |
| 63 | `internal/plugins/fib/kernel/ecmp_labels_integration_linux_test.go` | B | P6 dataplane | 0 | `integration && linux` | `integration && linux`: converted, but uses helpers in the retained MPLS MTU file |
| 64 | `internal/plugins/fib/kernel/fibwithhold_integration_linux_test.go` | B | P6 dataplane | 0 | `integration && linux` | `linux` |
| 65 | `internal/plugins/fib/kernel/integration_linux_test.go` | B | P6 dataplane | 0 | `integration && linux` | `linux` |
| 66 | `internal/plugins/fib/kernel/mplscontext_integration_linux_test.go` | B | P6 dataplane | 0 | `integration && linux` | `linux` |
| 67 | `internal/plugins/fib/kernel/mplsentry_integration_linux_test.go` | B | P6 dataplane | 0 | `integration && linux` | `linux`; its best-effort `modprobe` and feature skip stay (Behavior to preserve) |
| 68 | `internal/plugins/fib/kernel/mplsframe_integration_linux_test.go` | A | P6 dataplane | 0 | `integration && linux` | `linux` |
| 69 | `internal/plugins/fib/kernel/mplsipmtuprobe_integration_linux_test.go` | B | P6 dataplane | 0 | `integration && linux` | `integration && linux`: reclassified below |
| 70 | `internal/plugins/fib/kernel/mplsmtu_integration_linux_test.go` | B | P6 dataplane | 0 | `integration && linux` | `integration && linux`: PathMTU and TransitIPv4Options converted; FragmentProgress reclassified below |
| 71 | `internal/plugins/fib/kernel/mplsowner_integration_linux_test.go` | B | P6 dataplane | 0 | `integration && linux` | `linux` |
| 72 | `internal/plugins/fib/kernel/mplstransitmtu_integration_linux_test.go` | B | P6 dataplane | 0 | `integration && linux` | `integration && linux`: reclassified below |
| 73 | `internal/plugins/fib/kernel/ownership_integration_linux_test.go` | B | P6 dataplane | 0 | `integration && linux` | `linux` |
| 74 | `internal/plugins/fib/kernel/rfc1195_onlink_integration_linux_test.go` | B | P6 dataplane | 4 | `integration && linux` | `linux` |
| 75 | `internal/plugins/fib/kernel/richroute_discard_integration_linux_test.go` | B | P6 dataplane | 0 | `integration && linux` | `linux` |
| 76 | `internal/plugins/fib/kernel/srv6_integration_linux_test.go` | B | P6 dataplane | 0 | `integration && linux` | `integration && linux`: reclassified below |
| 77 | `internal/plugins/firewall/nft/counter_integration_linux_test.go` | B | P6 dataplane | 0 | `integration && linux` | `linux` |
| 78 | `internal/plugins/firewall/nft/integration_linux_test.go` | B | P6 dataplane | 0 | `integration && linux` | `linux` |
| 79 | `internal/plugins/flowexport/conntrack/reader_integration_linux_test.go` | A | P6 dataplane | 0 | `integration && linux` | `linux` |
| 80 | `internal/plugins/flowexport/producer_integration_linux_test.go` | A | P6 dataplane | 0 | `integration && linux` | `integration && linux`: converted; explicit Ze executable prerequisite retained (D-5), custom flag now forwarded |
| 81 | `internal/plugins/flowspec-firewall/rfc8955_packet_zero_integration_linux_test.go` | A | P6 dataplane | 2 | `integration && linux` | `linux` |
| 82 | `internal/plugins/flowspec-firewall/rfc8955_selected_integration_linux_test.go` | A | P6 dataplane | 4 | `integration && linux` | `integration && linux`: whole privileged path retained; nftables payload writes require an initial-userns-owned netns |
| 83 | `internal/plugins/iface/dhcp/dhcp_lease_owner_integration_linux_test.go` | A | P5 iface-plugins | 0 | `integration && linux` | `linux` |
| 84 | `internal/plugins/iface/dhcp/dhcp_shortframe_integration_linux_test.go` | A | P5 iface-plugins | 0 | `integration && linux` | `linux` |
| 85 | `internal/plugins/iface/dhcp/rfc2131_dhcp_flags_integration_linux_test.go` | A | P5 iface-plugins | 3 | `integration && linux` | `linux` |
| 86 | `internal/plugins/iface/dhcp/rfc2131_dhcp_lease_maintenance_integration_linux_test.go` | A | P5 iface-plugins | 16 | `integration && linux` | `linux` |
| 87 | `internal/plugins/iface/dhcp/rfc2131_dhcp_multiiface_integration_linux_test.go` | A | P5 iface-plugins | 1 | `integration && linux` | `linux` |
| 88 | `internal/plugins/iface/netlink/counter_integration_linux_test.go` | B | P5 iface-plugins | 0 | `integration && linux` | `linux` |
| 89 | `internal/plugins/iface/netlink/macvlan_integration_linux_test.go` | B | P5 iface-plugins | 0 | `integration && linux` | `linux` |
| 90 | `internal/plugins/iface/netlink/mirror_integration_linux_test.go` | B | P5 iface-plugins | 0 | `integration && linux` | `linux` |
| 91 | `internal/plugins/iface/netlink/route_integration_linux_test.go` | B | P5 iface-plugins | 0 | `integration && linux` | `linux` |
| 92 | `internal/plugins/iface/netlink/rpf_integration_linux_test.go` | A | P5 iface-plugins | 0 | `integration && linux` | `linux` |
| 93 | `internal/plugins/iface/netlink/rpf_linux_test.go` | A | P5 iface-plugins | 0 | `linux` | unchanged; `TestReplaceIPv6RPFUnprivileged` gets no `Enter` |
| 94 | `internal/plugins/iface/netlink/tunnel_linux_test.go` | B | P5 iface-plugins | 0 | `integration && linux` | `linux` |
| 95 | `internal/plugins/iface/netlink/tunnel_rfc_integration_linux_test.go` | B | P5 iface-plugins | 4 | `integration && linux` | `linux` |
| 96 | `internal/plugins/iface/netlink/vlanqos_integration_linux_test.go` | B | P5 iface-plugins | 0 | `integration && linux` | `linux` |
| 97 | `internal/plugins/iface/netlink/vlanqos_replace_integration_linux_test.go` | B | P5 iface-plugins | 0 | `integration && linux` | `linux` |
| 98 | `internal/plugins/iface/netlink/vlanqoslab_integration_linux_test.go` | B | P5 iface-plugins | 0 | `integration && linux` | `integration && linux`: execs the `nft` binary, which `./le setup` does not install |
| 99 | `internal/plugins/iface/netlink/wireguard_linux_test.go` | B | P5 iface-plugins | 0 | `integration && linux` | `linux` |
| 100 | `internal/plugins/iface/ra/ra_integration_linux_test.go` | B | P5 iface-plugins | 0 | `integration && linux` | `linux` |
| 101 | `internal/plugins/iface/ra/rfc4861_ra_integration_linux_test.go` | B | P5 iface-plugins | 2 | `integration && linux` | `linux` |
| 102 | `internal/plugins/isis/adjacency_integration_linux_test.go` | B | P7 routing | 0 | `integration && linux` | `linux` |
| 103 | `internal/plugins/isis/frr_interop_integration_linux_test.go` | C | out of scope | 0 | `integration && linux` | unchanged |
| 104 | `internal/plugins/isis/transport/rfc1195_ish_integration_linux_test.go` | B | P7 routing | 1 | `integration && linux` | `linux` |
| 105 | `internal/plugins/isis/transport/transport_integration_linux_test.go` | B | P7 routing | 0 | `integration && linux` | `linux` |
| 106 | `internal/plugins/ldp/resolve_integration_linux_test.go` | B | P7 routing | 0 | `integration && linux` | `linux` |
| 107 | `internal/plugins/ospf/rfc4302_ipsec_integration_linux_test.go` | A | P7 routing | 2 | `integration && linux` | `linux` |
| 108 | `internal/plugins/ospf/rfc4302_ipsec_pmtu_integration_linux_test.go` | A | P7 routing | 2 | `integration && linux` | `linux` |
| 109 | `internal/plugins/ospf/transport/rfc2328_transport_integration_linux_test.go` | B | P7 routing | 2 | `integration && linux` | `linux` |
| 110 | `internal/plugins/ospf/transport/rfc2328_unnumbered_source_linux_test.go` | B | P7 routing | 2 | `integration && linux` | `linux` |
| 111 | `internal/plugins/ospf/v3/transport/transport_integration_linux_test.go` | B | P7 routing | 0 | `integration && linux` | `linux` |
| 112 | `internal/plugins/policyroute/rules_integration_linux_test.go` | A | P6 dataplane | 0 | `integration && linux` | `linux` |
| 113 | `internal/plugins/rsvpte/freertr_interop_integration_linux_test.go` | C | out of scope | 0 | `integration && linux` | unchanged |
| 114 | `internal/plugins/rsvpte/producer_integration_linux_test.go` | B | P7 routing | 0 | `integration && linux` | `linux` (execs `ip`, which `./le setup` installs as `iproute2`) |
| 115 | `internal/plugins/rsvpte/rfc2205_transport_integration_linux_test.go` | A | P7 routing | 7 | `integration && linux` | `linux` |
| 116 | `internal/plugins/static/resolve_integration_linux_test.go` | B | P7 routing | 0 | `integration && linux` | `linux` |
| 117 | `internal/plugins/tftpserver/rfc1350_socket_integration_linux_test.go` | A | P8 services | 1 | `integration && linux` | `linux` |
| 118 | `internal/plugins/traffic/netlink/cs6_integration_linux_test.go` | B | P6 dataplane | 0 | `integration && linux` | `linux` |
| 119 | `internal/plugins/traffic/netlink/integration_linux_test.go` | B | P6 dataplane | 0 | `integration && linux` | `linux` |
| 120 | `internal/plugins/trafficusage/attach_integration_linux_test.go` | A | P6 dataplane | 0 | `integration && linux` | `integration && linux`: original privileged path retained after memlock EPERM in the Enter experiment |
| 121 | `internal/plugins/vrrp/backup_multicast_integration_linux_test.go` | A | P7 routing | 0 | `integration && linux` | `linux` |
| 122 | `internal/plugins/vrrp/nd_nonowner_integration_linux_test.go` | A | P7 routing | 8 | `integration && linux` | `linux` |
| 123 | `internal/plugins/vrrp/owner_answer_integration_linux_test.go` | A | P7 routing | 20 | `integration && linux` | `linux` |
| 124 | `internal/plugins/vrrp/redirect_integration_linux_test.go` | A | P7 routing | 8 | `integration && linux` | `linux` |
| 125 | `internal/plugins/vrrp/redirect_v6_integration_linux_test.go` | A | P7 routing | 4 | `integration && linux` | `linux` |
| 126 | `internal/plugins/vrrp/rfc792_gateway_icmp_integration_linux_test.go` | A | P7 routing | 17 | `integration && linux` | `linux` |
| 127 | `internal/plugins/vrrp/rfc9568_nd_router_flag_integration_linux_test.go` | A | P7 routing | 2 | `integration && linux` | `linux` |
| 128 | `internal/plugins/vrrp/rfc9568_vmac_link_local_integration_linux_test.go` | A | P7 routing | 8 | `integration && linux` | `linux` |
| 129 | `internal/plugins/vrrp/transport/private_macvlan_rx_integration_linux_test.go` | A | P7 routing | 0 | `integration && linux` | `linux` |
| 130 | `internal/plugins/vrrp/transport/rfc_rx_iface_verdict_linux_test.go` | A | P7 routing | 4 | `integration && linux` | `linux` |
| 131 | `internal/plugins/vrrp/transport/rfc_tx_verdict_linux_test.go` | A | P7 routing | 15 | `integration && linux` | `linux` |
| 132 | `internal/plugins/vrrp/transport/transport_integration_linux_test.go` | A | P7 routing | 2 | `integration && linux` | `linux` |
| 133 | `internal/plugins/vrrp/vmac_state_integration_linux_test.go` | A | P7 routing | 30 | `integration && linux` | `linux` |
| 134 | `pkg/zefs/check_tree_unix_test.go` | C | out of scope | 0 | `linux` or `darwin` | unchanged |

Counts: A 68 files (25 tagged), B 62 (6 tagged), D 2, C 4. Rows 26a and
36a add dependencies found during implementation without renumbering the
original inventory.

### Group C, out of scope
| File | Why it stays privileged |
|------|-------------------------|
| `internal/component/config/storage/open_test.go` | chowns to another uid, which a one-uid map cannot represent |
| `pkg/zefs/check_tree_unix_test.go` | the same |
| `internal/plugins/rsvpte/freertr_interop_integration_linux_test.go` | drives an external freeRtr process |
| `internal/plugins/isis/frr_interop_integration_linux_test.go` | drives external FRR daemons (zebra, isisd), skipped when FRR is absent; reclassified from the survey's group A/B by this spec |

### Not privilege skips (matched the search, no conversion)
| File | Why |
|------|-----|
| `cmd/ze/hub/mcp_keyperm_test.go`, `internal/component/cli/sshclient/client_test.go`, `internal/component/host/dmi_linux_test.go`, `internal/le/ai/sync/actions_test.go`, `internal/le/doc/consistency/consistency_test.go`, `internal/le/doc/yangcontract/contract_test.go`, `internal/le/doc/yangcontract/drift_test.go`, `internal/le/rfc/rename_test.go`, `pkg/zefs/store_test.go` | skip AS root (a chmod root bypasses), the opposite case |
| `internal/component/config/archive/archive_test.go` | runs one branch only when not root; no skip |
| `internal/core/reboot/reboot_linux_test.go` | proves the non-root guard refuses; must stay unprivileged |
| `internal/component/ike/engine/child_test.go`, `internal/component/ike/engine/rfc7296_rekey_test.go` | `EPERM` is a table value, not a skip |

### Reclassified during implementation
| File | Test | From | To | Reason |
|------|------|------|----|--------|
| `internal/plugins/fib/kernel/mplsipmtuprobe_integration_linux_test.go` | `TestMPLSIntegration_ProbeAnswersFromAFreshNamespace` | B | privileged integration | The prior userns trial reported `bind /proc/sys: invalid argument` from the MPLS IP MTU probe; old session scratch `job-p6-fib2-52444b02.log`. Effective-capability skip retained, independently approved. |
| `internal/plugins/fib/kernel/mplstransitmtu_integration_linux_test.go` | `TestMPLSIntegration_TransitPathMTUFollowsTheProbe` | B | privileged integration | The same probe returned unknown after its `/proc/sys` bind failed in the prior userns trial; same log. Effective-capability skip retained, independently approved. |
| `internal/plugins/fib/kernel/mplsmtu_integration_linux_test.go` | `TestMPLSIntegration_FragmentProgress` | B | privileged integration | The probe cannot establish its prerequisite in the observed userns trial. PathMTU and TransitIPv4Options use Enter and pass; this case retains its capability guard and all packet assertions. |
| `internal/plugins/fib/kernel/srv6_integration_linux_test.go` | `TestFIBServiceSIDEncapsulationOnWire` | B | privileged integration | The prior userns trial timed out waiting for an SRv6 packet; cause remains untraced, not a proven universal userns restriction. The capability guard preserves the existing privileged run. |
| `internal/plugins/fib/kernel/ecmp_labels_integration_linux_test.go` | `TestFIBECMPMemberLabelsOnWire` | B | converted; integration tag retained | Uses helpers from the retained MPLS MTU file. Current `-race` run passes in Enter. |
| `internal/component/ike/transport/udp_df_integration_linux_test.go` | `TestDFSendThenPlainCopyOnTheWire`, `TestPlainSendNeverLeavesUnderTheProbeOption` | B | privileged integration | Both failed at `SO_RCVBUFFORCE` with EPERM under Enter. Linux checks initial-user-namespace `CAP_NET_ADMIN` for this socket option. Original privileged entry restored after independent judgment; 8 MiB capture buffer and all packet counts/assertions unchanged. |
| `internal/plugins/trafficusage/attach_integration_linux_test.go` | `TestAttachTCX_CountsTraffic`, `TestMetricsScrape` | A | privileged integration | Enter-only overlay stopped at the existing `requireBPF` memlock prerequisite, EPERM; neither test reached TCX attachment/accounting. Original code retained wholly. Enter supplies no delegated BPF token or initial-user-namespace BPF authority. |
| `internal/plugins/flowexport/producer_integration_linux_test.go` | `TestFlowExportDaemonCounterGeneration` | A | converted; integration tag retained | Needs an explicitly supplied Ze executable. Parent built that executable and observed the real daemon's counter-generation workflow PASS through Enter with the custom flag. |
| `internal/plugins/flowspec-firewall/rfc8955_selected_integration_linux_test.go` | `TestSelectedFlowSpecKernelPacketSemantics` | A | privileged integration | Linux 7.2.9 `nft_payload_set_init` returns EPERM when `ctx->net->user_ns != &init_user_ns`; Ze's `setDSCPv4` emits PayloadWrite. Diagnostic `job-userns-flowspec-diagnostic-03ec1f4f.log` exposes the refused marking table before the delivery timeout. Independent judge approved restoration of the whole original privileged path; all four RFC claims and packet assertions are unchanged. The separate zero-rate test remains converted and has both native discrimination records. |

Current FIB evidence: session 644a4e78 scratch
`job-userns-batch-smoke-9d2dbdbc.log` records PASS for ECMP labels, PathMTU,
and TransitIPv4Options, and the four named capability skips above. The
independent judge approved the retained guards without changing their packet
inputs or assertions. This is partial P6 evidence, not completion of P6.

## Files to Modify
- `internal/test/userns/userns_linux.go` - mount namespace, private `/run/netns`, every process error fails, conditional namespace-policy fix guidance
- `internal/le/setup/system.go`, `internal/le/setup/setup.go` - `ppp-access` state, visit and install steps
- every group A, B and D file in the inventory - `Enter`, privilege skip to failure, build tag
- `docs/architecture/testing/qemu-integration.md`, `docs/contributing/testing.md`, `docs/guide/developer-setup.md`, `docs/architecture/testing/ci-workflows.md` - P10

## Files to Create
- `internal/test/userns/userns_linux_test.go` - the package's own tests
- `internal/test/userns/userns_other.go` and `internal/test/userns/userns_other_test.go` - the non-Linux `Enter` (D-4)

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | no config |
| YANG validation constraints | N-A | no config |
| YANG custom validators | N-A | no config |
| CLI commands/flags | No | `./le setup` keeps its verbs; one more check row |
| CLI grammar (keyword before value) | N-A | no new grammar |
| Editor autocomplete | N-A | no YANG |
| Functional test for new RPC/API | N-A | no RPC |
| Pipe completeness | N-A | no CLI output |
| Env var registration | N-A | `ZE_TEST_USERNS_CHILD` is a test-binary marker, already in place, not a `ze.*` setting |
| Doctor check for runtime dependencies | Yes | the runtime dependency is a developer-host one, so its check is `./le setup check` `ppp-access` (`internal/le/setup/`), not `ze doctor` |
| Prometheus counters/metrics | N-A | none |
| BGP family surface | N-A | none |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | test evidence only; no `features/` entry changes level |
| 2 | Config syntax changed? | No | none |
| 3 | CLI command added/changed? | No | none |
| 4 | API/RPC added/changed? | No | none |
| 5 | Plugin added/changed? | No | none |
| 6 | Has a user guide page? | Yes | `docs/guide/developer-setup.md`: the `ppp-access` paragraph (P2) |
| 7 | Wire format changed? | No | none |
| 8 | Plugin SDK/protocol changed? | No | none |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | no summary text changes; tagged units get fresh records (AC-11) |
| 10 | Test infrastructure changed? | Yes | `docs/contributing/testing.md` (when to call `Enter`), `docs/architecture/testing/qemu-integration.md` (userns paragraph, Build Tags table) |
| 11 | Affects daemon comparison? | No | none |
| 12 | Internal architecture changed? | No | `docs/architecture/core-design.md`, declared by the `// Design:` headers of `internal/le/setup/setup.go`, `internal/le/setup/system.go` and `internal/plugins/fib/kernel/crossfamily_integration_linux_test.go`, is unaffected: one more setup check, and tests running in a namespace, change no core design |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | No | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | none |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | derived with `./le spec citation anchors spec plan/release/validation/spec-privileged-tests-run-in-a-user-namespace.md` in each batch; the `// Design:` docs of converted files are named in that batch's commit, and each batch fixes a page its conversion makes wrong (for example a page saying a suite needs root) in the same commit |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/architecture/testing/ci-workflows.md` line 37 lists the packages that build their own namespace; P10 updates it |

## Implementation Steps

Each phase runs in its own agent. P1 and P2 touch disjoint files and may run
together. P3..P8 start after P1 lands, at most four at once, one package set
each, never sharing a file. P9 starts after P1 and P2 land. P10 runs last.

Every batch (P3..P9) owes, in order:
1. List the batch's top-level tests that reach a privilege skip, and its RFC-tagged units among them.
2. One independent `ze-test-judge` pass for all the batch's tagged units, briefed with the planned edit per unit (the `Enter` guard first, each privilege skip turned into a failure, nothing else). Record each approval with `./le rfc approve unit <package>.<TestName> reason "<verdict>"` before the edit. A refusal stands: that unit moves to the Reclassified table.
3. The conversion: `Enter` as the first statement of each listed test, which returns at once when `Enter` answers false; privilege skips in the reached helpers become failures naming the operation and errno; the build tag per the inventory; `// Design:` and comments that say "skips without CAP_NET_ADMIN" rewritten.
4. Run every listed test once with `-v` as uid 1000 through `./le job run label unit-pkg quiet command go test -v -count=1 -run '<regex>' <package>` (plus `-tags integration` for a file that keeps the tag) and keep the output in the session scratch; every one shows `--- PASS`.
5. Re-take each changed tagged unit's discrimination record with `./le rfc record`.
6. Scoped lint of the batch's packages, then commit the batch through `./le commit create` with the `RFC-approved:` lines, naming any red it knows of.

1. **Phase P1: `Enter` mount namespace (wiring first)** - write `TestEnterGivesAPrivateNetnsDirectory`, `TestEnterServesIPNetns`, `TestEnterCreatesAMissingNetnsDirectory`, `TestEnterRefusalNamesTheFix`; show them red; then extend `Enter` (`CLONE_NEWNS`; in the child: recursive private propagation on `/`, tmpfs on `/run/netns`, or tmpfs on `/run` plus `/run/netns` when absent; `lo` up); add the non-Linux `Enter` and its test.
   - Tests: the four above, `TestEnterRunsInPlaceOffLinux` compiled for darwin, and the existing BFD and dhcpserver `Enter` users rerun.
   - Startup repair (D-9): `TestEnterRejectsAnInvalidExecutableName` must fail under the old startup classifier and pass after removing it. Keep the selected-child SKIP and missing-result guards.
   - Files: `internal/test/userns/*`
   - Verify: red, then green; the BFD and dhcpserver users still PASS. If A-1 breaks, record why in Design Insights and switch group B to unnamed `netns.New()` per file (D-1 fallback).
2. **Phase P2: `ppp-access` setup check** - state, visit, install steps, manual-fix notes, tests.
   - Tests: `TestPPPAccessTellsApartItsStates`, `TestPPPAccessInstallSteps`
   - Files: `internal/le/setup/system.go`, `internal/le/setup/setup.go`, `internal/le/setup/setup_test.go`, `docs/guide/developer-setup.md`
   - Verify: unit tests; then `./le setup check` on this host prints `ppp-access` `missing` (it is root 0600 today). Running `./le setup install` changes the host and is Thomas's to run: the phase hands him the command.
3. **Phase P3: ike** - `internal/component/ike/dataplane`, `ike/engine`, `ike/transport` (20 files).
4. **Phase P4: iface-core** - `internal/component/iface` (20 files).
5. **Phase P5: iface-plugins** - `internal/plugins/iface/{netlink,dhcp,ra}` (19 files; 3 tagged in `dhcp`, 2 in `netlink` and `ra`).
6. **Phase P6: dataplane** - `fib/kernel`, `firewall/nft`, `flowspec-firewall`, `flowexport`, `traffic/netlink`, `trafficusage`, `policyroute`, `copp` (27 files).
7. **Phase P7: routing** - `ospf`, `isis` (not `frr_interop`), `ldp`, `static`, `rsvpte` (not `freertr_interop`), `vrrp` (25 files; the largest tagged set: the vrrp files hold 118 tag lines).
8. **Phase P8: services** - `gtsm`, `bfd/transport`, `core/probe`, `core/routewatch`, `core/dnsserver`, `as112`, `dhcpserver`, `tftpserver`, `mtu/cmd`, `traceroute/cmd`, `chaos/peer`, `l2tp/pppoeclient` dialer (17 files). Traceroute targets only `127.0.0.1`, so inside the namespace nothing leaves the host; confirm in the batch.
9. **Phase P9: ppp** - after Thomas has run `./le setup install` and logged in again (or the run uses `sg dip`): the A-4 experiment, then convert `l2tp/bridge_integration_linux_test.go` and `l2tp/pppoeclient/unit_reservation_integration_linux_test.go`, keeping `integration && linux`, with a refused open of `/dev/ppp` failing with a message that names `./le setup install` and `ppp-access`. If the experiment fails: AC-16.
10. **Phase P10: docs and closure** - `docs/contributing/testing.md` (call `Enter` first, no `t.Parallel`, privilege refusals fail, choose `linux` unless the file execs a binary setup does not install), `docs/architecture/testing/qemu-integration.md` (userns paragraph names the mount namespace and private `/run/netns`; the Build Tags table row for `integration && linux` no longer reads "needs root"), `docs/architecture/testing/ci-workflows.md` line 37, then `/ze-close`.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every inventory row converted, reclassified, or moved to C with a reason |
| Correctness | No assertion removed or loosened; every privilege skip in a reached helper is now a failure; feature-absence skips untouched |
| Correctness | `Enter` is the first statement of each top-level test and appears in no helper, subtest, or `t.Parallel` test |
| Correctness | No test that must run unprivileged (root-skip tests) gained `Enter` |
| Data flow | Nothing the child mounts or creates is visible on the host (AC-2) |
| Rule: testing.md | every tagged unit changed has its judge row before the edit and a fresh record after |
| Rule: user-facing-errors.md | the startup failure diagnostic preserves the error and conditionally names the namespace-policy fix; `ppp-access` details name its exact fix command |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| `Enter` serves named netns | `TestEnterGivesAPrivateNetnsDirectory` PASS output |
| every A/B/D test runs | each batch's scratch log of `--- PASS` lines, one per listed test |
| privilege skips gone | the AC-9 `git grep` |
| `ppp-access` | `./le setup check` output and the two setup tests |
| records fresh | `./le rfc check` over the batches' stems |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Host isolation | child mounts are private and owned by the child's user namespace; no test writes the host `/run/netns` |
| Device grant | `/dev/ppp` 0660 to `dip` only; R-6 reasoning holds against the kernel source of the running kernel |
| Root route | setup writes only `/etc/udev/rules.d/60-ze-ppp.rules` (and the module drop-in when needed), through the existing privileged route, never asking for a password without a terminal |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test setup, never its assertion; the judge sees any assertion change |
| Test fails on behavior mismatch | A product defect: fix the producer in its own commit, as 88e01f1f6c did |
| Lint failure | Fix inline |
| A-1 broken | D-1 fallback: unnamed `netns.New()` per B file, recorded here |
| A-4 broken | AC-16 |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

### Exposed producer and kernel prerequisites

P3's migrated oversized-DF test found `probe.DrainErrorQueue` losing the router
offender: IKE enables packet info, but the ancillary buffer held only the extended
error, and `MSG_CTRUNC` was ignored. The repair reserves both messages and refuses
truncated control data. The unchanged transport assertion failed before and
passed after; `TestDrainErrorQueueRefusesTruncatedControlMessages` proves the
refusal with a real UDP error queue. Code and direct documentation landed in
`409b985e1e`. Current-session scratch `job-userns-errqueue-fixed-f42fc6eb.log`
records ten probe tests and the oversized transport test passing under race;
the two initial-privilege capture cases explicitly skip on this host.

P6's current-host run exposed three kernel prerequisites. `/proc/config.gz`
disables `NF_FLOW_TABLE`, `NET_ACT_POLICE`, and `NET_ACT_GACT`; these are not
loadable modules. The first two are already required by repository kernel
profiles. GACT was absent from the runtime fragment, manifest and compiled floor,
although the unchanged foreign-filter fixture encodes a generic PIPE action.
The related prerequisite repair adds it to `gokrazy/kernel/runtime.config`,
`runtime.require`, `internal/appliance/kernelreq.go`, its regression and
`docs/architecture/appliance/kernel-profiles.md`.

`TestRuntimeFloorRequiresBuiltInGenericAction` passes all four emitted states
under race; an overlay removing only GACT from the compiled floor fails modular,
disabled and absent states. `TestRuntimeProfileRequestsFloorSymbols` also passes.
Evidence: current-session scratch `job-userns-gact-floor-red-cb24ef76.log` and
`job-userns-gact-floor-green-bccc8dcc.log`. This proves enforcement, not a built
kernel. A runtime kernel build/boot and the unchanged three failed nft/traffic
tests remain owed. Do not convert their ENOENT failures into namespace skips.

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `unwired-feature.md` | 2026-10-28 | Runtime GACT prerequisite for ingress-policer coexistence |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| D-1: `Enter` unshares a mount namespace and mounts a private tmpfs on `/run/netns` | convert each B file to unnamed `netns.New()` | one change in one helper serves 61 files with no edit of their netns code, and `ip netns` users (rsvpte) keep working; proved possible by experiment. The fallback stays if the P1 test breaks A-1 |
| D-2: where `/run/netns` is absent, mount the tmpfs on `/run` and create `/run/netns` | require the host to create `/run/netns` | a host without it cannot get it without root; the child needs nothing else under `/run` because it has no network beyond its own namespace |
| D-3: a privilege refusal inside the namespace fails the test | keep the skip | the namespace grants the capability, so a refusal is a defect or an unmet need, which a skip would hide again (88e01f1f6c precedent) |
| D-4: `userns` gains a non-Linux `Enter` that runs the test in place | move the traceroute tests into `_linux_test.go` files | the traceroute tests carry no build tag; moving them would drop their non-Linux runs |
| D-5: a file drops from `integration && linux` to `linux` unless it execs a binary `./le setup` does not install (`nft`) or needs a host grant (D files) | keep `integration` everywhere | ordinary runs and agents' scoped evidence then execute them; QEMU still runs them |
| D-6: `/dev/ppp` goes to group `dip` | a dedicated `ze` group | `dip` is Debian's group for dial-up IP, which owns `/etc/ppp/peers` and `/etc/chatscripts` here, and the user already holds it, so the grant needs no new group and no new membership on this host. Setup creates `dip` with `groupadd --system` only where a distribution lacks it |
| D-7: one judge pass per batch | one per unit | the edit is the same mechanical change in every unit, so one brief carries every unit's before and after |
| D-8: D files keep `integration && linux` | drop to `linux` | the hosted CI runner and a fresh host lack the `/dev/ppp` grant, so a `linux` tag would fail them on host setup rather than on Ze |
| D-9: every child-process error fails, including namespace denial | skip permission errnos; add a stage-reporting launcher | Thomas selected “Fail all startup errors” on 2026-10-28, relayed by the parent session. This one-off approval changes AC-5 and the startup-skip contract. Go's `syscall.forkExec` returns clone and exec failures through the same errno channel, so an allowlist can hide executable failures. Keep the existing isolated re-exec, conditional setup guidance, and explicit selected-child skips. No launcher or host fallback is added. |

## Known Limitations
- Group C stays privileged: a one-uid map cannot chown to another uid, and the freeRtr and FRR interop tests drive external daemons that QEMU and the interop labs run.
- Performance of the re-exec per test is not measured on this host (perf runs happen on Thomas's Mac); R-5 tracks wall time only.

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
- [ ] AC-1..AC-17 all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass: each batch's packages, and `./le verify changed` over the closing commits. The full verification is Thomas's release check
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior (N-A: developer tooling, proved by the run-once PASS logs)
- [ ] Interop tests for protocol features (N-A: no wire change)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/release/validation/spec-privileged-tests-run-in-a-user-namespace.md` only, in the same `./le commit create` script
