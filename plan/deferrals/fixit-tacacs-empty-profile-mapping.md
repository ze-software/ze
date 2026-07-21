# Deferrals: fixit-tacacs-empty-profile-mapping

Deferral rows for this source. The aggregate live backlog is folded on
read from `plan/deferrals/` by `/ze-status`; nothing stores it (`ai/rules/deferral-tracking.md`).

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-07-16 | spec-fixit-tacacs-empty-profile-mapping, spec-fixit-radius-empty-profile-mapping | The `authz.Store.Authorize` admin fall-throughs and the `hasUsers` logic (`authz.go:343`, `:345-350`, `:385-390`). Both escalation fixes enforce "authenticated implies at least one profile" LOCALLY at their authenticator; `Authorize` still grants admin for any OTHER producer of a profile-less authenticated user | Scoped by the user to the two empty-mapping defects, which are live and config-reachable. The general policy (what a profile-less user is entitled to) is answered "denied always" (user, 2026-07-16) but its implementation is a distinct change that also depends on the unanswered Q-4 (are internal identities subjects of authorization) | `plan/spec-fixit-authz-admin-fallthrough.md` (Q-2 answered; O-1' recorded) | deferred |

