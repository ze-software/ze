# 779 -- Transactional Config Commit

## Context

Config commits were not uniformly transactional. SIGHUP already used the runtime reload path, but CLI, web, REST/API, and managed push could persist config before runtime accepted it. A failed verify or apply then left disk state ahead of running state. The goal was to make runtime authoritative across all user-facing commit surfaces.

## Decisions

- Persist commits as timestamped config versions with named pointers: `active`, `candidate`, `rollback`, and `recovery`.
- Stage a candidate before reload, promote it only after runtime reload succeeds, and clear it on failed verify/apply.
- Keep storage pointer handling storage-agnostic; filesystem storage and blob storage both use the same active/candidate semantics.
- Reuse `doReload` as the convergence point rather than moving persistence into the transaction coordinator.
- Managed config push now ACKs after synchronous commit result, not after pre-validation or cache write.

## Consequences

- CLI, web, REST/API, SIGHUP, and managed pushes now share the candidate -> reload -> promote flow.
- `rollback` records the previous active version after each successful promotion.
- Overlapping commits reject an existing candidate instead of overwriting it.
- Web commit handlers must have their reload hook installed before serving requests and must wait for plugin startup before accepting commits.
- Legacy active-file mirror failures after active pointer promotion are warnings, not commit failures.

## Gotchas

- A blob-only guard around API startup masked filesystem REST commit tests. API startup is independent from web TLS blob requirements.
- A web server can begin serving before plugin startup completes; early commits can otherwise bypass configured plugin apply rejection.
- Session candidate staging must write the version and candidate pointer under the same guard, or another commit can race in between.
- Listener migration happens after plugin/provider/engine reload and must roll those states back if listener reconfiguration fails.
- Full verification was blocked by an unrelated ExaBGP compatibility timeout, so transactional evidence came from targeted unit and functional suites.

## Files

- `internal/component/config/storage/pointer.go`
- `cmd/ze/hub/main.go`
- `cmd/ze/hub/main_reload.go`
- `cmd/ze/hub/managed.go`
- `internal/component/cli/editor_commit.go`
- `internal/component/cli/model_commands.go`
- `internal/component/api/config_session.go`
- `internal/component/web/editor.go`
- `internal/component/managed/client.go`
- `test/plugin/*config-commit*.ci`, `test/reload/commit-*.ci`, `test/ui/web-commit-*.ci`, `test/managed/config-push-transactional.ci`
