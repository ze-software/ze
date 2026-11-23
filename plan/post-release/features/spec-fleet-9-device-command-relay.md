# Spec: fleet-9 -- Device Command Relay Through the Hub

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | fleet, protocol, cli |
| Priority | P3 |
| Release reason | Adds a capability absent from the supported release; existing supported behavior and its obligations remain required. |
| RFC gaps | - |
| Depends | plan/release/features/spec-fleet-1-device-registry.md |
| Phase | 9/10 of plan/post-release/features/spec-fleet-0-umbrella.md |
| Handoff | - |
| Updated | 2026-11-18 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

**Re-read after compaction:** this file, `spec-fleet-0-umbrella.md`,
`docs/architecture/fleet-config.md`, `internal/component/managed/client.go`
(the device's serial loop for hub requests),
`internal/component/plugin/server/managed_serve.go`,
`internal/component/plugin/dispatch.go` (`CommandDispatcher`).

## Task

Let an operator on the hub run a command on a managed device over the
connection the device already holds to the hub. The device may be behind
address translation, with no address the operator can reach; the managed
channel is the only path, and it already exists.

Today that channel carries four verbs, all about configuration. This spec adds
the relay: the hub sends a command to a device, the device runs it through its
own command dispatcher under an authority its own configuration grants, and the
answer returns to the hub, streamed when the command streams.

This is what the device tabs and the per-device console of the fleet web
interface run on (`spec-fleet-ui-4-device-tabs-and-console.md`). It is also a
command-line feature in its own right: an operator at the hub's CLI, or any
client of the hub's API, can run a command on a named device.

### What this changes in the fleet umbrella

`spec-fleet-0-umbrella.md` listed "Web per-device command routing" as out of
scope and described it as the hub proxying HTTP to a device. The owner brought
it into scope on 2026-11-18, and the mechanism is different: the hub relays
commands, not HTTP. A command answers data under one authorization and audit
path. A proxied page would carry another daemon's session, routes and markup
through the hub.

### Identity and authorization (owner decision, 2026-11-18)

A device does not know the hub's users, and nothing is granted by implication.

| Rule | Detail |
|------|--------|
| The grant is the device's | The device's own configuration names the authorization profile that relayed commands run under |
| No grant, no relay | A device with no such line refuses every relayed command, and the refusal names the line that adds it |
| The profile bounds everything | A relayed command is authorized on the device exactly as if a user holding that profile had typed it. A read-only profile makes the relay read-only |
| The hub checks first | Before relaying, the hub checks that its own user may relay to that device and may run that command class |
| The actor is carried | The hub user's name travels with the command and is written to the device's audit log beside the profile used |
| The hub is authenticated, not trusted further | A hub that authenticates gains only what the device's profile allows |

The two sides, as an operator sees them:

| On the device | On the hub |
|---------------|------------|
| A configuration line granting relayed commands a named profile. Its place in the schema is fixed at design; it belongs with the device's managed-client settings | Authorization profiles that allow or deny the relay, per command class (read, operate, console) |
| `show fleet status` names the profile granted, or says that none is | The device page states the profile the device grants |
| The audit log records each relayed command with the hub user's name | The audit log records each relayed command with the device's name |

### The relay on the channel

| Rule | Proposal |
|------|----------|
| Verbs | One verb, hub to device, carrying the command text, the actor's name, a deadline and a request identifier; its answer is streamed. One verb to cancel a running request |
| Not serial | The device runs relayed commands on a small bounded pool of workers, apart from the loop that handles configuration notices, so a slow command can never delay a configuration push |
| Concurrency | A stated maximum of relayed commands running at once per device; beyond it the device refuses with a message saying so |
| Deadline | Every request carries one. The device stops the command when it passes, and the hub stops waiting |
| Cancellation | Closing the operator's tab, cancelling a task or losing the connection cancels the command on the device |
| Streaming | Commands that stream (`monitor` and long listings) send their output as it is produced, with flow control, inside the frame bound of the channel |
| Bounds | Output is capped at a stated size; a capped answer says it was cut |
| Unknown verb | A device answers an unknown verb with an error instead of dropping it, so a hub never waits out its deadline against a device that does not know the relay |
| Capability | The device states at connect which verbs it supports, so the hub knows before asking whether a device can relay |
| Disconnect | A lost connection ends every relayed command of that device, on both sides |

### What the hub exposes

| Surface | Form |
|---------|------|
| A dispatcher bound to one device | The same type as the existing command dispatcher, so anything written against a dispatcher (the web command line, the workbench page builders that read only through one, the related tools) works against a device unchanged |
| A hub command | Runs one command on one named device and answers its data, so pipes and formats apply on the hub |
| A hub command over several devices | Runs one read-only command on a set and answers one result per device, as a task (`plan/post-release/features/spec-fleet-ui-9-tasks-notifications-and-alarm-pages.md`) |
| Completion and help | Answered by the device, so they match the device's software and build |

The command grammar for these is fixed at design, following the keyword-first
rule: the device is named by a keyword, never by position.

### What is not relayed

| Not relayed | Reason |
|-------------|--------|
| Configuration editing sessions | The hub is the single writer of a managed device's configuration (`fleet-6`); it edits its own copy and pushes it |
| Commands answered in the caller's own process | They would answer about the hub. The device's daemon form of the command is what is relayed |
| File transfer | Not a command. Software update has its own path |

### Scale

The hub holds each managed connection for its life under a cap of 128
(`managedMaxConns`). An estate larger than that cannot connect, whatever the
interface shows. Lifting that cap belongs to the registry work
(`plan/release/features/spec-fleet-1-device-registry.md`); this spec states the dependency and adds
its own per-device and hub-wide bounds on relayed commands.

### Key design decisions (to confirm at the design gate)

| Decision | Alternative | Reason |
|----------|-------------|--------|
| Relay commands | Proxy HTTP to the device's web server | One authorization and audit path; no trust in another version's markup; no second session; works for the CLI and the API, not only the browser |
| The device's configuration grants the profile | The hub decides alone; a same-named user on each device | The owner's rule: what a device does is what its operator configured. The hub-decides form makes a compromised hub the owner of every device. Same-named users defeat the purpose of fleet management |
| A worker pool on the device | Running relayed commands in the existing serial loop | A slow command would block configuration notices |
| A dispatcher of the existing type | A relay-specific interface | Everything written against a dispatcher works against a device with no change |
| Streamed answers | One answer per command | `monitor` commands and long listings are part of operating a device |
| Unknown verbs are answered | Left dropped | A silent drop costs the hub its whole deadline for every device that does not know the verb |

### Risks to resolve in design

| Risk | Question |
|------|----------|
| A compromised or mistaken hub | What can it do with the profile granted? The answer must be exactly "what a user with that profile can do", proven by test |
| Runaway output | A relayed listing of a full routing table must be bounded and cancellable |
| Abandoned streams | A stream whose operator has gone must end on the device, not only on the hub |
| Version difference | The device and the hub may run different software. The relay carries text and data, and assumes nothing about the device's command set |
| Public contract | `pkg/fleet` is public API. The new verbs are a contract change, made with `docs/architecture/fleet-config.md` |
| Audit completeness | A relayed command must be audited on both sides even when it fails or is refused |

### Draft acceptance criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | A device granting a read-only profile; a hub user allowed to relay | A read command run from the hub answers the device's data, identical to the same command on the device | Formatting pipes run on the hub, because the answer is data |
| AC-2 | The same device; a command the profile forbids | The device refuses it as it would refuse that profile locally, and the refusal reaches the operator word for word | None |
| AC-3 | A device with no grant | Every relayed command is refused with the line that grants it | The configuration channel keeps working, because it does not use the relay |
| AC-4 | A hub user not allowed to relay | The hub refuses before anything reaches the device | None |
| AC-5 | A slow relayed command and a configuration push at the same time | The push is delivered and acknowledged without waiting for the command | None |
| AC-6 | A streaming command, then the operator leaves | The command stops on the device | None |
| AC-7 | A command past its deadline, or over the output cap | It is stopped, and the answer says which bound it hit | Output already received is kept and marked as cut |
| AC-8 | Any relayed command, allowed or refused | An audit entry on the hub naming the device, and one on the device naming the hub user and the profile | None |
| AC-9 | A device whose software does not know the relay | The hub learns it at connect and answers at once that the device cannot relay | None |
| AC-10 | A standalone device | No relay code path is reachable | None |
