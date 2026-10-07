# Traffic Control Lifecycle

## Meta

| Field | Value |
|-------|-------|
| Name | Traffic Control Lifecycle |
| Kind | daemon |
| Scope | complete |
| Level | experimental |
| Components | internal/component/traffic |
| Real-path tests | test/traffic/traffic-boot-apply.ci, test/traffic/traffic-reload-apply.ci, test/traffic/traffic-boot-qdisc-tc.ci, test/traffic/traffic-reload-qdisc-tc.ci, test/traffic/storage-tc-restart.ci |
| Docs | docs/guide/traffic-control.md |
| Doc review | 2026-10-07: every source anchor resolves (runEngine, OnConfigure, OnConfigApply, DefaultBackendName in internal/component/traffic); the row states reactor-level kernel-state evidence remains open |
| Defect review | 2026-10-07: journal identity-default-hides-a-mapping.md and gate-excludes-part-of-its-population.md rows name the traffic component; journal rows naming a Component, not each re-verified here: bound-wraps-before-it-refuses.md:17, closure-deletes-a-cited-document.md:6, comment-describes-superseded-behaviour.md:34, helper-bypassed-by-an-open-coded-copy.md:36, registry-contamination.md:14, unwired-feature.md:39, unwired-feature.md:132 |
| Extra criteria | supported: each journal row naming a Component re-verified as fixed or not a defect = none yet |

## Description

The `traffic/control` section of the config is now programmed at boot and on SIGHUP reload. The traffic component's reactor calls the selected backend's `Apply(map[string]InterfaceQoS)` in `OnConfigure` and `OnConfigApply`, with `sdk.Journal` rollback on apply failure. Linux default backend is `tc` (netlink); future backends plug in via `traffic.RegisterBackend`. Local privileged integration covers netlink qdisc snapshot/restore after backend restart; target-runner and reactor-level boot/reload kernel-state evidence remain open. <!-- source: internal/component/traffic/register.go -- runEngine, OnConfigure, OnConfigVerify, OnConfigApply, OnConfigRollback --> <!-- source: internal/component/traffic/backend.go -- Backend interface, DefaultBackendName -->
