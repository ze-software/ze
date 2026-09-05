# Verification debt -- commit session ceca979b

Gates that had not run green over these commits when they were made.
Clear rows only through `le commit debt-clear` after the named gate exits 0.

| Date | Session | Subject | Gate owed | Reason | Status |
|------|---------|---------|-----------|--------|--------|
| 2026-09-05 | ceca979b | feat(doctor): report a hub block whose managed listener cannot bind | full native verification (not FRESH-green) | the shared gates are red on other sessions' files only: a duplicate codes declaration in internal/component/doctor tests, internal/component/iface, internal/component/ike/engine, internal/component/plugin/leaf_test.go, internal/component/plugin/process, internal/test/runner, and a command list rename in internal/core/ipc. This commit's own targets are green: go test for internal/component/plugin/doctor, and ze-test ui for the two new .ci files | open |
| 2026-09-05 | ceca979b | feat(doctor): report a hub block whose managed listener cannot bind | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
| 2026-09-05 | ceca979b | feat(doctor): report a hub block whose managed listener cannot bind | discovery-index freshness | the new Go file joins the existing package internal/component/plugin/doctor, so it adds no package. ./le discovery-index update was run and left ai/PACKAGE-MAP.md byte-identical, which git diff confirms | open |
