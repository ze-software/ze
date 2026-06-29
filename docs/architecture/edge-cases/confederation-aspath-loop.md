# Confederation + AS-Override: Endless AS_PATH Loop

**Reference:** [BGP AS-Path Infinite Loop](https://vincent.bernat.ch/en/blog/2024-bgp-endless-aspath) (Vincent Bernat, 2024)
**CVE:** CVE-2025-20115 (Cisco IOS XR crash from unbounded AS_PATH growth)
**Purpose:** Document the vulnerability, Ze's exposure, and the defenses in place

---

## The Problem

BGP confederations (RFC 5065) split an AS into sub-ASes. Confederation segments
(AS_CONFED_SEQUENCE, AS_CONFED_SET) are not counted in path length for best-path
selection. This is by design: internal confederation hops should not penalize a
route compared to alternatives.

AS-override (`session > as-override`) replaces the peer's ASN with the local ASN
in outbound AS_PATH. This is commonly used when two sites share the same ASN and
would otherwise reject each other's routes via loop detection.

When both features are active in the same network, they can interact to defeat
loop detection entirely:

1. A route enters a confederation loop (e.g. R0 -> R1 -> R2 -> R3 -> R1).
2. AS-override at R3 replaces R1's ASN with R3's own, so R1 no longer sees
   itself in the path and accepts the route.
3. Confederation segments don't count toward path length, so the looping path
   ties with the original on length. The route can win on tiebreakers (router ID).
4. Each loop iteration appends more ASNs. The AS_PATH grows without bound until
   it hits BGP message size limits or crashes the implementation.

The Cisco CVE demonstrated that unbounded AS_PATH growth can crash routers that
allocate proportionally to path length without a cap.

## Ze's Exposure

**Ze does not implement BGP confederations as a speaker.** There is no
confederation-id, no member-as configuration, and no sub-AS session type. Ze
cannot create the internal confederation topology that the attack requires.

Ze does implement `as-override` (outbound AS_PATH rewriting in
`reactor_api_forward.go:applyASOverride`), but without confederation sessions the
specific loop mechanism cannot occur within Ze.

Ze does parse confederation segment types correctly when receiving routes from
peers that use confederations. AS_CONFED_SEQUENCE (3) and AS_CONFED_SET (4) are
recognized in `ParseASPath` and excluded from `PathLength()` per RFC 5065.

## Defenses

### MaxASPathTotalLength (parsing cap)

```
MaxASPathTotalLength = 1000
```
<!-- source: internal/core/bgp/attribute/aspath.go -- MaxASPathTotalLength -->

Any received AS_PATH with more than 1000 total ASNs (across all segments) is
rejected as malformed during parsing. This prevents the memory-exhaustion crash
that CVE-2025-20115 exploited, even if a peer is stuck in the infinite loop and
sends inflated paths to Ze.

Real-world AS_PATHs rarely exceed 50 ASNs. The 1000 cap is generous enough for
segment-splitting (255+255+...) while blocking pathological growth.

### Loop detection covers all segment types

The ingress loop filter (`reactor/filter/loop.go:LoopIngress`) iterates all
AS_PATH segments when checking for the local ASN, including AS_CONFED_SEQUENCE
and AS_CONFED_SET. The `Contains()` method on ASPath similarly checks every
segment type. There is no code path where confederation segments are skipped
during loop detection.

### allow-own-as is count-based

The `allow-own-as` knob (`peersettings.LoopAllowOwnAS`) sets a maximum count of
tolerated local-ASN occurrences, not a boolean bypass. With `allow-own-as 2`, the
route is accepted if the local ASN appears at most twice and rejected on the
third occurrence. This limits exposure even when the feature is enabled.

### Loop detection disable is explicit

Full loop-detection bypass requires setting `LoopDisabled` per-peer via config.
There is no implicit disable triggered by other features.

## Summary

| Defense | Protects against |
|---------|-----------------|
| No confederation speaker | Cannot create the loop topology internally |
| MaxASPathTotalLength=1000 | Crash/DoS from inflated paths received from peers |
| Loop check on all segment types | Confederation ASNs in received paths are not invisible to loop detection |
| Count-based allow-own-as | Partial loop tolerance does not become full bypass |
