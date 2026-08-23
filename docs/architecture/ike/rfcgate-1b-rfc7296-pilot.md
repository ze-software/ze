# Traffic-selector narrowing and transport-mode negotiation

Ze narrows IKEv2 traffic selectors from operator policy rather than accepting
whatever the peer proposes, and it negotiates transport mode as an explicit
notification. Both surfaces came out of the RFC 7296 extraction pilot, which
raised the enrolled requirement count for that RFC from 23 rows to 227.

## RFC obligations carried by this code

The requirement text below is quoted from `rfc/short/rfc7296.md`. Do not
paraphrase these into prose: the gate reads the id, the level and the section.

- `[RFC7296-2.9-1] [MUST]` Responder may narrow traffic selectors but never
  widen; if narrowed result is empty, respond with TS_UNACCEPTABLE (Section 2.9)
- `[RFC7296-2.9-2] [MUST]` If the responder's policy allows it to accept the
  first selector of TSi and TSr, then the responder MUST narrow the Traffic
  Selectors to a subset that includes the initiator's first choices (Section 2.9)
- `[RFC7296-2.9.2-1] [MUST NOT]` Thus, the new SA MUST NOT have narrower
  selectors than the original (Section 2.9.2)
- `[RFC7296-2.9.2-2] [MUST NOT]` The responder MUST NOT narrow down the Traffic
  Selectors narrower than the scope currently in use (Section 2.9.2)

Three more MUSTs that the extraction found, and that no gate could have asked
for because no summary listed them:

- `[RFC7296-3.2-5] [MUST]` The Critical bit MUST be set to zero if the sender
  wants the recipient to skip this payload if it does not understand the payload
  type code in the Next Payload field of the previous payload (Section 3.2)
- `[RFC7296-3.2-6] [MUST]` The Critical bit MUST be set to one if the sender
  wants the recipient to reject this entire message if it does not understand
  the payload type (Section 3.2)
- `[RFC7296-1.3.3-2] [MUST]` The REKEY_SA notification MUST be included in a
  CREATE_CHILD_SA exchange if the purpose of the exchange is to replace an
  existing ESP or AH SA (Section 1.3.3)
- `[RFC7296-4-5] [MUST]` Every implementation MUST be capable of doing
  four-message IKE_SA_INIT and IKE_AUTH exchanges establishing two SAs (one for
  IKE, one for ESP or AH) (Section 4)

<!-- source: internal/component/ike/engine/ts_narrow.go -- narrowSelectors, narrowChildSelectors, recordInitiatorSelectors, checkAnswerWithin -->
<!-- source: internal/component/ike/ipsec/traffic_selector.go -- TrafficSelectorPolicy, ValidateTrafficSelectors, validatePeerSelectors -->

## Decisions

**Narrowing is an intersection against operator policy, with a floor.**
`narrowSelectors` intersects what the peer proposed with the configured policy
pairs. `floorWithinProposal` keeps the floor inside the proposal, which is how
Section 2.9.2 is honored on a rekey: the new SA cannot come out narrower than
the scope in use.

**The initiator checks the answer, it does not trust it.**
`recordInitiatorSelectors` and `checkAnswerWithin` reject a response whose
selectors are wider than the proposal. `errTSWidened` is that refusal.

**The initiator INSTALLS the answer, on a rekey as well as on IKE_AUTH.**
Section 2.9 makes the response TS payloads the criteria for packets forwarded
over the new SA, so the answer is what the initiator programs.
`recordInitiatorSelectors` therefore takes the same rekey FLOOR that
`narrowChildSelectors` takes, and `coversFloor` refuses an answer below the
scope in use: Section 2.9.2 binds the new SA, so it binds the end that installs
one as much as the end that answers. `applyChildRekeyResponse`
(`internal/component/ike/engine/rekey.go`) passes that floor and hands the
recorded set to `newRekeyedChild`, which is the single place a replacement Child
SA takes its scope from. The floor is nil on IKE_AUTH, where no scope is in use
yet.

<!-- source: internal/component/ike/engine/rekey.go -- applyChildRekeyResponse, respondChildRekey, newRekeyedChild -->
<!-- source: internal/component/ike/engine/ts_narrow.go -- coversFloor, floorWithinProposal -->

RFC 7296 Section 2.21.3 decides what the initiator sends when it refuses:
nothing. "Because sending such error messages as an INFORMATIONAL exchange might
lead to further errors that could cause loops, such errors SHOULD NOT be sent."
The rekey is abandoned, both selector sets are logged, and the SA in use stays.

**A selector the dataplane cannot program is refused at config time.** The
config parser validates the port form and the single-host constraint, so an
unprogrammable policy is a config error rather than a runtime surprise.

<!-- source: internal/component/ike/ipsec/traffic_selector.go -- checkPortProgrammable, checkSingleHost, parsePortSelector -->
<!-- source: internal/component/ike/engine/ts_narrow.go -- programmablePair, programmableSelector -->

**Transport mode is an explicit notification, decided per role.** The initiator
sends USE_TRANSPORT_MODE when the peer's child mode asks for it, and records
whether the responder echoed it. The responder decides from its own config.
A mismatch deletes the SA rather than installing the wrong mode.

<!-- source: internal/component/ike/engine/transport_mode.go -- wantsTransportMode, decideResponderTransportMode, adoptAuthResponseNegotiation, recordInitiatorTransportMode -->

## Traps this code exists to avoid

**Ze ships fail-closed on EAP-TLS over TLS 1.2 without RFC 7627.** A build-wide
`go:debug tlsunsafeekm=1` directive weakened the export rule for every user to
suit one peer version. It was removed. The lab then opted in per scenario, until
Go 1.27 removed the setting from the toolchain: a removed key carrying its old
value is a fatal error raised before `main()`, so the opt-in stopped the daemon
instead of starting a session. Nothing opts in now, and there is no replacement.
Ze reports the refusal instead, naming the peer, the negotiated version, RFC 7627
and what an operator can change (`eapTLS12ExportRefused`).

**A green test suite can be measuring nothing.** Three findings from the pilot
are worth carrying:

| Shape | What was green | The tell |
|-------|----------------|----------|
| An interop check that swallows its own assertion | two `check.py` files wrapped the ESP assertion in a bare `except`, one calling the pass logger from the handler | remove the swallow and the scenario reds |
| A carrier green for an unrelated reason | `test/ipsec/ipsec-child-rekey.ci` passed while the rekey was broken, because it sets the dataplane to `noop` | the assertion never reaches the code the test names |
| A test asserting a call happened, not what reached the wire | five review rounds, two of which found the previous round's fixes inoperative | mutate the producer; a reviewer that only reads does not find this |

**Five defects were invisible to same-implementation testing.** A bare OID where
RFC 7427 wants an `AlgorithmIdentifier` SEQUENCE, which Ze's own verifier
accepts. RFC 2759's one-octet Success Response, of which Ze demanded four. A
discarded EAP-TLS closing flight. An MSK derived as `sha256(TLSUnique)`, which
no other implementation computes. A client certificate with no subjectAltName.
EAP-TLS had never once interoperated while its suite was green. A self-test
suite cannot find a defect that both ends share.

<!-- source: internal/component/ike/engine/auth.go -- algorithmIdentifierRSA, algorithmIdentifierEC, selectSignatureAlgorithm -->

## Related

- `rfc/short/rfc7296.md` carries the full requirement checklist.
- `rfc/extraction/rfc7296.json` carries the extraction sign-off.
- `docs/features/rfc-status.md` carries the public support claim.
