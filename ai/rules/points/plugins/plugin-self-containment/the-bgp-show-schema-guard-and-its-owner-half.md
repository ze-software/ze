---
kind: note
level:
stage:
---
The first instance is `TestShowSchemaHasNoBGPPluginCommands`
(`internal/component/cmd/show/yang/self_containment_test.go`): it asserts the
central `show` verb schema declares no part of the `show bgp ...` subtree
(`ze-rib-api:`, `ze-bgp:peer-`, `ze-show:bgp-decode`, `ze-show:bgp-encode`),
because `show bgp rib ...` / `show bgp peer ...` are owned by
`internal/component/bgp/plugins/cmd/{rib,peer}/yang` and the offline
`show bgp decode` / `show bgp encode` diagnostics are owned by
`internal/component/bgp/cli/yang`. The owner half is asserted by
`internal/component/bgp/cli/yang`'s `TestBGPToolsSchemaOwnsDecodeEncode` (the surface moved,
it did not vanish).
