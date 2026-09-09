package wireu

// This file is EMPTY of declarations, and that is deliberate rather than an
// oversight. It held the three tests for the
// draft-mangin-idr-attr-tombstone-00 Section 5.3 Transitive clear at the EBGP
// boundary, plus the two payload helpers they shared with tombstone_test.go.
// Thomas retired that support on 2026-09-09 and the code performing it was
// deleted with the whole-payload AS_PATH rewrite (test/weakened/49b0956f.md,
// plan/journal/unwired-feature.md). Removing the last reader of the two helpers
// left them unused, so they went too.
//
// The file itself stays on disk because deleting a test file needs the owner.
