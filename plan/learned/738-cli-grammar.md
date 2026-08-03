# 738 -- CLI Grammar: Action Before Identifier

## Context

Four CLI handler families (show interface, clear interface, cache, commit) accepted user-supplied identifiers (interface names, cache IDs, commit names) in the same argument position as action keywords. This created an ambiguity class: an interface named "brief" or a commit named "list" would collide with keywords. The clear.go handler already documented this ambiguity with a workaround comment. The fix enforces a universal grammar rule across all CLI commands.

## Decisions

- Chose action-before-identifier grammar (`cache retain <id>`) over identifier-before-action (`cache <id> retain`) because the keyword set is closed and known at compile time, while identifiers are open-ended user input. Putting the closed set first eliminates the ambiguity class entirely.
- Chose deprecation warnings in response data (`"deprecated": "use: ..."`) over stderr logging because handler responses are the only output channel available to RPC handlers. The deprecated field is a sibling in map responses and an added key in unmarshaled JSON responses.
- Chose NOT to add YANG sub-containers for action keywords (cache retain, commit start, etc.) because the YANG dispatcher consumes sub-container tokens from the arg stream, breaking handler dispatch. The handlers do their own keyword matching; YANG descriptions document the grammar.
- Chose `json.Unmarshal` to add deprecation to JSON string responses over wrapping in `{"result": ..., "deprecated": ...}` because wrapping changes the response shape and breaks script parsers.
- Chose string-typed IDs at CLI layer over uint64 parsing at dispatch, consistent with the grammar rule. Parse to uint64 only at the point of calling the reactor API.

## Consequences

- `.claude/rules/cli-grammar.md` is BLOCKING for all future CLI command work. Wired into INSTRUCTIONS.md "Before You" table, cli.md rules/checklist, and cli-command.md pattern doc.
- Old grammar forms are accepted with deprecation. Scripts using old grammar will see a `"deprecated"` key in successful responses. This is intentional and non-breaking (new key, not changed shape).
- Cache and commit YANG files remain flat (single container, no sub-containers). Autocomplete for action keywords comes from the YANG description text, not container nesting.
- Fleet specs (0, 1, 4, 5) and the cli-grammar spec itself were updated to use action-first grammar.

## Gotchas

- YANG sub-containers with the same WireMethod as the parent consume tokens from the dispatch arg stream. Adding `container retain { ze:command "ze-bgp:cache"; }` under `container cache` caused `cache retain 42` to dispatch with args=[] instead of args=["retain", "42"]. The existing show interface sub-containers (brief, type, errors, rate) have the same issue but are not exposed because show interface tests call the handler directly, not through the dispatcher.
- `withDeprecation` for string Data (JSON) must unmarshal to map and add the field as a sibling. Wrapping in a new map (`{"result": jsonStr, "deprecated": msg}`) breaks `assert.Contains(resp.Data, "lo")` in tests and `jq .name` in scripts.
- Commit names that collide with action keywords (e.g., a commit named "list") cannot be created via the old grammar. The new grammar (`commit start list`) handles this correctly.

## Files

- `ai/rules/cli.md` (new rule)
- `ai/INSTRUCTIONS.md`, `ai/patterns/cli-command.md`, `ai/rules/cli.md` (enforcement chain)
- `internal/component/cmd/show/show.go` (handleShowInterface refactor)
- `internal/component/iface/cmd/clear.go` (handleClearInterfaceCounters refactor)
- `internal/component/bgp/plugins/cmd/cache/cache.go` (handleBgpCache refactor)
- `internal/component/bgp/plugins/cmd/commit/commit.go` (handleCommit refactor)
- `internal/component/cmd/{show,clear,cache,commit}/yang/*.yang` (description updates)
- `internal/component/iface/cmd/clear_test.go`, `cache_test.go`, `commit_test.go` (new tests)
- `docs/guide/command-reference.md` (grammar documentation)
- `test/plugin/cli-grammar-action-first.ci` (functional test)
- `plan/spec-fleet-{0,1,4,5}*.md` (grammar fixes in specs)
