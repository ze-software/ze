# Handover 63: fleet first-release specs and the listener certificate fix

Session of 2026-11-23. Thomas's words below are voice-transcribed.

## Landed (committed)

| Commit | What |
|--------|------|
| 4d5236a783, e54bba27d9 | ai/META-DECISIONS.md: fleet registry, health, metric history, alarm list, alarm causes and drift detection ship in the first release, through commands only; web pages, tasks, notifications and live updates after it |
| 4fe5c04f84 | le: plan/release/features category |
| 7af7e45c4d, 5a0bd0e762, 4fc6a60e92 | Six fleet specs moved to plan/release/features; fleet-ui-5 split (alarms in release, plan/post-release/features/spec-fleet-ui-9-tasks-notifications-and-alarm-pages.md after); web ACs moved to fleet-ui-2/3/4 |
| ae6ace9720, b3fcbb150e, ac0154d5f1, bd5b977372 | fleet-1 design: Design documents and presentation; issued leaf follows reload addresses; C-10 decided (c) "on commit we can perform the certificate change on the listener" (recorded in docs/architecture/fleet-config.md Design decisions); doctor coverage for failed hub certificate checks |

## Outstanding, every item

Updated at the end of the session of 2026-11-23, after the items below landed.

| # | Item | State and next step |
|---|------|---------------------|
| 1-3 | Web/LG listener keeps a removed named certificate on reload | Done: 2984d3eaa5. Unit tests, two mutations, lint, and the functional tests reload/web-pki-reference-dropped and reload/lg-pki-reference-dropped (80 stress iterations, runner-leased port) |
| 4 | Looking glass with no store, started on a named certificate, reload drops the name | The fix refuses the reload (fail closed) rather than starting plaintext; recorded reading in tls-listeners.md, Thomas may revisit |
| 5 | fleet-1 design | Done: revised e6eda5e8d8, judged APPROVE (digest e1897d459b10), approved by Thomas with C-11 (a), recorded 80f69b9f18. Next: implement plan/release/features/spec-fleet-1-device-registry.md in a fresh session (/ze-implement). Phase 6 adds the C-11 decision row to docs/architecture/pki/tls-listeners.md |
| 6 | Rename fleet-ui-5 | Done: 7b6cd880d6, now plan/release/features/spec-fleet-ui-5-alarm-list.md |
| 7 | spec-fleet-10-drift-hash-chain.md | The untracked post-release duplicate is gone. The release copy carries another session's uncommitted rewrite, which still names the old stem spec-fleet-ui-5-tasks-alarms-notifications on 5 lines: it must use spec-fleet-ui-5-alarm-list when it lands. HEAD's copy was already updated by hunk in 7b6cd880d6 |
| 8 | fleet-0 umbrella "fleet YANG container" | Done: fbe3d27b87 |
| 9 | Pre-existing reds seen, not from this work | cmd/ze/hub TestConfiguredPluginConformance* (unknown command) and TestCheckKeyFilePath*; reload suite: signal-quit, bgp-filter-irr-added-live, bgp-filter-irr-first-add-live, bgp-filter-irr-loaded-over-ssh, irr-bgp-commit-new-set, bgp-attach-grant-change-live, rfc7999-blackhole-agreement-removed-by-reload, traffic-usage-added-live, rfc8212-import-rollback (reads the durable config before any reload); internal/test/fixture lint: goconst "local" in deployment_quickstart.go, plugin_shell_extra_1_auth.go, tunnel_fixture_l2tp_auth_precedence.go; earlier: spec/roadmap TestRoadmapCommandUsesSelectedRevision, fixture TestPluginObserversWaitForTheScriptedPeer, TestToolFixturesReachTheirProducer, TestRulesFixtureRootAliasesReachOnlyOwnedTree; ./le spec citation check dangling refs in spec-single-owner-lifecycle.md and spec-feature-maturity-declared.md |
| 10 | Remaining fleet design choices for Thomas | fleet-ui-8 O-1..O-3 and its hide-already-shown and display rules; fleet-10 departure from the fleet-config non-goal "full config on change"; presented with each spec's design |
