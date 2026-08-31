#!/usr/bin/env python3
"""Replayable classification of the rfc9069 extraction sign-off.

Re-derive the skeleton first:
    ./le --name rfc9069walk rfc extraction-create stem rfc9069
then run this script. It classifies every SECTION and the seven sites that
have a home in rfc/short/rfc9069.md, and leaves 22 sites null on purpose.

The 22 are NOT unclassified through inattention. Each one states an obligation
that rfc/short/rfc9069.md declares no row for, so `mapped-to` has nothing legal
to name (evaluateExtraction, internal/le/rfc/signoff.go: a mapped-to that is
not an id of this summary is refused), and none of the six exclusion kinds is
honest for a sentence that binds ze. They are listed at the bottom with the
verdict the walk reached for each.
"""

import json
import pathlib
import sys

SCRATCH = pathlib.Path(__file__).resolve().parent
DOC = SCRATCH / "rfc9069.json"

SIGNED_OFF = "2026-08-31"
REVIEWER = "ze-work agent, spec-rfcgate-6 phase 4, rfc9069"
REGISTER_REASON = (
    "Standards Track, with the BCP 14 key-words paragraph at section 2 and capitalised "
    "MUST-level keywords in sections 3, 4.2, 5.1, 5.2, 5.2.1, 5.3, 5.4.1, 6.1.1, 6.1.2, "
    "6.1.3 and 8.3. The derivation grades the source 'rfc2119' and the sign-off claims "
    "exactly that grade."
)

SECTIONS = {
    "front": ("skipped", "front-matter", None,
        "Title block, Abstract, Status of This Memo, Copyright Notice and Table of "
        "Contents. The Abstract restates section 1 and states no obligation."),
    "1": ("walked", None, None,
        "Introduction. Indicative prose: BMP defines no method to send the Loc-RIB, "
        "three use cases for Loc-RIB access, and the statement that this document "
        "replaces Section 8.2 of RFC 7854. No sentence directs a speaker."),
    "1.1": ("walked", None, None,
        "Alternative Method to Monitor Loc-RIB. Argues why deriving a Loc-RIB from a "
        "second router's Adj-RIB-In pre-policy is complex and error prone. Wholly "
        "indicative; no directive."),
    "2": ("walked", None, None,
        "Terminology. The BCP 14 key-words paragraph, which binds the key words only "
        "when they appear in all capitals. It tells a reader how to read the other "
        "sections and binds no speaker, which is why the derivation excludes it from "
        "the site inventory."),
    "3": ("walked", None, None,
        "Definitions. Five terms: BGP Instance, Adj-RIB-In and Adj-RIB-Out quoted from "
        "RFC 4271, Loc-RIB quoted from RFC 4271 Section 1.1, and the pre- and "
        "post-policy Adj-RIB-Out pair. The one capitalised keyword sits in the "
        "post-policy Adj-RIB-Out definition and is excluded below; RFC 9069 never uses "
        "that term again (the only 'Post-Policy' occurrence in the source is this "
        "definition) and states no Adj-RIB-Out obligation of its own."),
    "4": ("walked", None, None,
        "Per-Peer Header. A heading with no body text: 4.1 and 4.2 carry the section."),
    "4.1": ("walked", None, None,
        "Peer Type. Registers Peer Type 3, Loc-RIB Instance Peer, and contrasts it with "
        "the RFC 7854 Section 4.2 Local Instance Peer. Stated indicatively as a value "
        "definition, so the site scan sees nothing; ze's PeerTypeLocRIB "
        "(internal/component/bgp/plugins/bmp/header.go) is that value, and every "
        "rfc/short/rfc9069.md row that names Peer Type 3 carries it."),
    "4.2": ("walked", None, None,
        "Peer Flags. Redefines the per-peer header flags byte for Peer Type 3: bit 7 is "
        "the F flag and bits 0 to 6 are reserved. Three capitalised MUSTs. The reserved "
        "bits sentence is mapped below to RFC9069-x-1. The other two -- locally sourced "
        "routes conveyed with the Loc-RIB Instance Peer Type, and the F flag set when a "
        "filter is applied -- state obligations rfc/short/rfc9069.md declares no row "
        "for and are held; see the notes at the foot of this script."),
    "5": ("walked", None, None,
        "Loc-RIB Monitoring. Two indicative paragraphs: what the Loc-RIB contains, per "
        "RFC 4271 Section 9.1 and 9.4, and that a subset MAY be sent by setting the F "
        "flag. The MAY gates nothing."),
    "5.1": ("walked", None, None,
        "Per-Peer Header. One capitalised MUST introducing an indented definition list "
        "of six field values: Peer Type, Peer Distinguisher, Peer Address, Peer "
        "Autonomous System, Peer BGP ID and Timestamp. The site scan sees the "
        "introducing sentence only, and the six values are stated indicatively beneath "
        "it. rfc/short/rfc9069.md renders three of them (RFC9069-x-5 Peer Address, "
        "RFC9069-x-6 Peer AS, RFC9069-x-7 Peer BGP ID) and none of the other three. "
        "The site is HELD rather than mapped: mapping it to any single one of those "
        "rows would sign the whole 'MUST use the following values' sentence off against "
        "a row covering one field, and the Timestamp value is the one ze gets wrong. "
        "See the notes at the foot of this script."),
    "5.2": ("walked", None, None,
        "Peer Up Notification. Clarifies Section 4.10 of RFC 7854 for the Loc-RIB peer: "
        "Local Address zero-filled, Local Port and Remote Port 0, a fabricated sent "
        "OPEN whose capabilities carry one capitalised MUST (mapped below), and a "
        "received OPEN that repeats it. The zero addresses and ports and the repeated "
        "received OPEN are stated indicatively, so the site scan cannot see them; "
        "RFC9069-x-3 is read from the Received OPEN sentence and is listed unsourced "
        "here.",
        ["RFC9069-x-3"]),
    "5.2.1": ("walked", None, None,
        "Peer Up Information. Defines Peer Up Information TLV type 3, VRF/Table Name. "
        "Eleven capitalised MUSTs: the TLV's content and size rules, the two conditions "
        "under which it must be included, the default value 'global' for the default "
        "instance, the requirement that its presence in the Peer Up carries into the "
        "Peer Down, and the order of repeated strings. The RFC states the paragraph "
        "twice, once inside the bullet and once outside it, so sites 6 to 10 repeat "
        "sites 1 to 5 word for word. Every one of the eleven is HELD: ze's BMP sender "
        "has no producer for a Peer Up Information TLV of any type "
        "(senderSession.writePeerUpLocked, internal/component/bgp/plugins/bmp/sender.go, "
        "takes no TLV argument and builds a PeerUp literal with InfoTLVs unset), so "
        "site 4 is a found-and-unmet obligation under one reading and the other ten "
        "hang on it. See the notes at the foot of this script."),
    "5.3": ("walked", None, None,
        "Peer Down Notification. Four capitalised MUSTs. The reason-code-6 sentence is "
        "mapped below to RFC9069-5.3-1. The other three restate the VRF/Table Name "
        "TLV's content and size rules and require it in the Peer Down when it was in "
        "the Peer Up; all three hang on the section 5.2.1 decision and are held."),
    "5.4": ("walked", None, None,
        "Route Monitoring. Two indicative sentences: Route Monitoring is used for "
        "initial synchronization of the Loc-RIB and for incremental changes, and the "
        "per-peer header is followed by a BGP Update PDU, quoted from Section 4.6 of "
        "RFC 7854. No capitalised keyword, so the site scan sees nothing; RFC9069-x-4, "
        "which records that Loc-RIB monitoring starting triggers a full-table dump, is "
        "read from the initial-synchronization sentence and is listed unsourced here.",
        ["RFC9069-x-4"]),
    "5.4.1": ("walked", None, None,
        "ASN Encoding. One sentence, one capitalised MUST, mapped below to "
        "RFC9069-5.4.1-1."),
    "5.4.2": ("walked", None, None,
        "Granularity. State compression and throttling SHOULD be used, with a worked "
        "example, and a receiver should expect granularity to vary. A SHOULD and two "
        "indicative sentences; none gates. ze applies no state compression: "
        "handleBestChange (internal/component/bgp/plugins/bmp/bmp_locrib.go) writes one "
        "Route Monitoring per entry of each best-change batch, which is permitted."),
    "5.5": ("walked", None, None,
        "Route Mirroring. States that verbatim duplication is not applicable to the "
        "Loc-RIB because the PDUs are originated by the router, and that received Route "
        "Mirroring messages SHOULD be ignored. A SHOULD; it never gates. ze's receiver "
        "meets it anyway: processRouteMirroring "
        "(internal/component/bgp/plugins/bmp/bmp.go) logs the message and acts on "
        "nothing."),
    "5.6": ("walked", None, None,
        "Statistics Report. Lists the two Stat Types relevant to the Loc-RIB, 8 and 10, "
        "as value definitions. No capitalised keyword and no directive: nothing here "
        "requires a sender to emit either. ze declares both constants "
        "(StatRoutesLocRIB, StatRoutesPerAFILocRIB, internal/component/bgp/plugins/bmp/tlv.go) "
        "and emits neither, which this section permits."),
    "6": ("walked", None, None,
        "Other Considerations. A heading with no body text."),
    "6.1": ("walked", None, None,
        "Loc-RIB Implementation. One indicative paragraph: the implementation emulates a "
        "peer with Peer Up, Peer Down and Route Monitoring messages. No directive."),
    "6.1.1": ("walked", None, None,
        "Multiple Loc-RIB Peers. Three capitalised MUSTs. The at-least-one-emulated-peer "
        "sentence and the each-peer-sends-a-Peer-Up sentence are mapped below. The "
        "third binds a BMP receiver to process the OPEN capabilities, which ze "
        "implements as a role and does not do; it is held. See the notes at the foot "
        "of this script."),
    "6.1.2": ("walked", None, None,
        "Filtering Loc-RIB to BMP Receivers. Describes the F flag's use case, then one "
        "capitalised MUST conditioned on multiple filters against the same Loc-RIB. ze "
        "offers no Loc-RIB filter, so the antecedent is unreachable, but "
        "rfc/short/rfc9069.md declares no row for it and the site is held."),
    "6.1.3": ("walked", None, None,
        "Changes to Existing BMP Sessions. One sentence, one capitalised MUST: a change "
        "that alters the behavior of an existing BMP session bounces it with a Peer "
        "Down / Peer Up sequence. ze meets it -- behaviorOf carries the loc-rib leaf "
        "(internal/component/bgp/plugins/bmp/sender_config.go) and applySenderConfig "
        "acts on a change to it -- but rfc/short/rfc9069.md declares no row for it, so "
        "it is held. RFC 8671 Section 7.2 states the same obligation and ze declares "
        "that one as RFC8671-7.2-1."),
    "7": ("walked", None, None,
        "Security Considerations. Imports Section 11 of RFC 7854, states that "
        "implementations SHOULD require sessions only with authorized and trusted "
        "monitoring devices, and states that this document adds no further "
        "consideration. The SHOULD is advisory and never gates."),
    "8": ("walked", None, None,
        "IANA Considerations. Names the BMP Parameters registry the five subsections "
        "write to. Walked rather than skipped because 8.3 below carries two derived "
        "sites, and a reader who saw the parent skipped would not look for them."),
    "8.1": ("skipped", "iana", None,
        "Registration of BMP Peer Type 3, Loc-RIB Instance Peer. Binds IANA, not a "
        "speaker."),
    "8.2": ("skipped", "iana", None,
        "Creation of the 'BMP Peer Flags for Loc-RIB Instance Peer Type 3' registry and "
        "registration of flag 0, the F flag. Binds IANA, not a speaker."),
    "8.3": ("walked", None, None,
        "Registration of Peer Up Information TLV type 3, VRF/Table Name. The registry "
        "action binds IANA, but the section restates the TLV's content and size rules "
        "word for word and the derivation reads two sites from that restatement. Walked "
        "rather than skipped so both are visible; both hang on the section 5.2.1 "
        "decision and are held."),
    "8.4": ("skipped", "iana", None,
        "Registration of BMP Peer Down reason code 6, 'Local system closed, TLV data "
        "follows'. Binds IANA, not a speaker; the obligation to USE it is section 5.3."),
    "8.5": ("skipped", "iana", None,
        "Deprecation of the F Flag entry in the 'BMP Peer Flags for Peer Types 0 "
        "through 2' registry. Binds IANA, not a speaker."),
    "9": ("skipped", "references", None,
        "References. A heading over 9.1 and 9.2."),
    "9.1": ("skipped", "references", None,
        "Normative References: RFC 2119, RFC 4271, RFC 7854, RFC 8174."),
    "9.2": ("skipped", "references", None,
        "Informative References: RFC 7911. The section also absorbs the "
        "Acknowledgements and Authors' Addresses blocks, neither of which states an "
        "obligation."),
}

# ---------------------------------------------------------------------------
# Sites with a home in rfc/short/rfc9069.md.
# ---------------------------------------------------------------------------

MAPPED = {
    "4.2:3": ("RFC9069-x-1",
        "The reserved bits of the Peer Type 3 flags byte. For this peer type the "
        "reserved positions ARE the V, L, A and O flags of RFC 7854 and RFC 8671, which "
        "is what RFC9069-x-1 [MUST NOT] renders. Transmit side: locRIBPeerHeader "
        "(internal/component/bgp/plugins/bmp/bmp_locrib.go) sets Flags: 0 literally and "
        "is the only producer of a Peer Type 3 header -- ensureLocRIBPeerUp, "
        "primeLocRIBPeerUp, handleBestChange, closeDumpFamilies and sendLocRIBPeerDown "
        "each call it and none touches Flags afterwards. Receipt side: "
        "PeerHeader.IsIPv6 (internal/component/bgp/plugins/bmp/header.go) answers false "
        "for PeerTypeLocRIB before it reads the V bit, and isPostPolicy, is2ByteAS and "
        "isAdjRIBOut have no non-test caller anywhere in internal/, so no receipt path "
        "reads a reserved bit for any peer type."),
    "5.2:1": ("RFC9069-5.2-1",
        "fabricateLocRIBOpen (internal/component/bgp/plugins/bmp/bmp_locrib.go) builds "
        "the capability list as one capability.ASN4 carrying the router's own 4-octet "
        "ASN plus one capability.Multiprotocol per family of dumpFamilies, and writes "
        "them into the RFC 5492 Section 4 parameter type 2 blob of the fabricated OPEN. "
        "dumpFamilies is the same list closeDumpFamilies closes with End-of-RIB "
        "markers, so what the OPEN advertises and what the dump delivers are one "
        "declaration."),
    "5.3:1": ("RFC9069-5.3-1",
        "sendLocRIBPeerDown (internal/component/bgp/plugins/bmp/bmp_locrib.go) is the "
        "only producer of a Loc-RIB Peer Down and passes the constant PeerDownTLVData "
        "(= 6, internal/component/bgp/plugins/bmp/msg.go) to writePeerDown, which "
        "writes it into the reason octet (writePeerDown, msg.go)."),
    "5.4.1:1": ("RFC9069-5.4.1-1",
        "buildLocRIBUpdateBody (internal/component/bgp/plugins/bmp/bmp_locrib.go) builds "
        "the AS_PATH through attribute.NewBuilder().SetASPath, whose encoder writes "
        "4-byte ASNs unconditionally, and fabricateLocRIBOpen advertises the matching "
        "4-octet ASN capability, so the encoding and the capability agree by "
        "construction rather than by negotiation."),
    "6.1.1:1": ("RFC9069-x-2",
        "ze runs one Loc-RIB instance, the global BGP RIB, and emits exactly one "
        "emulated peer for it: the Loc-RIB Peer Up is claimed behind the per-session "
        "one-shot senderSession.locRIBUpSent, taken by ensureLocRIBPeerUp and "
        "primeLocRIBPeerUp (internal/component/bgp/plugins/bmp/bmp_locrib.go), and "
        "every best change from every BGP peer passes through that same guard. One "
        "instance with one emulated peer satisfies 'at least one for each'."),
    "6.1.1:2": ("RFC9069-5.2-1",
        "The same requirement row, which cites both sections. Every producer of a "
        "Loc-RIB Peer Up -- ensureLocRIBPeerUp and primeLocRIBPeerUp "
        "(internal/component/bgp/plugins/bmp/bmp_locrib.go) -- passes one "
        "fabricateLocRIBOpen result, and that OPEN carries one "
        "capability.Multiprotocol per family of dumpFamilies."),
}

EXCLUDED = {
    "3:1": ("cross-document",
        "Section 3 is the Definitions glossary and this sentence completes the entry "
        "for 'Post-Policy Adj-RIB-Out'. The obligation is RFC 8671's: RFC 8671 "
        "Section 3 carries the identical glossary sentence and RFC 8671 Section 5.1 "
        "states it as the requirement, which ze declares as RFC8671-5.1-1 in "
        "rfc/short/rfc8671.md and rfc/extraction/rfc8671.json maps site 5.1:1 to. "
        "RFC 9069 defines no Adj-RIB-Out behavior of its own and never uses the term "
        "again -- this definition is the only occurrence of 'Post-Policy' in the "
        "source."),
}

# ---------------------------------------------------------------------------
# HELD sites: an obligation with no row in rfc/short/rfc9069.md to map to.
# Each entry is the verdict the walk reached. None of them is written into the
# artifact; they are here so the next session does not re-derive them.
# ---------------------------------------------------------------------------

HELD = {
    "4.2:1": "MET, UNDECLARED. 'If locally sourced routes are communicated using BMP, "
        "they MUST be conveyed using the Loc-RIB Instance Peer Type.' ze has exactly "
        "two producers of a per-peer header: locRIBPeerHeader (PeerTypeLocRIB) and "
        "peerHeaderFromEvent (bmp_events.go, PeerTypeGlobal), and the second is driven "
        "only by an rpc.StructuredEvent naming a real BGP peer session. ze implements "
        "no RFC 7854 Section 8.2 locally-originated-routes peer, so a locally sourced "
        "route reaches a collector only through the Loc-RIB feed.",
    "4.2:2": "MET VACUOUSLY, UNDECLARED. 'This MUST be set when a filter is applied to "
        "Loc-RIB routes sent to the BMP collector.' ze offers no Loc-RIB filter: "
        "ze-bmp-conf.yang has one loc-rib boolean and route-monitoring-policy governs "
        "the Adj-RIB feed only, so the antecedent is unreachable and locRIBPeerHeader "
        "sets Flags: 0. RFC9069-x-1 is about the OTHER bits and does not render this "
        "sentence.",
    "5.1:1": "HELD ON A DEFECT. 'All peer messages that include a per-peer header as "
        "defined in Section 4.2 of [RFC7854] MUST use the following values:' governs "
        "six fields. ze declares three (x-5 Peer Address, x-6 Peer AS, x-7 Peer BGP "
        "ID) and meets those; Peer Type 3 and a zero-filled Peer Distinguisher are met "
        "and undeclared. Timestamp is NOT met on the dump path: locRIBPeerHeader stamps "
        "uint32(time.Now().Unix()) while the RFC's value is 'the time when the "
        "encapsulated routes were installed in the Loc-RIB', and neither "
        "ribevents.BestChangeEntry nor ribevents.BestChangeBatch carries an install "
        "time. Mapping this site to any one of the three rows would sign the whole "
        "sentence off against one field.",
    "5.2.1:1": "MET VACUOUSLY, UNDECLARED. TLV content rule, conditional on emitting "
        "the TLV; ze emits none.",
    "5.2.1:2": "MET VACUOUSLY, UNDECLARED. TLV size rule, conditional on emitting the "
        "TLV; ze emits none.",
    "5.2.1:3": "MET VACUOUSLY, UNDECLARED. 'If a name is configured, it MUST be "
        "included.' ze has no VRF/table-name configuration leaf, so no name can be "
        "configured.",
    "5.2.1:4": "FOUND AND UNMET under the strict reading. 'The default value of "
        "\"global\" MUST be used for the default Loc-RIB instance with a zero-filled "
        "distinguisher.' ze's Loc-RIB IS the default instance with a zero-filled "
        "distinguisher (locRIBPeerHeader leaves Distinguisher at 0) and sends no "
        "VRF/Table Name TLV at all. senderSession.writePeerUpLocked (sender.go) takes "
        "no TLV argument, so the producer does not exist.",
    "5.2.1:5": "MET VACUOUSLY, UNDECLARED. 'If the TLV is included, then it MUST also "
        "be included in the Peer Down notification.' Antecedent false today; falls with "
        "5.2.1:4.",
    "5.2.1:6": "DUPLICATE of 5.2.1:1, stated a second time outside the bullet. Blocked "
        "only because duplicate-of needs the original MAPPED.",
    "5.2.1:7": "DUPLICATE of 5.2.1:2. Blocked for the same reason.",
    "5.2.1:8": "DUPLICATE of 5.2.1:3. Blocked for the same reason.",
    "5.2.1:9": "DUPLICATE of 5.2.1:4. Blocked for the same reason.",
    "5.2.1:10": "DUPLICATE of 5.2.1:5. Blocked for the same reason.",
    "5.2.1:11": "MET, UNDECLARED. 'If multiple strings are included, their ordering "
        "MUST be preserved when they are reported.' Binds the reporting receiver. "
        "DecodeTLVs (internal/component/bgp/plugins/bmp/tlv.go) appends in wire order "
        "and decodePeerUp stores that slice; processPeerUp reports none of them. RFC "
        "8671 Section 6.3.1 states the analogue and ze declares it as RFC8671-6.3.1-1.",
    "5.3:2": "DUPLICATE of 5.2.1:1, restated in the Peer Down section.",
    "5.3:3": "DUPLICATE of 5.2.1:2, restated in the Peer Down section.",
    "5.3:4": "MET VACUOUSLY, UNDECLARED. 'The VRF/Table Name informational TLV MUST be "
        "included if it was in the Peer Up.' ze's Loc-RIB Peer Up carries none, so the "
        "antecedent is false; sendLocRIBPeerDown passes nil data. Falls with 5.2.1:4.",
    "6.1.1:3": "FOUND AND UNMET. 'A BMP receiver MUST process these capabilities to "
        "know which peer belongs to which address family.' ze implements the receiver "
        "role (bmp.go readLoop -> DecodeMsg -> processMessage). processPeerUp "
        "(internal/component/bgp/plugins/bmp/bmp.go) passes the per-peer header to "
        "bmpState.peerUp and logs peer-as, peer-bgp-id and the two ports; it never "
        "parses PeerUp.SentOpenMsg or PeerUp.ReceivedOpenMsg, and no caller of "
        "capability.ParseFromOptionalParams exists in the bmp package outside "
        "bgpIdentityFromSentOpen, which is sender-side.",
    "6.1.2:1": "MET VACUOUSLY, UNDECLARED. 'If multiple filters are associated with the "
        "same Loc-RIB, a table name MUST be used...' ze offers no Loc-RIB filter and no "
        "table name.",
    "6.1.3:1": "MET, UNDECLARED. behaviorOf (sender_config.go) carries the loc-rib leaf "
        "and applySenderConfig acts on a change to it: turning it off sends the RFC "
        "9069 Peer Down and turning it on re-subscribes and re-dumps, so the emulated "
        "peer sees a Peer Down / Peer Up pair. RFC 8671 Section 7.2 states the same "
        "obligation and ze declares that one as RFC8671-7.2-1.",
    "8.3:1": "DUPLICATE of 5.2.1:1, restated in the IANA registration.",
    "8.3:2": "DUPLICATE of 5.2.1:2, restated in the IANA registration.",
}


def main() -> int:
    if not DOC.exists():
        print(f"missing {DOC}; run ./le --name rfc9069walk rfc extraction-create stem rfc9069", file=sys.stderr)
        return 1
    doc = json.loads(DOC.read_text())

    doc["signed-off"] = SIGNED_OFF
    doc["reviewer"] = REVIEWER
    doc["register-reason"] = REGISTER_REASON

    seen_sections = set()
    for section in doc["sections"]:
        spec = SECTIONS.get(section["id"])
        if spec is None:
            print(f"section {section['id']} has no classification", file=sys.stderr)
            return 1
        seen_sections.add(section["id"])
        disposition, skip_kind, _unused, reason = spec[0], spec[1], spec[2], spec[3]
        section["disposition"] = disposition
        if skip_kind:
            section["skip-kind"] = skip_kind
        section["reason"] = reason
        if len(spec) > 4:
            section["unsourced-ids"] = spec[4]
    missing = set(SECTIONS) - seen_sections
    if missing:
        print(f"classification names sections the derivation does not: {sorted(missing)}", file=sys.stderr)
        return 1

    held = 0
    for site in doc["sites"]:
        sid = site["id"]
        if sid in MAPPED:
            rid, reason = MAPPED[sid]
            site["disposition"] = "mapped"
            site["mapped-to"] = rid
            site["reason"] = reason
        elif sid in EXCLUDED:
            kind, reason = EXCLUDED[sid]
            site["disposition"] = "excluded"
            site["excluded-kind"] = kind
            site["reason"] = reason
        elif sid in HELD:
            held += 1
        else:
            print(f"site {sid} is in no table", file=sys.stderr)
            return 1

    DOC.write_text(json.dumps(doc, indent=2) + "\n")
    print(f"classified {len(MAPPED)} mapped, {len(EXCLUDED)} excluded, {held} HELD -> {DOC}")
    print("HELD sites carry no disposition on purpose: the artifact MUST NOT be moved")
    print("into rfc/extraction/ until the owner rules on them.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
