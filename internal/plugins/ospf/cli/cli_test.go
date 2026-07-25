// VALIDATES: the offline `ze ospf decode` CLI (run.go + decode.go) end-to-end: help/usage,
// the hex-input helpers (isHexString/stripWhitespace/toWire), and the three decode paths
// (OSPFv2 packet, IPv4 opaque LSA, OSPFv3 LSA) plus their error branches, driven through the
// real stdin/stdout/exit-code entry point with NO running engine.
// PREVENTS: a regression in the offline decoder that mis-renders typed fields, swallows a
// malformed-input error, or returns the wrong exit code.
package cli

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
	ospfv3packet "github.com/ze-software/ze/internal/plugins/ospf/v3/packet"
	ospfv3types "github.com/ze-software/ze/internal/plugins/ospf/v3/types"
)

// runCLI drives Run(args) with stdin fed from a temp file and stdout/stderr captured
// through pipes, restoring the process streams afterwards. It returns the exit code and
// the captured stdout/stderr. These tests mutate the global process streams, so they must
// not run in parallel.
func runCLI(t *testing.T, args []string, stdin []byte) (int, string, string) {
	t.Helper()
	inFile, err := os.CreateTemp(t.TempDir(), "stdin")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	defer func() { _ = inFile.Close() }()
	if _, err := inFile.Write(stdin); err != nil {
		t.Fatalf("write stdin: %v", err)
	}
	if _, err := inFile.Seek(0, io.SeekStart); err != nil {
		t.Fatalf("seek stdin: %v", err)
	}

	rOut, wOut, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stdout: %v", err)
	}
	rErr, wErr, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stderr: %v", err)
	}

	oldIn, oldOut, oldErr := os.Stdin, os.Stdout, os.Stderr
	os.Stdin, os.Stdout, os.Stderr = inFile, wOut, wErr
	code := Run(args)
	os.Stdin, os.Stdout, os.Stderr = oldIn, oldOut, oldErr

	if err := wOut.Close(); err != nil {
		t.Fatalf("close stdout writer: %v", err)
	}
	if err := wErr.Close(); err != nil {
		t.Fatalf("close stderr writer: %v", err)
	}
	outBytes, err := io.ReadAll(rOut)
	if err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	errBytes, err := io.ReadAll(rErr)
	if err != nil {
		t.Fatalf("read stderr: %v", err)
	}
	return code, string(outBytes), string(errBytes)
}

func mustRouterID(t *testing.T, s string) types.RouterID {
	t.Helper()
	id, err := types.ParseRouterID(s)
	if err != nil {
		t.Fatalf("ParseRouterID(%q): %v", s, err)
	}
	return id
}

func mustAreaID(t *testing.T, s string) types.AreaID {
	t.Helper()
	id, err := types.ParseAreaID(s)
	if err != nil {
		t.Fatalf("ParseAreaID(%q): %v", s, err)
	}
	return id
}

// helloWire builds a valid OSPFv2 Hello packet and returns its on-wire bytes.
func helloWire(t *testing.T) []byte {
	t.Helper()
	hello := packet.Hello{
		NetworkMask:   [4]byte{255, 255, 255, 0},
		HelloInterval: 10,
		Options:       types.OptionE | types.OptionO,
		Priority:      1,
		DeadInterval:  40,
		DR:            [4]byte{10, 0, 0, 1},
		BDR:           [4]byte{10, 0, 0, 2},
		Neighbors:     []types.RouterID{mustRouterID(t, "10.0.0.2"), mustRouterID(t, "10.0.0.3")},
	}
	p := packet.Packet{
		Header: packet.Header{
			Type:     packet.PacketTypeHello,
			RouterID: mustRouterID(t, "10.0.0.1"),
			AreaID:   mustAreaID(t, "0"),
			AuType:   packet.AuTypeNull,
		},
		Hello: &hello,
	}
	buf := make([]byte, p.EncodedLen())
	if n := (&p).WriteTo(buf, 0); n != len(buf) {
		t.Fatalf("Packet.WriteTo wrote %d, want %d", n, len(buf))
	}
	return buf
}

// opaqueWire builds a valid IPv4 opaque-area LSA (Opaque Type 250 / ID 1) carrying one
// generic TLV (type 1, value 01020304) and returns its on-wire bytes.
func opaqueWire(t *testing.T) []byte {
	t.Helper()
	body := []byte{0x00, 0x01, 0x00, 0x04, 0x01, 0x02, 0x03, 0x04}
	l := packet.LSA{
		Header: packet.LSAHeader{
			Age:               types.LSAge(10),
			Options:           types.OptionE | types.OptionO,
			Type:              types.LSTypeOpaqueArea,
			LinkStateID:       packet.OpaqueLinkStateID(250, 1),
			AdvertisingRouter: mustRouterID(t, "10.0.0.1"),
			Sequence:          types.InitialSequenceNumber,
		},
		Body: body,
	}
	buf := make([]byte, l.EncodedLen())
	if n := (&l).WriteTo(buf, 0); n != len(buf) {
		t.Fatalf("LSA.WriteTo wrote %d, want %d", n, len(buf))
	}
	return buf
}

// v3RouterWire builds a valid OSPFv3 Router-LSA (LS Type 0x2001) and returns its bytes.
func v3RouterWire(t *testing.T) []byte {
	t.Helper()
	rb := ospfv3packet.RouterLSA{}
	l := ospfv3packet.LSA{
		Header: ospfv3packet.LSAHeader{
			Age:      ospfv3types.LSAge(10),
			Type:     ospfv3types.LSTypeRouter,
			Sequence: ospfv3types.LSSequenceNumber(1),
		},
		Router: &rb,
	}
	buf := make([]byte, l.EncodedLen())
	if n := l.WriteTo(buf, 0); n != len(buf) {
		t.Fatalf("v3 LSA.WriteTo wrote %d, want %d", n, len(buf))
	}
	return buf
}

// v3Wire encodes a fully-formed OSPFv3 LSA (typed body + Header.Type) to its wire bytes.
func v3Wire(t *testing.T, l *ospfv3packet.LSA) []byte {
	t.Helper()
	buf := make([]byte, l.EncodedLen())
	if n := l.WriteTo(buf, 0); n != len(buf) {
		t.Fatalf("v3 LSA.WriteTo wrote %d, want %d", n, len(buf))
	}
	return buf
}

// VALIDATES: --v3 selects the correct typed-body decoder per LS Type (Network, Intra-Area-
// Prefix, Link, NSSA) and reports each type's scope-aware LS Type hex and flooding scope.
// PREVENTS: a v3 LS Type falling through to raw body-hex when a typed decoder exists.
func TestCmdDecodeV3TypedBodies(t *testing.T) {
	cases := []struct {
		name      string
		build     func() *ospfv3packet.LSA
		wantHex   string
		wantScope string
	}{
		{"network", func() *ospfv3packet.LSA {
			return &ospfv3packet.LSA{Header: ospfv3packet.LSAHeader{Type: ospfv3types.LSTypeNetwork}, Network: &ospfv3packet.NetworkLSA{}}
		}, "0x2002", "area"},
		{"intra-area-prefix", func() *ospfv3packet.LSA {
			return &ospfv3packet.LSA{Header: ospfv3packet.LSAHeader{Type: ospfv3types.LSTypeIntraAreaPrefix}, IntraAreaPfx: &ospfv3packet.IntraAreaPrefixLSA{}}
		}, "0x2009", "area"},
		{"link", func() *ospfv3packet.LSA {
			return &ospfv3packet.LSA{Header: ospfv3packet.LSAHeader{Type: ospfv3types.LSTypeLink}, Link: &ospfv3packet.LinkLSA{}}
		}, "0x0008", "link-local"},
		{"nssa-external", func() *ospfv3packet.LSA {
			return &ospfv3packet.LSA{Header: ospfv3packet.LSAHeader{Type: ospfv3types.LSTypeNSSA}, External: &ospfv3packet.ExternalLSA{}}
		}, "0x2007", "area"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			hexIn := hex.EncodeToString(v3Wire(t, c.build()))
			code, out, errOut := runCLI(t, []string{"--v3"}, []byte(hexIn))
			if code != 0 {
				t.Fatalf("exit = %d, stderr = %q", code, errOut)
			}
			var got struct {
				LSTypeHex string          `json:"ls-type-hex"`
				Scope     string          `json:"scope"`
				Decoded   json.RawMessage `json:"decoded"`
				BodyHex   string          `json:"body-hex"`
			}
			if err := json.Unmarshal([]byte(out), &got); err != nil {
				t.Fatalf("unmarshal %q: %v", out, err)
			}
			if got.LSTypeHex != c.wantHex || got.Scope != c.wantScope {
				t.Fatalf("ls-type/scope = %q/%q, want %q/%q", got.LSTypeHex, got.Scope, c.wantHex, c.wantScope)
			}
			if len(got.Decoded) == 0 || string(got.Decoded) == "null" {
				t.Fatalf("typed body missing for %s: %q", c.name, out)
			}
			if got.BodyHex != "" {
				t.Fatalf("body-hex should be empty for a typed body, got %q", got.BodyHex)
			}
		})
	}
}

// VALIDATES: isHexString classifies each byte as a hex digit; any non-hex byte fails.
// PREVENTS: toWire hex-decoding a string that is not pure hex (or rejecting valid hex).
func TestIsHexString(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"", true},
		{"00", true},
		{"0123456789", true},
		{"abcdef", true},
		{"ABCDEF", true},
		{"DeadBeef", true},
		{"0g", false},
		{"xyz", false},
		{"12 34", false}, // space is not a hex digit
		{"0x01", false},  // the 'x' is not a hex digit
		{"!!", false},
	}
	for _, c := range cases {
		if got := isHexString(c.in); got != c.want {
			t.Errorf("isHexString(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

// VALIDATES: stripWhitespace removes every Unicode space rune and keeps the rest in order.
// PREVENTS: pasted hex with spaces/newlines/tabs failing the even-length + hex check.
func TestStripWhitespace(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", ""},
		{"abcdef", "abcdef"},
		{"  ab cd\tef\n", "abcdef"},
		{"ab\r\ncd", "abcd"},
		{" \t\n\r ", ""},
		{"00 11\t22", "001122"},
	}
	for _, c := range cases {
		if got := stripWhitespace(c.in); got != c.want {
			t.Errorf("stripWhitespace(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// VALIDATES: toWire hex-decodes clean even-length hex (after whitespace stripping) and
// passes the raw bytes through unchanged for odd-length, non-hex, or empty input.
// PREVENTS: a non-hex payload being mangled, or spaced hex not being decoded.
func TestToWire(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []byte
	}{
		{"even-hex-decoded", "01020304", []byte{0x01, 0x02, 0x03, 0x04}},
		{"spaced-hex-decoded", "01 02\n03\t04", []byte{0x01, 0x02, 0x03, 0x04}},
		{"uppercase-hex-decoded", "DEADBEEF", []byte{0xDE, 0xAD, 0xBE, 0xEF}},
		{"odd-length-passthrough", "010", []byte("010")},
		{"non-hex-passthrough", "zzzz", []byte("zzzz")},
		{"empty-passthrough", "", []byte("")},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := toWire([]byte(c.in))
			if !bytes.Equal(got, c.want) {
				t.Fatalf("toWire(%q) = % x, want % x", c.in, got, c.want)
			}
		})
	}
}

// VALIDATES: v3OfflineScope maps the S2/S1 bits (RFC 5340 A.4.2.1) to the flooding scope.
// PREVENTS: an OSPFv3 decode mislabelling a link-local/area/AS/reserved LSA scope.
func TestV3OfflineScope(t *testing.T) {
	cases := []struct {
		lsType uint16
		want   string
	}{
		{uint16(ospfv3types.LSTypeLink), "link-local"}, // 0x0008
		{uint16(ospfv3types.LSTypeRouter), "area"},     // 0x2001
		{uint16(ospfv3types.LSTypeASExternal), "as"},   // 0x4005
		{0x6000, "reserved"},                           // S2S1 = 11
	}
	for _, c := range cases {
		if got := v3OfflineScope(c.lsType); got != c.want {
			t.Errorf("v3OfflineScope(0x%04x) = %q, want %q", c.lsType, got, c.want)
		}
	}
}

// VALIDATES: isHelpArg recognizes exactly -h, --help, and help.
// PREVENTS: a decode flag being mistaken for a help request (or vice-versa).
func TestIsHelpArg(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"-h", true},
		{"--help", true},
		{"help", true},
		{"--pretty", false},
		{"--opaque", false},
		{"", false},
		{"h", false},
	}
	for _, c := range cases {
		if got := isHelpArg(c.in); got != c.want {
			t.Errorf("isHelpArg(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

// VALIDATES: Run with a help arg prints usage to stderr and exits 0 without reading stdin.
// PREVENTS: help mode attempting a decode or exiting non-zero.
func TestRunHelp(t *testing.T) {
	for _, arg := range []string{"--help", "-h", "help"} {
		code, out, errOut := runCLI(t, []string{arg}, nil)
		if code != 0 {
			t.Fatalf("Run(%q) exit = %d, want 0", arg, code)
		}
		if out != "" {
			t.Fatalf("Run(%q) wrote to stdout: %q", arg, out)
		}
		if !strings.Contains(errOut, "usage: ze ospf decode") {
			t.Fatalf("Run(%q) stderr = %q, want usage line", arg, errOut)
		}
	}
}

// VALIDATES: the default path decodes a hex OSPFv2 Hello to JSON with its typed fields.
// PREVENTS: the offline packet decode dropping the packet type, router id, or Hello body.
func TestCmdDecodeV2Packet(t *testing.T) {
	hexIn := hex.EncodeToString(helloWire(t))
	code, out, errOut := runCLI(t, []string{}, []byte(hexIn))
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, errOut)
	}
	var got struct {
		Type     string `json:"type"`
		RouterID string `json:"router-id"`
		AreaID   string `json:"area-id"`
		Hello    *struct {
			HelloInterval uint16   `json:"hello-interval"`
			DeadInterval  uint32   `json:"dead-interval"`
			DR            string   `json:"dr"`
			Neighbors     []string `json:"neighbors"`
		} `json:"hello"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("unmarshal %q: %v", out, err)
	}
	if got.Type != "hello" {
		t.Fatalf("type = %q, want hello", got.Type)
	}
	if got.RouterID != "10.0.0.1" || got.AreaID != "0.0.0.0" {
		t.Fatalf("router/area = %q/%q, want 10.0.0.1/0.0.0.0", got.RouterID, got.AreaID)
	}
	if got.Hello == nil {
		t.Fatalf("hello body missing in %q", out)
	}
	if got.Hello.HelloInterval != 10 || got.Hello.DeadInterval != 40 {
		t.Fatalf("intervals = %d/%d, want 10/40", got.Hello.HelloInterval, got.Hello.DeadInterval)
	}
	if got.Hello.DR != "10.0.0.1" {
		t.Fatalf("dr = %q, want 10.0.0.1", got.Hello.DR)
	}
	if len(got.Hello.Neighbors) != 2 || got.Hello.Neighbors[0] != "10.0.0.2" || got.Hello.Neighbors[1] != "10.0.0.3" {
		t.Fatalf("neighbors = %v, want [10.0.0.2 10.0.0.3]", got.Hello.Neighbors)
	}
}

// VALIDATES: --pretty indents the JSON output (emitJSON SetIndent branch).
// PREVENTS: the --pretty flag being ignored (compact output).
func TestCmdDecodeV2Pretty(t *testing.T) {
	hexIn := hex.EncodeToString(helloWire(t))
	code, out, errOut := runCLI(t, []string{"--pretty"}, []byte(hexIn))
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, errOut)
	}
	if !strings.Contains(out, "\n  \"type\": \"hello\"") {
		t.Fatalf("pretty output not indented: %q", out)
	}
}

// VALIDATES: a truncated OSPFv2 packet returns exit 1 with a decode error on stderr.
// PREVENTS: a malformed packet decoding to a partial/garbage JSON with a success code.
func TestCmdDecodeV2Error(t *testing.T) {
	code, out, errOut := runCLI(t, []string{}, []byte("0201")) // 2 bytes: shorter than the 24-byte header
	if code != 1 {
		t.Fatalf("exit = %d, want 1 (stdout=%q)", code, out)
	}
	if !strings.Contains(errOut, "error: decode packet:") {
		t.Fatalf("stderr = %q, want decode-packet error", errOut)
	}
}

// VALIDATES: --opaque renders an IPv4 opaque LSA's Opaque Type/ID and generic TLVs (RFC 5250).
// PREVENTS: the opaque path hiding the Opaque Type/ID or the TLV breakdown.
func TestCmdDecodeOpaque(t *testing.T) {
	hexIn := hex.EncodeToString(opaqueWire(t))
	code, out, errOut := runCLI(t, []string{"--opaque"}, []byte(hexIn))
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, errOut)
	}
	var got struct {
		OpaqueType uint8  `json:"opaque-type"`
		OpaqueID   uint32 `json:"opaque-id"`
		Malformed  bool   `json:"malformed"`
		TLVs       []struct {
			Type     uint16 `json:"type"`
			Length   int    `json:"length"`
			ValueHex string `json:"value-hex"`
		} `json:"tlvs"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("unmarshal %q: %v", out, err)
	}
	if got.OpaqueType != 250 || got.OpaqueID != 1 {
		t.Fatalf("opaque type/id = %d/%d, want 250/1", got.OpaqueType, got.OpaqueID)
	}
	if got.Malformed {
		t.Fatalf("well-formed body flagged malformed: %q", out)
	}
	if len(got.TLVs) != 1 || got.TLVs[0].Type != 1 || got.TLVs[0].Length != 4 || got.TLVs[0].ValueHex != "01020304" {
		t.Fatalf("tlvs = %+v, want type1 len4 value 01020304", got.TLVs)
	}
}

// VALIDATES: --opaque on a buffer shorter than the 20-byte LSA header exits 1 with an error.
// PREVENTS: a truncated opaque LSA reported as a successful (empty) decode.
func TestCmdDecodeOpaqueError(t *testing.T) {
	code, _, errOut := runCLI(t, []string{"--opaque"}, []byte("0001")) // 2 bytes, short LSA header
	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	if !strings.Contains(errOut, "error: decode opaque LSA:") {
		t.Fatalf("stderr = %q, want decode-opaque error", errOut)
	}
}

// VALIDATES: renderOpaqueLSA flags a malformed TLV body and keeps the raw hex (RFC 5250 sec 5).
// PREVENTS: a truncated opaque TLV stream silently rendering as if fully parsed.
func TestRenderOpaqueLSAMalformed(t *testing.T) {
	// TLV header claims a 4-byte value but only 2 value bytes follow: DecodeOpaqueTLVs faults.
	body := []byte{0x00, 0x01, 0x00, 0x04, 0x01, 0x02}
	l := packet.LSA{
		Header: packet.LSAHeader{
			Type:        types.LSTypeOpaqueArea,
			LinkStateID: packet.OpaqueLinkStateID(250, 7),
			Length:      uint16(types.LSAHeaderLen + len(body)),
		},
		Body: body,
	}
	out := renderOpaqueLSA(l)
	if !out.Malformed {
		t.Fatalf("malformed body not flagged: %+v", out)
	}
	if out.BodyHex != hex.EncodeToString(body) {
		t.Fatalf("body-hex = %q, want %q", out.BodyHex, hex.EncodeToString(body))
	}
	if len(out.TLVs) != 0 {
		t.Fatalf("expected no fully-decoded TLVs, got %+v", out.TLVs)
	}
	if out.OpaqueID != 7 {
		t.Fatalf("opaque-id = %d, want 7", out.OpaqueID)
	}
}

// VALIDATES: --v3 renders an OSPFv3 LSA's scope-aware LS Type + typed body (RFC 5340).
// PREVENTS: the v3 path reporting a flat type number, hiding the scope, or dropping the body.
func TestCmdDecodeV3(t *testing.T) {
	hexIn := hex.EncodeToString(v3RouterWire(t))
	code, out, errOut := runCLI(t, []string{"--v3"}, []byte(hexIn))
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, errOut)
	}
	var got struct {
		LSTypeHex    string          `json:"ls-type-hex"`
		Scope        string          `json:"scope"`
		FunctionCode uint16          `json:"function-code"`
		UBit         bool            `json:"u-bit"`
		Length       uint16          `json:"length"`
		Decoded      json.RawMessage `json:"decoded"`
		BodyHex      string          `json:"body-hex"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("unmarshal %q: %v", out, err)
	}
	if got.LSTypeHex != "0x2001" {
		t.Fatalf("ls-type-hex = %q, want 0x2001", got.LSTypeHex)
	}
	if got.Scope != "area" || got.FunctionCode != 1 || got.UBit {
		t.Fatalf("scope/func/ubit = %q/%d/%v, want area/1/false", got.Scope, got.FunctionCode, got.UBit)
	}
	if len(got.Decoded) == 0 || string(got.Decoded) == "null" {
		t.Fatalf("decoded body missing: %q", out)
	}
	if got.BodyHex != "" {
		t.Fatalf("body-hex should be empty for a typed body, got %q", got.BodyHex)
	}
	if got.Length <= 20 {
		t.Fatalf("length = %d, want > 20 (header + body)", got.Length)
	}
}

// VALIDATES: --v3 on a buffer shorter than the 20-byte v3 LSA header exits 1 with an error.
// PREVENTS: a truncated OSPFv3 LSA reported as a successful decode.
func TestCmdDecodeV3Error(t *testing.T) {
	code, _, errOut := runCLI(t, []string{"--v3"}, []byte("2001")) // 2 bytes, short v3 LSA header
	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	if !strings.Contains(errOut, "error: decode OSPFv3 LSA:") {
		t.Fatalf("stderr = %q, want decode-v3 error", errOut)
	}
}

// VALIDATES: an OSPFv3 LSA whose type has no typed decoder falls back to the raw body hex,
// and v3OfflineScope still names the link-local scope (Grace-LSA, function code 11).
// PREVENTS: an unknown v3 LSA type panicking or emitting an empty/typed body.
func TestRenderV3LSAUnknownTypeBodyHex(t *testing.T) {
	body := []byte{0xDE, 0xAD, 0xBE, 0xEF}
	l := ospfv3packet.LSA{
		Header: ospfv3packet.LSAHeader{
			Type:   ospfv3types.LSTypeGrace, // 0x000B: not in the typed-body switch
			Length: uint16(20 + len(body)),
		},
		Body: body,
	}
	out := renderV3LSA(l)
	if out.LSTypeHex != "0x000b" {
		t.Fatalf("ls-type-hex = %q, want 0x000b", out.LSTypeHex)
	}
	if out.Scope != "link-local" {
		t.Fatalf("scope = %q, want link-local", out.Scope)
	}
	if out.FunctionCode != 11 {
		t.Fatalf("function-code = %d, want 11", out.FunctionCode)
	}
	if out.Decoded != nil {
		t.Fatalf("unknown type should not decode a typed body, got %v", out.Decoded)
	}
	if out.BodyHex != hex.EncodeToString(body) {
		t.Fatalf("body-hex = %q, want %q", out.BodyHex, hex.EncodeToString(body))
	}
}

// VALIDATES: a flag-parse failure (unknown flag) exits 1 without reading stdin.
// PREVENTS: an unrecognized flag being silently ignored and a decode proceeding.
func TestCmdDecodeBadFlag(t *testing.T) {
	code, _, _ := runCLI(t, []string{"--nope"}, nil)
	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
}

// VALIDATES: input larger than the stdin cap exits 1 with an over-limit error (readWire guard).
// PREVENTS: an unbounded read of an oversized offline payload.
func TestReadWireOverLimit(t *testing.T) {
	big := make([]byte, maxStdinBytes+1)
	for i := range big {
		big[i] = 'a' // valid hex char, but the size guard trips before any decode
	}
	code, _, errOut := runCLI(t, []string{}, big)
	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	if !strings.Contains(errOut, "exceeds") {
		t.Fatalf("stderr = %q, want over-limit error", errOut)
	}
}
