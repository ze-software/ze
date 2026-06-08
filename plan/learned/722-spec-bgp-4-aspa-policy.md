# 722 -- ASPA Policy Enforcement

## Context

ASPA path verification was implemented (spec-bgp-2-aspa) but operated in informational mode only: the verification result appeared in event JSON as `"aspa-state"` but never triggered accept/reject decisions. Operators needed configurable policy enforcement to reject routes whose AS_PATH fails ASPA verification, comparable to origin validation's hardcoded reject behavior.

## Decisions

- **Chose `log-only` as default for `aspa-invalid-action`** over `reject` (the RFC SHOULD): ASPA deployment is incomplete and false-Invalid results from missing ASPA records would silently drop traffic. Operators must explicitly opt into enforcement.
- **Chose atomic fields on `RPKIPlugin`** over retaining `rpkiConfig`: follows the existing `aspaEnabled` pattern. Config is parsed once and policy values are stored as `atomic.Uint32` for lock-free reads from the dispatch path.
- **Chose to reuse `validationRequest`/`validateCh` for re-validation reject** over direct dispatch in `handleASPAChange`: reuses the existing retry logic in `dispatchValidation()`. The re-validation request uses `state: ValidationValid` since origin state hasn't changed, and `aspaState` carries the new ASPA result.
- **Did not implement origin policy config parsing** (YANG leaves `invalid-action`/`not-found-action` exist but are unused in Go code): origin validation policy is hardcoded and changing it is a separate scope. The spec documents this explicitly.

## Consequences

- The ASPA override happens after origin validation in `dispatchValidation`: if origin rejects, ASPA is irrelevant; if origin accepts, ASPA can override to reject. This ordering means ASPA reject always wins over origin accept.
- Re-validation can reject but cannot un-reject: a route that was rejected by ASPA policy and then becomes Valid on cache update still needs a fresh UPDATE from the peer to be re-installed.
- The `rpki/policy` YANG container now has four leaves (two for origin, two for ASPA). Only the ASPA leaves are parsed by Go code.

## Gotchas

- The spec initially claimed origin policy parsing existed ("rpki_config.go parses policy/invalid-action, policy/not-found-action"). This was false: origin policy is hardcoded. The design section "mirrors existing pattern" was based on a wrong assumption. Caught during spec review before implementation.
- `rpkiConfig` is not retained on `RPKIPlugin` after `startSessions()`. New config values that need runtime access must use atomic fields.

## Files

- `internal/component/bgp/plugins/rpki/rpki_config.go` -- policy constants, config parsing
- `internal/component/bgp/plugins/rpki/rpki.go` -- policy dispatch, re-validation reject
- `internal/component/bgp/plugins/rpki/rpki_test.go` -- policy decision tests (new file)
- `internal/component/bgp/plugins/rpki/rpki_config_test.go` -- config parsing tests
- `internal/component/bgp/plugins/rpki/yang/ze-rpki.yang` -- new YANG leaves
- `test/plugin/rpki-aspa-policy-reject.ci` -- functional test (new)
- `test/plugin/rpki-aspa-policy-logonly.ci` -- functional test (new)
- `test/plugin/rpki-aspa-policy-unknown-reject.ci` -- functional test (new)
- `docs/guide/rpki.md` -- documentation update
