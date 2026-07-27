// VALIDATES: the ze-analyze subcommands WIRE their damage counters -- a damaged
// MRT record reaches the operator on stderr instead of silently skewing the
// numbers on stdout.
// PREVENTS: the failure that made this necessary. Counting the damage in a
// helper proves nothing if no subcommand calls it: `density` had no counter at
// all, so one corrupt NLRI octet produced a wrong burst profile with exit 0 and
// no warning. These tests drive the real entry points (runDensity,
// runCountAttrs, runMRTDump) over a crafted damaged file, which is the only
// thing that can fail when a `damaged.note(...)` call is dropped
// (ai/rules/integration-completeness.md).
package analyze

import (
	"bytes"
	"encoding/binary"
	"io"
	"os"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/cliio"
	"github.com/ze-software/ze/internal/mrt"
)

// mrtRecord wraps a payload in the 12-byte MRT common header (RFC 6396 Section 2).
func mrtRecord(mrtType, subtype uint16, payload []byte) []byte {
	rec := make([]byte, 12+len(payload))
	binary.BigEndian.PutUint32(rec[0:4], 1000) // timestamp
	binary.BigEndian.PutUint16(rec[4:6], mrtType)
	binary.BigEndian.PutUint16(rec[6:8], subtype)
	binary.BigEndian.PutUint32(rec[8:12], uint32(len(payload)))
	copy(rec[12:], payload)
	return rec
}

// runSubcommand drives a ze-analyze subcommand over stdin and returns its exit
// code and everything it wrote to stderr.
//
// Both real streams are replaced: the subcommands write their reports with
// fmt.Println to os.Stdout and their warnings to os.Stderr directly, so neither
// can be observed (or kept out of the test log) through cliio alone.
func runSubcommand(t *testing.T, stdin []byte, fn func() int) (exitCode int, stderr string) {
	t.Helper()
	code, _, errOut := runSubcommandCapturing(t, stdin, fn)
	return code, errOut
}

// runSubcommandCapturing is runSubcommand plus the subcommand's stdout, for the
// tests that need to assert on the report itself and not only on the warning.
func runSubcommandCapturing(t *testing.T, stdin []byte, fn func() int) (exitCode int, stdout, stderr string) {
	t.Helper()

	restore := cliio.SwapStreams(bytes.NewReader(stdin), &bytes.Buffer{})
	defer restore()

	outR, outW, err := os.Pipe()
	require.NoError(t, err)
	errR, errW, err := os.Pipe()
	require.NoError(t, err)

	origOut, origErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = outW, errW

	// Drain both pipes concurrently: a subcommand writing more than the pipe
	// buffer would otherwise block forever on stdout and hang the test.
	var wg sync.WaitGroup
	var outBuf, errBuf bytes.Buffer
	wg.Add(2)
	go func() {
		defer wg.Done()
		io.Copy(&outBuf, outR) //nolint:errcheck // drain; a read error just ends the copy
	}()
	go func() {
		defer wg.Done()
		io.Copy(&errBuf, errR) //nolint:errcheck // drain; a read error just ends the copy
	}()

	exitCode = fn()

	os.Stdout, os.Stderr = origOut, origErr
	require.NoError(t, outW.Close())
	require.NoError(t, errW.Close())
	wg.Wait()
	require.NoError(t, outR.Close())
	require.NoError(t, errR.Close())

	return exitCode, outBuf.String(), errBuf.String()
}

// damagedBGP4MPFile is a one-record MRT stream whose UPDATE carries an NLRI
// field that claims a /24 but supplies only two of its three octets.
func damagedBGP4MPFile() []byte {
	body := buildUpdate(nil, nil, []byte{24, 10, 0})
	return mrtRecord(mrtBGP4MP, subtypeBGP4MPMessageAS4,
		makeBGP4MPRecord(4, 65000, 1, body))
}

// cleanBGP4MPFile is the same record with an intact NLRI.
func cleanBGP4MPFile() []byte {
	body := buildUpdate(nil, nil, []byte{24, 10, 0, 0})
	return mrtRecord(mrtBGP4MP, subtypeBGP4MPMessageAS4,
		makeBGP4MPRecord(4, 65000, 1, body))
}

// damagedRIBFile is a one-record TABLE_DUMP_V2 stream whose RIB record is too
// short to decode.
func damagedRIBFile() []byte {
	return mrtRecord(mrtTableDumpV2, mrt.TDV2RIBIPv4Unicast, []byte{0, 1})
}

func TestRunDensity_ReportsDamagedUpdate(t *testing.T) {
	// VALIDATES: `ze-analyze density` warns on stderr when an UPDATE's NLRI
	// cannot be counted, and stays silent when the input is clean.
	// PREVENTS: the B1 blocker -- a burst profile computed from a silently
	// truncating counter, published as a measurement with exit 0.
	code, stderr := runSubcommand(t, damagedBGP4MPFile(), func() int {
		return runDensity([]string{"-"})
	})
	assert.Equal(t, 0, code)
	assert.Contains(t, stderr, "malformed MRT record",
		"density must tell the operator its figures are incomplete")

	code, stderr = runSubcommand(t, cleanBGP4MPFile(), func() int {
		return runDensity([]string{"-"})
	})
	assert.Equal(t, 0, code)
	assert.NotContains(t, stderr, "malformed MRT record",
		"a clean file must produce no warning, or the warning is noise")
}

func TestRunCountAttrs_ReportsDamagedRIBRecord(t *testing.T) {
	// VALIDATES: `ze-analyze count-attrs` warns when a RIB record fails to
	// decode, so its attribute distribution is not read as complete.
	// PREVENTS: dropping the damaged.note(...) around forEachRIBEntry, which no
	// test previously covered.
	code, stderr := runSubcommand(t, damagedRIBFile(), func() int {
		return runCountAttrs([]string{"-"})
	})
	assert.Equal(t, 0, code)
	assert.Contains(t, stderr, "malformed MRT record")
}

func TestRunMRTDump_ReportsDamagedRIBRecord(t *testing.T) {
	// VALIDATES: `ze-analyze mrt-dump` warns when a RIB record fails to decode.
	// PREVENTS: a truncated hex dump being piped into `ze bgp decode` as though
	// it were the whole file.
	code, stderr := runSubcommand(t, damagedRIBFile(), func() int {
		return runMRTDump([]string{"-"})
	})
	assert.Equal(t, 0, code)
	assert.Contains(t, stderr, "malformed MRT record")
}

func TestProcessMRTFile_DispatchesAddPathRIBSubtypes(t *testing.T) {
	// VALIDATES: the RFC 8050 add-path RIB subtypes (8-12) reach OnRIB.
	// PREVENTS: the subcommand reader silently ignoring them. forEachRIBEntry
	// handles the 4-octet Path Identifier and says so, but this dispatch listed
	// only subtypes 2-6, so that handling was unreachable: an add-path RIB dump
	// produced NO routes from count-attrs, mrt-dump, aspath, attributes or
	// communities -- an empty analysis presented as a complete one, exit 0.
	addPathSubtypes := []uint16{
		mrt.TDV2RIBIPv4UnicastAP, mrt.TDV2RIBIPv4MulticastAP,
		mrt.TDV2RIBIPv6UnicastAP, mrt.TDV2RIBIPv6MulticastAP,
		mrt.TDV2RIBGenericAP,
	}
	for _, subtype := range addPathSubtypes {
		file := mrtRecord(mrtTableDumpV2, subtype, []byte{0, 0, 0, 1, 8, 10, 0, 0})

		var seen int
		restore := cliio.SwapStreams(bytes.NewReader(file), &bytes.Buffer{})
		err := processMRTFile("-", mrtHandler{
			OnRIB: func([]byte, uint16) { seen++ },
		})
		restore()

		require.NoError(t, err)
		assert.Equal(t, 1, seen, "add-path RIB subtype %d must reach OnRIB", subtype)
	}

	// The non-add-path subtypes must keep working.
	file := mrtRecord(mrtTableDumpV2, mrt.TDV2RIBIPv4Unicast, []byte{0, 0, 0, 1, 8, 10, 0, 0})
	var seen int
	restore := cliio.SwapStreams(bytes.NewReader(file), &bytes.Buffer{})
	err := processMRTFile("-", mrtHandler{
		OnRIB: func([]byte, uint16) { seen++ },
	})
	restore()
	require.NoError(t, err)
	assert.Equal(t, 1, seen, "the plain RIB subtypes must still dispatch")
}

func TestRunASPath_ReportsDamagedASPath(t *testing.T) {
	// VALIDATES: `ze-analyze aspath` warns when an AS_PATH attribute cannot be
	// decoded, rather than excluding it from every statistic in silence.
	// PREVENTS: a file of unreadable paths reporting a clean, small, entirely
	// fictitious distribution.
	//
	// The AS_PATH here is 2-byte encoded (one segment, two ASNs) but the record
	// subtype is BGP4MP_MESSAGE_AS4, so the correct 4-byte read overruns it --
	// which is exactly what the operator needs to be told.
	asPath := []byte{2, 2, 0xFD, 0xE8, 0xFD, 0xE9} // AS_SEQUENCE, 2 ASNs, 2-byte
	attrs := makeAttr(0x40, attrASPath, asPath)
	body := buildUpdate(nil, attrs, nil)
	file := mrtRecord(mrtBGP4MP, subtypeBGP4MPMessageAS4,
		makeBGP4MPRecord(4, 65000, 1, body))

	code, stderr := runSubcommand(t, file, func() int {
		return runASPath([]string{"-"})
	})
	assert.Equal(t, 0, code)
	assert.Contains(t, stderr, "AS_PATH attribute(s) failed to decode")
}

func TestRunASPath_TwoByteBGP4MPRecordIsDecodedNotMangled(t *testing.T) {
	// VALIDATES: `ze-analyze aspath` reads a BGP4MP_MESSAGE (subtype 1) AS_PATH
	// at 2-byte width, per RFC 6396 Section 4.4.2, and reports the real ASNs.
	// PREVENTS: the hardcoded 4-byte read this replaced. That is the ONLY
	// direction that catches it: on an _AS4 record a 2-byte path fails either
	// way, so a test built on the AS4 subtype passes with the defect present.
	// Here the correct width decodes cleanly and the hardcoded one overruns, so
	// the assertion moves when the code does.
	asPath := []byte{2, 2, 0xFD, 0xE8, 0xFD, 0xE9} // AS_SEQUENCE 65000 65001, 2-byte
	body := buildUpdate(nil, makeAttr(0x40, attrASPath, asPath), nil)
	file := mrtRecord(mrtBGP4MP, subtypeBGP4MPMessage,
		makeBGP4MPRecord(2, 65000, 1, body))

	code, stdout, stderr := runSubcommandCapturing(t, file, func() int {
		return runASPath([]string{"-"})
	})
	require.Equal(t, 0, code)

	assert.NotContains(t, stderr, "failed to decode",
		"a correctly-encoded 2-byte AS_PATH must decode without complaint")
	assert.Contains(t, stdout, `"total-paths": 1`,
		"the path must be counted, not discarded as unreadable")
	assert.Contains(t, stdout, `"damaged-paths": 0`)
	// The trie is reversed, so the origin ASN (65001) is a root child.
	assert.Contains(t, stdout, `"asn": 65001`,
		"the real ASNs must appear; a 4-byte read would fabricate or drop them")
}

// damagedASPathRIBFile is a TABLE_DUMP_V2 RIB record whose AS_PATH is 2-byte
// encoded. TABLE_DUMP_V2 is 4-byte throughout (RFC 6396 Section 4.3.4), so the
// correct read overruns it: a genuinely damaged record.
func damagedASPathRIBFile() []byte {
	attrs := []byte{0x40, 0x02, 0x06, 2, 2, 0xFD, 0xE8, 0xFD, 0xE9}
	payload := []byte{
		0, 0, 0, 1, // sequence number
		8, 10, // 10.0.0.0/8
		0, 1, // entry count
		0, 0, // peer index
		0, 0, 0, 0, // originated time
		0, byte(len(attrs)), // attribute length
	}
	payload = append(payload, attrs...)
	return mrtRecord(mrtTableDumpV2, mrt.TDV2RIBIPv4Unicast, payload)
}

func TestRunRoutes_ReportsDamagedASPath(t *testing.T) {
	// VALIDATES: `ze-analyze routes` warns on stderr when a route's AS_PATH
	// cannot be decoded, and still emits the route.
	// PREVENTS: dropping the damaged.note(recErr) wiring. The JSON on stdout
	// carries an EMPTY as-path for such a route, which every consumer reads as
	// "locally originated" -- a wrong answer that looks like a right one, with
	// nothing on stderr and exit 0.
	code, stdout, stderr := runSubcommandCapturing(t, damagedASPathRIBFile(), func() int {
		return runRoutes([]string{"-"})
	})
	require.Equal(t, 0, code)
	assert.Contains(t, stderr, "malformed MRT record",
		"an undecodable AS_PATH must be reported, not emitted as an empty path")
	assert.Contains(t, stdout, `"prefix":"10.0.0.0/8"`,
		"the route is still emitted: the prefix is usable even when the path is not")
}

func TestRunRoutes_CleanFileProducesNoWarning(t *testing.T) {
	// VALIDATES: a clean RIB record produces no warning.
	// PREVENTS: a counter that fires on every record, which is noise an
	// operator learns to ignore.
	attrs := []byte{0x40, 0x02, 0x06, 2, 1, 0x00, 0x00, 0xFD, 0xE8} // 4-byte, 1 ASN
	payload := []byte{
		0, 0, 0, 1,
		8, 10,
		0, 1,
		0, 0,
		0, 0, 0, 0,
		0, byte(len(attrs)),
	}
	payload = append(payload, attrs...)
	file := mrtRecord(mrtTableDumpV2, mrt.TDV2RIBIPv4Unicast, payload)

	code, stdout, stderr := runSubcommandCapturing(t, file, func() int {
		return runRoutes([]string{"-"})
	})
	require.Equal(t, 0, code)
	assert.NotContains(t, stderr, "malformed MRT record")
	assert.Contains(t, stdout, `"as-path":[65000]`)
}

func TestRunASPath_AS4PathIsAlwaysFourByte(t *testing.T) {
	// VALIDATES: AS4_PATH (type 17) is parsed with four-octet AS numbers even
	// inside a 2-byte record, per RFC 6793 Section 4 ("encoded with four-octet
	// AS numbers"). The record's own width governs AS_PATH, never AS4_PATH.
	// PREVENTS: applying the record width uniformly. That misread does NOT
	// error -- a 4-byte AS4_PATH read at 2-byte width decodes into plausible
	// small ASNs -- so only asserting the ASN VALUE catches it. Here AS 65536
	// (the first ASN that needs four octets) would silently become AS 1.
	as4Path := []byte{2, 1, 0x00, 0x01, 0x00, 0x00} // AS_SEQUENCE, AS 65536
	body := buildUpdate(nil, makeAttr(0xC0, attrAS4Path, as4Path), nil)
	file := mrtRecord(mrtBGP4MP, subtypeBGP4MPMessage,
		makeBGP4MPRecord(2, 65000, 1, body))

	code, stdout, stderr := runSubcommandCapturing(t, file, func() int {
		return runASPath([]string{"-"})
	})
	require.Equal(t, 0, code)
	assert.NotContains(t, stderr, "failed to decode")
	assert.Contains(t, stdout, `"asn": 65536`,
		"AS4_PATH carries 4-octet ASNs regardless of the record's AS width")
	assert.NotContains(t, stdout, `"asn": 1,`,
		"reading AS4_PATH at the record's 2-byte width fabricates AS 1")
}
