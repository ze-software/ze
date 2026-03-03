# 254 — RFC 7606 Enforcement

## Objective

Close the gap between RFC 7606 error detection and enforcement: move validation before callback dispatch, apply treat-as-withdraw and session-reset correctly.

## Decisions

- Iota ordering: None / AttributeDiscard / TreatAsWithdraw / SessionReset — numeric comparison gives correct strength ordering (higher = more severe).
- `attribute-discard` does NOT strip wire bytes — zero-copy constraint makes stripping impossible; motivated `draft-mangin-idr-attr-discard-00` ATTR_DISCARD path attribute proposal.
- `fsm.EventUpdateMsg` must fire even for treat-as-withdraw — RFC 4271 §8.2.2 requires it for all UPDATEs.
- NLRI overrun → session-reset (not treat-as-withdraw); treat-as-withdraw requires the NLRI field to be fully parseable.

## Patterns

- Validation before dispatch: validate UPDATE error action, then dispatch (or suppress) callback — never dispatch then validate.
- Severity iota: strength ordering via integer comparison; append-only (new levels go after existing, never reorder).

## Gotchas

- attribute-discard and zero-copy are incompatible: stripping bytes from wire encoding breaks zero-copy forwarding. The solution is a new path attribute, not in-place stripping.
- NLRI overrun is not a treat-as-withdraw condition — the NLRI is not parseable, so individual prefix withdrawal is not possible; must session-reset.

## Files

- `internal/plugins/bgp/reactor/` — RFC 7606 enforcement point (before callback dispatch)
- `internal/plugins/bgp/message/` — error action iota
