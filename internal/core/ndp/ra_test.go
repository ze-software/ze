package ndp

import (
	"encoding/hex"
	"net/netip"
	"testing"
)

// Golden strings below are derived by hand from the RFC field layouts, never
// captured from the encoder, so they state what the wire must carry rather than
// what the code happens to produce.

// VALIDATES: the 16-octet Router Advertisement header layout of RFC 4861
// Section 4.2: type 134, code 0, zero checksum, Cur Hop Limit, M/O flags with
// six zero Reserved bits, Router Lifetime, Reachable Time, Retrans Timer.
// PREVENTS: a field written at the wrong offset, in the wrong byte order, or a
// Reserved bit leaking a non-zero value onto the wire.
func TestBuildRAHeader(t *testing.T) {
	tests := []struct {
		name string
		cfg  RAConfig
		want string
	}{
		{
			name: "zero config is a bare header",
			cfg:  RAConfig{},
			want: "86000000" + "00" + "00" + "0000" + "00000000" + "00000000",
		},
		{
			name: "managed and other-config both set",
			cfg: RAConfig{
				CurHopLimit:    64,
				Managed:        true,
				OtherConfig:    true,
				RouterLifetime: 1800,
			},
			want: "86000000" + "40" + "c0" + "0708" + "00000000" + "00000000",
		},
		{
			name: "managed only",
			cfg:  RAConfig{CurHopLimit: 64, Managed: true, RouterLifetime: 600},
			want: "86000000" + "40" + "80" + "0258" + "00000000" + "00000000",
		},
		{
			name: "other-config only",
			cfg:  RAConfig{CurHopLimit: 64, OtherConfig: true, RouterLifetime: 600},
			want: "86000000" + "40" + "40" + "0258" + "00000000" + "00000000",
		},
		{
			// RFC 4861 Section 4.2: a Router Lifetime of 0 says this router is
			// not a default router. The rest of the RA still applies, so it is
			// a legitimate value to encode, not an error (spec AC-13).
			name: "router lifetime zero is not a default router",
			cfg:  RAConfig{CurHopLimit: 64, RouterLifetime: 0},
			want: "86000000" + "40" + "00" + "0000" + "00000000" + "00000000",
		},
		{
			name: "maximum router lifetime",
			cfg:  RAConfig{CurHopLimit: 255, RouterLifetime: 65535},
			want: "86000000" + "ff" + "00" + "ffff" + "00000000" + "00000000",
		},
		{
			// RFC 4861 Section 6.2.1: AdvReachableTime is capped at 3600000 ms.
			name: "reachable time and retransmit timer at their bounds",
			cfg: RAConfig{
				CurHopLimit:   64,
				ReachableTime: 3600000,
				RetransTimer:  4294967295,
			},
			want: "86000000" + "40" + "00" + "0000" + "0036ee80" + "ffffffff",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertEncodes(t, tt.cfg, tt.want)
		})
	}
}

// VALIDATES: the Prefix Information option of RFC 4861 Section 4.6.2: type 3,
// length 4, prefix length, L and A flags with six zero Reserved1 bits, valid
// and preferred lifetimes, zero Reserved2, and a prefix whose bits past the
// prefix length are zero.
// PREVENTS: advertising host bits the RFC requires the sender to zero, and
// swapping the valid and preferred lifetime fields.
func TestBuildRAPrefixOption(t *testing.T) {
	const header = "86000000" + "40" + "00" + "0708" + "00000000" + "00000000"

	tests := []struct {
		name string
		p    PrefixInfo
		want string
	}{
		{
			name: "on-link and autonomous with radvd default lifetimes",
			p: PrefixInfo{
				Prefix:            netip.MustParsePrefix("2001:db8:1::/64"),
				OnLink:            true,
				Autonomous:        true,
				ValidLifetime:     2592000,
				PreferredLifetime: 604800,
			},
			want: "03" + "04" + "40" + "c0" + "00278d00" + "00093a80" + "00000000" +
				"20010db8000100000000000000000000",
		},
		{
			name: "on-link only clears the A flag",
			p: PrefixInfo{
				Prefix:        netip.MustParsePrefix("2001:db8:1::/64"),
				OnLink:        true,
				ValidLifetime: 2592000,
			},
			want: "03" + "04" + "40" + "80" + "00278d00" + "00000000" + "00000000" +
				"20010db8000100000000000000000000",
		},
		{
			name: "autonomous only clears the L flag",
			p: PrefixInfo{
				Prefix:            netip.MustParsePrefix("2001:db8:1::/64"),
				Autonomous:        true,
				ValidLifetime:     2592000,
				PreferredLifetime: 2592000,
			},
			want: "03" + "04" + "40" + "40" + "00278d00" + "00278d00" + "00000000" +
				"20010db8000100000000000000000000",
		},
		{
			// RFC 4861 Section 4.6.2: bits after the prefix length MUST be
			// initialized to zero by the sender.
			name: "host bits past the prefix length are zeroed",
			p: PrefixInfo{
				Prefix:            netip.MustParsePrefix("2001:db8:1::dead:beef/64"),
				OnLink:            true,
				Autonomous:        true,
				ValidLifetime:     2592000,
				PreferredLifetime: 604800,
			},
			want: "03" + "04" + "40" + "c0" + "00278d00" + "00093a80" + "00000000" +
				"20010db8000100000000000000000000",
		},
		{
			name: "prefix length 0 is the low boundary",
			p: PrefixInfo{
				Prefix:        netip.MustParsePrefix("::/0"),
				ValidLifetime: 1,
			},
			want: "03" + "04" + "00" + "00" + "00000001" + "00000000" + "00000000" +
				"00000000000000000000000000000000",
		},
		{
			name: "prefix length 128 is the high boundary",
			p: PrefixInfo{
				Prefix:        netip.MustParsePrefix("2001:db8::1/128"),
				OnLink:        true,
				ValidLifetime: LifetimeInfinity,
			},
			want: "03" + "04" + "80" + "80" + "ffffffff" + "00000000" + "00000000" +
				"20010db8000000000000000000000001",
		},
		{
			// RFC 4861 Section 4.6.2: all one bits represents infinity.
			name: "infinite valid and preferred lifetimes",
			p: PrefixInfo{
				Prefix:            netip.MustParsePrefix("2001:db8:2::/48"),
				OnLink:            true,
				Autonomous:        true,
				ValidLifetime:     LifetimeInfinity,
				PreferredLifetime: LifetimeInfinity,
			},
			want: "03" + "04" + "30" + "c0" + "ffffffff" + "ffffffff" + "00000000" +
				"20010db8000200000000000000000000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := RAConfig{CurHopLimit: 64, RouterLifetime: 1800, Prefixes: []PrefixInfo{tt.p}}
			assertEncodes(t, cfg, header+tt.want)
		})
	}
}

// VALIDATES: several Prefix Information options are emitted back to back, in
// configuration order (RFC 4861 Section 4.6.2 allows any number).
// PREVENTS: an encoder that only ever writes the first prefix.
func TestBuildRAMultiplePrefixes(t *testing.T) {
	cfg := RAConfig{
		CurHopLimit:    64,
		RouterLifetime: 1800,
		Prefixes: []PrefixInfo{
			{Prefix: netip.MustParsePrefix("2001:db8:1::/64"), OnLink: true, ValidLifetime: 1},
			{Prefix: netip.MustParsePrefix("2001:db8:2::/64"), Autonomous: true, ValidLifetime: 2},
		},
	}
	want := "86000000" + "40" + "00" + "0708" + "00000000" + "00000000" +
		"03" + "04" + "40" + "80" + "00000001" + "00000000" + "00000000" +
		"20010db8000100000000000000000000" +
		"03" + "04" + "40" + "40" + "00000002" + "00000000" + "00000000" +
		"20010db8000200000000000000000000"
	assertEncodes(t, cfg, want)
}

// VALIDATES: the Source Link-layer Address option of RFC 4861 Section 4.6.1:
// type 1, length 1 in 8-octet units, six address octets.
// PREVENTS: emitting an option whose length field disagrees with the bytes
// written, which makes a receiver mis-parse every option that follows.
func TestBuildRASourceLinkLayer(t *testing.T) {
	const header = "86000000" + "40" + "00" + "0708" + "00000000" + "00000000"

	t.Run("six-octet IEEE 802 address", func(t *testing.T) {
		cfg := RAConfig{
			CurHopLimit:            64,
			RouterLifetime:         1800,
			SourceLinkLayerAddress: []byte{0x02, 0x11, 0x22, 0x33, 0x44, 0x55},
		}
		assertEncodes(t, cfg, header+"01"+"01"+"021122334455")
	})

	t.Run("option precedes the prefix options", func(t *testing.T) {
		cfg := RAConfig{
			CurHopLimit:            64,
			RouterLifetime:         1800,
			SourceLinkLayerAddress: []byte{0x02, 0x11, 0x22, 0x33, 0x44, 0x55},
			Prefixes: []PrefixInfo{
				{Prefix: netip.MustParsePrefix("2001:db8:1::/64"), OnLink: true, ValidLifetime: 1},
			},
		}
		want := header + "01" + "01" + "021122334455" +
			"03" + "04" + "40" + "80" + "00000001" + "00000000" + "00000000" +
			"20010db8000100000000000000000000"
		assertEncodes(t, cfg, want)
	})

	t.Run("absent address emits no option", func(t *testing.T) {
		cfg := RAConfig{CurHopLimit: 64, RouterLifetime: 1800}
		assertEncodes(t, cfg, header)
	})

	t.Run("address that does not fit one 8-octet unit is omitted", func(t *testing.T) {
		cfg := RAConfig{
			CurHopLimit:            64,
			RouterLifetime:         1800,
			SourceLinkLayerAddress: []byte{1, 2, 3, 4, 5, 6, 7, 8},
		}
		assertEncodes(t, cfg, header)
	})
}

// VALIDATES: the RDNSS option of RFC 8106 Section 5.1: type 25, length
// 1+2*addresses in 8-octet units, zero Reserved, shared lifetime, then the
// addresses.
// PREVENTS: a length field that disagrees with the address count, which makes
// the receiver read the wrong number of resolvers.
func TestBuildRARDNSS(t *testing.T) {
	const header = "86000000" + "40" + "00" + "0708" + "00000000" + "00000000"
	dns1 := netip.MustParseAddr("2001:4860:4860::8888")
	dns2 := netip.MustParseAddr("2001:4860:4860::8844")

	tests := []struct {
		name     string
		servers  []netip.Addr
		lifetime uint32
		want     string
	}{
		{
			name:     "one resolver has length 3",
			servers:  []netip.Addr{dns1},
			lifetime: 3600,
			want: "19" + "03" + "0000" + "00000e10" +
				"20014860486000000000000000008888",
		},
		{
			name:     "each extra resolver adds 2 to the length",
			servers:  []netip.Addr{dns1, dns2},
			lifetime: 1800,
			want: "19" + "05" + "0000" + "00000708" +
				"20014860486000000000000000008888" +
				"20014860486000000000000000008844",
		},
		{
			// RFC 8106 Section 5.1: a lifetime of zero means the addresses
			// MUST no longer be used. It is a meaningful value to encode, not
			// an error (spec AC-14).
			name:     "lifetime zero retires the resolvers",
			servers:  []netip.Addr{dns1},
			lifetime: 0,
			want: "19" + "03" + "0000" + "00000000" +
				"20014860486000000000000000008888",
		},
		{
			name:     "infinite lifetime",
			servers:  []netip.Addr{dns1},
			lifetime: LifetimeInfinity,
			want: "19" + "03" + "0000" + "ffffffff" +
				"20014860486000000000000000008888",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := RAConfig{
				CurHopLimit:    64,
				RouterLifetime: 1800,
				RDNSS:          tt.servers,
				RDNSSLifetime:  tt.lifetime,
			}
			assertEncodes(t, cfg, header+tt.want)
		})
	}

	t.Run("no resolvers emits no option", func(t *testing.T) {
		assertEncodes(t, RAConfig{CurHopLimit: 64, RouterLifetime: 1800}, header)
	})
}

// VALIDATES: BuildRA writes at the offset it is given and reports only the
// octets it wrote.
// PREVENTS: an encoder that ignores off and overwrites data already in the
// caller's buffer.
func TestBuildRAWritesAtOffset(t *testing.T) {
	cfg := RAConfig{CurHopLimit: 64, RouterLifetime: 1800}
	buf := make([]byte, 32)
	for i := range buf {
		buf[i] = 0xaa
	}

	n := BuildRA(buf, 4, cfg)
	if n != 16 {
		t.Fatalf("BuildRA wrote %d octets, want 16", n)
	}
	if got := hex.EncodeToString(buf[:4]); got != "aaaaaaaa" {
		t.Errorf("octets before off were overwritten: %s", got)
	}
	if got := hex.EncodeToString(buf[4:20]); got != "8600000040000708"+"00000000"+"00000000" {
		t.Errorf("RA at offset 4 = %s", got)
	}
	if got := hex.EncodeToString(buf[20:]); got != "aaaaaaaaaaaaaaaaaaaaaaaa" {
		t.Errorf("octets after the RA were overwritten: %s", got)
	}
}

// VALIDATES: BuildRA refuses a buffer that cannot hold the whole message,
// returns 0, and leaves the buffer untouched.
// PREVENTS: a truncated RA reaching the wire, and a partial write that a caller
// reading only the length would mistake for a valid short message.
func TestBuildRAShortBufferWritesNothing(t *testing.T) {
	cfg := RAConfig{
		CurHopLimit:            64,
		RouterLifetime:         1800,
		SourceLinkLayerAddress: []byte{0x02, 0x11, 0x22, 0x33, 0x44, 0x55},
		Prefixes: []PrefixInfo{
			{Prefix: netip.MustParsePrefix("2001:db8:1::/64"), OnLink: true, ValidLifetime: 1},
		},
	}
	full := RALen(cfg)
	if full != 16+8+32 {
		t.Fatalf("RALen = %d, want %d", full, 16+8+32)
	}

	for _, size := range []int{0, 1, 15, 16, full - 1} {
		buf := make([]byte, size)
		if n := BuildRA(buf, 0, cfg); n != 0 {
			t.Errorf("buffer of %d octets: BuildRA = %d, want 0", size, n)
		}
		for i, b := range buf {
			if b != 0 {
				t.Errorf("buffer of %d octets: octet %d written as 0x%02x, want untouched", size, i, b)
			}
		}
	}

	buf := make([]byte, full)
	if n := BuildRA(buf, 0, cfg); n != full {
		t.Errorf("exactly-sized buffer: BuildRA = %d, want %d", n, full)
	}

	if n := BuildRA(make([]byte, full), -1, cfg); n != 0 {
		t.Errorf("negative offset: BuildRA = %d, want 0", n)
	}
}

// VALIDATES: RALen predicts exactly what BuildRA writes for every option
// combination, which is what lets a caller size its buffer without allocating
// a worst-case one.
// PREVENTS: RALen and BuildRA drifting apart, which turns every send into
// either a truncation or a silent over-allocation.
func TestRALenMatchesBuildRA(t *testing.T) {
	dns := netip.MustParseAddr("2001:4860:4860::8888")
	prefix := PrefixInfo{Prefix: netip.MustParsePrefix("2001:db8:1::/64"), OnLink: true}

	tests := []struct {
		name string
		cfg  RAConfig
	}{
		{"bare header", RAConfig{}},
		{"link-layer only", RAConfig{SourceLinkLayerAddress: []byte{1, 2, 3, 4, 5, 6}}},
		{"one prefix", RAConfig{Prefixes: []PrefixInfo{prefix}}},
		{"three prefixes", RAConfig{Prefixes: []PrefixInfo{prefix, prefix, prefix}}},
		{"rdnss only", RAConfig{RDNSS: []netip.Addr{dns}}},
		{"eight resolvers", RAConfig{RDNSS: []netip.Addr{dns, dns, dns, dns, dns, dns, dns, dns}}},
		{"every option", RAConfig{
			SourceLinkLayerAddress: []byte{1, 2, 3, 4, 5, 6},
			Prefixes:               []PrefixInfo{prefix, prefix},
			RDNSS:                  []netip.Addr{dns, dns},
		}},
		{"oversized link-layer address contributes nothing", RAConfig{
			SourceLinkLayerAddress: []byte{1, 2, 3, 4, 5, 6, 7, 8},
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want := RALen(tt.cfg)
			buf := make([]byte, want+64)
			if got := BuildRA(buf, 0, tt.cfg); got != want {
				t.Errorf("BuildRA wrote %d octets, RALen said %d", got, want)
			}
			for i, b := range buf[want:] {
				if b != 0 {
					t.Fatalf("octet %d past RALen was written as 0x%02x", want+i, b)
				}
			}
		})
	}
}

// VALIDATES: encoding an RA allocates nothing, as the send loop builds one per
// interval into a reused buffer.
// PREVENTS: a per-send allocation creeping into the hot path.
func TestBuildRADoesNotAllocate(t *testing.T) {
	cfg := RAConfig{
		CurHopLimit:            64,
		Managed:                true,
		RouterLifetime:         1800,
		SourceLinkLayerAddress: []byte{0x02, 0x11, 0x22, 0x33, 0x44, 0x55},
		Prefixes: []PrefixInfo{
			{Prefix: netip.MustParsePrefix("2001:db8:1::/64"), OnLink: true, Autonomous: true},
		},
		RDNSS: []netip.Addr{netip.MustParseAddr("2001:4860:4860::8888")},
	}
	buf := make([]byte, 256)

	if allocs := testing.AllocsPerRun(100, func() { BuildRA(buf, 0, cfg) }); allocs != 0 {
		t.Errorf("BuildRA allocated %v times per run, want 0", allocs)
	}
}

func assertEncodes(t *testing.T, cfg RAConfig, want string) {
	t.Helper()
	buf := make([]byte, 512)
	n := BuildRA(buf, 0, cfg)
	if got := hex.EncodeToString(buf[:n]); got != want {
		t.Errorf("BuildRA\n got %s\nwant %s", got, want)
	}
	if wantLen := len(want) / 2; n != wantLen {
		t.Errorf("BuildRA wrote %d octets, want %d", n, wantLen)
	}
	if got := RALen(cfg); got != n {
		t.Errorf("RALen = %d, BuildRA wrote %d", got, n)
	}
}
