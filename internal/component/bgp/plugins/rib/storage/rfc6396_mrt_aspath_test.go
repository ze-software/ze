// RFC: rfc/short/rfc6396.md -- TABLE_DUMP_V2 RIB entries carry 4-byte AS numbers
//
// Drives ParseAttributes (attrparse.go), which is where an MRT TABLE_DUMP_V2 RIB
// entry's AS_PATH actually acquires its 4-byte encoding. The MRT side copies:
// rib_mrt.go reads the interned entry.ASPath into the record's attribute blob,
// and internal/mrt/encode.go WriteRIBEntry writes those bytes verbatim. So the
// obligation is met here, on ingest, and nowhere downstream.

package storage

// This file holds no test. TestRFC6396RIBEntryASPathStoredFourByte and
// TestRFC6396RIBEntryASPathFourByteSessionUnchanged moved to
// internal/core/bgp/attribute/rfc6793_reconcile_test.go on 2026-09-09, under
// the same names, when the RFC 6793 reconciliation moved there. Both drove a
// function this package no longer holds.
//
// The file is empty and awaits deletion, which is the owner's to approve.
