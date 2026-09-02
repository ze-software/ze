#!/usr/bin/env python3
"""Replayable classification of the RFC 3748 extraction sign-off.

Reads the skeleton `./le rfc extraction-create stem rfc3748` writes into the
session scratch, applies one disposition per site and per section, and writes
the file back. Sites with no honest disposition are left null on purpose: each
one is reported to the owner rather than annotated away.

SUPERSEDED IN PART, 2026-09-01, AND NO LONGER SAFE TO REPLAY OVER THE LANDED
ARTIFACT. `rfc/extraction/rfc3748.json` has since been edited in place by
`plan/spec-eap-notification-and-nak.md`, and a replay would overwrite that. Ten
sites now hold dispositions this script does not know: `5.2:1`, `5.2:2`,
`5.3.1:1` through `5.3.1:4`, `5:2`, `5.4:1` and `5.4:2` are `mapped`, and
`5.2:5` and `5.7:2` are `duplicate-of`. The D-1 routing below is void: Thomas
withdrew the 2026-08-30 deviation on 2026-09-01 and ordered MD5-Challenge
implemented, so no exclusion kind for an authorized deviation was ever needed.
`5.2:4` is the one site still routed correctly here, and it waits on D-2.

Second pass, 2026-09-01. Three changes, and each is a correction rather than an
addition:

1. Four sites became mappings, because the obligations they state were
   implemented: `4:1`, `4.2:5`, `4.2:6` and `4.2:12` now map `RFC3748-4-5`,
   `RFC3748-4.2-8`, `RFC3748-4.2-7` and `RFC3748-4.2-9`.

2. Eighteen sites moved from `binds-another-role` to `feature-out-of-scope`.
   `binds-another-role` is presumed wrong (`ai/rules/CORE.md`) and it was wrong
   here: each of the eighteen names a role Ze DOES play, declining an option the
   RFC states in its own words. Ze is an authenticator that does not pass
   through ("Support for pass-through is optional", Section 2) and a peer that
   offers neither Type 5, Type 6 nor Type 254 ("Implementations MAY support
   other Types defined here or in future RFCs", Section 5). Saying another role
   is bound would have published "not our problem" where the truth is "our
   problem, declined".

3. Sixteen keep `binds-another-role`, and every reason now names the role as a
   DOCUMENT-authoring one and cites the producer that would act as it if Ze did.
   Fifteen bind the author of an EAP METHOD SPECIFICATION; one binds a PPP
   implementation that negotiated EAP, which `authMethodFromAuthProto`
   (`internal/component/l2tp/ppp/auth.go`) shows Ze is not.

Twenty-two sites still carry no honest disposition, and the report names each.
Fifteen state obligations Ze MEETS that no checklist row declares, which is a
package of rows and tagged tests rather than a classification. Seven are blocked
on a decision that is not this walk's: D-1 and D-2 of
`plan/spec-eap-notification-and-nak.md`, and two obligations whose implementation
would red an RFC-tagged test that pins the current behavior.
"""
import json, sys

path = sys.argv[1]
doc = json.load(open(path))

SPEC = "plan/spec-eap-notification-and-nak.md"

# site id -> ("mapped", id) | ("excluded", kind, reason) | ("relocated", reserved-id, reason)
M = {}
def mapped(sid, rid): M[sid] = ("mapped", rid)
def excl(sid, kind, reason, mapped_to=None): M[sid] = ("excluded", kind, reason, mapped_to)
def reloc(sid, rid, reason): M[sid] = ("relocated", rid, reason)

# The two sentences that make a feature optional, quoted verbatim from
# rfc/full/rfc3748.txt. feature-out-of-scope requires the quote, so it is spelled
# once and every reason that rests on it names the same words.
PASSTHROUGH_OPTIONAL = (
    "RFC 3748 Section 2 states the option in its own words: 'Network Access Server (NAS) "
    "devices (e.g., a switch or access point) do not have to understand each authentication "
    "method and MAY act as a pass-through agent for a backend authentication server.  Support "
    "for pass-through is optional.'")
OTHER_TYPES_OPTIONAL = (
    "RFC 3748 Section 5 states the option in its own words: 'All EAP implementations MUST "
    "support Types 1-4, which are defined in this document, and SHOULD support Type 254.  "
    "Implementations MAY support other Types defined here or in future RFCs.'")

# The scope decision every feature-out-of-scope reason here names, and the
# producer that shows it. NewSession is the one gate on which EAP methods exist.
NEWSESSION = ("NewSession (internal/component/ike/eap/eap.go) builds a method for Type 13 and "
              "Type 26 and refuses every other type, so no other Type is offered")
LOCAL_ONLY = ("NewSession (internal/component/ike/eap/eap.go) builds the method in process and "
              "the IKE engine has no AAA back end for EAP: nothing under "
              "internal/component/radius/ produces an EAP-Message attribute, and "
              "handleResponderEAP (internal/component/ike/engine/responder_eap.go) answers from "
              "the local eap.Session alone")
DISCLOSED = ("The absent feature is disclosed in docs/features/rfc-status.md as an "
             "implementation gap a later scope decision can revisit, never as a conformance "
             "gap.")

def scope(sid, feature, obligation, quote, producer):
    """An obligation conditional on an OPTIONAL feature Ze does not offer."""
    excl(sid, "feature-out-of-scope",
         "Conditional on " + feature + ", which Ze does not offer. The sentence states " +
         obligation + ". " + quote + " " + producer + ". " + DISCLOSED)

# --- Section 1.2 -----------------------------------------------------------
excl("1.2:1", "not-a-requirement",
     "Section 1.2 is the Terminology list and the sentence sits inside the definition ENTRY "
     "for 'Displayable Message', fixing what the term means wherever a later section uses it. "
     "The obligations that put a displayable message on the wire are stated at Sections 5.1, "
     "5.2, 5.5 and 5.6, and those sites carry their own dispositions.")

# --- Section 2 -------------------------------------------------------------
# 2:1 left null: see RESIDUAL.
excl("2:2", "duplicate-of",
     "Restates the Section 2.1 obligation to close the conversation with a Success or a Failure, "
     "for the Failure arm. Site 4.2:1 maps the id.", "RFC3748-2.1-2")
excl("2:3", "duplicate-of",
     "Restates the same Section 2.1 obligation for the Success arm. Site 4.2:1 maps the id.",
     "RFC3748-2.1-2")

mapped("2.1:1", "RFC3748-2.1-1")
# 2.1:2 left null: see RESIDUAL.
mapped("2.1:3", "RFC3748-2.1-3")
excl("2.1:4", "binds-another-role",
     "The role is the AUTHOR of a 'tunneled' EAP method specification, a document role rather "
     "than a wire role, and the sentence states what such a specification must say about "
     "running a second method inside the tunnel. Ze publishes no EAP method specification, and "
     "neither method it runs tunnels a second one: tlsMethod carries TLS records and no EAP "
     "packet (tlsMethod.Process, internal/component/ike/eap/eap_tls.go), and mschapv2Method "
     "carries the MS-CHAPv2 opcodes alone (mschapv2Method.Process, "
     "internal/component/ike/eap/eap_mschapv2.go). The producer that would act as the role if "
     "Ze did is the specification of such a method, and Ze holds only the implementations of "
     "RFC 5216 and draft-kamath-pppext-eap-mschapv2-02.")
# 2.2:1 left null: see RESIDUAL.

scope("2.3:1", "pass-through operation",
      "that a pass-through authenticator must be capable of forwarding a Code=2 Response to the "
      "backend authentication server",
      PASSTHROUGH_OPTIONAL, LOCAL_ONLY)
scope("2.3:2", "pass-through operation",
      "that a pass-through authenticator must be capable of forwarding Code=1, Code=3 and Code=4 "
      "packets received from the backend authentication server to the peer",
      PASSTHROUGH_OPTIONAL, LOCAL_ONLY)
mapped("2.3:3", "RFC3748-2.3-1")
scope("2.3:4", "pass-through operation",
      "that a compliant pass-through authenticator must by default forward EAP packets of any "
      "Type",
      PASSTHROUGH_OPTIONAL, LOCAL_ONLY)

# --- Section 3 -------------------------------------------------------------
mapped("3.1:1", "RFC3748-3.1-1")
excl("3.2:1", "binds-another-role",
     "The role is a PPP implementation that has negotiated EAP as its LCP Authentication-"
     "Protocol (0xC227), which Section 3.2 titles 'EAP Usage Within PPP'. Ze never acts as it, "
     "and the producer that would is authMethodFromAuthProto "
     "(internal/component/l2tp/ppp/auth.go): it recognises PAP and CHAP alone and answers "
     "AuthMethodNone for every other Auth-Protocol value, 0xC227 included, so Ze's PPP falls "
     "back to the no-wire-auth phase rather than starting an EAP conversation. Ze carries EAP "
     "only inside the IKEv2 SK payload (startEAPExchange, internal/component/ike/engine/fsm.go). "
     "RFC 3748 obliges no implementation to support PPP as a lower layer: Section 2.2 lists PPP "
     "among the layers EAP 'has been run over', which is a description rather than a "
     "requirement.")

# --- Section 4 -------------------------------------------------------------
mapped("4:1", "RFC3748-4-5")
mapped("4:2", "RFC3748-4-4")
mapped("4:3", "RFC3748-4-1")

# 4.1:1 left null: see RESIDUAL.
excl("4.1:2", "duplicate-of",
     "Restates the same-Identifier rule for a retransmitted Request. Site 4.1:8 maps the id.",
     "RFC3748-4.1-3")
# 4.1:3 left null: see RESIDUAL.
mapped("4.1:4", "RFC3748-4.1-1")
mapped("4.1:5", "RFC3748-4.1-2")
# 4.1:6 left null: see RESIDUAL.
excl("4.1:7", "duplicate-of",
     "Restates the same-Identifier rule for a Request retransmitted on a timeout. Site 4.1:8 "
     "maps the id.", "RFC3748-4.1-3")
mapped("4.1:8", "RFC3748-4.1-3")
mapped("4.1:9", "RFC3748-4.1-4")
# 4.1:10 left null: see RESIDUAL.
excl("4.1:11", "duplicate-of",
     "Repeats the Section 4 Length paragraph verbatim inside Section 4.1. Site 4:2 maps the id.",
     "RFC3748-4-4")
excl("4.1:12", "duplicate-of",
     "Repeats the Section 4 Length paragraph verbatim inside Section 4.1. Site 4:3 maps the id.",
     "RFC3748-4-1")
# 4.1:13 left null: see RESIDUAL.
mapped("4.1:14", "RFC3748-4.1-5")
excl("4.1:15", "duplicate-of",
     "Restates Section 2.1's ban on a Nak after an initial non-Nak Response. Site 2.1:3 maps "
     "the id.", "RFC3748-2.1-3")
# 4.1:16 left null: see RESIDUAL.

mapped("4.2:1", "RFC3748-2.1-2")
excl("4.2:2", "duplicate-of",
     "The Failure arm of the same obligation, one sentence after the Success arm. Site 4.2:1 "
     "maps the id.", "RFC3748-2.1-2")
mapped("4.2:3", "RFC3748-4.2-2")
# 4.2:4 left null: see RESIDUAL.
mapped("4.2:5", "RFC3748-4.2-8")
mapped("4.2:6", "RFC3748-4.2-7")
# 4.2:7 left null: see RESIDUAL.
# 4.2:8 left null: see RESIDUAL.
# 4.2:9 left null: see RESIDUAL.
mapped("4.2:10", "RFC3748-4.2-5")
# 4.2:11 left null: see RESIDUAL.
mapped("4.2:12", "RFC3748-4.2-9")
mapped("4.2:13", "RFC3748-4.2-6")
# 4.2:14 left null: see RESIDUAL.
excl("4.2:15", "advisory-in-context",
     "The MUST is the consequent of an unexercised MAY: 'However, an authenticator MAY omit "
     "having the peer authenticate to it in situations where limited access is offered (e.g., "
     "guest access).  In this case, the authenticator MUST send a Success packet.' Ze's "
     "authenticator offers no guest access: every path to a Success runs through a completed "
     "method (Session.handleMethod, internal/component/ike/eap/eap.go).")
mapped("4.2:16", "RFC3748-4.2-4")

excl("4.3:1", "advisory-in-context",
     "The MUST is the consequent of a MAY inside a RECOMMENDED algorithm: '...the "
     "retransmission timer is calculated with a jitter by using the RTO value and randomly "
     "adding a value drawn between -RTOmin/2 and RTOmin/2.  Alternative calculations to create "
     "jitter MAY be used.  These MUST be pseudo-random.' Ze runs no EAP-layer retransmission "
     "timer at all, which Section 4.3 itself directs for a reliable lower layer.")

# --- Section 5 -------------------------------------------------------------
mapped("5:1", "RFC3748-5-1")
# 5:2 left null: see RESIDUAL.

mapped("5.1:1", "RFC3748-5.1-2")

reloc("5.2:1", "RFC3748-5.2-1",
      "Type 2 (Notification) is unimplemented on the peer and " + SPEC + " implements it, "
      "reserving this id.")
reloc("5.2:2", "RFC3748-5.2-2",
      "The ban on answering a Notification Request with a Nak is owed by the same spec, which "
      "reserves this id.")
excl("5.2:3", "advisory-in-context",
     "The MUST is the consequent of a MAY, and the enclosing construction is: 'An EAP method "
     "MAY indicate within its specification that Notification messages must not be sent during "
     "that method.  In this case, the peer MUST silently discard Notification Requests from the "
     "point where an initial Request for that Type is answered with a Response of the same "
     "Type.' The antecedent is false for both methods Ze runs. Neither specification exercises "
     "that MAY: the string 'Notification' appears nowhere in rfc/full/rfc5216.txt and nowhere "
     "in rfc/full/rfc2759.txt, so no method Ze offers prohibits Notification messages and the "
     "peer is never in the state this sentence describes. What Ze owes a Notification Request "
     "OUTSIDE that state is Section 5.2's opening obligation, which sites 5.2:1 and 5.2:5 "
     "relocate to " + SPEC + " under RFC3748-5.2-1.")
# 5.2:4 left null: see RESIDUAL.
reloc("5.2:5", "RFC3748-5.2-1",
      "Restates Section 5.2's opening obligation to answer a Notification Request with a "
      "Notification Response, which site 5.2:1 relocates to the same spec under the same "
      "reserved id. It cannot be duplicate-of, because that kind requires another site to have "
      "MAPPED the id and no site maps a relocated obligation.")

reloc("5.3.1:1", "RFC3748-5.3.1-4",
      "The ban on using a legacy Nak as a general purpose error indication is owed by " + SPEC +
      ", which reserves this id.")
reloc("5.3.1:2", "RFC3748-5.3.1-3",
      "The legacy Nak Identifier rule is owed by " + SPEC + ", which reserves this id.")
reloc("5.3.1:3", "RFC3748-5.3.1-1",
      "Ze's peer answers an unacceptable Type with an error rather than a Nak "
      "(PeerSession.handleRequest, internal/component/ike/eap/peer.go). " + SPEC +
      " implements the Nak and reserves this id.")
reloc("5.3.1:4", "RFC3748-5.3.1-2",
      "The legacy Nak Type-Data contents are owed by " + SPEC + ", which reserves this id.")

for sid, what in (("5.3.2:1", "when an Expanded Nak may be sent"),
                  ("5.3.2:2", "the ban on an Expanded Nak as a general error indication"),
                  ("5.3.2:3", "the Expanded Nak Identifier rule"),
                  ("5.3.2:4", "the Expanded Nak Vendor-Data contents")):
    scope(sid, "the Expanded Type (254) namespace", what, OTHER_TYPES_OPTIONAL,
          "TypeExpandedEAP is a bare constant with no producer and " + NEWSESSION +
          " (internal/component/ike/eap/eap.go). Section 5.7 routes a peer that cannot "
          "interpret an Expanded Type to the LEGACY Nak of Section 5.3.1 instead, which is "
          "site 5.7:2, so declining Type 254 leaves Ze a conformant answer rather than none")

# 5.4:1 left null: see RESIDUAL.
# 5.4:2 left null: see RESIDUAL.
scope("5.4:3", "pass-through operation",
      "what an authenticator that supports only pass-through must do with an MD5-Challenge "
      "Response",
      PASSTHROUGH_OPTIONAL, LOCAL_ONLY)
excl("5.4:4", "cross-document",
     "The obligation is [RFC1994]'s, cited by this sentence: 'while [RFC1994] states that both "
     "the Identifier and Challenge fields MUST change each time a Challenge ... is sent'.")

for sid, what in (("5.5:1", "answering an OTP Request"),
                  ("5.5:2", "the Type of the answering Response"),
                  ("5.5:3", "the ban on cleartext passwords"),
                  ("5.5:4", "the message not being null terminated")):
    scope(sid, "the One Time Password method (Type 5)", what, OTHER_TYPES_OPTIONAL,
          NEWSESSION + ", and Type 5 derives no MSK, which RFC3748-7.10-3 forbids an IKEv2 "
          "conversation from using at all")

for sid, what in (("5.6:1", "answering a GTC Request"),
                  ("5.6:2", "the Type of the answering Response"),
                  ("5.6:3", "the ban on cleartext passwords outside a protected tunnel"),
                  ("5.6:4", "the message not being null terminated"),
                  ("5.6:5", "the Type field of the answering Response")):
    scope(sid, "the Generic Token Card method (Type 6)", what, OTHER_TYPES_OPTIONAL,
          NEWSESSION + ", and Type 6 derives no MSK, which RFC3748-7.10-3 forbids an IKEv2 "
          "conversation from using at all")

mapped("5.7:1", "RFC3748-5.7-1")
reloc("5.7:2", "RFC3748-5.3.1-1",
      "The Nak this sentence demands IS the legacy Nak of Section 5.3.1, which it cites by "
      "number, so the obligation is the one RFC3748-5.3.1-1 reserves and site 5.3.1:3 relocates "
      "under the same id. " + SPEC + " implements it and its AC-6 tags this arm: a Request "
      "carrying an Expanded Type the peer cannot interpret is answered with a LEGACY Nak. It "
      "cannot be duplicate-of, because that kind requires another site to have MAPPED the id, "
      "and no site maps a relocated obligation.")

# --- Section 7 -------------------------------------------------------------
AUTHOR = (
    "The role is the AUTHOR of an EAP METHOD SPECIFICATION, which is a document role rather "
    "than a wire role: Section 7.2 addresses it as 'EAP method specifications MUST include a "
    "Security Claims section'. Ze publishes no EAP method specification. The producer that "
    "would act as the role if Ze did is the specification itself, and for the two methods Ze "
    "runs those documents are RFC 5216 (EAP-TLS) and draft-kamath-pppext-eap-mschapv2-02 "
    "(EAP-MSCHAPv2); newTLSMethod and newMSCHAPv2Method (internal/component/ike/eap/eap.go) "
    "implement what those two state, and state nothing themselves.")

for sid, what in (("7.2:1", "the Security Claims section a method specification must include"),
                  ("7.2:2", "the effective key strength estimate"),
                  ("7.2:3", "the key hierarchy reference or MSK/EMSK derivation description"),
                  ("7.2:4", "the claims that are NOT being made")):
    excl(sid, "binds-another-role", AUTHOR + " This sentence states " + what + ".")

for sid, what in (("7.2.1:1", "describing the EAP packets and fields protected"),
                  ("7.2.1:2", "the identity protection a claiming method must support")):
    excl(sid, "binds-another-role", AUTHOR + " Section 7.2.1 is the claims VOCABULARY a "
         "specification writes its Security Claims section in, and this sentence states " +
         what + ".")

mapped("7.5:1", "RFC3748-7.5-1")

mapped("7.10:1", "RFC3748-7.10-1")
mapped("7.10:2", "RFC3748-7.10-4")
# 7.10:3 left null: see RESIDUAL.
excl("7.10:4", "binds-another-role", AUTHOR + " This sentence states that keying material a "
     "method exports must be independent of the ciphersuite negotiated to protect data, which "
     "is a property of the DERIVATION a specification defines. The two Ze runs are defined "
     "elsewhere: RFC 5216 Section 2.3 for EAP-TLS (exportEAPTLSMSK, "
     "internal/component/ike/eap/eap_tls.go) and draft-kamath-pppext-eap-mschapv2-02 for "
     "EAP-MSCHAPv2 (DeriveMSK, internal/component/ike/eap/mschapv2.go).")
for sid, what in (("7.10:5", "cryptographic separation between the MSK and EMSK branches"),
                  ("7.10:6", "the non-recoverability of one key from the other"),
                  ("7.10:7", "the separation of non-overlapping MSK substrings"),
                  ("7.10:8", "the same property stated as a knowledge bound"),
                  ("7.10:9", "the separation of non-overlapping EMSK substrings")):
    excl(sid, "binds-another-role", AUTHOR + " This sentence states " + what + ", which is a "
         "property a method's key hierarchy must be SHOWN to have. The showing is the "
         "specification's, and Ze holds the implementations of the two it runs.")
# 7.10:10 left null: see RESIDUAL.
# 7.10:11 left null: see RESIDUAL.

scope("7.13:1", "pass-through operation",
      "that the AAA protocol spoken between an authenticator and a backend authentication "
      "server must support per-packet authentication. Section 7.13 states its own antecedent, "
      "'in the case where the authenticator and authentication server reside on different "
      "machines', which is pass-through",
      PASSTHROUGH_OPTIONAL, LOCAL_ONLY)

for sid, what in (("7.16:1", "which result indications are protected"),
                  ("7.16:2", "the four claims a protected-result-indication method must also support")):
    excl(sid, "binds-another-role", AUTHOR + " This sentence states " + what +
         ", which a specification claiming protected result indications must document.")

# --- sections --------------------------------------------------------------
SKIP = {"front": "front-matter", "6": "iana", "6.1": "iana", "6.2": "iana",
        "8": "acknowledgements", "9": "references", "9.1": "references",
        "9.2": "references", "A": "appendix-non-normative"}
SKIP_REASON = {
    "front": "Title block, abstract, status of this memo and table of contents.",
    "6": "IANA Considerations: the registry actions bind IANA, not an implementation.",
    "6.1": "IANA Considerations, Packet Codes: a registry allocation policy.",
    "6.2": "IANA Considerations, Method Types: a registry allocation policy.",
    "8": "Acknowledgements.",
    "9": "References.",
    "9.1": "Normative References.",
    "9.2": "Informative References.",
    "A": "Appendix A, Changes from RFC 2284: a historical diff against an obsoleted document.",
}
# ids the summary declares that no site targets, recorded on the section they were read from.
UNSOURCED = {
    "2": ["RFC3748-2-1", "RFC3748-2-2"],
    "3.1": ["RFC3748-3.1-2", "RFC3748-3.1-3", "RFC3748-3.1-4"],
    "4": ["RFC3748-4-2"],
    "4.2": ["RFC3748-4.2-1"],
    "7.10": ["RFC3748-7.10-2", "RFC3748-7.10-3"],
}

for sec in doc["sections"]:
    sid = sec["id"]
    if sid in SKIP:
        sec["disposition"] = "skipped"
        sec["skip-kind"] = SKIP[sid]
        sec["reason"] = SKIP_REASON[sid]
    else:
        sec["disposition"] = "walked"
    if sid in UNSOURCED:
        sec["unsourced-ids"] = UNSOURCED[sid]

# RESIDUAL: every site this walk cannot dispose of honestly, and why. A site is
# here because the classification is BLOCKED, never because nobody looked.
#
#   row   the obligation is MET and no checklist row declares it. The
#         disposition is `mapped`, and it cannot be written until the row and
#         its tagged tests exist, because a MUST-level row of an enrolled RFC is
#         gated the moment it is written
#   code  the obligation is NOT met and the code is reachable from here
#   test  the obligation is NOT met, and making it met turns an RFC-tagged test
#         red. Correcting one needs the owner and a row in test/rfc-changed.md
#   D-1   blocked on D-1 of plan/spec-eap-notification-and-nak.md
#   D-2   blocked on D-2 of the same spec
RESIDUAL = {
    "2:1":    ("row",  "MET by construction: the authenticator holds no timer, so nothing can "
                       "make it send a terminal packet on a retransmission or a timeout. "
                       "Session.Process runs only on a received packet (eap.go)"),
    "2.1:2":  ("code", "The AUTHENTICATOR half is met (s.method is fixed at NewSession and "
                       "handleMethod calls only it, so no Request of another Type can leave). "
                       "The PEER half is not: handleMSCHAPv2Request answers a Request of the "
                       "wrong Type with an error, which kills the SA, where the sentence asks "
                       "for a silent discard"),
    "2.2:1":  ("row",  "MET: Packet.Encode gives Success and Failure four octets and copies no "
                       "TypeData (eap.go), and Ze emits no Nak and no Notification, so none of "
                       "the four named messages can carry method data"),
    "4.1:1":  ("row",  "MET through the carrier: the EAP layer holds no retry counter, and "
                       "IKEv2 retransmits the IKE_AUTH message carrying the Request "
                       "(sa.LastSentMsg and cacheResponse, internal/component/ike/engine)"),
    "4.1:3":  ("row",  "MET: handleRequest answers every valid Request with a Response, and "
                       "returns an error rather than silence for one it cannot answer "
                       "(peer.go)"),
    "4.1:6":  ("row",  "MET by construction: PeerSession.Process is synchronous and takes one "
                       "packet, driven from the SA's own goroutine, so a second Request cannot "
                       "be inspected before the first completes"),
    "4.1:10": ("code", "NOT met: Session.Process never compares response.Identifier against "
                       "s.identifier, so a Response answering no outstanding Request is acted "
                       "on. The comparison and the nil return are a few lines"),
    "4.1:13": ("row",  "MET by construction: Packet.Type is one octet and Encode writes exactly "
                       "one at offset 4 (eap.go)"),
    "4.1:16": ("test", "NOT met: handleMethod answers a Response whose Type is neither the "
                       "Request's nor a Nak with s.failure, where the sentence asks for a "
                       "silent discard. The RFC3748-2.1-1 NEGATIVE arm of "
                       "TestRFC3748OneMethodPerConversation (rfc3748_test.go) requires that "
                       "EAP-Failure by name, so the fix reds a tagged test"),
    "4.2:4":  ("row",  "MET: the only Success is the result.Done arm of handleMethod and the "
                       "only Failure follows a method refusal or a protocol error, so neither "
                       "leaves at a point the method did not reach"),
    "4.2:7":  ("row",  "MET through the carrier: a lost EAP-Success is retransmitted with the "
                       "IKE_AUTH message that carried it, which is what RFC 3748 Section 4.3 "
                       "directs for a reliable lower layer"),
    "4.2:8":  ("row",  "MET by stateLastWord (eap.go), which exists for this sentence and "
                       "quotes it in its own doc comment: the exchange answers whatever comes "
                       "back after a failure result indication with the EAP-Failure"),
    "4.2:9":  ("row",  "MET: handleMethod sends EAP-Success on result.Done, which for "
                       "MS-CHAPv2 is the round that receives the peer's Success "
                       "acknowledgement, so both indications have been exchanged first"),
    "4.2:11": ("test", "PARTLY met: the peer refuses a later Success rather than discarding it "
                       "in silence (errSessionEnded, peer.go), so it never hands out the MSK "
                       "but does report an error. The RFC2759-x-7 NEGATIVE arm of "
                       "TestRFC2759PeerEndsSessionOnBadAuthenticatorResponse "
                       "(rfc2759_authenticator_response_test.go) requires that error by name"),
    "4.2:14": ("row",  "MET: handleMethod sends EAP-Failure for a method refusal and reaches "
                       "the Success arm only through result.Done, so a failed peer is never "
                       "granted access"),
    # VOID 2026-09-01: the deviation was withdrawn and Types 2, 3 and 4 are all
    # implemented, so this site is mapped to RFC3748-5-2 in the landed artifact.
    "5:2":    ("D-1",  "NOT met for Types 2, 3 and 4. The owner authorized the deviation on "
                       "2026-08-30 and no exclusion kind means 'an authorized deviation', "
                       "which is D-1 itself"),
    "5.2:4":  ("D-2",  "Binds the SENDER of a Notification Request. Ze sends none today, and "
                       "whether it ever should is the MAY of Section 5.2, which D-2 puts to "
                       "the owner. Writing feature-out-of-scope here would answer D-2 by "
                       "asserting a scope decision nobody has taken"),
    # VOID 2026-09-01: MD5-Challenge is implemented on both roles, so these two
    # are mapped to RFC3748-5.4-1 and RFC3748-5.4-2 in the landed artifact.
    "5.4:1":  ("D-1",  "Binds a peer that implements MD5-Challenge. Its antecedent is 5.4:2, "
                       "so it takes the same disposition and waits on the same decision"),
    "5.4:2":  ("D-1",  "NOT met: MD5-Challenge is unimplemented. Same authorized deviation as "
                       "5:2"),
    "7.10:3": ("row",  "MET: the MSK feeds the AUTH payload PRF (ComputeAuthFromMSK, "
                       "internal/component/ike/engine/eap_auth.go) and never a data-protecting "
                       "key. The Child SA keys come from the IKEv2 KEYMAT derivation"),
    "7.10:10":("row",  "Vacuously MET: Ze derives no EMSK anywhere under "
                       "internal/component/ike/eap, which is the same fact RFC3748-7.10-2 "
                       "already records as not-applicable. The row this site needs states the "
                       "confinement rather than the size"),
    "7.10:11":("row",  "MET: when one party discards the key state the IKEv2 AUTH verification "
                       "fails, the SA is torn down, and the initiator re-authenticates rather "
                       "than wedging (verifyRemoteAuth, internal/component/ike/engine)"),
}

left = []
for site in doc["sites"]:
    sid = site["id"]
    d = M.get(sid)
    if d is None:
        left.append(sid)
        continue
    if d[0] == "mapped":
        site["disposition"] = "mapped"
        site["mapped-to"] = d[1]
    elif d[0] == "relocated":
        site["disposition"] = "excluded"
        site["excluded-kind"] = "relocated-to-spec"
        site["relocated-to"] = SPEC
        site["reserved-id"] = d[1]
        site["reason"] = d[2]
    else:
        site["disposition"] = "excluded"
        site["excluded-kind"] = d[1]
        site["reason"] = d[2]
        if d[3]:
            site["mapped-to"] = d[3]

doc["signed-off"] = "2026-08-31"
doc["reviewer"] = "rfc3748walk (extraction sign-off agent, spec rfcgate-6-supported-extraction-signoff)"

json.dump(doc, open(path, "w"), indent=2)
open(path, "a").write("\n")
KINDS = {"row": "met, no row declares it", "code": "unmet, reachable from here",
         "test": "unmet, the fix reds an RFC-tagged test",
         "D-1": "blocked on D-1", "D-2": "blocked on D-2"}

print("unclassified sites:", len(left))
for sid in left:
    kind, why = RESIDUAL.get(sid, ("?", "no reason recorded, which is itself the defect"))
    print("  " + sid.ljust(9) + kind.ljust(6) + why)
print()
for kind, label in KINDS.items():
    n = sum(1 for sid in left if RESIDUAL.get(sid, ("?",))[0] == kind)
    print("  " + str(n).rjust(3) + "  " + kind.ljust(6) + label)
missing = [sid for sid in left if sid not in RESIDUAL]
if missing:
    raise SystemExit("BUG: unclassified sites with no recorded reason: " + ", ".join(missing))
