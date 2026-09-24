// Design: docs/architecture/aaa-tacacs.md -- TACACS+ configuration and secret handling.
package tacacs

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/aaa"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/secret"
	_ "github.com/ze-software/ze/internal/component/tacacs/yang"
	"github.com/ze-software/ze/internal/core/redact"
)

func parseTacacsConfig(t *testing.T, body string) (*config.Tree, *config.Schema) {
	t.Helper()
	schema, err := config.YANGSchema()
	if err != nil {
		t.Fatal(err)
	}
	tree, err := config.NewParser(schema).Parse("system { authentication { tacacs { " + body + " } } }")
	if err != nil {
		t.Fatal(err)
	}
	return tree, schema
}

// RFC requirement: RFC8907-10.5.1-1 positive -- a parsed $9$ secret remains available for an obfuscated accounting exchange while the display tree masks it.
// RFC requirement: RFC8907-10.5.1-2 positive -- Build accepts and uses a 32-character shared secret in a real accounting exchange without truncation.
func TestRFC8907SharedSecretConfigBuildAndDisplay(t *testing.T) {
	key := "0123456789abcdef0123456789abcdef"
	encoded, err := secret.Encode(key)
	if err != nil {
		t.Fatal(err)
	}
	seen := make(chan []byte, 1)
	srv := newTestServer(t, []byte(key), func(_ PacketHeader, body []byte) []byte {
		seen <- append([]byte(nil), body...)
		return []byte{0, 0, 0, 0, AcctStatusSuccess}
	})
	t.Cleanup(srv.close)
	host, port, err := net.SplitHostPort(srv.addr())
	if err != nil {
		t.Fatal(err)
	}
	tree, schema := parseTacacsConfig(t, "accounting true; server "+host+" { port "+port+"; key "+strconv.Quote(encoded)+"; }")
	contribution, err := (tacacsBackend{}).Build(aaa.BuildParams{ConfigTree: tree})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := contribution.Close(); err != nil {
			t.Error(err)
		}
	})
	if contribution.Accountant == nil || contribution.Authorizer != nil {
		t.Fatal("accounting must be selected independently of authorization")
	}
	contribution.Accountant.CommandStart("alice", "192.0.2.7", "show version")
	select {
	case body := <-seen:
		if len(body) < 9 || body[0] != AcctFlagStart || !bytes.Contains(body, []byte("cmd=show")) {
			t.Fatalf("shared secret did not decode a command accounting START: %x", body)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Build's accountant did not send a START")
	}
	masked := ExtractConfig(config.MaskSecrets(tree, schema))
	if len(masked.Servers) != 1 || string(masked.Servers[0].Key) != config.SecretDataPlaceholder {
		t.Fatal("display tree exposes the TACACS+ shared secret")
	}
	live := ExtractConfig(tree)
	if len(live.Servers) != 1 || string(live.Servers[0].Key) != key {
		t.Fatal("display masking altered the live shared secret")
	}
}

// RFC requirement: RFC8907-10.5.1-1 negative -- fmt, Go syntax, JSON and slog renderings of an extracted server never contain the secret, its byte list or its base64 encoding.
func TestRFC8907SharedSecretDiagnosticRedaction(t *testing.T) {
	key := "private-TACACS-secret-32-characters"
	tree, _ := parseTacacsConfig(t, "server 192.0.2.1 { key "+strconv.Quote(key)+"; }")
	cfg := ExtractConfig(tree)
	encoded, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	var log bytes.Buffer
	slog.New(slog.NewJSONHandler(&log, nil)).Info("configuration", "server", cfg.Servers[0], "config", cfg)
	outputs := []string{fmt.Sprintf("%v", cfg), fmt.Sprintf("%+v", cfg), fmt.Sprintf("%#v", cfg), string(encoded), log.String()}
	for _, output := range outputs {
		for _, exposed := range []string{key, base64.StdEncoding.EncodeToString([]byte(key)), fmt.Sprint([]byte(key))} { //nolint:staticcheck // QF1010: the decimal byte-list rendering of the key is one of the exposures this test forbids.
			if strings.Contains(output, exposed) {
				t.Fatalf("diagnostic output exposes a shared secret: %s", output)
			}
		}
		if !strings.Contains(output, "192.0.2.1:49") {
			t.Fatalf("redaction removed the endpoint needed to diagnose the configuration: %s", output)
		}
	}
}

// RFC requirement: RFC8907-10-2 negative -- Build refuses a server without a shared secret instead of constructing a client that could send an unobfuscated packet.
func TestRFC8907BuildRejectsMissingSecret(t *testing.T) {
	tree, _ := parseTacacsConfig(t, "server 192.0.2.1 { }")
	contribution, err := (tacacsBackend{}).Build(aaa.BuildParams{ConfigTree: tree})
	if contribution.Close != nil {
		t.Cleanup(func() { _ = contribution.Close() })
	}
	if !errors.Is(err, ErrNoSharedSecret) || contribution.Authenticator != nil {
		t.Fatalf("keyless server accepted: contribution=%+v err=%v", contribution, err)
	}
}

// RFC requirement: RFC8907-8.3-4 negative -- selecting accounting without any destination fails Build rather than silently disabling accounting.
func TestRFC8907AccountingRequiresDestination(t *testing.T) {
	tree, _ := parseTacacsConfig(t, "accounting true;")
	contribution, err := (tacacsBackend{}).Build(aaa.BuildParams{ConfigTree: tree})
	if err == nil || contribution.Accountant != nil {
		t.Fatalf("accounting without a destination was accepted: %v", err)
	}
}

// Independent protocol selection remains valid under RFC 8907 Section 1.
func TestTacacsAccountingSelectionIsOptional(t *testing.T) {
	for _, body := range []string{"accounting false;", "server 192.0.2.1 { key \"local-secret\"; }"} {
		tree, _ := parseTacacsConfig(t, body)
		contribution, err := (tacacsBackend{}).Build(aaa.BuildParams{ConfigTree: tree})
		if err != nil {
			t.Fatal(err)
		}
		if contribution.Close != nil {
			t.Cleanup(func() { _ = contribution.Close() })
		}
		if contribution.Accountant != nil {
			t.Fatal("accounting enabled without being selected")
		}
	}
}

// RFC requirement: RFC8907-10.5.1-1 negative -- config-command display masks an entire quoted shared secret using the TACACS+ schema, including a malformed quoted value.
func TestRFC8907SharedSecretCommandRedaction(t *testing.T) {
	for _, input := range []string{
		`set system authentication tacacs server 192.0.2.1 key "private secret with spaces"`,
		`config set system authentication tacacs server 192.0.2.1 key "private secret with spaces"`,
		`ze config set /etc/ze.conf system authentication tacacs server 192.0.2.1 key "private unfinished secret`,
	} {
		output := config.DisplayCommand(input)
		if strings.Contains(output, "private") || strings.Contains(output, "spaces") || strings.Contains(output, "unfinished") {
			t.Fatalf("configuration command leaked its shared secret: %s", output)
		}
		if !strings.Contains(output, "192.0.2.1 key "+redact.Placeholder) {
			t.Fatalf("redacted command lost its configuration path: %s", output)
		}
	}
	ordinary := "set system authentication tacacs server 192.0.2.1 port 4949"
	if got := config.DisplayCommand(ordinary); got != ordinary {
		t.Fatalf("nonsecret configuration changed: %s", got)
	}
}

// Enabling command authorization without a destination cannot silently leave
// permissive local policy in charge, including when strict fallback is set.
func TestTacacsAuthorizationRequiresDestination(t *testing.T) {
	tree, _ := parseTacacsConfig(t, "authorization true; strict-fallback true;")
	contribution, err := (tacacsBackend{}).Build(aaa.BuildParams{ConfigTree: tree})
	if err == nil {
		t.Fatal("authorization without a destination was accepted")
	}
	if contribution.Authorizer != nil {
		t.Fatal("invalid authorization configuration installed a policy")
	}
}
