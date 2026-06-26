# Fleet Management: How the New Direction Changes the Existing Design

**Date:** 2026-06-26
**Author:** review for Thomas
**Scope:** Compares the new "central hub + SSH tunnel + per-device web routing + device enrollment" direction against the fleet management design already in the codebase.

---

## TL;DR

The existing fleet design and the new direction agree on the single most important thing:
**the device opens a permanent, authenticated, outbound connection to a central hub, and
that connection survives NAT and reboots.** That part is already built and field-tested.

The new direction changes four things on top of that shared foundation, in increasing
order of cost:

| Axis | Existing design | New direction | Size of change |
|------|-----------------|---------------|----------------|
| Connection direction | Router dials hub, outbound, persistent | Same | None (already aligned) |
| Transport carrier | Custom TLS RPC (`#id verb json`) | SSH tunnel carrying HTTP | Medium / debatable |
| Config sync | Hub-authoritative pull, one-way down | Bidirectional (local edits flow up too) | **Large** |
| Web control | UI-only selector linking to each device's own URL | Hub proxies HTTP to the selected device over the tunnel | **Large** |
| Trust model | Pre-shared per-client secret declared in hub config | Runtime enrollment, admin approval, cryptographic device identity | **Large** |

The biggest single insight: **the "SSH tunnel" requirement is an implementation choice,
not a new capability.** Ze already has the persistent, NAT-traversing, authenticated
channel you want. The genuinely new requirements are (1) carry HTTP over that channel for
web command routing, (2) make config sync two-way, (3) add runtime enrollment with admin
approval, and (4) bind a cryptographic identity to each device. None of those four
*require* SSH. Choosing SSH means building reverse-tunnel support Ze does not have today
and re-implementing identity, enrollment, and sync that you would have to build anyway.

---

## 1. What the fleet idea was

There are two layers, and only the second is "fleet management":

- **Foundation (implemented, all 17 ACs done).** "Managed configuration": a ze instance
  fetches its config from a central hub over TLS and caches it locally for partition
  resilience. Learned decisions `444-fleet-config` and `481-managed-config`.
- **Operations platform (designed, not built).** The `spec-fleet-0` umbrella plus five
  child specs (`fleet-1` through `fleet-5`): persistent device registry, config templates,
  audit trail, inventory/health reporting, staged rollout. All currently `skeleton` /
  `design` status.

### The foundation in detail

Source of truth: `docs/architecture/fleet-config.md`, `internal/component/managed/`,
`internal/component/plugin/server/managed.go`, `pkg/fleet/`.

- **Connection direction.** The *device* dials the *hub*. A `client <name> { host; port;
  secret }` block under `plugin { hub { } }` declares where the hub is; the hub listens on
  a `server <name> { ip; port; secret }` block. The hub never dials the device. This is
  exactly the "permanent connection initiated from the ze router to the ze hub" you
  describe. It already solves NAT: the device is reachable because it holds the connection
  open from its side.

- **Transport.** TLS 1.3 with MuxConn multiplexed RPC. Line framing `#id verb [json]\n`.
  Four verbs only: `config-fetch`, `config-changed`, `config-ack`, `ping`. Heartbeat every
  30 s, 90 s timeout. Reconnect with exponential backoff (1 s to 60 s, 10% jitter).

- **Authentication.** Per-client shared secret. Each accepted client is *pre-declared* in
  the hub config: `server central { client edge-01 { secret "..."; } }`. At `#0 auth` the
  hub looks the token up against that list and binds it to the client name. One connection
  per name; a client cannot fetch another client's config (name is implicit from the auth
  session). This is "auth = authz."

- **Config sync.** One-way and hub-authoritative. The hub holds each client's config in its
  ZeFS blob. The admin edits it there (blob tools, SSH editor, web config editor). On
  change the hub sends `config-changed`, the client pulls with `config-fetch` *when it is
  ready* (not forced mid-convergence), validates, applies, and replies `config-ack`. The
  **device never pushes its local config up to the hub.** The hub is the single source of
  truth.

- **Web.** `CollectFleetPeers` in `internal/component/web/page_system.go` renders a peer
  selector, but it is **UI-only**: it lists config-declared peer URLs and links you to each
  device's *own* web UI. There is no proxy and no command routing. The web server is a
  plain single-device `http.ServeMux`; every handler dispatches to the *local* command
  executor. This works only when each device's web UI is directly reachable.

- **Registry.** Today the hub tracks connected clients in an in-memory
  `map[string]struct{}` with no metadata and no persistence (forgotten on restart). The
  persistent registry is `fleet-1`, still a skeleton.

### What was explicitly ruled out (and the new direction reverses)

From the `spec-fleet-0` umbrella "Out of Scope" table:

- **PKI / certificate lifecycle** — deferred. "Current ephemeral self-signed certs + token
  auth works for the target scale."
- **Zero-touch provisioning (ZTP)** — deferred to a separate spec.
- **Multi-hub replication** — hard non-goal.

The new direction pulls the first two back in.

### Naming caution: there are two "hubs"

`docs/architecture/hub-architecture.md` describes a *different* hub: `ze` as a local
process orchestrator running BGP/RIB/GR plugins over pipes on a **single box**. That is not
fleet management. Your "central ze hub / mothership" is the fleet hub in
`docs/architecture/fleet-config.md`. They share the `internal/component/hub/` TLS
infrastructure and the `plugin { hub { } }` config block, which is why the word is
overloaded. Worth keeping the two straight in any spec so reviewers do not conflate them.

---

## 2. What you are proposing (restated to confirm)

1. Routers connect to a central ze hub and **register** their configuration there.
2. **Bidirectional sync:** local changes on the device propagate up to the mothership, and
   mothership changes propagate down to the device.
3. The **web interface gains a router selector**; once a router is chosen, HTTP commands are
   channelled to that specific device.
4. The link is a **permanent SSH tunnel initiated by the router to the hub** (could be SSL),
   securing the data; HTTPS/HTTP control traffic rides **inside** that tunnel.
5. The hub must **identify incoming tunnels and let the admin accept them**.
6. We must be able to be **sure the connecting device is who it says it is**.

---

## 3. Point-by-point: how this changes the existing design

### 3.1 Connection direction — no change

Already done. Device-initiated, outbound, persistent, NAT-friendly. Keep it.

### 3.2 Transport carrier — medium change, and a real decision

Existing: bespoke 4-verb TLS RPC. New: SSH tunnel carrying generic HTTP.

Two honest paths:

- **Path A — reuse the existing TLS channel.** It is already persistent, authenticated, and
  multiplexed (MuxConn). Adding "carry HTTP frames" to it is a new verb / new stream type,
  not a new transport. Reuses auth, reconnect, heartbeat, caching as-is.
- **Path B — switch to SSH reverse tunnels.** More "standard" operationally (it is just
  `ssh -R`), but **Ze has no reverse-tunnel or port-forwarding code today** (confirmed:
  neither the SSH server nor the SSH client library implements `-L` / `-R` / `-D`). You
  would build that, then still build identity, enrollment, and sync on top.

Recommendation: treat SSH-vs-reuse-TLS as an explicit decision in the spec, and bias toward
reusing the existing channel unless there is an operational reason SSH specifically is
required (for example, you want operators to be able to `ssh` to a device by hand through
the same tunnel). "SSL" in your description is essentially "keep TLS," which is Path A.

### 3.3 Config sync direction — large change

This is the deepest semantic change. Existing sync is one-way and hub-authoritative; the
device is a follower that validates and applies. Your "local changes are pushed to the
mothership" makes the device a **co-author**. That introduces everything two-way sync
implies:

- Conflict resolution: what happens when the hub and the device both changed since the last
  sync? Last-writer-wins, hub-wins, device-wins, or a merge?
- A change-origin and version vector per config, not just the current single SHA-256 hash.
- A decision on whether local edits are *allowed* to win, or are only *proposals* the admin
  approves on the hub (a softer, safer first step that keeps the hub authoritative while
  still surfacing on-box changes).

None of this exists today. It is the item most likely to be underestimated. I would scope a
dedicated spec for it and start with "device edits become proposals visible on the hub"
before committing to true symmetric merge.

### 3.4 Web per-device command routing — large change, and the strongest justification

Existing selector is cosmetic (links to each device's own URL). Your design makes the hub an
**HTTP reverse proxy over the tunnel**: pick a device, and the hub forwards your HTTP request
down that device's held-open tunnel and streams the response back.

This is the feature that most justifies the whole effort, because it is the one the current
design *cannot* deliver: NAT'd CPE has no directly reachable web UI, so the UI-only selector
fails for exactly the devices a fleet most needs to reach. Routing HTTP through the tunnel
solves it.

Cost: the web component is single-device today (no proxy, no target parameter, auth keyed to
the caller's address). You need a hub-side reverse proxy, a device selector that carries a
target through to a per-device tunnel, per-device authorization, and SSE/streaming that works
through the proxy. This pairs naturally with `fleet-1` (the registry is the device list the
selector is built from).

### 3.5 Enrollment and identity — large change, reverses two deferrals

Existing: every client is pre-declared in hub config with a shared secret. There is no
runtime "a new device showed up, approve it?" step, and identity is a shared string that can
leak.

Your design needs three new things:

- **Runtime enrollment.** A device the hub has not seen connects; the hub records it in a
  pending state. Today there is no such state (unknown client = rejected). The `fleet-1`
  registry is the natural home for a "pending / approved / rejected" lifecycle.
- **Admin approval.** A queue and an accept/reject action (CLI + web). New surface, but small
  once the registry exists.
- **Cryptographic device identity.** "Sure the device is who it says it is" means moving from
  shared secret to a per-device keypair plus certificate, bound at enrollment. This is the
  `PKI/certificate lifecycle` the umbrella explicitly deferred. The building blocks exist
  (PKI cert store with chain validation, SSH host certificates, TLS fingerprint pinning in
  `internal/component/plugin/ipc/tls.go`) but the binding, enrollment, and mutual
  authentication do not. mTLS with a device cert issued at approval time is the natural fit
  and reuses the existing TLS channel (another point for Path A).

---

## 4. What already exists that you can build on

| Capability | Where | Reuse value |
|-----------|-------|-------------|
| Persistent outbound device→hub channel | `internal/component/managed/`, `internal/component/hub/` | High — this *is* your tunnel, minus SSH framing |
| Per-client auth bound to name | `internal/component/plugin/ipc/tls.go` (`AuthenticateWithLookup`) | High — extend to cert-based |
| Reconnect, heartbeat, local cache, partition resilience | `internal/component/managed/{reconnect,heartbeat,handler}.go` | High — keep as-is |
| Two-phase config change (notify, fetch-when-ready, ack) | `pkg/fleet/`, `managed.go` | High — the "down" half of bidirectional sync |
| PKI cert store + chain validation | `internal/component/pki/` | Medium — basis for device identity |
| SSH host certificates (CA-signed, skip TOFU) | `internal/component/ssh/ssh.go` | Medium — if you go SSH |
| SSH client library (exec/stream/protocol session) | `internal/core/ssh/client/client.go` | Medium — but no tunneling |
| TLS fingerprint pinning | `internal/component/plugin/ipc/tls.go` | Medium — device-identity primitive |
| Device registry / dashboard / templates / audit / rollout (designed) | `spec-fleet-1..5` | High — directly serves the new direction |

---

## 5. What is missing (honest gap list)

Confirmed absent in the codebase today:

1. **SSH reverse tunnels / port forwarding** in either the SSH server or the SSH client
   library. If you go the SSH route, this is foundational and unbuilt.
2. **HTTP-over-tunnel + multi-device web routing.** The web server is single-device; no
   reverse proxy, no device/target parameter.
3. **Runtime device enrollment with admin approval.** Auth is static, config-declared
   secrets; unknown clients are rejected, not queued.
4. **Cryptographic device-identity binding and mutual authentication.** PKI stores certs but
   does not enroll devices, issue per-device certs, or do client-cert auth.
5. **Bidirectional config sync.** The "up" direction (device edits → hub) and its conflict
   model do not exist.

---

## 6. Impact on the existing fleet-1..5 specs

The new direction does not invalidate the planned specs; it raises their priority and
re-roles a couple of them:

- **fleet-1 (device registry):** becomes more central. It is where enrollment state
  (pending/approved), device identity, and the web selector's device list live. Build first.
- **fleet-2 (config templates):** unaffected, still useful, but secondary to enrollment and
  web routing.
- **fleet-3 (audit trail):** gains obvious new events — enrollment requests, approvals,
  rejections, on-device edits pushed up. Higher value now.
- **fleet-4 (inventory/health):** unaffected; the report RPCs ride the same channel.
- **fleet-5 (staged rollout):** unaffected, downstream.
- **New specs implied:** (a) tunnel transport for HTTP / web reverse proxy, (b) device
  enrollment + identity (the reversed PKI deferral), (c) bidirectional config sync + conflict
  model.

---

## 7. Decisions to resolve before writing specs

1. **SSH tunnel or reuse the existing TLS channel?** This is the pivotal choice and it
   cascades into identity (SSH host/client certs vs mTLS) and effort (build reverse-tunneling
   vs extend MuxConn). My lean: reuse TLS unless hand-`ssh` access through the tunnel is a
   hard requirement.
2. **Is the device an author or a proposer?** True symmetric sync vs "local edits become
   approvals on the hub." My lean: start with proposals; it is safer and far cheaper.
3. **Identity model:** per-device mTLS cert issued at approval, vs SSH host cert, vs keeping
   shared secret plus pinning. My lean: mTLS cert issued at enrollment, reusing PKI.
4. **Approval UX:** CLI only, web only, or both; auto-approve within a trusted network vs
   always-manual.
5. **Does this supersede the "pre-declare every client in hub config" model,** or coexist
   with it? Enrollment and static declaration are two different trust models; pick one as the
   default.

---

## 8. Recommendation

The new direction is a coherent and worthwhile evolution, and most of it is additive rather
than a rewrite, **provided you reframe "SSH tunnel" as "persistent authenticated channel"**
— which Ze already has. Suggested order:

1. Build `fleet-1` (persistent registry) — prerequisite for everything else.
2. Add enrollment + admin approval + device identity (mTLS over the existing channel).
3. Add web reverse-proxy command routing over the channel (the highest-value new capability).
4. Add config-edit-from-device as *proposals* first; defer true bidirectional merge until the
   model is proven.

If, after step 2, you still specifically want SSH as the carrier (for operator hand-access),
add reverse-tunnel support as its own spec rather than letting it block the rest.

---

## Addendum (2026-06-26): decisions and step-one design

After review, the following decisions were made. They turn the open questions in section 7
into a small, additive first step.

### Decisions

| Question (section 7) | Decision |
|----------------------|----------|
| SSH tunnel or reuse TLS? | **Keep the current TLS protocol.** No SSH for now. |
| Device an author or a proposer? | **Neither — freeze local edits when in a fleet.** The device is a strict follower; the hub stays authoritative. |
| Identity model? | **Keep the existing pre-declared per-client shared secret** as the first step. Enrollment / PKI device identity deferred. |
| Approval UX / enrollment | Deferred (no runtime enrollment in step one). |
| Supersede pre-declared clients? | No — pre-declared clients remain the model for now. |

Net effect: **step one changes nothing about transport or auth.** It only adds a local-edit
freeze plus an emergency way to leave the fleet. Bidirectional sync, web reverse-proxy
routing, enrollment, and cryptographic identity all stay deferred.

### One flag drives both behaviors

`meta/instance/managed` already exists and is the natural single switch:

- `true`  → connect to the hub **and** freeze operator config edits.
- `false` → no hub connection, local edits allowed (standalone behaviour).

"Part of a fleet" and "frozen" become the same state. "Disable the fleet" and "unfreeze"
become the same action. This is the minimal model and matches the intent exactly.

### Where the freeze guard goes (confirmed against the code)

Every operator edit path converges on two editor functions and writes to `config.conf`
*before* reload. The hub-push path does **not** pass through them (it writes via
`OnCommit` → `WriteCandidateVersion`). So the guard goes on the operator functions:

| Operator path | Guard point |
|---------------|-------------|
| CLI / SSH editor commit | `internal/component/cli/editor_commit.go:23` `CommitSession` (before write at :161) |
| CLI / SSH editor candidate commit | `editor_commit.go:199` `CommitSessionCandidate` (before write at :319) |
| Web config editor | `internal/component/web/editor.go:168` — delegates to `CommitSession`, covered transitively |
| Rollback | `internal/component/cli/editor_commands.go:1075` `Rollback` (before write at :1092) |
| Save (daemon down) | `editor_commands.go:917` `Save` (before write at :936) |

The guard checks a `frozen()` predicate (true when `meta/instance/managed` is true) and
returns an error that **names the disable command**, so the operator learns the escape hatch
from the rejection itself.

**Do not** put the guard at `TxCoordinator.Execute()` / `ReloadConfig`
(`internal/component/config/transaction/orchestrator.go:198`) — the hub push reloads through
the same path, so a guard there would block the hub. The operator-vs-hub distinction exists
only at the write stage, which is why the editor functions are the correct chokepoint.

### Emergency disable: a real command, and an immediate sever

Today there is no toggle command (operators would use raw `ze data write
meta/instance/managed false`), and a disable only takes effect on the next reconnect or
heartbeat timeout (~90 s) or a restart — not immediately. Step one adds a proper command:

- `ze fleet disable` (naming TBD): set `meta/instance/managed=false`, **cancel the managed
  client context** so the live connection drops at once (today only the reconnect loop checks
  the flag — `internal/component/managed/client.go:57`), and unfreeze local edits.
- `ze fleet enable`: set the flag true and (re)start the managed client. Runtime re-enable is
  not wired today (the client goroutine is started once at `cmd/ze/hub/main.go:877`); either
  wire a restart or document that enable takes effect on the next `ze start`.
- `ze fleet status`: show managed on/off, hub address, connection state, and frozen state.

To make disable immediate, hold the managed client's `cancel` func somewhere the command can
reach (the client runs as `go managed.RunManagedClient(managedCtx, ...)`; cancelling
`managedCtx` stops it cleanly).

### Reconnect must not silently stomp the local change (operator decides)

A naive re-enable would let the hub config overwrite the emergency local edit on the next
fetch. That is unacceptable: the local change may be load-bearing, which is why the operator
made it. So on reconnect, if the configs differ, the device holds and the operator chooses
whether to adopt the router's change or revert to the hub.

**The rule that makes this simple: config is single-writer.** The hub's config for a device
cannot be changed while that device is disconnected. This is the mirror of the device-side
freeze, and together they mean a managed device's config is only ever editable in one place at
a time:

| Device state | Who may edit its config | Who is frozen |
|--------------|------------------------|---------------|
| Connected (in fleet) | the hub (pushes down, device acks) | the router (operator edits frozen) |
| Disconnected (fleet disabled) | the router (emergency edit) | the hub (this device's config frozen) |

Under these editor-level guards there is no window in which both sides change, so the normal case
needs no merge. (Raw `ze data write` to the blob can still bypass the guards and create a
two-sided divergence; the persisted baseline below keeps even that safe, since the device holds
rather than stomps.)

**Reconnect is a two-outcome decision, and it needs a persisted baseline.** The device persists
the hash of the last config it received from the hub (`meta/instance/managed/base-version`). On
reconnect it compares its active config to that baseline:

| Active config vs baseline | Meaning | Action |
|---------------------------|---------|--------|
| equal | not edited while away | **resync**: normal fetch, apply any pending hub update |
| different | edited while away (emergency) | **hold + show the diff** for the operator to adopt or revert |

The baseline is required, not optional. (An earlier version of this design claimed it was
unnecessary under single-writer; that was wrong, caught in the critical review.) `cfg.Version`,
the token the client sends, is recomputed from the live config at startup (`ze_core_start.go:325`),
so on re-enable it equals the *edited* config, not the last hub-agreed one. Without a persisted
baseline the device cannot distinguish a local emergency edit from a normal pending hub update
(the hub legitimately gets ahead when the client defers a fetch mid-convergence, a deliberate
existing feature), so it would either miss the edit or prompt on every routine update. The
baseline lives in `fleet-6`.

While holding, the device keeps running its current config (the fix stays live) and reports a
`conflict` status to the hub. This reuses the existing `config-ack {"ok":false,...}` shape, so
no new message is needed to surface the hold.

**Enforcing single-writer on the hub side.** The hub's per-device config editor (CLI and web)
must refuse edits to a device whose session is not currently connected, the same way the router
refuses local edits while managed. This hub-side half of the freeze is what guarantees the
two-case reconnect above. *Implication, by design:* you cannot stage a config change for an
offline device; the change waits until it reconnects and you make it then. This is a deliberate
trade of staged-offline edits for zero merge complexity, and it means a future staged rollout
(`fleet-5`) targets only connected devices.

**One up-verb is still needed.** "Adopt the router's change" means the hub takes the device's
config as the new authoritative copy, which needs a client-to-hub push that does not exist today
(the four verbs are `config-fetch`, `config-changed`, `config-ack`, `ping`). Add one verb,
`config-push`, on the existing TLS transport (same `#id verb json` framing, exactly how
`fleet-4` plans to add `inventory-report`). It carries the router's config up so the hub can
render the diff and, on adopt, store it as authoritative. Bounded and operator-gated, not
continuous bidirectional sync.

**Scope.** `spec-fleet-6` owns the device-side freeze, the hub-side single-writer guard, the
persisted baseline, and the reconnect hold (so re-enable never stomps), plus `fleet
disable/enable/status`. `spec-fleet-7` owns resolution of a held `diverged` device: the
`config-push` up-verb, the commit-style diff, and adopt/revert.

### Resolving the conflict in the interface (a diff, like a commit)

When an operator opens a router that is in the conflict state, the interface shows a **warning**
and a side-by-side **diff**, reusing the existing config commit diff view. Label mapping, from
the hub operator's seat:

| Side | Is | Role |
|------|-----|------|
| Local (current) | the hub's config | what the hub holds as authoritative |
| Remote / new (candidate) | the router's divergent config | the emergency change made on the device |
| Apply | adopt remote into local | the router's changes are pulled up into the hub config |

This is the same mental model as a normal Ze commit (current vs candidate, Apply promotes the
candidate). Here the candidate was authored on the router, and applying it pulls those changes
up into the hub's authoritative config.

For the hub to render the diff it needs the router's config, so on conflict detection the device
pushes its current config up (the `config-push` verb from the previous section) as a pending
candidate. The hub then holds **both** versions, so the entire diff-and-decide step happens at
the hub, in the hub web UI, with no need to proxy HTTP into the router. (This is why the web
reverse-proxy routing can stay deferred: conflict resolution does not depend on it.)

Two outcomes from the diff screen:

- **Apply (router wins):** the router's config becomes the hub's authoritative copy;
  `base := router config` on both sides. The device is already running it, so they are
  immediately back in sync.
- **Keep hub (hub wins):** discard the router candidate, push the hub config down via
  `config-changed`, the device applies it, `base := hub`. This overrides the emergency change,
  but only on an explicit operator click after the diff has been seen.

**Terminology caution for the spec.** "Local" is used from two seats and means opposite things:
at the hub UI it is the hub config (the operator is local to the hub); in the device-internal
model it is the router's own config. The spec should use unambiguous internal names
(`hub_config`, `device_config`) and present `Local` / `Remote` only in the hub interface,
matching this wording. The device state itself is named `diverged`, not `conflict`, because the
editor already uses `Conflict`/`ConflictStale`/`ConflictLive` for concurrent-edit-session
conflicts (a different concept in the same subsystem).

**Where it lives.** The hub web and CLI, on the per-router view of the fleet dashboard
(`fleet-1`). The diff reuses `internal/component/config/diff/` and the existing commit-diff
renderer (the same component behind `EditorManager.Commit` at `internal/component/web/editor.go`
and the CLI editor's commit diff). This is part of `spec-fleet-7` (resolution), surfaced through
the `fleet-1` dashboard.

### Known gap left open in step one

`ze data write file/active/...` followed by SIGHUP can still mutate config offline, bypassing
the editor guard. That is effectively the manual escape hatch and is acceptable for step one
(an operator doing it has blob-level access already). A source-aware secondary guard at
`ReloadConfig` could close it later, but it must whitelist the managed (hub-apply) source so
it does not block hub pushes.

### Suggested spec

This is a clean, self-contained spec — e.g. `spec-fleet-6-config-freeze.md`: the freeze guard,
the `fleet disable/enable/status` commands, and the immediate sever. It depends on nothing
new and reuses the existing managed infrastructure. It can land before `fleet-1..5`.

---

## Sources

- `docs/architecture/fleet-config.md`, `docs/features/fleet-management.md`,
  `docs/architecture/hub-api-commands.md`, `docs/architecture/hub-architecture.md`
- `plan/spec-fleet-0-umbrella.md`, `plan/spec-fleet-1-device-registry.md` (+ fleet-2..5)
- `plan/learned/444-fleet-config.md`, `plan/learned/481-managed-config.md`
- `internal/component/managed/`, `internal/component/plugin/server/managed.go`, `pkg/fleet/`
- `internal/component/ssh/ssh.go`, `internal/core/ssh/client/client.go`,
  `internal/component/web/server.go`, `internal/component/web/page_system.go`
- `internal/component/pki/`, `internal/component/aaa/`,
  `internal/component/plugin/ipc/tls.go`
