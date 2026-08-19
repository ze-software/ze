# Deferrals: cli-pipe-aliases

Rows deferred from spec-cli-pipe-aliases, which closed on 2026-08-19 and is no
longer in the tree. Each row names where the work goes, so nothing is recorded
without a destination.

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-08-19 | spec-cli-pipe-aliases | Operator-defined aliases in configuration, so an operator can name their own pipe expressions rather than only using the ones registered in Go at init | `container cli` (`internal/component/hub/yang/ze-hub-conf.yang`) can express a keyed list, and `list ... key "name"` is standard in this tree. The blocker is plumbing rather than schema: no list-valued config reaches `internal/component/command` today, and the one existing `environment cli` leaf arrives there as a scalar env string through `env.MustRegister` and `configuredDefault`. Building that path is a larger change than the alias mechanism itself | `plan/future/spec-cli-operator-defined-aliases.md`, which carries the four design questions the row was waiting on: which registry an operator's alias joins, what replaces the four registration-time `panic("BUG:")` refusals in `checkedAlias` when a typo comes from config, what a reload does to a table written once at init, and whether an operator's alias may shadow a shipped one | deferred |
