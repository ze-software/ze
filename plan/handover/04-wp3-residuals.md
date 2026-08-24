# 04 -- WP-3 residuals: one owner question, two blocked tests, one measurement

| Field | Value |
|-------|-------|
| Spec | the rfcgate-1b RFC 7296 pilot spec |
| Primary handover | `plan/handover/03-rfcgate-1b-pilot-after-wp8.md` -- read that first, it holds the queue |
| Adds | only what WP-3 left open. It supersedes nothing |
| Written | 2026-07-31 |

WP-3 landed complete in `87d749149`: fifteen rows, both polarities, 39 of 39 mutations
killed. `03-` already records that. Three things it does not record are below, and none of
them is discoverable from the code.

## 1. One owner question, and it is the only reason to read this file

RFC 7296 Section 2.21.2 lets a responder send IDr, CERT and AUTH beside an error notify for
a piggybacked exchange, and keep the IKE SA alive. Ze now sends the notify where it used to
send silence, and still tears the SA down.

That clause is a MAY, so no gated row is unproven. But it is the one place in this package
where Ze knowingly does less than the section offers. `ai/rules/rfc-compliance.md` makes the
question mandatory rather than optional. **Ask Thomas before WP-4 closes.**

The cost if he wants it: `finishResponderEstablish` (`engine/responder.go`) is the only path
that reaches `StateEstablished`, and it requires an installed Child SA. Keeping the IKE SA
alive means an establish path with no Child SA, which no state in the responder FSM
currently expresses. That is a real change, not a flag.

## 2. Two tests the WP-3 design asked for, and why neither exists

Both are blocked on the same fact, and a session that retries them will rediscover it.

| Test | Blocker |
|------|---------|
| `test/ipsec/ipsec-child-rekey-no-proposal.ci` | A static config cannot make IKE_AUTH succeed and the later CHILD REKEY fail. A disjoint `esp-group` fails `selectResponderESP` at IKE_AUTH, so the SA never establishes and the rekey never happens. `respondChildRekey` matches against `ESPGroup.Proposals[0]`, which `selectResponderESP` has already narrowed to the negotiated suite, so a two-proposal responder still matches |
| `test/interop-ipsec/scenarios/error-notifications/` | Same blocker, against strongSwan |

A session would need one of two things. A config surface that changes `esp-group` on a live
SA while the peer stays up, or a test seam that narrows `ESPGroup` after establishment.
Neither exists. The handler is covered instead by `TestErrRefusedChildRekeyIsAnswered`, which drives
the real handler and reads the real datagram off a real UDP socket.

Both are recorded in `plan/deferrals/rfcgate-1b-rfc7296-pilot.md` with this pilot as their
destination, because the pilot is still open and owns them.

## 3. A measurement that stops one investigation being repeated

`make ze-interop-ipsec-test` on this macOS host reports **2 passed, 8 failed**. That looks
alarming after a wire-parser change, and it is not a regression.

I rebuilt `ze` with all twelve WP-3 files reverted to HEAD through `go build -overlay` and
reran the lab. **Identical verdict, identical scenario list.** The eight failures are a
Docker-for-Mac limitation the lab's own output names: `XFRM not available on Ze (expected on
Docker for Mac), skipping ESP checks`. Scenario psk-site-to-site passes end to end with ESP counters
advancing in both directions, which is the real interop proof for the inner-chain parser.

Do not spend a session on those eight from macOS. `03-` already points at
`spec-rfcgate-2-deferred-unrun-interop-trees.md`, which is the real fix.

## 4. Two defects WP-3 fixed in passing, worth knowing

`ParsePayloadChain` (`wire/chain.go`) accepted a chain that named a next payload and
delivered none. It also treated one refused proposal as fatal where `Message.ReadFrom`
tolerates it. That broke RFC 7296 Section 3.3.6 for every SA payload in an inner chain, and
every CREATE_CHILD_SA and IKE_AUTH SA payload travels there.

**`Message.ReadFrom` must NOT gain the same dangling-next-payload check.** An Encrypted
payload is the last payload of an outer message, and its Next Payload names the first
payload INSIDE the ciphertext. Every well-formed protected message therefore ends the outer
chain with a non-zero Next Payload. The check is correct for the inner chain only.

## 5. The rules moved under this session

Three changes landed mid-session and are not in `03-`.

| Change | Effect |
|--------|--------|
| `ai/rules/writing.md` (new) | Replies under 15 lines, tables before prose. Cite `file.go` `Symbol`, not `file:line`, unless a gate pins the line |
| ASD-STE100 is advisory | `ste_problems` prints its findings and lets the commit through |
| `block-premature-stop.sh` re-registered on `Stop` | It fires again. A digest that still calls it inert is stale |
