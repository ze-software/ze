# ZeBGP vs ExaBGP Behavioral Differences

This file documents intentional differences between ZeBGP and ExaBGP behavior.
These are not bugs - they are design decisions where ZeBGP diverges from ExaBGP.

**Impact on testing:** When `.ci` files from ExaBGP tests don't match ZeBGP output
due to these differences, update the `.ci` files to match ZeBGP's behavior.

---

## Attribute Ordering in UPDATE Messages

**ExaBGP behavior:**
- Sorts path attributes by type code before packing
- Order: ORIGIN(1), AS_PATH(2), NEXT_HOP(3), MED(4), LOCAL_PREF(5), ... MP_REACH_NLRI(14), ... LARGE_COMMUNITY(32)

**ZeBGP behavior:**
- Adds attributes in a fixed logical order during construction
- Order follows RFC 4271 Section 5 description order, then optional attributes
- MP_REACH_NLRI added after LARGE_COMMUNITY (for IPv6 routes)

**Example difference:**
```
ExaBGP:  ORIGIN, AS_PATH, LOCAL_PREF, MP_REACH_NLRI, LARGE_COMMUNITY
ZeBGP:   ORIGIN, AS_PATH, LOCAL_PREF, LARGE_COMMUNITY, MP_REACH_NLRI
```

**RFC compliance:**
- RFC 4271 does NOT mandate attribute ordering
- Both orderings are valid per specification
- Most BGP implementations accept any ordering

**Impact:**
- `.ci` test files may need updating to match ZeBGP's attribute order
- Wire bytes will differ but semantic meaning is identical

**Files affected:**
- `pkg/reactor/reactor.go` - `buildAnnounceUpdate()`
- `test/data/api/*.ci` - Expected message files

**Decision rationale:**
1. Fixed order is simpler to implement and maintain
2. No runtime sorting overhead
3. RFC-compliant (ordering is not mandated)
4. Peers must accept any valid ordering per RFC

---

## Template for Future Differences

### Feature Name

**ExaBGP behavior:** [Description]

**ZeBGP behavior:** [Description]

**RFC compliance:** [Analysis]

**Impact:** [Testing/compatibility notes]

**Files affected:** [List]

**Decision rationale:** [Why ZeBGP differs]
