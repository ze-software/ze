# Deferrals: kernel-profile-fixtures-leak-into-registry

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-08-09 | working-tree triage of `tools/installer-kernel/` untracked files | `test/install/appliance-kernel-registry.ci` and `test/install/kernel-compose.ci` write kernel profile fragments into the tracked source directory `registeredKernelProfiles` scans, cleaned only by an EXIT trap; an interrupted run leaves a real, resolvable profile behind and `.gitignore` covers none of it | Found while answering what the three untracked files were and whether they belonged in a commit. That question does not depend on the tests being fixed, so it gets a spec rather than a fix folded into the work in hand (`ai/rules/completion.md`) | `plan/spec-kernel-profile-fixtures-leak-into-registry.md` | open |
