# Spec: fleet-ui-6 -- Configuration, Rollouts, Audit and Administration Pages

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | web, fleet |
| Priority | P3 |
| Release reason | Adds a capability absent from the supported release; existing supported behavior and its obligations remain required. |
| RFC gaps | - |
| Depends | spec-fleet-ui-2-estate-tree-and-device-tables.md, plan/release/features/spec-fleet-ui-5-alarm-list.md, spec-fleet-ui-9-tasks-notifications-and-alarm-pages.md, spec-fleet-2-config-templates.md, spec-fleet-3-audit-trail.md, spec-fleet-5-staged-rollout.md, spec-fleet-7-config-reconnect-resolution |
| Phase | 6/9 of plan/post-release/features/spec-fleet-ui-0-umbrella.md |
| Handoff | - |
| Updated | 2026-11-25 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

**Re-read after compaction:** this file, `spec-fleet-ui-0-umbrella.md`,
`spec-fleet-0-umbrella.md` (the single-writer rule and the divergence model).

## Task

Give every remaining fleet capability a page. The `fleet` set specifies config
templates, an audit trail, staged rollouts and the resolution of a diverged
device, each with CLI commands and, until now, a one-line "web view". Those
views are specified here, in the shell and with the components of
`spec-fleet-ui-0-umbrella.md`, together with the administration pages of the
Fleet mode.

This spec is the last of the set and the one with the most dependencies. Each
of its four page families can land on its own as soon as the `fleet` child
behind it exists.

### Device Config tab (needs `fleet-6`, `fleet-7`; templates need `fleet-2`)

The hub holds the desired configuration of every client it declares. The Config
tab lets an authorized operator edit that copy before the first connection,
during a session, or while a device is offline. A device that has a local edit
holds it on reconnect until the difference is resolved.

| Section | Content |
|---------|---------|
| State line | Configuration state word (In sync, Pending, Rejected, Diverged, Never fetched), the version the device runs, the version the hub holds, the time and result of the last fetch |
| Source | Whether the device's configuration comes from a template and its group, or is its own; a link to the template |
| Configuration | The configuration as the hub holds it, in the YANG editor's tree, read-only for an operator who may not edit. Editing uses the existing per-user candidate and commit bar |
| History | A table of versions the hub has held for this device: time, actor, summary, result on the device. A row's side panel shows the diff against the version before it, in the existing diff view. Row action: Roll back to this version, which stages it as a candidate |
| Offline or newly declared device | Authorized operators can stage, review and commit the hub's configuration. The page shows the last contact time or Never fetched. A committed hub version awaits contact until the device supplies a result. |

Divergence, when the device is Diverged:

```
+--------------------------------------------------------------------------+
| (!) edge-lon-04 was edited locally while out of the fleet and is holding |
|     its own configuration. Nothing has been overwritten.                 |
|     Edited: 3 changes, last at 02:14.   [Review the difference]          |
+--------------------------------------------------------------------------+
|  HUB CONFIGURATION (a3f9)          |  DEVICE CONFIGURATION (c71e)        |
|    commit-style diff, hub on the left, device on the right               |
+--------------------------------------------------------------------------+
|  [Keep the device's changes]              [Restore the hub's version]    |
+--------------------------------------------------------------------------+
```

| Rule | Detail |
|------|--------|
| Banner | Shown on the device's Dashboard and Config tabs and counted on the estate's Needs attention card |
| Diff | The existing commit-style diff; the hub's configuration on the left, the device's on the right |
| Keep the device's changes | The device's configuration becomes the hub's copy. Confirmed in a dialog stating that the hub's version is replaced |
| Restore the hub's version | The hub's configuration is pushed to the device and the local edit is discarded. A destructive action: confirmed by typing the device's name, and only offered after the diff has been opened |
| Result | A task (`plan/post-release/features/spec-fleet-ui-9-tasks-notifications-and-alarm-pages.md`) and an audit entry |

### Rollouts tab of the estate (needs `fleet-5`)

| View | Content |
|------|---------|
| Rollouts table | The umbrella's table stack: name, target (the filter or group it was started from), state word (Pending, Rolling, Paused, Complete, Failed, Aborted), progress as batches and as devices, actor, started. Filter fields: `state`, `actor`, `target` |
| Rollout page (the row's link) | A header with the state and the controls valid for it: Pause, Resume, Abort, Force complete. A batch strip: one segment per batch with its state. A devices table: device, batch, result word, the device's answer, time |
| Paused rollout | A banner stating why it paused, in the hub's own words, and which devices caused it, with links to them |
| New rollout form | Target: a saved filter, a group node or the current selection, showing the count of connected devices it resolves to and the count it skips as offline. What to push: a template version or a configuration change. Batch size as a percentage. Failure threshold. Optional health gate. A preview of the batches before starting |
| Start | Confirmed in a dialog naming the count of devices and batches. Creates a task per batch |
| Destructive controls | Abort and Force complete are separated from Pause and Resume and confirmed by typing the rollout's name |

### Audit page (needs `fleet-3`)

Under Administration, at `/fleet/admin/audit/`, and scoped copies as a section of
the device's Tasks tab.

| Rule | Detail |
|------|--------|
| Table | Time, actor, event, device or target, result, summary. Newest first |
| Filter | The umbrella's query bar with fields `actor`, `event`, `device`, `result`, and a time range picker writing `after` and `before` terms |
| Side panel | The full entry, the task it belongs to, and the diff when the event changed a configuration |
| Export | The filtered entries as CSV or JSON, bounded by the existing auditExportMax of 1,000 entries |
| Read-only | No page control edits or removes an entry |

### Templates page (needs `fleet-2`)

Under Administration: a table of templates (name, groups using it, devices
using it, last change, actor). A template's page shows its text in the editor,
the variables it uses, the groups assigned to it, and a Preview control that
renders it for a chosen device and shows the result and its diff against what
that device runs. Changing a template shows, before commit, how many connected
devices will receive a new configuration and offers to push it as a rollout
instead of at once.

### Administration pages (sidebar tab Administration)

| Page | Content | Backing |
|------|---------|---------|
| Devices | Declared devices: name, group, labels, whether a secret is set, connected or not. Add, edit, remove. Adding a device shows the configuration the device needs to join, and never shows a secret after it is set | Hub configuration (`fleet-1`), through the editor and commit |
| Labels | Every label key with its values and the count of devices carrying each; rename a value across devices; remove a key | Hub configuration |
| Tree views | The named tree views and their ordered levels; add, reorder, remove | Hub configuration (`fleet-ui-2`) |
| Saved filters | The operator's saved filters; rename, remove, set the default | Per-user state |
| Templates | As above | `fleet-2` |
| Access | Which authorization profiles may use the Fleet mode, the device console, relayed diagnostics, configuration pushes and rollouts; shown as a table of profile against capability, edited through the existing authorization configuration | Hub configuration |
| Audit | As above | `fleet-3` |
| History | The metric history's retention, its size on disk against its cap, the oldest sample held, the devices using most | `fleet-8` |
| Settings | Offline alarm delay, resource alarm thresholds, finished task retention, the maximum number of live streams | Hub configuration |

Every administration page that changes hub configuration uses the existing
candidate, pending change bar, diff and commit. Nothing is applied on save.

### Key design decisions

| Decision | Detail |
|----------|--------|
| The hub's copy is edited with the existing editor | A device's configuration on the hub is a configuration tree like any other. No second editor is built |
| Divergence is resolved only after the diff is seen | The control that discards the device's edit is offered only once the difference has been opened, so nothing is overwritten unseen |
| A rollout shows what it will skip | A staged batch needs an acknowledgement, so the form counts offline devices separately before starting. The operator can still edit a declared offline client's desired configuration. |
| Administration changes are configuration changes | Reviewable, committed and audited like every other change |
| Each page family lands with its back-end | The four families do not wait for each other |

### Dependencies and readiness

| Page family | Depends on | State |
|-------------|-----------|-------|
| Device Config tab, history | `fleet-1`, `fleet-6` | The baseline and hold are implemented. This page needs the registry and the real config and history sources. |
| Divergence resolution | `fleet-7` | The hub's divergence commands exist. This spec supplies their page binding. |
| Rollouts | `fleet-5` | Missing |
| Audit | `fleet-3` | Missing |
| Templates | `fleet-2` | Missing |
| Administration of devices, labels, tree views, access, settings | `fleet-1`, `fleet-ui-2`; the settings page reads the alarm settings of `fleet-ui-5` and the task and stream settings of `fleet-ui-9` | Missing |
| The editor, candidate, diff and commit bar | Available | |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | A connected device's Config tab | The operator reads the hub's copy, edits it, reviews the diff, commits, and sees the device's result as a task | An operator without the right sees it read-only |
| AC-2 | An authorized operator opens the Config tab for a declared offline client, including one that has never connected | The operator can edit the desired config, review the diff and commit. The tab shows the committed hub version and last-seen time or Never fetched. After a clean reconnect it displays the device's actual result. A returning device with local edits holds and exposes its divergence. | A reader without write access sees the hub copy and history read-only. Until a device reports, its applied version cannot be inferred from the hub commit. |
| AC-3 | A diverged device | The banner, the diff with the hub on the left and the device on the right, and the two resolutions; the destructive one only after the diff was opened | None |
| AC-4 | A new rollout over a target with offline devices | The form states the connected count it will change and the offline count it will skip, and previews the batches | None |
| AC-5 | A rollout that pauses | Its page states why and which devices caused it, and offers Resume and Abort | None |
| AC-6 | The audit page with a filter | Exactly the matching entries, reproducible from the URL, exportable | No entry can be edited or removed from any page |
| AC-7 | Any administration change | It is staged, shown as pending, diffed and committed through the existing path | Saved filters apply at once, because they are per-user state, not configuration |
| AC-8 | Adding a device | The page shows what the device needs to join, and the secret is never shown again once set | None |

### Moved in (owner ruling 2026-11-23)

Thomas ruled (voice-transcribed answer "1 b") that the fleet features of the
first release are reached through commands on every command surface, and that
their web pages and views come after the first release (`ai/META-DECISIONS.md`,
"Goal and scope"). With that ruling the fleet interface's part table moved from
`plan/release/features/spec-fleet-1-device-registry.md` to
`plan/post-release/features/spec-fleet-ui-2-estate-tree-and-device-tables.md`
("Moved from fleet-1"), which names this spec as the owner of two parts:

| Part | Why here | Serves |
|------|----------|--------|
| `DivergencePart` | Its owner in `fleet-1`'s table, `spec-fleet-7-config-reconnect-resolution`, closed on 2026-11-19 without registering it; the divergence banner and diff are this spec's Device Config tab family. It reads `show fleet divergence name <name>` through the dispatcher | AC-3, and AC-9 below |
| `AccessPart` | Already this spec's (the access administration page) | The Access administration page |

The Config tab of a device stays here, after the first release, beside
`plan/post-release/features/spec-fleet-2-config-templates.md`, which owns the
`DeviceConfigPart` the tab reads. The first-release alarm Configuration resync
failed (`plan/release/features/spec-fleet-10-drift-hash-chain.md`) points at a
command, not at this tab; its web binding to the Config tab is homed by
`fleet-10` in `plan/post-release/features/spec-fleet-ui-9-tasks-notifications-and-alarm-pages.md`,
with the alarm pages.

| AC ID | From | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|------|-------------------|-------------------|-----------------------------|
| AC-9 | The `DivergencePart` registration (`fleet-1`'s part table, owner `fleet-7` closed) | A real build with the divergence part registered, and a device holding a local edit | The Dashboard and Config tabs show the divergence banner and the diff exactly as `show fleet divergence name <name>` answers them; a device that is not diverged shows no banner; a refused dispatch is an operator error the tab shows | None |

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md`, `ai/doors/api-surfaces.md` - a page reads commands through the dispatcher; edits go through the existing editor and commit
- [ ] `docs/architecture/fleet-config.md`, "The baseline and the hold" - the divergence the Config tab shows

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/web/fleetui/source.go` - `Source` is the union of six interfaces, `ConfigSource` among them; `fleet-ui-2` splits `ConfigSource` into the device config, config history, divergence, rollout, template, access and settings parts.

## Existing Mechanisms

| Mechanism | Source | Use in this design |
|-----------|--------|--------------------|
| Fleet Config page and admin actions | `internal/component/web/fleetui/handler_config.go`, `configTextFragment`, `offlineBanner`, `manageActionAnswer` | The prototype disables edit when a client is away and returns a toast for a mutation. Bind the real page to the named configuration editor and show the time of last contact independently of edit rights. |
| Declared-client write guard | `cmd/ze/hub/session_factory.go`, `undeclaredClientSessionRefusal` | A named editor session requires a declared client. Opening its desired configuration is allowed while it is offline. |
| Rollout and audit sources | `internal/component/web/fleetui/source_config.go`, `ConfigSource`; `handler_audit.go`, `auditExport.ServeHTTP` | Show the backend's structured rollout state and audit records. Export the filtered rows within the existing `auditExportMax` bound. |
| Divergence resolution | `docs/architecture/fleet-config.md`, “The hub's copy of a hold” | Read the held version and diff through the existing command and use the guarded adopt or revert operation. |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | One schema, one command registry, every interface | the door every change reads | "A command is declared once and reaches every interface." | agrees | The backend owns the command and structured result across command surfaces. The web UI consumes its result with the same user rights. |
| `docs/architecture/fleet-interface.md` | A source answers facts, the package derives | the // Design: page of internal/component/web/fleetui/handler_config.go | "A source answers facts only" | agrees | Authorization, desired and applied versions, rollout results and audit entries come from the hub. The page formats those facts. |
| `ai/doors/api-surfaces.md` | Boundaries | the door covering internal/component/web/fleetui/handler_config.go | "The adapter reaches it through the dispatcher." | agrees | Fleet pages submit commands to their owner and show a refusal from that owner to the operator. |
| `docs/architecture/web-interface.md` | Per-User Editor | anchored to internal/component/web/fleetui/handler_audit.go | "The `EditorManager` creates independent `cli.Editor` instances per authenticated user." | agrees | Open the named configuration in the caller’s candidate. Reviewing or committing another user’s candidate would violate the write boundary. |
| `ai/doors/le-tooling.md` | Where it sits | The door covering test/web | "A shipped `ze` never links an `internal/le` package." | agrees | The browser runner drives the shipped page through HTTP. The page imports no test tooling. |
| `docs/architecture/fleet-interface.md` | Design decisions | Fleet interface contract | "The proposed alarm defaults apply together. A missing relay grant stays visible while the device is offline. A measured alarm uses its existing threshold duration after reconnect. Causes match within one device. Thomas selected “Use all three defaults”." | agrees | Configuration and administration pages use the hub’s alarm state if a grant or resource problem affects an action. |
| `docs/architecture/fleet-interface.md` | Design decisions | Fleet interface contract | "Thomas chose the following rule for Resource high at reconnect. “End the old occurrence. Show a new alarm only if fresh samples qualify.”" | agrees | Historical resource alarms retain the occurrence boundary used by the hub. |
| `docs/architecture/fleet-interface.md` | Design decisions | Fleet interface contract | "When a critical alarm becomes visible again after its cause clears, show its toast to an authorized viewer of the affected device. Thomas selected “Show the critical toast”." | agrees | The task and alarm notification provider owns delivery after checking access to the device. |
| `docs/architecture/fleet-interface.md` | Design decisions | Fleet interface contract | "For Force complete on a staged rollout, Thomas chose “Leave unstarted devices on their prior version”." | agrees | The confirmation tells the operator that unstarted devices keep their previous version. |
| `docs/architecture/fleet-interface.md` | Design decisions | Fleet interface contract | "Thomas chose “Add the device Alarms tab”. The post-release device page offers the same authorized alarm rows as the device filter on the estate or group Alarms tab." | agrees | Configuration and rollout administration use the shared navigation, including the device Alarms tab. |

## Data Flow (MANDATORY)

### Entry Point
- A Config tab, rollout, audit, template or administration page request on the hub's fleet interface.

### Transformation Path
1. The page asks its part through the dispatcher-backed source.
2. A change is staged in the operator's candidate and committed through the existing path.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Web to daemon | `web.RouteDeps.Dispatch`; the existing editor for changes | No |

### Integration Points
- `ConfigSource` supplies the structured facts for each page. Fleet-ui-2 supplies its optional part registrations.
- Open the existing configuration session on the named client's stored configuration. Reuse its per-user candidate and declared-client check, including for a first configuration.
- Route live actions through their owner's authorized commands. A successful hub commit records a desired version. Only a device response may supply its apply result.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| A diverged device's Config tab on a real build | → | the divergence part over `show fleet divergence name <name>` | `test/web/fleet-config-divergence-live.wb` |
| A newly declared or previously connected offline device's Config tab | → | the named config session, commit and stored version history | `test/web/fleet-config-offline-edit.wb` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestDivergencePartReadsDivergenceCommand` | Fleet Config page's divergence provider | AC-9, including refusal from the dispatcher | |

## Files to Modify
- `internal/component/web/fleetui/handler_config.go` -- replace the prototype action and offline edit refusal with the named configuration session and its permission checks
- `internal/component/web/fleetui/handler_rollouts.go` -- preview connected targets and show the orchestrator's result
- `internal/component/web/fleetui/handler_audit.go` -- render and export the scoped, authorized audit result
- `internal/component/web/fleetui/handler_admin.go` -- wire the existing administration and template controls to the candidate or per-user preferences
- `docs/architecture/fleet-interface.md` -- record which management actions use real data and which require a device connection

## Files to Create
- `test/web/fleet-config-divergence-live.wb` -- compare the banner and diff with the device's recorded hold
- `test/web/fleet-config-offline-edit.wb` -- edit and commit on the hub, observe a later apply or hold, and preserve read-only access

## Design presentation

### What changes

The existing Config, Rollouts, Audit and Administration pages gain their real sources and actions as each backend lands. Config changes use the authenticated user's candidate. Rollouts count connected targets, and audit export uses the same authorized, filtered records shown by the page.

### Before and after

The prototype disables Edit for a declared client that has never connected. Its real Config tab lets a writer stage and commit the client's first configuration and shows Never fetched until contact. A subsequent offline edit likewise creates a hub version. The page shows the actual acknowledgement, refusal or hold when the device reports. A rollout preview separately lists how many members it can reach and how many it will skip.

### Alternatives set aside

Making edit depend on a live device would prevent first provisioning. Sending a page's edits through a live device command would bypass the hub's configuration history and existing hold resolution. The named hub editor already checks declaration and access and provides a candidate, diff and commit.

### Documents departed from

None. The prototype action is explicitly a demonstration. The functional page follows the hub's declared-client write rule.

### Choices for Thomas

Offline configuration editing and connected-only rollout targets were settled in the fleet umbrella on 2026-11-19. The Force complete dialog states that unstarted devices keep their prior version, per the 2026-11-25 fleet-5 decision. It identifies the rollout and how many targets have yet to start before the operator confirms.

## Implementation Steps

1. **Phase: Wiring** - bind the Config tab to a declared client's named editor session and prove that an authorized operator can commit the first configuration before the client connects.
   - Verify: `./le test functional web` runs the fleet browser cases of this spec
2. Connect the config, history and divergence parts. Show pending delivery and hold resolution from the hub's answers.
3. Bind rollout controls and previews to fleet-5, including its distinct Aborted state. Confirm destructive actions against the selected rollout.
4. Bind audit and template pages to their providers. Apply the caller's authorization to both the table and export. Bind administration writes through the configuration candidate or per-user preference store identified in the administration table.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-9 all demonstrated

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
