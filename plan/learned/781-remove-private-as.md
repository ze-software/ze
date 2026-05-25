# 781 -- Remove Private AS Policy Actions

## Context

This note captures the implementation lessons from `spec-pol-2-actions.md` while the spec is still open. The feature adds a `remove-private-as` policy action that strips or replaces RFC 6996 Private Use ASNs in AS_PATH and AS4_PATH. The hard part was not the range check. The hard part was preserving BGP wire semantics while still composing with Ze's text-based policy filter chain.

## Decisions

- Kept `remove-private-as` as a single-purpose filter plugin, because the policy roadmap prefers named action macros over a general policy language.
- Made the plugin return text-level intent plus a dedicated directive, because AS_PATH segment type and AS4_PATH presence are wire facts that flat filter text cannot preserve.
- Kept authoritative AS_PATH and AS4_PATH mutation in the reactor, using existing attribute parsing and `ModAccumulator` handlers.
- Applied export policy rewrites before EBGP local-AS prepend, because the transmitted path must be built from the policy-modified base wire.
- Treated `enable` and `disable` as valid boolean aliases in YANG validation, because the config parser and set serializer already accept and emit them.
- Isolated runner-generated daemon config files in per-test temp directories, because file-storage pointers are directory-scoped and shared temp dirs leak state between functional tests.

## Consequences

- Future policy action filters should send intent, not replacement wire bytes, unless the runtime path is explicitly changed to consume raw plugin output.
- Attribute action handlers need composition tests whenever two operations can target the same attribute, as with AS_PATH Set plus Prepend.
- AS4_PATH has to be considered whenever an AS_PATH policy claims RFC 6996 or four-octet ASN correctness.
- Config validation must mirror accepted config syntax, not only canonical internal values.
- Functional tests that run `ze -` from stdin need isolated storage roots if the daemon may create `meta/config/*` pointers or rollback versions.
- Targeted tests and interop evidence are useful, but they do not close a spec while `make ze-verify-changed` is still blocked.

## Gotchas

- `FilterUpdateOutput.Raw` exists but is not consumed by the policy path, so setting it would not change forwarded UPDATEs.
- `attribute.AttributesWire.GetRaw()` returns attribute value bytes, not a full path-attribute header.
- Flat AS_PATH text is suitable for matching and downstream filter state, but it is not suitable for segment-preserving rewrites.
- `os.CreateTemp("", ...)` creates files in the shared process temp directory. If code derives metadata paths from the config directory, separate filenames are not enough isolation.
- `golangci-lint` can report transient or cache-corrupted diagnostics. Reproduce with the exact package list before changing unrelated code.
- The remaining verifier blocker is `test/plugin/task-cancel.ci`, a separate MCP task timeout, not a remove-private-as failure.

## Files

- `internal/component/bgp/plugins/filter_remove_private_as/`
- `internal/component/bgp/reactor/filter_delta.go`
- `internal/component/bgp/reactor/filter_delta_handlers.go`
- `internal/component/bgp/reactor/reactor_notify.go`
- `internal/component/bgp/reactor/reactor_api_forward.go`
- `internal/component/bgp/config/filter_registry.go`
- `internal/component/plugin/server/server.go`
- `internal/component/config/yang/validator.go`
- `internal/test/runner/runner_exec.go`
- `test/parse/remove-private-as*.ci`
- `test/plugin/remove-private-as-*.ci`
- `test/interop/scenarios/36-remove-private-as-frr/`
- `test/interop/scenarios/37-remove-private-as-as4path-frr/`
