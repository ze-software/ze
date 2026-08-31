#!/usr/bin/env python3
"""Replayable classification of the RFC 3748 extraction sign-off.

Reads the skeleton `./le rfc extraction-create stem rfc3748` writes into the
session scratch, applies one disposition per site and per section, and writes
the file back. Sites with no honest disposition are left null on purpose: each
one is reported to the owner rather than annotated away.
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

# --- Section 1.2 -----------------------------------------------------------
excl("1.2:1", "not-a-requirement",
     "Section 1.2 is the Terminology list and the sentence sits inside the definition ENTRY "
     "for 'Displayable Message', fixing what the term means wherever a later section uses it. "
     "The obligations that put a displayable message on the wire are stated at Sections 5.1, "
     "5.2, 5.5 and 5.6, and those sites carry their own dispositions.")

# --- Section 2 -------------------------------------------------------------
# 2:1 left null: met and undeclared, see the report.
excl("2:2", "duplicate-of",
     "Restates the Section 2.1 obligation to close the conversation with a Success or a Failure, "
     "for the Failure arm. Site 4.2:1 maps the id.", "RFC3748-2.1-2")
excl("2:3", "duplicate-of",
     "Restates the same Section 2.1 obligation for the Success arm. Site 4.2:1 maps the id.",
     "RFC3748-2.1-2")

mapped("2.1:1", "RFC3748-2.1-1")
# 2.1:2 left null: the peer half (silently discard an out-of-turn Request) is unmet.
mapped("2.1:3", "RFC3748-2.1-3")
excl("2.1:4", "binds-another-role",
     "Binds the specification author of a 'tunneled' EAP method. Ze implements EAP-TLS "
     "(RFC 5216) and EAP-MSCHAPv2 and authors no EAP method specification; neither method "
     "tunnels a second EAP method (newTLSMethod and newMSCHAPv2Method, "
     "internal/component/ike/eap/eap.go).")
# 2.2:1 left null: met and undeclared.

excl("2.3:1", "binds-another-role",
     "Binds a pass-through authenticator. Ze's authenticator terminates the EAP method locally: "
     "NewSession (internal/component/ike/eap/eap.go) builds the method in process and the IKE "
     "engine has no AAA back end (no EAP-Message producer exists under "
     "internal/component/radius/).")
excl("2.3:2", "binds-another-role",
     "Same role: a pass-through authenticator forwarding from a backend authentication server. "
     "Ze has no backend authentication server for EAP (NewSession, "
     "internal/component/ike/eap/eap.go).")
mapped("2.3:3", "RFC3748-2.3-1")
excl("2.3:4", "binds-another-role",
     "Binds a pass-through authenticator deciding on an Accept/Reject from a backend server. "
     "Ze's authenticator decides from its own method result (Session.handleMethod, "
     "internal/component/ike/eap/eap.go).")

# --- Section 3 -------------------------------------------------------------
mapped("3.1:1", "RFC3748-3.1-1")
excl("3.2:1", "binds-another-role",
     "Binds a PPP implementation carrying EAP (Section 3.2, 'EAP Usage Within PPP'). Ze carries "
     "EAP only inside the IKEv2 SK payload (startEAPExchange, "
     "internal/component/ike/engine/fsm.go); its PPP stack under internal/component/l2tp/ "
     "negotiates no EAP authentication protocol.")

# --- Section 4 -------------------------------------------------------------
# 4:1 left null: unmet, neither role silently discards an unknown Code.
mapped("4:2", "RFC3748-4-4")
mapped("4:3", "RFC3748-4-1")

# 4.1:1 left null: met through the carrier and undeclared.
excl("4.1:2", "duplicate-of",
     "Restates the same-Identifier rule for a retransmitted Request. Site 4.1:8 maps the id.",
     "RFC3748-4.1-3")
# 4.1:3 left null: met and undeclared.
mapped("4.1:4", "RFC3748-4.1-1")
mapped("4.1:5", "RFC3748-4.1-2")
# 4.1:6 left null: met and undeclared.
excl("4.1:7", "duplicate-of",
     "Restates the same-Identifier rule for a Request retransmitted on a timeout. Site 4.1:8 "
     "maps the id.", "RFC3748-4.1-3")
mapped("4.1:8", "RFC3748-4.1-3")
mapped("4.1:9", "RFC3748-4.1-4")
# 4.1:10 left null: unmet, the authenticator never compares the Identifier.
excl("4.1:11", "duplicate-of",
     "Repeats the Section 4 Length paragraph verbatim inside Section 4.1. Site 4:2 maps the id.",
     "RFC3748-4-4")
excl("4.1:12", "duplicate-of",
     "Repeats the Section 4 Length paragraph verbatim inside Section 4.1. Site 4:3 maps the id.",
     "RFC3748-4-1")
# 4.1:13 left null: met by construction and undeclared.
mapped("4.1:14", "RFC3748-4.1-5")
excl("4.1:15", "duplicate-of",
     "Restates Section 2.1's ban on a Nak after an initial non-Nak Response. Site 2.1:3 maps "
     "the id.", "RFC3748-2.1-3")
# 4.1:16 left null: unmet, the EAP server does not silently discard such a Response.

mapped("4.2:1", "RFC3748-2.1-2")
excl("4.2:2", "duplicate-of",
     "The Failure arm of the same obligation, one sentence after the Success arm. Site 4.2:1 "
     "maps the id.", "RFC3748-2.1-2")
mapped("4.2:3", "RFC3748-4.2-2")
# 4.2:4 left null: met and undeclared.
# 4.2:5 left null: unmet, the peer accepts a Success at any point.
# 4.2:6 left null: unmet, the peer accepts a canned Success.
# 4.2:7 left null: met through the carrier and undeclared.
# 4.2:8 left null: met by stateLastWord and undeclared.
# 4.2:9 left null: met and undeclared.
mapped("4.2:10", "RFC3748-4.2-5")
# 4.2:11 left null: met and undeclared.
# 4.2:12 left null: unmet at the EAP layer.
mapped("4.2:13", "RFC3748-4.2-6")
# 4.2:14 left null: met and undeclared.
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
# 5:2 left null: unmet (Types 2, 3 and 4), owner decision D-1 of plan/spec-eap-notification-and-nak.md.

mapped("5.1:1", "RFC3748-5.1-2")

reloc("5.2:1", "RFC3748-5.2-1",
      "Type 2 (Notification) is unimplemented on the peer and " + SPEC + " implements it, "
      "reserving this id.")
reloc("5.2:2", "RFC3748-5.2-2",
      "The ban on answering a Notification Request with a Nak is owed by the same spec, which "
      "reserves this id.")
# 5.2:3 left null: no reserved id exists for the prohibits-Notification discard.
# 5.2:4 left null: owner decision D-2, whether Ze's authenticator ever SENDS a Notification.
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
    excl(sid, "binds-another-role",
         "Binds an Expanded-Type-capable peer, which states " + what + ". Section 5 makes Type "
         "254 support a SHOULD ('All EAP implementations MUST support Types 1-4 ... and SHOULD "
         "support Type 254'), Ze declines it, and it emits no Expanded Type packet: "
         "TypeExpandedEAP is a bare constant and NewSession refuses every type other than 13 "
         "and 26 (internal/component/ike/eap/eap.go). Section 5.7 routes such a peer to a "
         "LEGACY Nak instead, which is site 5.7:2.")

# 5.4:1 left null: MD5-Challenge is unimplemented; owner decision D-1.
# 5.4:2 left null: unmet, owner-authorized deviation without a disposition kind (D-1).
excl("5.4:3", "binds-another-role",
     "Binds an authenticator that supports only pass-through. Ze's authenticator authenticates "
     "locally (NewSession, internal/component/ike/eap/eap.go) and has no backend authentication "
     "server, so the sentence's own antecedent is false for it.")
excl("5.4:4", "cross-document",
     "The obligation is [RFC1994]'s, cited by this sentence: 'while [RFC1994] states that both "
     "the Identifier and Challenge fields MUST change each time a Challenge ... is sent'.")

for sid, what in (("5.5:1", "answering an OTP Request"),
                  ("5.5:2", "the Type of the answering Response"),
                  ("5.5:3", "the ban on cleartext passwords"),
                  ("5.5:4", "the message not being null terminated")):
    excl(sid, "binds-another-role",
         "Binds an implementation of the EAP OTP method (Type 5), which states " + what + ". "
         "Section 5's support obligation covers Types 1-4 only, so Type 5 is optional, and Ze "
         "implements it nowhere: NewSession accepts 13 and 26 alone "
         "(internal/component/ike/eap/eap.go).")

for sid, what in (("5.6:1", "answering a GTC Request"),
                  ("5.6:2", "the Type of the answering Response"),
                  ("5.6:3", "the ban on cleartext passwords outside a protected tunnel"),
                  ("5.6:4", "the message not being null terminated"),
                  ("5.6:5", "the Type field of the answering Response")):
    excl(sid, "binds-another-role",
         "Binds an implementation of the EAP GTC method (Type 6), which states " + what + ". "
         "Section 5's support obligation covers Types 1-4 only, so Type 6 is optional, and Ze "
         "implements it nowhere: NewSession accepts 13 and 26 alone "
         "(internal/component/ike/eap/eap.go).")

mapped("5.7:1", "RFC3748-5.7-1")
# 5.7:2 left null: unmet; the legacy Nak it demands is the subject of the reserved id
# RFC3748-5.3.1-1, which site 5.3.1:3 already relocates.

# --- Section 7 -------------------------------------------------------------
for sid, what in (("7.2:1", "the Security Claims section a method specification must include"),
                  ("7.2:2", "the effective key strength estimate"),
                  ("7.2:3", "the key hierarchy reference or MSK/EMSK derivation description"),
                  ("7.2:4", "the claims that are NOT being made")):
    excl(sid, "binds-another-role",
         "Binds the author of an EAP METHOD SPECIFICATION: 'EAP method specifications MUST "
         "include a Security Claims section'. It states " + what + ". Ze authors no EAP method "
         "specification; it implements EAP-TLS (RFC 5216) and EAP-MSCHAPv2 as they are "
         "specified elsewhere.")

for sid, what in (("7.2.1:1", "describing the EAP packets and fields protected"),
                  ("7.2.1:2", "the identity protection a claiming method must support")):
    excl(sid, "binds-another-role",
         "Section 7.2.1 is the claims vocabulary for a METHOD SPECIFICATION and this sentence "
         "binds its author, over " + what + ". Ze authors none.")

mapped("7.5:1", "RFC3748-7.5-1")

mapped("7.10:1", "RFC3748-7.10-1")
mapped("7.10:2", "RFC3748-7.10-4")
# 7.10:3 left null: met and undeclared.
excl("7.10:4", "binds-another-role",
     "Binds the design of an EAP method's key derivation, which is a METHOD SPECIFICATION "
     "property. Ze's exports follow RFC 5216 Section 2.3 for EAP-TLS and "
     "draft-kamath-pppext-eap-mschapv2-02 for EAP-MSCHAPv2 (DeriveMSK, "
     "internal/component/ike/eap/mschapv2.go), and neither derivation is Ze's to state.")
for sid, what in (("7.10:5", "cryptographic separation between the MSK and EMSK branches"),
                  ("7.10:6", "the non-recoverability of one key from the other"),
                  ("7.10:7", "the separation of non-overlapping MSK substrings"),
                  ("7.10:8", "the same property stated as a knowledge bound"),
                  ("7.10:9", "the separation of non-overlapping EMSK substrings")):
    excl(sid, "binds-another-role",
         "Binds the author of a key-deriving EAP METHOD SPECIFICATION, over " + what + ". It is "
         "a property a method's key hierarchy must be shown to have, and Ze authors no such "
         "hierarchy: it implements RFC 5216 Section 2.3 and "
         "draft-kamath-pppext-eap-mschapv2-02.")
# 7.10:10 left null: met vacuously and undeclared.
# 7.10:11 left null: met and undeclared.

excl("7.13:1", "binds-another-role",
     "Binds the AAA protocol between an authenticator and a backend authentication server. Ze "
     "runs neither side of that link for EAP: its authenticator terminates the method in "
     "process (NewSession, internal/component/ike/eap/eap.go).")

for sid, what in (("7.16:1", "which result indications are protected"),
                  ("7.16:2", "the four claims a protected-result-indication method must also support")):
    excl(sid, "binds-another-role",
         "Binds the author of a METHOD SPECIFICATION that supports protected result "
         "indications, over " + what + ". Ze authors none.")

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
print("unclassified sites:", len(left))
for s in left:
    print("  ", s)
