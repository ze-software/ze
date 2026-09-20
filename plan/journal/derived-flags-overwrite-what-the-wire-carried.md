# Derived flags overwrite what the wire carried

An attribute's flag octet is recomputed from its TYPE when the attribute is
written back out, rather than kept from the octet the peer sent. Every value the
type implies survives. Every value the PATH carried is lost, and no test sees
it, because the encoder and the decoder agree with each other about what the
flags "should" be.

The tell is a `Flags()` method that returns a constant.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-09-20 | - (walked into while giving RFC 8669 BGP Prefix-SID a parser, for the ExaBGP compatibility suite) | `Flags()` on every known optional-transitive attribute in `internal/core/bgp/attribute`, re-encoded through `AttributesSizeWithContext` and `WriteAttrToWithContext` (`internal/component/bgp/rib/commit.go`) | RFC 4271 Section 5 states it as a MUST NOT: "If a path with a recognized, transitive optional attribute is accepted and passed along to other BGP peers and the Partial bit in the Attribute Flags octet is set to 1 by some previous AS, it MUST NOT be set back to 0 by the current AS." Ze sets it back to 0. Each known attribute's `Flags()` answers a constant derived from its type, so a re-encode writes `0xC0` whatever arrived, and the Partial bit an upstream AS set because IT did not recognize the attribute is erased at Ze. The bit records a fact about the PATH, not about the attribute, so nothing in the type can reproduce it. It binds AGGREGATOR (7), COMMUNITIES (8), EXT_COMMUNITIES (16), TUNNEL_ENCAP (23), IPV6_EXT_COMMUNITIES (25), LARGE_COMMUNITIES (32) and now PREFIX_SID (40). `OpaqueAttribute`, which every UNRECOGNIZED attribute still decodes to, keeps the received octet, which is why the defect is invisible until a code gains a parser: attribute 40 had the correct behavior this morning and lost it when it stopped being unknown | not fixed, and not folded into the work in hand: it is class-wide, it is reachable through the RIB commit path rather than through anything the ExaBGP suite exercises, and `ai/rules/rule-precedence.md` keeps an unrelated repair out of a closing change. The repair is one field: a parsed attribute carries the flag octet it was built from, and `Flags()` answers that octet with the type's value as the default for a locally originated attribute. `parseSpan` already has the received octet in hand when it builds the value. Proving it wants a test per attribute that receives Partial set and asserts it survives the re-encode, in both polarities |
