# A status field is a constant, so it reports nothing

A field in a status payload is written as a literal rather than read from the
state it names. It carries no information, and it states the opposite of what
the daemon did whenever the state it names is conditional. The tell is a status
key whose writer names a runtime property and whose value has no producer.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-08-22 | - | bgp rpki commands | `summaryCommand` writes `"validation-enabled":true` and `statusCommand` writes `"running":true`, both as literals (`internal/component/bgp/plugins/rpki/rpki.go`). Validation is conditional: `startSessions` clears `active` when the config names no cache server, and the `OnAllPluginsReady` handler then skips the `request bgp adj-rib-in enable-validation` dispatch and logs that it skipped it. So a daemon with the RPKI plugin loaded and no cache server configured answers `show bgp rpki summary` with `validation-enabled true` while no route is validated. `sessions-total 0` is the only field that hints at it, and it answers a different question | not fixed. Reading the fields from `active` changes what `show bgp rpki summary` answers, which AC-13 of `plan/spec-plugin-registers-pipe-operations.md` holds unchanged for phase 5. Found while giving the bare `show bgp rpki` command the aggregate half that `show bgp rpki summary` reports, which made the two answers share one writer and put both literals in one place |
