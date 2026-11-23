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

| # | Item | State and next step |
|---|------|---------------------|
| 1 | Web/LG listener keeps a removed named certificate on reload (cmd/ze/hub/listener_migrate.go early return on empty name) | Fix UNCOMMITTED in: cmd/ze/hub/{listener_migrate.go,listener_migrate_lg_test.go,main_reload.go,main_reload_pki_test.go,reload_unwind.go,service_lg.go,service_lg_tls_test.go,service_tls.go,service_web.go,service_web_tls_test.go}, internal/test/fixture/{lg_pki_fixture.go,listener_migration_fixture.go,register_lg_pki.go,pki_reference_dropped_fixture.go}, docs/architecture/pki/tls-listeners.md, docs/guide/{config-reload.md,configuration.md,looking-glass.md}. Includes the parsed-tree gate on the certificate push and the rollback restore of installed listener certificates. Certificate tests passed (TestReloadDropping*, TestListenerMigratorUpdate*, TestRunReload*, TestPlaintextLG*) before the rollback restore was added |
| 2 | Verification of item 1 | Blocked: cmd/ze/hub does not build because another session's edit to internal/component/bgp/plugins/rib/rib.go leaves wasUp, isUp, seenBefore undefined. Once it builds: ./le test unit pkg path ./cmd/ze/hub/ run 'TestRollbackReloadRestoresInstalledListenerCertificate|TestReloadDropping|TestListenerMigratorUpdate|TestRunReload|TestReload|TestPlaintextLG'; mutations (remove 'restoreCertificates = priorCertificates.restoreOn(lm, false)' -> rollback test red; re-add 'if certName == "" { return nil }' in updateWebCertificate/updateLGCertificate -> TestReloadDropping* red); lint ./cmd/ze/hub/ and ./internal/test/fixture/ (quiet form: ./le job run label le-go quiet command ...); functional reload/web-pki-reference-dropped and reload/lg-pki-reference-dropped (drafts per the fix agent, then promote) |
| 3 | Commit item 1 | Trailers required (approvals in tmp/commit-weakening-approved-46487921.md): Weakening-approved lines for hub.TestListenerMigratorUpdateWebCertificate and hub.TestListenerMigratorUpdateLGCertificateNoOp (verdict APPROVE (b)); do not edit those two tests or the approvals go stale |
| 4 | Looking glass with no store, started on a named certificate, reload drops the name | Fix refuses the reload (fail closed) rather than starting plaintext; two tls-listeners.md rules conflict; recorded reading, Thomas may revisit |
| 5 | fleet-1 design (plan/release/features/spec-fleet-1-device-registry.md) | Revised at bd5b977372; needs a fifth independent ze-design-judge run (after item 1 lands, since its tls-listeners row quotes the fixed page), then Thomas runs ./le spec design approve spec plan/release/features/spec-fleet-1-device-registry.md words "..." from his terminal (no approval record exists; status says ready but is stale) |
| 6 | Rename plan/release/features/spec-fleet-ui-5-tasks-alarms-notifications.md (now the alarm list only) | Pure stem rename in a commit of its own, references updated |
| 7 | spec-fleet-10-drift-hash-chain.md | Another session rewrote the release copy (uncommitted) and created an untracked duplicate in plan/post-release/features/. Ask Thomas which copy is current; do not touch until then |
| 8 | fleet-0 umbrella Files to Modify names ze-plugin-conf.yang "fleet YANG container" | Stale (fleet-1 adds labels to the client entry, no fleet root); small fix |
| 9 | Pre-existing reds seen, not from this work | spec/roadmap TestRoadmapCommandUsesSelectedRevision (doc namespace summary), cmd/ze/hub TestConfiguredPluginConformance* (unknown command) and TestCheckKeyFilePath* (~/.zt group-writable), internal/test/fixture TestPluginObserversWaitForTheScriptedPeer, TestToolFixturesReachTheirProducer, TestRulesFixtureRootAliasesReachOnlyOwnedTree; ./le spec citation check dangling ref in spec-single-owner-lifecycle.md |
| 10 | Remaining fleet design choices for Thomas | fleet-ui-8 O-1..O-3 and its hide-already-shown and display rules; fleet-10 departure from the fleet-config non-goal "full config on change"; presented with each spec's design |
