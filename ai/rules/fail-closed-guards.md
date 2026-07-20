# Fail-Closed Guards

**When:** A guard must fail closed or say something
**Severity:** blocking
**Related:** exact-or-reject, no-fabrication

## Directives

A guard must fail closed or say something. Silent degradation into a permissive no-op is the bug, and a zero value that downstream reads as a legitimate answer is how it hides.

## Rule

A guard is any code whose purpose is to reject: an authorization check, a
cardinality constraint, a capability lookup, a ratchet, a validator.

| Requirement | Meaning |
|-------------|---------|
| Fail closed | On a miss, an unmapped input, an empty set, or an error, deny. Never fall through to the permissive branch. |
| Or say something | A guard that genuinely cannot deny MUST log, error, or fail its gate. A guard that neither denies nor speaks does not exist. |
| Make the miss explicit at the producer | Do not rely on a downstream layer to notice. The layer that knows it missed is the only one that can say so. |

## The zero-value trap

A zero value must never be a valid-looking answer. Where a lookup's miss returns
a zero that downstream reads as a legitimate outcome (allow, match-nothing,
success, count-of-1), the miss is invisible at every later layer.

Go's bare map read is the archetype: `m[k]` on an absent key yields the zero
value with no signal. Prefer `v, ok := m[k]` and handle `!ok` explicitly.

**A present-but-empty value passes `ok`.** `ok` proves the key exists, not that
the value is usable. When empty is also wrong, check `!ok || len(v) == 0`.

## Test corollary

**Drive the guard from the entry point that triggers it.** A unit test on the
guard helper proves the helper is correct. It proves nothing about whether the
caller ever reaches it with the input that matters.

A green unit test on an uncalled guard is worse than no test: it stops the
question being asked. `TestCheckCardinality`
(`internal/component/config/yang/validator_test.go:35`) passes, including its
count-0 row, while `walkTree` (`internal/component/config/yang/validator.go:631`)
iterates only present keys and `:669` skips non-strings, so leaf-list
`min-elements` is only ever handed exactly 1 and can never reject.

Test the shape that should be rejected, not only the shapes that work.

## Evidence corollary

**A doc or comment asserting a safety property is not evidence the property
holds.** Read the producing function. This is `ai/rules/no-fabrication.md`
applied to safety claims, where the cost of a reassuring wrong answer is highest:
the reader asks the right question, gets the doc's answer, and stops.

Beware the guard that works where you spot-check it and not where it matters.
Cardinality visibly rejects on `list` nodes, which is exactly why "YANG checks
cardinality" survives a probe and is false for leaf-lists.

## Worked example

`authz.Store.Authorize` (`internal/component/authz/authz.go:385-390`): with no
assignment and no config users (`hasUsers == false`) it returns
`BuiltinAdminProfile()`. An empty profile set is indistinguishable from "never
seen", because `aaa.RecordLoginProfiles`
(`internal/component/aaa/login_profiles.go:45-48`) early-returns on
`len(profiles) == 0` and records nothing. The zero value meant ADMIN: two live
privilege escalations, via TACACS+ and RADIUS. `docs/guide/radius.md` asserted
the opposite as fact. Fixed in `ff87bf61a`.

Four more instances of the same shape, found in one day:
`plan/learned/1157-fail-open-auth-empty-profiles.md`.
