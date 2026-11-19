//go:build ze_telemetry

// Design: docs/guide/authentication.md -- a migrated account keeps its SHA-crypt hash
// Related: server.go -- basicAuthConfig.validate and basicAuthAccepted, the producers these tests drive

package exporter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// shaCryptScrapeHash is a published test vector of the SHA-crypt specification,
// made from shaCryptScrapePassword.
const (
	shaCryptScrapeHash     = "$6$saltstring$svn8UoSVapNtMuq1ukKS4tPQd8iKwSMHWjl/O817G3uBnIFNjnQJuesI68u4OTLiBFdcbYEdFCoEOfaS35inz1"
	shaCryptScrapePassword = "Hello world!"
)

// TestBasicAuthAcceptsSHACryptHash: the metrics server's basic-auth password
// leaf is a ze:bcrypt leaf, so it accepts what the config validator accepts on
// every such leaf: a SHA-crypt hash starts the server and verifies its password.
//
// VALIDATES: a $6$ hash on the password leaf passes validate and authenticates
// a scrape with the right password, and refuses a wrong one.
// PREVENTS: a hash the config check calls valid that never authenticates.
func TestBasicAuthAcceptsSHACryptHash(t *testing.T) {
	auth := basicAuthConfig{Enabled: true, Username: "prometheus", BcryptHash: shaCryptScrapeHash}
	require.NoError(t, auth.validate())

	assert.True(t, basicAuthAccepted(auth, "prometheus", shaCryptScrapePassword), "right password")
	assert.False(t, basicAuthAccepted(auth, "prometheus", "Hello world?"), "wrong password")
	assert.False(t, basicAuthAccepted(auth, "other", shaCryptScrapePassword), "wrong username")
}

// TestBasicAuthRefusesMalformedSHACryptHash: a value that names SHA-crypt but
// is malformed stops the server at validate, as a plaintext does.
func TestBasicAuthRefusesMalformedSHACryptHash(t *testing.T) {
	auth := basicAuthConfig{Enabled: true, Username: "prometheus", BcryptHash: shaCryptScrapeHash[:len(shaCryptScrapeHash)-1]}
	err := auth.validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bcrypt or SHA-crypt hash")
}
