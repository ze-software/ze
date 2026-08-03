# 1157 -- an empty profile set authenticated as admin (TACACS+ and RADIUS)

Two live, config-reachable privilege escalations, closed together because they
are one bug with two front doors. Specs: `spec-fixit-tacacs-empty-profile-mapping`,
`spec-fixit-radius-empty-profile-mapping`. Both were found while *designing* an
unrelated spec (`spec-fixit-authz-admin-fallthrough`), not by looking for them.

## The bug

An authenticated user whose profile set resolved to EMPTY was granted admin.

`aaa.RecordLoginProfiles` (`login_profiles.go`) early-returns on
`len(profiles) == 0`, so an empty set records **nothing** -- not an empty entry,
*no* entry. `authz.Store.Authorize` then finds no assignment and no login
profiles and, when no config user exists (`hasUsers == false`), returns
`BuiltinAdminProfile()` (`authz.go`). "No profiles" and "never seen"
are indistinguishable downstream, and the latter means admin.

Two ways in, both reaching that same tail:

- **TACACS+**: `handlePass` (`authenticator.go`) did `profiles, ok := a.privLvlMap[privLvl]`.
  A level mapped to an *empty* list is still PRESENT, so `ok` is true and the
  unmapped-denies guard is skipped. Needs an operator to write
  `tacacs-profile { level 15; }` with no `profile` entries -- which the YANG accepts.
- **RADIUS**: `mapProfiles` (`authenticator.go`) falls back to
  `defaultProfiles`, nil when `default-profile` is unset. An Access-Accept
  carrying no `Filter-Id` then authenticates with nothing. Needs **no
  misconfiguration at all**: it is the default config shape.

## The fix (both)

One invariant, enforced at the authenticator: **authenticated implies at least
one profile**. `if !ok || len(profiles) == 0` (tacacs) / `if len(profiles) == 0`
(radius) -> WARN + `aaa.ErrAuthRejected`, reusing each component's existing
reject path. `len()==0` also covers a second route into the empty shape:
`Tree.GetSlice` (`tree.go`) returns nil when a leaf-list is absent **or
when every member is deactivated**.

The general case (what a profile-less user gets when the set is empty for any
*other* reason) is a policy question answered separately: user decided
2026-07-16 that it is **denied, always**, tracked in
`spec-fixit-authz-admin-fallthrough`. These two fixes enforce that invariant
locally; `Authorize` still owes it globally.

## Why it survived, which is the reusable part

**The artifact that should have caught it was the reason it did not.**

- `docs/guide/radius.md` stated the safety property as fact: a profile-less user
  "is authenticated with no profiles **(and RBAC denies privileged actions)**".
  The parenthetical was false -- RBAC *granted* admin. Anyone who asked the right
  question got a reassuring wrong answer and stopped. The doc was the shield.
- YANG could not have caught either: `min-elements` never fires for count 0
  (`spec-fixit-yang-min-elements-inert`), so a code guard is the only defence.
- No test covered the empty shape. The tests that existed passed.

Corollary for reviews: a **zero value that is a valid-looking answer** is the
shape to hunt. `len(profiles) == 0` meaning "never seen" meaning "admin" is the
same shape as `EventTypeID(0)` matching nothing, and as `min-elements` being
handed a count that is never 0. None of them log. Each looks correct locally.

## Guidance

- When a lookup's zero value is indistinguishable from a legitimate answer, make
  the guard explicit at the producer. Do not rely on a downstream layer to notice.
- Test the *shape that should be rejected*, not only the shapes that work. Both
  fixes were proven by a RED that authenticated an empty set, and an invariant
  test (`Authenticated => len(Profiles) > 0`) that pins it for every future shape.
- **A doc asserting a safety property is not evidence the property holds.** If a
  guide claims a check denies something, read the producer before believing it.
- Mirror the *shape* of a sibling fix, not its letter: RADIUS deliberately did
  NOT copy tacacs's `min-elements 1`, because `default-profile` is optional by
  design and the constraint would have declared it mandatory.

## Files

None recorded.
