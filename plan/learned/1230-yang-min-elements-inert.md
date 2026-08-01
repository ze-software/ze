# YANG leaf-list min-elements was inert; the count-0 case was the hole

`ze config validate` declared leaf-list `min-elements` in the schema but enforced
nothing: `walkTree` (`internal/component/config/yang/validator.go`) guarded its
`checkCardinality` call with `if len(items) > 0`, and **zero is the one count that
violates a minimum** -- so the single count that should reject was the single count
skipped. `max-elements` was unaffected (an over-count is necessarily non-empty),
which is why the max-side coverage passed while the min side enforced nothing.

The fix has TWO halves; shipping only the first is a half-fix:
1. **Present-but-empty** (`foo [ ]`): hoist `checkCardinality` out of the length
   guard so a present leaf-list with count 0 is checked.
2. **Absent entirely** (`foo` omitted): a leaf-list absent from the data never
   reaches the value loop, so add a scan over `entry.Dir` for
   `child.IsLeafList() && child.ListAttr.MinElements > 0 && data[name] absent`.
   The obvious assumption "absence is the mandatory check's job" is FALSE at the
   producer: goyang's `case *LeafList` synthesizes a Leaf WITHOUT copying
   Mandatory, and `*LeafList` has no Mandatory field, so `Entry.Mandatory` is
   `TSUnset` forever for a leaf-list -- the mandatory loop can never fire on one.
   Omitting the leaf entirely is also the shape an operator is most likely to write.

The two halves are mutually exclusive per leaf-list (absent -> only the Dir scan;
present -> only the value loop), so there is no double-reporting.

## Scope boundaries confirmed at the source (not assumed)

- **LIST branch** has the identical count-0 shape but ZERO live triggers: a grep
  proved the only `min-elements` declarations in the whole tree are three
  **leaf-lists** (`ze-vrrp-conf.yang:56,165`, `ze-tacacs-conf.yang:91`); no `list`
  declares `min-elements`. Documented as theoretical, not fixed.
- **TACACS `profile`** stays inert at `ze config validate` (`system`/`tacacs` is not
  in `yangSectionsToValidate`), but that is SAFE: a runtime backstop fails closed at
  `internal/component/tacacs/authenticator.go:105-110` (a priv-level mapped to zero
  profiles is REJECTED, not escalated to BuiltinAdminProfile).
- **Severity asymmetry** (pre-existing, deliberately excluded here): an absent `mandatory` leaf
  WARNS (`cmd_validate.go` `ErrTypeMissing`->Warning) but an absent min-bounded
  leaf-list REJECTS (`ErrTypeCardinality`->Error), though both mean "required". The
  reject direction is the defensible one and no config is *spuriously* rejected.

## The coverage trap that hid it, and a review trap

The defect hid because every test drove `checkCardinality` DIRECTLY or tested only
`max-elements`; the walkTree-through path for count 0 had no test. The proving test
must go through `walkTree` (`TestWalkTreeLeafListCardinality`, and end-to-end
`test/parse/vrrp-virtual-address-min-elements.ci`). Review trap worth remembering:
`test/parse/sysctl-profile-max-elements.ci` proves the earlier `max-elements`
leaf-list commit (`0305077f9`), NOT this min-elements diff -- a green `.ci` that
looks like proof but exercises a different, already-shipped path.

R-5 boundary: this fix touches ONLY `ze config validate`; the daemon load path still
runs no YANG cardinality validation, so a saved config that violates a bound still
boots. Closing that is a separate spec.

## Files

None recorded.
