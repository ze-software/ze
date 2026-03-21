# 410 -- Validate Completion

## Objective

Wire the existing `ze:validate` extension's `CompleteFn` into the CLI completer so that YANG leaves/leaf-lists annotated with `ze:validate` get tab-completion from their registered validator functions, instead of falling through to a generic `<value>` hint.

## Decisions

- `ValidatorRegistry` created fresh in `NewCompleter()` via `config.RegisterValidators()` -- same pattern as `ConfigValidator`
- `validateCompletions()` inserted after enum/bool checks, before generic hint -- CompleteFn takes priority over type hints
- Pipe-separated validators union their CompleteFn results with dedup
- `ReceiveEventValidator` queries `plugin.ValidBgpEvents` dynamically at call time (not at creation)
- `SendMessageValidator` combines hardcoded base types (`update`, `refresh`) with `plugin.ValidSendTypes`
- AC-2 (family list key completion) left partial: list keys use `listKeyCompletions()`, a different code path from `valueCompletions()` -- wiring ze:validate into list key completion would be a separate enhancement

## Patterns

- Validators with `CompleteFn` follow the same dynamic-query-at-call-time pattern as `AddressFamilyValidator`
- `GetValidateExtension()` and `SplitValidatorNames()` reused as-is -- no new YANG parsing code needed
- `TestCheckAllValidatorsRegistered_AllPresent` must be updated whenever new `ze:validate` names are added to YANG

## Gotchas

- YANG list keys (e.g., `family[name]`) go through `listKeyCompletions()`, not `valueCompletions()` -- ze:validate on a list key leaf does NOT automatically show CompleteFn results in the current architecture
- The auto-linter (`goimports`) removes imports added without usage in the same edit -- add import + first usage together
- `slicescontains` modernize lint requires both the `slices` import and the `slices.Contains` call in the same edit

## Files

- `internal/component/cli/completer.go` -- added `registry` field, `validateCompletions()` method
- `internal/component/cli/completer_test.go` -- 6 new tests (AC-1 through AC-7)
- `internal/component/config/validators.go` -- added `ReceiveEventValidator()`, `SendMessageValidator()`
- `internal/component/config/validators_register.go` -- registered 2 new validators
- `internal/component/bgp/schema/ze-bgp-conf.yang` -- added `ze:validate` to receive/send leaf-lists
- `internal/component/config/validator_yang_test.go` -- added 2 validators to exhaustive registration test
