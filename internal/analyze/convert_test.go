package analyze

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/mrt"
)

func fixedTime() time.Time {
	return time.Unix(1700000000, 0)
}

func TestWritePcapGlobalHeader(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "test-*.pcap")
	require.NoError(t, err)
	defer func() { _ = f.Close() }()

	err = writePcapGlobalHeader(f)
	require.NoError(t, err)

	_, err = f.Seek(0, 0)
	require.NoError(t, err)

	var hdr [24]byte
	_, err = f.Read(hdr[:])
	require.NoError(t, err)

	magic := binary.LittleEndian.Uint32(hdr[0:4])
	assert.Equal(t, uint32(0xa1b2c3d4), magic, "pcap magic number")

	major := binary.LittleEndian.Uint16(hdr[4:6])
	assert.Equal(t, uint16(2), major, "version major")

	minor := binary.LittleEndian.Uint16(hdr[6:8])
	assert.Equal(t, uint16(4), minor, "version minor")

	snaplen := binary.LittleEndian.Uint32(hdr[16:20])
	assert.Equal(t, uint32(65535), snaplen)

	linkType := binary.LittleEndian.Uint32(hdr[20:24])
	assert.Equal(t, uint32(228), linkType, "LINKTYPE_IPV4")
}

func TestWritePcapBGPPacket(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "test-*.pcap")
	require.NoError(t, err)
	defer func() { _ = f.Close() }()

	bgpMsg := make([]byte, 19)
	for i := range 16 {
		bgpMsg[i] = 0xff
	}
	binary.BigEndian.PutUint16(bgpMsg[16:], 19)
	bgpMsg[18] = 4 // KEEPALIVE

	srcIP := []byte{10, 0, 0, 1}
	dstIP := []byte{10, 0, 0, 2}

	err = writePcapBGPPacket(f, fixedTime(), srcIP, dstIP, bgpMsg)
	require.NoError(t, err)

	data, err := os.ReadFile(f.Name())
	require.NoError(t, err)

	// pcap record header (16) + IPv4 (20) + TCP (20) + BGP (19) = 75
	assert.Equal(t, 16+20+20+19, len(data))

	// IPv4 header at offset 16.
	assert.Equal(t, byte(0x45), data[16], "IPv4 version+IHL")
	assert.Equal(t, byte(6), data[16+9], "protocol TCP")
	assert.Equal(t, srcIP, data[16+12:16+16], "source IP")
	assert.Equal(t, dstIP, data[16+16:16+20], "dest IP")

	// TCP header at offset 36.
	srcPort := binary.BigEndian.Uint16(data[36:38])
	assert.Equal(t, uint16(179), srcPort)

	// BGP payload at offset 56.
	assert.Equal(t, bgpMsg, data[56:])
}

func TestIpTo4(t *testing.T) {
	tests := []struct {
		name string
		ip   []byte
		want [4]byte
	}{
		{"ipv4", []byte{10, 0, 0, 1}, [4]byte{10, 0, 0, 1}},
		{"ipv6-mapped", []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 192, 168, 1, 1}, [4]byte{192, 168, 1, 1}},
		{"nil", nil, [4]byte{}},
		{"short", []byte{1, 2}, [4]byte{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ipTo4(tt.ip)
			assert.Equal(t, tt.want, got)
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
	dir := t.TempDir()
	path := filepath.Join(dir, "test.mrt")
	f, err := os.Create(path)
	require.NoError(t, err)
	defer func() { _ = f.Close() }()

	bgpMsg := make([]byte, 19)
	for i := range 16 {
		bgpMsg[i] = 0xff
	}
	binary.BigEndian.PutUint16(bgpMsg[16:], 19)
	bgpMsg[18] = 4

	hdr := &mrt.BGP4MPHeader{
		PeerAS:  65000,
		LocalAS: 65001,
		AFI:     mrt.AFIIPv4,
		PeerIP:  []byte{10, 0, 0, 1},
		LocalIP: []byte{10, 0, 0, 2},
	}

	buf := make([]byte, 4096)
	off := mrt.CommonHeaderLen
	msgLen := mrt.WriteBGP4MPMessage(buf, off, hdr, true, bgpMsg)
	mrt.WriteCommonHeader(buf, 0, 1700000000, mrt.TypeBGP4MP, mrt.BGP4MPMessageAS4, uint32(msgLen))

	_, err = f.Write(buf[:off+msgLen])
	require.NoError(t, err)

	return path
}
