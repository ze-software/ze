# Test weakenings this commit accepts

The BFD two-link prestarts moved from `*-prestart.sh` files in the scenario
directory to Go constants indexed by link-local owner. The removed case tested a
refusal for a state the new type cannot hold, so nothing it guarded is left
unguarded.

| Test | Reason |
|------|--------|
| TestFlowSpecPreferredNumericWideReception | Moved, not lost (spec-plugin-ownership-imports phase D1, owner "approve D1/D2" 2026-10-16): it names the unexported numericComponent, which moved with the FlowSpec wire codec to internal/core/bgp/nlri/flowspec, so the test moved to internal/core/bgp/nlri/flowspec/numeric_wide_reception_test.go and runs there unchanged except the exported names and that IPv4FlowSpec is a family built from core constants. |
| TestIPv6FlowPrefixPatternAndPadding | Moved, not lost (D1, same ruling): names the unexported prefixComponent; now internal/core/bgp/nlri/flowspec/rfc8956_prefix_test.go, unchanged except the exported names and the family built from core constants. |
| TestFlagNamesRoundTrip | Moved, not lost (D1, same ruling): names the unexported tcpFlagsToString; now internal/core/bgp/nlri/flowspec/vocabulary_test.go, unchanged except the exported names. |
| TestProtocolNameReverseAgrees | Moved, not lost (D1, same ruling): names the unexported protocolNumberToName; now internal/core/bgp/nlri/flowspec/vocabulary_test.go, unchanged except the exported names. |
| rfc8955_width_test | Moved, not lost (D1, owner "approve D1/D2" 2026-10-16): the file-level assertion count drops 21 to 15 because TestFlowSpecPreferredNumericWideReception moved to internal/core/bgp/nlri/flowspec/numeric_wide_reception_test.go with its assertions. |
| rfc8956_prefix_test | Moved, not lost (D1, same ruling): the count drops 15 to 6 because TestIPv6FlowPrefixPatternAndPadding moved to internal/core/bgp/nlri/flowspec/rfc8956_prefix_test.go with its assertions. |
| vocabulary_test | Moved, not lost (D1, same ruling): the count drops 19 to 12 because TestFlagNamesRoundTrip and TestProtocolNameReverseAgrees moved to internal/core/bgp/nlri/flowspec/vocabulary_test.go with their assertions. |
