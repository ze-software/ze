// Design: docs/features/cli-commands.md -- publication failure survives worker join.
package cli

import (
	"errors"
	"io/fs"
	"testing"

	"github.com/stretchr/testify/require"
)

type failingHistoryPublication struct {
	failure error
}

func (*failingHistoryPublication) ReadFile(string) ([]byte, error)               { return nil, fs.ErrNotExist }
func (s *failingHistoryPublication) WriteFile(string, []byte, fs.FileMode) error { return s.failure }

// TestHistoryCloseRetainsFailedPublication proves Close's final status names
// the actual write failure rather than interpreting a drained queue as success.
func TestHistoryCloseRetainsFailedPublication(t *testing.T) {
	failure := errors.New("history disk refused publication")
	history := NewHistory(&failingHistoryPublication{failure: failure}, "owner")
	_, err := history.Record(ModeOperational, nil, "show bgp")
	require.NoError(t, err)
	history.Close()
	require.ErrorIs(t, history.LastWriteError(), failure)
}

// TestModelCloseReturnsHistoryPublicationFailure follows the actual model
// owner, so a write error cannot disappear between History and SSH cleanup.
func TestModelCloseReturnsHistoryPublicationFailure(t *testing.T) {
	failure := errors.New("history disk refused model publication")
	model := NewCommandModel(FilesystemAuthorityUnknown)
	model.SetHistory(NewHistory(&failingHistoryPublication{failure: failure}, "owner"))
	model.recordHistory("show bgp")
	require.ErrorIs(t, model.Close(), failure)
}
