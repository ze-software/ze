package analyze

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ze-software/ze/internal/mrt"
)

func TestFormatASPath_Sequence(t *testing.T) {
	segs := []mrt.ASPathSegment{
		{Type: 2, ASNs: []uint32{174, 1916, 52888}},
	}
	assert.Equal(t, "174 1916 52888", mrt.FormatASPath(segs))
}

func TestFormatASPath_Set(t *testing.T) {
	segs := []mrt.ASPathSegment{
		{Type: 2, ASNs: []uint32{174}},
		{Type: 1, ASNs: []uint32{100, 200}},
		{Type: 2, ASNs: []uint32{52888}},
	}
	assert.Equal(t, "174 {100,200} 52888", mrt.FormatASPath(segs))
}

func TestFormatASPath_Empty(t *testing.T) {
	assert.Equal(t, "", mrt.FormatASPath(nil))
}

func TestMatchASPath_Matches(t *testing.T) {
	attrs := []mrt.PathAttribute{
		{Code: 2, Flags: 0x40, Value: []byte{2, 3, 0, 0, 0, 174, 0, 0, 7, 124, 0, 0, 206, 88}},
	}
	re := regexp.MustCompile(`174`)
	assert.True(t, matchASPath(attrs, true, re))
}

func TestMatchASPath_NoMatch(t *testing.T) {
	attrs := []mrt.PathAttribute{
		{Code: 2, Flags: 0x40, Value: []byte{2, 2, 0, 0, 0, 174, 0, 0, 7, 124}},
	}
	re := regexp.MustCompile(`^13335`)
	assert.False(t, matchASPath(attrs, true, re))
}

func TestMatchASPath_NoASPathAttr(t *testing.T) {
	attrs := []mrt.PathAttribute{
		{Code: 1, Flags: 0x40, Value: []byte{0}},
	}
	re := regexp.MustCompile(`.*`)
	assert.False(t, matchASPath(attrs, true, re))
}

func TestMatchCommunityRegex_Standard(t *testing.T) {
	// Community 13335:100
	comm := []byte{0x34, 0x17, 0x00, 0x64}
	attrs := []mrt.PathAttribute{
		{Code: 8, Flags: 0xC0, Value: comm},
	}
	re := regexp.MustCompile(`13335:`)
	assert.True(t, mrt.MatchCommunityRegex(attrs, re.MatchString))
}

func TestMatchCommunityRegex_Large(t *testing.T) {
	// Large community 13335:100:200
	large := make([]byte, 12)
	large[0], large[1], large[2], large[3] = 0, 0, 0x34, 0x17 // 13335
	large[4], large[5], large[6], large[7] = 0, 0, 0, 100     // 100
	large[8], large[9], large[10], large[11] = 0, 0, 0, 200   // 200
	attrs := []mrt.PathAttribute{
		{Code: 32, Flags: 0xC0, Value: large},
	}
	re := regexp.MustCompile(`13335:100:200`)
	assert.True(t, mrt.MatchCommunityRegex(attrs, re.MatchString))
}

func TestMatchCommunityRegex_NoMatch(t *testing.T) {
	comm := []byte{0x34, 0x17, 0x00, 0x64} // 13335:100
	attrs := []mrt.PathAttribute{
		{Code: 8, Flags: 0xC0, Value: comm},
	}
	re := regexp.MustCompile(`^9999:`)
	assert.False(t, mrt.MatchCommunityRegex(attrs, re.MatchString))
}

func TestMatchCommunityRegex_NoCommunityAttr(t *testing.T) {
	attrs := []mrt.PathAttribute{
		{Code: 1, Flags: 0x40, Value: []byte{0}},
	}
	re := regexp.MustCompile(`.*`)
	assert.False(t, mrt.MatchCommunityRegex(attrs, re.MatchString))
}

func TestMatchMessageContent_NonUpdate(t *testing.T) {
	// KEEPALIVE message (type 4, no body)
	bgpMsg := make([]byte, 19)
	for i := range 16 {
		bgpMsg[i] = 0xff
	}
	bgpMsg[16] = 0
	bgpMsg[17] = 19
	bgpMsg[18] = 4

	m := &mrt.MessageRecord{BGPMessage: bgpMsg}
	opts := &filterOpts{asPathRe: regexp.MustCompile(`.*`)}
	assert.False(t, matchMessageContent(m, true, opts))
}
