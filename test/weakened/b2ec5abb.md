# Test weakenings this commit accepts

The BFD two-link prestarts moved from `*-prestart.sh` files in the scenario
directory to Go constants indexed by link-local owner. The removed case tested a
refusal for a state the new type cannot hold, so nothing it guarded is left
unguarded.

| Test | Reason |
|------|--------|
| TestExtractLegacyNLRIOverride | Replaced, not lost: extractLegacyNLRIOverride is deleted (no-layering) and the legacy announce block now goes through sectionNLRIDelta/subsetNLRI. Its nine cases are covered, stricter, by TestSectionNLRIDeltaLegacyAnnounce in filter_nlri_delta_test.go (subset, emptied, added, widened, reordered, not-a-prefix, ipv6, duplicate, retired form, no section, more blocks than the table). |
| filter_delta_test | Three assertions left with TestExtractLegacyNLRIOverride, whose behaviour is now judged in filter_nlri_delta_test.go. |
| TestIsCIDRFamily | Renamed, not lost: the table moved verbatim to internal/component/bgp/filtertext/cidr_test.go as TestCIDRFamily, beside the predicate itself. |
| filter_format_test | Assertions 4 to 3: the one that left is TestIsCIDRFamily's, now in filtertext as TestCIDRFamily. |
