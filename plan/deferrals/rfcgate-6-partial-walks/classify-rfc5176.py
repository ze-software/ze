#!/usr/bin/env python3
"""Replayable classification of the RFC 5176 extraction walk.

Ze implements ONE of the two roles RFC 5176 binds: the Dynamic Authorization
Server (DAS), co-resident with the NAS.  `coaListener` in
internal/component/l2tp/plugins/authradius/coa.go is the only producer or
consumer of codes 40-45 anywhere in the tree (verified by grep over
CodeCoARequest/CodeDisconnectRequest/CodeCoAACK/CodeCoANAK/CodeDisconnectACK/
CodeDisconnectNAK, non-test files), so Ze is neither a Dynamic Authorization
Client nor a RADIUS forwarding proxy.

A site left with a null disposition here is one of two things, and the report
names which: an obligation ze MEETS that rfc/short/rfc5176.md never declared
(it owes a new id and its tagged tests), or an obligation ze does NOT meet (it
owes product work).  Neither may be excluded: no kind in the closed set says
"owed and absent".
"""
import json, sys, pathlib

path = pathlib.Path(sys.argv[1])
doc = json.loads(path.read_text())

DAC = ("the Dynamic Authorization Client, the entity originating CoA-Request and "
       "Disconnect-Request packets (RFC 5176 Section 1.3). ze originates neither: "
       "coaListener (internal/component/l2tp/plugins/authradius/coa.go) is the only "
       "site in the tree that names radius.CodeCoARequest or "
       "radius.CodeDisconnectRequest, and it only receives them")
PROXY = ("the RADIUS forwarding proxy. ze forwards no CoA or Disconnect packet: "
         "coaListener.handlePacket (internal/component/l2tp/plugins/authradius/coa.go) "
         "dispatches to handleCoA or handleDisconnect and both answer locally, and no "
         "other file in the tree emits a code in the 40-45 range")
AUTHZONLY = ('RFC 5176 Section 3.2: "Support for a CoA-Request including a Service-Type '
             'Attribute with value \\"Authorize Only\\" is OPTIONAL on the NAS and Dynamic '
             'Authorization Client." The obligation is the interior of that OPTIONAL '
             'feature, which the owner declined on 2026-08-31')

MAPPED = {
    "2.3:1":  "RFC5176-3.5-4",
    "3.4:1":  "RFC5176-3.4-3",
    "3.4:3":  "RFC5176-3.4-1",
}

EXCLUDED = {
    # --- binds-another-role: the Dynamic Authorization Client ---------------
    "2.3:4":  ("binds-another-role", "Identifier management by the sender. Binds " + DAC),
    "2.3:5":  ("binds-another-role", "Identifier reuse on retransmission by the sender. Binds " + DAC),
    "2.3:6":  ("binds-another-role", "Retransmission by the sender. Binds " + DAC),
    "2.3:7":  ("binds-another-role", "Authenticator and Identifier choice by the sender. Binds " + DAC),
    "2.3:8":  ("binds-another-role", "Failover to a secondary DAS by the sender. Binds " + DAC),
    "3:3":    ("binds-another-role", "What a Disconnect-Request MUST contain, an obligation on the composer. Binds " + DAC + ". The receive-side counterpart is site 3:4, which binds ze and is left unclassified"),
    "3.2:1":  ("binds-another-role", "What a Disconnect-Request MUST NOT contain, an obligation on the composer. Binds " + DAC),
    "3.2:4":  ("binds-another-role", "What an Authorize Only CoA-Request MUST contain, an obligation on the composer. Binds " + DAC + ". The receive-side counterpart is site 3.2:5, left unclassified"),
    "3.3:9":  ("binds-another-role", "A packet-shape rule on the CoA-Request the sender builds; RFC 5176 Section 3.6 states the same bound as the 0-1 column for State. Binds " + DAC),
    "3.4:2":  ("binds-another-role", "Verification of a CoA/Disconnect-ACK or -NAK, a packet ze emits and never receives. Binds " + DAC),
    "3.6:2":  ("binds-another-role", "Note 1 governs how a sender USES an identification attribute. Binds " + DAC),
    "3.6:3":  ("binds-another-role", "Note 7 governs how a sender USES a VSA, for identification or for authorization change. Binds " + DAC),
    "4:1":    ("binds-another-role", "The Diameter-considerations restatement of Section 3.2's rule on what a Disconnect-Request may carry. Binds " + DAC),

    # --- binds-another-role: the RADIUS forwarding proxy --------------------
    "3.1:5":  ("binds-another-role", "Binds " + PROXY),
    "3.1:6":  ("binds-another-role", "Binds " + PROXY),
    "3.1:7":  ("binds-another-role", "Binds " + PROXY),
    "3.1:8":  ("binds-another-role", "Binds " + PROXY),
    "3.1:9":  ("binds-another-role", "Binds " + PROXY),
    "3.1:10": ("binds-another-role", "Binds " + PROXY),
    "3.1:11": ("binds-another-role", "Binds " + PROXY),

    # --- advisory-in-context ------------------------------------------------
    "3.2:6":  ("advisory-in-context", AUTHZONLY),
    "3.2:7":  ("advisory-in-context", AUTHZONLY),
    "3.3:4":  ("advisory-in-context", AUTHZONLY + ". ze sends no Access-Request carrying Service-Type Authorize Only"),
    "3.3:5":  ("advisory-in-context", AUTHZONLY + ". Its first clause binds " + DAC + ", and its second is conditioned on \"the resulting Access-Request, if any\", which ze never sends"),
    "3.4:4":  ("advisory-in-context", 'The response-direction Message-Authenticator, whose enclosing construction is RFC 5176 Section 3.4: "The Message-Authenticator Attribute MAY be used to authenticate and integrity-protect CoA-Request, CoA-ACK, CoA-NAK, Disconnect-Request, Disconnect-ACK, and Disconnect-NAK packets in order to prevent spoofing." coaListener.sendResponse (internal/component/l2tp/plugins/authradius/coa.go) includes no Message-Authenticator in a CoA-ACK, CoA-NAK, Disconnect-ACK or Disconnect-NAK, which the MAY permits, so the computation rule has no packet to govern'),
    "6.1:2":  ("advisory-in-context", 'The else-branch of an optional check. Its enclosing construction is RFC 5176 Section 6.1: "In situations where the Dynamic Authorization Client is co-resident with a RADIUS authentication or accounting server, a proxy MAY perform a \\"reverse path forwarding\\" (RPF) check to verify that a Disconnect-Request or CoA-Request originates from an authorized Dynamic Authorization Client." ze performs no RPF check and maintains no realm routing table, which the same section says makes an RPF check impossible for a NAS'),

    # --- cross-document -----------------------------------------------------
    "3.3:2":  ("cross-document", 'Block-quoted RFC 2865 Section 5.44, introduced by "[RFC2865], Section 5.44 states:". The obligation is RFC 2865\'s and is carried by rfc/short/rfc2865.md'),
    "3.3:3":  ("cross-document", 'The second sentence of the same block quote of RFC 2865 Section 5.44. The obligation is RFC 2865\'s'),

    # --- not-a-requirement --------------------------------------------------
    "3.6:1":  ("not-a-requirement", "The legend of the Section 3.6 table of attributes. The keywords define what the 0, 0+, 0-1 and 1 columns MEAN; they state no obligation on an implementation. The obligations the table expresses are its rows"),
}

sites = {s["id"]: s for s in doc["sites"]}
unknown = (set(MAPPED) | set(EXCLUDED)) - set(sites)
if unknown:
    raise SystemExit("classification names sites the inventory does not derive: %s" % sorted(unknown))

for sid, rid in MAPPED.items():
    sites[sid]["disposition"] = "mapped"
    sites[sid]["mapped-to"] = rid
for sid, (kind, reason) in EXCLUDED.items():
    sites[sid]["disposition"] = "excluded"
    sites[sid]["excluded-kind"] = kind
    sites[sid]["reason"] = reason

SECTIONS = {
    "front": ("skipped", "front-matter", "Title, status, copyright and table of contents."),
    "1":     ("walked", None, None),
    "1.1":   ("walked", None, None),
    "1.2":   ("walked", None, None),
    "1.3":   ("walked", None, None),
    "2":     ("walked", None, None),
    "2.1":   ("walked", None, None),
    "2.2":   ("walked", None, None),
    "2.3":   ("walked", None, None),
    "3":     ("walked", None, None),
    "3.1":   ("walked", None, None),
    "3.2":   ("walked", None, None),
    "3.3":   ("walked", None, None),
    "3.4":   ("walked", None, None),
    "3.5":   ("walked", None, None),
    "3.6":   ("walked", None, None),
    "4":     ("walked", None, None),
    "5":     ("skipped", "iana", "IANA Considerations: it allocates Error-Cause values 407 and 508 and binds IANA, not an implementation."),
    "6":     ("walked", None, None),
    "6.1":   ("walked", None, None),
    "6.2":   ("walked", None, None),
    "6.3":   ("walked", None, None),
    "7":     ("walked", None, None),
    "8":     ("skipped", "references", "Reference list."),
    "8.1":   ("skipped", "references", "Normative references."),
    "8.2":   ("skipped", "references", "Informative references."),
    "9":     ("skipped", "acknowledgements", "Acknowledgments."),
    "A":     ("skipped", "appendix-non-normative", "Appendix A, Changes from RFC 3576: a change log against the obsoleted document."),
}

UNSOURCED = {
    "2.3": ["RFC5176-3.5-1", "RFC5176-3.5-2", "RFC5176-3.5-3"],
    "3.3": ["RFC5176-3.3-1"],
    "3.4": ["RFC5176-3.4-2"],
}

for sec in doc["sections"]:
    disp, kind, reason = SECTIONS[sec["id"]]
    sec["disposition"] = disp
    if kind:
        sec["skip-kind"] = kind
    if reason:
        sec["reason"] = reason
    if sec["id"] in UNSOURCED:
        sec["unsourced-ids"] = UNSOURCED[sec["id"]]

doc["signed-off"] = ""
doc["reviewer"] = ""

path.write_text(json.dumps(doc, indent=2) + "\n")

nsites = len(doc["sites"])
done = sum(1 for s in doc["sites"] if s.get("disposition"))
print("sites %d, classified %d, unclassified %d" % (nsites, done, nsites - done))
print("mapped %d, excluded %d, exclusion ratio over classified %.2f"
      % (len(MAPPED), len(EXCLUDED), len(EXCLUDED) / nsites))
