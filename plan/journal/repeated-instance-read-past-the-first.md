# A repeated instance is read past the first

A format says the first instance of a field counts and later ones are ignored.
The reader loops over every instance, so a second copy adds data the format
says to drop, and every test that sends one instance stays green.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-10-20 | - (rfc8362 test triage, walked into) | `bgplsArea.extendedV3` (`internal/plugins/ospf/bgpls_export.go`), `engine.v6ReceivedPrefixSIDs` (`sr_reception_v6.go`) | Both readers use every Attached-Routers, Inter-Area-Prefix and External-Prefix TLV in an LSA, so a second instance adds links, prefixes and Prefix-SIDs. RFC 8362 Section 4.2: "Instances of the Attached-Router TLV subsequent to the first MUST be ignored."; Sections 4.3 and 4.5 say the same. Read at both producers | open: keep the first instance per LSA in both readers, negative test with two instances |
