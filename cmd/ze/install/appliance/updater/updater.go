// Design: plan/spec-install-7c-vendor-updater.md — vendored gokrazy updater library
//
// Package updater implements updating the different parts of a running gokrazy
// installation (boot/root file systems and MBR).
//
// Vendored from github.com/gokrazy/updater v0.0.0-20250705135802-db129c40879c
// to avoid adding external dependencies to ze's main go.mod.
package updater

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"hash/crc32"
	"io"
	"net/http"
	"slices"
	"strings"
)

// ErrUpdateHandlerNotImplemented is returned when the requested update
// destination is not yet implemented on the target device.
var ErrUpdateHandlerNotImplemented = errors.New("update handler not implemented")

// A HTTPDoer is satisfied by any *http.Client, but also easy to implement in
// case extra middleware is desired.
type HTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}

// Target represents a gokrazy installation to be updated.
type Target struct {
	doer HTTPDoer

	baseURL  string
	supports []string

	eeprom EEPROMVersion
}

// NewTarget queries the target for supported update protocol features and
// returns a ready-to-use updater Target.
func NewTarget(ctx context.Context, baseURL string, httpClient HTTPDoer) (*Target, error) {
	target := &Target{
		baseURL: baseURL,
		doer:    httpClient,
	}
	if err := target.requestFeatures(ctx); err != nil {
		return nil, err
	}

	return target, nil
}

// A ProtocolFeature represents an optionally available feature of the update
// protocol.
type ProtocolFeature string

const (
	ProtocolFeaturePARTUUID   ProtocolFeature = "partuuid"
	ProtocolFeatureUpdateHash ProtocolFeature = "updatehash"
)

// Supports returns whether the target is known to support the specified update
// protocol feature.
func (t *Target) Supports(feature ProtocolFeature) bool {
	return slices.Contains(t.supports, string(feature))
}

// StreamTo streams from the specified io.Reader to the specified destination:
//
//   - mbr: stream content directly onto the root block device
//   - root: stream content to the currently inactive root partition
//   - boot: stream content to the boot partition
//   - bootonly: stream content to the boot partition, keep active root
func (t *Target) StreamTo(ctx context.Context, dest string, r io.Reader) error {
	updateHash := t.Supports("updatehash")
	var h hash.Hash
	if updateHash {
		h = crc32.NewIEEE()
	} else {
		h = sha256.New()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, t.baseURL+"update/"+dest, io.TeeReader(r, h))
	if err != nil {
		return err
	}
	if updateHash {
		req.Header.Set("X-Gokrazy-Update-Hash", "crc32")
	}
	resp, err := t.doer.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck // response body
	if got, want := resp.StatusCode, http.StatusOK; got != want {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected HTTP status code: got %v, want %v (body %q)", resp.Status, want, string(body))
	}
	// The response is a hex-encoded hash (tens of bytes); cap it defensively
	// since the target is a remote device.
	remoteHash, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	if bytes.HasPrefix(remoteHash, []byte("<!DOCTYPE html>")) {
		return ErrUpdateHandlerNotImplemented
	}
	decoded := make([]byte, hex.DecodedLen(len(remoteHash)))
	n, err := hex.Decode(decoded, remoteHash)
	if err != nil {
		return err
	}
	if got, want := decoded[:n], h.Sum(nil); !bytes.Equal(got, want) {
		return fmt.Errorf("unexpected checksum: got %x, want %x", got, want)
	}
	return nil
}

// Switch changes the active root partition from the currently running root
// partition to the currently inactive root partition.
func (t *Target) Switch(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "POST", t.baseURL+"update/switch", http.NoBody)
	if err != nil {
		return err
	}
	resp, err := t.doer.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck // response body
	if got, want := resp.StatusCode, http.StatusOK; got != want {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected HTTP status code: got %d, want %d (body %q)", got, want, string(body))
	}
	return nil
}

// Testboot marks the inactive root partition to be tested upon the next boot,
// and made active if the test boot succeeds.
func (t *Target) Testboot(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "POST", t.baseURL+"update/testboot", http.NoBody)
	if err != nil {
		return err
	}
	resp, err := t.doer.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck // response body
	if got, want := resp.StatusCode, http.StatusOK; got != want {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected HTTP status code: got %d, want %d (body %q)", got, want, string(body))
	}
	return nil
}

// RebootOption configures a reboot operation.
type RebootOption func(*rebootConfig)

type rebootConfig struct {
	kexec bool
	async bool
}

// WithKexec configures whether to use kexec for rebooting.
func WithKexec(kexec bool) RebootOption {
	return func(c *rebootConfig) {
		c.kexec = kexec
	}
}

// WithAsync configures whether to perform an asynchronous reboot.
func WithAsync(async bool) RebootOption {
	return func(c *rebootConfig) {
		c.async = async
	}
}

// Reboot reboots the target, picking up the updated partitions.
func (t *Target) Reboot(ctx context.Context, opts ...RebootOption) error {
	config := &rebootConfig{
		kexec: true,
		async: false,
	}

	for _, opt := range opts {
		opt(config)
	}

	u := t.baseURL + "reboot"
	params := make([]string, 0)
	if !config.kexec {
		params = append(params, "kexec=off")
	}
	if config.async {
		params = append(params, "async=true")
	}

	if len(params) > 0 {
		u += "?" + strings.Join(params, "&")
	}

	req, err := http.NewRequestWithContext(ctx, "POST", u, http.NoBody)
	if err != nil {
		return err
	}
	resp, err := t.doer.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck // response body
	if got, want := resp.StatusCode, http.StatusOK; got != want {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected HTTP status code: got %d, want %d (body %q)", got, want, string(body))
	}
	return nil
}

// InstalledEEPROM returns the Raspberry Pi EEPROM version currently installed
// on the target device.
func (t *Target) InstalledEEPROM() EEPROMVersion {
	return t.eeprom
}

func (t *Target) requestFeatures(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", t.baseURL+"update/features", http.NoBody)
	if err != nil {
		return err
	}

	resp, err := t.doer.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck // response body

	if resp.StatusCode == http.StatusNotFound {
		return nil
	}

	if got, want := resp.StatusCode, http.StatusOK; got != want {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected HTTP status code: got %d, want %d (body %q)", got, want, string(body))
	}

	// Cap the response: a features blob is tiny, but the target is a remote
	// device, so guard against an unbounded body exhausting memory.
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}

	if strings.HasPrefix(resp.Header.Get("Content-Type"), "text/plain") {
		t.supports = strings.Split(strings.TrimSpace(string(body)), ",")
		t.eeprom = EEPROMVersion{}
		return nil
	}

	var featuresResp struct {
		Features string        `json:"features"`
		EEPROM   EEPROMVersion `json:"EEPROM"`
	}
	if err := json.Unmarshal(body, &featuresResp); err != nil {
		return err
	}
	t.supports = strings.Split(strings.TrimSpace(featuresResp.Features), ",")
	t.eeprom = featuresResp.EEPROM
	return nil
}

// EEPROMVersion contains the signatures of a set of Raspberry Pi EEPROM files.
type EEPROMVersion struct {
	PieepromSHA256 string
	VL805SHA256    string
}
