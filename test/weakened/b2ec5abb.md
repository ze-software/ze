# Test weakenings this commit accepts

The BFD two-link prestarts moved from `*-prestart.sh` files in the scenario
directory to Go constants indexed by link-local owner. The removed case tested a
refusal for a state the new type cannot hold, so nothing it guarded is left
unguarded.

| Test | Reason |
|------|--------|
| TestDescriptorBGPRouterID | Moved, not lost (spec-plugin-ownership-imports phase D2, owner "approve D1/D2" 2026-10-16): it names the unexported node descriptor parser, which moved with the BGP-LS wire codec to internal/core/bgp/nlri/bgpls, so the test moved to internal/core/bgp/nlri/bgpls/descriptor_test.go and runs there unchanged except the exported names. |
| TestDescriptorConfedMember | Moved, not lost (D2, same ruling): names the unexported node descriptor parser; now internal/core/bgp/nlri/bgpls/descriptor_test.go, unchanged. |
| TestDescriptorSRv6SID | Moved, not lost (D2, same ruling): names the unexported parseBGPLS; now internal/core/bgp/nlri/bgpls/descriptor_test.go, unchanged. |
| TestFormatRouterID | Moved, not lost (D2, same ruling): names the unexported formatRouterID, which moved to internal/core/bgp/nlri/bgpls/descriptor_info.go; now internal/core/bgp/nlri/bgpls/descriptor_info_test.go, unchanged. |
| TestRFC9552NLRIContentsNeverMakeItMalformed | Moved, not lost (D2, same ruling): names the unexported parseBGPLS; now internal/core/bgp/nlri/bgpls/rfc9552_test.go with its helper nodeNLRIOddButLegal and core copies of buildAttrTLV and unknownAttrTLVType, unchanged. |
| TestRFC9552NLRIFramingErrorsRejected | Moved, not lost (D2, same ruling): names the unexported parseBGPLS; now internal/core/bgp/nlri/bgpls/rfc9552_test.go, unchanged. |
| nodeNLRIOddButLegal | Moved, not lost (D2, same ruling): the helper of TestRFC9552NLRIContentsNeverMakeItMalformed, moved with it to internal/core/bgp/nlri/bgpls/rfc9552_test.go. |
| attr_test | Moved, not lost (D2, same ruling): the file-level assertion count drops 185 to 176 because TestDescriptorBGPRouterID, TestDescriptorConfedMember and TestDescriptorSRv6SID moved to internal/core/bgp/nlri/bgpls/descriptor_test.go with their assertions; the file also gains TestSRCapabilitiesDecoderReturnsNilInterfaceOnError. |
| plugin_test | Moved, not lost (D2, same ruling): the count drops 51 to 50 because TestFormatRouterID moved to internal/core/bgp/nlri/bgpls/descriptor_info_test.go with its assertions. |
| rfc9552_test | Moved, not lost (D2, same ruling): the count drops 42 to 30 because TestRFC9552NLRIContentsNeverMakeItMalformed and TestRFC9552NLRIFramingErrorsRejected moved to internal/core/bgp/nlri/bgpls/rfc9552_test.go with their assertions. |
