// VALIDATES: RFC 5880 Sections 6.7.2, 6.7.3 and 6.7.4 hexadecimal key
// management through the interface the operator uses: a `secret-hex` leaf in
// the config text, parsed against the bfd YANG module, handed to the plugin
// as its config section, and resolved into the session request a client's
// session is built from.
// PREVENTS: a management interface that can only carry the bytes of an ASCII
// string, so a binary password or key (a zero byte, a byte above 0x7F) cannot
// be configured, and one that accepts a malformed or oversized hex form
// instead of refusing it.
package bfd

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/bfd/api"
	bfdyang "github.com/ze-software/ze/internal/component/bfd/yang"
	"github.com/ze-software/ze/internal/component/config"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// rfc5880HexKeyConfigText renders a bfd config holding one profile whose auth
// block names authType and carries the key lines verbatim, so a test can write
// `secret-hex`, `secret`, or both.
func rfc5880HexKeyConfigText(authType string, keyLines ...string) string {
	var b strings.Builder
	b.WriteString("bfd {\n    profile keyed {\n        auth {\n")
	b.WriteString("            type " + authType + ";\n")
	b.WriteString("            key-id 9;\n")
	for _, line := range keyLines {
		b.WriteString("            " + line + ";\n")
	}
	b.WriteString("        }\n    }\n}\n")
	return b.String()
}

// rfc5880HexKeyResolve drives the whole management path for config text and
// returns the session secret, or the first error any stage reported: the YANG
// parse, the plugin's section load, or the profile resolution.
func rfc5880HexKeyResolve(text string) ([]byte, error) {
	tree, err := config.ParseTreeWithYANG(text, map[string]string{
		configRoot: bfdyang.ZeBFDConfYANG,
	})
	if err != nil {
		return nil, err
	}
	data, err := json.Marshal(map[string]any{configRoot: tree.ToPluginMap()[configRoot]})
	if err != nil {
		return nil, err
	}
	cfg, err := parseSections([]sdk.ConfigSection{{Root: configRoot, Data: string(data)}})
	if err != nil {
		return nil, err
	}
	req, err := resolveProfile(api.SessionRequest{Profile: "keyed", Mode: api.SingleHop}, cfg.profiles)
	if err != nil {
		return nil, err
	}
	if req.Auth == nil {
		return nil, errNoAuthForTest
	}
	return req.Auth.Secret, nil
}

// errNoAuthForTest marks a resolved session request that carries no auth
// settings, which every config in this file names.
var errNoAuthForTest = errors.New("the session request carries no auth settings")

// rfc5880BinaryKey returns a key of length octets that starts with a zero byte,
// ends with 0xFF, and holds a high byte in between, so a path that stops at a
// NUL, sign-extends, or re-encodes bytes above 0x7F as UTF-8 changes it.
func rfc5880BinaryKey(length int) []byte {
	key := make([]byte, length)
	for i := range key {
		key[i] = byte(i * 37)
	}
	key[0] = 0x00
	key[length/2] = 0x80
	key[length-1] = 0xFF
	return key
}

// RFC requirement: RFC5880-6.7.2-2 positive -- "the management interface by
// which the password is configured MUST accept ASCII strings, and SHOULD also
// allow for the configuration of any arbitrary binary string in hexadecimal
// form." A simple-password written as `secret-hex` in the operator's config
// text reaches the session request's Secret as the decoded binary string,
// holding a zero byte, a 0x80 byte and a 0xFF byte, at 1 and 16 bytes.
// RFC requirement: RFC5880-6.7.3-14 positive -- the same `secret-hex` entry
// carries a binary key for keyed-md5 and meticulous-keyed-md5, decoded byte
// for byte into the session request, at 1 and 16 bytes.
// RFC requirement: RFC5880-6.7.4-8 positive -- the same `secret-hex` entry
// carries a binary key for keyed-sha1 and meticulous-keyed-sha1, decoded byte
// for byte into the session request, at 1 and 20 bytes.
func TestRFC5880KeyManagementConfigEntryAcceptsHexBinaryStrings(t *testing.T) {
	for _, kt := range rfc5880KeyTypes {
		t.Run(kt.name, func(t *testing.T) {
			keys := [][]byte{{0x00}, {0xFF}, rfc5880BinaryKey(kt.keyMax)}
			for _, key := range keys {
				for _, written := range []string{hex.EncodeToString(key), strings.ToUpper(hex.EncodeToString(key))} {
					got, err := rfc5880HexKeyResolve(rfc5880HexKeyConfigText(kt.name, `secret-hex "`+written+`"`))
					if err != nil {
						t.Fatalf("secret-hex %s: refused: %v", written, err)
					}
					if !bytes.Equal(got, key) {
						t.Fatalf("secret-hex %s: session secret = % x, want the decoded binary string % x", written, got, key)
					}
				}
			}
		})
	}
}

// RFC requirement: RFC5880-6.7.2-2 negative -- the input is forced toward the
// violation: every byte value 0x00 to 0xFF, the ones an ASCII string entry
// cannot carry included, is written in hexadecimal as a simple-password in
// chunks of the 16-byte limit. A management interface that dropped, trimmed or
// re-encoded any byte fails here; every chunk reaches the session request byte
// for byte.
// RFC requirement: RFC5880-6.7.3-14 negative -- the same sweep for keyed-md5
// and meticulous-keyed-md5 keys.
// RFC requirement: RFC5880-6.7.4-8 negative -- the same sweep for keyed-sha1
// and meticulous-keyed-sha1 keys, in chunks of their 20-byte limit.
func TestRFC5880KeyManagementConfigEntryKeepsEveryBinaryByte(t *testing.T) {
	all := make([]byte, 256)
	for i := range all {
		all[i] = byte(i)
	}
	for _, kt := range rfc5880KeyTypes {
		t.Run(kt.name, func(t *testing.T) {
			for start := 0; start < len(all); start += kt.keyMax {
				key := all[start:min(start+kt.keyMax, len(all))]
				got, err := rfc5880HexKeyResolve(rfc5880HexKeyConfigText(kt.name, `secret-hex "`+hex.EncodeToString(key)+`"`))
				if err != nil {
					t.Fatalf("secret-hex % x: refused: %v", key, err)
				}
				if !bytes.Equal(got, key) {
					t.Errorf("session secret = % x, want the configured binary string % x", got, key)
				}
			}
		})
	}
}

// TestRFC5880KeyManagementHexRefusesMalformed proves the hex entry refuses
// what it cannot decode into the key the operator meant, and what the Auth Len
// field cannot describe, rather than truncating or guessing. Method: each
// config is driven through the whole management path and must be refused, and
// the refusal must name the problem.
func TestRFC5880KeyManagementHexRefusesMalformed(t *testing.T) {
	cases := []struct {
		name     string
		authType string
		lines    []string
		want     string
	}{
		{"odd digit count", "keyed-sha1", []string{`secret-hex "abc"`}, "secret-hex"},
		{"not a hex digit", "keyed-sha1", []string{`secret-hex "zz"`}, "secret-hex"},
		{"0x prefix", "keyed-sha1", []string{`secret-hex "0x41"`}, "secret-hex"},
		{"empty", "keyed-sha1", []string{`secret-hex ""`}, "secret"},
		{"simple-password over 16 bytes", "simple-password", []string{`secret-hex "` + strings.Repeat("ab", 17) + `"`}, "6.7.2"},
		{"keyed-md5 over 16 bytes", "keyed-md5", []string{`secret-hex "` + strings.Repeat("ab", 17) + `"`}, "6.7.3"},
		{"keyed-sha1 over 20 bytes", "keyed-sha1", []string{`secret-hex "` + strings.Repeat("ab", 21) + `"`}, "secret-hex"},
		{"both forms", "keyed-sha1", []string{`secret "ascii"`, `secret-hex "4142"`}, "secret"},
		{"neither form", "keyed-sha1", nil, "secret"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := rfc5880HexKeyResolve(rfc5880HexKeyConfigText(tc.authType, tc.lines...))
			if err == nil {
				t.Fatalf("accepted with session secret % x, want a refusal", got)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("refusal %q does not name %q", err, tc.want)
			}
		})
	}
}
