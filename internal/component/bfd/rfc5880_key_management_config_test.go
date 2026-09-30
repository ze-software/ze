// VALIDATES: RFC 5880 Sections 6.7.2, 6.7.3 and 6.7.4 key management through
// the interface the operator uses: config text, parsed against the bfd YANG
// module, handed to the plugin as its config section, and resolved into the
// session request a client's session is built from.
// PREVENTS: a management interface that refuses, trims, unescapes twice or
// otherwise re-encodes an ASCII password or key between the operator's config
// and the session's authentication settings.
package bfd

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/bfd/api"
	bfdyang "github.com/ze-software/ze/internal/component/bfd/yang"
	"github.com/ze-software/ze/internal/component/config"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"

	// The bfd command module imports ze-cli-show-cmd, and the YANG loader
	// resolves every registered module, so the show module must be registered.
	_ "github.com/ze-software/ze/internal/component/cmd/show/yang"
)

// rfc5880KeyConfigText renders a bfd config holding one profile whose auth
// block names authType and secret. The secret is written as a quoted config
// string, escaping only the two characters the config tokenizer treats as
// special inside quotes: the backslash and the double quote.
func rfc5880KeyConfigText(authType, secret string) string {
	quoted := strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(secret)
	return "bfd {\n" +
		"    profile keyed {\n" +
		"        auth {\n" +
		"            type " + authType + ";\n" +
		"            key-id 9;\n" +
		"            secret \"" + quoted + "\";\n" +
		"        }\n" +
		"    }\n" +
		"}\n"
}

// rfc5880SecretFromConfigText drives the whole management path: the config
// text is parsed against the bfd YANG module, the plugin map is serialized as
// the config section the plugin receives, parseSections loads it, and
// resolveProfile builds the session request a client's session starts from.
// It returns the secret that request carries.
func rfc5880SecretFromConfigText(t *testing.T, authType, secret string) []byte {
	t.Helper()
	tree, err := config.ParseTreeWithYANG(rfc5880KeyConfigText(authType, secret), map[string]string{
		configRoot: bfdyang.ZeBFDConfYANG,
	})
	if err != nil {
		t.Fatalf("%s secret %q: config parse refused it: %v", authType, secret, err)
	}
	data, err := json.Marshal(map[string]any{configRoot: tree.ToPluginMap()[configRoot]})
	if err != nil {
		t.Fatalf("%s secret %q: marshal the bfd section: %v", authType, secret, err)
	}
	cfg, err := parseSections([]sdk.ConfigSection{{Root: configRoot, Data: string(data)}})
	if err != nil {
		t.Fatalf("%s secret %q: the plugin refused the section: %v", authType, secret, err)
	}
	req, err := resolveProfile(api.SessionRequest{Profile: "keyed", Mode: api.SingleHop}, cfg.profiles)
	if err != nil {
		t.Fatalf("%s secret %q: resolve profile: %v", authType, secret, err)
	}
	if req.Auth == nil {
		t.Fatalf("%s secret %q: the session request carries no auth settings", authType, secret)
	}
	return req.Auth.Secret
}

// rfc5880KeyTypes lists each auth type with the longest key its Auth Len
// field can describe: 16 bytes for Simple Password (Section 6.7.2) and the
// MD5 variants (Section 6.7.3), 20 bytes for the SHA1 variants (Section
// 6.7.4).
var rfc5880KeyTypes = []struct {
	name   string
	keyMax int
}{
	{"simple-password", 16},
	{"keyed-md5", 16},
	{"meticulous-keyed-md5", 16},
	{"keyed-sha1", 20},
	{"meticulous-keyed-sha1", 20},
}

// RFC requirement: RFC5880-6.7.2-1 positive -- "For interoperability, the
// management interface by which the password is configured MUST accept ASCII
// strings." A simple-password secret written in the operator's config text
// reaches the session request's Secret byte for byte.
// RFC requirement: RFC5880-6.7.3-7 positive -- the same config entry accepts
// an ASCII key for keyed-md5 and meticulous-keyed-md5, and the session request
// carries it byte for byte.
// RFC requirement: RFC5880-6.7.4-5 positive -- the same config entry accepts
// an ASCII key for keyed-sha1 and meticulous-keyed-sha1, and the session
// request carries it byte for byte.
func TestRFC5880KeyManagementConfigEntryAcceptsASCIIStrings(t *testing.T) {
	const secret = "BFD-SHARED-KEY1"
	for _, kt := range rfc5880KeyTypes {
		t.Run(kt.name, func(t *testing.T) {
			got := rfc5880SecretFromConfigText(t, kt.name, secret)
			if string(got) != secret {
				t.Fatalf("session secret = %q, want the configured ASCII string %q", got, secret)
			}
		})
	}
}

// RFC requirement: RFC5880-6.7.2-1 negative -- the input is forced toward the
// violation: every ASCII character from 0x01 to 0x7F, including the ones the
// config syntax gives a meaning (quote, backslash, semicolon, braces, hash,
// space, tab, newline), plus a leading and a trailing space, is written as a
// simple-password secret. A management interface that refused, trimmed or
// re-encoded any of them fails here; every chunk reaches the session request
// byte for byte.
// RFC requirement: RFC5880-6.7.3-7 negative -- the same sweep for keyed-md5
// and meticulous-keyed-md5 keys.
// RFC requirement: RFC5880-6.7.4-5 negative -- the same sweep for keyed-sha1
// and meticulous-keyed-sha1 keys, in chunks up to their 20-byte limit.
func TestRFC5880KeyManagementConfigEntryKeepsEveryASCIICharacter(t *testing.T) {
	var all strings.Builder
	for c := byte(0x01); c <= 0x7F; c++ {
		all.WriteByte(c)
	}
	ascii := all.String()
	for _, kt := range rfc5880KeyTypes {
		t.Run(kt.name, func(t *testing.T) {
			chunks := []string{" leading", "trailing ", " both "}
			for start := 0; start < len(ascii); start += kt.keyMax {
				chunks = append(chunks, ascii[start:min(start+kt.keyMax, len(ascii))])
			}
			for _, secret := range chunks {
				got := rfc5880SecretFromConfigText(t, kt.name, secret)
				if string(got) != secret {
					t.Errorf("session secret = %q, want the configured ASCII string %q", got, secret)
				}
			}
		})
	}
}
