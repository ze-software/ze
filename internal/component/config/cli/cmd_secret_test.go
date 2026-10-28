package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/component/config/secret"
	"github.com/ze-software/ze/internal/core/cliio"
)

// runSecretCapture runs one `ze secret` invocation with stdin set to input and
// returns the exit code, stdout and stderr.
func runSecretCapture(t *testing.T, input string, args ...string) (int, string, string) {
	t.Helper()
	restore := cliio.SwapStreams(strings.NewReader(input), &bytes.Buffer{})
	defer restore()

	var stdout, stderr bytes.Buffer
	code := runSecret(args, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

// TestSecretRoundTrip proves decode returns exactly the plaintext encode was
// given, through the command's own operand handling, for plaintexts that carry
// shell-hostile characters, spaces and multi-byte UTF-8.
func TestSecretRoundTrip(t *testing.T) {
	for _, plain := range []string{"my-md5-key", "p@ss w0rd!$", "clé", "x"} {
		code, encoded, stderr := runSecretCapture(t, "", "encode", plain)
		if code != exitOK {
			t.Fatalf("encode %q: exit %d, stderr %q", plain, code, stderr)
		}
		encoded = strings.TrimSuffix(encoded, "\n")
		if !secret.IsEncoded(encoded) {
			t.Fatalf("encode %q answered %q, which has no $9$ prefix", plain, encoded)
		}
		code, decoded, stderr := runSecretCapture(t, "", "decode", encoded)
		if code != exitOK {
			t.Fatalf("decode %q: exit %d, stderr %q", encoded, code, stderr)
		}
		if decoded != plain+"\n" {
			t.Errorf("decode(encode(%q)) = %q, want the plaintext and one newline", plain, decoded)
		}
	}
}

// TestSecretDecodeStdin proves "-" reads the value from stdin, dropping the one
// line ending echo adds and nothing else.
func TestSecretDecodeStdin(t *testing.T) {
	encoded, err := secret.Encode("from-stdin ")
	if err != nil {
		t.Fatal(err)
	}
	for _, input := range []string{encoded, encoded + "\n", encoded + "\r\n"} {
		code, stdout, stderr := runSecretCapture(t, input, "decode", "-")
		if code != exitOK {
			t.Fatalf("decode - with %q: exit %d, stderr %q", input, code, stderr)
		}
		if stdout != "from-stdin \n" {
			t.Errorf("decode - with %q = %q, want %q", input, stdout, "from-stdin \n")
		}
	}
}

// TestSecretRefusals proves every input that cannot be decoded or encoded is
// refused with its own exit code and a message that names the problem and the
// command that fixes it, and that nothing reaches stdout.
func TestSecretRefusals(t *testing.T) {
	bcrypt := "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"
	tests := []struct {
		name  string
		input string
		args  []string
		code  int
		want  []string
	}{
		{"bcrypt hash", "", []string{"decode", bcrypt}, exitInvalid,
			[]string{"one-way password hash ($2a$)", "cannot be decoded", "ze config set"}},
		{"sha-512 crypt", "", []string{"decode", "$6$salt$abcdef"}, exitInvalid,
			[]string{"one-way password hash ($6$)", "cannot be decoded"}},
		{"bcrypt on stdin", bcrypt + "\n", []string{"decode", "-"}, exitInvalid,
			[]string{"one-way password hash ($2a$)"}},
		{"no prefix", "", []string{"decode", "plaintext"}, exitInvalid,
			[]string{"does not start with $9$", "ze secret decode '$9$...'"}},
		{"bad character", "", []string{"decode", "$9$~~~~"}, exitInvalid,
			[]string{"not a valid $9$ secret", "character", "single quotes"}},
		{"cut short", "", []string{"decode", "$9$"}, exitInvalid,
			[]string{"not a valid $9$ secret", "ends too early"}},
		{"empty stdin", "", []string{"decode", "-"}, exitInvalid,
			[]string{"value to decode is empty"}},
		{"encode already encoded", "", []string{"encode", "$9$abc"}, exitInvalid,
			[]string{"already encoded", "ze secret decode"}},
		{"missing value", "", []string{"decode"}, exitError,
			[]string{"needs the value", "ze secret decode -"}},
		{"two values", "", []string{"decode", "a", "b"}, exitError,
			[]string{"takes one value, and it got 2"}},
		{"dash value", "", []string{"encode", "-x"}, exitError,
			[]string{"has no option \"-x\"", "ze secret encode -"}},
		{"unknown action", "", []string{"reveal", "x"}, exitError,
			[]string{"unknown ze secret action \"reveal\"", "decode and encode"}},
		{"no action", "", nil, exitError,
			[]string{"ze secret decode <value|->"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, stdout, stderr := runSecretCapture(t, tt.input, tt.args...)
			if code != tt.code {
				t.Errorf("exit %d, want %d (stderr %q)", code, tt.code, stderr)
			}
			if stdout != "" {
				t.Errorf("stdout %q, want nothing", stdout)
			}
			for _, want := range tt.want {
				if !strings.Contains(stderr, want) {
					t.Errorf("stderr %q does not contain %q", stderr, want)
				}
			}
		})
	}
}

// TestSecretHelp proves every help spelling prints the page to stdout and exits
// zero, and that the page says how to keep the value out of shell history.
func TestSecretHelp(t *testing.T) {
	for _, args := range [][]string{{"help"}, {"--help"}, {"-h"}, {"decode", "--help"}} {
		code, stdout, _ := runSecretCapture(t, "", args...)
		if code != exitOK {
			t.Errorf("%v: exit %d, want 0", args, code)
		}
		if !strings.Contains(stdout, "shell history") {
			t.Errorf("%v: help does not mention stdin keeping the value out of shell history", args)
		}
	}
}

// TestSecretRootRegistered proves `ze secret` reaches runSecret through the
// root registry, with both actions offered for completion.
func TestSecretRootRegistered(t *testing.T) {
	if registry.LookupRoot("secret") == nil {
		t.Fatal("secret root handler not registered")
	}
	for _, cmd := range registry.ListRoot() {
		if cmd.Name != "secret" {
			continue
		}
		if got := cmd.Meta.ResolveSubs(); got != "decode, encode" {
			t.Errorf("secret Subs = %q, want %q", got, "decode, encode")
		}
		return
	}
	t.Fatal("secret root command has no metadata in ListRoot")
}
