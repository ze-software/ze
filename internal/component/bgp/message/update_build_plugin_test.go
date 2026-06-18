package message

import (
	"net/netip"
	"testing"
)

func TestUpdateBuilder_BuildPlugin_Basic(t *testing.T) {
	t.Parallel()
	ub := GetUpdateBuilder(65533, true, true, false)
	defer PutUpdateBuilder(ub)

	params := PluginParams{
		IsIPv6:  false,
		SAFI:    73,
		NLRI:    []byte{0x60, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x64, 0x0A, 0x00, 0x00, 0x01},
		NextHop: netip.MustParseAddr("192.0.2.1"),
	}

	update := ub.BuildPlugin(params)
	if update == nil {
		t.Fatal("BuildPlugin returned nil")
	}
	if len(update.PathAttributes) == 0 {
		t.Fatal("empty PathAttributes")
	}
}

func TestUpdateBuilder_BuildPlugin_WithRawAttrs(t *testing.T) {
	t.Parallel()
	ub := GetUpdateBuilder(65533, true, true, false)
	defer PutUpdateBuilder(ub)

	// Tunnel Encap attribute: flags(0xC0) + code(23) + len(4) + tunnel-type(15) + tunnel-len(0)
	tunnelEncap := []byte{0xC0, 0x17, 0x04, 0x00, 0x0F, 0x00, 0x00}

	params := PluginParams{
		IsIPv6:   false,
		SAFI:     73,
		NLRI:     []byte{0x60, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x64, 0x0A, 0x00, 0x00, 0x01},
		NextHop:  netip.MustParseAddr("192.0.2.1"),
		RawAttrs: [][]byte{tunnelEncap},
	}

	update := ub.BuildPlugin(params)
	if update == nil {
		t.Fatal("BuildPlugin returned nil")
	}
	if len(update.PathAttributes) == 0 {
		t.Fatal("empty PathAttributes")
	}
}

func TestUpdateBuilder_BuildPlugin_IPv6(t *testing.T) {
	t.Parallel()
	ub := GetUpdateBuilder(65533, true, true, false)
	defer PutUpdateBuilder(ub)

	// IPv6 SR-Policy NLRI: length(0xC0=192bits) + distinguisher(4) + color(4) + endpoint(16)
	nlri := make([]byte, 25)
	nlri[0] = 0xC0
	// rest is zeros (distinguisher=0, color=0, endpoint=::)

	params := PluginParams{
		IsIPv6:  true,
		SAFI:    73,
		NLRI:    nlri,
		NextHop: netip.MustParseAddr("2001:db8::2"),
	}

	update := ub.BuildPlugin(params)
	if update == nil {
		t.Fatal("BuildPlugin returned nil")
	}
}
