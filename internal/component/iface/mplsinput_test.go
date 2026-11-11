package iface

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ReplaceMPLSInput accepts every set; the set the apply builds is judged by
// TestDesiredMPLSInput.
func (b *fakeBackend) ReplaceMPLSInput(_ []string) error { return nil }

func mplsUnit(enable bool) unitEntry {
	return unitEntry{Label: "0", MPLSEnable: &enable}
}

// VALIDATES: AC-27 -- desiredMPLSInput names every device whose enabled unit
// sets `mpls { enable true; }`, once each: enable false, no leaf, a disabled
// unit and an unbound selector name none, and a VLAN unit names its VLAN
// device. METHOD: one config holding each case, compared with the exact set.
func TestDesiredMPLSInput(t *testing.T) {
	disabledUnit := mplsUnit(true)
	disabledUnit.Disable = true
	cfg := &ifaceConfig{
		Dummy: []ifaceEntry{
			{Name: "dum0", Units: []unitEntry{mplsUnit(true)}},
			{Name: "dum1", Units: []unitEntry{mplsUnit(false)}},
			{Name: "dum2", Units: []unitEntry{{Label: "0"}}},
			{Name: "dum3", Units: []unitEntry{disabledUnit}},
		},
		Ethernet: []ifaceEntry{
			{Name: "wan", Units: []unitEntry{func() unitEntry {
				u := mplsUnit(true)
				u.VLANID = 100
				return u
			}()}},
			{Name: "absent", Units: []unitEntry{mplsUnit(true)}},
		},
		Loopback: &loopbackEntry{Units: []unitEntry{mplsUnit(true), mplsUnit(true)}},
	}
	devices := map[string]string{"wan": "xe7", "absent": ""}

	assert.ElementsMatch(t, []string{"xe7.100", "dum0", "lo"}, desiredMPLSInput(cfg, devices))
	assert.Empty(t, desiredMPLSInput(nil, nil), "a nil config enables no interface")
}
