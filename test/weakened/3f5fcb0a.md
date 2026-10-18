| Test | Reason |
|---|---|
| TestLoaderRefusesUncheckedTypes | Deleted. owner 2026-10-18: implement leafref/identityref/instance-identifier, replacing the refusal these tests pinned. The loader now accepts the three kinds and checks them (rfc7950_reference_test.go). |
| validator_lexical_test | owner 2026-10-18: implement leafref/identityref/instance-identifier, replacing the refusal these tests pinned. The assertion count fell by the deleted test above and by the three kinds TestValidateUncheckedKindsRefused no longer covers, whose assertions moved to rfc7950_reference_test.go; TestValidateLeafrefNeedsLeaf was added. |
