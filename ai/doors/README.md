# Subsystem doors

<!-- Living docs, maintained by hand (NOT generated). -->

A door is the page an agent opens before it works on a subsystem. It does not
explain how the code flows: that is the digest's job (`ai/digests/`). It does
not restate a rule: that is the rule file's job. It answers what an agent that
writes from habit gets wrong:

- where the subsystem sits in the design
- who outside this repository depends on it
- which boundaries it must not cross
- what to read before the first edit

**Read `system.md` first, always.** It holds the design every subsystem shares:

- third-party plugins
- the process boundary
- the public `pkg/` API
- registration over enumeration
- one schema and one command registry behind every interface

Then open the door for the area you work in.

**The write hook enforces the read.** Before a session's or a subagent's first
Write, Edit or MultiEdit of a Go file (tests included), a YANG module or a templ
file, it must have read `system.md` and the door covering that file, each in
full: a Read with no offset and no limit short of the last line, or a `cat`
with no pipe. The covering door is every door whose `door-base` header names
the longest whole-component prefix of the file's path, so a file under
`internal/component/bgp/plugins/` owes `bgp-plugins.md`, not `bgp-engine.md`,
and doors tied on that prefix (`pkg/ze`) are all owed. A file no header reaches
owes `system.md` alone. Prose and other files are never refused. The refusal
names each unread door (`writeDesignDoorsRead`,
`internal/le/hookruntime/writeedit.go`;
`ai/rules/points/architecture/directives/read-the-design-doors-before-the-first-code-edit.md`).

**A spec edit is gated the same way.** Before a Write, Edit or MultiEdit of a
spec (`plan/**/spec-*.md`), the context must have read `system.md` and every
door covering a repository path the spec names, in the spec on disk or in the
proposed text: its Files to Modify and Files to Create, and any other path it
cites. A brand-new spec that names no path owes `system.md` alone
(`writeSpecDoorsRead`, `internal/le/hookruntime/specdoors.go`;
`ai/rules/points/architecture/directives/read-the-design-doors-before-writing-a-spec.md`).

**A new door joins the gate by its header alone.** The hook parses the
`door-base` header of every page here at call time, so no list of doors exists
to update. Every page here except this README owes one well-formed header: a
page with none, a misspelt opener, or a header that does not close or names no
path refuses every code edit until it is repaired, rather than covering nothing
in silence.

**A read counts for the checkout whose copy it read.** An edit owes the doors
and the Go style guide of the checkout the edited file lies in: the nearest
directory above it that is the session's root or a git checkout of Ze (`go.mod`,
`feature-gates.txt` and a `.git` entry). The path as written is walked first,
so a file under the session's root stays its file through a symlink that
leaves it (`tmp/`); only a path that reaches no checkout as written is walked
again with symlinks resolved, and a checkout whose resolved directory is the
session's resolved root is that root. A read is the Read tool's whole read, or
a plain Bash line such as `cat <path>`, optionally after `cd <dir> &&`: only
`cat`, `cat -n` and `nl` of plain paths, joined by `&&` or `;`, with a `cd`
allowed only first, and no quote, variable, glob, comment, pipe, redirection or
heredoc. Any other line records nothing, absolute paths included. Each path is
resolved as the kernel opens it, links before `..`, a relative one from the
physical directory of the `cd` or of the session's cwd. Every operand must be
a regular file the hook can read, or the whole line records nothing: a `cat`
that fails makes `&&` skip the prints after it. A session cwd, a `cd` target,
an operand or a link on the way under `/proc` or `/dev` records nothing
either, because those resolve against the hook's process, not the shell's. A
line whose prints total more than 25000 bytes, counting the seven bytes of
line number `cat -n` and `nl` put before each line, records nothing, because
the harness cuts a long Bash output: print fewer files per line. A file in a
worktree under `.claude/worktrees/<name>/`, or in one elsewhere on the disk
(`ai/owner/machines.md`), owes that worktree's own `ai/doors/` and
`docs/contributing/ze-go-style.md`, and reading them is what clears it. A read
of the main checkout's copy does not count for a worktree edit, nor the
reverse: a branch can change a door, add one, or change the guide, and the
edit's own checkout is the one whose constraints apply. A refusal names the
copy to read by its path from the session's root
(`.claude/worktrees/<name>/ai/doors/system.md`), or by its absolute path when
the checkout lies outside it. A code file in no checkout (scratch) is not gated
by any doors; a symlink alias of the checkout is gated like the checkout
(`gatedCheckout`, `internal/le/hookruntime/readgate.go`).

**The main thread names the door in every subagent brief** that touches a
subsystem: "Read `ai/doors/system.md` and `ai/doors/<door>.md` before you act."

| Door | Subsystem | Covers |
|------|-----------|--------|
| `system.md` | Every change | The design all subsystems share |
| `core.md` | Core libraries and runtime state | `internal/core/`, `pkg/zefs`, statestore, families, env, events |
| `config-yang.md` | Config and YANG | `internal/component/config`, YANG modules, transactions, env vars |
| `plugin-infra.md` | Plugin infrastructure and public SDK | `internal/component/plugin`, `pkg/plugin`, `pkg/ze`, process protocol |
| `hub-engine.md` | Daemon startup and orchestration | `internal/component/engine`, `internal/component/hub`, `cmd/ze`, managed/fleet |
| `bgp-engine.md` | BGP engine | reactor, FSM, wire, messages, attributes, capabilities, NLRI, pools, filter API |
| `bgp-plugins.md` | BGP plugins | `internal/component/bgp/plugins/*` |
| `system-plugins.md` | System plugins | `internal/plugins/*` outside the routing protocols, ExaBGP bridge |
| `commands-cli.md` | Command registry, CLI and SSH | `internal/component/command`, `cmd`, `cli`, `ssh` |
| `api-surfaces.md` | External interfaces | `api`, `web`, `lg`, `gnmi`, `mcp`, `telemetry` |
| `forwarding.md` | Interfaces and forwarding | `iface`, `sysrib`, FIB, `vpp`, `firewall`, `traffic`, `sysctl`, `mtu` |
| `routing-protocols.md` | IGP and label protocols | OSPF, IS-IS, LDP, RSVP-TE, MPLS, BFD |
| `access-security.md` | Access and security | `aaa`, `authz`, `tacacs`, `radius`, `pki`, `ike`, `l2tp`, PPPoE/PPP |
| `appliance.md` | Appliance | `internal/appliance`, `gokrazy`, installer, updater |
| `le-tooling.md` | Development tooling and tests | `internal/le`, `test/`, the functional and interop harnesses |

## The shape of a door

Each door carries a `<!-- door-base: ... -->` header naming the subtrees it
covers, then these sections, in this order:

1. **Where it sits.** Its tier, what depends on it, what it depends on, whether
   it can be compiled out.
2. **Who depends on it from outside.** Third-party plugins, API clients,
   operators, protocol peers, migrated ExaBGP scripts: each contract, and the
   page that defines it.
3. **Boundaries.** What it must reach through a registry, the bus, the bridge
   or a command, and the import it must never add.
4. **Read before code.** A table: the page, the digest, the pattern, the rule,
   each with the question it answers.
5. **Traps.** The mistakes agents have made here, each traced to a journal
   row, a learned summary or the code.
6. **Gates.** The `./le` checks that judge a change here.

A door points. It does not copy. Every claim names the page or the code that
owns it, so a reader can check it and a stale door is visible. When a change
makes a door wrong, edit the door in the same change (`ai/rules/documentation.md`).
