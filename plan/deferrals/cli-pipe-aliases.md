# Deferrals: cli-pipe-aliases

Rows deferred from `plan/spec-cli-pipe-aliases.md`. Each names where the work
goes, so nothing is recorded without a destination.

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-08-19 | spec-cli-pipe-aliases | Operator-defined aliases in configuration, so an operator can name their own pipe expressions rather than only using the ones registered in Go at init | `container cli` (`internal/component/hub/yang/ze-hub-conf.yang`) can express a keyed list, and `list ... key "name"` is standard in this tree. The blocker is plumbing rather than schema: no list-valued config reaches `internal/component/command` today, and the one existing `environment cli` leaf arrives there as a scalar env string through `env.MustRegister` and `configuredDefault`. Building that path is a larger change than the alias mechanism itself | needs a destination spec | open |
