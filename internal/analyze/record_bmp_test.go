package analyze

import (
	"encoding/binary"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/mrt"
)

func TestBmpRouteMonToMRT_IPv4(t *testing.T) {
	body := make([]byte, BMPPeerHdrLen+19)
	body[1] = 0x00
	body[22], body[23], body[24], body[25] = 10, 0, 0, 1
	binary.BigEndian.PutUint32(body[26:30], 65001)
	binary.BigEndian.PutUint32(body[34:38], 1700000000)
	for i := range 16 {
		body[BMPPeerHdrLen+i] = 0xff
	}
	binary.BigEndian.PutUint16(body[BMPPeerHdrLen+16:], 19)
	body[BMPPeerHdrLen+18] = 4

	sw := &syncWriter{w: mrt.NewWriter(filepath.Join(t.TempDir(), "out.mrt"))}
	defer func() { _ = sw.Close() }()

	n := bmpRouteMonToMRT(body, sw)
	assert.Equal(t, uint64(1), n)
}

func TestBmpRouteMonToMRT_IPv6(t *testing.T) {
	body := make([]byte, BMPPeerHdrLen+19)
	body[1] = 0x80
	body[10] = 0x20
	body[11] = 0x01
	body[25] = 0x01
	binary.BigEndian.PutUint32(body[26:30], 65002)
	binary.BigEndian.PutUint32(body[34:38], 1700000001)
	for i := range 16 {
		body[BMPPeerHdrLen+i] = 0xff
	}
	binary.BigEndian.PutUint16(body[BMPPeerHdrLen+16:], 19)
	body[BMPPeerHdrLen+18] = 4

	sw := &syncWriter{w: mrt.NewWriter(filepath.Join(t.TempDir(), "out.mrt"))}
	defer func() { _ = sw.Close() }()

	n := bmpRouteMonToMRT(body, sw)
	assert.Equal(t, uint64(1), n)
}

func TestBmpRouteMonToMRT_TooShort(t *testing.T) {
	sw := &syncWriter{w: mrt.NewWriter(filepath.Join(t.TempDir(), "out.mrt"))}
	defer func() { _ = sw.Close() }()

	n := bmpRouteMonToMRT(make([]byte, 10), sw)
	assert.Equal(t, uint64(0), n)
}

func TestBmpRouteMonToMRT_ShortBGP(t *testing.T) {
	body := make([]byte, BMPPeerHdrLen+5)
	binary.BigEndian.PutUint32(body[34:38], 1700000000)

	sw := &syncWriter{w: mrt.NewWriter(filepath.Join(t.TempDir(), "out.mrt"))}
	defer func() { _ = sw.Close() }()

	n := bmpRouteMonToMRT(body, sw)
	assert.Equal(t, uint64(0), n)
}

func TestBmpStateChangeToMRT(t *testing.T) {
	body := make([]byte, BMPPeerHdrLen)
	body[1] = 0x00
	body[22], body[23], body[24], body[25] = 192, 168, 1, 1
	binary.BigEndian.PutUint32(body[26:30], 65003)
	binary.BigEndian.PutUint32(body[34:38], 1700000002)

	path := filepath.Join(t.TempDir(), "state.mrt")
	sw := &syncWriter{w: mrt.NewWriter(path)}

	n1 := bmpPeerUpToMRT(body, sw)
	n2 := bmpPeerDownToMRT(body, sw)
	assert.Equal(t, uint64(1), n1)
	assert.Equal(t, uint64(1), n2)
	require.NoError(t, sw.Close())

	var count int
	err := mrt.ReadFile(path, &mrt.Handler{
		OnStateChange: func(_ mrt.Header, _ uint32, s *mrt.StateChangeRecord) error {
			count++
			if count == 1 {
				assert.Equal(t, mrt.FSMIdle, s.OldState)
				assert.Equal(t, mrt.FSMEstablished, s.NewState)
			} else {
				assert.Equal(t, mrt.FSMEstablished, s.OldState)
				assert.Equal(t, mrt.FSMIdle, s.NewState)
			}
			return nil
		},
	})
	require.NoError(t, err)
	assert.Equal(t, 2, count)
}
