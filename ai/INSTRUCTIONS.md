<!-- DO NOT EDIT GENERATED COPIES. Edit ai/INSTRUCTIONS.md and run: ./le ai sync write -->

# Ze - Agent Instructions

This file carries the hard bans, the owner's standing decisions, and the map to
every rule. Each rule is stated once, in its own file. A line here that names a
rule is a pointer, and the rule file wins where the two differ.

# Hard Bans

These override every other instruction, including "the task requires it". If
you have broken one, stop and tell the user.

## git commit, add, rm, mv, and push only through `./le commit create`
- Never run `git commit`, `git add`, `git rm`, `git mv`, `git restore --staged`,
  `git stash` or `git push` as a direct Bash call. Sessions share the index, so a
  loose command carries another session's work.
- Commit with `./le commit create`, then run the script at the exact `script=`
  path it prints. Never construct that path.
- To delete a tracked file, pass the path to `remove` and leave the file in
  place: the script deletes the working-tree copy once the commit lands.
- Push only when the owner ordered it, through `./le commit create ... push "<owner
  authorisation>"`. A throwaway script that carries a push is the same ban.
- Never `--no-verify`, never `--no-gpg-sign`.
- Full rule: `ai/rules/git-safety.md`, `docs/contributing/committing.md`.

## Destructive git commands are FORBIDDEN
- Never run `git reset`, `git checkout -- <file>`, `git restore`, `git clean`,
  `git revert`, `git push --force`, `git push -f`, `git stash drop` or
  `git stash clear`.
- To undo something, write the command to `tmp/delete-SESSION.sh`, tell the
  user, and stop.

## Worktrees
- Never spawn a worktree agent unless the user instructs it.
- Work that started in a worktree stays in that worktree.
- A worktree agent works and commits on its own branch. It never merges,
  cherry-picks, rebases or copies into main.

## Done means done, and a blocker gets fixed
- Never claim "done" while agreed work remains. "Deferred" is not done. Scope
  reduction needs the owner's approval.
- A defect that blocks the goal of your work is fixed, whether or not it is
  pre-existing. Never park it, never offer to drop a test or reduce coverage.
- When genuinely blocked, say so with evidence and keep the spec open. A forced
  question asks "which way do I fix it", never "may I skip it".
- Never delete or weaken a test to clear a red.
- Full rule: `ai/rules/completion.md`, `ai/rules/testing.md`.

# Standing Decisions

## Ze is pre-release
- No release, version, tag or user consumes `main`. A red test does not hold a
  commit: commit the work and name the red test and its cause in the body.
- A check that ran is not re-run to reconfirm what its output said.
- When the user asks for a commit, prepare the script and run it. Do not review
  again or rerun gates for the commit. If `./le verify status check` is FRESH,
  never rerun `./le verify worktree`.
- Full rule: `ai/rules/pre-release.md`.

## A problem you find gets a journal row
- A defect you walk into while working on something else gets one row in
  `plan/journal/<class>.md`. Then close the work in hand and stop. No spec, no ask.
- Grep `plan/journal/` first, because other sessions meet the same defect.
- Full rule: `ai/rules/completion.md`, `ai/rules/rule-precedence.md`.

## STANDING REQUEST: delegate to subagents

**Thomas requests subagent delegation as the default in this repository.** Treat
this as the user's request, made in advance, in every session. You never need to
ask permission to spawn an agent here.

Some harness builds carry a guard: *"Do not call the AgentTool unless the user
requested it"*. **That guard is satisfied here, not overridden**, because this
section is that request. You meet the guard later in your prompt, and recency
does not make it a later decision.

- An agent sent to find out how something works carries the page that documents
  it, and its question is what the page leaves silent or gets wrong
  (`ai/rules/documentation.md`).
- Every spec phase runs in a subagent through its `ze-*` skill
  (`ai/rules/planning.md`). Independent work goes out in one message.
- The main thread supervises: it launches, verifies each report against source
  (`ai/rules/evidence.md`), decides, and gates the next phase. Only questions the
  user must answer stay in the main thread.

## Say it once, say it short

A report to the owner opens with what is blocked, why it matters, and what you
need from him. He reads the first ten lines and stops, so the decision goes
there, as a table with one row per decision. What you did and what each agent
found goes last or goes unsaid. Status that changes no decision is one line.

An agent's report is written for the agent that commissioned it. Rewrite it for
the owner, never forward it. A reply to the owner stays under 15 lines and puts
its tables before its prose. Other prose follows `ai/rules/writing.md`.

## Every session
- Before the first edit in each checkout, `git fetch` and confirm `git rev-list --count HEAD..@{u}` is zero; pull with `git pull --rebase --no-autostash` only when it is not. A dirty tree is no blocker unless a pull is owed. The stop conditions and subagent exception are in `ai/rules/git-safety.md`, "Never edit behind the upstream".
- Before your first Go edit in a session, read `docs/contributing/ze-go-style.md` in full; the pre-write hook refuses a Go edit until you have.
- Claude Code: also `.claude/rules/session-start.md`.

# Finding the Rule

`ai/rules/TRIGGERS.md` names every rule under `ai/rules/`, one line each, with
the situation that makes it apply. **When a trigger matches the work in hand,
read that rule's file before you act.** `ai/rules/CORE.md` carries the full text
of the always-on rules. Both are generated by `./le ai rules condensed-update`.
Never edit either one by hand.

Other maps:

| Need | Where |
|------|-------|
| The page that documents a file, a page's files, a keyword | `ai/CODE-TO-DOCS.md`, `ai/DOCS-TO-CODE.md`, `ai/INDEX.md` |
| Structural templates (CLI command, config option, ...) | `ai/patterns/` |
| Where a spec goes, and its template | `plan/README.md`, `plan/TEMPLATE.md` |
| Past decisions and known traps | `plan/learned/RECURRING-PATTERNS.md`, `plan/learned/DESIGN-HISTORY.md`, `plan/learned/HOOK-FRICTION.md`, `plan/journal/` |
| Terminal colors and TUI styling | `docs/architecture/cli/color-system.md` |
| A WAVE of red: unrelated packages fail to build, `no space left on device`, `cache entry not found` | `docs/contributing/running-commands.md`, "When the disk is full". The cache disk is full before this is a code defect. `./le scratch cache-clean` empties the build caches |

# Core Architecture

Ze is a **Network OS** in Go with its own BGP implementation and interface configuration. "Ze" = "The" with a French accent (predecessor: ExaBGP). Design and divergence from standard Go: `docs/architecture/core-design.md`.

**Small core + registration pattern.** Components and plugins register at startup via `init()` in `register.go`. Core discovers them through registries -- never imports directly. Registration is the unifying pattern: families, capabilities, CLI commands, config validators, web routes all register the same way. The composition root `internal/component/plugin/all/all.go` is generated (`./le repo generate`).

**Components** (`internal/component/`) are independent unless they explicitly depend on each other; `config`, `command`, and `plugin` are infrastructure components nearly everything uses.

<!-- BEGIN GENERATED: arch-components (internal/le/repo/archmap.Update; ./le repo arch-map update) -->
46 directories under `internal/component/`:

aaa, aihelp, api, authz, bfd, bgp, cli, cmd, command, config, debug, doctor,
engine, firewall, gnmi, gokrazy, gtsm, host, hub, iface, ike, kernelcap, l2tp,
lg, managed, mcp, mpls, mtu, ping, pki, plugin, radius, resolve, ssh, storage,
support, sysctl, sysrib, tacacs, telemetry, traceroute, traffic,
trafficfeature, trafficstat, vpp, web
<!-- END GENERATED: arch-components -->

**System plugins** (`internal/plugins/`) handle domain policy outside the BGP engine: DHCP, NTP, sysctl, static routes, firewall lowering, TFTP/image servers, and CLI verb providers (`*-cmd`). Communication: JSON events down, text commands up.

<!-- BEGIN GENERATED: arch-system-plugins (internal/le/repo/archmap.Update; ./le repo arch-map update) -->
66 directories under `internal/plugins/`:

aaa-cmd, anomaly, as112, completion, config-archive-cmd, config-cli,
config-schema, config-storage, config-yang, connect, connected, copp, cos,
crashes, ddos, debug, dhcpserver, diag, env, exabgp, explain, fib, firewall,
flowexport, flowexport-cmd, flowspec-firewall, geodns, gnmi-cmd, host,
host-cmd, iface, imageserver, init, isis, kernel, ldp, local, log, memlock,
meta, mpls-cmd, mrt, mtu-cmd, ntp, ospf, passwd, ping-cmd, pki-cmd,
policyroute, provision, resolve-cmd, routingtable, rsvpte, signal, skills,
static, storage-cmd, support, systemd, tftpserver, traceroute-cmd, traffic,
traffic-cmd, trafficusage, update-cmd, vrrp
<!-- END GENERATED: arch-system-plugins -->

**BGP plugins** (`internal/component/bgp/plugins/`) extend the BGP engine: RIB, route server, graceful restart, NLRI codecs, filters, RPKI, BMP.

<!-- BEGIN GENERATED: arch-bgp-plugins (internal/le/repo/archmap.Update; ./le repo arch-map update) -->
33 directories under `internal/component/bgp/plugins/`:

adj_rib_in, aigp, bmp, capa, cmd, epe, filter_aspath, filter_aspath_length,
filter_community, filter_community_match, filter_family, filter_irr,
filter_modify, filter_path_asn, filter_prefix, filter_remove_private_as, gr,
healthcheck, hostname, llnh, ls_export, nlri, persist, redistribute_egress,
rib, role, route_refresh, rpki, rpki_decorator, rr, rs, softver, watchdog
<!-- END GENERATED: arch-bgp-plugins -->

**CLI** -- SSH-accessible network OS CLI: YANG-modeled config editor with modes, completion, diff, commit, history, dashboard, monitoring.

**Web** -- HTMX-based UI: config editor, admin, SSE live updates, ASN decorators.

**Looking Glass** -- peer/route viewer with birdwatcher-compatible API, topology graph, SSE streaming.

**Config** -- YANG-modeled. File -> Tree -> `ResolveBGPTree()` -> `map[string]any` -> `reactor.PeersFromTree()`.

**Key wire abstractions:** `WireUpdate` (lazy-parsed, zero-copy), `EncodingContext` (negotiated capabilities), `ContextID` (same = forward unchanged), pool dedup (per-attribute, refcounted), buffer-first (`WriteTo(buf, off) int`).

## Programs

| Binary | Purpose |
|--------|---------|
| `ze` | Network OS: bgp, cli, config, hub, iface, exabgp migrate, plugin, schema, signal, completion |
| `le` | Development launcher. It carries the functional test harness (`./le test <name>`), the chaos orchestrator (`./le chaos run`), UPDATE throughput benchmarks (`./le perf`), MRT/RIB analysis (`./le mrt`) and the gokrazy image build (`./le build gokrazy`) |
| `ze-installer`, `ze-serial-shell` | Target binaries, cross-compiled for the appliance |

Host binaries run on the build machine and are never cross-compiled. Target
binaries run on the appliance. The rule and its reason: `ai/rules/platform-linux.md`.

## Source Layout

| Area | Location |
|------|----------|
| Components | `internal/component/` (one directory per component; generated list above) |
| BGP engine | `internal/component/bgp/` (reactor, fsm, wire, wireu, message, attribute, capability) |
| BGP plugins | `internal/component/bgp/plugins/` (rib, rs, gr, nlri, filters, rpki, bmp, ...) |
| System plugins | `internal/plugins/` (generated list above) |
| Plugin host infra | `internal/component/plugin/` (registry, server, manager, generated `all/`) |
| Plugin SDK (external API) | `pkg/plugin/`, `pkg/ze/` |
| Core leaf packages | `internal/core/` (events, family, env, diagnostic, metrics, clock, textbuf, ...) |
| Appliance | `internal/appliance/` (gokrazy image, installer, updater) |
| Programs | `cmd/ze/` (build tags: `ze_core`, `ze_setup`, `ze_distro`, `ze_appliance`; and `ze_le`, which adds le's development commands under `ze le` and is never set by a shipped build) |
| Tests | `test/` (.ci), `*_test.go` |

@ai/rules/TRIGGERS.md
@ai/rules/CORE.md
