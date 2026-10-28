//go:build ze_bgp

package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
)

// fmtSecretConfig is a BGP peer whose TCP MD5 key (a ze:sensitive leaf) is
// written as %s, so one template carries the clear and the $9$ spellings.
const fmtSecretConfig = `bgp {
	router-id 10.0.0.1
	session {
		asn {
			local 65533
		}
	}
	peer peer3 {
		connection {
			remote {
				ip 10.0.0.3
			}
			md5 {
				password %s
			}
		}
		session {
			asn {
				remote 65534
			}
		}
	}
}
`

// md5PasswordOf parses text and answers the TCP MD5 key the tree holds for
// peer3, which the parser has decoded when the file spelled it $9$.
func md5PasswordOf(t *testing.T, text string) string {
	t.Helper()
	schema, err := config.YANGSchema()
	if err != nil {
		t.Fatalf("YANG schema: %v", err)
	}
	tree, err := config.NewParser(schema).Parse(text)
	if err != nil {
		t.Fatalf("parse: %v\n%s", err, text)
	}
	md5 := tree.GetContainerPath("bgp").GetList("peer")["peer3"].GetContainerPath("connection/md5")
	if md5 == nil {
		t.Fatalf("no md5 container under peer3:\n%s", text)
	}
	value, ok := md5.Get("password")
	if !ok {
		t.Fatalf("no md5 password under peer3:\n%s", text)
	}
	return value
}

// TestConfigFmtPrintsSecretEncoded formats a config whose MD5 key is written
// in clear, through configFmtBytes, the whole of `ze config fmt`.
//
// VALIDATES: the key prints in the $9$ form and not in clear, the printed form
// parses back to the same key, and a second fmt changes no byte although
// secret.Encode draws a random salt: the second fmt reads $9$ text and prints
// the text it read.
// PREVENTS: fmt writing an operator's key in clear into the file, and an fmt
// that re-encodes on every run so `ze config fmt --check` never passes.
func TestConfigFmtPrintsSecretEncoded(t *testing.T) {
	const key = "my-md5-key"
	input := strings.Replace(fmtSecretConfig, "%s", key, 1)

	formatted, _, err := configFmtBytes("test.conf", []byte(input))
	if err != nil {
		t.Fatalf("fmt: %v", err)
	}
	if strings.Contains(formatted, key) {
		t.Errorf("fmt printed the MD5 key in clear:\n%s", formatted)
	}
	if !strings.Contains(formatted, "password $9$") {
		t.Errorf("fmt did not print the MD5 key in the $9$ form:\n%s", formatted)
	}
	if got := md5PasswordOf(t, formatted); got != key {
		t.Errorf("the printed form reads back as key %q, want %q", got, key)
	}

	again, changed, err := configFmtBytes("test.conf", []byte(formatted))
	if err != nil {
		t.Fatalf("second fmt: %v", err)
	}
	if changed || again != formatted {
		t.Errorf("second fmt changed the output:\n%s\nvs\n%s", formatted, again)
	}
}

// TestConfigFmtKeepsStoredSecretEncoding formats a config whose MD5 key is
// already in the $9$ form, the test/parse/config-secret-roundtrip.ci value.
//
// VALIDATES: fmt prints the $9$ string it read byte for byte, the Junos model:
// a stored encoding is printed back, only a value given in clear is encoded.
// PREVENTS: fmt re-encoding a stored key with a fresh salt, which rewrites
// every secret on every run and makes each fmt a diff.
func TestConfigFmtKeepsStoredSecretEncoding(t *testing.T) {
	const stored = "$9$QI1cF39tpBEhrAtM87Nbw"
	input := strings.Replace(fmtSecretConfig, "%s", stored, 1)

	formatted, _, err := configFmtBytes("test.conf", []byte(input))
	if err != nil {
		t.Fatalf("fmt: %v", err)
	}
	if !strings.Contains(formatted, "password "+stored+"\n") {
		t.Errorf("fmt did not keep the stored encoding %s:\n%s", stored, formatted)
	}
}

// TestConfigMigratePrintsSecretEncoded runs `ze config migrate` to the
// hierarchical form on a config whose MD5 key is in clear.
//
// VALIDATES: migrate's hierarchical output carries the key in the $9$ form,
// through the helper fmt uses, and it reads back as the same key.
// PREVENTS: migrate writing a key in clear that fmt would have encoded.
func TestConfigMigratePrintsSecretEncoded(t *testing.T) {
	const key = "my-md5-key"
	path := filepath.Join(t.TempDir(), "in.conf")
	if err := os.WriteFile(path, []byte(strings.Replace(fmtSecretConfig, "%s", key, 1)), 0o600); err != nil {
		t.Fatal(err)
	}

	output, _, _, err := configMigrateWithWarnings(path, "", formHierarchical)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if strings.Contains(output, key) {
		t.Errorf("migrate printed the MD5 key in clear:\n%s", output)
	}
	if got := md5PasswordOf(t, output); got != key {
		t.Errorf("the migrated form reads back as key %q, want %q", got, key)
	}
}

// fmtSecretSetConfig is fmtSecretConfig in the set form, its MD5 key written
// as %s.
const fmtSecretSetConfig = `set bgp router-id 10.0.0.1
set bgp session asn local 65533
set bgp peer peer3 connection remote ip 10.0.0.3
set bgp peer peer3 connection md5 password %s
set bgp peer peer3 session asn remote 65534
`

// setMD5PasswordOf parses set text and answers the TCP MD5 key the tree
// holds for peer3.
func setMD5PasswordOf(t *testing.T, text string) string {
	t.Helper()
	schema, err := config.YANGSchema()
	if err != nil {
		t.Fatalf("YANG schema: %v", err)
	}
	tree, err := config.NewSetParser(schema).Parse(text)
	if err != nil {
		t.Fatalf("set parse: %v\n%s", err, text)
	}
	value, ok := tree.GetContainerPath("bgp").GetList("peer")["peer3"].GetContainerPath("connection/md5").Get("password")
	if !ok {
		t.Fatalf("no md5 password under peer3:\n%s", text)
	}
	return value
}

// TestConfigMigrateSetFormPrintsSecretEncoded runs `ze config migrate` to the
// set form, the default, on a config whose MD5 key is in clear.
//
// VALIDATES: the set output carries the key in the $9$ form, and the set
// parser reads it back as the same key, as the hierarchical parser does.
// PREVENTS: the set writer printing a key in clear, and a set parser that
// keeps the $9$ text as the literal key.
func TestConfigMigrateSetFormPrintsSecretEncoded(t *testing.T) {
	const key = "my-md5-key"
	path := filepath.Join(t.TempDir(), "in.conf")
	if err := os.WriteFile(path, []byte(strings.Replace(fmtSecretConfig, "%s", key, 1)), 0o600); err != nil {
		t.Fatal(err)
	}

	output, _, _, err := configMigrateWithWarnings(path, "", formSet)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if strings.Contains(output, key) {
		t.Errorf("migrate printed the MD5 key in clear:\n%s", output)
	}
	if !strings.Contains(output, "md5 password $9$") {
		t.Errorf("migrate did not print the MD5 key in the $9$ form:\n%s", output)
	}
	if got := setMD5PasswordOf(t, output); got != key {
		t.Errorf("the migrated set form reads back as key %q, want %q", got, key)
	}
}

// TestSetFormKeepsStoredSecretEncoding reads a set config whose MD5 key is in
// the $9$ form, through the set parser and the set-meta parser the editor's
// store uses, and prints it back through each one's printer.
//
// VALIDATES: both parsers hold the decoded key, and both printers write the
// $9$ string read byte for byte, so a set config round-trips unchanged.
// PREVENTS: the set form decoding a key and then printing it in clear, or
// re-encoding it with a fresh salt on every write.
func TestSetFormKeepsStoredSecretEncoding(t *testing.T) {
	const stored = "$9$QI1cF39tpBEhrAtM87Nbw"
	input := strings.Replace(fmtSecretSetConfig, "%s", stored, 1)
	key := md5PasswordOf(t, strings.Replace(fmtSecretConfig, "%s", stored, 1))
	if got := setMD5PasswordOf(t, input); got != key {
		t.Fatalf("the set parser reads the key as %q, the hierarchical parser as %q", got, key)
	}

	schema, err := config.YANGSchema()
	if err != nil {
		t.Fatalf("YANG schema: %v", err)
	}
	tree, err := config.NewSetParser(schema).Parse(input)
	if err != nil {
		t.Fatalf("set parse: %v", err)
	}
	if out := config.SerializeSet(tree, schema); !strings.Contains(out, "md5 password "+stored+"\n") {
		t.Errorf("SerializeSet did not keep the stored encoding %s:\n%s", stored, out)
	}

	metaTree, meta, err := config.NewSetParser(schema).ParseWithMeta("#alice " + strings.Replace(input, "\nset", "\n#alice set", -1))
	if err != nil {
		t.Fatalf("set-meta parse: %v", err)
	}
	out := config.SerializeSetWithMeta(metaTree, meta, schema)
	if !strings.Contains(out, "md5 password "+stored+"\n") {
		t.Errorf("SerializeSetWithMeta did not keep the stored encoding %s:\n%s", stored, out)
	}
	if strings.Contains(out, key) {
		t.Errorf("SerializeSetWithMeta printed the key in clear:\n%s", out)
	}
}
