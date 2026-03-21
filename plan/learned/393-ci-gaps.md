# 393 -- CI Test Gaps

## Objective

Close all 42 functional test gaps identified in `docs/ci-test-coverage.md`. Write .ci tests proving existing features are wired and reachable from their intended entry points (CLI commands, API dispatch, config options, plugin behavior).

## Decisions

- Parse test runner requires `stdin=config` block even for commands that don't read stdin -- added dummy blocks to satisfy the framework
- `cli-status.ci` moved from `test/parse/` to `test/plugin/` because the parse runner infers "expect failure" from `expect=exit:code=1`
- `signal-quit.ci` skipped: `ze signal` has no quit handler, test framework has no `action=sigquit`
- Python plugin tests use `try/finally` for crash-safe daemon shutdown -- `sys.exit(1)` raises `SystemExit` which is a `BaseException`, so `finally` always runs
- Python plugin exit codes don't propagate to test verdict (`expect=exit:code=0` checks daemon, not plugin) -- wire-level assertions (`expect=bgp:hex=`) are the only reliable mechanism
- Content assertions (checking response data) serve as documentation and catch regressions if framework ever propagates plugin exit codes
- Dispatch command syntax must match exact CLI paths from `ze run help` (e.g., `peer detail` not `peer show`, `subscribe bgp event update` not `subscribe update`)
- For handler response format: summary wraps in `{"summary": {...}}`, capabilities returns flat object for single peer
- ADD-PATH and group-updates tests use capability/dispatch verification instead of exact hex (hex is fragile across ASN4/LOCAL_PREF variations)
- Role strict config is a YANG-augmented peer-level container (`role { name provider; strict true; }`), not inside `capability` block
- Role NOTIFICATION is subcode 11 (RFC 9234 Role Mismatch), not subcode 7 (Unsupported Capability)

## Patterns

- Parse tests: `stdin=config` + `tmpfs=file.conf` + `cmd=foreground:exec=ze <cmd> file.conf` + `expect=exit/stdout/stderr`
- Plugin dispatch tests: `stdin=peer` + `tmpfs=<name>.run` Python + `stdin=ze-bgp` config + `cmd=background` ze-peer + `cmd=foreground` ze daemon
- Wire-level tests: `expect=bgp:hex=` in ze-peer stdin (strongest assertion, checked by ze-peer)
- OPEN inspection: `option=open:value=inspect-open-message` for exact OPEN hex verification
- Capability removal: `option=open:value=drop-capability:code=N` to test capability enforcement
- Reload tests: config must be `tmpfs=ze-bgp.conf` (not `stdin=`), used with `action=rewrite` + `action=sighup`
- Python plugin naming: descriptive names (`summary-test`, `peer-list-test`) matching tmpfs file and process config

## Gotchas

- Parse test runner auto-validates stdin config before running `cmd=foreground` -- tests with `expect=stderr:contains=` get classified as "expect failure" validation tests
- Python `dispatch()` must guard against `_call_engine()` returning None (belt-and-suspenders since it actually raises RuntimeError)
- `json.loads()` inside `isinstance(data, str)` guard handles the engine's string-typed `data` field correctly
- Extended-message capability auto-included by ze changes OPEN byte count -- must verify actual OPEN bytes before writing hex expects
- iBGP (same ASN) auto-includes LOCAL_PREF in UPDATEs -- use eBGP or account for extra attribute bytes
- `connection both` causes port conflict with ze-peer (both try to bind same port) -- test `connection active` explicitly instead
- `cmd=api:` lines in peer stdin conflict with Python plugin pattern in test/plugin/ -- use dispatch or text send pattern instead

## Files

- 6 parse tests: `test/parse/cli-config-{check,fmt,set}.ci`, `cli-schema-{handlers,protocol}.ci`, `cli-exabgp-migrate.ci`
- 25 plugin tests: `test/plugin/api-{peer-list,peer-show,peer-add,peer-remove,peer-pause-resume,peer-capabilities,bgp-summary,subscribe,unsubscribe,route-refresh,rib-show-in,rib-show-out,rib-clear-in,rib-clear-out,cache-ops,cache-forward,commit-workflow,commit-lifecycle,raw}.ci`, `cli-{run,show,status,run-command,run-command-peer,show-summary}.ci`, `config-{connection-mode,group-updates,addpath-mode,ext-nexthop,role-strict,adj-rib}.ci`, `{adj-rib-in-query,role-strict-enforcement}.ci`
- 1 encode test: `test/encode/router-id-override.ci`
- 1 reload test: `test/reload/persist-across-restart.ci`
- Updated: `docs/ci-test-coverage.md`
