# 623 -- fw-9-traffic-lifecycle

## Context

The traffic component had a data model (fw-1), a YANG schema (fw-4), a tc backend
(fw-3), and a CLI (fw-5). It did NOT have a component reactor: the `traffic-control`
section parsed correctly but never reached the backend's `Apply` call, so tc was
never programmed on boot or reload. The pre-fw-9 state was a fully-shipped backend
with no way for a user to reach it from config. The goal was to land the missing
`register.go` (mirroring `internal/component/iface/register.go`) so traffic-control
is programmed at boot, re-programmed on SIGHUP, and gated by the shared
backend-feature walker from learned/621 -- lifecycle-only, no new YANG, so the
spec could land before fw-7 introduces a second traffic backend.

## Decisions

- **`Name: "traffic"`, `ConfigRoots: ["traffic-control"]`** over matching names.
  Chose the single-word plugin name (matching iface's `interface` and firewall's
  `firewall`) with the hyphenated YANG root. The spec's Critical Review row read
  "Name=\"traffic-control\"" but the Plugin Naming section, the deliverables, and
  the grep commands all say `"traffic"` -- went with the rationale, not the
  drafting slip.
- **Journal-based reload rollback over a no-op rollback.** Mirrored iface's
  `sdk.Journal.Record(apply, undo)` shape: on reload, `Apply(newDesired)` runs
  inside the journal, and the undo re-applies `previousCfg` so a failed
  partial-Apply can be unwound. The alternative was to trust trafficnetlink's
  own idempotence, but that produces silent mis-configurations when Apply
  half-succeeds.
- **`hasTrafficSection` helper over checking `cfg.Interfaces == nil`.**
  A config with `traffic-control { backend tc; }` and no interfaces is a valid
  "section present, no interfaces" state that still must run the backend gate
  and load the backend. An absent section is the "plugin idle" state. Separating
  presence-of-section from presence-of-interfaces kept both branches explicit.
- **Reactor-side backend default over requiring explicit config.** Added
  `default_linux.go` (`"tc"`) + `default_other.go` (`""`) + exported
  `DefaultBackendName()` mirroring iface. The CLI (`cmd/ze/config`) imports
  `traffic` and a sync-test asserts the CLI constant equals the runtime one, so
  cross-platform drift fails at build time.
- **`.ci` assertions on reactor log lines over `tc qdisc show`.** The shared
  `.ci` runner does not grant CAP_NET_ADMIN, and trafficnetlink's
  `netlink.QdiscReplace` fails with EPERM in that sandbox. Asserting
  `traffic-control config applied` / `traffic-control config reloaded` still
  proves the wiring reaches `backend.Apply`; the kernel-state assertion is
  deferred to a privileged integration test.
- **Backend-leaf deletion as the reload semantic change** in
  `002-reload-apply.ci`. The spec envisioned a qdisc-type mutation but any real
  qdisc change requires CAP_NET_ADMIN. Removing the explicit `backend tc` leaf
  still registers as a diff in the reload coordinator and exercises
  OnConfigVerify -> OnConfigApply -> Journal -> Apply -- which was the target.

## Consequences

- `traffic-control { ... }` now programs tc on boot and every reload. The
  previously-silent behaviour is gone; operators get log evidence at
  `ze.log.traffic=info`.
- Future annotation work (`spec-fw-7-traffic-vpp`) can add `ze:backend
  "<names>"` to tc-only qdisc/filter nodes as a one-line edit -- the gate call
  already runs in both OnConfigure and OnConfigVerify and the offline CLI row
  is already wired. The deferrals log records which tests land in that follow-up.
- `ze-test traffic` is a new subcommand dispatching `test/traffic/*.ci` via the
  shared `.ci` runner. `make ze-verify-fast` does NOT yet call `ze-test traffic`;
  the two new `.ci` files are exercised by explicit runs and by developers who
  want to validate the reactor. Wiring into `make ze-verify-fast` (alongside the
  other `.ci` suites) is a trivial follow-up.
- Adding a plugin triggers two hard-coded expected-name lists:
  `cmd/ze/main_test.go` (`TestAvailablePlugins`) and
  `internal/component/plugin/all/all_test.go` (`TestAllPluginsRegistered`).
  Forgetting either is an instant red in `make ze-verify-fast`. The check is
  cheap but the failure mode ("unexpected plugin \"foo\" registered") reads
  like a regression until you remember it's the new plugin itself.
- Third component reactor in the codebase (iface -> firewall (fw-8, still
  pending) -> traffic). The helper shape is stable: `validateBackendGate`,
  `parseTrafficSections`, `hasTrafficSection`, SDK 5-stage closures, journal
  Apply/undo. Future components can lift the template wholesale.

## Gotchas

- **The SDK does NOT dispatch OnConfigVerify/OnConfigApply when the diff for a
  plugin's root is empty.** `internal/component/plugin/server/reload.go:190-216`
  only adds a plugin to `affected` if `rootHasChanges(diff, root)` is true. A
  comment-only change inside `traffic-control` produces an empty semantic diff
  and the reactor never runs -- the reload test's first draft (comment-only
  config2) passed boot but silently skipped reload. The fix was to make the
  reload actually change a leaf.
- **Removing a plugin's root entirely auto-stops the plugin** (via
  `autoStopForRemovedConfigPaths`, `reload.go:160-170`). So "reload with
  traffic-control section removed" does NOT run OnConfigApply -- it closes the
  backend and kills the plugin process. Tests that want to exercise the reload
  path must keep the section present and mutate inside it.
- **Background `ze` in a `.ci` test does not receive `ZE_READY_FILE`.** Only
  the foreground path writes daemon.pid + daemon.ready (see
  `runner_exec.go:705-717`). Reload tests need ze foreground and the signalling
  script as a background cmd that polls for daemon.ready.
- **`expect=stderr:contains=` only fires inside the `ExpectExitCode != nil`
  branch** (`runner_exec.go:838-859`). Without `expect=exit:code=0` the runner
  falls through to the "peer produced successful" path and reports
  TYPE=unknown. Orchestrated `.ci` tests without a peer must set an exit code
  expectation.
- **The test file cannot reach `config.ListNode{children:...}` directly**
  because those fields are package-private. The exported helpers `config.List`,
  `config.Field`, `config.Container`, `config.NewSchema` are the only way to
  build a synthetic schema from outside `internal/component/config`. The
  `Backend` field on the returned `*ListNode` IS exported, so annotations can
  be applied after construction.
- **`sync.Once` cannot be copied -- govet's copylocks flags any
  `orig := backendGateSchemaOnce` -- but assigning `sync.Once{}` to the
  variable is fine because that's a value construction, not a copy. Tests that
  want to reset the cache must reset to a fresh zero value and mark it Done (or
  leave it undone, depending on whether the override or the real loader should
  win).

## Files

- `internal/component/traffic/register.go` (new, ~260 lines)
- `internal/component/traffic/register_test.go` (new)
- `internal/component/traffic/default_linux.go`, `default_other.go` (new)
- `internal/component/traffic/backend.go` (+ `DefaultBackendName()`)
- `internal/component/plugin/all/all.go` (blank import)
- `cmd/ze/config/cmd_validate.go` (traffic-control row)
- `cmd/ze/config/default_backend_traffic_linux.go`, `_other.go`, `_test.go` (new)
- `cmd/ze/main_test.go`, `internal/component/plugin/all/all_test.go`
  (expected-name lists updated)
- `cmd/ze-test/traffic.go` (new), `cmd/ze-test/main.go` (dispatch + usage)
- `test/traffic/001-boot-apply.ci`, `test/traffic/002-reload-apply.ci`,
  `test/parse/traffic-empty-backend.ci` (new)
- `docs/features.md`, `docs/guide/configuration.md`,
  `docs/architecture/core-design.md` (updated)
- `plan/deferrals.md` (+1 row: kernel-state assertion deferred)
