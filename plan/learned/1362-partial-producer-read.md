# 1362 - Reading the first half of a producer is reading the caller

**Date:** 2026-08-08
**Scope:** evidence, agent workflow, review

## What Changed

`markMgmtAuth` (`cmd/ze/hub/mgmt_auth_reload.go`) now classifies a management
surface only when `(*ListenerMigrator).hasService`
(`cmd/ze/hub/listener_migrate.go`) holds a handle for it, and it runs after every
management server is built rather than before. Three functional tests landed in
`test/reload/`, closing rows that had been open since the spec's TDD plan was
written.

## The Failure

**A reload was refused over a server the daemon was not running.** An operator
enabling REST alone and deleting the API token got `grpc cannot change its
authentication while running` from a daemon running no gRPC server. The whole
SIGHUP failed.

## The verification failure that nearly shipped the wrong repro

A review reported the defect with a repro: a bare `api-server { token "x" }`
carrying no transport block at all. The main thread set out to verify it at the
producer, per `ai/rules/evidence.md`, and reported back that it held **at every
link of the chain**.

It did not. `ExtractAPIConfig` (`internal/component/config/loader_extract.go`)
ends with:

```
if !cfg.RESTOn && !cfg.GRPCOn {
    return APIConfig{}, false
}
```

so the reported repro answers `ok=false` and reaches nothing. The main thread had
read the function's first forty lines, seen the three early returns, formed a
coherent account of what the function answers, and stopped. The tail was
inferred.

**Reading part of a producer and inferring the rest is the same error as reading
the caller.** It has the same shape (a coherent story standing in for the text),
the same feel (certainty), and it is harder to catch, because "I read the
producer" is true.

The defect was real, and the implementer found the condition that does reproduce:
ONE transport enabled and the other absent, which is the common configuration
rather than a degenerate one. Had nobody re-derived it, the fix would have been
built against a repro that cannot happen, and its functional test would have been
vacuous while looking green.

## What To Do Next Time

| Situation | Do |
|-----------|-----|
| You are verifying that a function ANSWERS something | Read to its `return`. Every one. An early return proves what the function refuses, never what it accepts |
| You are about to write "verified at every link" | Name the links. The sentence is cheap to write and expensive to be wrong about, and writing them out is what exposes the one you inferred |
| An agent hands you a repro | Re-derive the CONDITION, not just the mechanism. The mechanism was right here and the condition was wrong, and only the condition decides whether a test is vacuous |
| A `sed`/`grep` range gave you the function | The range is your choice, so the cut is invisible in the output. Prefer the whole function, or check the last line you read is a `}` at column 0 |

## Files

- `cmd/ze/hub/mgmt_auth_reload.go` -- `markMgmtAuth` became a package function
  gated on `hasService`; name constants `svcWeb` to `svcGRPC`
- `cmd/ze/hub/listener_migrate.go` -- `(*ListenerMigrator).hasService` is new;
  `checkAuthRebuildable` is deliberately untouched
- `cmd/ze/hub/main.go` -- the marking call moved after the REST/gRPC build block
- `test/reload/mgmt-guard-reload-auth-rebuild.ci`,
  `mgmt-guard-reload-refuses-unauth.ci`,
  `mgmt-guard-reload-unbuilt-transport.ci` -- the three owed functional tests,
  each fenced on the reload generation so no assertion is vacuous against a
  reload that never ran
- `docs/guide/authentication.md` -- new `### Authentication on reload`

## Related

- `ai/rules/evidence.md` - the rule this nearly broke from the inside
- `plan/learned/1355-wire-edit-4-api-origin-deferred-bird-interop.md` - the
  neighbouring failure: a test that runs but asserts on an anchor the broken
  path also satisfies
