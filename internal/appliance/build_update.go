// Design: docs/architecture/appliance/builder.md -- the image's updater identity.

package appliance

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/appliance/instance"
	"github.com/ze-software/ze/internal/core/redact"
)

// gokUpdate is the gokrazy Update boundary object. Named builds construct it
// from the same secrets pushOne reads, never from process arguments.
type gokUpdate struct {
	HTTPPassword string
	HTTPSPort    string
	cert         []byte
	key          []byte
}

func readGokUpdate(dir, name string, cfg *applianceConfig, passphrase []byte) (*gokUpdate, error) {
	token, err := readSecret(secretFilePath(dir, name, "update.token"), passphrase)
	if err != nil {
		return nil, fmt.Errorf("read update token: %w", err)
	}
	defer ZeroBytes(token)
	if len(token) == 0 {
		return nil, errors.New("update token is empty; initialize this appliance with ze appliance init <name>")
	}
	cert, err := os.ReadFile(filepath.Join(tLSDir(dir, name), "cert.pem")) //nolint:gosec // Named appliance's public certificate.
	if err != nil {
		return nil, fmt.Errorf("read update certificate: %w", err)
	}
	key, err := readSecret(filepath.Join(tLSDir(dir, name), "key.pem"), passphrase)
	if err != nil {
		return nil, fmt.Errorf("read update private key: %w", err)
	}
	if err := validateTLSPair(cert, key, "appliance cert.pem", "appliance key.pem"); err != nil {
		ZeroBytes(key)
		return nil, err
	}
	port := cfg.Device.UpdatePort
	if port == 0 {
		port = 443
	}
	// The caller MUST clear the returned private key after the build.
	return &gokUpdate{HTTPPassword: string(token), HTTPSPort: strconv.Itoa(port), cert: cert, key: key}, nil
}

// writeGokUpdate changes only the disposable prepared instance. The caller
// MUST remove that instance after gok returns, including on build failure.
func writeGokUpdate(parent string, update *gokUpdate) error {
	path := filepath.Join(parent, instance.Name, "config.json")
	data, err := os.ReadFile(path) //nolint:gosec // Private prepared instance.
	if err != nil {
		return err
	}
	defer ZeroBytes(data)
	var document map[string]json.RawMessage
	if err := json.Unmarshal(data, &document); err != nil {
		return err
	}
	certPath, err := writeGokTLSFile(filepath.Dir(path), ".ze-update-cert-", update.cert)
	if err != nil {
		return err
	}
	keyPath, err := writeGokTLSFile(filepath.Dir(path), ".ze-update-key-", update.key)
	if err != nil {
		return err
	}
	// The upstream fallback discards inline CertPEM/KeyPEM. Its TLS selector
	// reads custom paths from the original Update.UseTLS instead.
	encoded, err := json.Marshal(struct {
		HTTPPassword string
		HTTPSPort    string
		UseTLS       string
	}{HTTPPassword: update.HTTPPassword, HTTPSPort: update.HTTPSPort, UseTLS: certPath + "," + keyPath})
	if err != nil {
		return err
	}
	defer ZeroBytes(encoded)
	document["Update"] = encoded
	patched, err := json.MarshalIndent(document, "", "    ")
	if err != nil {
		return err
	}
	defer ZeroBytes(patched)
	// WriteFile does not change an existing file's mode. Restrict the prepared
	// non-secret configuration before publishing its update password and key.
	if err := os.Chmod(path, 0o600); err != nil {
		return err
	}
	return os.WriteFile(path, patched, 0o600)
}

func writeGokTLSFile(dir, prefix string, data []byte) (string, error) {
	file, err := os.CreateTemp(dir, prefix)
	if err != nil {
		return "", err
	}
	if _, err := file.Write(data); err != nil {
		return "", errors.Join(err, file.Close())
	}
	if err := file.Close(); err != nil {
		return "", err
	}
	return file.Name(), nil
}

// gokBuildFailure keeps the builder's cause while removing its updater secret.
// Upstream can flatten a URL error with %v, so typed URL redaction alone does
// not cover every diagnostic. The prepared password itself remains unchanged.
func gokBuildFailure(err error, update *gokUpdate) string {
	text := redact.URLError(err).Error()
	if update == nil {
		return text
	}
	if update.HTTPPassword == "" {
		return text
	}
	encoded := strings.TrimPrefix(url.UserPassword("", update.HTTPPassword).String(), ":")
	return strings.NewReplacer(encoded, redact.Placeholder, update.HTTPPassword, redact.Placeholder).Replace(text)
}
