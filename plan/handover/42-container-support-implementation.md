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
