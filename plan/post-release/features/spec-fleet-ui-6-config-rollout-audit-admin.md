# Spec: fleet-ui-6 -- Configuration, Rollouts, Audit and Administration Pages

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | web, fleet |
| Priority | P3 |
| Release reason | Adds a capability absent from the supported release; existing supported behavior and its obligations remain required. |
| RFC gaps | - |
| Depends | spec-fleet-ui-2-estate-tree-and-device-tables.md, plan/release/features/spec-fleet-ui-5-tasks-alarms-notifications.md, spec-fleet-2-config-templates.md, spec-fleet-3-audit-trail.md, spec-fleet-5-staged-rollout.md, spec-fleet-7-config-reconnect-resolution |
| Phase | 6/8 of plan/post-release/features/spec-fleet-ui-0-umbrella.md |
| Handoff | - |
| Updated | 2026-11-18 |

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

The hub holds each managed device's configuration and is its single writer
while the device is connected. The Config tab is where an operator reads it,
changes it and resolves a divergence.

| Section | Content |
|---------|---------|
| State line | Configuration state word (In sync, Pending, Rejected, Diverged, Never fetched), the version the device runs, the version the hub holds, the time and result of the last fetch |
| Source | Whether the device's configuration comes from a template and its group, or is its own; a link to the template |
| Configuration | The configuration as the hub holds it, in the YANG editor's tree, read-only for an operator who may not edit. Editing uses the existing per-user candidate and commit bar |
| History | A table of versions the hub has held for this device: time, actor, summary, result on the device. A row's side panel shows the diff against the version before it, in the existing diff view. Row action: Roll back to this version, which stages it as a candidate |
| Offline device | The hub cannot change a disconnected device's configuration. The editor is read-only and a banner says why, naming when the device was last seen |

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
| Result | A task (`fleet-ui-5`) and an audit entry |

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
| Export | The filtered entries as CSV or JSON, bounded to a stated maximum |
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
| A rollout shows what it will skip | The hub cannot change an offline device, so the form states how many devices are left out before it starts |
| Administration changes are configuration changes | Reviewable, committed and audited like every other change |
| Each page family lands with its back-end | The four families do not wait for each other |

### Dependencies and readiness

| Page family | Depends on | State |
|-------------|-----------|-------|
| Device Config tab, history | `fleet-1`, `fleet-6` | Missing; `fleet-6` is a release fix and lands first |
| Divergence resolution | `fleet-7` | Missing |
| Rollouts | `fleet-5` | Missing |
| Audit | `fleet-3` | Missing |
| Templates | `fleet-2` | Missing |
| Administration of devices, labels, tree views, access, settings | `fleet-1`, `fleet-ui-2`, `fleet-ui-5` | Missing |
| The editor, candidate, diff and commit bar | Available | |

### Draft acceptance criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | A connected device's Config tab | The operator reads the hub's copy, edits it, reviews the diff, commits, and sees the device's result as a task | An operator without the right sees it read-only |
| AC-2 | An offline device's Config tab | The configuration is read-only with the reason and the last-seen time | The history and its diffs stay readable, because the hub holds them |
| AC-3 | A diverged device | The banner, the diff with the hub on the left and the device on the right, and the two resolutions; the destructive one only after the diff was opened | None |
| AC-4 | A new rollout over a target with offline devices | The form states the connected count it will change and the offline count it will skip, and previews the batches | None |
| AC-5 | A rollout that pauses | Its page states why and which devices caused it, and offers Resume and Abort | None |
| AC-6 | The audit page with a filter | Exactly the matching entries, reproducible from the URL, exportable | No entry can be edited or removed from any page |
| AC-7 | Any administration change | It is staged, shown as pending, diffed and committed through the existing path | Saved filters apply at once, because they are per-user state, not configuration |
| AC-8 | Adding a device | The page shows what the device needs to join, and the secret is never shown again once set | None |
