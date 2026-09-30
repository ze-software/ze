# reactor author B handoff (child spec-rfc-verdict-fix-bgp, package internal/component/bgp/reactor, continuation B)

Scope: reactor verdicts outside rfc4271/5549/8950/9830 (author A holds those), plus three re-records.
Listing at start: 46 weak (my stems). Blocked: DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-2, RFC6286-2.1-1. Worked list: 44.
No verdict stamped. Nothing committed. No existing tagged unit edited. All new tests are in reactor_b_*_test.go.

(in progress; table below is updated as verdicts finish)

## Resolved

| id | resolution | what now proves each clause (+/-) | records written | expected verdict | notes |
|----|-----------|------------------------------------|-----------------|------------------|-------|

## Re-records (producer-changed)

| id | result |
|----|--------|

## Unresolved

## Files changed

## Copied from the per-spec state file (authoritative handoff, author B)

- Handoff file children/bgp/reactor-B-author.md holds only the early skeleton; this entry is the authoritative handoff.
- New units, all green under ./le job run (scratch/b-t3.log) except rfc4724 (runs inside discriminate-record): reactor_b_rfc7611_test.go (RFC7611-2.2-1 +/-), reactor_b_rfc7911_test.go (RFC7911-5-5 +/- per-family), reactor_b_open_caps_test.go (RFC2918-2-1 +, BFD-STRICT-MODE-6-1 +/- on sent OPEN octets), reactor_b_rfc2918_test.go (RFC2918-3-4 + sender octet 0), reactor_b_rfc5492_test.go (RFC5492-3-1 +/-, 5-1 +/- wire Data), reactor_b_rfc9687_test.go (RFC9687-4.3-8 + four writers, 4.3-10 + notification/tcp-close), reactor_b_rfc7705_test.go (RFC7705-4.2-2 +/- to Established, iBGP), reactor_b_rfc4724_test.go (RFC4724-4-1 -, 4.2-9 - EoR waits for initial update).
- Records: children/bgp/b-records.sh running in background, log children/bgp/b-records.log. Continuation MUST read it: every rc=0 line is a written record; rc!=0 needs a fix (likely: ambiguous producer capability.go::WriteTo and message/routerefresh.go::WriteTo).
- Re-records: RFC7947-2.2.2.2-1 +/- done (revert LoopIngress). RFC7611-2.1-1 and RFC7705-3.3-1 retried in b-records.sh (first try failed: build broken by full cache disk / other session L2TP edit).
- rfc9687_test.go was edited and restored (net zero). No existing tagged unit edited; D-15 approvals recorded only for my own new units.
- Unresolved (32): ABRAITIS-3-7, BFD-STRICT 10-2, 4-1, LINKLOCAL 3-2, 4-1, 4-3, 4-4, 4-7, 4-8, 4-9, RFC2385-2.0-4, 2.0-5, RFC4659-3.4-1, RFC4760-7-1, RFC6286-2.1-2, RFC7311-3.4.3-2, -6, -7, RFC7313-4-3, 5-3, RFC7432-8.2.1-9, RFC7705-3.3-2, 4.2-4, RFC7911-5-3, 5-4, RFC7947-2.2-1, x-4, RFC9252-3.3-1, RFC9552-5.1-5, 5.2.1.4-1, 8.2.2-9, RFC9687-4.3-3.
- Gates owed: ./le go lint run (reactor tests), ./le rfc check for the touched stems. Cache disk hit 100% once; ./le scratch cache-clean freed it.

- b-records.sh finished; outcome lines (== / rc):
  == id RFC7611-2.2-1 polarity positive	rc=0
  == id RFC7611-2.2-1 polarity negative	rc=0
  == id RFC7911-5-5 polarity positive	rc=0
  == id RFC7911-5-5 polarity negative	rc=0
  == id RFC2918-2-1 polarity positive	rc=2
  == id DRAFT-IETF-IDR-BGP-BFD-STRICT-MODE-6-1 polarity positive	rc=0
  == id DRAFT-IETF-IDR-BGP-BFD-STRICT-MODE-6-1 polarity negative	rc=0
  == id RFC2918-3-4 polarity positive	rc=0
  == id RFC5492-3-1 polarity positive	rc=0
  == id RFC5492-3-1 polarity negative	rc=0
  == id RFC5492-5-1 polarity positive	rc=0
  == id RFC5492-5-1 polarity negative	rc=0
  == id RFC9687-4.3-8 polarity positive	rc=0
  == id RFC9687-4.3-10 polarity positive	rc=0
  == id RFC7705-4.2-2 polarity positive	rc=0
  == id RFC7705-4.2-2 polarity negative	rc=0
  == id RFC7611-2.1-1 polarity negative	rc=0
  == id RFC7611-2.1-1 polarity positive	rc=0
  == id RFC7705-3.3-1 polarity negative	rc=0
  == id RFC7705-3.3-1 polarity positive	rc=0
  == id RFC4724-4-1 polarity negative	rc=0
  == id RFC4724-4.2-9 polarity negative	rc=0


## Continuation C (2026-09-29)

### Records
| id | polarity | unit | route / producer | result |
|----|----------|------|------------------|--------|
| RFC2918-2-1 | + | reactor_b_open_caps_test.go::TestRFC2918RouteRefreshCapabilityOctetsInSentOpen | revert session_negotiate.go::buildOpen (capability.go::WriteTo is ambiguous: the revert route matches the bare func name, 13 WriteTo methods; no gomu report exists for a mutant) | observed red, written |
| RFC7611-2.1-1, RFC7705-3.3-1, RFC7947-2.2.2.2-1 | +/- | (re-records) | written by b-records.sh / B | `./le rfc discriminate id` shows no stale, no unproven |
| RFC7313-5-3 | + and - | reactor_b_rfc7313_test.go::TestRouteRefreshUnknownSubtypeNeverDelivered | revert session_handlers.go::routeRefreshSubtypeUnknown | observed red, written (the negative's break is the panic revert, not an ignore-all mutant: no gomu) |
| RFC7313-5-3 + / RFC7313-5-1 - | | rfc7313_error_scope_test.go::TestRouteRefreshUnknownSubtypeIsIgnoredWhateverItsLength (edited, D-15 approved) | revert routeRefreshSubtypeUnknown / validateRouteRefreshLength | observed red, written |

### Race
`go test -race -count=1` reactor package under ./le job run: ok 149.3s, 0 DATA RACE (job-unit-pkg-cd10b99a.log). Ran before the RFC7313 fix; after the fix the refresh subset (-run 'RouteRefresh|Refresh', no race) is green (job-unit-pkg-31b86b12.log).

### RFC7313-5-3: D-8 defect, FIXED
Producer: screenRouteRefresh (session_handlers.go) ignored an unknown subtype only when the body length was wrong; a 4-octet unknown subtype passed the screen and session_read.go delivered it to onMessageReceived (every plugin/subscriber) and onRefreshRecv, and only handleRouteRefresh dropped it afterwards. Fix: new routeRefreshSubtypeUnknown (RFC quote above it, logs the error §5 asks for) called from screenRouteRefresh; the dead subtype branches in handleRouteRefresh deleted. Failing test first: TestRouteRefreshUnknownSubtypeNeverDelivered red on subtypes 3/5/42/255 (job-unit-pkg-037cd6f6.log), green after. Existing unit TestRouteRefreshUnknownSubtypeIsIgnoredWhateverItsLength asserted delivered=1 for the two well-formed unknown cases: corrected to 0 (a test wrong about what it asserts). Docs: fsm-established.md, wire/messages.md.
Expected verdict: strong for 5-3 on the new unit. Old session_test.go 5-3 tags remain unproven (pre-existing, unedited).

### RFC9687-4.3-3: tests (partial)
New TestRFC9687Event29ReleasesHoldTimerAndConnection (reactor_b_rfc9687_test.go): + after Event 29 the HoldTimer and KeepaliveTimer are stopped and the session holds no TCP connection; - one nanosecond short, all three are held. Records +/- written (revert session_write.go::sendHoldTimerExpired). Expected verdict: still weak on "routes learned over the connection" (released through Peer state -> RIB plugin, outside a session unit); needs a functional/Peer-level proof or a row narrowing (the row is the list item "releases all BGP resources").

### Post-fix race run
Full reactor package `go test -race -count=1` after every edit above: ok 148.7s, no DATA RACE (job-unit-pkg-cd10b99a.log, overwritten per identical command).

### Still unresolved (29 of B's 32; worked this pass: 7313-5-3 fixed, 9687-4.3-3 partial)
ABRAITIS-3-7, BFD-STRICT 10-2, 4-1, LINKLOCAL 3-2 (next step: assert the MP_REACH Next Hop Length octet on the wire; applyFactsNextHop emits op 14 with f.nhGlobalLL[:], no wire applier helper in reactor tests), 4-1, 4-3, 4-4, 4-7, 4-8, 4-9, RFC2385-2.0-4, 2.0-5, RFC4659-3.4-1, RFC4760-7-1, RFC6286-2.1-2, RFC7311-3.4.3-2, -6, -7, RFC7313-4-3 (the reactor half is now proven by TestRouteRefreshUnknownSubtypeNeverDelivered's outcomes differing by subtype; the "appropriate actions" for BoRR/EoRR live in the rib plugin, so a tag here alone would not close it), RFC7432-8.2.1-9, RFC7705-3.3-2, 4.2-4, RFC7911-5-3, 5-4, RFC7947-2.2-1, x-4, RFC9252-3.3-1, RFC9552-5.1-5, 5.2.1.4-1 (note says the sender code violates it: NodeDescriptor.WriteTo emits one sub-TLV 518 per SID, a D-8 candidate in nlri/ls, another package), 8.2.2-9.

### Files changed in continuation C
- internal/component/bgp/reactor/session_handlers.go (routeRefreshSubtypeUnknown added, called from screenRouteRefresh; dead subtype branches removed from handleRouteRefresh)
- internal/component/bgp/reactor/reactor_b_rfc7313_test.go (new)
- internal/component/bgp/reactor/rfc7313_error_scope_test.go (two expectations 1 -> 0, one comment; D-15 approval recorded)
- internal/component/bgp/reactor/reactor_b_rfc9687_test.go (new function + helper appended)
- docs/architecture/behavior/fsm-established.md, docs/architecture/wire/messages.md
- rfc/discrimination/rfc2918.json, rfc7313.json, rfc9687.json (records)

### Gates owed (not run here)
./le go lint run (reactor), ./le rfc check for rfc2918/rfc7313/rfc9687 and B's stems. Nothing stamped, nothing committed.
