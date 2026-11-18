// Design: docs/guide/rpki.md -- file-driven complete mock RTR snapshots.
// Related: rtr.go -- the existing RTR protocol responder.
// RFC 8210 Section 8.3 -- Cache Reset precedes a replacement full set.
package rtr

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

const rtrMockSnapshotBytes = 64 * 1024

// rtrMockReadVRPs reads one immutable test-owned snapshot. The first line is a
// nonzero uint32 serial; each following nonempty line is prefix,maxlen,asn.
// There is no snapshot history or mutable cache shared across connections.
func rtrMockReadVRPs(path string) (uint32, vrpList, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, nil, err
	}
	defer file.Close() //nolint:errcheck // The read or parse result owns failure.
	data, err := io.ReadAll(io.LimitReader(file, rtrMockSnapshotBytes+1))
	if err != nil {
		return 0, nil, err
	}
	if len(data) > rtrMockSnapshotBytes {
		return 0, nil, fmt.Errorf("VRP snapshot exceeds %d bytes", rtrMockSnapshotBytes)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	serial, err := strconv.ParseUint(lines[0], 10, 32)
	if err != nil {
		return 0, nil, fmt.Errorf("VRP snapshot serial: %w", err)
	}
	if serial == 0 {
		return 0, nil, fmt.Errorf("VRP snapshot serial must be nonzero")
	}
	var vrps vrpList
	seen := make(map[string]bool)
	for i, line := range lines[1:] {
		if line == "" {
			return 0, nil, fmt.Errorf("VRP snapshot line %d is empty", i+2)
		}
		if err := vrps.Set(line); err != nil {
			return 0, nil, fmt.Errorf("VRP snapshot line %d: %w", i+2, err)
		}
		row := vrps[len(vrps)-1]
		prefixLength, bits := row.prefix.Mask.Size()
		if int(row.maxLength) < prefixLength {
			return 0, nil, fmt.Errorf("VRP snapshot line %d maxlen is shorter than prefix", i+2)
		}
		if int(row.maxLength) > bits {
			return 0, nil, fmt.Errorf("VRP snapshot line %d maxlen exceeds address width", i+2)
		}
		key := fmt.Sprintf("%s,%d,%d", row.prefix.String(), row.maxLength, row.asn)
		if seen[key] {
			return 0, nil, fmt.Errorf("VRP snapshot line %d duplicates a record", i+2)
		}
		seen[key] = true
	}
	return uint32(serial), vrps, nil //nolint:gosec // ParseUint limited serial to 32 bits.
}

// rtrMockFileQuery preserves the existing fixed mock path when vrpFile is
// absent. A changed serial gets Cache Reset: the file describes a complete
// current set and cannot stand in for a delta which withdraws old records.
// The router then sends Reset Query, receiving the whole current snapshot.
func rtrMockFileQuery(conn io.Writer, cache *rtrMockCache, version, pduType uint8, querySerial uint32) error {
	serial, vrps, err := rtrMockReadVRPs(cache.vrpFile)
	if err != nil {
		return err
	}
	if pduType == rtrMockPDUSerialQuery {
		if querySerial != serial {
			_, err := conn.Write([]byte{version, 8, 0, 0, 0, 0, 0, 8})
			return err
		}
		vrps = nil // An unchanged serial has an empty delta, never a full-set merge.
	}
	snapshot := *cache
	snapshot.serial = serial
	snapshot.vrps = vrps
	return rtrMockSendResponse(conn, &snapshot, version)
}
