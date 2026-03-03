package shrink

import (
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/chaos/peer"
)

// failAfterReader wraps a reader and returns an error after n bytes.
type failAfterReader struct {
	r     io.Reader
	limit int
	read  int
}

func (f *failAfterReader) Read(p []byte) (int, error) {
	if f.read >= f.limit {
		return 0, errors.New("simulated I/O error")
	}
	// Read up to limit bytes.
	remaining := f.limit - f.read
	if len(p) > remaining {
		p = p[:remaining]
	}
	n, err := f.r.Read(p)
	f.read += n
	if f.read >= f.limit && err == nil {
		// Return the bytes read so far, next call will return error.
		return n, nil
	}
	return n, err
}

// TestParseLogValid verifies parsing a well-formed NDJSON event log.
//
// VALIDATES: Header and events are correctly parsed.
// PREVENTS: Broken shrink input from file-based --shrink mode.
func TestParseLogValid(t *testing.T) {
	input := `{"record-type":"header","version":1,"seed":42,"peers":2,"chaos-rate":0.1,"start-time":"2024-01-01T00:00:00Z"}
{"record-type":"event","seq":1,"time-offset-ms":0,"event-type":"established","peer-index":0}
{"record-type":"event","seq":2,"time-offset-ms":0,"event-type":"established","peer-index":1}
{"record-type":"event","seq":3,"time-offset-ms":100,"event-type":"route-sent","peer-index":0,"prefix":"10.0.0.0/24"}
{"record-type":"event","seq":4,"time-offset-ms":200,"event-type":"route-received","peer-index":1,"prefix":"10.0.0.0/24"}
`

	meta, events, err := ParseLog(strings.NewReader(input))
	require.NoError(t, err)

	assert.Equal(t, uint64(42), meta.Seed)
	assert.Equal(t, 2, meta.Peers)
	assert.InDelta(t, 0.1, meta.ChaosRate, 0.001)

	require.Len(t, events, 4)
	assert.Equal(t, peer.EventEstablished, events[0].Type)
	assert.Equal(t, 0, events[0].PeerIndex)
	assert.Equal(t, peer.EventEstablished, events[1].Type)
	assert.Equal(t, 1, events[1].PeerIndex)
	assert.Equal(t, peer.EventRouteSent, events[2].Type)
	assert.Equal(t, "10.0.0.0/24", events[2].Prefix.String())
	assert.Equal(t, peer.EventRouteReceived, events[3].Type)
}

// TestParseLogEmpty verifies error on empty input.
//
// VALIDATES: Empty input produces descriptive error.
// PREVENTS: Panic on empty reader.
func TestParseLogEmpty(t *testing.T) {
	_, _, err := ParseLog(strings.NewReader(""))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty event log")
}

// TestParseLogBadHeader verifies error when header is malformed.
//
// VALIDATES: Malformed header produces descriptive error.
// PREVENTS: Silent corruption from bad log files.
func TestParseLogBadHeader(t *testing.T) {
	_, _, err := ParseLog(strings.NewReader("not json"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parsing header")
}

// TestParseLogSkipsUnknownEvents verifies unknown event types are skipped.
//
// VALIDATES: Forward compatibility with future event types.
// PREVENTS: Parse failure on logs with newer event types.
func TestParseLogSkipsUnknownEvents(t *testing.T) {
	input := `{"record-type":"header","version":1,"seed":1,"peers":2,"chaos-rate":0,"start-time":"2024-01-01T00:00:00Z"}
{"record-type":"event","seq":1,"time-offset-ms":0,"event-type":"established","peer-index":0}
{"record-type":"event","seq":2,"time-offset-ms":0,"event-type":"future-event","peer-index":0}
{"record-type":"event","seq":3,"time-offset-ms":100,"event-type":"route-sent","peer-index":0,"prefix":"10.0.0.0/24"}
`

	_, events, err := ParseLog(strings.NewReader(input))
	require.NoError(t, err)
	assert.Len(t, events, 2, "unknown event type should be skipped")
}

// TestParseLogAllEventTypes verifies all known event types are parsed.
//
// VALIDATES: Every EventType in the map is recognized.
// PREVENTS: Missing event type mapping breaking shrink input.
func TestParseLogAllEventTypes(t *testing.T) {
	input := `{"record-type":"header","version":1,"seed":1,"peers":2,"chaos-rate":0,"start-time":"2024-01-01T00:00:00Z"}
{"record-type":"event","seq":1,"time-offset-ms":0,"event-type":"established","peer-index":0}
{"record-type":"event","seq":2,"time-offset-ms":10,"event-type":"route-sent","peer-index":0,"prefix":"10.0.0.0/24"}
{"record-type":"event","seq":3,"time-offset-ms":20,"event-type":"route-received","peer-index":1,"prefix":"10.0.0.0/24"}
{"record-type":"event","seq":4,"time-offset-ms":30,"event-type":"route-withdrawn","peer-index":1,"prefix":"10.0.0.0/24"}
{"record-type":"event","seq":5,"time-offset-ms":40,"event-type":"eor-sent","peer-index":0}
{"record-type":"event","seq":6,"time-offset-ms":50,"event-type":"disconnected","peer-index":0}
{"record-type":"event","seq":7,"time-offset-ms":60,"event-type":"error","peer-index":0}
{"record-type":"event","seq":8,"time-offset-ms":70,"event-type":"chaos-executed","peer-index":0,"chaos-action":"disconnect"}
{"record-type":"event","seq":9,"time-offset-ms":80,"event-type":"reconnecting","peer-index":0}
{"record-type":"event","seq":10,"time-offset-ms":90,"event-type":"withdrawal-sent","peer-index":0,"count":5}
`

	_, events, err := ParseLog(strings.NewReader(input))
	require.NoError(t, err)
	assert.Len(t, events, 10, "all 10 known event types should parse")
	assert.Equal(t, peer.EventWithdrawalSent, events[9].Type)
	assert.Equal(t, 5, events[9].Count)
	assert.Equal(t, "disconnect", events[7].ChaosAction)
}

// TestParseLogBadStartTime verifies error on malformed start-time in header.
//
// VALIDATES: Invalid start-time produces descriptive error instead of silent fallback.
// PREVENTS: Non-deterministic replay when start-time defaults to time.Now().
func TestParseLogBadStartTime(t *testing.T) {
	input := `{"record-type":"header","version":1,"seed":1,"peers":2,"chaos-rate":0,"start-time":"not-a-time"}
{"record-type":"event","seq":1,"time-offset-ms":0,"event-type":"established","peer-index":0}
`
	_, _, err := ParseLog(strings.NewReader(input))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parsing start-time")
}

// TestParseLogBadPrefix verifies error on malformed prefix in event.
//
// VALIDATES: Invalid CIDR prefix produces descriptive error with seq number.
// PREVENTS: Silent zero-value prefix from malformed event log data.
func TestParseLogBadPrefix(t *testing.T) {
	input := `{"record-type":"header","version":1,"seed":1,"peers":2,"chaos-rate":0,"start-time":"2024-01-01T00:00:00Z"}
{"record-type":"event","seq":1,"time-offset-ms":0,"event-type":"route-sent","peer-index":0,"prefix":"not-a-cidr"}
`
	_, _, err := ParseLog(strings.NewReader(input))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parsing prefix")
	assert.Contains(t, err.Error(), "seq 1")
}

// TestParseLogReadError verifies that I/O errors during event scanning are reported.
//
// VALIDATES: scanner.Err() is checked after the scan loop.
// PREVENTS: Silent truncation of event log on read error.
func TestParseLogReadError(t *testing.T) {
	input := `{"record-type":"header","version":1,"seed":1,"peers":2,"chaos-rate":0,"start-time":"2024-01-01T00:00:00Z"}
{"record-type":"event","seq":1,"time-offset-ms":0,"event-type":"established","peer-index":0}
{"record-type":"event","seq":2,"time-offset-ms":100,"event-type":"route-sent","peer-index":0,"prefix":"10.0.0.0/24"}
`
	// Allow the header line to be read, then fail mid-events.
	// Header line is ~100 bytes, so fail at 150 to cut off during events.
	r := &failAfterReader{r: strings.NewReader(input), limit: 150}

	_, _, err := ParseLog(r)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reading event log")
}
