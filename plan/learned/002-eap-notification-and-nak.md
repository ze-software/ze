# Learned: eap-notification-and-nak

RFC 3748 Section 5 obliges every EAP implementation to support Types 1-4. Ze
supported Type 1. The peer answered an Identity Request and one configured
method and returned an error for everything else, so an authenticator offering a
method Ze does not run got a dead IKE SA where the RFC requires a Type-3 Nak
naming the method Ze does run. This spec implemented Types 2, 3 and 4, corrected
the authenticator's Nak handling, and landed the rfc3748 extraction sign-off.

## What the design turned on

**One dispatcher, four outcomes, decided by Type before any method sees the
packet.** The peer's refusal path had one shape for four situations: an
unacceptable method, an out-of-order Request, a malformed method payload and a
Notification all returned an error, and `handleEAPResponse` reads any error as a
reason to kill the SA. The RFC gives each a different answer. Three of the four
belong to the EAP framework rather than to a method, which is why the Type is
settled above the dispatch and each method now reads only its own opcode.

**Section 2.1's "initial non-Nak Response" is the peer's first Response to a
METHOD Request, not the Identity Response.** The literal reading makes Section
5.3.1's Nak MUST unreachable in every conversation, and Section 5.4 describes a
Nak sent in answer to the Request that follows the Identity Response. The
boundary is a flag set when a method ANSWERS, never when a Request arrives: a
method that discarded or errored has sent nothing.

**A SHOULD NOT is not a MUST NOT, and the summary said otherwise.**
`RFC3748-7.10-3` read "Methods that do not generate MSK MUST NOT be used with
IKEv2 (S7.10, RFC 7296 S2.16)". RFC 3748 Section 7.10 does not mention IKE at
all, and RFC 7296 Section 2.16 states a SHOULD NOT and then specifies what to do
when such a method IS used: key the AUTH payloads from SK_pi and SK_pr. Our own
row was stricter than either source, and it was the row forbidding the feature
the owner had just ordered. A requirement summary is a derived artifact; when it
disagrees with the RFC text, the text wins.

**A vacuity proof expires when the feature it assumed absent gets built.**
`RFC7296-2.16-5` is conditional: IF a keyless method is used, THEN key AUTH from
SK_pi and SK_pr. It was discharged by a test proving the antecedent never fires,
whose own claim read "A keyless method never starts, which keeps the SK_pi and
SK_pr AUTH mode unreachable". Implementing the consequent made both halves false
while the test stayed green, because nothing ties a vacuity argument to the
condition it rests on.

## The trap worth carrying forward

**A zero value that GUARDS by accident.** `verifyRemoteAuth` gated an EAP peer's
AUTH payload on `sa.EAPMSK != [64]byte{}`. That test asks which key to sign
with. It also happened to answer whether the exchange had succeeded, because the
MSK stays zero until a method fills it. Nothing named the second job, no test
covered it, and the comment described only the first. MD5-Challenge derives no
key, so the MSK is legitimately zero forever, and the accidental guard would have
become an authentication bypass: SK_pi descends from SKEYSEED, which anyone who
completed IKE_SA_INIT holds. The replacement asks the exchange whether it
succeeded, on purpose.

Before writing a zero, nil, false or empty as a result, ask two questions. Can a
caller tell this from a failure? And is anything downstream relying on this value
being absent? A yes to the second means the zero is a guard, and a guard gets a
name, a comment and a test. This is now a section of
`docs/contributing/ze-go-style.md` and a sentence in
`ai/rules/points/principles/directives/a-wrong-value-must-not-look-like-a-right-one.md`.

## What the review caught that the authors did not

Three defects survived implementation and the authors' own reading, and an
independent three-lens pass found all three. Each is the same shape: the rewrite
was right and one arm kept the old behavior.

- `handleRequest`'s terminal-state arm still returned an error, so a Request
  after EAP-Success let one unauthenticated packet kill the SA. That is the exact
  defect the rest of the function was rewritten to remove, four lines into the
  rewrite.
- The authenticator answered ANY Type-3 Response with an EAP-Failure. Section 2.1
  says an authenticator receiving an UNEXPECTED Nak should discard it and log the
  event, precisely because spoofed Requests exist. The peer half got that
  treatment; the authenticator half did not.
- The accepted-method set lost its only test when two tests moved packages, so
  `NewSession`'s refusal path went unswept and Types 5 and 6 were unpinned.

## Deviations from plan

**A-2 was broken.** The spec assumed the strongSwan image ships `eap-dynamic`,
the only charon plugin that answers a Nak by offering another method. It does
not, verified by listing `/usr/lib/ipsec/plugins` in the built image. So the
interop scenario proves strongSwan PARSES Ze's Type-3 Nak as a Nak, and cannot
prove the method the Nak names is the one Ze then runs. That octet is proven by
`TestNakNamesTheConfiguredMethod` and by
`test/ipsec/ipsec-eap-nak-unacceptable-type.ci`, where Ze's own authenticator
names it. An assumption that fails is worth more than one that holds: it moved a
published claim off a scenario that could not carry it.

**D-1 dissolved rather than being answered.** The spec asked how an extraction
sign-off dispositions an owner-authorized deviation, because the closed
exclusion-kind set has no word for one. The owner withdrew the deviation and
ordered the method built, so sites `5:2`, `5.4:1` and `5.4:2` became ordinary
mapped rows and no new kind was ever needed. A vocabulary question can be the
wrong question about a scope decision nobody has taken yet.
