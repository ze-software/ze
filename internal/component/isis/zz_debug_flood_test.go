package isis

// test-relax: this file held only TestDebugFloodTimeline2, a throwaway diagnostic
// added this session to trace the intermittent IS-IS flooding stall (P2P
// clear-on-send dropping a lost first transmission, and a PSNP request echoing
// the holder's sequence so the holder read it as an ack). Both bugs are now fixed
// (clear-on-ack in lsdb/flooding.go; seq-0 request in lsdb/snp.go). Permanent
// coverage lives in TestISISLSDBSync (flooding_wiring_test.go), TestSRMDrivenSend
// and TestCSNPGapRequestPending (lsdb). The diagnostic is removed now.
