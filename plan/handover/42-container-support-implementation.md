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
