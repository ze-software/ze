// test-relax: this file held temporary diagnostic scaffolding (TestZZDiagFlood,
// TestZZDiagFloodPoll) used only to root-cause the lossy-wire flakiness while
// developing isis-7 flooding. The real, permanent coverage lives in
// flooding_wiring_test.go (TestISISLSDBSync, TestISISFloodSRMTimer,
// TestISISCSNPGapRequest, TestISISPSNPAck) and the lsdb-package unit tests. The
// diagnostics are removed now that the cause (the test wire dropped frames) is
// fixed by the lossless relWire harness; they were never spec deliverables.
package isis
