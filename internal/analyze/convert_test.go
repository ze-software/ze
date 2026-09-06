package analyze

import (
	"bytes"
	"encoding/binary"
	"net/netip"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/pcap"
	"github.com/ze-software/ze/internal/mrt"
)

// TestConvertFlow covers what the conversion now drops and what it keeps. The
// IPv6 case is the point: before internal/core/pcap took over the framing, an
// IPv6 record was counted into skippedV6 and thrown away, because link type 228
// carries IPv4 alone.
func TestConvertFlow(t *testing.T) {
	tests := []struct {
		name    string
		peerIP  []byte
		localIP []byte
		wantOK  bool
		want    pcap.Flow
	}{
		{
			name:    "ipv4",
			peerIP:  []byte{10, 0, 0, 1},
			localIP: []byte{10, 0, 0, 2},
			wantOK:  true,
			want: pcap.Flow{
				SourceAddr: netip.MustParseAddr("10.0.0.1"),
				TargetAddr: netip.MustParseAddr("10.0.0.2"),
				SourcePort: bgpPort,
				TargetPort: bgpPort,
			},
		},
		{
			name:    "ipv6 is converted, not skipped",
			peerIP:  netip.MustParseAddr("2001:db8::1").AsSlice(),
			localIP: netip.MustParseAddr("2001:db8::2").AsSlice(),
			wantOK:  true,
			want: pcap.Flow{
				SourceAddr: netip.MustParseAddr("2001:db8::1"),
				TargetAddr: netip.MustParseAddr("2001:db8::2"),
				SourcePort: bgpPort,
				TargetPort: bgpPort,
			},
		},
		{
			name:    "ipv4-mapped is unmapped to IPv4",
			peerIP:  []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0xFF, 0xFF, 192, 168, 1, 1},
			localIP: []byte{192, 168, 1, 2},
			wantOK:  true,
			want: pcap.Flow{
				SourceAddr: netip.MustParseAddr("192.168.1.1"),
				TargetAddr: netip.MustParseAddr("192.168.1.2"),
				SourcePort: bgpPort,
				TargetPort: bgpPort,
			},
		},
		{name: "nil peer", peerIP: nil, localIP: []byte{10, 0, 0, 2}},
		{name: "short peer", peerIP: []byte{1, 2}, localIP: []byte{10, 0, 0, 2}},
		{name: "nil local", peerIP: []byte{10, 0, 0, 1}, localIP: nil},
		{
			name:    "mixed families",
			peerIP:  []byte{10, 0, 0, 1},
			localIP: netip.MustParseAddr("2001:db8::2").AsSlice(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := convertFlow(tt.peerIP, tt.localIP)
			assert.Equal(t, tt.wantOK, ok)
			if tt.wantOK {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

// TestRunConvertPcapFramesBothFamilies drives the whole command over an MRT
// file holding one IPv4 and one IPv6 message, and reads the pcap back through
// the reassembler. Both messages must come out: an IPv6 record used to be
// counted and dropped.
func TestRunConvertPcapFramesBothFamilies(t *testing.T) {
	for _, tt := range []struct {
		name    string
		afi     uint16
		peerIP  []byte
		localIP []byte
	}{
		{"ipv4", mrt.AFIIPv4, []byte{10, 0, 0, 1}, []byte{10, 0, 0, 2}},
		{"ipv6", mrt.AFIIPv6, netip.MustParseAddr("2001:db8::1").AsSlice(), netip.MustParseAddr("2001:db8::2").AsSlice()},
	} {
		t.Run(tt.name, func(t *testing.T) {
			mrtPath := writeTempMRTAFI(t, tt.afi, tt.peerIP, tt.localIP)
			pcapPath := filepath.Join(t.TempDir(), "out.pcap")

			code := runConvertPcap([]string{mrtPath, pcapPath})
			require.Equal(t, 0, code)

			file, err := os.Open(pcapPath)
			require.NoError(t, err)
			defer func() { _ = file.Close() }()

			streams, report, err := pcap.Reassemble(file, bgpPort)
			require.NoError(t, err)
			require.Len(t, streams, 1, "report: %s", report)
			assert.Equal(t, keepaliveBytes(), streams[0].Bytes)
			peer, _ := netip.AddrFromSlice(tt.peerIP)
			assert.Equal(t, peer, streams[0].Flow.SourceAddr,
				"the record must be framed from the peer, in the peer's own family")
		})
	}
}

func TestRunConvertJSON(t *testing.T) {
	mrtFile := writeTempMRT(t)

	old := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w

	code := runConvertJSON([]string{mrtFile})

	require.NoError(t, w.Close())
	os.Stdout = old

	assert.Equal(t, 0, code)

	var buf bytes.Buffer
	_, err = buf.ReadFrom(r)
	require.NoError(t, err)
	output := buf.String()

	assert.True(t, len(output) > 4, "should produce JSON output")
	assert.True(t, output[0] == '[', "should start with [")
	assert.Contains(t, output, `"type":`)
	assert.Contains(t, output, `"subtype":`)
}

func TestRunConvert_UnknownFormat(t *testing.T) {
	code := runConvert([]string{"xyz", "input.mrt"})
	assert.Equal(t, 1, code)
}

func TestRunConvert_TooFewArgs(t *testing.T) {
	code := runConvert([]string{"pcap"})
	assert.Equal(t, 1, code)
}

func writeTempMRT(t *testing.T) string {
	t.Helper()
	return writeTempMRTAFI(t, mrt.AFIIPv4, []byte{10, 0, 0, 1}, []byte{10, 0, 0, 2})
}

// keepaliveBytes is the 19 octets of a BGP KEEPALIVE: the all-ones marker,
// length 19, type 4 (RFC 4271 Section 4.4).
func keepaliveBytes() []byte {
	msg := make([]byte, 19)
	for i := range 16 {
		msg[i] = 0xff
	}
	binary.BigEndian.PutUint16(msg[16:], 19)
	msg[18] = 4
	return msg
}

func writeTempMRTAFI(t *testing.T, afi uint16, peerIP, localIP []byte) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.mrt")
	f, err := os.Create(path)
	require.NoError(t, err)
	defer func() { _ = f.Close() }()

	hdr := &mrt.BGP4MPHeader{
		PeerAS:  65000,
		LocalAS: 65001,
		AFI:     afi,
		PeerIP:  peerIP,
		LocalIP: localIP,
	}

	buf := make([]byte, 4096)
	off := mrt.CommonHeaderLen
	msgLen := mrt.WriteBGP4MPMessage(buf, off, hdr, true, keepaliveBytes())
	mrt.WriteCommonHeader(buf, 0, 1700000000, mrt.TypeBGP4MP, mrt.BGP4MPMessageAS4, uint32(msgLen))

	_, err = f.Write(buf[:off+msgLen])
	require.NoError(t, err)

	return path
}
