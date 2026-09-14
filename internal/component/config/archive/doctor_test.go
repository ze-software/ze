// Design: docs/guide/config-archive.md -- the readiness check this component owns
// Detail: doctor.go -- checkArchiveDestinations and its registration
//
// The two destination cases arrived from internal/component/doctor with the
// check they drive, case for case.

package archive

import (
	"errors"
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
// declaration states, and whether the code it emits resolves for `ze explain`.
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
}
