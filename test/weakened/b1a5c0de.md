# Session b1a5c0de, 2026-10-08

MPLS fragmentation: a test that can livelock a stock kernel runs only where the
kernel capability probe proves the patched datapath. Published rpc names: an
assertion on a deleted dead -api module leaves the suite with the module.

| Test | Reason |
|------|--------|
| TestWireModule | Removed with the function it tested: WireModule, which built a wire method from a module's file name, is deleted (owner decision Q3). TestPublishedRPCsTakeTheMethodOfThePointingNode and TestPublishedRPCsRefuseABrokenPointer in the same file assert the method now comes from the pointing ze:command node and that a broken pointer is refused. |
| TestCmdMethods | Hard-coded per-module row counts became one derived assertion per rpc: every rpc ze-bgp-api, ze-system-api, ze-plugin-api and ze-rib-api declare must be published under some method, and each module must declare at least one rpc. A count passed when one rpc vanished and another was added, and named nothing. |
| TestPublishedRPCsRefuseALoaderWithNoAPIModule | Renamed, not removed: it is TestPublishedRPCsRefuseALoaderThatPublishesNoRPC with the same body and assertion, because the gate no longer selects modules by the -api file suffix; errNoAPIModule became errNoPublishedRPC. |
