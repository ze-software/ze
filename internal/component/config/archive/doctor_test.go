// Design: docs/guide/config-archive.md -- the readiness check this component owns
// Detail: doctor.go -- checkArchiveDestinations and its registration
//
// The two destination cases arrived from internal/component/doctor with the
// check they drive, case for case.

package archive

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// withDestinationProbe stands in the HTTP HEAD probe for one test and records
// every location it was asked about.
func withDestinationProbe(t *testing.T, err error) *[]string {
	t.Helper()
	previous := archiveHTTPHead
	var asked []string
	archiveHTTPHead = func(location string, _ time.Duration) error {
		asked = append(asked, location)
		return err
	}
	t.Cleanup(func() { archiveHTTPHead = previous })
	return &asked
}

// archiveTree builds a config with one named archive block at location.
func archiveTree(name, location string) *config.Tree {
	tree := config.NewTree()
	block := config.NewTree()
	block.Set("location", location)
	tree.GetOrCreateContainer("system").AddListEntry("archive", name, block)
	return tree
}

func TestDoctorArchiveDestinations_HTTPUnreachable(t *testing.T) {
	asked := withDestinationProbe(t, errors.New("connection refused"))

	diags := checkArchiveDestinations(diagnostic.DoctorCheckContext{Tree: archiveTree("remote-backup", "https://archive.example.invalid/configs")})
	require.Len(t, diags, 1)
	assert.Equal(t, codeArchiveUnreachable, diags[0].Code)
	assert.Equal(t, diagnostic.SeverityWarning, diags[0].Severity)
	assert.Contains(t, diags[0].Message, "remote-backup")
	assert.Equal(t, []string{"https://archive.example.invalid/configs"}, *asked)
}

// TestDoctorArchiveDestinations_Redirected drives the real probe against a
// destination that answers with a redirect, and against one that answers 200.
//
// VALIDATES: a destination that redirects is reported under
// codeArchiveRedirected, naming the Location it answered with, with the `set`
// command that configures that location as the fix, and the probe never
// reaches the target; a destination that answers 200 is reported nothing.
// PREVENTS: `ze doctor` reporting a destination healthy while ToHTTP, which
// follows no redirect, refuses every upload to it.
func TestDoctorArchiveDestinations_Redirected(t *testing.T) {
	var targetHits atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		targetHits.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()

	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Location", target.URL+"/configs")
		w.WriteHeader(http.StatusPermanentRedirect)
	}))
	defer origin.Close()

	diags := checkArchiveDestinations(diagnostic.DoctorCheckContext{Tree: archiveTree("remote-backup", origin.URL+"/configs")})
	require.Len(t, diags, 1)
	assert.Equal(t, codeArchiveRedirected, diags[0].Code)
	assert.Equal(t, diagnostic.SeverityWarning, diags[0].Severity)
	assert.Contains(t, diags[0].Message, "remote-backup")
	assert.Contains(t, diags[0].Message, "308")
	assert.Contains(t, diags[0].Message, target.URL+"/configs")
	assert.Equal(t, "set system archive remote-backup location "+target.URL+"/configs", diags[0].Fix)
	assert.Zero(t, targetHits.Load(), "the probe must not follow the redirect")

	assert.Empty(t, checkArchiveDestinations(diagnostic.DoctorCheckContext{Tree: archiveTree("direct", target.URL+"/configs")}),
		"a destination that answers 200 is healthy")
}

func TestDoctorArchiveDestinations_FileSkipped(t *testing.T) {
	asked := withDestinationProbe(t, errors.New("connection refused"))

	diags := checkArchiveDestinations(diagnostic.DoctorCheckContext{Tree: archiveTree("local", "file:///var/backup/configs")})
	assert.Empty(t, diags, "file:// archives should not be probed by HTTP")
	assert.Empty(t, *asked)
}

func TestDoctorArchiveDestinations_SilentWithoutArchives(t *testing.T) {
	asked := withDestinationProbe(t, errors.New("connection refused"))

	assert.Empty(t, checkArchiveDestinations(diagnostic.DoctorCheckContext{Tree: config.NewTree()}), "no block")
	assert.Empty(t, checkArchiveDestinations(diagnostic.DoctorCheckContext{}), "nil tree")
	assert.Empty(t, *asked)
}

// TestArchiveDoctorCheckRegistered asks the registry the doctor runner reads
// whether it holds this component's check, at the phase and order the
// declaration states, and whether the code it emits resolves for `ze show diagnostic`.
//
// VALIDATES: the init() in register.go installed the check and the registry
// accepted it.
// PREVENTS: a check that `ze doctor` stopped running while its unit tests
// stayed green.
func TestArchiveDoctorCheckRegistered(t *testing.T) {
	want := archiveDoctorCheck
	var found *diagnostic.DoctorCheck
	checks := diagnostic.DoctorChecksForPhase(want.Phase)
	for i := range checks {
		if checks[i].Name == want.Name {
			found = &checks[i]
			break
		}
	}
	require.NotNil(t, found, "doctor check %q is not registered for phase %q", want.Name, want.Phase)
	assert.Equal(t, want.Order, found.Order)
	assert.Equal(t, want.Component, found.Component)

	diagnostic.RegisterBuiltinCodes()
	assert.NotNil(t, diagnostic.Lookup(codeArchiveUnreachable), "diagnostic code %q is not registered", codeArchiveUnreachable)
	assert.NotNil(t, diagnostic.Lookup(codeArchiveRedirected), "diagnostic code %q is not registered", codeArchiveRedirected)
	assert.Contains(t, found.Codes, codeArchiveRedirected, "the check must declare the redirect code it emits")
}
