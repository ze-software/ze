// Design: docs/architecture/appliance/installer-initrd.md -- rescue-shell credential encoding

// Package rescueauth encodes and verifies the installer's rescue-shell
// credential. It is shared by the provisioner that mints the credential, the
// image server that puts it on the installer kernel cmdline, and the installer
// initrd that checks a typed token against it.
package rescueauth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// The rescue credential travels on the installer kernel cmdline
// (ze.rescue-auth), which crosses the PXE network in clear text inside an
// unauthenticated iPXE script and is readable from /proc/cmdline. It is
// therefore a public value, and the only thing standing between it and the
// rescue shell is the cost of inverting it.
//
// Two properties follow:
//
//   - It must be a slow, salted KDF. An unsalted sha256 of an operator secret is
//     recoverable offline with commodity hardware and rainbow tables.
//   - It must not commit to any credential that grants more than the rescue
//     shell. The provisioner mints a dedicated random rescue token for this and
//     never derives it from the admin password.
//
// Encoding is "<saltHex>:<digestHex>". Hex plus ':' is safe both in an iPXE
// script (no '$' to interpolate) and on a kernel cmdline (no spaces or quotes).
const (
	SaltLen   = 16
	digestLen = 32

	// argon2id parameters. The verifier runs once per typed attempt on a machine
	// that has just failed to install, so latency matters far less than cost to
	// an attacker; memory is kept at 64 MiB so the initrd stays comfortable on
	// small appliances.
	argonTime    = 1
	argonMemory  = 64 * 1024
	argonThreads = 4

	separator = ':'

	// tokenBytes of entropy is ample for a credential that is argon2id-verified
	// and limited to rescueMaxAttempts tries at a console, and short enough that
	// an operator will actually type it rather than paste it somewhere unsafe.
	tokenBytes      = 10
	tokenGroupBytes = 2
)

var (
	errEmpty     = errors.New("empty")
	errShape     = errors.New("expected <saltHex>:<digestHex>")
	errSaltLen   = errors.New("salt must be 32 lowercase hex chars")
	errDigestLen = errors.New("digest must be 64 lowercase hex chars")
	errHex       = errors.New("salt and digest must be lowercase hex")
)

// Value encodes salt and the argon2id digest of token as the
// "<saltHex>:<digestHex>" string carried on the installer kernel cmdline.
//
// Callers MUST pass a salt of SaltLen bytes from a cryptographic random
// source; a reused or predictable salt defeats the point of having one.
func Value(token string, salt []byte) string {
	digest := digestOf(token, salt)

	var tb textbuf.Buffer
	return tb.Hex(salt).Byte(separator).Hex(digest).String()
}

// Check reports whether typed matches the credential encoded in
// authValue. It fails closed: a malformed authValue never authenticates.
func Check(typed, authValue string) bool {
	salt, want, err := split(authValue)
	if err != nil {
		return false
	}
	got := digestOf(typed, salt)
	return subtle.ConstantTimeCompare(got, want) == 1
}

// Validate checks the shape of an encoded credential without verifying any
// token against it. Config parsers and the installer cmdline parser use it so a
// malformed value is rejected where it enters, not at the rescue prompt where a
// decode failure is indistinguishable from a wrong token.
func Validate(authValue string) error {
	_, _, err := split(authValue)
	return err
}

// NewValue mints a fresh rescue token and returns it alongside the encoded
// credential to publish. The token is shown to the operator once and never
// stored; only the returned value is written to config or a kernel cmdline.
func NewValue() (token, authValue string, err error) {
	token, err = NewToken()
	if err != nil {
		return "", "", err
	}
	salt := make([]byte, SaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", "", fmt.Errorf("read rescue salt: %w", err)
	}
	return token, Value(token, salt), nil
}

// NewToken returns a random rescue token in the operator-facing form
// "xx-xx-...": tokenBytes of entropy, hex-encoded and grouped so it can be read
// off a provisioning log and typed at a serial console.
func NewToken() (string, error) {
	raw := make([]byte, tokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("read rescue token: %w", err)
	}

	var tb textbuf.Buffer
	for i := 0; i < tokenBytes; i += tokenGroupBytes {
		if i > 0 {
			tb.Byte('-')
		}
		tb.Hex(raw[i : i+tokenGroupBytes])
	}
	return tb.String(), nil
}

// digestOf derives the argon2id digest of token under salt.
func digestOf(token string, salt []byte) []byte {
	return argon2.IDKey([]byte(token), salt, argonTime, argonMemory, argonThreads, digestLen)
}

// split decodes a "<saltHex>:<digestHex>" value into its raw parts.
func split(authValue string) (salt, digest []byte, err error) {
	if authValue == "" {
		return nil, nil, errEmpty
	}
	saltHex, digestHex, found := strings.Cut(authValue, string(separator))
	if !found {
		return nil, nil, errShape
	}
	if len(saltHex) != SaltLen*2 {
		return nil, nil, errSaltLen
	}
	if len(digestHex) != digestLen*2 {
		return nil, nil, errDigestLen
	}
	salt, err = hex.DecodeString(saltHex)
	if err != nil {
		return nil, nil, errHex
	}
	digest, err = hex.DecodeString(digestHex)
	if err != nil {
		return nil, nil, errHex
	}
	// hex.DecodeString accepts uppercase; the cmdline form is lowercase only so
	// the value has exactly one spelling and comparisons stay exact.
	if strings.ToLower(authValue) != authValue {
		return nil, nil, errHex
	}
	return salt, digest, nil
}
