# Handover: container support, from design to implementation

Written 2026-11-16 by the session that designed the container work, when its
context passed the handoff threshold. Implementation had just started.

## What the work is

Ze runs OCI container images itself: one binary, behind a compile-time gate,
no Docker daemon. The owner asked for it, ruled on its design, approved moving
the specs to ready and starting implementation on 2026-11-16, and asked that
implementation agents run on the strongest tier. His closing test: run the
published Redis image as a container and reach the service from outside.

## Where everything is

| Thing | Where |
|-------|-------|
| Umbrella: the approved design, every owner ruling, the corrections table | `plan/post-release/features/spec-container-0-umbrella.md` |
| Persistent partition grows at install; a format the kernel can cap a folder on | `plan/post-release/features/spec-appliance-perm-grows-at-install.md` |
| Runtime: the upstream runtime library as a dependency, the patch series, the filter, limits, supervision | `plan/post-release/features/spec-container-1-runtime.md` |
| Image store: registry fetch, a folder per digest, the writable layer, volumes | `plan/post-release/features/spec-container-2-image-store.md` |
| Network: container networks, published ports, a new firewall match | `plan/post-release/features/spec-container-3-network.md` |
| Operator surface: every config word and command word, and the closing Redis test | `plan/post-release/features/spec-container-4-operator-surface.md` |
| Shell: a shell in a running container, and a general command with an input stream | `plan/post-release/features/spec-container-5-shell.md` |
| Health check: a skeleton, a later child | `plan/post-release/features/spec-container-6-health-check.md` |
| Upstream runtime source, for reference | `~/Code/github.com/opencontainers/runc` (read tag v1.5.3 with `git show v1.5.3:<path>`), `~/Code/github.com/opencontainers/cgroups` (tag v0.1.0) |

All seven design specs are `ready` at commit `3bb507a717`. The reference
operator model the owner asked Ze to follow must never be named or quoted in a
spec, a page or a commit.

## State when this was written

| Item | State |
|------|-------|
| Code | None committed |
| Appliance spec, phase 1 (wiring) | An implementation agent was launched from the design session. Its handoff is appended to that spec's state file (`./le spec state latest spec appliance-perm-grows-at-install`) |
| Runtime spec, step 0 and phase 1 (wiring) | An implementation agent was launched from the design session. Its handoff is appended to that spec's state file (`./le spec state latest spec container-1-runtime`) |
| Both agents | They run nothing. Their owed commands are run by a main thread, one heavy job at a time, and each green piece is committed |
| Work-in-progress cap | 10 in progress before these two, cap 12: these two fill it. The image store, network, shell and operator surface children wait for a slot |

A session picking this up reads each spec's state file first, checks `git
status` and `git log` for what the design session committed after this
handover, and runs whatever owed commands are still unrun.

## Where implementation stopped in the design session (2026-11-16)

Both specs are claimed and in progress. Committed:

| Commit | What |
|--------|------|
| `3fb832b762` | The upstream runtime module at v1.5.3, alone: `go.mod`, `go.sum`, `vendor/`. The vendored files are already patched |
| `da9ac4a1b0` | The six vendor patches (one per hunk H-1 to H-6 of the runtime spec), their guard test, the importing file `internal/plugins/container/initrole_linux.go`, the page `docs/architecture/container/runtime.md` |
| `2cc0bbf3c3` | Appliance spec phase 1: the three core leaves as stubs, the installer's growth seams, the `request system storage grow` command, the guest action |

Proven by runs: the patched library compiles and vets with cgo off; the three
patch guards pass; every patch stamp matches; the installer unit group and the
packages the appliance wiring touched pass; lint is clean on the files changed.

Next, runtime child: its wiring is one third done. The first agent finished
edge (a), the dependency and the patch series. A continuation takes edge (b)
(the gate lines, the early-role leaf and dispatch, the plugin skeleton with
its registration, config root, verifier entry and schema nodes) and edge (c)
(the seams, the suite registration, the wiring tests). `runLibraryInit` is
unused until edge (b) registers it, which lint reports. That agent did not
read the operator surface child's tables, the feature-gates page or the two
pattern pages: the continuation must.

Next, appliance spec: phase 1 owes two things, then phase 2 (kernel facts in
the guest) starts.

1. The doctor check for an ungrown partition. The spec named
   `internal/component/host` for it, and that package cannot register a check:
   `internal/core/diagnostic` imports it, so the import would be a cycle.
   Ruling: register it in `internal/component/doctor`, beside the existing
   storage check in `checks_storage.go`, reading the state from a function of
   the host component, as the spec's own Existing Mechanisms row says. The
   code `doctor-storage-not-grown` and its check do not exist yet, so
   `./le arch doctor-coverage check` is red on the new log line in the
   installer until they do. Correct the spec's file list in the same work.
2. The functional wiring test is still a draft at
   `test/draft/plugin/host-storage-grow-refused.ci`. Its first run failed for
   the wrong reason: the test plugin never connected, most likely because the
   harness runs the committed fixture and the fixture was uncommitted. It is
   committed now: rerun `./le test bgp plugin --draft --pattern
   host-storage-grow-refused` and expect the predicted red ("refusal does not
   say this system is not a Ze appliance"), then promote the test.

Not yet run for the appliance wiring: `./le arch tier check`, `./le arch
operator-error check`, `./le arch error-wrap check`, `./le cli grammar`,
`./le cli ownership`, `./le doc yang-contract command-contract`, `./le plugin
boundary check`, `./le doc check all`.

Findings that change later phases of the runtime child:

- The device filter needs a patch on the cgroups module too. Its `devices`
  package imports the systemd driver and its bus client, which the design
  keeps out of the link, and without that package device rules are refused.
  The devices phase carries that patch with a stamp of its own.
- A start that fails midway leaves the container's cgroup behind until the
  container is destroyed: the patched parent creates the cgroup before the
  start. The supervision phase cleans it up.
- The filter seam is in place but reports itself as not enabled until the
  filter phase registers: a description that sets filter flags is refused
  until then.
- The two spec files' `Phase` cells still read `-`: the spec hook refused the
  agents' one-cell edits for lack of reads in their context.

## State at the end of 2026-11-16

The section above is history. This is where the work stands.

| Commit on the server's `main` | What | On the laptop's `main` |
|-------------------------------|------|------------------------|
| `9583686570` | Doctor check `storage-grown`, code `doctor-storage-not-grown` | Yes |
| `702d01ffc2` | Container plugin behind `ze_container`: gate, early init role (`internal/core/earlyrole`), config root, seams | Yes |
| `9b209e27e0` | Kernel enrolment for the CPU limit, four runtime doctor checks, pages | Yes |
| `4f375c8a39` | Appliance phase 2: the ext4 and project quota leaves are real, and the guest run settles the kernel assumptions | Yes |
| `06bea1adb5` | Runtime phase 2: a container starts and stops, `test/container/container-start.ci` green in the guest | Not yet: the tunnel to the laptop was down |

Proven by guest runs on Ze's kernel 7.3, as root:

- A container starts through the patched library with cgo off, under the
  appliance's own init: the first process cloned into a control group that
  already exists, six namespaces, loopback up and the only device. The
  runtime spec's assumptions about the start path and control groups hold.
- A mounted ext4 filesystem grows online (256 MiB to 1 GiB in about 0.27 s)
  and keeps its data; a stored mount option enforces project quota at mount;
  a write past a folder's limit is refused; enforcement can be turned on for a
  mounted filesystem.
- One recalled fact was wrong and is corrected in code: on a filesystem
  without the quota feature the kernel refuses the state query, so the leaf
  reads the feature bits from the superblock.

Decisions taken on the way that the owner has not ruled on:

| Decision | Why | Where |
|----------|-----|-------|
| The capability to create device nodes is withheld from a container's default set (thirteen capabilities, where his ruling says fourteen) until the control group's device allowlist lands | Without the allowlist it lets a container open any device of the host | Runtime spec, Key Design Decisions, marked pending the owner |
| While a container is enabled the plugin asks the sysctl plugin for `net.core.fb_tunnels_only_for_init_net=1` | The kernel builds the tunnel drivers in, so every new network namespace otherwise holds eight fallback tunnel devices, and the network child's criterion is that loopback is the only link | `docs/architecture/container/runtime.md` |
| Implementation agents ran on the development tier from the afternoon on | The strongest tier's allowance was exhausted until 2026-11-22 | This handover |

Owed next:

1. Deliver `06bea1adb5` to the laptop's `main` once the tunnel answers. A
   plain cherry-pick is refused there on most commits because the laptop's
   uncommitted work touches the same files: the commit's changes are merged
   beside those edits (`git merge-file` against the commit's parent) and only
   the commit's own hunks are committed through `./le commit create ... hunks`.
2. Runtime spec bookkeeping for the last two changes: the exact text is in the
   spec's state file, last handoff. The operator surface child's code table
   must say the runtime child now declares `doctor-container-failed`.
3. Runtime phase 3 onward (stop signal and timeout, the leftover sweep, the
   device allowlist with its patch on the cgroups module, the syscall filter,
   restart policy, the sealed-binary and memory-footprint tests: the 64 MiB
   floor is still unmeasured).
4. Appliance phase 3 onward. The draft
   `test/draft/plugin/host-storage-grow-refused.ci` stays red by design until
   the refusal text of its acceptance criterion lands.
5. Not run: lint of `internal/le/test/qemu` after its last edits; a stress run
   of the container test, which needs root and so runs only in the guest.

Known and not ours: the server's root disk logged write errors on 2026-11-13
and twice on 2026-11-16; one `le` build was damaged by them and set aside. A
file that reads as corrupt on that disk is checked against the kernel log
before it is treated as a code defect.

## Where and how the work continues (written late on 2026-11-16)

The owner moved the work off `main`. Everything from here is done in the
worktree `.claude/worktrees/container-support` of the server's checkout, on
the branch `worktree-container-support`, which started at origin's tip
`a443bf716e`. The server's `main` equals origin and stays so. Commits stay on
the branch until the owner lands it; ask him whether each commit should also
be delivered to the laptop's `main`.

**Start the next session inside the worktree.** A session that starts in the
shared checkout and enters the worktree afterwards cannot edit source there:
its edit hook still runs from the shared checkout and reads the claim state
there, so a claim made in the worktree is invisible to it and every source
edit is refused with "source edit while selected spec is not in-progress".
Reads of the doors and of the Go style page count only when the
shared-checkout copies are read. This stopped the design session's agents on
2026-11-16. A session started in the worktree directory does not have the
problem. It is a hook defect and no journal row records it yet.

Owner permissions for this work, not rules:

| Permission | His words, 2026-11-16 |
|------------|-----------------------|
| The container specs may be claimed above the work-in-progress cap (14 were in progress against a cap of 12). Each spec records it; the cap itself is unchanged | "Ignore the caps for that work." |

The three designs named below are kept in
`plan/handover/44-container-designs-ready-to-apply.md`, so they survive the
worktree's scratch.

State of each track:

| Track | State |
|-------|-------|
| Runtime | Phases 1 to 3 committed. Phase 4 is the syscall filter, phase 6 the device allowlist. Phase 4 is designed and not written: `tmp/handoff-runtime.md` in the worktree holds the filter program layout, the action mapping, the loader, the default profile's rules, the probe checks, the draft test and the test list. The profile source it chose and the two syscall tables are in the worktree's `tmp/session/2026-11-16-200343/scratch/`. Four questions it raises are in that handoff: the table file names (a `_amd64.go` suffix would compile only one table per host), the profile names no supported architecture has, one kernel-version rule, and the test pinning thirteen capabilities until phase 6 |
| Image store | Claimed above the cap and set in progress, no code written (the hook defect above). Its wiring phase is designed in `tmp/handoff-image-store.md` in the worktree: ten command nodes with their wire methods, and sixteen files in an order that keeps the tree compiling. Findings that change the plan: a plugin command cannot declare its own time bound today (every plugin command gets 30 seconds), so the one-hour bound of fetch and update needs a new field in the plugin's Stage 1 declaration, a change to the public plugin protocol to confirm before its phase 5; the image commands go in an interface of their own beside the `Store` seam; the test store is chosen at engine entry by its environment key, because a build linking both stores would register twice; no config node lands in phase 1, since a mandatory `image` leaf would refuse the runtime's six test configs; no new Go module is needed |
| Network | Claimed above the cap and set in progress, `Phase` 1/7, no code written (the hook defect above). Phase 1 is designed file by file in `tmp/handoff-network.md` in the worktree. It is narrower than the design session assumed: it uses no container word and covers only the interface and firewall engine requests, their API modules, the SDK methods, a test plugin and four wiring tests; the participant, config nodes and doctor codes are its phases 4 and 6. The three interface requests need a module of their own beside `ze-iface-api.yang`, because a test requires every request of that module to be bound to a command path |
| Appliance | Phases 1 to 3 committed. Open with the owner: the kernel's `gpt` boot option, so that an interrupted partition rewrite is survivable. `TestKernelReadsATableAtEveryStopOfTheRewrite` is red until he chooses, and growth stays unwired from the installer and the storage command |

Not built, in order: the image store and the network (the closing Redis test
depends on both), the device allowlist and the syscall filter, the shell, the
operator surface, the Redis test. No real image can run on Ze yet: the
containers that start come from a test store holding one fixture program.

## Order of implementation

1. The appliance spec and the runtime child, independent of each other.
   The runtime's first commit is the upstream runtime module alone (`go.mod`,
   `go.sum`, `vendor/`), before the commit that first uses it.
2. The image store (needs the runtime and the appliance spec) and the network
   child (needs the runtime).
3. The shell child, its probes first.
4. The operator surface child's own phases, last, ending with the closing
   Redis test in the appliance guest. Its tables of words are already settled
   and every sibling uses them.

One spec per session, each implemented by one agent per phase, implementation
agents on the strongest tier.

## What no run has proven

Each is an assumption row in its spec with the test that settles it, and each
child's early phases exist to settle its own.

| Unproven | Settled by |
|----------|-----------|
| The patched start path of the upstream library (init cloned into a cgroup that already exists) | Runtime phase 2: a container that starts in the appliance guest |
| Cgroup control under the appliance's init | The same test |
| The 64 MiB memory floor, which is a guess | Runtime phase 2's footprint test |
| Every kernel behaviour the appliance spec uses, recalled because no kernel source was on the design machine | Appliance phase 2, in the guest |
| The shell's entry window (a host process briefly inside the container's process namespace) | The shell child's probes, before its phase 1. If the probe for its assumption A-8 fails, its fallback entry design applies |

## Owner rulings a new session must not re-ask

They are in the umbrella's "Owner decisions" table. The ones most easily
re-asked: the runtime is the upstream library with cgo off; images are fetched
only by an explicit command and stored unpacked in a folder per digest; a
container's filesystem persists until its image is updated; containers run on
the appliance and under a root daemon at first; a published port must also be
allowed by the operator's firewall policy; no `privileged` node and no
host-path volumes; containers restart when Ze restarts; both external-program
uses (a container's entrypoint, and a program an operator names for a shell)
are authorised, and each register row lands with the code it names.

## Found on the way

A Bash hook reads the body of a here-document, and quoted text in a command's
arguments, as commands. It refused a commit message that mentioned a build
command and a temporary path as prose. Recorded in
`plan/journal/hook-existing-patterns-false-positive.md` (2026-11-16), owned by
`plan/post-release/engineering/spec-hook-le-data-arguments.md`. Until it is
fixed, word such text without the literal strings.
